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

const PASS_DEFINITIONS = [
  { id: "sky", label: "Sky" },
  { id: "world", label: "Opaque BSP World" },
  { id: "lightmaps", label: "Lightmaps & Lighting" },
  { id: "brush", label: "Brush Entities (Doors/Plats)" },
  { id: "alias", label: "Alias Models (Monsters/Pickups)" },
  { id: "viewmodel", label: "Viewmodel (Weapon)" },
  { id: "water", label: "Translucent Liquids" },
  { id: "particles", label: "Particles & Trails" },
  { id: "decals", label: "Decal Marks" },
  { id: "overlay", label: "2D Overlays (HUD/Menu)" },
];

let activeLayer = "host";
let paused = false;   // UI mirror of the engine's wasm pause state
let anchors = {};

// Per-frame pass activity on the renderer layer: the engine exposes monotonic
// pass counters; we diff successive snapshots to show what ran since last tick.
let lastPassStats = null;
let passTicker = 0;

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

function renderState(force) {
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
  if (!force && panelBuilt && key === lastPanelJSON) return; // unchanged since last frame

  // Throttle play-time rebuilds to ~8/s so a running sim doesn't churn the
  // DOM; when paused the cache above already stops rebuilds entirely.
  const now = performance.now();
  if (!force && panelBuilt && now - lastPanelRender < 125) return;
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

  // Renderer layer: per-frame pass activity card (diffs the engine's
  // monotonic counters so a paused frame shows zeros; a playing frame shows
  // how many world/overlay/particle passes ran since the last tick).
  if (activeLayer === "renderer" && state.passStats) {
    passTicker++;
    const now = state.passStats;
    let deltas = null;
    if (lastPassStats) {
      deltas = {
        world: now.worldDraws - lastPassStats.worldDraws,
        overlay: now.overlayDraws - lastPassStats.overlayDraws,
        worldUploads: now.worldUploads - lastPassStats.worldUploads,
        scene: now.sceneDraws - lastPassStats.sceneDraws,
        particles: now.particlesDrawn - lastPassStats.particlesDrawn,
        alias: now.aliasDraws - lastPassStats.aliasDraws,
        sprites: now.spriteDraws - lastPassStats.spriteDraws,
        lightmaps: now.lightmapUploads - lastPassStats.lightmapUploads,
      };
    }
    lastPassStats = now;
    if (deltas) {
      const passCard = el("div", "card");
      passCard.appendChild(el("div", "card-title", "Pass activity (since last tick, t=" + passTicker + ")"));
      const passPre = el("pre", "mono");
      passPre.textContent = [
        "world           " + deltas.world,
        "overlay         " + deltas.overlay,
        "scene composite " + deltas.scene,
        "particles       " + deltas.particles,
        "alias models    " + deltas.alias,
        "sprites         " + deltas.sprites,
        "world uploads   " + deltas.worldUploads,
        "lightmap upload " + deltas.lightmaps,
      ].join("\n");
      passCard.appendChild(passPre);
      panel.appendChild(passCard);
    }
  }

  // Renderer layer: Granular Render Pass Toggles
  if (activeLayer === "renderer") {
    const toggleCard = el("div", "card");
    toggleCard.appendChild(el("div", "card-title", "Granular Render Pass Toggles"));

    const btnBar = el("div", "pass-btn-bar");
    const enableAllBtn = el("button", "pass-action-btn", "Enable All");
    enableAllBtn.onclick = () => {
      const allTrue = {};
      for (const p of PASS_DEFINITIONS) allTrue[p.id] = true;
      if (insp.setPassToggles) insp.setPassToggles(allTrue);
      renderState(true);
    };
    const disableAllBtn = el("button", "pass-action-btn", "Disable All");
    disableAllBtn.onclick = () => {
      const allFalse = {};
      for (const p of PASS_DEFINITIONS) allFalse[p.id] = false;
      if (insp.setPassToggles) insp.setPassToggles(allFalse);
      renderState(true);
    };
    btnBar.appendChild(enableAllBtn);
    btnBar.appendChild(disableAllBtn);
    toggleCard.appendChild(btnBar);

    const grid = el("div", "pass-grid");
    const currentToggles = (state.passToggles || (insp.getPassToggles ? insp.getPassToggles() : {})) || {};

    for (const p of PASS_DEFINITIONS) {
      const label = el("label", "pass-toggle-label");
      const cb = document.createElement("input");
      cb.type = "checkbox";
      cb.checked = currentToggles[p.id] !== false; // default true
      cb.onchange = () => {
        if (insp.setPassToggle) {
          insp.setPassToggle(p.id, cb.checked);
        }
      };
      label.appendChild(cb);
      label.appendChild(document.createTextNode(p.label));
      grid.appendChild(label);
    }
    toggleCard.appendChild(grid);
    panel.appendChild(toggleCard);
  }

  if (insp.getGoroutines && (activeLayer === "host" || activeLayer === "boot")) {
    const gr = insp.getGoroutines();
    if (gr && gr.count) {
      const grCard = el("div", "card");
      grCard.appendChild(el("div", "card-title", "Goroutines (" + gr.count + " active)"));
      const grPre = el("pre", "mono");
      grPre.style.maxHeight = "200px";
      grPre.style.overflow = "auto";
      grPre.textContent = gr.stack || "(no stack)";
      grCard.appendChild(grPre);
      panel.appendChild(grCard);
    }
  }

  if (insp.getTelemetryLog && (activeLayer === "host" || activeLayer === "boot")) {
    const tele = insp.getTelemetryLog();
    if (tele && tele.length) {
      const teleCard = el("div", "card");
      teleCard.appendChild(el("div", "card-title", "Engine Telemetry Log (" + tele.length + " events)"));
      const telePre = el("pre", "mono");
      telePre.style.maxHeight = "150px";
      telePre.style.overflow = "auto";
      telePre.textContent = tele.map(e => "+" + e.timeMs + "ms [" + e.phase + "] " + e.message).join("\n");
      teleCard.appendChild(telePre);
      panel.appendChild(teleCard);
    }
  }
}

function renderTimeline() {
  const tl = document.getElementById("timeline");
  const insp = window.ironwailInspector;
  if (!insp || !insp.getTimeline) return;
  const t = insp.getTimeline();
  if (!t) return;
  const text = "frame " + t.frameCount + " · srvTime " + (t.srvTime !== undefined ? t.srvTime.toFixed(2) : "?") + " · dt " + (t.frameTimeMs !== undefined ? t.frameTimeMs.toFixed(1) + "ms" : "?") + (paused ? " · PAUSED" : "");
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
  if (window.__ironwailRecordFrameTick) window.__ironwailRecordFrameTick();
  syncPause();
  renderState();
  renderTimeline();
  renderEdicts();
  requestAnimationFrame(tick);
}

function setupCanvasInput() {
  const canvas = document.getElementById("canvas");
  if (!canvas) return;
  canvas.addEventListener("click", () => {
    if (document.pointerLockElement !== canvas) {
      canvas.requestPointerLock();
    }
    const resume = window.__ironwailAudioResume;
    if (resume) resume();
  });
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
  setupCanvasInput();
  renderState();
  // Poll the inspector after the wasm boot installs it.
  setTimeout(tick, 500);
});
