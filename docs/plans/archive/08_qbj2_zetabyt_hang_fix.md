# Implementation Plan: Fix `qbj2_zetabyt` Map Load Hang

**Priority**: P0 (blocking — map completely hangs the engine on load)
**Status**: Completed (Fixed in commits `f865138` and `ea76301`)
**Target Milestone**: Pre-Phase 5

---

## 1. Executive Summary & Symptom

`qbj2_zetabyt` (from the Quake Brutalist Jam 2 map pack) causes the engine
to completely lock up during map load. Logging stops after the line:

```
external skybox async load complete, ready for GPU upload
```

No further log lines appear. The window either never spawns or freezes.
Standard maps (`start`, `e1m1`, `qbj2_start`, `qbj3_stickflip`) load and
render fine. Five prior fix attempts have either failed to resolve the hang
or introduced severe visual regressions (see
`docs/qbj2_zetabyt_investigation_log.md`).

---

## 2. Root Cause Analysis

After tracing the full code path from skybox load through GPU upload and
frame render, three distinct failure mechanisms have been identified. They
are listed in order of likelihood, but all three likely contribute to the
hang on `qbj2_zetabyt`.

### 2.1: Cross-Thread `r.mu` Deadlock (Primary Suspect)

**The core problem**: `UploadPendingExternalSkybox()` acquires `r.mu.Lock()`
(a **write** lock) and holds it across `queue.WriteTexture()` — a
synchronous GPU operation that can block on Vulkan/Wayland.

**Thread model** (from `renderer_gogpu.go:92`):
- `OnUpdate` runs on gogpu's **main-thread event loop**
- `OnDraw` runs on gogpu's **dedicated locked render thread**

**Call path on the `OnUpdate` thread** (`runtime_frame.go:95-135`):

```
OnUpdate callback
  → g.RunRuntimeFrame(dt, cb)           // game logic, server frame
  → g.syncRuntimeVisualEffects(...)      // particle/decal updates
  → state.storePendingRendererFrame(...)
```

**Call path on the `OnDraw` thread** (`runtime_frame.go:137-150`):

```
OnDraw callback
  → g.applyRuntimeRendererState(state)   // line 147
    → g.applyRuntimeRendererSkybox(g.Renderer)  // line 173
      → assets.SetExternalSkybox(...)     // spawns async goroutine
      → assets.UploadPendingExternalSkybox()    // line 225 — HOLDS r.mu.Lock()
    → g.uploadDeferredRuntimeWorld()      // line 148
      → g.Renderer.UploadWorld(tree)      // also acquires r.mu.Lock()
  → g.drawRuntimeRendererFrame(dc)       // line 149
    → drawCtx.RenderFrame(state, ...)    // renderer_gogpu_frame.go:82
      → dc.renderWorld(state)            // acquires r.mu.RLock()
```

The `OnDraw` callback does **three things sequentially that all need `r.mu`**:

1. `applyRuntimeRendererSkybox` → `UploadPendingExternalSkybox()` → `r.mu.Lock()`
   held across `queue.WriteTexture()` for each skybox face (6 faces, each a
   separate `CreateTexture` + `WriteTexture` + `CreateTextureView`).
2. `uploadDeferredRuntimeWorld` → `UploadWorld()` → `r.mu.Lock()` held across
   the **entire world upload** (vertex buffers, index buffers, lightmap
   arrays, texture atlas, materials buffer, pipeline creation).
3. `drawRuntimeRendererFrame` → `RenderFrame` → `renderWorldInternal` →
   `r.mu.RLock()`.

**The deadlock**: `UploadPendingExternalSkybox` only uploads **one face per
call** (incrementing `uploadCursor`). It must be called 7 times (6 faces +
bind group creation) to complete. But `SetExternalSkybox` is called
**every frame** in `applyRuntimeRendererSkybox` (line 224). On each call,
`SetExternalSkybox` checks if the name matches; if it does, it returns
early. So the upload progresses one face per frame.

