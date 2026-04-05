# Parity Scene Matrix

Visual parity validation framework for Ironwail Go against reference C Ironwail.
Part of Milestone 0 ("Parity Harness and Scene Matrix") from the Entity Rendering PRD.

---

## Scope boundary: CSQC for near-term parity

Near-term parity work is scoped to the canonical NetQuake/FitzQuake engine path.
CSQC runtime enablement (for example host/client loading and execution of
`csprogs.dat`) is explicitly **deferred** from near-term parity milestones.

Rationale:

- Current parity harness/sign-off in this document is centered on deterministic
  baseline engine behavior and renderer parity.
- The repository has a CSQC VM wrapper in `internal/qc/csqc.go`, but host/client
  runtime wiring for a full CSQC gameplay path is not yet part of the current
  parity milestone criteria.
- Keeping CSQC runtime deferred prevents expanding protocol/runtime scope while
  baseline parity milestones are still being stabilized.

Deferred means "not in near-term milestones," not "rejected." Keep CSQC wrapper
infrastructure maintained and revisit runtime integration after near-term parity
targets are complete or when parity scope is explicitly expanded.

---

## 1. Scene Matrix

Canonical id1 scenes for parity comparison. Each row defines a repeatable
test scenario with a specific map, camera position, and validation focus.

| # | Category | Map | Location / Setup | What to Validate |
|---|---|---|---|---|
| 1 | Indoor lightmapped | `e1m1` | Start area, looking down the hallway | Lightmap quality, brush surface rendering, texture alignment on walls/floors |
| 2 | Indoor combat | `e1m3` | First ogre encounter room | Dynamic lights from gunfire, entity model rendering, particle effects |
| 3 | Outdoor sky | `e1m5` | Outside area with sky visible | Sky rendering, sky/world boundary seams, sky texture scroll |
| 4 | Liquid-heavy | `e1m4` | Bridge over the slime pool | Liquid warp effect, transparency, liquid surface edges, depth sorting |
| 5 | Underwater | `e2m2` | Submerged in water | Waterwarp screen distortion, color tint, content-driven visual effects |
| 6 | Dynamic lighting | `e1m1` | Torches in the first corridor | Lightstyle animation, flicker patterns, light radius falloff |
| 7 | Occlusion / visibility | `e1m2` | Doorway transition areas | PVS correctness, no pop-in during movement through portals |
| 8 | External skybox | *(custom)* | Any map with an external skybox loaded | Cubemap face rendering, fallback behavior, orientation |
| 9 | Particle effects | `e1m1` | Shoot wall with nailgun | Particle spawning count, motion direction, alpha blending |
| 10 | Entity models | `e1m3` | Approaching a knight enemy | Alias model geometry, frame animation, skin texture mapping |
| 11 | Sprites | `e1m1` | Torch sprite in corridor | Billboard orientation, placement accuracy, transparency |
| 12 | Viewmodel | `e1m1` | Holding shotgun, firing | Weapon placement, view bob, kick offset, muzzle flash animation |
| 13 | Transparency | `e2m2` | Near water surface from above | Translucent surface sorting, depth buffer interaction, alpha edges |

### Scene Status Tracking

Use this table to track sign-off per milestone. Mark each cell as
`pass`, `fail`, `skip` (not relevant to milestone), or `deviation` (accepted difference).

| # | Category | M0 | M1 | M2 | M3 | M4 | Notes |
|---|---|---|---|---|---|---|---|
| 1 | Indoor lightmapped | — | | | | | |
| 2 | Indoor combat | — | | | | | |
| 3 | Outdoor sky | — | | | | | |
| 4 | Liquid-heavy | — | | | | | |
| 5 | Underwater | — | | | | | |
| 6 | Dynamic lighting | — | | | | | |
| 7 | Occlusion / visibility | — | | | | | |
| 8 | External skybox | — | | | | | |
| 9 | Particle effects | — | | | | | |
| 10 | Entity models | — | | | | | |
| 11 | Sprites | — | | | | | |
| 12 | Viewmodel | — | | | | | |
| 13 | Transparency | — | | | | | |

---

## 2. Screenshot Capture Workflow

### Freezing Time

Both engines must be in an identical, deterministic state. Use:

```
host_framerate 0.0001
```

This effectively freezes simulation time so animations, particles, and
lighting are captured at a single consistent instant.

### Capturing from C Ironwail (Reference)

1. Launch C Ironwail: `./ironwail -basedir $QUAKE_DIR`
2. Load the target map: `map e1m1`
3. Navigate to the scene location described in the matrix.
4. Freeze time: `host_framerate 0.0001`
5. Take screenshot: `screenshot`
6. Screenshots land in `<basedir>/id1/screenshots/` by default in current
   Ironwail builds.

### Capturing from Go

1. Launch Ironwail Go (GoGPU build for renderer parity): `mise run run-gogpu`
2. Repeat steps 2–5 identically.

### Naming Convention

```
<map>_<scene>_<engine>.tga
```

Examples:
- `e1m1_indoor_lightmapped_c.tga`
- `e1m1_indoor_lightmapped_go.tga`
- `e1m3_entity_models_c.tga`
- `e1m3_entity_models_go.tga`

The automated harness in this repo normalizes captures into:

- `testdata/parity/reference/`
- `testdata/parity/go/`
- `testdata/parity/diff/`

These are working artifacts for local parity investigation; do **not** treat
them as canonical golden images to commit casually.

### Reproducing Camera Position

For exact reproducibility, record the console output of `viewpos` in C
Ironwail and replay it in Go:

```
setpos <x> <y> <z> <pitch> <yaw> <roll>
```

Document the `setpos` command for each scene row in a companion file or
inline comments when capturing.

