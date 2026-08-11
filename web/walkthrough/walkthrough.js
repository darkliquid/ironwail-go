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
let paused = false;
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

function renderState() {
  const panel = document.getElementById("panel");
  const insp = window.ironwailInspector;
  if (!insp) {
    panel.innerHTML = "<p>Inspector not available. Load the wasm boot first.</p>";
    return;
  }
  const state = insp.getState(activeLayer);

  const title = el("h2", "", LAYER_TITLES[activeLayer]);
  panel.innerHTML = "";
  panel.appendChild(title);

  // Source anchor card.
  const a = insp.getSourceAnchor(activeLayer);
  const card = el("div", "card");
  card.appendChild(el("div", "card-title", "Source anchor"));
  card.appendChild(el("div", "mono", a.file + (a.line ? ":" + a.line : "")));
  if (a.doc) card.appendChild(el("div", "dim", "doc: " + a.doc));
  if (a.error) card.appendChild(el("div", "err", a.error));
  panel.appendChild(card);

  // Layer snapshot.
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
  if (!insp) return;
  const t = insp.getTimeline();
  tl.textContent = "frame " + t.frameCount + " · srvTime " + (t.srvTime !== undefined ? t.srvTime.toFixed(2) : "?") + " · dt " + (t.frameTimeMs !== undefined ? t.frameTimeMs.toFixed(1) + "ms" : "?");
}

function renderEdicts() {
  const box = document.getElementById("edicts");
  const insp = window.ironwailInspector;
  if (!insp) return;
  const state = insp.getState("server");
  if (!state.edicts) { box.textContent = "(no server)"; return; }
  box.textContent = state.edicts.slice(0, 12).map(e =>
    "#" + e.num + " " + (e.classname || "?") + " @(" + (e.origin ? e.origin.map(v => v.toFixed(0)).join(",") : "?") + ")"
  ).join("\n");
}

function tick() {
  if (!paused) {
    renderState();
    renderTimeline();
    renderEdicts();
  }
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
  renderState();
  // Poll the inspector after the wasm boot installs it.
  setTimeout(tick, 500);
});
