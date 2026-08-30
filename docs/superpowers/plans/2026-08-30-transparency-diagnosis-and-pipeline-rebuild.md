# Transparency Diagnosis, Test Suite, and Pipeline Rebuild Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the Go WebGPU rendering pipeline for complete transparency parity with C Ironwail, supported by a 4-quadrant test map (`test_transparency.bsp`), pure-Go synthetic procedural test suite, intermediate pass PNG dumpers (`r_dump_passes`), and GPU trace automation (gfxrecon/RenderDoc).

**Architecture:** A unified single-encoder frame graph dividing frame rendering into 5 explicit passes: (1) Opaque Scene Pass, (2) Translucent Accumulation Pass (OIT MRT accumulating world liquids, glass brush entities, alias models, and particles together), (3) OIT Resolve & View Model Pass, (4) Scene Composite & Post-Process Pass (warp + gamma/contrast), and (5) 2D Overlay Pass.

**Tech Stack:** Go 1.26, pure-Go `github.com/gogpu/gogpu` / `wgpu` / `gputypes`, WGSL shaders, Quake BSP/MAP formats, `gfxrecon` / `RenderDoc`.

---

## File Structure & Responsibilities

| File | Purpose |
| --- | --- |
| `internal/renderer/types.go` | CVar constants (`CvarRDumpPasses`, `CvarRPassIsolate`, `CvarROIT`), pass isolation enum. |
| `internal/renderer/pass_dump.go` | Staging buffer texture readback and PNG dumpers for intermediate render stages. |
| `internal/renderer/pass_dump_test.go` | Unit tests verifying staging readback and PNG encoding logic. |
| `internal/renderer/renderer_gogpu_oit.go` | OIT Accumulation & Resolve pipelines, textures, and MRT attachment descriptors. |
| `internal/renderer/renderer_gogpu_frame.go` | Rebuilt 5-pass unified frame graph recording loop. |
| `internal/renderer/renderer_gogpu_world_render.go` | World opaque and sky rendering pass implementations. |
| `internal/renderer/renderer_gogpu_world_translucent.go` | Translucent brush and liquid face rendering routines. |
| `internal/renderer/synthetic_bsp_transparency_test.go` | Synthetic in-memory multi-liquid & glass bmodel procedural test suite. |
| `tasks/parity-transparency.sh` | Shell script running C vs Go side-by-side screenshot and PSNR/SSIM comparison on `test_transparency.bsp`. |
| `tasks/gpu-trace/trace-vulkan.sh` | Vulkan API trace capture script via `gfxrecon-capture.py` / RenderDoc. |
| `tasks/gpu-trace/trace-opengl.sh` | OpenGL API trace capture script for C Ironwail. |
| `tasks/gpu-trace/trace-compare.py` | Automated comparison script analyzing draw calls, attachment formats, and blend states. |

---

### Task 1: Four-Quadrant Transparency Test Map & Synthetic BSP Test Suite

**Files:**
- Create: `internal/renderer/synthetic_bsp_transparency_test.go`
- Create: `tasks/parity-transparency.sh`
- Modify: `mise.toml`

- [ ] **Step 1: Write the failing unit test for synthetic multi-liquid transparency**

Create `internal/renderer/synthetic_bsp_transparency_test.go`:
```go
package renderer_test

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestSyntheticMultiLiquidTransparencyParity(t *testing.T) {
	// Build a synthetic world model with an opaque ground, a water pool, a glass brush, and submerged props.
	m := &model.Model{
		Type: model.ModBrush,
		Name: "maps/synthetic_transparency.bsp",
		Mins: types.Vec3{X: -256, Y: -256, Z: -128},
		Maxs: types.Vec3{X: 256, Y: 256, Z: 256},
	}
	if m == nil {
		t.Fatal("failed to initialize synthetic model")
	}
}
```

- [ ] **Step 2: Run test to verify it compiles and runs**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer -run TestSyntheticMultiLiquidTransparencyParity -count=1`
Expected: PASS

- [ ] **Step 3: Implement procedural synthetic test model with translucent liquids and stacked brushes**

Expand `internal/renderer/synthetic_bsp_transparency_test.go` to construct a multi-surface synthetic BSP geometry structure that simulates:
1. Ground floor with opaque texture and lightmap
2. Recessed water pool (`SURF_DRAWTURB`) with `wateralpha = 0.6`
3. Semi-transparent glass brush entity with `alpha = 0.5`
4. Verify face classification maps faces to `PassTranslucent` and `PassWorldOpaque`.

- [ ] **Step 4: Create parity test runner script `tasks/parity-transparency.sh`**

Create `tasks/parity-transparency.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

QUAKE_DIR="${QUAKE_DIR:-/home/darkliquid/Games/Heroic/Quake Enhanced/}"
IRONWAIL_C="${ROOT_DIR}/ironwail/Linux/ironwail"
IRONWAIL_GO="${ROOT_DIR}/ironwailgo"

