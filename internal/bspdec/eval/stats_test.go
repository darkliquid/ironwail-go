package eval

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// fixtureMapPath resolves testdata/bspdec/room.map from this package.
func fixtureMapPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "..", "testdata", "bspdec", "room.map")
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	return p
}

// compileRoomFixture compiles the committed fixture and returns its BSP bytes.
func compileRoomFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(fixtureMapPath(t))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	m, err := mapfile.Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatal("fixture room leaks")
	}
	return res.Data
}

func roomOccupancy(t *testing.T, data []byte) map[[3]int]struct{} {
	t.Helper()
	occ, err := OccupancyMap(data, 8,
		mapfile.Vec3{X: -64, Y: -64, Z: -64},
		mapfile.Vec3{X: 320, Y: 320, Z: 256})
	if err != nil {
		t.Fatalf("OccupancyMap: %v", err)
	}
	return occ
}

func TestBSPStats(t *testing.T) {
	data := compileRoomFixture(t)
	st, err := BSPStats(data)
	if err != nil {
		t.Fatalf("BSPStats: %v", err)
	}
	if st.Faces == 0 || st.Planes == 0 || st.Leafs == 0 || st.Edges == 0 || st.Clipnodes == 0 {
		t.Fatalf("stats empty: %+v", st)
	}
}

func TestOccupancySelfIoU(t *testing.T) {
	data := compileRoomFixture(t)
	a := roomOccupancy(t, data)
	if len(a) == 0 {
		t.Fatal("empty occupancy")
	}
	if v := VoxelIoU(a, a); v < 0.999 {
		t.Fatalf("self IoU = %v", v)
	}
}

func TestOccupancyClampRespected(t *testing.T) {
	data := compileRoomFixture(t)
	full := roomOccupancy(t, data)
	occ, err := OccupancyMap(data, 8,
		mapfile.Vec3{X: -64, Y: -64, Z: -64},
		mapfile.Vec3{X: 320, Y: 320, Z: 0})
	if err != nil {
		t.Fatalf("OccupancyMap: %v", err)
	}
	if v := VoxelIoU(full, occ); v > 0.8 {
		t.Fatalf("floor-only IoU too high: %v — clamp broken", v)
	}
}

func TestPointInSolidOutsideIsEmpty(t *testing.T) {
	data := compileRoomFixture(t)
	tree := loadFixtureTree(t, data)
	// the interior cavity center is air
	if PointInSolid(tree, mapfile.Vec3{X: 128, Y: 128, Z: 64}) {
		t.Fatal("room center should be air")
	}
	// deep inside a wall is solid
	if !PointInSolid(tree, mapfile.Vec3{X: -32, Y: 128, Z: 64}) {
		t.Fatal("west wall interior should be solid")
	}
}

func loadFixtureTree(t *testing.T, data []byte) *bsp.Tree {
	t.Helper()
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	return tree
}