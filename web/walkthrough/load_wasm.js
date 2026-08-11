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

window.addEventListener("load", () => {
  bootWasm().catch((err) => {
    const status = document.getElementById("boot-status");
    if (status) status.textContent = "boot failed: " + err.message;
    console.error("wasm boot error:", err);
  });
});
