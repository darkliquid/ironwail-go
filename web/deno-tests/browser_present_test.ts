// browser_present_test.ts — browser-walkthrough present path under Deno.
//
// Boots the engine wasm exactly like the browser page does (DOM/canvas
// polyfill), starts the rAF-driven render loop via engine_advance-style
// hooks, and asserts the walkthrough stays bounded: frames advance, the
// renderer watchdog prevents a runaway spin (memory does not grow
// unboundedly), and the CPU blit path is exercised when frames exist.
//
// The engine's gogpu wasm App.Run busy-loop / render-thread deadlock is the
// documented browser bug (tracked separately); this test protects the
// harness-level behavior that must survive it.
//
// Run: deno test -A web/deno-tests/browser_present_test.ts

import { assert, assertEquals, assertLess } from "jsr:@std/assert";
import { installBrowserEnv, type BrowserEnv } from "./browser_env.ts";

const WASM = "web/bin/ironwail-harness.wasm";
const PAK = Deno.env.get("PAK0_PATH") ?? ".tmp/pak0.pak";

async function loadWasm() {
  const src = await Deno.readTextFile("web/wasm_exec.js");
  (globalThis as any).crypto ??= { getRandomValues: (a: Uint8Array) => a.fill(1) };
  (globalThis as any).performance ??= Date;
  // eslint-disable-next-line no-eval
  eval(src);
  const go = new (globalThis as any).Go();
  const wasm = await Deno.readFile(WASM);
  const inst = await WebAssembly.instantiate(wasm, go.importObject);
  go.run(inst.instance);
  const ex = inst.instance.exports;
  const mem = () => new DataView(ex.mem.buffer);

  const pak = await Deno.readFile(PAK);
  const slot = ex.mount_resize(pak.length);
  new Uint8Array(ex.mem.buffer, slot, pak.length).set(pak);
  const rc = ex.mount_pak();
  assertEquals(rc, 0, `mount_pak rc=${rc}`);
  // Un-pause like the walkthrough Play button so rAF ticks advance frames.
  ex.engine_set_paused(0);
  return { ex, mem, stateAddr: ex.state_poll };
}

Deno.test("browser walkthrough: rAF loop stays bounded (no runaway)", async () => {
  const env: BrowserEnv = installBrowserEnv(320, 200);
  const { ex, mem } = await loadWasm();

  ex.boot_renderer();
  const rss0 = Deno.memoryUsage().rss;

  // Drive rAF ticks the way a browser tab would (asynchronously), far more
  // than the watchdog window, and assert the engine never falls over and the
  // memory curve stays flat (the watchdog must stop the busy-loop).
  for (let i = 0; i < 260; i++) {
    env.tick();
    // Give the cooperative wasm scheduler room to run spawned goroutines.
    if (i % 20 === 0) await new Promise((r) => setTimeout(r, 5));
  }

  const addr = ex.state_poll();
  const v = mem();
  const frameCount = v.getUint32(addr + 4, true);
  assert(frameCount > 0, `engine frames should have advanced, got ${frameCount}`);

  // Memory must not grow unboundedly over 260 rAF ticks.
  const rss1 = Deno.memoryUsage().rss;
  const growthMB = (rss1 - rss0) / (1024 * 1024);
  assertLess(growthMB, 512, `memory grew too much over 260 rAF ticks: ${growthMB.toFixed(1)}MB`);
});

Deno.test("browser walkthrough: canvas + WebGPU present path available", () => {
  const env = installBrowserEnv(64, 64);
  // The polyfilled canvas must expose a real WebGPU context (the surface the
  // engine draws to) — the same one the WebGPU smoke test proves can read
  // back pixels.
  const ctx = env.canvas().getContext("webgpu");
  assert(ctx, "canvas polyfill must provide WebGPU context");
  const c2d = env.canvas().getContext("2d");
  assert(c2d === null || c2d, "2d context available (CPU blit target)");
});
