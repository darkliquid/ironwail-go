// walkthrough_test.ts — Deno end-to-end tests for the wasm walkthrough
// engine build (cmd/ironwailgo-harness).
//
// Proves the engine boots under Deno, loads a real map from a data pak,
// advances the host frame loop, responds to injected input, and reports
// renderer status (with graceful headless degradation — real pixel capture
// requires a WebGPU surface; the separate webgpu_smoke_test covers the
// platform's WebGPU pixel round-trip).
//
// A single harness instance is shared across tests (Deno wasm_exec only
// supports one Go instance per process comfortably) — tests advance it
// deterministically rather than re-booting.
//
// Requires:
//   - `mise run build-wasm-harness` (produces web/bin/ironwail-harness.wasm)
//   - a Quake data pak (shareware id1) at .tmp/pak0.pak or $PAK0_PATH
//
// Run: deno test -A web/deno-tests/walkthrough_test.ts

import { assertEquals, assert, assertGreater } from "jsr:@std/assert";
import { loadHarness, type IronwailHarness, FLAG } from "./harness.ts";

const WASM = "web/bin/ironwail-harness.wasm";
const PAK = Deno.env.get("PAK0_PATH") ?? ".tmp/pak0.pak";

function pakExists(): boolean {
  try {
    Deno.statSync(PAK);
    return true;
  } catch {
    return false;
  }
}

const hasPak = pakExists();
if (!hasPak) {
  console.warn(`No pak found at ${PAK} — map/input/pixel tests will skip. Set PAK0_PATH to a pak for full coverage.`);
}

// Singleton harness — one wasm instance per process.
let harness: IronwailHarness | undefined;
async function getHarness(): Promise<IronwailHarness> {
  if (!harness) {
    harness = await loadHarness(WASM, hasPak ? PAK : undefined);
  }
  return harness;
}

Deno.test("harness boots and reaches running state", async () => {
  const h = await getHarness();
  const s = h.state();
  assert((s.flags & FLAG.RUNNING) !== 0, "engine should report running");
});

Deno.test("mount + advance: map loads, frames advance", async () => {
  const h = await getHarness();
  if (!hasPak) return;
  const s0 = h.state();
  assert((s0.flags & FLAG.MAP) !== 0, "start map should be active");
  assert(s0.mapEntities > 0, `map entities > 0, got ${s0.mapEntities}`);

  const s1 = h.advanceFrames(5);
  assert(s1.frameCount > s0.frameCount, "frameCount should advance");
  assert(s1.frameCount >= s0.frameCount + 5, `frameCount ${s1.frameCount} >= ${s0.frameCount}+5`);
});

Deno.test("input injection reaches the engine's mouse state", async () => {
  const h = await getHarness();
  if (!hasPak) return;
  h.advanceFrames(3); // settle

  h.injectInput([0, 0, 40, 0, 0, 0, 0, 0]); // yaw delta +40
  h.advanceFrames(1);
  const after = h.state();
  assertEquals(after.inputApplied, 0, "input should be consumed by advance");
  // The harness's stub backend feeds the input System; the frame's State()
  // must have seen the delta (raw cl.ViewAngles may not move because the
  // harness client has no local usercmd producer — the engine's camera is
  // server-driven here. The InputState still proves the walkthrough input
  // plumbing works end to end.)
  const dbg = h.debugState();
  assert(dbg.stateMouseDX === 40, `state.MouseDX should be 40, got ${dbg.stateMouseDX}`);
});

Deno.test("renderer reports status; pixels degrade gracefully headless", async () => {
  const h = await getHarness();
  h.advanceFrames(3);
  const captured = h.capturePixels();
  assert(typeof captured === "boolean", "capturePixels returns a boolean");
  // Without a WebGPU surface the readback is unavailable — the engine
  // must stay healthy.
  const s = h.state();
  assert(s.frameCount > 0, "engine still advancing");
});

Deno.test("engine_advance dt affects frame timing", async () => {
  const h = await getHarness();
  h.advanceFrames(1, 33_333_333n);
  const s = h.state();
  assertGreater(s.lastDtUs, 30000, "33ms dt should report ~33,333us");
});
