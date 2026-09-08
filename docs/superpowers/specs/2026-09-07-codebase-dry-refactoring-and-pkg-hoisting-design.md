# Codebase DRY Refactoring and Public Package Hoisting Design

**Date:** 2026-09-07  
**Status:** Approved  
**Topic:** Code deduplication, math/type interface unification, public `pkg/` package hoisting for third-party tooling, and compiler pipeline modernization.

---

## 1. Executive Summary & Goals

Over time, as the Ironwail-Go engine has grown, several subsystems have developed code duplication, type misalignments, and blurred architectural boundaries:
1. **Mathematical Duplication & Type Inconsistency:** Vector operations are reimplemented across packages (e.g., `decal.go`, `internal/qbsp`, `internal/light`, `cmd/ironwailgo/chase.go`). In many locations, raw `[3]float32` or `[3]float64` arrays are passed instead of standard vector types, requiring manual conversions at callsites.
2. **Third-Party Developer Isolation:** Key Quake asset decoders and encoders (BSP/BSP2 maps, PAK archives, WAD2 texture containers, MDL/SPR models) currently reside under `internal/`. In Go, `internal/` packages cannot be imported by external packages. Moving self-contained file format packages to public `pkg/` modules empowers third-party Go developers to build Quake tooling while decoupling data parsing from engine runtime state.
3. **Map Tooling Fragmentation:** The map compilation toolchain (`qbsp`, `light`, `vis`) duplicates data structures (`PortalFile`, `Portal`) and relies on fragile, ad-hoc lump index slicing and text re-parsing rather than shared typed packages.
4. **Accumulated Dead Code:** During previous refactoring milestones, camera and telemetry helpers were extracted to `internal/game/`, but legacy prototype implementations remained in `cmd/ironwailgo/` (`viewcalc.go`, `chase.go`, `camera_compat.go`, `debug_view_telemetry.go`).

### Key Objectives
- **DRY Math:** Consolidate vector math into generic `types.Vec3T[T Float]` with transparent aliases `type Vec3 = Vec3T[float32]` and `type Vec3d = Vec3T[float64]`.
- **Align Engine Interfaces:** Standardize renderer uniform packers, DAP edict inspectors, and math helpers on `types.Vec3`, removing repetitive `[3]float32` boilerplate.
- **Publish Core Formats to `pkg/`:** Hoist clean, zero-dependency packages: `pkg/pak`, `pkg/wad`, `pkg/bsp`, and `pkg/mdl`. Relocate Quake's standard palette from the 2D UI drawer (`internal/draw`) to `pkg/wad`.
- **Modernize Toolchain Pipeline:** Unify `qbsp`, `light`, and `vis` around `types.Vec3d`, `types.Plane64`, and `pkg/bsp` lump APIs.
- **Prune Dead Code:** Remove orphaned files in `cmd/ironwailgo/` and consolidate `cmd/wadgen` into `cmd/qcmod wad`.
- **Zero Parity Drift:** Guarantee that all changes preserve byte-exact rendering, network protocol compatibility, and physics simulation parity.

---

## 2. Target Architecture & Package Layout

```
┌────────────────────────────────────────────────────────────────────────┐
│                               IRONWAIL GO                              │
└────────────────────────────────────────────────────────────────────────┘
                                    │
          ┌─────────────────────────┴────────────────────────┐
          ▼                                                  ▼
┌──────────────────┐                               ┌──────────────────┐
│  internal/...    │                               │     pkg/...      │
│  (Engine Core)   │                               │ (Public Tooling) │
├──────────────────┤                               ├──────────────────┤
│ - client         │─── uses public packages ────▶│ - types          │
│ - server         │                               │   (Vec3, Vec3d,  │
│ - renderer       │                               │    Plane, Box3)  │
│ - game           │                               │ - bsp            │
│ - qbsp/light/vis │                               │ - pak            │
│ - audio          │                               │ - wad            │
│ - cvar/cmdsys    │                               │ - mdl            │
└──────────────────┘                               └──────────────────┘
          ▲                                                  ▲
          │                                                  │
┌──────────────────┐                               ┌──────────────────┐
│ cmd/ironwailgo   │                               │ cmd/             │
│ (Game Binary)    │                               │ (CLI Toolchain)  │
│                  │                               │ - bspdiag        │
│                  │                               │ - qcmod          │
│                  │                               │ - qbsp           │
│                  │                               │ - light          │
│                  │                               │ - vis            │
└──────────────────┘                               └──────────────────┘
```