---

## 3. Comparison Method & Thresholds

### Primary: Human Visual Review

Side-by-side comparison is the **authoritative** method. Open both
screenshots in an image viewer and look for:

- Geometry misalignment or missing surfaces
- Lighting intensity or color differences
- Texture coordinate errors (stretched, offset, or missing textures)
- Missing or incorrectly sorted transparent surfaces
- Particle count, direction, or blending errors
- Animation frame mismatches

### Secondary: Per-Pixel RMSE

Use ImageMagick `compare` for an objective numeric delta:

```bash
compare -metric RMSE screenshot_c.tga screenshot_go.tga diff.tga
```

This outputs an RMSE value (0–255 scale) and generates a visual diff image.

### Thresholds

| Scene Type | RMSE Threshold | Rationale |
|---|---|---|
| Static scenes (lightmapped, sky, occlusion) | < 2.0 | Fully deterministic; differences indicate real bugs |
| Dynamic lighting (lightstyles, gunfire) | < 3.0 | Minor timing jitter in flicker patterns |
| Motion / particles | < 5.0 | Non-deterministic spawning and physics |

**Hard-fail rule:** Any scene with **any pixel** where the per-channel
delta exceeds **30** (out of 255) is an automatic failure regardless of
RMSE. This catches localized rendering errors (e.g., a single missing
surface or broken texture) that might average out in the global RMSE.

### Per-Channel Max-Delta Check

```bash
compare -metric AE -fuzz 30 screenshot_c.tga screenshot_go.tga null:
```

If the output (absolute error count) is `0`, the hard-fail rule passes.

---

## 4. Sign-off Process

### Per-Milestone Sign-off

Each milestone in the Entity Rendering PRD requires passing the scene
categories relevant to it:

1. **Run the full scene matrix** against the current build.
2. **Capture screenshots** for every applicable scene row.
3. **Run RMSE and max-delta checks** for each pair.
4. **Perform human visual review** of any borderline results.
5. **Update the status tracking table** (§1) with pass/fail/skip.

### Regression Testing

After each milestone lands, **re-run the full matrix** (not just the
scenes added in that milestone). This catches regressions in previously
passing categories.

### Accepted Deviations

Some differences may be intentional or unavoidable (e.g., slightly
different floating-point rounding in Go vs C). Document any accepted
deviation with:

- **Scene #** and category
- **Description** of the visible difference
- **RMSE value** for the pair
- **Rationale** for accepting the deviation

Keep accepted deviations in a section at the bottom of the status
tracking table or in a separate `PARITY_DEVIATIONS.md` if the list grows.

### Maintainer Approval

Final sign-off for each milestone is a **manual review by the
maintainer**. Automated RMSE checks are evidence, not a substitute for
human judgment.

---

## 5. Smoke Test Coverage

Existing smoke tests in `mise.toml` validate engine lifecycle stages
before visual parity testing begins. These must all pass as a
prerequisite for any scene matrix run.

| Smoke Test | What It Validates |
|---|---|
| `mise run smoke-menu` | Basic GoGPU startup: renderer loop start plus filesystem mount (`"FS mounted"`), QuakeC load (`"QC loaded"`), menu activation (`"menu active"`), frame loop (`"frame loop started"`) |
| `mise run smoke-headless` | Headless mode operation from the GoGPU parity build (no window, no renderer) |
| `mise run smoke-map-start` | GoGPU map spawn lifecycle: renderer loop start plus `"map spawn started"` → `"map spawn finished"` |
| `mise run smoke-all` | Runs all of the above in sequence |

### Current Harness Commands

Use this repo-local harness sequence for parity work on the canonical
GoGPU path:

1. `mise run smoke-all`
2. `mise run parity-ref`
3. `mise run parity-go`
4. `PARITY_COMPARE_TOLERANCE=0 PARITY_MAX_MISMATCH_PERCENT=0 mise run parity-compare`

`parity-compare` is expected to exit nonzero on missing captures or on any
scene that exceeds the configured mismatch threshold.

### Deterministic Markers

Smoke tests look for specific log markers in engine output. These markers
**must not be changed or removed** without updating the corresponding
smoke test scripts:

- `"FS mounted"`
- `"QC loaded"`
- `"menu active"`
- `"frame loop started"`
- `"map spawn started"`
- `"map spawn finished"`

---

## 6. Existing Tooling

### Function-Level Parity Audit

These tools compare C Ironwail functions against their Go counterparts to
track porting progress. They are complementary to (not a replacement for)
visual parity testing.

| Tool | Description |
|---|---|
| `tools/compare_parity` | Go CLI for the C-to-Go function mapping audit. Parses C source and matches against Go implementations. |
| `tasks/compare_parity.sh` | Shell wrapper that runs the Go parity-audit CLI. |

### Workflow Integration

A typical parity validation session:

1. **Smoke tests pass** → `mise run smoke-all`
2. **Function audit clean** → `go run ./tools/compare_parity`
3. **Scene matrix captured** → Screenshots from both engines
4. **RMSE + visual review** → Thresholds checked, human review done
5. **Status table updated** → This document updated with results
6. **Maintainer sign-off** → Manual approval recorded

---

## Appendix: Quick Reference

```bash
# Freeze time for deterministic capture
host_framerate 0.0001

# Take screenshot
screenshot

# Record camera position
viewpos

# Set camera position (for reproduction)
setpos <x> <y> <z> <pitch> <yaw> <roll>

# RMSE comparison
compare -metric RMSE ref.tga test.tga diff.tga

# Max-delta hard-fail check (fuzz=30)
compare -metric AE -fuzz 30 ref.tga test.tga null:
```