The problem: `UploadPendingExternalSkybox` holds `r.mu.Lock()` during
`queue.WriteTexture`. On Vulkan/Wayland, `queue.WriteTexture` can stall if
the GPU queue is busy (e.g., the previous frame's `queue.Submit` hasn't
completed, or a semaphore wait is pending). Since `OnDraw` is the render
thread, and the render thread is the one that submits command buffers, the
queue may be in a state where `WriteTexture` blocks waiting for a prior
`Submit` to drain — but that `Submit` was the last frame's render, which
already completed... unless `qbj2_zetabyt` is so large that the first world
upload + render pass hasn't finished draining when the skybox upload starts.

For `qbj2_zetabyt` specifically, the world geometry is large enough that
`UploadWorld` runs on the **same `OnDraw` call** (line 148), immediately
after the skybox upload attempt. If `UploadWorld` triggers heavy GPU
allocations (textures, lightmaps, buffers) that saturate Vulkan memory or
stall the queue, then the subsequent frame's `UploadPendingExternalSkybox`
will block on `r.mu.Lock()` while `UploadWorld` still holds it... except
they're sequential, not concurrent.

**The actual deadlock is more subtle**: `SetExternalSkybox` spawns an async
goroutine (`loadExternalSkyboxAsync`). That goroutine acquires `r.mu.Lock()`
at line 197 to store the loaded faces. If the goroutine completes and tries
to acquire `r.mu.Lock()` while `UploadPendingExternalSkybox` already holds
`r.mu.Lock()`, it blocks. But since Go's `sync.RWMutex` is not reentrant,
this is fine — the goroutine just waits.

**But**: the async goroutine is spawned from `SetExternalSkybox` which is
called on the `OnDraw` thread. The goroutine runs independently. If the
goroutine is in the middle of `loadExternalSkyboxFaces` (doing file I/O and
image decoding) and then tries to acquire `r.mu.Lock()` at line 197, while
simultaneously the `OnDraw` thread calls `UploadPendingExternalSkybox`
which acquires `r.mu.Lock()` at line 217 — there's a race, but not a
deadlock (one waits for the other).

**The real hang**: The last log line is "external skybox async load complete,
ready for GPU upload" (line 213). This is logged from inside the async
goroutine while holding `r.mu.Lock()`. After this, `UploadPendingExternalSkybox`
should be called on the next `OnDraw` to upload faces. But if it's never
called, or if it returns immediately without logging, the hang is that the
`OnDraw` callback itself is never invoked again.

**Why `OnDraw` stops being called**: The gogpu app event loop (`r.app.Run()`)
drives both `OnUpdate` and `OnDraw`. If `OnDraw` blocks (e.g., on
`UploadWorld` which holds `r.mu.Lock()` across heavy GPU operations), the
entire event loop stalls. `qbj2_zetabyt` likely has enough geometry/textures
that `UploadWorld` takes a very long time, and during that time the window
appears frozen. If `UploadWorld` itself hangs (e.g., `queue.WriteBuffer` for
a multi-MB lightmap array blocks on Vulkan), the event loop never recovers.

### 2.2: Missing `worldSkyExternalBindGroupLayout` During Upload

`UploadPendingExternalSkybox` (line 219) checks:
```go
if r.worldSkyExternalMode != externalSkyboxRenderFaces ||
   r.worldSkyExternalLoaded == 0 ||
   r.worldSkyExternalBindGroup != nil {
    return nil
}
```

Then `uploadNextGoGPUExternalSkyboxFaceLocked` (line 114) checks:
```go
if device == nil || queue == nil ||
   r.worldLightmapSampler == nil ||
   r.worldSkyExternalBindGroupLayout == nil {
    return fmt.Errorf("external sky resources not ready")
}
```

`worldSkyExternalBindGroupLayout` is created during `UploadWorld` (line 554).
If `UploadWorld` hasn't run yet (or failed), `worldSkyExternalBindGroupLayout`
is `nil`, and every call to `UploadPendingExternalSkybox` returns
`"external sky resources not ready"`. The skybox never uploads. This alone
isn't a hang, but it means the skybox stays in "pending upload" forever.

The ordering in `applyRuntimeRendererState` (line 173-174) is:
1. `applyRuntimeRendererSkybox` (calls `SetExternalSkybox` + `UploadPendingExternalSkybox`)
2. `uploadDeferredRuntimeWorld` (calls `UploadWorld`)

