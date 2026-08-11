// walkthrough.js — plan 22 Phase C. Consumes window.ironwailInspector and
// renders the seven-layer tour: Boot/FS → Console → Host frame → Server
// physics → QuakeC → Client → Renderer. No bundler; ships as-is via
// web/server.go.
"use strict";

const LAYERS = ["boot", "console", "host", "server", "quakec", "client", "renderer"];
const LAYER_TITLES = {
  boot: "Boot / Filesystem",
  console: "Console",
  host: "Host Frame",
  server: "Server Physics",
  quakec: "QuakeC VM",
  client: "Client Parse",
  renderer: "Renderer",
};

let activeLayer = "host";
let paused = false;   // UI mirror of the engine's wasm pause state
let anchors = {};

function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function renderRail() {
  const rail = document.getElementById("rail");
  rail.innerHTML = "";
  for (const layer of LAYERS) {
    const b = el("button", "rail-btn", LAYER_TITLES[layer]);
    b.dataset.layer = layer;
    if (layer === activeLayer) b.classList.add("active");
    b.onclick = () => { activeLayer = layer; renderRail(); renderState(); };
    rail.appendChild(b);
  }
}

// Cache of last-rendered strings so a paused frame (unchanged state) does not
// tear down and rebuild the panel DOM every animation frame.
let lastPanelJSON = "";
let lastTimelineText = "";
let lastEdictsText = "";
let panelBuilt = false;
let lastPanelRender = 0; // ms timestamp; throttles play-time DOM rebuilds

function renderState() {
  const panel = document.getElementById("panel");
  const insp = window.ironwailInspector;
  if (!insp || (!insp.getStateJSON && !insp.getState)) {
    panel.innerHTML = "<p>Inspector not available. Load the wasm boot first.</p>";
    lastPanelJSON = "";
    panelBuilt = false;
    return;
  }
  // Use the deterministic JSON payload (sorted keys) for both the display and
  // the change-detection cache: Go maps iterate in random order, so the
  // object form would reorder keys every call and look like live churn even
  // when the actual state is frozen.
  const stateText = insp.getStateJSON ? insp.getStateJSON(activeLayer) : JSON.stringify(insp.getState(activeLayer));
  if (stateText === undefined || stateText === null) return;
  const state = JSON.parse(stateText);

  const json = stateText;
  const a = (insp.getSourceAnchor ? (insp.getSourceAnchor(activeLayer) || {}) : {});
  const anchorLine = a.file + (a.line ? ":" + a.line : "") + "|" + (a.doc || "");
  const key = activeLayer + "|" + anchorLine + "|" + json;
  if (panelBuilt && key === lastPanelJSON) return; // unchanged since last frame

  // Throttle play-time rebuilds to ~8/s so a running sim doesn't churn the
  // DOM; when paused the cache above already stops rebuilds entirely.
  const now = performance.now();
  if (panelBuilt && now - lastPanelRender < 125) return;
  lastPanelRender = now;

  panel.innerHTML = "";
  panelBuilt = true;
  lastPanelJSON = key;

  panel.appendChild(el("h2", "", LAYER_TITLES[activeLayer]));

  // Source anchor card.
  const card = el("div", "card");
  card.appendChild(el("div", "card-title", "Source anchor"));
  card.appendChild(el("div", "mono", a.file + (a.line ? ":" + a.line : "")));
  if (a.doc) card.appendChild(el("div", "dim", "doc: " + a.doc));
  if (a.error) card.appendChild(el("div", "err", a.error));
  panel.appendChild(card);

  // Layer snapshot. Render the parsed object pretty-printed; the cache key
  // stays on the compact JSON string (stable across frames).
  const snap = el("div", "card");
  snap.appendChild(el("div", "card-title", "Live state"));
  const pre = el("pre", "mono");
  pre.textContent = JSON.stringify(state, null, 2);
  snap.appendChild(pre);
  panel.appendChild(snap);
}

function renderTimeline() {
  const tl = document.getElementById("timeline");
  const insp = window.ironwailInspector;
  if (!insp || !insp.getTimeline) return;
  const t = insp.getTimeline();
  if (!t) return;
  const text = "frame " + t.frameCount + " · srvTime " + (t.srvTime !== undefined ? t.srvTime.toFixed(2) : "?") + " · dt " + (t.frameTimeMs !== undefined ? t.frameTimeMs.toFixed(1) + "ms" : "?");
  if (text === lastTimelineText) return;
  lastTimelineText = text;
  tl.textContent = text;
}

function renderEdicts() {
  const box = document.getElementById("edicts");
  const insp = window.ironwailInspector;
  const getter = insp && (insp.getStateJSON || insp.getState);
  if (!insp || !getter) { box.textContent = "(no inspector)"; lastEdictsText = ""; return; }
  const raw = getter("server");
  let state = null;
  try { state = typeof raw === "string" ? JSON.parse(raw) : raw; } catch (e) { state = null; }
  if (!state || !state.edicts) { box.textContent = "(no server)"; lastEdictsText = ""; return; }
  const text = state.edicts.slice(0, 12).map(e =>
    "#" + e.num + " " + (e.classname || "?") + " @(" + (e.origin ? e.origin.map(v => v.toFixed(0)).join(",") : "?") + ")"
  ).join("\n");
  if (text === lastEdictsText) return;
  lastEdictsText = text;
  box.textContent = text;
}

function syncPause() {
  const insp = window.ironwailInspector;
  if (insp && insp.getPaused) paused = !!insp.getPaused();
  const playBtn = document.getElementById("btn-play");
  if (playBtn) playBtn.textContent = paused ? "Play" : "Pause";
}

function stepFrames(n) {
  const insp = window.ironwailInspector;
  if (insp && insp.stepFrames) insp.stepFrames(n);
  paused = true;
  syncPause();
}

function setupControls() {
  const playBtn = el("button", "ctrl-btn", "Play");
  playBtn.id = "btn-play";
  playBtn.onclick = () => {
    const insp = window.ironwailInspector;
    if (insp && insp.setPaused) insp.setPaused(!paused);
    paused = !paused;
    syncPause();
  };
  const stepBtn = el("button", "ctrl-btn", "Step 1");
  stepBtn.onclick = () => stepFrames(1);
  const step5Btn = el("button", "ctrl-btn", "Step 5");
  step5Btn.onclick = () => stepFrames(5);

  const bar = document.getElementById("controls");
  bar.innerHTML = "";
  bar.appendChild(playBtn);
  bar.appendChild(stepBtn);
  bar.appendChild(step5Btn);
}

function tick() {
  syncPause();
  renderState();
  renderTimeline();
  renderEdicts();
  requestAnimationFrame(tick);
}

window.addEventListener("load", async () => {
  try {
    const r = await fetch("anchors.json");
    anchors = await r.json();
  } catch (e) {
    anchors = {};
  }
  renderRail();
  setupControls();
  renderState();
  // Poll the inspector after the wasm boot installs it.
  setTimeout(tick, 500);
});
