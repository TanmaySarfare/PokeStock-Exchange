/* PokéStock Exchange — browser bridge.
 *
 * GitHub Pages can't run the Go server, so on the hosted site the exchange
 * runs inside the page: cmd/pokestock-web compiles the same engine and /api
 * handlers to WebAssembly (pokestock.wasm). This script starts it and sends
 * app.js's fetch("/api/…") calls and its EventSource("/api/stream") to Go,
 * so app.js works unchanged.
 *
 * Loaded after wasm_exec.js and before app.js (see `make site`).
 */
(() => {
  "use strict";

  const realFetch = window.fetch.bind(window);
  const RealEventSource = window.EventSource;
  const CONNECTING = 0, OPEN = 1, CLOSED = 2;
  const streams = new Set(); // open live feeds, so they can be shut if the engine stops
  let stopped = false;

  // Shown while the engine loads; app.js replaces it once the market is up.
  const view = document.getElementById("view");
  if (view && !view.childElementCount) view.innerHTML = '<div class="panel">Opening the exchange…</div>';

  // ---- start the engine -------------------------------------------------------
  let engineUp, engineDown;
  const ready = new Promise((resolve, reject) => { engineUp = resolve; engineDown = reject; });
  ready.catch(() => {}); // reported through the API calls below instead
  window.pokestockReady = () => engineUp(); // Go calls this once the market is warmed up

  async function instantiate(go) {
    const url = "pokestock.wasm";
    try {
      return await WebAssembly.instantiateStreaming(realFetch(url), go.importObject);
    } catch {
      // Some hosts send .wasm with the wrong content type; compile from the bytes instead.
      const res = await realFetch(url);
      if (!res.ok) throw new Error(`${url} returned HTTP ${res.status}`);
      return WebAssembly.instantiate(await res.arrayBuffer(), go.importObject);
    }
  }

  (async () => {
    const go = new Go(); // from wasm_exec.js
    const { instance } = await instantiate(go);
    await go.run(instance); // only settles if the Go program exits
    throw new Error("it stopped unexpectedly");
  })().catch((err) => {
    stopped = true;
    engineDown(new Error(`the market engine couldn't start (${err.message}). Reload the page to try again.`));
    for (const es of [...streams]) es._fail();
  });

  async function engine() {
    await ready;
    if (stopped) throw new Error("the market engine stopped. Reload the page to restart it.");
  }

  // "/api/market", "api/market" or "/PokeStock-Exchange/api/market" → "/api/market"
  function apiPath(input) {
    const url = new URL(input, location.href);
    if (url.origin !== location.origin) return null;
    const at = url.pathname.indexOf("/api/");
    return at === -1 ? null : url.pathname.slice(at) + url.search;
  }

  // ---- API calls ----------------------------------------------------------------
  window.fetch = async (input, init) => {
    const req = input instanceof Request ? input : null;
    const path = apiPath(req ? req.url : String(input));
    if (path === null) return realFetch(input, init);
    if (path.startsWith("/api/stream")) throw new TypeError("open /api/stream with EventSource, not fetch");

    const opts = init || {};
    const method = String(opts.method || (req ? req.method : "GET")).toUpperCase();
    let body = "";
    if (opts.body != null) body = String(opts.body);
    else if (req && method !== "GET" && method !== "HEAD") body = await req.text();

    await engine();
    const res = await window.pokestockRequest(method, path, body);
    const noBody = method === "HEAD" || res.status === 204 || res.status === 304;
    return new Response(noBody ? null : res.body, { status: res.status, headers: res.headers });
  };

  // ---- live feed ----------------------------------------------------------------
  // Stands in for EventSource, reading the server-sent events straight from Go.
  class GoEventSource extends EventTarget {
    constructor(url, path) {
      super();
      this.url = url;
      this.withCredentials = false;
      this.readyState = CONNECTING;
      this.onopen = this.onmessage = this.onerror = null;
      this._path = path;
      this._buf = "";
      this._stop = null;
      streams.add(this);
      engine().then(() => this._open(), () => this._fail());
    }

    close() {
      this.readyState = CLOSED;
      streams.delete(this);
      const stop = this._stop;
      this._stop = null;
      if (stop && !stopped) stop();
    }

    _open() {
      if (this.readyState === CLOSED) return; // closed while the engine was loading
      let stop;
      try {
        stop = window.pokestockStream(this._path, (chunk) => {
          // Called from inside Go: return at once and do the work right after,
          // because an error thrown back into Go would stop the engine.
          queueMicrotask(() => (chunk === null ? this._fail() : this._read(chunk)));
        });
      } catch {
        stop = null;
      }
      if (typeof stop !== "function") return this._fail();
      this._stop = stop;
      this.readyState = OPEN;
      this._emit(new Event("open"));
    }

    _fail() {
      if (this.readyState === CLOSED) return;
      this.close();
      this._emit(new Event("error"));
    }

    // Turns "event: tick\ndata: {…}\n\n" blocks into events.
    _read(text) {
      if (this.readyState !== OPEN) return;
      this._buf += text;
      let end;
      while ((end = this._buf.indexOf("\n\n")) !== -1) {
        const block = this._buf.slice(0, end);
        this._buf = this._buf.slice(end + 2);
        let type = "message";
        const data = [];
        for (const line of block.split("\n")) {
          if (!line || line.startsWith(":")) continue; // blank or comment
          const colon = line.indexOf(":");
          const field = colon === -1 ? line : line.slice(0, colon);
          let value = colon === -1 ? "" : line.slice(colon + 1);
          if (value.startsWith(" ")) value = value.slice(1);
          if (field === "event") type = value || "message";
          else if (field === "data") data.push(value);
        }
        if (data.length) this._emit(new MessageEvent(type, { data: data.join("\n"), origin: location.origin }));
      }
    }

    _emit(event) {
      const handler = this["on" + event.type];
      if (typeof handler === "function") {
        try { handler.call(this, event); } catch (err) { setTimeout(() => { throw err; }); }
      }
      this.dispatchEvent(event);
    }
  }

  window.EventSource = function EventSource(url, options) {
    const path = apiPath(String(url));
    if (path === null) return new RealEventSource(url, options);
    return new GoEventSource(new URL(url, location.href).href, path);
  };
  Object.assign(window.EventSource, { CONNECTING, OPEN, CLOSED });
  Object.assign(GoEventSource, { CONNECTING, OPEN, CLOSED });
})();
