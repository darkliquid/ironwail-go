# Implementation Plan 23: Parity Hardening — Behavior Divergences C vs Go

**Priority**: #1 (Parity is the project's primary oracle)
**Status**: IN PROGRESS (2026-08-08) — D1 fixed earlier; D4 fixed (inactive-slots gate
+ C-cited test); D3 NaN warnings landed; D2 verified-equal; D6 documented in
PARITY.md; D8 verified by existing tests + doc note. D5 (renderer audit) and
D7 (docs, folded into plan 26) remain.
**Prerequisite**: stable baseline (all tests pass); includes a **landed fix** from
research (`internal/server/physics/leafs.go` pusher think-gate) already in tree.
**Estimated effort**: 4-7 focused sessions

---

## 1. Executive Summary & Architectural Context

Research (`docs/plans/22_*` sibling report + `$HOME/.local/share/crush/research/
ironwail-go-parity/report.md`) cataloged the remaining behavioral divergences
between C Ironwail (`ironwail/Quake/*.c`) and Go. The strongest one — the pusher
think-gate race — is **fixed in-tree** (see §4, D1). This plan hardens the rest,
each with a C-cited red/green test, and expands the parity tooling to turn the
current manual sweep into deterministic, CI-able gates.

## 2. Catalog of Divergences to Fix (severity order)

| # | Divergence | C anchor | Go anchor | Severity | Status |
| --- | --- | --- | --- | --- | --- |
| D1 | Pusher think-gate re-read → double/skip think | `SV_Physics_Pusher` sv_phys.c:618-652 | `leafs.go` PhysicsPusher | **high** | FIXED in-tree (+test) |
| D2 | flymove clip-plane budget: C `MAX_CLIP_PLANES 5`, Go `maxClipPlanes=5` + `bumpCount<4` (matches C `numbumps=4`) — **verified equal, no change needed** | sv_phys.c:230,254 | leafs.go:21,193 | n/a (verified) | closed |
| D3 | NaN velocity/origin: C warns, Go silent | sv_phys.c:87-110 | leafs.go CheckVelocity | low | FIXED (`a77e64f`) — slog.Warn added |
| D4 | Inactive client slots still dispatch movetype | sv_phys.c:946-956 | stepframe.go | med | FIXED (`a77e64f`) — slot gate + test |
| D5 | Renderer brightness/contrast (qbj3 ~7 mean delta) | gl_rmain.c:352 + gl_shaders.h postprocess | renderer present path | high (visual) | IMPROVED (2026-08-08): index-255 fullbright parity + engine-capture default landed; median ratio 0.90; residual multi-factor (see §23.5) |
| D6 | Entity send distance-sorted (Go addition; C edict order) | sv_main.c | server_net_send.go:363+ | intentional | DONE — documented in PARITY.md deviations table |
| D7 | Stale docs claim QCVM sync-all; code is no-op accessors | QCVM_ENTITY_SYNC.md | server_qc_sync.go:11-17 | low (teachability) | DONE — rewritten in plan 26 (`8117ea0`) |
| D8 | LERP_FINISH byte: verify equality at encode time | sv_main.c:952 | net/encode.go:32-43 | low | DONE — verified by existing tests + doc note |

## 3. Step-by-Step Implementation Sequence

### Step 23.1: D2 — FlyMove MAX_CLIP_PLANES 4 → 5 (+ C-cited probe)
- **Files**: `internal/server/physics/leafs.go` (`maxClipPlanes`),
  `internal/server/physics/*_test.go`.
- **Actions**: align `maxClipPlanes` to C's 5; add a parity probe that runs an
  entity into a synthetic 5-plane crease (mock collision world emitting 5
  co-planar terms) and asserts the Go path does not dead-stop (returns blocked
  per C, not 7-with-zero-velocity).
- **Where in C**: `#define MAX_CLIP_PLANES 5` sv_phys.c:216 + SV_FlyMove
  plane/crease block sv_phys.c:320-345.

### Step 23.2: D4 — inactive client slots skip movetype dispatch
- **Files**: `internal/server/physics/stepframe.go`.
- **Actions**: mirror C's `if (!svs.clients[num-1].active) return;` gate at the
  top of the client-slot iteration (i in 1..maxclients): skip the movetype
  switch AND pre/post think for inactive slots. Add a test with an inactive
  slot whose movetype is WALK asserting no `PlayerPreThink`/`PhysicsWalk` runs.
- **Where in C**: `SV_Physics_Client` sv_phys.c:946-956.

### Step 23.3: D3 — NaN warns (cosmetic parity)
- **Files**: `internal/server/physics/leafs.go` CheckVelocity.
- **Actions**: emit `slog.Warn` (like C `Con_Printf`) when NaN velocity/origin
  zeroed; keep behavior identical.
- **Where in C**: sv_phys.c:87-110.

