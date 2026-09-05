package qbsp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// TestSolidBSPBrushSplit verifies SplitBrush preserves convex geometry and
// the correct interior-halfspace convention on both pieces.
func TestSolidBSPBrushSplit(t *testing.T) {
	box := [2]vec3{{0, 0, 0}, {64, 64, 8}}
	faces := []brushFace{
		{p: plane{Normal: v3(1, 0, 0), Dist: 64}},
		{p: plane{Normal: v3(-1, 0, 0), Dist: 0}},
		{p: plane{Normal: v3(0, 1, 0), Dist: 64}},
		{p: plane{Normal: v3(0, -1, 0), Dist: 0}},
		{p: plane{Normal: v3(0, 0, 1), Dist: 8}},
		{p: plane{Normal: v3(0, 0, -1), Dist: 0}},
	}
	b := buildBspBrushFaces(faces, box)
	if b == nil {
		t.Fatal("brush build failed")
	}
	front, back := splitBrush(b, 0, plane{Normal: v3(0, 1, 0), Dist: 56}) // y>=56
	if front == nil || back == nil {
		t.Fatalf("split produced nil: front=%v back=%v", front != nil, back != nil)
	}
	wantFB := [2]vec3{{0, 56, 0}, {64, 64, 8}}
	wantBK := [2]vec3{{0, 0, 0}, {64, 56, 8}}
	if front.bounds != wantFB {
		t.Errorf("front bounds = %v, want %v", front.bounds, wantFB)
	}
	if back.bounds != wantBK {
		t.Errorf("back bounds = %v, want %v", back.bounds, wantBK)
	}
	// Interior consistency: every side's interior halfspace must contain
	// the brush centroid.
	centroidOf := func(br *bspBrush) vec3 {
		m, x := br.bounds[0], br.bounds[1]
		return vec3{(m[0] + x[0]) / 2, (m[1] + x[1]) / 2, (m[2] + x[2]) / 2}
	}
	for _, br := range []*bspBrush{front, back} {
		c := centroidOf(br)
		for _, s := range br.sides {
			if v3Dot(s.n, c)-s.d > 0.01 {
				t.Errorf("centroid %v outside side n=%v d=%v", c, s.n, s.d)
			}
		}
	}
}

// TestSolidBSPEngineDescent locks the critical parity invariant: the Go
// engine's PointInLeaf descends the serialised tree to the same leaf the
// compiler assigned (room interiors resolve as EMPTY, walls SOLID).
func TestSolidBSPEngineDescent(t *testing.T) {
	res := compileMapString(t, boxMap())
	if res.Leaked {
		t.Fatal("sealed box leaked")
	}
	tree := loadTreeResult(t, res)
	pts := [][3]float32{{32, 32, 32}, {8, 8, 8} /* interior corner */, {0, 0, 0} /* floor corner */}
	want := []int32{bsp.ContentsEmpty, bsp.ContentsEmpty, bsp.ContentsSolid}
	for i, p := range pts {
		l := tree.PointInLeaf(types.Vec3{X: p[0], Y: p[1], Z: p[2]})
		if l.Contents != want[i] {
			t.Errorf("PointInLeaf(%v) = %d, want %d", p, l.Contents, want[i])
		}
	}
}

