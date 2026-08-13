// webgpu_smoke_test.ts — engine-independent Deno WebGPU smoke test.
//
// Proves the host platform (Deno + wgpu) can create a WebGPU device, draw,
// and READ BACK pixels synchronously via a staging buffer. This is the
// foundation for the engine harness's pixel probe: if this fails, no engine
// render test can pass, and the failure is environmental, not the engine.
//
// Run: deno test -A web/deno-tests/webgpu_smoke_test.ts

import { assert, assertEquals, assertExists } from "jsr:@std/assert";

Deno.test("Deno WebGPU: device + canvas + draw + readback", async () => {
  const adapter = await navigator.gpu.requestAdapter();
  assertExists(adapter, "No WebGPU adapter found (host lacks GPU?)");
  const device = await adapter.requestDevice();
  assertExists(device);

  // OffscreenCanvas is Deno's canvas-like; getContext('webgpu') is real.
  const canvas = new OffscreenCanvas(64, 64);
  const ctx = canvas.getContext("webgpu");
  assertExists(ctx, "OffscreenCanvas WebGPU context");

  const format = navigator.gpu.getPreferredCanvasFormat();
  ctx.configure({ device, format, alphaMode: "premultiplied" });

  // Minimal full-canvas triangle cleared to a known color via a render pass
  // with a clear color, then read the texture back.
  const texture = ctx.getCurrentTexture();
  const view = texture.createView();

  const encoder = device.createCommandEncoder();
  const pass = encoder.beginRenderPass({
    colorAttachments: [{
      view,
      loadOp: "clear",
      storeOp: "store",
      clearValue: { r: 0.2, g: 0.4, b: 0.6, a: 1.0 },
    }],
  });
  pass.end();
  device.queue.submit([encoder.finish()]);

  // Read back via CopyTextureToBuffer + mapAsync.
  const bytesPerRow = 256; // 64px * 4 = 256, already aligned
  const staging = device.createBuffer({
    size: bytesPerRow * 64,
    usage: GPUBufferUsage.COPY_DST | GPUBufferUsage.MAP_READ,
  });
  const copyEncoder = device.createCommandEncoder();
  copyEncoder.copyTextureToBuffer(
    { texture },
    { buffer: staging, bytesPerRow: 256, rowsPerImage: 64 },
    { width: 64, height: 64, depthOrArrayLayers: 1 }
  );
  device.queue.submit([copyEncoder.finish()]);

  await staging.mapAsync(GPUMapMode.READ);
  const data = new Uint8Array(staging.getMappedRange());
  // The canvas format may be bgra8unorm (Deno default on Linux) or rgba8unorm
  // elsewhere — decode by preferred format so the assertion is portable.
  const isBGRA = format === "bgra8unorm" || format === "bgra8unorm-srgb";
  const R = 51, G = 102, B = 153; // clearValue r=0.2 g=0.4 b=0.6 on 8-bit
  const [b0, b1, b2] = isBGRA ? [B, G, R] : [R, G, B];
  assertEquals(data[0], b0, `byte0 expected ${b0} got ${data[0]}`);
  assertEquals(data[1], b1, `byte1 expected ${b1} got ${data[1]}`);
  assertEquals(data[2], b2, `byte2 expected ${b2} got ${data[2]}`);
  // A corner pixel too, proving the whole texture was written (A is 255 either way).
  const lastIdx = (64 - 1) * bytesPerRow + 3;
  assertEquals(data[lastIdx], 255, `A expected 255 got ${data[lastIdx]}`);
  staging.unmap();

  assert(true, "WebGPU draw + readback works on this host");
  device.destroy();
});
