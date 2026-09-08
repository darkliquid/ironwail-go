# Phase 1: Math & Type Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a unified, DRY mathematical foundation with generic `Vec3T[T]` (`Vec3` and `Vec3d`), `Plane64`, align renderer uniforms and DAP interfaces to `types.Vec3`, eliminate duplicate decal math helpers, and prune obsolete unreferenced files in `cmd/ironwailgo`.

**Architecture:** Refactor `pkg/types` to use a generic vector type constrained by `~float32 | ~float64` with transparent type aliases `type Vec3 = Vec3T[float32]` and `type Vec3d = Vec3T[float64]`, ensuring 100% backwards compatibility with zero duplicate code. Align consumer signatures in renderer uniform byte packers and DAP `Target` to use `types.Vec3` directly. Remove orphaned camera/viewcalc files in `cmd/ironwailgo` that were previously superseded by `internal/game/`.

**Tech Stack:** Go 1.26, pure Go (`CGO_ENABLED=0`), `pkg/types`, `internal/renderer`, `internal/qc/dap`, `internal/server`.

---

## Task 1: Prune Obsolete Dead Code in `cmd/ironwailgo`

**Files:**
- Delete: `cmd/ironwailgo/viewcalc.go`
- Delete: `cmd/ironwailgo/viewcalc_test.go`
- Delete: `cmd/ironwailgo/chase.go`
- Delete: `cmd/ironwailgo/chase_test.go`
- Delete: `cmd/ironwailgo/camera_compat.go`
- Delete: `cmd/ironwailgo/debug_view_telemetry.go`
- Delete: `cmd/ironwailgo/debug_view_telemetry_test.go`

- [ ] **Step 1: Verify current baseline tests pass before deleting**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./cmd/ironwailgo/... -count=1
```
Expected: PASS

- [ ] **Step 2: Delete unreferenced files in `cmd/ironwailgo`**

Run:
```bash
rm cmd/ironwailgo/viewcalc.go \
   cmd/ironwailgo/viewcalc_test.go \
   cmd/ironwailgo/chase.go \
   cmd/ironwailgo/chase_test.go \
   cmd/ironwailgo/camera_compat.go \
   cmd/ironwailgo/debug_view_telemetry.go \
   cmd/ironwailgo/debug_view_telemetry_test.go
```

- [ ] **Step 3: Run package test to verify `cmd/ironwailgo` builds and passes without the deleted files**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./cmd/ironwailgo/... -count=1
```
Expected: PASS

- [ ] **Step 4: Run full repo test to verify no other package depended on `cmd/ironwailgo`**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./... -count=1
```
Expected: PASS

---

## Task 2: Implement Generic `Vec3T[T]` and `Vec3`/`Vec3d` in `pkg/types`

**Files:**
- Modify: `pkg/types/types.go`
- Test: `pkg/types/types_test.go`

- [ ] **Step 1: Write failing tests for generic `Vec3T`, `Vec3d`, and new methods (`Vec3()`, `Vec3d()`, `NormalizeSafe()`, `String()`)**

In `pkg/types/types_test.go`, add:
```go
func TestVec3dOperations(t *testing.T) {
	a := Vec3d{X: 1.0, Y: 2.0, Z: 3.0}
	b := Vec3d{X: 4.0, Y: 5.0, Z: 6.0}

	sum := a.Add(b)
	if sum != (Vec3d{X: 5.0, Y: 7.0, Z: 9.0}) {
		t.Fatalf("Vec3d.Add got %v, want {5 7 9}", sum)
	}

	diff := b.Sub(a)
	if diff != (Vec3d{X: 3.0, Y: 3.0, Z: 3.0}) {
		t.Fatalf("Vec3d.Sub got %v, want {3 3 3}", diff)
	}

	dot := a.Dot(b)
	if dot != 32.0 {
		t.Fatalf("Vec3d.Dot got %v, want 32", dot)
	}

	cross := a.Cross(b)
	if cross != (Vec3d{X: -3.0, Y: 6.0, Z: -3.0}) {
		t.Fatalf("Vec3d.Cross got %v, want {-3 6 -3}", cross)
	}

	arr := a.Array()
	if arr != [3]float64{1.0, 2.0, 3.0} {
		t.Fatalf("Vec3d.Array got %v, want [1 2 3]", arr)
	}

	str := a.String()
	if str != "1 2 3" {
		t.Fatalf("Vec3d.String got %q, want '1 2 3'", str)
	}

	// Conversion tests
	v32 := a.Vec3()
	if v32 != (Vec3{X: 1.0, Y: 2.0, Z: 3.0}) {
		t.Fatalf("Vec3d.Vec3 got %v, want {1 2 3}", v32)
	}

	v64 := v32.Vec3d()
	if v64 != a {
		t.Fatalf("Vec3.Vec3d got %v, want %v", v64, a)
	}
}

