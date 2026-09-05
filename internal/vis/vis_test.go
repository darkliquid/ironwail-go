package vis

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// slabBrush renders one axis-aligned box brush (a solid volume) with the
// outward-winding convention verified by the qbsp tests.
func slabBrush(x0, y0, z0, x1, y1, z1 float64, tex string) string {
	format := func(p1, p2, p3 [3]float64) string {
		return fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g ) %s 0 0 0 1 1\n",
			p1[0], p1[1], p1[2], p2[0], p2[1], p2[2], p3[0], p3[1], p3[2], tex)
	}
	mins := [3]float64{x0, y0, z0}
	maxs := [3]float64{x1, y1, z1}
	return "{\n" +
		format([3]float64{maxs[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], maxs[2]}, [3]float64{maxs[0], maxs[1], mins[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{mins[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{maxs[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{mins[0], mins[1], maxs[2]}, [3]float64{maxs[0], mins[1], mins[2]}) +
		format([3]float64{mins[0], mins[1], maxs[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{maxs[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], mins[2]}, [3]float64{mins[0], maxs[1], mins[2]}) +
		"}\n"
}

// hollowRoom renders a room as six thin slab brushes (floor, ceiling, four
// walls) around an empty interior, which is how Quake maps enclose space.
func hollowRoom(x0, y0, z0, x1, y1, z1, t float64) string {
	var b strings.Builder
	b.WriteString(slabBrush(x0, y0, z0, x1, y1, z0+t, "mt_floor"))
	b.WriteString(slabBrush(x0, y0, z1-t, x1, y1, z1, "mt_floor"))
	b.WriteString(slabBrush(x0, y0, z0+t, x0+t, y1, z1-t, "mt_wall"))
	b.WriteString(slabBrush(x1-t, y0, z0+t, x1, y1, z1-t, "mt_wall"))
	b.WriteString(slabBrush(x0, y0, z0+t, x1, y0+t, z1-t, "mt_wall"))
	b.WriteString(slabBrush(x0, y1-t, z0+t, x1, y1, z1-t, "mt_wall"))
	return b.String()
}

// stripSlab removes the slab brush with the given bounds from the map text.
func stripSlab(src string, x0, y0, z0, x1, y1, z1 float64) string {
	block := slabBrush(x0, y0, z0, x1, y1, z1, "mt_wall")
	idx := strings.Index(src, block)
	if idx < 0 {
		return src
	}
	return src[:idx] + src[idx+len(block):]
}

// corridorPlusSealed builds a two-room corridor: each room is a hollow box
// opening onto a hollow door tunnel (four thin slab walls forming a passage
// y 20..44, z 12..52), plus a sealed hollow box that must never see the
// corridor. Returns the map source.
func corridorPlusSealed() string {
	var b strings.Builder
	b.WriteString("{\n\"classname\" \"worldspawn\"\n")
	b.WriteString(hollowRoom(0, 0, 0, 64, 64, 64, 8))
	b.WriteString(hollowRoom(96, 0, 0, 160, 64, 64, 8))
	b.WriteString(slabBrush(64, 0, 0, 96, 20, 64, "mt_wall"))
	b.WriteString(slabBrush(64, 44, 0, 96, 64, 64, "mt_wall"))
	b.WriteString(slabBrush(64, 20, 52, 96, 44, 64, "mt_wall"))
	b.WriteString(slabBrush(64, 20, 0, 96, 44, 12, "mt_wall"))
	b.WriteString(hollowRoom(200, 0, 0, 264, 64, 64, 8))
	src := b.String()
	// room1 opens on +x (x=64), room2 opens on -x (x=96): drop the facing
	// wall slabs.
	src = stripSlab(src, 56, 0, 8, 64, 64, 56)
	src = stripSlab(src, 96, 0, 8, 104, 64, 56)
	b.Reset()
	b.WriteString(src)
	b.WriteString("}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n")
	return b.String()
}

// compileVis runs qbsp + vis over a map source and returns the final BSP data.
func compileVis(t *testing.T, src string) []byte {
	t.Helper()
	m, err := qbsp.ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.PortalFile == nil || len(res.PortalFile.Serialize()) == 0 {
		t.Fatalf("no portal file produced (leak=%v)", res.Leaked)
	}
	out, err := Run(res.Data, res.PortalFile.Serialize())
	if err != nil {
		t.Fatalf("vis.Run: %v", err)
	}
	return out
}

func leafAt(t *testing.T, tree *bsp.Tree, x, y, z float32) *bsp.TreeLeaf {
	t.Helper()
	leaf := tree.PointInLeaf(types.Vec3{X: x, Y: y, Z: z})
	if leaf == nil {
		t.Fatalf("PointInLeaf(%v) nil", [3]float32{x, y, z})
	}
	return leaf
}

func leafIndex(t *testing.T, tree *bsp.Tree, want *bsp.TreeLeaf) int {
	t.Helper()
	for i := range tree.Leafs {
		if &tree.Leafs[i] == want {
			return i
		}
	}
	t.Fatalf("leaf not found")
	return -1
}

func TestVISCorridorPVS(t *testing.T) {
	data := compileVis(t, corridorPlusSealed())
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("LoadTree after vis: %v", err)
	}
	if len(tree.Visibility) == 0 {
		t.Fatal("visibility lump empty after vis")
	}

	room1 := leafAt(t, tree, 32, 32, 32)
	room2 := leafAt(t, tree, 128, 32, 32)
	sealed := leafAt(t, tree, 232, 32, 32)
	if room1.Contents == bsp.ContentsSolid || room2.Contents == bsp.ContentsSolid || sealed.Contents == bsp.ContentsSolid {
		t.Fatal("sample points landed in solid leaves")
	}

	i1 := leafIndex(t, tree, room1)
	i2 := leafIndex(t, tree, room2)
	is := leafIndex(t, tree, sealed)

	bit := func(pvs []byte, i int) bool {
		if i/8 >= len(pvs) {
			return false
		}
		return pvs[i/8]&(1<<uint(i%8)) != 0
	}
	if !bit(tree.LeafPVS(room1), i2) {
		t.Error("room1 does not see room2 through the door")
	}
	if bit(tree.LeafPVS(room1), is) {
		t.Error("room1 sees the sealed box (should be invisible)")
	}
	if !bit(tree.LeafPVS(room2), i1) {
		t.Error("room2 does not see room1")
	}
}

func TestVISPVSRowFormat(t *testing.T) {
	data := compileVis(t, corridorPlusSealed())
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	visLeafs := int(tree.Models[0].VisLeafs)
	if visLeafs == 0 {
		t.Fatal("world model visleafs is 0")
	}
	rowLen := (visLeafs + 7) / 8
	leafInt := 0
	for i := range tree.Leafs {
		leaf := &tree.Leafs[i]
		if leaf.Contents == bsp.ContentsSolid {
			continue
		}
		leafInt++
		got := tree.LeafPVS(leaf)
		if len(got) != rowLen {
			t.Fatalf("leaf %d PVS row = %d bytes, want %d (rows must cover leaf indices)", i, len(got), rowLen)
		}
	}
	if leafInt == 0 {
		t.Fatal("no non-solid leaves")
	}
}

func TestVISDeterministic(t *testing.T) {
	a := compileVis(t, corridorPlusSealed())
	b := compileVis(t, corridorPlusSealed())
	if !bytes.Equal(a, b) {
		t.Error("vis output is not deterministic")
	}
}

func TestCompressRowRoundTrip(t *testing.T) {
	rows := [][]byte{
		{1, 0, 3, 255},
		{0},
		{0xff, 0, 0, 0, 0, 0, 1, 0},
		make([]byte, 17),
	}
	for _, row := range rows {
		compressed := compressRow(row)
		decoded := decompressRow(compressed, len(row))
		if !bytes.Equal(decoded, row) {
			t.Errorf("round trip %v -> %v", row, decoded)
		}
	}
}

// decompressRow mirrors bsp.DecompressVis for the compressor test.
func decompressRow(in []byte, rowLen int) []byte {
	out := make([]byte, rowLen)
	outPos, inPos := 0, 0
	for outPos < rowLen && inPos < len(in) {
		if in[inPos] == 0 {
			inPos++
			if inPos >= len(in) {
				break
			}
			outPos += int(in[inPos])
			inPos++
			continue
		}
		out[outPos] = in[inPos]
		outPos++
		inPos++
	}
	return out
}
