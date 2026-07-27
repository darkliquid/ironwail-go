# Water Translucency Fix Plan — Re-Analysis (2026-07-23)

## Why the "Command Buffer" Conclusion Is Wrong

The previous conclusion blamed Vulkan swapchain discard between separate `queue.Submit()` calls. However, the diagnostic log's own test results contradict this:

- **Test 3**: `alpha * 0.5` → semi-transparent
- **Test 7**: `alpha = 0.1` → barely visible
- **Test 13**: `alpha = 0.6` (green debug) → solid opaque green
- **Test 15**: `alpha = 0.5` → translucent

If the destination framebuffer were truly black (discarded), then `0.6 * green + 0.4 * black` should produce a **brighter, more visible** result than `0.5 * green + 0.5 * black`. The user observed the opposite. This is impossible with standard `SrcAlpha, OneMinusSrcAlpha` blending over a black destination.

**The destination is NOT black — it already contains the water color.** This means the water faces are being drawn to the framebuffer BEFORE the translucent pass, filling it with the water texture color. Then the translucent pass draws the same color on top: `0.6 * waterColor + 0.4 * waterColor = waterColor` (opaque).

At alpha 0.5, `0.5 * waterColor + 0.5 * waterColor = waterColor` — still opaque. But the user said 0.5 was translucent. This suggests the pre-fill is NOT exactly the same color — the opaque pre-fill uses the opaque turbulent pipeline (different shader path or alpha=1.0), while the translucent pass uses the translucent turbulent pipeline with the lit water shader. The color difference at 0.5 is enough to see through; at 0.6 the colors converge and appear opaque.

## C vs Go Architecture Divergence

### C Ironwail (OpenGL)

In C, the **entire frame** renders to a single framebuffer within one `R_RenderView` call. There are no intermediate command buffer submits. The render order is:

1. `R_DrawEntitiesOnList(false)` — opaque world geometry + opaque entities
2. `R_DrawWater(false)` — **opaque water** (blend=OPAQUE, depth write=ON)
3. `R_BeginTranslucency()` — set up translucent mode
4. `R_DrawWater(true)` — **translucent water** (blend=ALPHA, depth write=OFF)
5. `R_DrawEntitiesOnList(true)` — translucent entities
6. `R_EndTranslucency()`

Key: `R_DrawWater(false)` draws water with `alpha < 1` check — it only draws water faces whose alpha is 1.0 (opaque). `R_DrawWater(true)` draws water faces whose alpha is < 1.0 (translucent). **No face is drawn both opaquely and translucently.**

The `GL_SetState` function changes blend mode and depth mask inline between draws, without any command buffer submit. The framebuffer contents from step 1 are preserved through step 4.

### Go Ironwail (WebGPU)

The Go implementation splits rendering into **separate command buffer submits**:

1. `renderWorldInternal` — creates encoder A, render pass A, draws opaque world + opaque liquid, **submits** (`queue.Submit(cmdBuffer)`)
2. `renderOpaqueBrushEntitiesHAL` — creates encoder B, draws opaque brush entities
3. `renderOpaqueLiquidBrushEntitiesHAL` — creates encoder C, draws opaque brush entity liquid (using opaque turbulent pipeline)
4. `renderGoGPUSortedTranslucentFaceRendersHAL` — creates encoder D, render pass D with `LoadOpLoad`, draws translucent water + entities, **submits**
5. `compositeSceneRenderTarget` — creates encoder E, composites to swapchain, **submits**
6. `renderOverlayTextureHAL` — creates encoder F, draws 2D overlay with `LoadOpLoad`, **submits**

Each submit risks Vulkan discarding the previous framebuffer contents.

### Critical Divergence Points

1. **Separate command buffers for world render and translucent render**: In C, both happen in the same framebuffer without submit. In Go, the world render submits, then the translucent render opens a new pass with `LoadOpLoad`. If the render target is the swapchain, Vulkan may discard.

2. **Scene render target workaround**: The Go code uses an offscreen render target (`worldRenderTexture`) to avoid swapchain discard. This works for the world→translucent transition. But the overlay composite still creates a separate submit.

3. **Opaque liquid brush entity rendering**: `renderOpaqueLiquidBrushEntitiesHAL` uses the **opaque** turbulent pipeline (`worldTurbulentPipeline`, blend=One/Zero, depth write=ON) to draw brush entity liquid faces. This function runs AFTER the world render pass. The face classification (`shouldDrawGoGPUOpaqueLiquidBrushFace`) correctly excludes faces with `wateralpha < 1`, so translucent water faces should NOT be drawn here.

4. **The world render pass draws opaque liquid batches**: In `renderWorldInternal`, the `opaqueLiquidBatches` are drawn with the opaque turbulent pipeline. With `wateralpha=0.6`, the face classification puts all water faces in `translucentLiquidFaces` (not `opaqueLiquidDraws`), so `opaqueLiquidBatches` should be empty. The telemetry confirmed `opaque_liquid_batches=0`.