echo "Running transparency parity comparison on test_transparency..."
# Generates side-by-side comparison images and prints metrics
```
Make executable: `chmod +x tasks/parity-transparency.sh`.

- [ ] **Step 5: Register `parity-transparency` in `mise.toml`**

Add task `parity-transparency` to `mise.toml`:
```toml
[tasks.parity-transparency]
description = "Run cross-engine transparency parity test on test_transparency.bsp"
run = "./tasks/parity-transparency.sh"
```

- [ ] **Step 6: Commit Task 1 changes**

```bash
git add internal/renderer/synthetic_bsp_transparency_test.go tasks/parity-transparency.sh mise.toml
git commit -m "feat(test): add synthetic transparency test suite and parity test task"
```

---

### Task 2: In-Engine Intermediate Attachment Dumper & Live Pass Isolation

**Files:**
- Create: `internal/renderer/pass_dump.go`
- Create: `internal/renderer/pass_dump_test.go`
- Modify: `internal/renderer/types.go`
- Modify: `internal/game/game_init.go`

- [ ] **Step 1: Write unit test for pass dump staging buffer extraction**

Create `internal/renderer/pass_dump_test.go`:
```go
package renderer

import (
	"image"
	"image/color"
	"testing"
)

func TestEncodeLinearizedDepthToGrayscalePNG(t *testing.T) {
	depthData := []float32{0.0, 0.5, 1.0, 0.25}
	img := encodeDepthToGrayImage(depthData, 2, 2)
	if img == nil {
		t.Fatal("expected non-nil image")
	}
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("unexpected dimensions: %v", img.Bounds())
	}
	c := img.At(0, 0).(color.Gray)
	if c.Y != 0 {
		t.Fatalf("expected 0, got %d", c.Y)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer -run TestEncodeLinearizedDepthToGrayscalePNG -count=1`
Expected: FAIL with `undefined: encodeDepthToGrayImage`

- [ ] **Step 3: Implement `pass_dump.go` with staging texture readback and image encoding**

Create `internal/renderer/pass_dump.go`:
```go
package renderer

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	CvarRDumpPasses   = "r_dump_passes"
	CvarRPassIsolate  = "r_pass_isolate"
)

func encodeDepthToGrayImage(depth []float32, width, height int) *image.Gray {
	if len(depth) != width*height || width <= 0 || height <= 0 {
		return nil
	}
	img := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			d := depth[y*width+x]
			if math.IsNaN(float64(d)) || d < 0 {
				d = 0
			} else if d > 1 {
				d = 1
			}
			img.SetGray(x, y, color.Gray{Y: uint8(d * 255.0)})
		}
	}
	return img
}

func saveImagePNG(img image.Image, filePath string) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
```

- [ ] **Step 4: Register `r_dump_passes` and `r_pass_isolate` cvars in `game_init.go` and `types.go`**

Register cvars:
- `r_dump_passes`: default `"0"`, description `"Dump intermediate render pass attachments to PNG (1=active frame, 0=off)"`
- `r_pass_isolate`: default `"0"`, description `"Isolate render pass on viewport (0=normal, 1=accum, 2=reveal, 3=depth, 4=opaque, 5=translucent)"`

- [ ] **Step 5: Run tests to verify they pass**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer -run TestEncodeLinearizedDepthToGrayscalePNG -count=1`
Expected: PASS

- [ ] **Step 6: Commit Task 2 changes**

```bash
git add internal/renderer/pass_dump.go internal/renderer/pass_dump_test.go internal/renderer/types.go internal/game/game_init.go
git commit -m "feat(renderer): add pass dump texture readback and cvar registration"
```

---

### Task 3: GPU Trace Automation Tooling (RenderDoc & gfxrecon)

**Files:**
- Create: `tasks/gpu-trace/trace-vulkan.sh`
- Create: `tasks/gpu-trace/trace-opengl.sh`
- Create: `tasks/gpu-trace/trace-compare.py`
- Modify: `mise.toml`

- [ ] **Step 1: Create `tasks/gpu-trace/trace-vulkan.sh`**

Implement automated Vulkan tracing using `gfxrecon-capture.py` or RenderDoc CLI layer:
```bash
#!/usr/bin/env bash
set -euo pipefail
OUTPUT_DIR="${1:-traces/vulkan}"
mkdir -p "${OUTPUT_DIR}"
# Run ironwailgo under gfxrecon
```

- [ ] **Step 2: Create `tasks/gpu-trace/trace-opengl.sh`**

Implement automated OpenGL tracing of C Ironwail using RenderDoc / apitrace.

- [ ] **Step 3: Create `tasks/gpu-trace/trace-compare.py`**

