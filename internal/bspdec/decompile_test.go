package bspdec

import (
	"math"
	"strings"
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// brushPlanes re-derives a map brush's planes from its face points.
func brushPlanes(t *testing.T, mb *mapfile.MapBrush) []mapfile.Plane {
	t.Helper()
	planes := make([]mapfile.Plane, 0, len(mb.Faces))
	for _, f := range mb.Faces {
		p, length := mapfile.PlaneFromPoints(f.Points[0], f.Points[1], f.Points[2])
		if length < 0.01 {
			t.Fatalf("degenerate face points %v", f.Points)
		}
		planes = append(planes, p)
	}
	return planes
}

// voxelOccupancy rasterizes worldspawn brushes: the set of lattice cells
// whose center is inside any brush (inside = behind every plane). The
// optional clamps bound the scanned region.
func voxelOccupancy(t *testing.T, m *mapfile.Map, cell float64, cmins, cmaxs mapfile.Vec3) map[[3]int]struct{} {
	t.Helper()
	ws := &m.Entities[0]
	type polyBrush struct{ planes []mapfile.Plane }
	var brushes []polyBrush
	mins := mapfile.Vec3{X: math.MaxFloat64, Y: math.MaxFloat64, Z: math.MaxFloat64}
	maxs := mapfile.Vec3{X: -math.MaxFloat64, Y: -math.MaxFloat64, Z: -math.MaxFloat64}
	for i := range ws.Brushes {
		brushes = append(brushes, polyBrush{brushPlanes(t, &ws.Brushes[i])})
		for _, f := range ws.Brushes[i].Faces {
			for _, p := range f.Points {
				mins.X = math.Min(mins.X, p.X)
				mins.Y = math.Min(mins.Y, p.Y)
				mins.Z = math.Min(mins.Z, p.Z)
				maxs.X = math.Max(maxs.X, p.X)
				maxs.Y = math.Max(maxs.Y, p.Y)
				maxs.Z = math.Max(maxs.Z, p.Z)
			}
		}
	}
	occ := map[[3]int]struct{}{}
	x0 := math.Floor(mins.X/cell) * cell
	y0 := math.Floor(mins.Y/cell) * cell
	z0 := math.Floor(mins.Z/cell) * cell
	x1, y1, z1 := maxs.X, maxs.Y, maxs.Z
	if cmins.X > x0 {
		x0 = cmins.X
	}
	if cmins.Y > y0 {
		y0 = cmins.Y
	}
	if cmins.Z > z0 {
		z0 = cmins.Z
	}
	if cmaxs.X < x1 {
		x1 = cmaxs.X
	}
	if cmaxs.Y < y1 {
		y1 = cmaxs.Y
	}
	if cmaxs.Z < z1 {
		z1 = cmaxs.Z
	}
	for x := x0; x < x1; x += cell {
		for y := y0; y < y1; y += cell {
			for z := z0; z < z1; z += cell {
				c := mapfile.Vec3{X: x + cell/2, Y: y + cell/2, Z: z + cell/2}
				for _, b := range brushes {
					inside := true
					for _, p := range b.planes {
						if v3Dot(c, p.Normal)-p.Dist > 0.01 {
							inside = false
							break
						}
					}
					if inside {
						occ[[3]int{int(x / cell), int(y / cell), int(z / cell)}] = struct{}{}
						break
					}
				}
			}
		}
	}
	return occ
}

// mapBounds returns the tight bounding box of all points in worldspawn.
func mapBounds(t *testing.T, m *mapfile.Map) (mapfile.Vec3, mapfile.Vec3) {
	t.Helper()
	ws := &m.Entities[0]
	min := mapfile.Vec3{X: math.MaxFloat64, Y: math.MaxFloat64, Z: math.MaxFloat64}
	max := mapfile.Vec3{X: -math.MaxFloat64, Y: -math.MaxFloat64, Z: -math.MaxFloat64}
	for i := range ws.Brushes {
		for _, f := range ws.Brushes[i].Faces {
			for _, p := range f.Points {
				min.X = math.Min(min.X, p.X)
				min.Y = math.Min(min.Y, p.Y)
				min.Z = math.Min(min.Z, p.Z)
				max.X = math.Max(max.X, p.X)
				max.Y = math.Max(max.Y, p.Y)
				max.Z = math.Max(max.Z, p.Z)
			}
		}
	}
	return min, max
}

func voxelIoU(t *testing.T, a, b map[[3]int]struct{}) float64 {
	t.Helper()
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		t.Fatal("both occupancy sets empty")
	}
	return float64(inter) / float64(union)
}

