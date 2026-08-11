// load wasm, boot engine, then the inspector drives this page.
(async () => {
  if (!window.ironwailInspector) {
    // The engine wasm (built with GOOS=js GOARCH=wasm) installs
    // window.ironwailInspector on boot. In the walkthrough deployment the
    // page is served from web/server.go next to web/bin/ironwail.wasm.
    const status = document.getElementById("boot-status");
    if (status) status.textContent = "waiting for engine wasm…";
  }
})();