### Step 23.4: D5 — Renderer brightness/contrast audit (photometric)
- **Files**: `internal/renderer` (gamma/tonemap/lightmap sampling candidates),
  `tools/parity_screenshots/compare.go` (already computes mean/max channel
  delta + SSIM — use it as the measurement tool).
- **Actions**:
  1. Classify the delta (whole-frame gain vs local lightmap drift): run the
     id1 scene matrix (all 31 viewpoints; `mise run parity-*`) and split
     diffs into global (gain/gamma) vs spatial (lightmap/texinfo) buckets.
  2. Compare Go's gamma application path against C (`r_gamma` handling in
     gl_rmain.c:352) and the lightmap sample→final pipeline against the C
     world shader (`gl_shaders.h`); fix the culprit.
  3. Re-run the matrix; target qbj3 mean channel delta < 2 with id1/expansion
     green.
- **Verify**: `mise run parity-ref && parity-go && parity-compare`; tighten
  `PARITY_COMPARE_TOLERANCE`/`PARITY_MAX_MISMATCH_PERCENT` as deltas shrink.

### 23.4-a D5 audit results (2026-08-08)

Structural comparison completed — most candidate stages are **proven equal** to
C, which narrows the hunt:

| Stage | C | Go | Verdict |
| --- | --- | --- | --- |
| Final present gamma/contrast | `postprocess_fragment_shader`: `rgb*=contrast; pow(rgb,gamma)` (gl_shaders.h:265-268), params from gl_rmain.c:352 | **no equivalent stage** — `overlay_composite_gogpu.go` fs_main is a plain `textureSample`; `r_gamma`/contrast read into `config.Gamma` but never consumed | Equal at defaults (C identity at gamma=1, contrast=1) — not the default-path culprit, but a real feature gap for non-default cvars |
| Lightmap combine | `total_light *= 2.0` then `mix(rgb, rgb*total_light, a)` (gl_shaders.h:720-728) | `lightExpr = "mix(sampled.rgb, sampled.rgb*totalLight*2.0, sampled.a)..."` | **Equal** |
| Lightmap byte path | raw sample bytes | `CompositePageRGBA`/`StackPages` copy bytes unfiltered | **Equal** (no 128/2 bias) |
| Fog | `ApplyFog`: `fog=clamp(exp2(-w·d²)); mix(Fog.rgb, clr, fog)` (gl_shaders.h:297-304) | same formula in world fragment shader | **Equal** |

Measured deltas on the *committed* captures (PIL, 1892x1072): **stale-artifact
warnings** — `go/id1-start-spawn.png` is 100% black (broken capture epoch,
not a real diff), `qbj3-start-view2` nearly black. Only `qbj3-start-view1`
is a clean pair: `go ≈ 0.53·ref + 13` luma fit (Go darker, lower contrast,
black-lift).

**Working hypothesis** (next session): the 0.53 gain + +13 lift pattern is
neither gamma (identity at defaults) nor lightmath/fog (equal). Prime
suspects, in order:
1. **Texture sampling**: Go world shader uses `textureSample` (bilinear) with
   a single `worldSampler`; C uses `textureLod` at explicit coords —
   a half-texel/atlas-edge bleed on the base texture could dim/cheapen
   large lit walls.
2. **Atlas UV half-texel**: Go clamps `localUV` to `halfTexel` of a hardcoded
   `2048.0` atlas (`renderer_gogpu_world_shaders.go:213-216`) — if the actual
   atlas width ≠ 2048 or mip sampling differs, sampled texels shift → dimmer.
3. **Dynamic-light cluster difference** (qbj3 has many): C accumulates
   `dynamic_light /256 * color` with minlight/rad falloff; Go's
   `accumulateDynamicLights` may scale/falloff differently → underlit areas.

**Next action**: capture fresh id1 + qbj3 frames with the current binary
(`mise run parity-go` with a working display/`PARITY_GO_CAPTURE=engine`),
then A/B the world sampler (Nearest vs Linear) and the atlas half-texel
constant on the clean qbj3-view1 pair.

### 23.4-b D5 continued (2026-08-08) — audit results & landed fixes

Follow-up audit after the viewpoint matrix was repaired (31 valid open-space
viewpoints, distinct positions) and Go captures switched to engine readback:

