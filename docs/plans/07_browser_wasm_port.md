# Implementation Plan: WebAssembly / Browser Port (`GOOS=js GOARCH=wasm`)

**Priority**: #2 (Feature Extension)  
**Status**: Completed (commits f326a7d, 94c61ad, f3e93a8, d2f86ae, ce2e4e4)  
**Target Milestone**: Phase 6  

---

## 1. Executive Summary & Architectural Context

Because `ironwail-go` is built with pure Go (`CGO_ENABLED=0`) and uses WebGPU as its canonical rendering API, the engine is uniquely positioned to run in modern web browsers without CGO cross-compilation toolchains.

Currently, windowing and surface management rely on desktop event loops (`gogpu.App` for X11/Wayland). To run in a browser:
1. Go code must compile to WebAssembly (`GOOS=js GOARCH=wasm`).
2. The GoGPU renderer surface must bind to an HTML5 `<canvas id="canvas">` element using browser native `navigator.gpu`.
3. Input events (`keydown`, `mousemove`, `pointerlock`) must bridge from DOM event listeners to `internal/input.Backend`.
Ironwail Go's pure Go design (`CGO_ENABLED=0`) and canonical GoGPU/WebGPU rendering pipeline make it uniquely suited for native browser execution. This plan establishes WebAssembly target support (`GOOS=js GOARCH=wasm`), enabling Ironwail Go to run directly inside modern web browsers with full WebGPU rendering, Web Audio playback, and DOM input handling.

---

## 2. Existing Code Analysis & Current State

- **Pure Go Target**: `CGO_ENABLED=0` build environment is already enforced in `mise.toml`.
- **WebGPU Shaders**: All shaders are written in WGSL string constants (`internal/renderer/world_shaders_gogpu.go`), which browsers support natively in WebGPU.
- **Desktop Windowing**: Desktop platform loop in `internal/renderer/gogpu/runtime.go` relies on OS window handles.

---

## 3. Step-by-Step Implementation Sequence

### Step 7.1: Create WASM Entry Point (COMPLETED - commit f326a7d)
- **Files**: `cmd/ironwailgo/main_wasm.go`, `cmd/ironwailgo/main.go`, `mise.toml`
- **Actions**:
  - Add `//go:build js && wasm` build tag and `//go:build !js || !wasm` guard to `main.go`.
  - Implemented WASM main entry point and added `mise run build-wasm` task (produces `bin/ironwail.wasm` at 21 MB).


### Step 7.2: Implement HTML5 WebGPU Surface Adapter (COMPLETED - commit ce2e4e4)
- **Files**: `internal/renderer/gogpu/wasm_surface.go`
- **Actions**:
  - Implemented surface adapter calling `js.Global().Get("navigator").Get("gpu")` via `syscall/js`.
  - Configured `<canvas>` WebGPU context (`canvas.getContext("webgpu")`).


### Step 7.3: Implement DOM Input Adapter (COMPLETED - commit f3e93a8)
- **Files**: `internal/input/wasm_input.go`
- **Actions**:
  - Implemented WASM DOM input listener implementation mapping browser keyboard (keydown/keyup), mouse movement deltas, character composition, and mouse buttons to Quake engine key codes.


### Step 7.4: Implement Web Audio API Adapter (COMPLETED - commit d2f86ae)
- **Files**: `internal/audio/wasm_audio.go`
- **Actions**:
  - Implemented WASM Web Audio API backend implementation utilizing `AudioContext` and `ScriptProcessorNode` for real-time PCM audio streaming from DMA buffer to browser audio output.


### Step 7.5: HTTP Asset & Memory PAK VFS (COMPLETED - commit 94c61ad)
- **Files**: `internal/fs/fs.go`, `internal/fs/fs_search.go`, `internal/fs/fs_test.go`
- **Actions**:
  - Abstracted `Pack.Handle` to `ReadSeekerCloserHandle` interface.
  - Implemented `LoadPackFromBytes` and `MountPack` for loading and mounting in-memory PAK archives downloaded over HTTP in WASM.


---

## 4. Edge Cases & C Parity Oracles

- **WASM Memory Limits**: Set initial WebAssembly memory allocation appropriately (e.g. 512 MB) to prevent out-of-memory errors when mounting PAK files.
- **Browser WebGPU Support**: WebGPU requires HTTPS or `localhost` context and Chrome 113+ / Firefox 120+ / Safari 18+.

---

## 5. Testing & Verification Plan

1. **WASM Compilation**:
   ```bash
   GOOS=js GOARCH=wasm go build -o ironwail.wasm ./cmd/ironwailgo
   ```
2. **Local Web Server Test**:
   - Serve `index.html`, `wasm_exec.js`, `ironwail.wasm`, and `id1/pak0.pak` locally.
   - Verify 60 FPS gameplay, pointer lock, and audio streaming in browser dev console.