### Package Scopes

| Package | Source Location | Public Scope / Responsibilities |
|---|---|---|
| `pkg/types` | `pkg/types` | Unified 3D mathematics: generic `Vec3T[T]`, aliases `Vec3` (float32) & `Vec3d` (float64), `Plane` / `Plane64`, `Box3`, `Angles`, `Mat4`. |
| `pkg/pak` | Extracted from `internal/fs` | Quake `.pak` archive reader (`Reader`), writer (`Writer`), and layered virtual filesystem (`FileSystem`). Zero engine dependencies. |
| `pkg/wad` | Extracted from `internal/image`, `internal/draw` | WAD2 graphics container reading/writing, MipTex textures, and Quake 256-color palette (including `StandardQuakePaletteHex` / `DefaultQuakePalette`). |
| `pkg/bsp` | Extracted from `internal/bsp`, `internal/qbsp/writebsp.go` | BSP29 and BSP2 map parser, typed lump structures (`DPlane`, `DFace`, `DLeaf`, etc.), lump patcher/writer, entity lump tokenizer/dictionary, and `PortalFile`. |
| `pkg/mdl` | Extracted from `internal/model` | Decoders for Quake MDL (Alias model) and SPR (Sprite) binary formats. Decoupled from runtime GPU caching. |
| `internal/...` | Main engine packages | Runtime simulation (client, server, renderer, QuakeC VM, audio, host loops) importing canonical format packages from `pkg/*`. |

---

## 3. Detailed Design: The Four Refactoring Tracks

### Track 1: Math & Type Consistency

#### 1.1 Generic Vector Base `Vec3T[T]`
In `pkg/types/types.go`, introduce a generic vector struct constrained by `Float`:
```go
type Float interface {
    ~float32 | ~float64
}

type Vec3T[T Float] struct {
    X, Y, Z T
}

type Vec3 = Vec3T[float32]
type Vec3d = Vec3T[float64]
```

**Methods on `Vec3T[T]`:**
- **Arithmetic:** `Add(other Vec3T[T]) Vec3T[T]`, `Sub`, `Scale(s T)`, `Mul`, `Div`, `Dot`, `Cross`
- **Magnitude & Distance:** `Len() T`, `LenSq() T`, `Distance(other Vec3T[T]) T`, `DistanceSq`
- **Fused Math & Interpolation:** `MA(scale T, b Vec3T[T]) Vec3T[T]`, `Lerp(other Vec3T[T], t T) Vec3T[T]`
- **Normalization:**
  - `Normalize() Vec3T[T]` (returns vector scaled to unit length, or original vector if zero)
  - `NormalizeSafe() (Vec3T[T], bool)` (returns zero vector and `false` if squared length $\le 10^{-12}$)
- **Arrays & Slices:** `Array() [3]T`, `Slice() []T`
- **Angles & Directions:**
  - `AngleVectors() (forward, right, up Vec3T[T])`
  - `Angles() Vec3T[T]`
- **Conversions:**
  - `(v Vec3T[T]) Vec3() Vec3`
  - `(v Vec3T[T]) Vec3d() Vec3d`
- **Formatting & Comparison:** `String() string` (`fmt.Sprintf("%v %v %v", v.X, v.Y, v.Z)`), `Equals(other Vec3T[T]) bool`, `ApproxEqual(other Vec3T[T], eps T) bool`