So skybox upload is attempted **before** world upload. On the first frame
after map load, `UploadWorld` hasn't run, so
`worldSkyExternalBindGroupLayout` is `nil`, and the skybox upload silently
fails with "resources not ready." On subsequent frames, `UploadWorld`
completes, the layout exists, and the skybox begins uploading one face per
frame. This is functional but fragile.

### 2.3: Potential `queue.WriteTexture` Stall on Large Skybox Textures

`qbj2_zetabyt` may use high-resolution skybox textures (e.g., 4096x4096 per
face). `createWorldExternalSkyFaceTexture` creates a `CreateTexture` +
`queue.WriteTexture` + `CreateTextureView` for each face while holding
`r.mu.Lock()`. On Vulkan, `queue.WriteTexture` is a blocking call that
copies data into GPU-accessible staging memory. For a 4096x4096x4 = 64MB
face, this can take significant time and block the render thread. Six
faces at 64MB each = 384MB of synchronous GPU writes while holding the
renderer mutex.

---

## 3. Verification Strategy (Diagnose Before Fixing)

Before implementing any fix, we must **prove** which mechanism is causing
the hang. Previous attempts failed because they guessed.

### Step 0: Establish Stable Baseline

```bash
# Verify current main builds and passes tests
mise run verify

# Verify standard maps load fine
mise run smoke-all
```

### Step 1: Capture a Stack Trace During the Hang

Run `qbj2_zetabyt` and, when it hangs, send `SIGQUIT` to the process to
dump all goroutine stack traces:

```bash
# Start the game with qbj2_zetabyt
./ironwailgo -basedir ${QUAKE_DIR} +map qbj2_zetabyt &
GAME_PID=$!

# Wait for hang (30s should be enough)
sleep 30

# Dump all goroutine stacks
kill -QUIT $GAME_PID

# Or use pprof if available
# The engine supports -pprof flag
./ironwailgo -basedir ${QUAKE_DIR} -pprof +map qbj2_zetabyt &
# Then: go tool pprof http://localhost:6060/debug/pprof/goroutine?debug=2
```

**What to look for in the stack trace**:
- Is the main goroutine blocked in `UploadWorld`? (confirms 2.1)
- Is the render thread blocked in `queue.WriteTexture` or `queue.Submit`?
  (confirms GPU stall)
- Is the async skybox goroutine blocked on `r.mu.Lock()`? (confirms lock
  contention)
- Is `OnDraw` never called? (confirms event loop stall)
- Are there multiple goroutines waiting on `r.mu.Lock()`? (confirms
  contention)

### Step 2: Add Diagnostic Logging at Critical Points

Add temporary `slog.Info` calls at:

1. **`runtime_frame.go` `OnDraw` callback**: Log entry/exit of
   `applyRuntimeRendererState` and `drawRuntimeRendererFrame`.
2. **`runtime_frame.go` `OnUpdate` callback**: Log entry/exit.
3. **`renderer_gogpu_runtime.go` `OnDraw` wrapper**: Log before/after
   `callback(dc)`.
4. **`world_upload_gogpu.go` `UploadWorld`**: Log start/finish of each
   major sub-step (vertex buffer, index buffer, lightmap, texture atlas,
   materials, pipelines).
5. **`UploadPendingExternalSkybox`**: Already has logging, but add a log
   line **before** `r.mu.Lock()` and **after** `r.mu.Unlock()` to measure
   lock hold time.

### Step 3: Run with `host_speeds 1`

```bash
./ironwailgo -basedir ${QUAKE_DIR} +map qbj2_zetabyt +host_speeds 1
```

This enables per-phase timing logs. If the hang occurs before any
`host_speeds` output, the stall is in the upload/setup path, not the render
loop.

### Step 4: Check Skybox Texture Dimensions

Use `bspdiag` to inspect the map's sky configuration:

```bash
mise run build-bspdiag
./bspdiag entities ${QUAKE_DIR} maps/qbj2_zetabyt.bsp worldspawn
```