func TestDecompileGoldenVoxelIoU(t *testing.T) {
	_, data := compileFixture(t, roomMap())
	out, stats, err := Decompile(data, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		t.Fatalf("Decompile: %v", err)
	}
	if len(stats) == 0 || stats[0].Brushes == 0 {
		t.Fatalf("stats = %+v", stats)
	}
	if stats[0].LeavesSolid == 0 || stats[0].PlanesUsed == 0 {
		t.Fatalf("stats not populated: %+v", stats[0])
	}
	orig, err := qbsp.ParseMap(strings.NewReader(roomMap()))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	// Compare solid-vs-solid over the map's own region: the bspc-convention
	// seed box overhangs the model AABB by 8 units, which would otherwise
	// add a thin shell to the decompiled side and drag IoU to ~0.84.
	omin, omax := mapBounds(t, orig)
	iou := voxelIoU(t, voxelOccupancy(t, orig, 8, omin, omax), voxelOccupancy(t, out, 8, omin, omax))
	// Below 0.95 means a treewalk/texturing bug. Investigate; do not relax.
	if iou < 0.95 {
		t.Fatalf("voxel IoU vs original = %v, want >= 0.95", iou)
	}
}

func TestDecompileSelfCheckOnOutput(t *testing.T) {
	_, data := compileFixture(t, roomWithDoorMap())
	out, _, err := Decompile(data, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		t.Fatalf("Decompile: %v", err)
	}
	if err := SelfCheck(out); err != nil {
		t.Fatalf("SelfCheck: %v", err)
	}
}

func TestValidateBrushRejectsDegenerate(t *testing.T) {
	bad := mapfile.MapBrush{Faces: []mapfile.MapFace{
		{Points: [3]mapfile.Vec3{vc(0, 0, 0), vc(1, 0, 0), vc(2, 0, 0)}}, // collinear
		{Points: [3]mapfile.Vec3{vc(0, 0, 0), vc(0, 1, 0), vc(0, 0, 1)}},
		{Points: [3]mapfile.Vec3{vc(1, 0, 0), vc(0, 1, 0), vc(0, 0, 1)}},
		{Points: [3]mapfile.Vec3{vc(0, 0, 0), vc(1, 1, 0), vc(0, 0, 1)}},
	}}
	if err := ValidateBrush(&bad); err == nil {
		t.Fatal("expected degenerate-plane rejection")
	}
}

func roomWithThinTrimMap() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		slabBox(-64, -64, -64, 0, 320, 256, "wwall") +
		slabBox(256, -64, -64, 320, 320, 256, "wwall") +
		slabBox(0, -64, -64, 256, 0, 256, "wwall") +
		slabBox(0, 256, -64, 256, 320, 256, "wwall") +
		slabBox(0, 0, -64, 256, 256, -24, "ffloor") +
		slabBox(64, 64, -24, 192, 192, -20, "ttrim") + // 4-unit thin trim on floor
		slabBox(0, 0, 192, 256, 256, 256, "cceil") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 64\"\n}\n"
}

func TestThinTrimSurvival(t *testing.T) {
	_, data := compileFixture(t, roomWithThinTrimMap())
	out, stats, err := Decompile(data, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		t.Fatalf("Decompile: %v", err)
	}
	if err := SelfCheck(out); err != nil {
		t.Fatalf("SelfCheck: %v", err)
	}
	// Verify that the thin trim brush survived decompilation (should be preserved
	// on adaptive sub-grid instead of being pruned as a 2-sided degenerate fragment).
	foundTrim := false
	for _, b := range out.Entities[0].Brushes {
		minZ, maxZ := 99999.0, -99999.0
		for _, f := range b.Faces {
			for _, p := range f.Points {
				if p.Z < minZ {
					minZ = p.Z
				}
				if p.Z > maxZ {
					maxZ = p.Z
				}
			}
		}
		if minZ == -24 && maxZ == -20 {
			foundTrim = true
			if len(b.Faces) != 6 {
				t.Fatalf("trim brush has %d faces, want 6", len(b.Faces))
			}
			break
		}
	}
	if !foundTrim {
		t.Fatalf("thin trim brush was dropped during decompilation (stats: %+v)", stats)
	}
}