**Type-Specific Network Wire Functions (Standalones):**
- `AngleMod(angle float32) float32`
- `AngleByte(angle float32) byte`
- `ByteToAngle(b byte) float32`
These operate specifically on Quake's single-precision wire protocol degrees.

#### 1.2 `Plane64` Geometric Primitive
In `pkg/types/plane.go`, add `Plane64`:
```go
type Plane64 struct {
    Normal Vec3d
    Dist   float64
    Type   int32
}

func ClassifyPlaneType64(n Vec3d) int32
func PlaneFromPoints(p0, p1, p2 Vec3d) (Plane64, float64)
func PlaneEqual(a, b Plane64, eps float64) bool
```
This unifies plane handling across `internal/qbsp` and `internal/light`.

#### 1.3 Renderer Uniform Type Alignment
In `internal/renderer/...`, replace `cameraOrigin [3]float32` parameter types with `types.Vec3`:
- `fillWorldSceneUniformBytes(..., cameraOrigin types.Vec3, ...)`
- `fillWorldSceneUniformBytesWithExternalSkyWind(..., cameraOrigin types.Vec3, ...)`
- `AppendAliasSceneUniformBytes(..., cameraOrigin types.Vec3, ...)`
- `particleUniformBytes(..., cameraOrigin types.Vec3, ...)`
- `gogpuWorldUniformInputs(...) (types.Vec3, float32, float32)`

Eliminate repeated `[3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}` boilerplate across callers in:
- `renderer_gogpu_world_render.go`
- `renderer_gogpu_world_brush_render.go`
- `renderer_gogpu_world_alias.go`
- `renderer_gogpu_particle.go`
- `renderer_gogpu_oit_accum.go`

#### 1.4 Decal Math Helper Consolidation
In `internal/renderer/decal/decal.go`:
- Delete `Add3`, `Scale3`, `Cross3`, `DistanceSq`, `Normalize3`.
- Replace with direct calls on `types.Vec3`: `a.Add(b)`, `a.Scale(s)`, `a.Cross(b)`, `origin.DistanceSq(camera)`, `v.NormalizeSafe()`.

#### 1.5 DAP Target Interface Alignment
In `internal/qc/dap/target.go`:
```go
type Target interface {
    VM() *qc.VM
    EdictCount() int
    GetEdictFloat(entNum, offset int) float32
    GetEdictString(entNum, offset int) string
    GetEdictVector(entNum, offset int) types.Vec3 // was [3]float32
    GetEdictClassName(entNum int) string
    FieldNames() map[string]int
}
```
Update implementations in `internal/server/server_dap.go` and `cmd/qcmod/dap.go` to return `s.QCVM.EVector(entNum, offset)` directly. Update `internal/qc/dap/variables.go` to format directly from `vec.X, vec.Y, vec.Z`.

---

### Track 2: Modular Public Format Packages

#### 2.1 `pkg/pak`
- **Location:** `pkg/pak/`
- **Contents:**
  - `pak.go`: Constants (`MaxQPath = 64`, `EnginePakName = "ironwail.pak"`), `Header`, `Entry`.
  - `reader.go`: `Reader`, `Open(path string)`, `NewReader(r io.ReaderAt, size int64)`, `ReadFile(name string) ([]byte, error)`.
  - `writer.go`: `Writer`, `NewWriter(w io.Writer)`, `AddFile(name string, data []byte)`, `AddReader(...)`.
  - `vfs.go`: `FileSystem`, `NewFileSystem()`, `MountDirectory(path string)`, `MountPak(path string)`.
- **Rewiring:** `internal/fs` becomes an alias or is replaced by `pkg/pak`.