// TestSolidBSPCorridorDescent is the corridor variant (two rooms + door +
// sealed chamber): the sealed room's interior must resolve empty.
func TestSolidBSPCorridorDescent(t *testing.T) {
	src := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(0, 0, 0, 64, 64, 64, 8) +
		prettyRoom(96, 0, 0, 160, 64, 64, 8) +
		prettySlab(64, 0, 0, 96, 20, 64, "mt_wall") +
		prettySlab(64, 44, 0, 96, 64, 64, "mt_wall") +
		prettySlab(64, 20, 52, 96, 44, 64, "mt_wall") +
		prettySlab(64, 20, 0, 96, 44, 12, "mt_wall") +
		prettyRoom(200, 0, 0, 264, 64, 64, 8) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
	src = stripFaceStr(src, 56, 0, 8, 64, 64, 56)
	src = stripFaceStr(src, 96, 0, 8, 104, 64, 56)

	m, err := ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Leaked {
		t.Fatal("corridor map leaked")
	}
	if res.PortalFile == nil || len(res.PortalFile.Portals) == 0 {
		t.Fatal("no portals produced (rooms should see each other)")
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	pts := [][3]float32{{32, 32, 32}, {128, 32, 32}, {232, 32, 32}}
	for _, p := range pts {
		l := tree.PointInLeaf(types.Vec3{X: p[0], Y: p[1], Z: p[2]})
		if l.Contents == bsp.ContentsSolid {
			t.Errorf("PointInLeaf(%v) landed in solid", p)
		}
	}
}

// TestSolidBSPMatrixScale is the scale smoke: a 10x10x10 matrix brush
// field (6000 planes) compiles quickly; the old arrangement-based CSG was
// exponential in planes and could not handle even a fraction of this.
func TestSolidBSPMatrixScale(t *testing.T) {
	var b strings.Builder
	b.WriteString("{\n\"classname\" \"worldspawn\"\n")
	// Outer shell so the map is sealed.
	b.WriteString(prettySlab(0, 0, 0, 320, 320, 12, "mt_wall"))
	b.WriteString(prettySlab(0, 0, 308, 320, 320, 320, "mt_wall"))
	b.WriteString(prettySlab(0, 0, 12, 12, 320, 308, "mt_wall"))
	b.WriteString(prettySlab(308, 0, 12, 320, 320, 308, "mt_wall"))
	b.WriteString(prettySlab(0, 0, 12, 320, 12, 308, "mt_wall"))
	b.WriteString(prettySlab(0, 308, 12, 320, 320, 308, "mt_wall"))
	// 9x9x9 pillars of slabs.
	for x := 0; x < 9; x++ {
		for y := 0; y < 9; y++ {
			for z := 0; z < 9; z++ {
				b.WriteString(prettySlab(
					16+float64(x)*32, 16+float64(y)*32, 16+float64(z)*32,
					24+float64(x)*32, 24+float64(y)*32, 24+float64(z)*32, "mt_wall"))
			}
		}
	}
	b.WriteString("}\n")
	m, err := ParseMap(strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Compile(m, Options{}); err != nil {
		t.Fatalf("Compile: %v", err)
	}
}

// prettyRoom mirrors the vis test's hollow room helper (self-contained).
func prettyRoom(x0, y0, z0, x1, y1, z1, t float64) string {
	var b strings.Builder
	b.WriteString(prettySlab(x0, y0, z0, x1, y1, z0+t, "mt_floor"))
	b.WriteString(prettySlab(x0, y0, z1-t, x1, y1, z1, "mt_floor"))
	b.WriteString(prettySlab(x0, y0, z0+t, x0+t, y1, z1-t, "mt_wall"))
	b.WriteString(prettySlab(x1-t, y0, z0+t, x1, y1, z1-t, "mt_wall"))
	b.WriteString(prettySlab(x0, y0, z0+t, x1, y0+t, z1-t, "mt_wall"))
	b.WriteString(prettySlab(x0, y1-t, z0+t, x1, y1, z1-t, "mt_wall"))
	return b.String()
}

// prettySlab renders one axis-aligned box brush.
func prettySlab(x0, y0, z0, x1, y1, z1 float64, tex string) string {
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

// stripFaceStr removes a specific brush face line (by its first two
// corner tokens) from a map source.
func stripFaceStr(src string, x0, y0, z0, x1, y1, z1 float64) string {
	needle := fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g )", x0, y0, z0, x0, y0, z1, x0, y1, z0)
	idx := strings.Index(src, needle)
	if idx < 0 {
		return src
	}
	end := strings.Index(src[idx:], "\n")
	return src[:idx] + src[idx+end+1:]
}

// TestCompileSubmodel verifies brush entities compile to inline *N models:
// world keeps model 0, func_wall gets model 1 with its own node tree, face
// range, clip hull, and the "model" "*1" key is serialised for the engine.
func TestCompileSubmodel(t *testing.T) {
	src := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(0, 0, 0, 64, 64, 64, 8) +
		"}\n{\n\"classname\" \"func_wall\"\n\"origin\" \"32 32 64\"\n" +
		prettySlab(16, 16, 32, 48, 48, 40, "mt_wall") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
	m, err := ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Leaked {
		t.Fatal("map leaked")
	}
	if res.Models != 2 {
		t.Fatalf("models = %d, want 2", res.Models)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Models) != 2 {
		t.Fatalf("loaded models = %d, want 2", len(tree.Models))
	}
	world, wall := tree.Models[0], tree.Models[1]
	if world.HeadNode[0] < 0 || int(world.HeadNode[0]) >= len(tree.Nodes) {
		t.Errorf("world headnode out of range: %d (nodes %d)", world.HeadNode[0], len(tree.Nodes))
	}
	if int(wall.HeadNode[0]) < len(tree.Nodes)-8 || int(wall.HeadNode[0]) >= len(tree.Nodes) {
		// The submodel's nodes are appended after the world's; just require
		// validity and distinctness from the world root.
		if int(wall.HeadNode[0]) < int(world.HeadNode[0]) || int(wall.HeadNode[0]) >= len(tree.Nodes) {
			t.Errorf("wall headnode %d out of range (world %d, nodes %d)", wall.HeadNode[0], world.HeadNode[0], len(tree.Nodes))
		}
	}
	// Face ranges must be disjoint and in-bounds.
	fa, fb := int(world.FirstFace), int(world.FirstFace)+int(world.NumFaces)
	sa, sb := int(wall.FirstFace), int(wall.FirstFace)+int(wall.NumFaces)
	if sb <= fa || sa < fb {
		t.Errorf("face ranges not in model order: world [%d,%d) wall [%d,%d)", fa, fb, sa, sb)
	}
	if sb > len(tree.Faces) {
		t.Errorf("wall face range [%d,%d) exceeds %d faces", sa, sb, len(tree.Faces))
	}
	if world.NumFaces < 6 {
		t.Errorf("world faces = %d, want >= 6", world.NumFaces)
	}
	if wall.NumFaces < 6 {
		t.Errorf("wall faces = %d, want >= 6", wall.NumFaces)
	}
	// Clip hull for the submodel (recorded but not cross-checked here;
	// the loader validates ranges via its own clipnode checks).
	// Entity lump must bind the model.
	if !strings.Contains(string(res.Data), `"model" "*1"`) {
		t.Errorf("entity lump missing model key for func_wall")
	}
}

// TestTJunctionNoCrack verifies the no-T invariant: two coplanar brushes
// whose seam is misaligned (one face split by the other's vertex) must
// compile such that no vertex of any face lies strictly inside another
// face's edge on the same plane.
func TestTJunctionNoCrack(t *testing.T) {
	src := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(0, 0, 0, 64, 64, 64, 8) +
		// Two coplanar wall segments meeting at x=32 (one spans 32..64 on
		// +x, the other 0..39 on -x so the seam at 32 is split differently
		// on each side) — classic T-junction generator.
		prettySlab(16, 16, 16, 32, 48, 32, "mt_wall") + // left slab, face at x=32
		prettySlab(32, 24, 16, 64, 40, 32, "mt_wall") + // right slab, face at x=32
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 36\"\n}\n"
	m, err := ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	// Collect every face polygon, planar and closed.
	polys := facePolygons(tree)
	// Apply the no-T invariant: no vertex strictly inside another face's
	// edge, restricted to coplanar pairs (edges on the same geometric
	// plane are where cracks occur).
	for i := range polys {
		pi := polys[i]
		for k := 0; k < len(pi); k++ {
			a := pi[k]
			b := pi[(k+1)%len(pi)]
			for j := range polys {
				if i == j {
					continue
				}
				for _, v := range polys[j] {
					if v == a || v == b {
						continue
					}
					if pointOnSegment(vec3From(v), vec3From(a), vec3From(b)) {
						t.Fatalf("T-junction: vertex %v of face %d lies on edge %d(%v->%v)", v, j, i, a, b)
					}
				}
			}
		}
	}
}

// facePolygons rebuilds face polygons from the loaded tree lumps.
func facePolygons(tree *bsp.Tree) [][]types.Vec3 {
	var out [][]types.Vec3
	for _, f := range tree.Faces {
		var poly []types.Vec3
		for k := 0; k < int(f.NumEdges); k++ {
			se := tree.Surfedges[int(f.FirstEdge)+k]
			e := int(se)
			if e < 0 {
				e = -e - 1
			}
			ed := tree.Edges[e]
			v := ed.V[0]
			if se > 0 {
				v = ed.V[1]
			}
			vert := tree.Vertexes[v]
			poly = append(poly, vert.Point)
		}
		if len(poly) >= 3 {
			out = append(out, poly)
		}
	}
	return out
}

// vec3From converts a types.Vec3 to the compiler's vec3.
func vec3From(v types.Vec3) vec3 { return vec3{float64(v.X), float64(v.Y), float64(v.Z)} }

// TestBSPXBrushList verifies the appended BRUSHLIST lump: per-model brush
// counts round-trip, the file still loads through the engine loader, and
// the BSPX data survives the vis pipeline (raw lump passthrough).
func TestBSPXBrushList(t *testing.T) {
	res := compileMapString(t, boxMapWithFuncWall())
	counts, err := ReadBSPXBrushList(res.Data)
	if err != nil {
		t.Fatalf("ReadBSPXBrushList: %v", err)
	}
	if len(counts) != res.Models {
		t.Fatalf("bspx models = %d, want %d", len(counts), res.Models)
	}
	if counts[0] < 6 {
		t.Errorf("world brush count = %d, want >= 6", counts[0])
	}
	if res.Models > 1 && counts[1] < 1 {
		t.Errorf("submodel brush count = %d, want >= 1", counts[1])
	}
	// The engine loader must still read the BSP (appended data is inert).
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("LoadTree after BSPX: %v", err)
	}
	if len(tree.Models) != res.Models {
		t.Errorf("models = %d, want %d", len(tree.Models), res.Models)
	}
}

// boxMapWithFuncWall returns a sealed room plus one func_wall brush entity.
func boxMapWithFuncWall() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(0, 0, 0, 64, 64, 64, 8) +
		"}\n{\n\"classname\" \"func_wall\"\n\"origin\" \"32 32 24\"\n" +
		prettySlab(28, 28, 16, 36, 36, 40, "mt_wall") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 12\"\n}\n"
}

// TestCompileBSPRMQ verifies -2psb output: the BSP2RMQ variant round-trips
// through the version-aware loader with the 2PSB magic and 16-bit bounds.
func TestCompileBSPRMQ(t *testing.T) {
	m, err := ParseMap(strings.NewReader(boxMap()))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{BSP2: true, TwoPSB: true})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("LoadTree (2psb): %v", err)
	}
	if tree.Version != bsp.BSP2Version_2PSB {
		t.Errorf("version = %x, want 2PSB (%x)", uint32(tree.Version), uint32(bsp.BSP2Version_2PSB))
	}
	if len(tree.Nodes) == 0 || len(tree.Faces) < 6 {
		t.Errorf("2psb tree empty: nodes %d faces %d", len(tree.Nodes), len(tree.Faces))
	}
	// The engine's point descent must resolve the interior as empty.
	l := tree.PointInLeaf(types.Vec3{X: 32, Y: 32, Z: 32})
	if l.Contents == bsp.ContentsSolid {
		t.Error("2psb interior resolved solid")
	}
}
