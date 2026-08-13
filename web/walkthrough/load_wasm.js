// load_wasm.js — boots the engine wasm and installs window.ironwailInspector.
// Requires Go's wasm_exec.js loader (from $GOROOT/lib/wasm/wasm_exec.js),
// served next to web/bin/ironwail.wasm by web/server.go.
"use strict";

async function bootWasm() {
  if (!window.ironwailInspector) {
    // Load Go's wasm glue, then instantiate the engine binary. Both live one
    // directory up (web/wasm_exec.js, web/bin/ironwail.wasm).
    if (!window.Go) {
      await loadScript("../wasm_exec.js");
    }
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(
      fetch("../bin/ironwail.wasm"),
      go.importObject
    );
    go.run(result.instance);
    const status = document.getElementById("boot-status");
    if (status) status.textContent = "engine wasm booted — inspect each layer";
  }
}

function loadScript(src) {
  return new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = src;
    s.onload = resolve;
    s.onerror = () => reject(new Error("failed to load " + src));
    document.head.appendChild(s);
  });
}

let lastFrameTime = Date.now();
let watchdogTriggered = false;

window.__ironwailRecordFrameTick = function() {
  lastFrameTime = Date.now();
};

function startDeadlockWatchdog() {
  setInterval(() => {
    if (watchdogTriggered) return;
    const insp = window.ironwailInspector;
    if (!insp || !insp.getPaused) return;

    // Only watch when unpaused (playing)
    const isPaused = insp.getPaused();
    if (isPaused) {
      lastFrameTime = Date.now();
      return;
    }

    const elapsed = Date.now() - lastFrameTime;
    if (elapsed > 1500) { // > 1.5s without a frame tick while playing
      watchdogTriggered = true;
      console.error("🚨 [IRONWAIL WASM DEADLOCK / HANG DETECTED] No frame tick for " + (elapsed / 1000).toFixed(1) + "s");

      let gr = null;
      let tele = null;
      let gpu = null;
      try {
        if (insp.getGoroutines) gr = insp.getGoroutines();
        if (insp.getTelemetryLog) tele = insp.getTelemetryLog();
        if (insp.getGpuStatus) gpu = insp.getGpuStatus();
      } catch (e) {
        console.error("Failed to query WASM state during watchdog:", e);
      }

      console.error("Active Goroutines:", gr);
      console.error("Telemetry Log:", tele);
      console.error("GPU Status:", gpu);

      const status = document.getElementById("boot-status");
      if (status) {
        status.style.color = "#ff4444";
        status.style.fontWeight = "bold";
        status.textContent = "⚠️ DEADLOCK / HANG DETECTED (" + (elapsed / 1000).toFixed(1) + "s stall) — see console for goroutine stack traces";
      }
    }
  }, 1000);
}

window.addEventListener("load", () => {
  bootWasm().then(() => {
    startDeadlockWatchdog();
  }).catch((err) => {
    const status = document.getElementById("boot-status");
    if (status) status.textContent = "boot failed: " + err.message;
    console.error("wasm boot error:", err);
  });
});