func TestNormalizeSafe(t *testing.T) {
	zero := Vec3{}
	norm, ok := zero.NormalizeSafe()
	if ok || norm != zero {
		t.Fatalf("zero.NormalizeSafe() = (%v, %v), want (zero, false)", norm, ok)
	}

	v := Vec3{X: 3, Y: 0, Z: 4}
	norm, ok = v.NormalizeSafe()
	if !ok || !norm.ApproxEqual(Vec3{X: 0.6, Y: 0, Z: 0.8}, 1e-5) {
		t.Fatalf("v.NormalizeSafe() = (%v, %v), want ({0.6 0 0.8}, true)", norm, ok)
	}

	v64 := Vec3d{X: 3, Y: 0, Z: 4}
	norm64, ok64 := v64.NormalizeSafe()
	if !ok64 || norm64.X != 0.6 || norm64.Z != 0.8 {
		t.Fatalf("v64.NormalizeSafe() = (%v, %v), want ({0.6 0 0.8}, true)", norm64, ok64)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./pkg/types -run "TestVec3dOperations|TestNormalizeSafe" -count=1
```
Expected: FAIL (Vec3d undefined, NormalizeSafe undefined)

- [ ] **Step 3: Update `pkg/types/types.go` to generic `Vec3T[T]` and define `Vec3`/`Vec3d`**

In `pkg/types/types.go`, replace `type Vec3 struct` and methods with generic base:
```go
// Float represents supported floating-point types for Quake vectors.
type Float interface {
	~float32 | ~float64
}

// Vec3T is a generic 3D vector with X, Y, Z components in Quake's right-handed
// coordinate system (X=forward, Y=left, Z=up).
type Vec3T[T Float] struct {
	X T
	Y T
	Z T
}

// Vec3 is the single-precision 3D vector used for physics, networking, and rendering.
type Vec3 = Vec3T[float32]

// Vec3d is the double-precision 3D vector used for high-precision map tools (CSG, lighting).
type Vec3d = Vec3T[float64]

// Vec3Add adds two vectors component-wise.
func Vec3Add(a, b Vec3) Vec3 {
	return a.Add(b)
}

// Vec3Sub subtracts two vectors component-wise.
func Vec3Sub(a, b Vec3) Vec3 {
	return a.Sub(b)
}

// Vec3Scale multiplies every component of v by scalar s.
func Vec3Scale(v Vec3, s float32) Vec3 {
	return v.Scale(s)
}

// Vec3Dot returns the dot product of two vectors.
func Vec3Dot(a, b Vec3) float32 {
	return a.Dot(b)
}

// Vec3Cross returns the cross product of two vectors.
func Vec3Cross(a, b Vec3) Vec3 {
	return a.Cross(b)
}

// Vec3Len returns Euclidean length.
func Vec3Len(v Vec3) float32 {
	return v.Len()
}

// Vec3Normalize returns a unit-length vector.
func Vec3Normalize(v Vec3) Vec3 {
	return v.Normalize()
}

// Vec3MA performs fused Multiply-Add: veca + scale*vecb.
func Vec3MA(veca Vec3, scale float32, vecb Vec3) Vec3 {
	return veca.MA(scale, vecb)
}

// Vec3Lerp performs linear interpolation between veca and vecb by fraction frac.
func Vec3Lerp(veca, vecb Vec3, frac float32) Vec3 {
	return veca.Lerp(vecb, frac)
}

func (v Vec3T[T]) Add(other Vec3T[T]) Vec3T[T] {
	return Vec3T[T]{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

func (v Vec3T[T]) Sub(other Vec3T[T]) Vec3T[T] {
	return Vec3T[T]{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

func (v Vec3T[T]) Scale(s T) Vec3T[T] {
	return Vec3T[T]{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

func (v Vec3T[T]) Mul(s T) Vec3T[T] {
	return v.Scale(s)
}

func (v Vec3T[T]) Div(s T) Vec3T[T] {
	inv := T(1.0) / s
	return Vec3T[T]{X: v.X * inv, Y: v.Y * inv, Z: v.Z * inv}
}

func (v Vec3T[T]) Dot(other Vec3T[T]) T {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

func (v Vec3T[T]) Cross(other Vec3T[T]) Vec3T[T] {
	return Vec3T[T]{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

func (v Vec3T[T]) LenSq() T {
	return v.Dot(v)
}

func (v Vec3T[T]) LengthSq() T {
	return v.LenSq()
}

func (v Vec3T[T]) Len() T {
	return T(math.Sqrt(float64(v.LenSq())))
}

func (v Vec3T[T]) Length() T {
	return v.Len()
}

func (v Vec3T[T]) Distance(other Vec3T[T]) T {
	return v.Sub(other).Len()
}

func (v Vec3T[T]) Dist(other Vec3T[T]) T {
	return v.Distance(other)
}

func (v Vec3T[T]) DistanceSq(other Vec3T[T]) T {
	return v.Sub(other).LenSq()
}

func (v Vec3T[T]) Normalize() Vec3T[T] {
	l := v.Len()
	if l > 0 {
		return v.Scale(T(1.0) / l)
	}
	return v
}

func (v Vec3T[T]) NormalizeSafe() (Vec3T[T], bool) {
	lSq := float64(v.LenSq())
	if lSq <= 1e-12 {
		return Vec3T[T]{}, false
	}
	invLen := T(1.0 / math.Sqrt(lSq))
	return v.Scale(invLen), true
}

func (v Vec3T[T]) Neg() Vec3T[T] {
	return Vec3T[T]{X: -v.X, Y: -v.Y, Z: -v.Z}
}

func (v Vec3T[T]) Negate() Vec3T[T] {
	return v.Neg()
}

func (v Vec3T[T]) MA(scale T, b Vec3T[T]) Vec3T[T] {
	return Vec3T[T]{
		X: v.X + scale*b.X,
		Y: v.Y + scale*b.Y,
		Z: v.Z + scale*b.Z,
	}
}

func (v Vec3T[T]) MultiplyAdd(scale T, b Vec3T[T]) Vec3T[T] {
	return v.MA(scale, b)
}

func (v Vec3T[T]) Lerp(other Vec3T[T], t T) Vec3T[T] {
	return Vec3T[T]{
		X: v.X + (other.X-v.X)*t,
		Y: v.Y + (other.Y-v.Y)*t,
		Z: v.Z + (other.Z-v.Z)*t,
	}
}

func (v Vec3T[T]) Array() [3]T {
	return [3]T{v.X, v.Y, v.Z}
}

func (v Vec3T[T]) Slice() []T {
	return []T{v.X, v.Y, v.Z}
}

func (v *Vec3T[T]) Set(x, y, z T) {
	v.X = x
	v.Y = y
	v.Z = z
}

func (v Vec3T[T]) Equals(other Vec3T[T]) bool {
	return v.X == other.X && v.Y == other.Y && v.Z == other.Z
}

func (v Vec3T[T]) ApproxEqual(other Vec3T[T], eps T) bool {
	return T(math.Abs(float64(v.X-other.X))) <= eps &&
		T(math.Abs(float64(v.Y-other.Y))) <= eps &&
		T(math.Abs(float64(v.Z-other.Z))) <= eps
}

func (v Vec3T[T]) String() string {
	return fmt.Sprintf("%v %v %v", v.X, v.Y, v.Z)
}

func (v Vec3T[T]) Vec3() Vec3 {
	return Vec3{X: float32(v.X), Y: float32(v.Y), Z: float32(v.Z)}
}

func (v Vec3T[T]) Vec3d() Vec3d {
	return Vec3d{X: float64(v.X), Y: float64(v.Y), Z: float64(v.Z)}
}
```

- [ ] **Step 4: Run tests to verify all `pkg/types` tests pass**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./pkg/types/... -count=1
```
Expected: PASS

---

## Task 3: Implement `Plane64` in `pkg/types`

**Files:**
- Modify: `pkg/types/plane.go`
- Test: `pkg/types/plane_test.go`

- [ ] **Step 1: Write failing tests for `Plane64`**

In `pkg/types/plane_test.go`, add:
```go
func TestPlane64FromPointsAndClassification(t *testing.T) {
	p0 := Vec3d{X: 0, Y: 0, Z: 0}
	p1 := Vec3d{X: 64, Y: 0, Z: 0}
	p2 := Vec3d{X: 64, Y: 64, Z: 0}

	plane, length := PlaneFromPoints(p0, p1, p2)
	if length <= 0 {
		t.Fatalf("expected positive length, got %v", length)
	}
	if plane.Normal != (Vec3d{X: 0, Y: 0, Z: 1}) {
		t.Fatalf("expected Normal {0 0 1}, got %v", plane.Normal)
	}
	if plane.Type != PlaneZ {
		t.Fatalf("expected PlaneZ, got %v", plane.Type)
	}

	pEqual := Plane64{Normal: Vec3d{X: 0, Y: 0, Z: 1.00001}, Dist: 0.00001}
	if !PlaneEqual(plane, pEqual, 0.0001) {
		t.Fatal("expected PlaneEqual to report true within epsilon")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./pkg/types -run TestPlane64FromPointsAndClassification -count=1
```
Expected: FAIL (Plane64, PlaneFromPoints undefined)

- [ ] **Step 3: Implement `Plane64` in `pkg/types/plane.go`**

In `pkg/types/plane.go`, add:
```go
// Plane64 represents an infinite 2D plane in double precision (Normal · P - Dist = 0).
type Plane64 struct {
	Normal Vec3d
	Dist   float64
	Type   int32
}

// ClassifyPlaneType64 determines whether a 64-bit normal is axial or non-axial.
func ClassifyPlaneType64(n Vec3d) int32 {
	if n.X == 1.0 || n.X == -1.0 {
		return PlaneX
	}
	if n.Y == 1.0 || n.Y == -1.0 {
		return PlaneY
	}
	if n.Z == 1.0 || n.Z == -1.0 {
		return PlaneZ
	}

	ax := math.Abs(n.X)
	ay := math.Abs(n.Y)
	az := math.Abs(n.Z)

	if ax >= ay && ax >= az {
		return PlaneAnyX
	}
	if ay >= ax && ay >= az {
		return PlaneAnyY
	}
	return PlaneAnyZ
}

// PlaneFromPoints constructs a Plane64 from three non-collinear points:
// normal = normalize(cross(p0-p1, p2-p1)) and dist = dot(p1, normal).
func PlaneFromPoints(p0, p1, p2 Vec3d) (Plane64, float64) {
	ab := p0.Sub(p1)
	cb := p2.Sub(p1)
	cr := ab.Cross(cb)
	normal, length := cr.Normalize()
	return Plane64{
		Normal: normal,
		Dist:   p1.Dot(normal),
		Type:   ClassifyPlaneType64(normal),
	}, length
}

// PlaneEqual reports whether two planes match within epsilon.
func PlaneEqual(a, b Plane64, eps float64) bool {
	if math.Abs(a.Normal.X-b.Normal.X) > eps ||
		math.Abs(a.Normal.Y-b.Normal.Y) > eps ||
		math.Abs(a.Normal.Z-b.Normal.Z) > eps {
		return false
	}
	return math.Abs(a.Dist-b.Dist) <= eps
}
```

- [ ] **Step 4: Run tests to verify all pass**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./pkg/types/... -count=1
```
Expected: PASS

---

## Task 4: Consolidate Decal Math Helpers

**Files:**
- Modify: `internal/renderer/decal/decal.go`
- Test: `internal/renderer/decal/decal_test.go`

- [ ] **Step 1: Check existing decal tests pass**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/decal/... -count=1
```
Expected: PASS

- [ ] **Step 2: Replace redundant standalone helper functions in `internal/renderer/decal/decal.go`**

In `internal/renderer/decal/decal.go`:
Replace calls to `Add3(a, b)` with `a.Add(b)`, `Scale3(a, s)` with `a.Scale(s)`, `Cross3(a, b)` with `a.Cross(b)`, `DistanceSq(origin, camera)` with `origin.DistanceSq(camera)`, and `Normalize3(v)` with `v.NormalizeSafe()`.
Then delete the redundant helper definitions `Add3`, `Scale3`, `Cross3`, `DistanceSq`, `Normalize3` (lines ~288-311).

- [ ] **Step 3: Run decal tests to verify equivalence**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/decal/... -count=1
```
Expected: PASS

---

## Task 5: Align DAP Target Interface to `types.Vec3`

**Files:**
- Modify: `internal/qc/dap/target.go`
- Modify: `internal/qc/dap/variables.go`
- Modify: `internal/qc/dap/variables_test.go`
- Modify: `internal/server/server_dap.go`
- Modify: `internal/server/dap_integration_test.go`
- Modify: `cmd/qcmod/dap.go`
- Modify: `cmd/qcmod/dap_test.go`

- [ ] **Step 1: Update `Target` interface in `internal/qc/dap/target.go`**

Change:
```go
GetEdictVector(entNum, offset int) [3]float32
```
To:
```go
GetEdictVector(entNum, offset int) types.Vec3
```
(Import `"github.com/darkliquid/ironwail-go/pkg/types"`).

- [ ] **Step 2: Update `internal/qc/dap/variables.go`**

In `formatEdictField`:
```go
vec := target.GetEdictVector(entNum, ofs)
return fmt.Sprintf("[%v, %v, %v]", vec.X, vec.Y, vec.Z), "vector"
```

- [ ] **Step 3: Update `internal/server/server_dap.go`**

In `GetEdictVector`:
```go
func (s *Server) GetEdictVector(entNum, offset int) types.Vec3 {
	if s == nil || s.QCVM == nil || entNum < 0 || entNum >= s.NumEdicts {
		return types.Vec3{}
	}
	return s.QCVM.EVector(entNum, offset)
}
```

- [ ] **Step 4: Update `cmd/qcmod/dap.go`**

In `GetEdictVector`:
```go
func (t *qcmodTarget) GetEdictVector(entNum, offset int) types.Vec3 {
	if t == nil || t.vm == nil || entNum < 0 || entNum >= t.vm.NumEdicts() {
		return types.Vec3{}
	}
	return t.vm.EVector(entNum, offset)
}
```

- [ ] **Step 5: Update tests in `variables_test.go`, `dap_integration_test.go`, and `cmd/qcmod/dap_test.go`**

Replace `[3]float32{10, 20, 30}` with `types.Vec3{X: 10, Y: 20, Z: 30}` in mock implementations and assertions.

- [ ] **Step 6: Run DAP and Server tests to verify they pass**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap/... ./internal/server -run TestDAP -count=1
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./cmd/qcmod/... -count=1
```
Expected: PASS

---

## Task 6: Align Renderer Uniform Functions to `types.Vec3`

**Files:**
- Modify: `internal/renderer/renderer_gogpu_world_render.go`
- Modify: `internal/renderer/renderer_gogpu_world_brush_render.go`
- Modify: `internal/renderer/renderer_gogpu_world_alias.go`
- Modify: `internal/renderer/renderer_gogpu_particle.go`
- Modify: `internal/renderer/renderer_gogpu_oit_accum.go`
- Modify: `internal/renderer/world/gogpu/aliasbytes.go`
- Modify: `internal/renderer/world/gogpu/aliasbytes_test.go`
- Modify: `internal/renderer/renderer_gogpu_entities_test.go`

- [ ] **Step 1: Update uniform packer signatures to accept `cameraOrigin types.Vec3`**

In `internal/renderer/world/gogpu/aliasbytes.go`:
```go
func AppendAliasSceneUniformBytes(dst []byte, targetOffset uint32, vp types.Mat4, cameraOrigin types.Vec3, alpha float32, fogColor types.Vec3, fogDensity float32) []byte
```
Inside the function, write `cameraOrigin.X, cameraOrigin.Y, cameraOrigin.Z` into the byte buffer.

In `internal/renderer/renderer_gogpu_world_render.go`:
```go
func fillWorldSceneUniformBytes(dst []byte, vp types.Mat4, cameraOrigin types.Vec3, fogColor types.Vec3, fogDensity float32, time float32, alpha float32, litWater float32)
func fillWorldSceneUniformBytesWithExternalSkyWind(dst []byte, vp types.Mat4, cameraOrigin types.Vec3, fogColor types.Vec3, fogDensity float32, timeValue float32, wind externalSkyboxWind, windLoaded bool)
func gogpuWorldUniformInputs(state *RenderFrameState, camera CameraState) (types.Vec3, float32, float32)
```

In `internal/renderer/renderer_gogpu_world_alias.go`:
```go
func appendAliasSceneUniformBytes(dst []byte, targetOffset uint32, vp types.Mat4, cameraOrigin types.Vec3, alpha float32, fogColor types.Vec3, fogDensity float32) []byte
```

In `internal/renderer/renderer_gogpu_particle.go`:
```go
func particleUniformBytes(vp types.Mat4, projScale [2]float32, uvScale float32, cameraOrigin types.Vec3, fogColor types.Vec3, fogDensity float32) []byte
```

- [ ] **Step 2: Update all callsites to pass `camera.Origin` directly**

In `renderer_gogpu_world_render.go`, `renderer_gogpu_world_brush_render.go`, `renderer_gogpu_world_alias.go`, `renderer_gogpu_particle.go`, and `renderer_gogpu_oit_accum.go`:
Remove:
```go
cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
```
And pass `camera.Origin` directly into uniform calls.

- [ ] **Step 3: Update renderer tests to pass `types.Vec3`**

In `internal/renderer/world/gogpu/aliasbytes_test.go` and `internal/renderer/renderer_gogpu_entities_test.go`, pass `types.Vec3{X: 1, Y: 2, Z: 3}` or `types.Vec3{X: 4, Y: 5, Z: 6}`.

- [ ] **Step 4: Run renderer tests to verify all pass**

Run:
```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1
```
Expected: PASS

---

## Task 7: Full Quality & Parity Verification Sweep

**Files:**
- Entire repository

- [ ] **Step 1: Run full test suite**

Run:
```bash
mise run test
```
Expected: PASS (all unit and integration tests green)

- [ ] **Step 2: Run build gate (includes go generate)**

Run:
```bash
mise run build
```
Expected: Binary `./ironwailgo` builds cleanly with code 0

- [ ] **Step 3: Run linter and vulnerability scanner**

Run:
```bash
mise run lint
```
Expected: PASS (zero lint errors or vulnerabilities)

- [ ] **Step 4: Run smoke test suite**

Run:
```bash
mise run smoke-all
```
Expected: PASS (`smoke-menu`, `smoke-headless`, `smoke-map-start` pass)

- [ ] **Step 5: Run parity screenshot comparison**

Run:
```bash
mise run parity-compare
```
Expected: 0 pixel regressions vs golden baselines

- [ ] **Step 6: Update graphify knowledge graph**

Run:
```bash
graphify update .
```
Expected: Graph updated with new AST nodes for `Vec3T`, `Vec3d`, and `Plane64`.