| Candidate | Verdict |
| --- | --- |
| Light combine `mix(sampled, sampled*light*2, a)` | Equal to C (opaque `a=1` → `sampled*light*2` both) |
| Lightmap byte path (`CompositePageRGBA`/`StackPages`) | Equal — raw bytes, no 128/2 bias; `.lit` RGB adopted identically |
| Lightstyle values (`DefaultStyleValues[0]=1`) | Equal to C's static `1.f` |
| Lightmap UV texel-centering / page padding | Equal — geometry emits `+0.5` interior-clamped coords |
| Fog (`exp2(-d²)`, `mix`) | Equal |
| Dynamic-light accumulation (`clamp((rad-min-dist)/16)*max(0,rad-dist)/256*color`) | Line-for-line equal |
| Final gamma/contrast postprocess | C applies `rgb*=contrast; pow(rgb,gamma)` at present; Go has no equivalent stage — but identity at defaults (`gamma=1`,`contrast=1`), not the default-path culprit (still a feature gap for non-default cvars) |
| **Fullbright set (real bug)** | C computes `is_fullbright` from the **colormap** (`gl_texmgr.c:760`), which for the standard palette marks **225..255** incl. **index 255** (brownish skin `(159,91,83)`). Go hardcoded `224..254`, **excluding 255** → Go lit skin/leather that C keeps unlit. **Fixed** (`texture.go:134` range → `224..255`), with regression test. |
| Capture path (harness bug) | Go parity default was **X11 window capture** (xdotool+ImageMagick `import`), which routes through the compositor and applies its own gamma/color management. Switched default to **`PARITY_GO_CAPTURE=engine`** (renderer `CaptureScreenshot` scene readback — same as C's internal screenshot). Verified engine readback == window pixels for the same frame, so window capture wasn't the dimmer, but engine capture is the parity-correct path. |

Measured after fixes (engine capture, 31 viewpoints): **median luma ratio 0.90**
(was ~0.81 window / ~0.78 stale), 10 viewpoints ≥0.93, several ≥1.04 (Go
*brighter*: qbj3-view4/5, e1m1-spawn-cross, hip1m1). Closest absolute pairs:
qbj2-water-room Δ3.15, qbj3-view2 Δ3.60, e3m7 Δ4.66, stickflip-spawn Δ4.53.

**Remaining residual** ~0.90 median: not attributable to any single audited
stage (all equal). Bright-region-specific dimming on some viewpoints
(stickflip bright walls ~2× dim) and pattern correlation drops on dark
viewpoints suggest a **lightmap addressing/sampling boundary** issue (e.g.
Linear sampling bleeding between styles/pages) plus dynamic-light cluster grid
differences. Requires on-GPU per-surface instrumentation to pin down; parked
pending the D5 photometric gate re-run on the repaired matrix.

### Step 23.5: D6 — Document the entity-send sort as intentional
- **Files**: `docs/PARITY.md` §Known parity gaps (extend the table with a
  "deliberate deviation" row citing `server_net_send.go`), plus a code comment
  at the sort (`entitySendSortKey`) naming C's edict-order baseline.
- **Action**: no behavior change; make the deviation auditable.

### Step 23.6: D8 — empirical LERP_FINISH probe
- **Files**: `internal/server/*_test.go` (parity probe), reuse
  `TestParitySoundEmittedSameFrame` conventions.
- **Actions**: build a client/server pair (loopback), drive a WALK entity with
  a known `nextthink`; assert `EncodeLerpFinish(nextThink, s.Time)` byte equals
  `(byte)Q_rint((nextthink - qcvm->time)*255)` from the C reference for the
  same inputs, including the gate on `sendinterval` (j in 0..255, j≠25,26).

### Step 23.7: QCVM/EntVars parity re-check (pigsback on zero-sync completion)
- **Files**: `docs/plans/10/12` (`EntVars` removal), `internal/server/edict`.
- **Actions**: after zero-sync completes, re-run the qbj2 door + lift chain
  probes (existing `TestQbj2TwinDoorsBothFireViaChain` etc.) and assert the
  ORIGINAL frame-parity tests (`internal/game/parity_test.go` demo parity) stay
  green — the accessor layer is the new source of truth.

## 4. Verification & Testing Strategy

1. **Red/green discipline per fix**: each step lands with its C-cited test
   first (fails before, passes after).
2. **Regression surface**: `mise run test` (no assets required), then
   `mise run smoke-map-start` (needs display) and `mise run parity-all` for
   visual regressions after D5.
3. **Frame-state parity** (`internal/game/parity_test.go`) re-run for demo1 on
   a full install with `demo1.dem` present (skips on Quake Enhanced data —
   extend with H1 dump schema from plan 24 so any demo becomes usable).

## 5. Risks & Mitigations

| Risk | Mitigation |
| --- | --- |
| D2 crease path hard to hit deterministically | mock collision world emits plane terms; also fuzz via H4 (plan 24) |
| D5 audit churns lightmap pipeline | gate every change on the 32-viewpoint matrix; keep deltas as fences |
| D4 changes feel-behavior for intermission/non-walk clients | C-cited gate + explicit tests incl. intermission MOVETYPE_NONE path |
| D8 is encode-time-clock-drift sensitive | drive clocks exactly (srvTime known); assert range not exact where C/Go frame timing differs |