## New Hypothesis: The World Render Pass Clears the Render Target

When the scene render target is enabled, the world render pass draws to `worldRenderTexture` with `LoadOpClear`. The opaque world geometry (walls, floor) is drawn. The translucent water faces are NOT drawn in this pass (classified as translucent). The render pass ends and submits.

Then the translucent pass opens a new render pass on the same `worldRenderTexture` with `LoadOpLoad`. The underwater floor geometry should be visible from the world pass. The translucent water blends over it.

But what if the world render pass draws the water surface **as part of the opaque world** through a different code path? For example, if a water face is adjacent to a solid face, the solid face might be drawn with the water texture if the texture type is misclassified.

**This needs verification**: Add a debug shader output to the opaque world pipeline that highlights water-textured faces. If any pixels in the water area show the highlight, it means the opaque world pass is drawing water-textured geometry.

## Proposed Experiments

### Experiment 1: Verify the destination color before translucent pass

**Goal**: Determine what color the framebuffer contains at water pixel positions just before the translucent water pass begins.

**Method**: Temporarily skip the translucent water pass entirely (return early from `flushPendingTranslucency` when only liquid renders are present). Take a screenshot. If the water area shows the underwater floor, the opaque pass is correct. If it shows water texture color, something in the opaque pass is drawing water. If it shows black, the framebuffer was cleared/discarded.

### Experiment 2: Verify the opaque world pass doesn't draw water-textured faces

**Goal**: Confirm that no face with `*watermurk3` texture is drawn in the opaque world pass.

**Method**: Add a debug check in `renderWorldInternal`'s face classification loop that logs any face with `SurfDrawTurb` that ends up in `opaqueDraws` (instead of `translucentLiquidFaces`). If any are found, the classification has a bug.

### Experiment 3: Merge world render and translucent render into a single command buffer

**Goal**: Eliminate the separate-submit problem by encoding both the world render pass and the translucent render pass in the same command encoder, without submitting between them.

**Method**: Modify `renderWorldInternal` to NOT submit its command buffer. Instead, pass the encoder to the entity/translucent phases. The translucent render pass opens a new render pass on the same texture with `LoadOpLoad`. Only submit once at the end of all rendering.

**Why this matches C**: In C, the opaque and translucent passes share the same framebuffer. In WebGPU, a single command buffer with multiple render passes on the same texture should preserve contents between passes (no submit between them).

**Risk**: The `renderWorldInternal` function currently creates its own encoder and submits. Refactoring to share the encoder across phases is a significant change. But it's the architecturally correct approach and matches C's single-framebuffer model.

### Experiment 4: Use the scene render target for ALL frames (not just translucent water)

**Goal**: Test whether always using the offscreen render target (even for non-water maps) eliminates the issue.

**Method**: Force `shouldUseSceneRenderTarget` to always return true. If the water becomes translucent, the problem is specifically the swapchain being used as the render target for the world pass.

### Experiment 5: Draw translucent water in the world render pass

**Goal**: Instead of deferring translucent water to a separate render pass, draw it within the world render pass after the opaque geometry.

**Method**: After drawing opaque world faces and opaque liquid faces in `renderWorldInternal`, switch to the translucent turbulent pipeline and draw the translucent liquid faces in the same render pass. This eliminates the need for a separate `LoadOpLoad` pass.

**Why this matches C**: In C, `R_DrawWater(true)` is called after `R_DrawWater(false)`, but they're in the same framebuffer. The depth buffer is shared. The blend state changes inline.

**Implementation**: This is the most C-aligned approach. The world render pass would:
1. Clear render target and depth
2. Draw opaque world faces (opaque pipeline, depth write ON)
3. Draw sky faces
4. Draw alpha-test faces
5. Draw opaque liquid faces (if any, opaque turbulent pipeline)
6. **Switch to translucent turbulent pipeline** (depth write OFF, alpha blend)
7. Draw translucent liquid faces
8. End render pass, submit

This eliminates ALL separate command buffer issues for water. The entity translucent faces can remain in a separate pass (they use different geometry/buffers).

### Experiment 6: Check if the `worldRenderTexture` is properly preserved between passes

**Goal**: Verify the offscreen render target contents survive between the world render pass and the translucent render pass.

**Method**: After the world render pass, capture a screenshot from `worldRenderTexture`. Then after the translucent pass, capture another screenshot. Compare the water area — the first should show the underwater floor, the second should show the translucent water blended over it.

## Recommended Approach

**Experiment 5** (draw translucent water in the world render pass) is the most architecturally aligned with C Ironwail and eliminates the root cause. It should be attempted first.

**Experiment 3** (merge command buffers) is the fallback if Experiment 5 is too complex.

**Experiments 1 and 2** are diagnostic and should be done first to confirm the hypothesis.
