// harness.ts — Deno-side driver for the pollable Ironwail-Go harness wasm
// (built by `mise run build-wasm-harness` into web/bin/ironwail-harness.wasm).
//
// Shared by the Deno test suite (web/deno-tests/*.test.ts). Owns:
//   - loading Go's wasm_exec.js glue + the harness module
//   - mounting a Quake data pak (pak0.pak) synchronously into Go memory
//   - typed reads of the shared state struct + pixel arena
//   - input injection + frame stepping
//
// The Go wasm event loop parks in main()'s select{}; every exported call from
// JS re-enters it, so all engine work is synchronous from the test's POV.

export interface EngineState {
  flags: number;
  frameCount: number;
  cameraOrigin: [number, number, number];
  cameraAngles: [number, number, number];
  mapEntities: number;
  pixelWidth: number;
  pixelHeight: number;
  pixelStride: number;
  pixelValid: number;
  pixelCount: number;
  pixelOffset: number;
  errorCode: number;
  inputApplied: number;
  lastDtUs: number;
  frameFlags: number;
}

// Minimal structural type for the Go/wasm export surface we use. The strict
// WebAssembly.Exports union makes .mem/calls awkward, so we type the exact
// members we touch.
type HarnessExports = {
  state_poll(): number;
  input_inject(addr: number): void;
  input_slot(): number;
  engine_advance(dtNS: bigint): number;
  pixels_capture(): number;
  debug_state(): number;
  mount_slot(): number;
  mount_resize(needed: number): number;
  mount_pak(): number;
  mem: { buffer: ArrayBuffer };
};

export interface IronwailHarness {
  exports: HarnessExports;
  memory: () => DataView;
  /** Read the shared state struct. */
  state(): EngineState;
  /** Debug: dump raw input/view diagnostics (harness builds with debug_state). */
  debugState(): { stateMouseDX: number; viewAnglesYaw100: number; keyGame: boolean; clientActive: boolean };
  /** Inject an input frame: [fwd, strafe, yaw, pitch, btn0..3]. */
  injectInput(x: Float32Array | number[]): void;
  /** Run n frames at fixed dt. Returns state after the last. */
  advanceFrames(n: number, dtNS?: bigint): EngineState;
  /** Best-effort pixel readback; returns true when fresh pixels captured. */
  capturePixels(): boolean;
}

const STATE_SIZE = 88;
const UTF8_DECODER = new TextDecoder("utf-8");

/** Minimal Node-like globals the Go loader expects. The loader's own fs
 *  shim (buffered stdout via console.log) is left untouched — overriding it
 *  breaks syscall.init which reads fs.constants at startup (see boot9
 *  experiment: stock fs boots clean, custom fs corrupts the export return). */
function installGoGlobals(): void {
  (globalThis as any).crypto ??= { getRandomValues: (a: Uint8Array) => a.fill(1) };
  (globalThis as any).performance ??= Date;
}

/** Loads the harness wasm + Go glue; returns a driver. The pak path is
 *  optional — without one the engine still boots to the menu. */
export async function loadHarness(
  wasmPath = "web/bin/ironwail-harness.wasm",
  pakPath?: string,
  wasmExecPath = "web/wasm_exec.js"
): Promise<IronwailHarness> {
  installGoGlobals();
  const src = await Deno.readTextFile(wasmExecPath);
  // eslint-disable-next-line no-eval
  eval(src); // defines globalThis.Go (browser glue; no other requires)
  const go = new (globalThis as any).Go();
  const wasm = await Deno.readFile(wasmPath);
  const inst = await WebAssembly.instantiate(wasm, go.importObject);
  go.run(inst.instance); // returns once main() reaches select{}

  const exports = inst.instance.exports as unknown as HarnessExports;
  const memory = () => new DataView(exports.mem.buffer);

  if (pakPath) {
    const pak = await Deno.readFile(pakPath);
    const slot = exports.mount_resize(pak.length);
    new Uint8Array(exports.mem.buffer, slot, pak.length).set(pak);
    const rc = exports.mount_pak();
    if (rc !== 0) throw new Error(`mount_pak failed rc=${rc}`);
  }

  const readState = (): EngineState => {
    const v = memory();
    const addr = exports.state_poll();
    const f32 = (o: number) => v.getFloat32(addr + o, true);
    const u32 = (o: number) => v.getUint32(addr + o, true);
    return {
      flags: u32(0),
      frameCount: u32(4),
      cameraOrigin: [f32(8), f32(12), f32(16)],
      cameraAngles: [f32(20), f32(24), f32(28)],
      mapEntities: u32(44),
      pixelWidth: u32(48),
      pixelHeight: u32(52),
      pixelStride: u32(56),
      pixelValid: u32(60),
      pixelCount: u32(64),
      pixelOffset: u32(68),
      errorCode: u32(72),
      inputApplied: u32(76),
      lastDtUs: u32(80),
      frameFlags: u32(84),
    };
  };

  return {
    exports,
    memory,
    state: readState,
    /** Inject an input frame: [fwd, strafe, yaw, pitch, btn0..3]. */
    injectInput(x: Float32Array | number[]) {
      const slot = exports.input_slot();
      // 8 x f32 little-endian
      const dv = new DataView(exports.mem.buffer);
      const f = Array.from(x as number[]);
      for (let i = 0; i < 8; i++) dv.setFloat32(slot + i * 4, f[i] ?? 0, true);
      exports.input_inject(slot);
    },
    advanceFrames(n: number, dtNS: bigint = 16_666_666n): EngineState {
      for (let i = 0; i < n; i++) exports.engine_advance(dtNS);
      return readState();
    },
    capturePixels(): boolean {
      return exports.pixels_capture() === 1;
    },
    debugState() {
      const addr = exports.debug_state();
      const v = memory();
      return {
        stateMouseDX: v.getInt32(addr + 104, true),
        viewAnglesYaw100: v.getInt32(addr + 92, true),
        keyGame: (v.getUint32(addr + 88, true) & 1) !== 0,
        clientActive: (v.getUint32(addr + 88, true) & 4) !== 0,
      };
    },
  };
}

export const FLAG = {
  RUNNING: 1 << 0,
  MAP: 1 << 1,
  PAUSED: 1 << 2,
} as const;