#### 2.2 `pkg/wad`
- **Location:** `pkg/wad/`
- **Contents:**
  - `wad.go`: Magic (`"WAD2"`), `LumpType`, `Lump`, `Wad`, `Open(r io.ReaderAt, size int64)`.
  - `writer.go`: `Writer`, `NewWriter(w io.Writer)`, `AddLump(name string, lType LumpType, data []byte)`.
  - `palette.go`: `Palette [256]color.RGBA`, `LoadPalette(r io.Reader) (Palette, error)`, `StandardQuakePaletteHex`, `DefaultQuakePalette() []byte`.
  - `miptex.go`: `MipTex` header and mip levels, `ParseMipTex(data []byte)`, `(m *MipTex) ToRGBA(palette Palette, level int) *image.RGBA`.
- **Rewiring:** Remove `StandardQuakePaletteHex` and `DefaultQuakePalette()` from `internal/draw/palette.go`, importing `pkg/wad`. Update `cmd/qcmod/wad.go` and `internal/renderer/renderer_gogpu_texture.go`.

#### 2.3 `pkg/bsp`
- **Location:** `pkg/bsp/`
- **Contents:**
  - `bsp.go`: Format constants (`BSPVersion = 29`, `BSP2Version_BSP2`, etc.), lump constants (`LumpEntities`, `LumpPlanes`, etc.), on-disk structures (`DPlane`, `DFace`, `DLeaf`, etc.).
  - `lumps.go`: `ReadLumps(r io.ReaderAt) (int32, [][]byte, error)`, `WriteLumps(w io.Writer, version int32, lumps [][]byte) error`, `PatchLump(r io.ReaderAt, w io.Writer, lumpIndex int, data []byte) error`.
  - `entity.go`: Dedicated entity lump parser:
    ```go
    type Entity map[string]string
    func ParseEntities(entityLumpData string) ([]Entity, error)
    func (e Entity) Vec3(key string) (types.Vec3, bool)
    func (e Entity) Float(key string, defaultVal float32) float32
    func (e Entity) Int(key string, defaultVal int) int
    func (e Entity) String(key string) string
    ```
  - `tree.go`: In-memory `Tree`, `LoadTree(r io.ReadSeeker) (*Tree, error)`, `PointInLeaf(p types.Vec3) int`.
  - `lit.go`: Color lightmap sidecar parser.
  - `prt.go`: `PortalFile` and `Portal` data structures.
- **Rewiring:**
  - Update `cmd/bspdiag` to use `bsp.ParseEntities` instead of importing `internal/renderer/world`.
  - Update `internal/server/server_net_main.go` and `internal/renderer/world/liquid_alpha.go` to use `bsp.ParseEntities`.

#### 2.4 `pkg/mdl`
- **Location:** `pkg/mdl/`
- **Contents:**
  - `mdl.go`: Binary parser for Quake Alias models (`.mdl`), header, skins, frames, triangles, vertices.
  - `sprite.go`: Binary parser for Quake Sprite models (`.spr`), header, frame pictures.
- **Rewiring:** `internal/model` imports `pkg/mdl` for file parsing, keeping only runtime GPU caches and game entity linking internal.

---

### Track 3: Toolchain Pipeline Modernization

#### 3.1 Unify Portal Data Structures
- Move `PortalFile` and `Portal` into `pkg/bsp`:
  ```go
  type Portal struct {
      Leafs  [2]int
      Points []types.Vec3d
  }
  
  type PortalFile struct {
      LeafCount int
      Portals   []Portal
  }
  ```
- Both `internal/qbsp` and `internal/vis` use `bsp.PortalFile`. Direct programmatic execution (`qbsp.Compile` ➔ `vis.Run`) passes `*bsp.PortalFile` in-memory without string serialization.

#### 3.2 Modernize `internal/qbsp` Math
- Replace `type vec3 [3]float64` in `internal/qbsp/vector.go` with `types.Vec3d`.
- Replace `plane` and `classifyPlane` in `internal/qbsp/planes.go` with `types.Plane64` and `types.ClassifyPlaneType64`.