Look for the `sky` key in `worldspawn`. Then check the actual skybox image
file sizes:

```bash
# Find the skybox files
find ${QUAKE_DIR} -path "*/gfx/env/*zetabyt*" -o -path "*/env/*zetabyt*" | head -20
# Check dimensions
for f in $(find ${QUAKE_DIR} -path "*/gfx/env/*zetabyt*" -name "*.tga" -o -path "*/gfx/env/*zetabyt*" -name "*.png" -o -path "*/gfx/env/*zetabyt*" -name "*.jpg"); do
    echo "$f: $(file "$f")"
done
```

If faces are 4096x4096 or larger, this confirms mechanism 2.3.

### Step 5: Test with Skybox Disabled

Add a temporary cvar or env var to skip external skybox loading:

```bash
# If we add a temporary skip
IRONWAIL_SKIP_EXTERNAL_SKY=1 ./ironwailgo -basedir ${QUAKE_DIR} +map qbj2_zetabyt
```

If the map loads fine without the skybox, the hang is in the skybox upload
path. If it still hangs, the hang is in `UploadWorld` or elsewhere.

---

## 4. Implementation Plan

Based on the root cause analysis, the fix is implemented in phases. Each
phase is independently testable and can be reverted without affecting the
others.

### Phase A: Move Skybox Upload After World Upload (Safe Reorder)

**Problem**: `applyRuntimeRendererState` calls
`applyRuntimeRendererSkybox` (which attempts `UploadPendingExternalSkybox`)
**before** `uploadDeferredRuntimeWorld` (which calls `UploadWorld` that
creates `worldSkyExternalBindGroupLayout`).

**Fix**: Reorder `applyRuntimeRendererState` so world upload happens first:

**File**: `internal/game/runtime_frame.go`
**Function**: `applyRuntimeRendererState` (line 161)

```go
// Before (current):
g.applyRuntimeRendererVisualEffects(renderDT, g.Renderer, renderEvents)
g.applyRuntimeRendererSkybox(g.Renderer)  // skybox upload attempt

// After (proposed):
g.applyRuntimeRendererVisualEffects(renderDT, g.Renderer, renderEvents)
g.uploadDeferredRuntimeWorld()            // world upload first
g.applyRuntimeRendererSkybox(g.Renderer)  // skybox upload after
```

Wait — `uploadDeferredRuntimeWorld()` is already called at line 148, before
`drawRuntimeRendererFrame` at line 149. But
`applyRuntimeRendererState` (which includes skybox) is called at line 147,
before `uploadDeferredRuntimeWorld` at line 148. Let me re-read:

```go
// runtime_frame.go:137-150
g.Renderer.OnDraw(func(dc renderer.RenderContext) {
    g.runtimeMu.Lock()
    defer g.runtimeMu.Unlock()
    ...
    g.applyRuntimeRendererState(state)   // line 147 — includes skybox
    g.uploadDeferredRuntimeWorld()       // line 148 — world upload
    g.drawRuntimeRendererFrame(dc)      // line 149 — actual render
})
```

So the order is: skybox → world upload → render. We need: world upload →
skybox → render.

**Fix**: Move `uploadDeferredRuntimeWorld()` call into
`applyRuntimeRendererState` before `applyRuntimeRendererSkybox`, or reorder
in the `OnDraw` callback:

```go
g.Renderer.OnDraw(func(dc renderer.RenderContext) {
    g.runtimeMu.Lock()
    defer g.runtimeMu.Unlock()
    ...
    g.applyRuntimeRendererVisualEffects(state)  // split out visual effects
    g.uploadDeferredRuntimeWorld()              // world upload FIRST
    g.applyRuntimeRendererSkybox(g.Renderer)     // skybox upload after
    g.drawRuntimeRendererFrame(dc)
})
```

This requires splitting `applyRuntimeRendererState` so that world upload
happens between visual effects and skybox. The cleanest approach: move
`uploadDeferredRuntimeWorld()` to the top of
`applyRuntimeRendererState`, before `applyRuntimeRendererSkybox`.

**Risk**: Low. World upload must complete before rendering anyway (the
render path checks for `worldVertexBuffer == nil`). Moving it earlier in
the same callback is safe.

