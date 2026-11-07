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
  const badge = (type) => `<span class="badge" style="background:${TYPE_COLORS[type] || "#555"}">${esc(type)}</span>`;
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
  function sparkline(canvas, data, color) {
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth || 90, h = canvas.clientHeight || 22;
    if (canvas.width !== w * dpr) { canvas.width = w * dpr; canvas.height = h * dpr; }
    const ctx = canvas.getContext("2d");
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    if (!data || data.length < 2) return;
    let min = Math.min(...data), max = Math.max(...data);
    if (max === min) { max += 1; min -= 1; }
    ctx.beginPath();
    data.forEach((v, i) => {
      const x = (i / (data.length - 1)) * (w - 2) + 1;
      const y = h - 2 - ((v - min) / (max - min)) * (h - 4);
      i ? ctx.lineTo(x, y) : ctx.moveTo(x, y);
    });
    ctx.strokeStyle = color || (data[data.length - 1] >= data[0] ? "#22c55e" : "#ef4444");
    ctx.lineWidth = 1.5;
    ctx.stroke();
  }

  function candleChart(canvas, candles, current, open) {
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth, h = canvas.clientHeight || 320;
    canvas.width = w * dpr; canvas.height = h * dpr;
    const ctx = canvas.getContext("2d");
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    const all = current ? [...candles, current] : candles;
    if (!all.length) return;
    const padL = 8, padR = 64, padT = 10, padB = 36;
    const cw = w - padL - padR, ch = h - padT - padB;
    let lo = Math.min(...all.map((c) => c.l), open), hi = Math.max(...all.map((c) => c.h), open);
    const span = hi - lo || 1; lo -= span * 0.05; hi += span * 0.05;
    const y = (p) => padT + ch - ((p - lo) / (hi - lo)) * ch;
    const n = all.length, step = cw / n, bw = Math.max(2, Math.min(14, step * 0.65));
    const maxV = Math.max(...all.map((c) => c.v), 1);

    // grid + axis labels
    ctx.font = "11px ui-monospace, Menlo, monospace";
    ctx.textAlign = "left"; ctx.textBaseline = "middle";
    for (let i = 0; i <= 5; i++) {
      const p = lo + ((hi - lo) * i) / 5, yy = y(p);
      ctx.strokeStyle = "#1a2330"; ctx.beginPath(); ctx.moveTo(padL, yy); ctx.lineTo(w - padR, yy); ctx.stroke();
      ctx.fillStyle = "#7f8ea3"; ctx.fillText("$" + p.toFixed(2), w - padR + 6, yy);
    }
    // session open line
    ctx.setLineDash([4, 4]); ctx.strokeStyle = "#f59e0b88"; ctx.beginPath(); ctx.moveTo(padL, y(open)); ctx.lineTo(w - padR, y(open)); ctx.stroke(); ctx.setLineDash([]);
    ctx.fillStyle = "#f59e0b"; ctx.fillText("open", w - padR + 6, y(open) - 12);

    // volume
    all.forEach((c, i) => {
      const x = padL + i * step + step / 2;
      const vh = (c.v / maxV) * 26;
      ctx.fillStyle = c.c >= c.o ? "#22c55e33" : "#ef444433";
      ctx.fillRect(x - bw / 2, h - padB + 2 + (26 - vh), bw, vh);
    });
    // candles
    all.forEach((c, i) => {
      const x = padL + i * step + step / 2;
      const up = c.c >= c.o;
      const col = i === n - 1 && current ? (up ? "#86efac" : "#fca5a5") : up ? "#22c55e" : "#ef4444";
      ctx.strokeStyle = col; ctx.fillStyle = col; ctx.lineWidth = 1;
      ctx.beginPath(); ctx.moveTo(x, y(c.h)); ctx.lineTo(x, y(c.l)); ctx.stroke();
      const top = y(Math.max(c.o, c.c)), bot = y(Math.min(c.o, c.c));
      ctx.fillRect(x - bw / 2, top, bw, Math.max(1, bot - top));
    });
    // time labels
    ctx.fillStyle = "#7f8ea3"; ctx.textAlign = "center"; ctx.textBaseline = "top";
    const every = Math.max(1, Math.floor(n / 8));
    all.forEach((c, i) => { if (i % every === 0) ctx.fillText(fmt.time(c.t), padL + i * step + step / 2, h - 10); });
  }

  function lineChart(canvas, data, baseline) {
    const dpr = window.devicePixelRatio || 1;
    const w = canvas.clientWidth, h = canvas.clientHeight || 90;
    canvas.width = w * dpr; canvas.height = h * dpr;
    const ctx = canvas.getContext("2d");
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    if (!data || data.length < 2) {
      ctx.fillStyle = "#7f8ea3"; ctx.font = "11px ui-monospace, monospace"; ctx.fillText("Net-worth history builds as you trade and check back.", 8, h / 2);
      return;
    }
    let min = Math.min(...data, baseline), max = Math.max(...data, baseline);
    if (max === min) { max += 1; min -= 1; }
    const y = (v) => h - 6 - ((v - min) / (max - min)) * (h - 12);
    ctx.setLineDash([3, 3]); ctx.strokeStyle = "#f59e0b88"; ctx.beginPath(); ctx.moveTo(0, y(baseline)); ctx.lineTo(w, y(baseline)); ctx.stroke(); ctx.setLineDash([]);
    ctx.beginPath();
    data.forEach((v, i) => { const x = (i / (data.length - 1)) * (w - 4) + 2; i ? ctx.lineTo(x, y(v)) : ctx.moveTo(x, y(v)); });
    ctx.strokeStyle = data[data.length - 1] >= baseline ? "#22c55e" : "#ef4444"; ctx.lineWidth = 2; ctx.stroke();
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

    function rowHTML(t) {
      const p = pctOf(t);
      return `<tr data-sym="${t.symbol}">
        <td class="muted">${t.dex}</td>
        <td class="sym">${t.symbol}</td>
        <td><img class="sprite" loading="lazy" src="${esc(t.image)}" alt="">${esc(t.name)}${t.rarity !== "None" ? ` <span class="badge rarity">${t.rarity[0]}</span>` : ""}</td>
        <td>${badge(t.sector)}${t.subsector ? badge(t.subsector) : ""}</td>
        <td class="num price">${t.price.toFixed(2)}</td>
        <td class="num pct ${cls(p)}">${fmt.pct(p)}</td>
        <td class="num"><canvas class="spark"></canvas></td>
        <td class="num mcap">${fmt.big(t.price * t.shares)}</td>
        <td class="num muted">${(t.volatility * 100).toFixed(2)}</td>
      </tr>`;
    }

    function renderTable() {
      const rows = filtered();
      $("#list-count").textContent = `${rows.length} shown`;
      tbody.innerHTML = rows.slice(0, limit).map(rowHTML).join("");
      $("#more").style.display = rows.length > limit ? "" : "none";
      for (const tr of tbody.children) sparkline(tr.querySelector(".spark"), state.tickers.get(tr.dataset.sym).spark);
      $$("th[data-sort]").forEach((th) => th.classList.toggle("sorted", th.dataset.sort === sortKey));
    }

    function updateTable() {
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

    function renderHeatmap() {
      const agg = {};
      for (const t of state.list) {
        for (const ty of [t.sector, t.subsector]) {
          if (!ty) continue;
          agg[ty] ??= { n: 0, sum: 0, best: null };
          agg[ty].n++; agg[ty].sum += pctOf(t);
          if (!agg[ty].best || pctOf(t) > pctOf(agg[ty].best)) agg[ty].best = t;
        }
      }
      const hm = $("#heatmap");
      hm.innerHTML = Object.keys(TYPE_COLORS).filter((k) => agg[k]).map((k) => {
        const a = agg[k], avg = a.sum / a.n, inten = Math.min(1, Math.abs(avg) / 4);
        const bg = avg >= 0 ? `rgba(34,197,94,${0.12 + inten * 0.55})` : `rgba(239,68,68,${0.12 + inten * 0.55})`;
        return `<div class="tile" data-sector="${k}" style="background:${bg};--tc:${TYPE_COLORS[k]}">
          <div class="t">${k}</div>
          <div class="p">${fmt.pct(avg)}</div>
          <div class="n">${a.n} · ★ ${a.best.symbol}</div></div>`;
      }).join("");
    }

    function renderMovers() {
      const sorted = [...state.list].sort((a, b) => pctOf(b) - pctOf(a));
      const li = (t) => { const p = pctOf(t); return `<li data-sym="${t.symbol}"><img src="${esc(t.image)}" loading="lazy" alt=""><span class="sym">${t.symbol}</span><span class="name">${esc(t.name)}</span><span class="pct ${cls(p)}">${fmt.pct(p)}</span></li>`; };
      $("#gainers").innerHTML = sorted.slice(0, 6).map(li).join("");
      $("#losers").innerHTML = sorted.slice(-6).reverse().map(li).join("");
    }

    renderTable(); renderHeatmap(); renderMovers();

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
      if (tr) { location.hash = "#/t/" + tr.dataset.sym; return; }
      const tile = e.target.closest(".tile");
      if (tile) { sel.value = tile.dataset.sector; sector = tile.dataset.sector; limit = 60; renderTable(); window.scrollTo({ top: $("#tbl").offsetTop - 80, behavior: "smooth" }); }
    }, { signal: ac.signal });

    return {
      onTick() { updateTable(); renderHeatmap(); renderMovers(); },
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
        (i ? `<span class="arrow">→</span>` : "") + `<a class="fam ${f.symbol === sym ? "me" : ""}" href="#/t/${f.symbol}"><img src="${esc(f.image)}" alt=""><div><div class="sym">${f.symbol}</div><div class="muted small">${esc(f.name)} <span class="fp"></span></div></div></a>`
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
        <td><img class="sprite" src="${esc(h.image)}" alt=""></td><td class="sym">${h.symbol}</td><td>${esc(h.name)} ${badge(h.sector)}</td>
        <td class="num">${fmt.int(h.qty)}</td><td class="num">${fmt.money(h.avg_cost)}</td><td class="num">${fmt.money(h.price)}</td><td class="num">${fmt.money(h.value)}</td>
        <td class="num ${cls(h.pnl)}">${fmt.money(h.pnl)} (${fmt.pct(h.avg_cost ? (h.price / h.avg_cost - 1) * 100 : 0)})</td>
        <td><button class="sellbtn" data-sell="${h.symbol}" data-qty="${h.qty}">Sell all</button></td></tr>`).join("");
      $("#holdings-empty").style.display = pf.holdings?.length ? "none" : "block";
      $("#trades tbody").innerHTML = (pf.trades || []).map((t) => `<tr data-sym="${t.symbol}"><td class="muted">${fmt.time(t.time)}</td><td class="${t.side === "buy" ? "up" : "down"}">${t.side.toUpperCase()}</td><td class="sym">${t.symbol}</td><td class="num">${fmt.int(t.qty)}</td><td class="num">${fmt.money(t.price)}</td><td class="num">${fmt.money(t.total)}</td></tr>`).join("");
    }
    $("#name-save").addEventListener("click", async () => {
      try { state.portfolio = await post("/api/me", { name: $("#name-input").value }); render(state.portfolio); renderHeader(); toast("Name saved", "ok"); } catch (e) { toast(e.message, "err"); }
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

  views.leaderboard = () => {
    $("#view").appendChild($("#tpl-leaderboard").content.cloneNode(true));
    const load = () => api("/api/leaderboard").then((rows) => {
      $("#lb tbody").innerHTML = rows.map((r, i) => `<tr><td class="muted">${i + 1}</td><td>${esc(r.name)} <span class="muted small">${r.id}</span></td><td class="num">${fmt.money(r.net_worth)}</td><td class="num ${cls(r.pnl_pct)}">${fmt.pct(r.pnl_pct)}</td><td class="num">${r.holdings}</td><td>${r.top ? `<a href="#/t/${r.top}" class="sym">${r.top}</a>` : "—"}</td></tr>`).join("");
    }).catch(() => {});
    load();
    let n = 0;
    return { onTick() { if (++n % 3 === 0) load(); } };
  };

  // ---------- router ----------
  function route() {
    const hash = location.hash || "#/";
    const m = hash.match(/^#\/t\/([A-Za-z0-9]+)/);
    const name = m ? "ticker" : (hash.replace(/^#\//, "") || "market");
    state.view?.destroy?.();
    $("#view").innerHTML = "";
    $$(".nav a").forEach((a) => a.classList.toggle("active", a.dataset.route === name));
    const factory = views[name] || views.market;
    state.view = factory(m ? m[1].toUpperCase() : undefined);
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