#### 3.3 Modernize `internal/light` and `internal/vis`
- Replace raw `[3]float64` in `internal/light` (`phong.go`, `bounce.go`, `sun.go`) with `types.Vec3d`.
- Update `light.PatchBSP` and `vis.Run` to call `bsp.PatchLump` and `bsp.WriteLumps`, removing manual hardcoded lump array indexing and byte slicing.

---

### Track 4: Dead Code Pruning & CLI Consolidation

#### 4.1 Delete Dead Files in `cmd/ironwailgo`
Delete the following unreferenced prototype files and their unit tests:
- `cmd/ironwailgo/viewcalc.go` & `viewcalc_test.go`
- `cmd/ironwailgo/chase.go` & `chase_test.go`
- `cmd/ironwailgo/camera_compat.go`
- `cmd/ironwailgo/debug_view_telemetry.go` & `debug_view_telemetry_test.go`

*(Note: Active runtime equivalents already live in `internal/game/game_camera_viewcalc.go`, `internal/game/camera/math.go`, `internal/game/game_camera_chase.go`, and `internal/game/debug_view_telemetry.go`.)*

#### 4.2 Consolidate `cmd/wadgen` into `cmd/qcmod wad`
- Ensure `qcmod wad` can generate dummy/placeholder WADs needed by tests.
- Retire `cmd/wadgen` or reduce it to a thin forwarding wrapper.

---

## 4. Verification, Testing & Parity Safety Gates

Every step of implementation must pass strict quality and parity gates:

1. **Unit Test Gates:**
   ```bash
   TMPDIR=.tmp CGO_ENABLED=0 go test ./... -count=1
   ```
2. **Build & Lint Gates:**
   ```bash
   mise run verify
   mise run lint
   ```
3. **Smoke Tests:**
   ```bash
   mise run smoke-all
   ```
4. **Parity Checkpoints:**
   - Visual Parity: `mise run parity-compare` (guarantees zero pixel regression after renderer uniform cleanup).
   - Tool Parity: `go test ./internal/qbsp -run TestParity` (guarantees map compilation output matches reference).
5. **Knowledge Graph Refresh:**
   ```bash
   graphify update .
   ```

---

## 5. Phased Implementation Roadmap

- **Phase 1: Foundation (Track 1 & Dead Code Pruning)**
  - 1.1 Prune unreferenced zombie files in `cmd/ironwailgo/` and verify clean build/tests.
  - 1.2 Implement generic `Vec3T[T]`, aliases `Vec3`/`Vec3d`, `Plane64`, and helper methods in `pkg/types` with comprehensive unit tests.
  - 1.3 Refactor renderer uniform functions and decal helpers to use `types.Vec3`.
  - 1.4 Update DAP `Target` interface and implementations to return `types.Vec3`.
- **Phase 2: Modular Public Packages (Track 2)**
  - 2.1 Hoist `pkg/pak` (PAK reader/writer/VFS) and rewire consumers.
  - 2.2 Hoist `pkg/wad` (WAD2 container, miptex, palette relocation) and retire `wadgen`.
  - 2.3 Hoist `pkg/bsp` (BSP29/BSP2 reader, lump patcher, entity parser, `PortalFile`).
  - 2.4 Hoist `pkg/mdl` (MDL and SPR format decoders) and rewire `internal/model`.
- **Phase 3: Map Toolchain Pipeline Modernization (Track 3)**
  - 3.1 Unify `PortalFile` and `Portal` in `pkg/bsp`.
  - 3.2 Refactor `internal/qbsp` to use `types.Vec3d` and `types.Plane64`.
  - 3.3 Refactor `internal/light` to use `types.Vec3d` and `bsp.PatchLump`.
  - 3.4 Refactor `internal/vis` to use `bsp.PortalFile` and `bsp.PatchLump`.
- **Phase 4: Final Integration & Knowledge Graph Update**
  - 4.1 Run full build, test, lint, smoke, and parity test sweeps.
  - 4.2 Run `graphify update .` to index new packages and types into the knowledge graph.