**Test**: Run `qbj2_zetabyt` — if the hang is caused by the skybox upload
failing because `worldSkyExternalBindGroupLayout` is `nil`, this fixes it.
Run `qbj2_start`, `qbj3_stickflip`, `e1m1` to verify no regression.

### Phase B: Upload All Skybox Faces in One `UploadPendingExternalSkybox` Call

**Problem**: `UploadPendingExternalSkybox` uploads only one face per call.
This means it takes 7 frames to complete (6 faces + bind group). During
each of those frames, it holds `r.mu.Lock()` across `CreateTexture` +
`WriteTexture` + `CreateTextureView`. While not a deadlock, it's 7 frames
of lock contention with the render path.

**Fix**: Change `UploadPendingExternalSkybox` to upload all pending faces in
a single call, then create the bind group:

**File**: `internal/renderer/world_external_sky_gogpu.go`
**Function**: `uploadNextGoGPUExternalSkyboxFaceLocked` (line 110)

Replace the single-face upload loop:
```go
// Current: uploads one face, returns
if r.worldSkyExternalUploadCursor < len(r.worldSkyExternalFaces) {
    return r.uploadGoGPUExternalSkyboxFaceLocked(device, queue, r.worldSkyExternalUploadCursor)
}
```

With a loop that uploads all pending faces:
```go
// Proposed: upload all pending faces in one call
for r.worldSkyExternalUploadCursor < len(r.worldSkyExternalFaces) {
    if r.worldSkyExternalViews[r.worldSkyExternalUploadCursor] != nil {
        r.worldSkyExternalUploadCursor++
        continue
    }
    if err := r.uploadGoGPUExternalSkyboxFaceLocked(device, queue, r.worldSkyExternalUploadCursor); err != nil {
        return err  // partial upload — will retry next frame
    }
}
// All faces uploaded — create bind group
```

**Risk**: Low. The function already has early-exit logic for partial uploads.
If one face fails, it returns an error and retries next frame. The only
change is doing the loop inside the lock instead of across frames.

**Test**: Verify skybox appears on first rendered frame after upload
completes. Run `qbj2_start` (which also has an external skybox) to verify
no regression.

### Phase C: Release `r.mu` During GPU Operations (Deadlock Prevention)

