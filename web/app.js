/* PokéStock Exchange — frontend. Vanilla JS, no build step. */
(() => {
  "use strict";

  const TYPE_COLORS = {
    Normal: "#A8A77A", Fire: "#EE8130", Water: "#6390F0", Electric: "#F7D02C", Grass: "#7AC74C", Ice: "#96D9D6",
    Fighting: "#C22E28", Poison: "#A33EA1", Ground: "#E2BF65", Flying: "#A98FF3", Psychic: "#F95587", Bug: "#A6B91A",
    Rock: "#B6A136", Ghost: "#735797", Dragon: "#6F35FC", Dark: "#705746", Steel: "#B7B7CE", Fairy: "#D685AD",
  };
  const START_CASH = 10000;

  const state = {
    tickers: new Map(), // symbol -> snapshot (mutated live)
    list: [],
    composite: 1000,
    tick: 0,
    portfolio: null,
    route: null,
    view: null, // per-view handlers { onTick(), destroy() }
    tapeSyms: [],
  };

  // ---------- utils ----------
  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => [...root.querySelectorAll(sel)];
  const fmt = {
    money: (v, d = 2) => (v < 0 ? "-" : "") + "$" + Math.abs(v).toLocaleString(undefined, { minimumFractionDigits: d, maximumFractionDigits: d }),
    pct: (v) => (v > 0 ? "+" : "") + v.toFixed(2) + "%",
    big: (v) => {
      const a = Math.abs(v);
      if (a >= 1e12) return "$" + (v / 1e12).toFixed(2) + "T";
      if (a >= 1e9) return "$" + (v / 1e9).toFixed(2) + "B";
      if (a >= 1e6) return "$" + (v / 1e6).toFixed(1) + "M";
      if (a >= 1e3) return "$" + (v / 1e3).toFixed(1) + "K";
      return "$" + v.toFixed(0);
    },
    time: (unix) => new Date(unix * 1000).toLocaleTimeString([], { hour12: false }),
    int: (v) => v.toLocaleString(),
  };
  const cls = (v) => (v > 0 ? "up" : v < 0 ? "down" : "");
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const pctOf = (t) => (t.open ? (t.price / t.open - 1) * 100 : 0);
  const badge = (type) => `<span class="badge" style="--bc:${TYPE_COLORS[type] || "#555"}">${esc(type)}</span>`;
  const img = (src, cls, size) => `<img class="${cls}" src="${esc(src)}" width="${size}" height="${size}" loading="lazy" decoding="async" alt="">`;
  const el = (html) => { const t = document.createElement("template"); t.innerHTML = html.trim(); return t.content.firstElementChild; };

  async function api(path, opts) {
    const r = await fetch(path, opts);
    const j = await r.json().catch(() => ({}));
    if (!r.ok) throw new Error(j.error || r.statusText);
    return j;
  }
  const post = (path, body) => api(path, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });

  let toastTimer;
  function toast(msg, kind = "") {
    const t = $("#toast");
    t.textContent = msg;
    t.className = "toast show " + kind;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (t.className = "toast"), 3200);
  }

  // ---------- canvas helpers ----------
  const C = { up: "#2ecc71", down: "#ff5c5c", accent: "#ffb020", grid: "rgba(255,255,255,.06)", muted: "#7d8a9b" };
  const MONO = '11px "JetBrains Mono", ui-monospace, Menlo, monospace';

  // Prepare a canvas for crisp HiDPI drawing. Returns {ctx,w,h}.
  function prep(canvas, fallbackH) {
    const dpr = Math.max(1, Math.min(3, window.devicePixelRatio || 1));
    const w = canvas.clientWidth || canvas.width, h = canvas.clientHeight || fallbackH;
    const W = Math.round(w * dpr), H = Math.round(h * dpr);
    if (canvas.width !== W || canvas.height !== H) { canvas.width = W; canvas.height = H; }
    const ctx = canvas.getContext("2d");
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    ctx.lineJoin = "round"; ctx.lineCap = "round";
    ctx.imageSmoothingEnabled = true; ctx.imageSmoothingQuality = "high";
    return { ctx, w, h };
  }

  // Catmull-Rom → bezier smoothing for line series.
  function smoothPath(ctx, pts, tension = 0.5) {
    if (pts.length < 2) return;
    ctx.moveTo(pts[0][0], pts[0][1]);
    for (let i = 0; i < pts.length - 1; i++) {
      const p0 = pts[i - 1] || pts[i], p1 = pts[i], p2 = pts[i + 1], p3 = pts[i + 2] || p2;
      const c1x = p1[0] + ((p2[0] - p0[0]) / 6) * tension, c1y = p1[1] + ((p2[1] - p0[1]) / 6) * tension;
      const c2x = p2[0] - ((p3[0] - p1[0]) / 6) * tension, c2y = p2[1] - ((p3[1] - p1[1]) / 6) * tension;
      ctx.bezierCurveTo(c1x, c1y, c2x, c2y, p2[0], p2[1]);
    }
  }

  function hexA(hex, a) {
    const n = parseInt(hex.slice(1), 16);
    return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`;
  }

  function sparkline(canvas, data, color) {
    const { ctx, w, h } = prep(canvas, 28);
    if (!data || data.length < 2) return;
    let min = Math.min(...data), max = Math.max(...data);
    if (max === min) { max += 1; min -= 1; }
    const pad = 2.5;
    const pts = data.map((v, i) => [pad + (i / (data.length - 1)) * (w - pad * 2), h - pad - ((v - min) / (max - min)) * (h - pad * 2)]);
    const col = color || (data[data.length - 1] >= data[0] ? C.up : C.down);
    // area fill
    ctx.beginPath(); smoothPath(ctx, pts); ctx.lineTo(pts[pts.length - 1][0], h); ctx.lineTo(pts[0][0], h); ctx.closePath();
    const g = ctx.createLinearGradient(0, 0, 0, h); g.addColorStop(0, hexA(col, 0.28)); g.addColorStop(1, hexA(col, 0));
    ctx.fillStyle = g; ctx.fill();
    // line
    ctx.beginPath(); smoothPath(ctx, pts);
    ctx.strokeStyle = col; ctx.lineWidth = 1.6; ctx.stroke();
    // end dot
    const [ex, ey] = pts[pts.length - 1];
    ctx.beginPath(); ctx.arc(ex, ey, 2, 0, Math.PI * 2); ctx.fillStyle = col; ctx.fill();
  }

  function candleChart(canvas, candles, current, open) {
    const { ctx, w, h } = prep(canvas, 340);
    const all = current ? [...candles, current] : candles;
    if (!all.length) return;
    const padL = 10, padR = 72, padT = 14, padB = 44, volH = 34;
    const cw = w - padL - padR, ch = h - padT - padB;
    let lo = Math.min(...all.map((c) => c.l), open), hi = Math.max(...all.map((c) => c.h), open);
    const span = hi - lo || 1; lo -= span * 0.06; hi += span * 0.06;
    const y = (p) => padT + ch - ((p - lo) / (hi - lo)) * ch;
    const n = all.length, step = cw / n, bw = Math.max(3, Math.min(16, step * 0.62));
    const maxV = Math.max(...all.map((c) => c.v), 1);
    const px = (v) => Math.round(v) + 0.5; // crisp 1px lines

    // grid + axis labels
    ctx.font = MONO; ctx.textAlign = "left"; ctx.textBaseline = "middle";
    for (let i = 0; i <= 5; i++) {
      const p = lo + ((hi - lo) * i) / 5, yy = px(y(p));
      ctx.strokeStyle = C.grid; ctx.lineWidth = 1; ctx.beginPath(); ctx.moveTo(padL, yy); ctx.lineTo(w - padR, yy); ctx.stroke();
      ctx.fillStyle = C.muted; ctx.fillText("$" + p.toFixed(2), w - padR + 10, yy);
    }
    // session open line
    ctx.setLineDash([5, 5]); ctx.strokeStyle = hexA(C.accent, 0.6); ctx.beginPath(); ctx.moveTo(padL, px(y(open))); ctx.lineTo(w - padR, px(y(open))); ctx.stroke(); ctx.setLineDash([]);
    ctx.fillStyle = C.accent; ctx.font = '600 10px "JetBrains Mono", ui-monospace, Menlo, monospace'; ctx.textBaseline = "bottom";
    ctx.fillText("OPEN " + open.toFixed(2), padL + 4, y(open) - 3); ctx.font = MONO; ctx.textBaseline = "middle";

    // volume bars
    all.forEach((c, i) => {
      const x = padL + i * step + step / 2, vh = Math.max(1, (c.v / maxV) * volH);
      ctx.fillStyle = hexA(c.c >= c.o ? C.up : C.down, 0.22);
      roundRect(ctx, x - bw / 2, h - padB + 4 + (volH - vh), bw, vh, 2); ctx.fill();
    });
    // candles
    all.forEach((c, i) => {
      const x = px(padL + i * step + step / 2) - 0.5;
      const up = c.c >= c.o, live = i === n - 1 && current;
      const col = up ? C.up : C.down;
      ctx.strokeStyle = col; ctx.lineWidth = 1.2;
      ctx.beginPath(); ctx.moveTo(x, y(c.h)); ctx.lineTo(x, y(c.l)); ctx.stroke();
      const top = y(Math.max(c.o, c.c)), bot = y(Math.min(c.o, c.c));
      const g = ctx.createLinearGradient(0, top, 0, bot); g.addColorStop(0, col); g.addColorStop(1, hexA(col, 0.72));
      ctx.fillStyle = live ? hexA(col, 0.55) : g;
      roundRect(ctx, x - bw / 2, top, bw, Math.max(1.5, bot - top), 2); ctx.fill();
      if (live) { ctx.strokeStyle = col; ctx.lineWidth = 1; ctx.stroke(); }
    });
    // last price tag
    const last = all[n - 1].c, ly = y(last), col = last >= open ? C.up : C.down;
    ctx.fillStyle = col; roundRect(ctx, w - padR + 4, ly - 10, padR - 8, 20, 5); ctx.fill();
    ctx.fillStyle = "#0a0d12"; ctx.font = '600 ' + MONO; ctx.fillText("$" + last.toFixed(2), w - padR + 10, ly);
    // time labels
    ctx.font = MONO; ctx.fillStyle = C.muted; ctx.textAlign = "center"; ctx.textBaseline = "top";
    const every = Math.max(1, Math.ceil(n / 8));
    all.forEach((c, i) => { if (i % every === 0) ctx.fillText(fmt.time(c.t), padL + i * step + step / 2, h - 14); });
  }

  function roundRect(ctx, x, y, w, h, r) {
    r = Math.min(r, w / 2, h / 2);
    ctx.beginPath();
    ctx.moveTo(x + r, y); ctx.arcTo(x + w, y, x + w, y + h, r); ctx.arcTo(x + w, y + h, x, y + h, r);
    ctx.arcTo(x, y + h, x, y, r); ctx.arcTo(x, y, x + w, y, r); ctx.closePath();
  }

  function lineChart(canvas, data, baseline) {
    const { ctx, w, h } = prep(canvas, 110);
    if (!data || data.length < 2) {
      ctx.fillStyle = C.muted; ctx.font = MONO; ctx.textBaseline = "middle";
      ctx.fillText("Net-worth history builds as the market ticks.", 10, h / 2);
      return;
    }
    let min = Math.min(...data, baseline), max = Math.max(...data, baseline);
    if (max === min) { max += 1; min -= 1; }
    const pad = 8;
    const y = (v) => h - pad - ((v - min) / (max - min)) * (h - pad * 2);
    const pts = data.map((v, i) => [pad + (i / (data.length - 1)) * (w - pad * 2), y(v)]);
    const col = data[data.length - 1] >= baseline ? C.up : C.down;
    ctx.setLineDash([4, 5]); ctx.strokeStyle = hexA(C.accent, 0.6); ctx.lineWidth = 1;
    ctx.beginPath(); ctx.moveTo(pad, Math.round(y(baseline)) + 0.5); ctx.lineTo(w - pad, Math.round(y(baseline)) + 0.5); ctx.stroke(); ctx.setLineDash([]);
    ctx.beginPath(); smoothPath(ctx, pts, 0.6); ctx.lineTo(pts[pts.length - 1][0], h); ctx.lineTo(pts[0][0], h); ctx.closePath();
    const g = ctx.createLinearGradient(0, 0, 0, h); g.addColorStop(0, hexA(col, 0.3)); g.addColorStop(1, hexA(col, 0));
    ctx.fillStyle = g; ctx.fill();
    ctx.beginPath(); smoothPath(ctx, pts, 0.6); ctx.strokeStyle = col; ctx.lineWidth = 2; ctx.stroke();
    const [ex, ey] = pts[pts.length - 1];
    ctx.beginPath(); ctx.arc(ex, ey, 3.5, 0, Math.PI * 2); ctx.fillStyle = col; ctx.fill();
    ctx.beginPath(); ctx.arc(ex, ey, 7, 0, Math.PI * 2); ctx.fillStyle = hexA(col, 0.25); ctx.fill();
  }

  // ---------- header / tape ----------
  function renderHeader() {
    const c = state.composite, p = (c / 1000 - 1) * 100;
    $("#composite-val").textContent = c.toFixed(2);
    const cc = $("#composite-chg"); cc.textContent = fmt.pct(p); cc.className = "chg " + cls(p);
    if (state.portfolio) {
      const pf = state.portfolio;
      let equity = 0;
      for (const h of pf.holdings || []) { const t = state.tickers.get(h.symbol); equity += (t ? t.price : h.price) * h.qty; }
      const net = pf.cash + equity, pnl = (net / START_CASH - 1) * 100;
      $("#acct-name").textContent = pf.name;
      $("#acct-net").textContent = fmt.money(net);
      const ap = $("#acct-pnl"); ap.textContent = fmt.pct(pnl); ap.className = "chg " + cls(pnl);
    }
  }

  function buildTape() {
    const syms = state.list.map((t) => t.symbol);
    state.tapeSyms = [];
    for (let i = 0; i < 40 && syms.length; i++) state.tapeSyms.push(syms.splice(Math.floor(Math.random() * syms.length), 1)[0]);
    const track = $("#tape-track");
    track.innerHTML = "";
    for (let rep = 0; rep < 2; rep++) {
      for (const s of state.tapeSyms) track.appendChild(el(`<a href="#/t/${s}" data-sym="${s}"><span class="s">${s}</span><span class="p"></span> <span class="c"></span></a>`));
    }
    updateTape();
  }
  function updateTape() {
    for (const a of $$("#tape-track a")) {
      const t = state.tickers.get(a.dataset.sym); if (!t) continue;
      const p = pctOf(t);
      a.querySelector(".p").textContent = t.price.toFixed(2);
      const c = a.querySelector(".c"); c.textContent = fmt.pct(p); c.className = "c " + cls(p);
    }
  }

  // ---------- views ----------
  const views = {};

  // Shared listing-row renderer + live updater (used by Market and Sectors).
  function rowHTML(t, withBeta = true) {
    const p = pctOf(t);
    return `<tr data-sym="${t.symbol}">
      <td class="muted">${t.dex}</td>
      <td class="sym">${t.symbol}</td>
      <td>${img(t.image, "sprite", 40)}${esc(t.name)}${t.rarity !== "None" ? ` <span class="badge rarity">${t.rarity[0]}</span>` : ""}</td>
      <td>${badge(t.sector)}${t.subsector ? badge(t.subsector) : ""}</td>
      <td class="num price">${t.price.toFixed(2)}</td>
      <td class="num pct ${cls(p)}">${fmt.pct(p)}</td>
      <td class="trend"><canvas class="spark"></canvas></td>
      <td class="num mcap">${fmt.big(t.price * t.shares)}</td>
      ${withBeta ? `<td class="num muted">${(t.volatility * 100).toFixed(2)}</td>` : ""}
    </tr>`;
  }
  function drawSparks(tbody) {
    for (const tr of tbody.children) { const t = state.tickers.get(tr.dataset.sym); if (t) sparkline(tr.querySelector(".spark"), t.spark); }
  }
  function updateRows(tbody) {
    for (const tr of tbody.children) {
      const t = state.tickers.get(tr.dataset.sym); if (!t) continue;
      const p = pctOf(t), priceCell = tr.querySelector(".price");
      const old = parseFloat(priceCell.textContent);
      priceCell.textContent = t.price.toFixed(2);
      const pc = tr.querySelector(".pct"); pc.textContent = fmt.pct(p); pc.className = "num pct " + cls(p);
      tr.querySelector(".mcap").textContent = fmt.big(t.price * t.shares);
      if (t.price !== old) { tr.classList.remove("flash-up", "flash-down"); void tr.offsetWidth; tr.classList.add(t.price > old ? "flash-up" : "flash-down"); }
      sparkline(tr.querySelector(".spark"), t.spark);
    }
  }

  // Aggregate every type sector from live prices.
  function sectorAgg() {
    const agg = {};
    for (const t of state.list) {
      t.types = t.types || [t.sector, t.subsector].filter(Boolean);
      for (const ty of t.types) {
        agg[ty] ??= { type: ty, n: 0, sum: 0, best: null, worst: null, cap: 0, members: [] };
        const a = agg[ty], p = pctOf(t);
        a.n++; a.sum += p; a.members.push(t);
        if (ty === t.sector) a.cap += t.price * t.shares;
        if (!a.best || p > pctOf(a.best)) a.best = t;
        if (!a.worst || p < pctOf(a.worst)) a.worst = t;
      }
    }
    for (const a of Object.values(agg)) a.avg = a.sum / a.n;
    return agg;
  }
  function renderHeatmap(hm, agg, active) {
    hm.innerHTML = Object.keys(TYPE_COLORS).filter((k) => agg[k]).map((k) => {
      const a = agg[k], inten = Math.min(1, Math.abs(a.avg) / 4);
      const base = a.avg >= 0 ? "46,204,113" : "255,92,92";
      const bg = `linear-gradient(160deg, rgba(${base},${0.14 + inten * 0.5}), rgba(${base},${0.06 + inten * 0.3}))`;
      return `<div class="tile ${k === active ? "active" : ""}" data-sector="${k}" style="--tile-bg:${bg};--tc:${TYPE_COLORS[k]}">
        <div class="t">${k}</div>
        <div class="p ${cls(a.avg)}">${fmt.pct(a.avg)}</div>
        <div class="n">${a.n} listings · ★ ${a.best.symbol}</div></div>`;
    }).join("");
  }

  views.market = () => {
    const root = $("#view");
    root.appendChild($("#tpl-market").content.cloneNode(true));
    let sortKey = "dex", sortDir = 1, limit = 60, q = "", sector = "", rarity = "";
    const tbody = $("#tbody");
    const sel = $("#sector-filter");
    Object.keys(TYPE_COLORS).forEach((t) => sel.appendChild(el(`<option>${t}</option>`)));

    function filtered() {
      const ql = q.toLowerCase();
      let rows = state.list.filter((t) =>
        (!ql || t.symbol.toLowerCase().includes(ql) || t.name.toLowerCase().includes(ql) || t.sector.toLowerCase() === ql || (t.subsector || "").toLowerCase() === ql) &&
        (!sector || t.sector === sector || t.subsector === sector) &&
        (!rarity || t.rarity === rarity));
      const key = sortKey === "pct" ? pctOf : sortKey === "mcap" ? (t) => t.price * t.shares : (t) => t[sortKey];
      rows.sort((a, b) => { const x = key(a), y = key(b); return (x > y ? 1 : x < y ? -1 : 0) * sortDir; });
      return rows;
    }

    function renderTable() {
      const rows = filtered();
      $("#list-count").textContent = `${rows.length} shown`;
      tbody.innerHTML = rows.slice(0, limit).map((t) => rowHTML(t)).join("");
      $("#more").style.display = rows.length > limit ? "" : "none";
      drawSparks(tbody);
      $$("th[data-sort]").forEach((th) => th.classList.toggle("sorted", th.dataset.sort === sortKey));
    }

    function renderMovers() {
      const sorted = [...state.list].sort((a, b) => pctOf(b) - pctOf(a));
      const li = (t) => { const p = pctOf(t); return `<li data-sym="${t.symbol}">${img(t.image, "", 44)}<span class="sym">${t.symbol}</span><span class="name">${esc(t.name)}</span><span class="price">${t.price.toFixed(2)}</span><span class="pct ${cls(p)}">${fmt.pct(p)}</span></li>`; };
      $("#gainers").innerHTML = sorted.slice(0, 8).map(li).join("");
      $("#losers").innerHTML = sorted.slice(-8).reverse().map(li).join("");
    }

    renderTable(); renderMovers();

    $("#search").addEventListener("input", (e) => { q = e.target.value.trim(); limit = 60; renderTable(); });
    sel.addEventListener("change", (e) => { sector = e.target.value; limit = 60; renderTable(); });
    $("#rarity-filter").addEventListener("change", (e) => { rarity = e.target.value; limit = 60; renderTable(); });
    $("#more").addEventListener("click", () => { limit += 100; renderTable(); });
    $$("th[data-sort]").forEach((th) => th.addEventListener("click", () => {
      const k = th.dataset.sort;
      if (sortKey === k) sortDir *= -1; else { sortKey = k; sortDir = k === "dex" || k === "symbol" || k === "name" ? 1 : -1; }
      renderTable();
    }));
    const ac = new AbortController();
    root.addEventListener("click", (e) => {
      const tr = e.target.closest("tr[data-sym], li[data-sym]");
      if (tr) location.hash = "#/t/" + tr.dataset.sym;
    }, { signal: ac.signal });

    return {
      onTick() { updateRows(tbody); renderMovers(); },
      destroy() { ac.abort(); },
    };
  };

  views.sectors = (initial) => {
    const root = $("#view");
    root.appendChild($("#tpl-sectors").content.cloneNode(true));
    const hm = $("#heatmap"), detail = $("#sector-detail"), tbody = $("#sector-tbody");
    let active = initial && TYPE_COLORS[initial] ? initial : null;

    function render() {
      const agg = sectorAgg();
      renderHeatmap(hm, agg, active);
      if (!active || !agg[active]) { detail.hidden = true; return; }
      const a = agg[active];
      detail.hidden = false;
      $("#sector-title").innerHTML = `${badge(active)} <span style="color:var(--text)">${active} sector</span> <small>${a.n} listings (primary + secondary exposure)</small>`;
      $("#sector-kpis").innerHTML = `
        <div><span class="lbl">Avg change</span><span class="val ${cls(a.avg)}">${fmt.pct(a.avg)}</span></div>
        <div><span class="lbl">Sector cap</span><span class="val">${fmt.big(a.cap)}</span></div>
        <div><span class="lbl">Leader</span><a class="val" href="#/t/${a.best.symbol}">${a.best.symbol} <span class="up">${fmt.pct(pctOf(a.best))}</span></a></div>
        <div><span class="lbl">Laggard</span><a class="val" href="#/t/${a.worst.symbol}">${a.worst.symbol} <span class="down">${fmt.pct(pctOf(a.worst))}</span></a></div>`;
      const rows = [...a.members].sort((x, y) => pctOf(y) - pctOf(x));
      tbody.innerHTML = rows.map((t) => rowHTML(t, false)).join("");
      drawSparks(tbody);
    }
    render();

    const ac = new AbortController();
    root.addEventListener("click", (e) => {
      const tile = e.target.closest(".tile");
      if (tile) { active = tile.dataset.sector; history.replaceState(null, "", "#/sectors/" + active); render(); detail.scrollIntoView({ behavior: "smooth", block: "start" }); return; }
      const tr = e.target.closest("tr[data-sym]");
      if (tr) location.hash = "#/t/" + tr.dataset.sym;
    }, { signal: ac.signal });

    return {
      onTick() { renderHeatmap(hm, sectorAgg(), active); if (active) updateRows(tbody); },
      destroy() { ac.abort(); },
    };
  };

  views.ticker = (sym) => {
    const root = $("#view");
    root.appendChild($("#tpl-ticker").content.cloneNode(true));
    let detail = null, alive = true, rendered = false;

    async function load() {
      try { detail = await api("/api/ticker/" + sym); } catch (e) { root.innerHTML = `<div class="panel">Unknown ticker ${esc(sym)}. <a href="#/">Back to market</a></div>`; return; }
      if (!alive) return;
      document.title = `${sym} ${detail.price.toFixed(2)} · PokéStock`;
      if (!rendered) { renderStatic(); rendered = true; }
      renderDynamic();
    }

    function renderDynamic() {
      for (const f of detail.family) {
        const live = state.tickers.get(f.symbol) || f, p = pctOf(live), span = root.querySelector(`.fam[href="#/t/${f.symbol}"] .fp`);
        if (span) { span.textContent = fmt.pct(p); span.className = "fp " + cls(p); }
      }
      renderNews($("#t-news"), detail.news, true);
      renderQuote();
      candleChart($("#chart"), detail.candles, detail.current, detail.open);
      $("#chart-sub").textContent = `${detail.candles.length + 1} candles · 10 ticks each · session open ${fmt.money(detail.open)}`;
      $("#kv-volume").textContent = fmt.int(detail.volume);
    }

    function renderStatic() {
      const L = detail.listing;
      $("#t-art").src = L.image;
      $(".ticker-head").style.setProperty("--hero", TYPE_COLORS[L.sector] || "#fff");
      $("#t-sym").textContent = sym;
      $("#t-badges").innerHTML = badge(L.sector) + (L.subsector ? badge(L.subsector) : "") +
        (L.rarity !== "None" ? `<span class="badge rarity">${L.rarity}</span>` : "") + (L.group !== "None" ? `<span class="badge group">${L.group}</span>` : "");
      $("#t-name").textContent = L.name;
      $("#t-cat").textContent = `#${String(L.dex).padStart(4, "0")} · ${L.category} · Gen ${L.generation}`;
      $("#t-prospectus").textContent = L.prospectus;
      const chips = [
        ...L.weak4x.map((w) => `<span class="chip weak weak4">4× ${esc(w)}</span>`),
        ...L.weak2x.map((w) => `<span class="chip weak">2× ${esc(w)}</span>`),
        ...L.resists.map((w) => w.includes("¼") ? `<span class="chip res">¼× ${esc(w.replace(/\s*\(¼×\)/, ""))}</span>` : `<span class="chip res">½× ${esc(w)}</span>`),
        ...L.immune.map((w) => `<span class="chip imm">0× ${esc(w)}</span>`),
      ];
      $("#t-risk").innerHTML = chips.join("");
      const S = L.stats;
      $("#stat-bars").innerHTML = [["HP", S.hp], ["Attack", S.atk], ["Defense", S.def], ["Sp. Atk", S.spa], ["Sp. Def", S.spd], ["Speed", S.spe]]
        .map(([n, v]) => `<div class="sb"><span>${n}</span><div class="bar"><i style="width:${Math.min(100, (v / 200) * 100)}%"></i></div><span class="v">${v}</span></div>`).join("");
      const abil = L.abilities.join(", ") + (L.hidden_ability ? ` · <span class="muted">${esc(L.hidden_ability)} (hidden)</span>` : "");
      $("#kv").innerHTML = [
        ["BST", S.bst], ["IPO price", fmt.money(L.ipo_price)], ["Float", fmt.int(L.shares)], ["Spread", (detail.spread * 100).toFixed(2) + "%"],
        ["Volatility β", (L.volatility * 100).toFixed(2)], ["Height", L.height_m != null ? L.height_m + " m" : "—"], ["Weight", L.weight_kg != null ? L.weight_kg + " kg" : "—"],
        ["% male", L.pct_male != null ? L.pct_male + "%" : "Genderless"], ["Abilities", abil], ["Stage", L.stage + (L.evolves_from ? ` (from ${esc(L.evolves_from)})` : "")],
        ["Evolves via", L.evolution_method ? esc(L.evolution_method) : "—"], ["Volume", `<span id="kv-volume">${fmt.int(detail.volume)}</span>`],
      ].map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
      $("#t-family").innerHTML = detail.family.sort((a, b) => (a.stage - b.stage) || (a.dex - b.dex)).map((f, i) =>
        (i ? `<span class="arrow">→</span>` : "") + `<a class="fam ${f.symbol === sym ? "me" : ""}" href="#/t/${f.symbol}">${img(f.image, "", 44)}<div><div class="sym">${f.symbol}</div><div class="muted small">${esc(f.name)} <span class="fp"></span></div></div></a>`
      ).join("");
    }

    function renderQuote() {
      const t = state.tickers.get(sym) || detail;
      const p = pctOf(t);
      $("#t-price").textContent = fmt.money(t.price);
      const c = $("#t-chg"); c.textContent = `${fmt.pct(p)} (${fmt.money(t.price - t.open)})`; c.className = "chg " + cls(p);
      $("#t-range").textContent = `Session range ${fmt.money(t.low)} – ${fmt.money(t.high)} · Mkt cap ${fmt.big(t.price * t.shares)}`;
      const pf = state.portfolio, pos = pf?.holdings?.find((h) => h.symbol === sym);
      const qty = Math.max(1, parseInt($("#trade-qty").value) || 1);
      $("#trade-info").innerHTML = `Cash ${fmt.money(pf?.cash ?? 0)} · ${qty} sh ≈ ${fmt.money(qty * t.price)}` + (pos ? ` · You hold <b>${pos.qty}</b> @ ${fmt.money(pos.avg_cost)}` : "");
    }

    async function trade(side) {
      const qty = parseInt($("#trade-qty").value);
      try {
        const r = await post("/api/trade", { symbol: sym, side, qty });
        state.portfolio = r.portfolio;
        toast(`${side === "buy" ? "Bought" : "Sold"} ${qty} ${sym} @ ${fmt.money(r.trade.price)} (${fmt.money(r.trade.total)})`, "ok");
        renderQuote(); renderHeader();
      } catch (e) { toast(e.message, "err"); }
    }
    $("#btn-buy").addEventListener("click", () => trade("buy"));
    $("#btn-sell").addEventListener("click", () => trade("sell"));
    $("#trade-qty").addEventListener("input", renderQuote);
    $("#trade-form").addEventListener("submit", (e) => e.preventDefault());

    load();
    return {
      onTick() { if (detail) { renderQuote(); load(); } },
      destroy() { alive = false; document.title = "PokéStock Exchange"; },
    };
  };

  function renderNews(ul, items, compact) {
    const seen = new Set($$("li", ul).map((li) => li.dataset.id));
    items = items || [];
    if (!items.length) { ul.innerHTML = `<li class="muted" style="display:block">No headlines yet. This one's flying under the radar.</li>`; return; }
    ul.innerHTML = items.map((n) => `<li data-id="${n.id}" class="${seen.size && !seen.has(String(n.id)) ? "fresh" : ""}">
      <time>${fmt.time(n.time)}</time>
      <div><div class="head"><span class="kind">${n.kind}</span>${esc(n.headline)}</div>
        ${compact ? "" : `<div class="body">${esc(n.body)}</div>`}
        ${n.symbols?.length ? `<div class="syms">${n.symbols.map((s) => `<a href="#/t/${s}">${s}</a>`).join("")}</div>` : ""}
      </div>
      <div class="impact ${cls(n.impact)}">${n.impact ? fmt.pct(n.impact) : ""}</div></li>`).join("");
  }

  views.news = () => {
    $("#view").appendChild($("#tpl-news").content.cloneNode(true));
    const ul = $("#news-list");
    const load = () => api("/api/news?limit=80").then((n) => renderNews(ul, n, false)).catch(() => {});
    load();
    return { onTick(u) { if (u?.news) load(); } };
  };

  views.portfolio = () => {
    $("#view").appendChild($("#tpl-portfolio").content.cloneNode(true));
    async function load() {
      const pf = await api("/api/me");
      state.portfolio = pf;
      $("#name-input").value = pf.name;
      $("#tag-input").value = pf.tagline || "";
      render(pf);
    }
    function render(pf) {
      let equity = 0;
      for (const h of pf.holdings || []) { const t = state.tickers.get(h.symbol); h.price = t ? t.price : h.price; h.value = h.price * h.qty; h.pnl = h.value - h.avg_cost * h.qty; equity += h.value; }
      const net = pf.cash + equity, pnl = net - START_CASH;
      $("#p-net").textContent = fmt.money(net);
      $("#p-cash").textContent = fmt.money(pf.cash);
      $("#p-equity").textContent = fmt.money(equity);
      const pp = $("#p-pnl"); pp.textContent = `${fmt.money(pnl)} (${fmt.pct((net / START_CASH - 1) * 100)})`; pp.className = "val " + cls(pnl);
      const pr = $("#p-real"); pr.textContent = fmt.money(pf.realized); pr.className = "val " + cls(pf.realized);
      $("#p-rank").textContent = `#${pf.rank} of ${pf.players}`;
      lineChart($("#nw-chart"), pf.net_worth_history, START_CASH);
      const tb = $("#holdings tbody");
      tb.innerHTML = (pf.holdings || []).map((h) => `<tr data-sym="${h.symbol}">
        <td>${img(h.image, "sprite", 34)}</td><td class="sym">${h.symbol}</td><td>${esc(h.name)} ${badge(h.sector)}</td>
        <td class="num">${fmt.int(h.qty)}</td><td class="num">${fmt.money(h.avg_cost)}</td><td class="num">${fmt.money(h.price)}</td><td class="num">${fmt.money(h.value)}</td>
        <td class="num ${cls(h.pnl)}">${fmt.money(h.pnl)} (${fmt.pct(h.avg_cost ? (h.price / h.avg_cost - 1) * 100 : 0)})</td>
        <td><button class="sellbtn" data-sell="${h.symbol}" data-qty="${h.qty}">Sell all</button></td></tr>`).join("");
      $("#holdings-empty").style.display = pf.holdings?.length ? "none" : "block";
      $("#trades tbody").innerHTML = (pf.trades || []).map((t) => `<tr data-sym="${t.symbol}"><td class="muted">${fmt.time(t.time)}</td><td class="${t.side === "buy" ? "up" : "down"}">${t.side.toUpperCase()}</td><td class="sym">${t.symbol}</td><td class="num">${fmt.int(t.qty)}</td><td class="num">${fmt.money(t.price)}</td><td class="num">${fmt.money(t.total)}</td></tr>`).join("");
    }
    $("#name-save").addEventListener("click", async () => {
      try { state.portfolio = await post("/api/me", { name: $("#name-input").value, tagline: $("#tag-input").value }); render(state.portfolio); renderHeader(); toast("Profile saved", "ok"); } catch (e) { toast(e.message, "err"); }
    });
    const ac = new AbortController();
    $("#view").addEventListener("click", async (e) => {
      const b = e.target.closest("[data-sell]");
      if (b) {
        e.stopPropagation();
        try { const r = await post("/api/trade", { symbol: b.dataset.sell, side: "sell", qty: +b.dataset.qty }); state.portfolio = r.portfolio; render(r.portfolio); renderHeader(); toast(`Sold ${b.dataset.qty} ${b.dataset.sell}`, "ok"); } catch (err) { toast(err.message, "err"); }
        return;
      }
      const tr = e.target.closest("tr[data-sym]");
      if (tr) location.hash = "#/t/" + tr.dataset.sym;
    }, { signal: ac.signal });
    load().catch((e) => toast(e.message, "err"));
    return { onTick() { if (state.portfolio) render(state.portfolio); }, destroy() { ac.abort(); } };
  };

  const ROLE_CLASS = { "Gym Leader": "gym", "Elite Four": "e4", "Champion": "champ" };

  views.leaderboard = () => {
    $("#view").appendChild($("#tpl-leaderboard").content.cloneNode(true));
    const load = () => api("/api/leaderboard").then((rows) => {
      const meId = state.portfolio && state.portfolio.id ? state.portfolio.id.slice(0, 8) : null;
      $("#lb tbody").innerHTML = rows.map((r, i) => {
        const me = !r.bot && r.id === meId;
        const roleTag = r.role ? `<span class="tag ${ROLE_CLASS[r.role] || "gym"}">${esc(r.role)}</span>` : "";
        const who = r.bot
          ? `<span class="lb-name">${esc(r.name)}</span>${roleTag}<span class="lb-tag">${esc(r.tagline || "")}</span>`
          : `<span class="lb-name">${esc(r.name)}</span>${me ? `<span class="tag you">You</span>` : ""}${r.tagline ? `<span class="lb-tag">${esc(r.tagline)}</span>` : ""}`;
        return `<tr class="${me ? "is-me" : ""}"><td class="muted">${i + 1}</td><td class="lb-who">${who}</td><td class="num">${fmt.money(r.net_worth)}</td><td class="num ${cls(r.pnl_pct)}">${fmt.pct(r.pnl_pct)}</td><td class="num">${r.holdings}</td><td>${r.top ? `<a href="#/t/${r.top}" class="sym">${r.top}</a>` : "—"}</td></tr>`;
      }).join("");
    }).catch(() => {});
    load();
    let n = 0;
    return { onTick() { if (++n % 3 === 0) load(); } };
  };

  // ---------- router ----------
  function route() {
    const hash = location.hash || "#/";
    const m = hash.match(/^#\/t\/([A-Za-z0-9]+)/);
    const s = hash.match(/^#\/sectors(?:\/([A-Za-z]+))?/);
    const name = m ? "ticker" : s ? "sectors" : (hash.replace(/^#\//, "") || "market");
    state.view?.destroy?.();
    $("#view").innerHTML = "";
    $$(".nav a").forEach((a) => a.classList.toggle("active", a.dataset.route === name));
    const factory = views[name] || views.market;
    const arg = m ? m[1].toUpperCase() : s && s[1] ? s[1][0].toUpperCase() + s[1].slice(1).toLowerCase() : undefined;
    state.view = factory(arg);
    state.route = name;
    window.scrollTo(0, 0);
  }

  // ---------- live feed ----------
  function connect() {
    const es = new EventSource("/api/stream");
    es.onopen = () => $("#live-dot").classList.add("on");
    es.onerror = () => $("#live-dot").classList.remove("on");
    es.addEventListener("tick", (e) => {
      const u = JSON.parse(e.data);
      state.tick = u.tick; state.composite = u.composite;
      for (const [sym, price] of Object.entries(u.prices)) {
        const t = state.tickers.get(sym); if (!t) continue;
        t.price = price;
        if (price > t.high) t.high = price;
        if (price < t.low) t.low = price;
        t.spark.push(price); if (t.spark.length > 40) t.spark.shift();
      }
      renderHeader(); updateTape();
      if (u.news) toast(`${u.news.headline}`, u.news.impact >= 0 ? "ok" : "err");
      state.view?.onTick?.(u);
    });
  }

  function tickClock() { $("#clock").textContent = new Date().toLocaleTimeString([], { hour12: false }); }

  async function boot() {
    const [m, pf] = await Promise.all([api("/api/market"), api("/api/me")]);
    state.composite = m.composite; state.tick = m.tick; state.portfolio = pf;
    for (const t of m.tickers) { t.shares = t.mcap / t.price; state.tickers.set(t.symbol, t); }
    state.list = m.tickers;
    renderHeader(); buildTape();
    route();
    connect();
    setInterval(tickClock, 1000); tickClock();
  }

  window.addEventListener("hashchange", route);
  boot().catch((e) => { $("#view").innerHTML = `<div class="panel">Failed to load market: ${esc(e.message)}</div>`; });
})();