Implement python analyzer comparing:
- Draw call counts per render pass
- Color attachment formats and clear values
- Depth/stencil state (write enable, compare func)
- Blend equations (SrcColor, DstColor, ColorOp, SrcAlpha, DstAlpha, AlphaOp)

- [ ] **Step 4: Register `trace-vulkan` and `trace-opengl` in `mise.toml`**

- [ ] **Step 5: Commit Task 3 changes**

```bash
git add tasks/gpu-trace/ mise.toml
git commit -m "feat(tooling): add automated Vulkan and OpenGL GPU trace comparison scripts"
```

---

### Task 4: Translucent Geometry Accumulation Unification (Pass 2)

**Files:**
- Modify: `internal/renderer/renderer_gogpu_oit.go`
- Modify: `internal/renderer/renderer_gogpu_world_translucent.go`
- Modify: `internal/renderer/renderer_gogpu_frame.go`
- Test: `internal/renderer/renderer_gogpu_oit_test.go`

- [ ] **Step 1: Write test for OIT accumulation multi-target pipeline setup**

Create test in `internal/renderer/renderer_gogpu_oit_test.go` verifying that OIT accumulation pipeline is created with 2 color targets (`RGBA16Float` and `R8Unorm`) and `DepthWriteEnabled = false`.

- [ ] **Step 2: Run test to verify behavior**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer -run TestOITPipelineSetup -count=1`

- [ ] **Step 3: Unify translucent entities to accumulate into OIT MRT targets**

In `internal/renderer/renderer_gogpu_oit.go` and `renderer_gogpu_world_translucent.go`:
- Extend OIT accumulation pass to accept translucent brush entity batches, translucent alias model entities, and translucent particles alongside world liquid faces.
- Ensure all translucent shaders output to Target 0 (`accum`: `vec4(color.rgb * color.a, color.a) * weight`) and Target 1 (`reveal`: `color.a`).
- Configure pipeline blend states matching C:
  - Target 0: `One, One, Add`
  - Target 1: `Zero, OneMinusSrcColor, Add`

- [ ] **Step 4: Run renderer tests to verify compilation and execution**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1`
Expected: PASS

- [ ] **Step 5: Commit Task 4 changes**

```bash
git add internal/renderer/renderer_gogpu_oit.go internal/renderer/renderer_gogpu_world_translucent.go internal/renderer/renderer_gogpu_frame.go
git commit -m "feat(renderer): unify all translucent entity types into OIT MRT accumulation pass"
```

---

### Task 5: OIT Resolve & Post-Process Integration (Pass 3 & Pass 4)

**Files:**
- Modify: `internal/renderer/renderer_gogpu_oit.go`
- Modify: `internal/renderer/renderer_gogpu_frame.go`
- Modify: `internal/renderer/renderer_gogpu_warpscale.go`

- [ ] **Step 1: Write test for OIT resolve blending and viewmodel sequencing**

Verify that Pass 3 binds `SceneRenderTargetView` with `LoadOpLoad` and executes the resolve draw before viewmodel.

- [ ] **Step 2: Update `RenderFrame` in `renderer_gogpu_frame.go` to execute Pass 3 & Pass 4**

- Pass 3: Opens render pass on `SceneRenderTargetView`, runs `resolveOITHAL()`, then draws weapon view model.
- Pass 4: Opens render pass on `SwapchainTextureView`, executes fullscreen blit with underwater sinusoidal warp and gamma/contrast power curve.

- [ ] **Step 3: Run all renderer tests**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1`
Expected: PASS

- [ ] **Step 4: Commit Task 5 changes**

```bash
git add internal/renderer/renderer_gogpu_oit.go internal/renderer/renderer_gogpu_frame.go internal/renderer/renderer_gogpu_warpscale.go
git commit -m "feat(renderer): integrate OIT resolve, post-process blit, and view model pass ordering"
```

---

### Task 6: Full Rebuilt Pipeline Verification & Cross-Engine Parity Diffing

**Files:**
- Test: `tasks/parity-transparency.sh`
- Test: `internal/renderer/...`

- [ ] **Step 1: Run comprehensive unit and package tests**

Run: `mise run test`
Expected: All package tests PASS.

- [ ] **Step 2: Build binary and verify zero compiler or linter errors**

Run: `mise run verify`
Expected: Clean build with zero warnings or errors.

- [ ] **Step 3: Execute cross-engine parity comparison**

Run: `mise run parity-transparency`
Expected: Side-by-side diffs show SSIM &ge; 0.95 and PSNR &ge; 35 dB on `test_transparency.bsp`.

- [ ] **Step 4: Execute pass attachment dump verification**

Run with `r_dump_passes 1` to verify all 8 intermediate PNG files are created and cleanly visualized.

- [ ] **Step 5: Final commit and summary**

```bash
git commit --allow-empty -m "chore: verify complete transparency pipeline rebuild and test suite"
```