**Problem**: `UploadPendingExternalSkybox` holds `r.mu.Lock()` across
`queue.WriteTexture` — a potentially blocking GPU operation. If the GPU
queue is stalled (waiting for a prior frame's `Submit` to complete), this
blocks the render thread indefinitely.

**Fix**: Snapshot the skybox face data under the lock, release the lock,
perform the GPU upload, then re-acquire the lock to store the results.

**File**: `internal/renderer/renderer_gogpu_worldstate.go`
**Function**: `UploadPendingExternalSkybox` (line 216)

```go
func (r *Renderer) UploadPendingExternalSkybox() error {
    r.mu.Lock()
    if r.worldSkyExternalMode != externalSkyboxRenderFaces ||
       r.worldSkyExternalLoaded == 0 ||
       r.worldSkyExternalBindGroup != nil {
        r.mu.Unlock()
        return nil
    }
    // Snapshot all data needed for upload
    device := r.getWGPUDevice()
    queue := r.getWGPUQueue()
    sampler := r.worldLightmapSampler
    layout := r.worldSkyExternalBindGroupLayout
    name := r.worldSkyExternalName
    faces := r.worldSkyExternalFaces  // value copy of [6]externalSkyboxFace
    cursor := r.worldSkyExternalUploadCursor
    r.mu.Unlock()  // release lock during GPU operations

    if device == nil || queue == nil || sampler == nil || layout == nil {
        return fmt.Errorf("external sky resources not ready")
    }

    // Upload all pending faces without holding the lock
    for i := cursor; i < len(faces); i++ {
        if r.worldSkyExternalViews[i] != nil {  // already uploaded
            continue
        }
        texture, view, err := r.createWorldExternalSkyFaceTexture(
            device, queue,
            fmt.Sprintf("World External Sky %s", skyboxFaceSuffixes[i]),
            faces[i].RGBA, faces[i].Width, faces[i].Height,
        )
        if err != nil {
            return err
        }
        // Re-acquire lock to store results
        r.mu.Lock()
        r.worldSkyExternalTextures[i] = texture
        r.worldSkyExternalViews[i] = view
        r.worldSkyExternalUploadCursor = i + 1
        r.mu.Unlock()
    }

    // Create bind group
    r.mu.Lock()
    defer r.mu.Unlock()
    // ... (existing bind group creation logic)
}
```

**Risk**: Medium. The `worldSkyExternalViews` field is read by the render
path under `r.mu.RLock()`. The snapshot-then-store pattern means there's a
window where `worldSkyExternalViews[i]` is nil while the upload is in
progress. The render path already checks `worldSkyExternalBindGroup != nil`
(line 369) before using the skybox, so a nil view during upload is safe —
the render path falls back to the embedded sky pipeline.

**Test**: Run `qbj2_zetabyt` and all standard maps. Verify no deadlock and
skybox renders correctly.

### Phase D: Prevent `SetExternalSkybox` Re-entry on Every Frame

**Problem**: `applyRuntimeRendererSkybox` calls
`assets.SetExternalSkybox(g.SkyboxNameKey, ...)` every frame. Inside
`SetExternalSkybox`, if the name matches the current name, it returns early
(line 156). But it still acquires `r.mu.Lock()` to check. This is wasteful
but not a hang cause.

**Fix**: Add a guard in `applyRuntimeRendererSkybox` to only call
`SetExternalSkybox` when the name changes:

**File**: `internal/game/game_visual.go`
**Function**: `applyRuntimeRendererSkybox` (line 216)

```go
func (g *Game) applyRuntimeRendererSkybox(assets RendererAssets) {
    if assets == nil {
        return
    }
    if g.SkyboxNameKey == "" || g.Subs == nil || g.Subs.Files == nil {
        // Only clear if not already cleared
        if g.lastSkyboxNameKey != "" {
            assets.SetExternalSkybox("", nil)
            g.lastSkyboxNameKey = ""
        }
        return
    }
    if g.SkyboxNameKey != g.lastSkyboxNameKey {
        assets.SetExternalSkybox(g.SkyboxNameKey, g.Subs.Files.LoadFile)
        g.lastSkyboxNameKey = g.SkyboxNameKey
    }
    if err := assets.UploadPendingExternalSkybox(); err != nil {
        slog.Debug("external skybox upload deferred", "name", g.SkyboxNameKey, "error", err)
    }
}
```

Add `lastSkyboxNameKey string` to the `Game` struct.

**Risk**: Very low. Only avoids redundant lock acquisition.

**Test**: Verify skybox changes correctly on map transitions.

### Phase E: Add Timeout/Watchdog to Detect Future Hangs

**Problem**: If the hang recurs, there's no way to detect it automatically
and dump diagnostics.

**Fix**: Add a frame watchdog in the `OnUpdate` callback that logs a warning
if the time between `OnDraw` calls exceeds a threshold (e.g., 5 seconds):

**File**: `internal/game/runtime_frame.go`

```go
var lastDrawTime time.Time
g.Renderer.OnDraw(func(dc renderer.RenderContext) {
    now := time.Now()
    if !lastDrawTime.IsZero() && now.Sub(lastDrawTime) > 5*time.Second {
        slog.Warn("frame stall detected", "gap_seconds", now.Sub(lastDrawTime).Seconds())
    }
    lastDrawTime = now
    // ... rest of callback
})
```

**Risk**: Very low. Diagnostic only.

**Test**: Verify the warning triggers when simulating a stall (e.g., add a
`time.Sleep(6*time.Second)` in `UploadWorld` temporarily).

---

## 5. Execution Order

1. **Step 0**: Establish stable baseline (`mise run verify`, `mise run smoke-all`)
2. **Step 1**: Capture goroutine stack trace during hang (SIGQUIT or pprof)
3. **Step 2**: Add diagnostic logging, rebuild, run `qbj2_zetabyt`, capture logs
4. **Step 3**: Inspect skybox texture dimensions
5. **Step 4**: Test with skybox disabled (temporary env var)
6. Based on diagnosis results, implement the appropriate phases:
   - If hang is in `UploadWorld` → investigate world upload path
     (lightmap/texture atlas stalls), not skybox
   - If hang is in skybox upload → implement Phases A + B + C
   - If hang is lock contention → implement Phase C
   - Always implement Phase D (guard) and Phase E (watchdog) as preventive measures
7. **Verify**: Run `qbj2_zetabyt`, `qbj2_start`, `qbj3_stickflip`, `e1m1`,
   `start` — all must load and render without hangs or regressions
8. **Run parity**: `mise run parity-all` to verify no visual regression

---

## 6. Files to Modify

| File | Phase | Change |
| --- | --- | --- |
| `internal/game/runtime_frame.go` | A, E | Reorder skybox/world upload; add frame watchdog |
| `internal/game/game_visual.go` | D | Guard `SetExternalSkybox` against re-entry |
| `internal/game/game.go` | D | Add `lastSkyboxNameKey` field |
| `internal/renderer/renderer_gogpu_worldstate.go` | C | Release `r.mu` during GPU operations in `UploadPendingExternalSkybox` |
| `internal/renderer/world_external_sky_gogpu.go` | B | Upload all faces in one call |

---

## 7. Testing & Verification Plan

### 7.1 Unit Tests

```bash
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/game/... -count=1
```

### 7.2 Integration Tests

```bash
# Build
mise run build

# Standard maps (must not regress)
./ironwailgo -basedir ${QUAKE_DIR} +map start -screenshot /tmp/start.png
./ironwailgo -basedir ${QUAKE_DIR} +map e1m1 -screenshot /tmp/e1m1.png
./ironwailgo -basedir ${QUAKE_DIR} +map qbj2_start -screenshot /tmp/qbj2_start.png

# The hanging map (must now load)
./ironwailgo -basedir ${QUAKE_DIR} +map qbj2_zetabyt -screenshot /tmp/qbj2_zetabyt.png

# QBJ3 (must not regress)
./ironwailgo -basedir ${QUAKE_DIR} +map qbj3_stickflip -screenshot /tmp/qbj3_stickflip.png
```

### 7.3 Parity Verification

```bash
mise run parity-all
```

### 7.4 Stress Test

Load `qbj2_zetabyt` interactively and:
- Verify the skybox renders correctly (not black, not missing)
- Verify world geometry renders (not pitch black)
- Verify brush entities (doors, lifts) render with correct textures
- Verify no frame stalls (check `host_speeds 1` output)
- Verify map transitions (load `qbj2_zetabyt` then `start` then
  `qbj2_zetabyt` again)

### 7.5 Race Detector

```bash
mise run race
```

---

## 8. Rollback Plan

Each phase is independent. If a phase causes regressions:
1. Revert the specific phase's commits
2. Re-run tests to confirm the revert resolved the regression
3. The other phases remain in place

If all phases fail:
1. Revert to `51a27dd` (current stable main)
2. Use the diagnostic logging from Step 2 to inform the next attempt

---

## 9. Key Architectural Insight (For Future Work)

The fundamental architectural issue is that the renderer uses a single
`sync.RWMutex` (`r.mu`) to protect **all** renderer state — GPU resources,
world data, camera, configuration, callbacks. This means any GPU operation
that blocks (even temporarily) under `r.mu.Lock()` can stall the entire
render pipeline.

The C Ironwail renderer has no such lock because it's single-threaded: the
render loop and game logic run on the same thread. The Go port introduced
the two-thread model (`OnUpdate` + `OnDraw`) for smooth input handling, but
inherited a single-lock design that creates contention points.

A longer-term fix (out of scope for this plan) would be to split `r.mu`
into:
- A **resource lock** for GPU resource creation/destruction (textures,
  buffers, pipelines)
- A **state lock** for per-frame mutable state (camera, uniforms,
  lightstyle values)
- A **callback lock** for draw/update callback registration

This would allow GPU uploads to hold only the resource lock while the render
path holds only the state lock, eliminating the contention entirely.
