package qbsp

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func parseMapString(t *testing.T, src string) *Map {
	t.Helper()
	m, err := ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseMap: %v\nmap:\n%s", err, src)
	}
	return m
}

func near(t *testing.T, got, want float64, what string) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func vecNear(t *testing.T, got, want vec3, what string) {
	t.Helper()
	for i := 0; i < 3; i++ {
		near(t, got[i], want[i], what)
	}
}

// slabBrush renders one axis-aligned box brush (a solid volume, with
// outward-faces convention verified by TestBrushPlaneNormalsPointOutward).
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
// walls) around an empty interior [x0+t..x1-t] on every axis, which is how
// Quake maps actually enclose space. Wall thickness t (8).
func hollowRoom(x0, y0, z0, x1, y1, z1 float64, t float64) string {
	var b strings.Builder
	b.WriteString(slabBrush(x0, y0, z0, x1, y1, z0+t, "mt_floor"))     // floor
	b.WriteString(slabBrush(x0, y0, z1-t, x1, y1, z1, "mt_floor"))    // ceiling
	b.WriteString(slabBrush(x0, y0, z0+t, x0+t, y1, z1-t, "mt_wall")) // west
	b.WriteString(slabBrush(x1-t, y0, z0+t, x1, y1, z1-t, "mt_wall")) // east
	b.WriteString(slabBrush(x0, y0, z0+t, x1, y0+t, z1-t, "mt_wall")) // north
	b.WriteString(slabBrush(x0, y1-t, z0+t, x1, y1, z1-t, "mt_wall")) // south
	return b.String()
}

// boxMap returns a sealed hollow room plus a player start inside it.
func boxMap() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		hollowRoom(0, 0, 0, 64, 64, 64, 8) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n\"angle\" \"90\"\n}\n"
}

func TestParseBoxMap(t *testing.T) {
	m := parseMapString(t, boxMap())
	if len(m.Entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(m.Entities))
	}
	world := m.Entities[0]
	if len(world.Brushes) != 6 {
		t.Fatalf("world brushes = %d, want 6 (hollow room slabs)", len(world.Brushes))
	}
	for i, b := range world.Brushes {
		if len(b.Faces) != 6 {
			t.Fatalf("brush %d faces = %d, want 6", i, len(b.Faces))
		}
	}

	player := m.Entities[1]
	origin, ok := player.Value("origin")
	if !ok || origin != "32 32 32" {
		t.Errorf("player origin = %q, want %q", origin, "32 32 32")
	}
	if len(player.Brushes) != 0 {
		t.Errorf("player entity has %d brushes, want 0", len(player.Brushes))
	}
}

func TestBrushPlaneNormalsPointOutward(t *testing.T) {
	src := "{\n\"classname\" \"worldspawn\"\n" + slabBrush(0, 0, 0, 64, 64, 8, "mt_wall") + "}\n"
	m := parseMapString(t, src)
	faces := m.Entities[0].Brushes[0].Faces
	want := []vec3{
		v3(1, 0, 0),  // +x at 64
		v3(-1, 0, 0), // -x at 0
		v3(0, 1, 0),  // +y
		v3(0, -1, 0), // -y
		v3(0, 0, 1),  // +z at 8
		v3(0, 0, -1), // -z at 0
	}
	wantDist := []float64{64, 0, 64, 0, 8, 0}
	for i, f := range faces {
		vecNear(t, f.Normal, want[i], "face normal")
		near(t, f.Dist, wantDist[i], "face dist")
	}
}

func TestTexInfoQuakeEdBaseaxis(t *testing.T) {
	// +z face: QuakeEd floor axis -> xv {1,0,0}, yv {0,-1,0}, scale 1.
	src := "{\n\"classname\" \"worldspawn\"\n" + slabBrush(0, 0, 0, 64, 64, 8, "mt_wall") + "}\n"
	m := parseMapString(t, src)
	faces := m.Entities[0].Brushes[0].Faces
	top := faces[4] // +z face of the slab
	for i := 0; i < 3; i++ {
		near(t, top.Vecs[0][i], []float64{1, 0, 0}[i], "s vector")
		near(t, top.Vecs[1][i], []float64{0, -1, 0}[i], "t vector")
	}
	near(t, top.Vecs[0][3], 0, "s offset")
	near(t, top.Vecs[1][3], 0, "t offset")

	// Rotated + scaled face via a QuakeEd texdef.
	rotateSrc := `{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 1 0 ) ( 1 0 0 ) rwall 10 20 90 2 4
}
}
`
	m2 := parseMapString(t, rotateSrc)
	f := m2.Entities[0].Brushes[0].Faces[0]
	// floor axis xv={1,0,0} (sv=0), yv={0,-1,0} (tv=1); rotate 90:
	// vectors[0] -> (0,1,0); vectors[1] -> (1,0,0).
	// scale: s=(0/2, 1/2, 0) = (0,0.5,0); t=(1/4, 0, 0) = (0.25,0,0)
	vecNear(t, v3(f.Vecs[0][0], f.Vecs[0][1], f.Vecs[0][2]), v3(0, 0.5, 0), "rotated s vector")
	vecNear(t, v3(f.Vecs[1][0], f.Vecs[1][1], f.Vecs[1][2]), v3(0.25, 0, 0), "rotated t vector")
	near(t, f.Vecs[0][3], 10, "s offset")
	near(t, f.Vecs[1][3], 20, "t offset")
}

func TestValve220Face(t *testing.T) {
	src := `{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 0 1 ) ( 0 1 0 ) mt_wall [ 0 1 0 -8 ] [ 1 0 0 16 ] 45 2 2
}
}
`
	m := parseMapString(t, src)
	f := m.Entities[0].Brushes[0].Faces[0]
	if f.Tex.QuakeEd {
		t.Error("expected Valve 220 texdef, got QuakeEd")
	}
	vecNear(t, f.Tex.Axis[0], v3(0, 1, 0), "valve u axis")
	vecNear(t, f.Tex.Axis[1], v3(1, 0, 0), "valve v axis")
	near(t, f.Tex.ShiftX, -8, "u offset")
	near(t, f.Tex.ShiftY, 16, "v offset")
	near(t, f.Tex.Rotate, 45, "rotate")
	// vecs: axis/scale
	vecNear(t, v3(f.Vecs[0][0], f.Vecs[0][1], f.Vecs[0][2]), v3(0, 0.5, 0), "valve s vector")
	vecNear(t, v3(f.Vecs[1][0], f.Vecs[1][1], f.Vecs[1][2]), v3(0.5, 0, 0), "valve t vector")
	near(t, f.Vecs[0][3], -8, "valve s offset")
	near(t, f.Vecs[1][3], 16, "valve t offset")
}

func TestMixedQuakeEdAndValveFaces(t *testing.T) {
	src := `{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 1 0 ) ( 1 0 0 ) floor 0 0 0 1 1
( 0 0 0 ) ( 0 0 1 ) ( 0 1 0 ) valvey [ 0 1 0 0 ] [ 1 0 0 0 ] 0 1 1
}
}
`
	m := parseMapString(t, src)
	faces := m.Entities[0].Brushes[0].Faces
	if len(faces) != 2 {
		t.Fatalf("faces = %d, want 2", len(faces))
	}
	if !faces[0].Tex.QuakeEd {
		t.Error("face 0 should be QuakeEd")
	}
	if faces[1].Tex.QuakeEd {
		t.Error("face 1 should be Valve 220")
	}
}

func TestEntityDictParsing(t *testing.T) {
	src := `{
"classname" "worldspawn"
"message" "Level 1: The Door To Hell"
"model" "*1"
"sounds" "1"
" wad " "some/path.wad"
}
`
	m := parseMapString(t, src)
	ent := m.Entities[0]
	cases := []struct{ key, want string }{
		{"classname", "worldspawn"},
		{"message", "Level 1: The Door To Hell"},
		{"model", "*1"},
		{"sounds", "1"},
		{"wad", "some/path.wad"}, // key whitespace trimmed
	}
	for _, c := range cases {
		got, ok := ent.Value(c.key)
		if !ok || got != c.want {
			t.Errorf("epair %q = %q (found %v), want %q", c.key, got, ok, c.want)
		}
	}
	if len(ent.Brushes) != 0 {
		t.Errorf("entity has %d brushes, want 0", len(ent.Brushes))
	}
}

func TestDuplicatePlaneSkipped(t *testing.T) {
	src := `{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 0 1 ) ( 0 1 0 ) mt_wall 0 0 0 1 1
( 0 0 0 ) ( 0 0 1 ) ( 0 1 0 ) mt_wall 0 0 0 1 1
( 0 0 0 ) ( 0 1 0 ) ( 1 0 0 ) mt_floor 0 0 0 1 1
( 0 0 0 ) ( 0 1 0 ) ( 1 0 0 ) mt_floor 0 0 0 1 1
}
}
`
	m := parseMapString(t, src)
	if faces := len(m.Entities[0].Brushes[0].Faces); faces != 2 {
		t.Errorf("faces = %d, want 2 (duplicates dropped)", faces)
	}
}

func TestDegenerateFaceSkipped(t *testing.T) {
	src := `{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 0 0 ) ( 0 0 0 ) mt_wall 0 0 0 1 1
( 0 0 0 ) ( 0 0 1 ) ( 0 1 0 ) mt_wall 0 0 0 1 1
( 0 0 0 ) ( 0 1 0 ) ( 1 0 0 ) mt_floor 0 0 0 1 1
}
}
`
	m := parseMapString(t, src)
	if faces := len(m.Entities[0].Brushes[0].Faces); faces != 2 {
		t.Errorf("faces = %d, want 2 (degenerate dropped)", faces)
	}
}

func TestQuake2ExtendedInfoConsumed(t *testing.T) {
	src := `{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 0 1 ) ( 0 1 0 ) mt_wall 0 0 0 1 1 33 8 1
( 0 0 0 ) ( 0 1 0 ) ( 1 0 0 ) mt_floor 0 0 0 1 1
}
}
`
	m := parseMapString(t, src)
	faces := m.Entities[0].Brushes[0].Faces
	if len(faces) != 2 {
		t.Fatalf("faces = %d, want 2 (Q2 extras must not swallow the next face)", len(faces))
	}
	if faces[0].TexName != "mt_wall" || faces[1].TexName != "mt_floor" {
		t.Errorf("textures wrong after Q2 extras: %q, %q", faces[0].TexName, faces[1].TexName)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		name, src, wantErr string
	}{
		{"unterminated quoted", `{
"classname" "worldspawn
}
`, "EOF inside quoted token"},
		{"missing entity brace", `"classname" "worldspawn"
}
`, `invalid entity format`},
		{"bad plane", `{
"classname" "worldspawn"
{
( 0 0 0 ) ( 0 0 1 ) hello
}
}
`, "invalid brush plane format"},
		{"value on next line", `{
"classname"
"worldspawn"
}
`, "line is incomplete"},
		{"unclosed entity", `{
"classname" "worldspawn"
`, "unexpected EOF (no closing brace)"},
		{"brush primitives", `{
"classname" "worldspawn"
{
patchDef2
{
( 3 3 16 16 0 0 0 )
( ( 1 0 0 ) ( 0 1 0 ) ( 0 0 1 ) ( 0 0 0 ) )
}
}
`, "brush primitives format not supported"},
	}
	for _, tc := range cases {
		_, err := ParseMap(strings.NewReader(tc.src))
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: err = %v, want substring %q", tc.name, err, tc.wantErr)
		}
	}
}

func TestParseMapCRLF(t *testing.T) {
	src := "{\r\n\"classname\" \"worldspawn\"\r\n{\r\n( 0 0 0 ) ( 0 0 1 ) ( 0 1 0 ) mt_wall 0 0 0 1 1\r\n}\r\n}\r\n"
	m := parseMapString(t, src)
	if len(m.Entities) != 1 || len(m.Entities[0].Brushes) != 1 {
		t.Fatalf("CRLF map parsed incorrectly: %d entities, %d brushes", len(m.Entities), len(m.Entities[0].Brushes))
	}
}

func TestPlaneFromPointsEpsilonOrder(t *testing.T) {
	// The normal is normalize(cross(p0-p1, p2-p1)); winding order matters.
	p0, p1, p2 := v3(64, 0, 0), v3(64, 0, 64), v3(64, 64, 0)
	pl, length := planeFromPoints(p0, p1, p2)
	if length == 0 {
		t.Fatal("degenerate plane")
	}
	vecNear(t, pl.Normal, v3(1, 0, 0), "normal")
	near(t, pl.Dist, 64, "dist")
	// Reverse winding flips the normal.
	pl2, _ := planeFromPoints(p0, p2, p1)
	vecNear(t, pl2.Normal, v3(-1, 0, 0), "reversed normal")
}

func TestQuakeEdAxisAllNormals(t *testing.T) {
	cases := []struct {
		normal vec3
		xv, yv vec3
	}{
		{v3(0, 0, 1), v3(1, 0, 0), v3(0, -1, 0)},  // floor
		{v3(0, 0, -1), v3(1, 0, 0), v3(0, -1, 0)}, // ceiling
		{v3(1, 0, 0), v3(0, 1, 0), v3(0, 0, -1)},  // west
		{v3(-1, 0, 0), v3(0, 1, 0), v3(0, 0, -1)}, // east
		{v3(0, 1, 0), v3(1, 0, 0), v3(0, 0, -1)},  // south
		{v3(0, -1, 0), v3(1, 0, 0), v3(0, 0, -1)}, // north
		{normalizedVec3(t, v3(1, 1, 0)), v3(0, 1, 0), v3(0, 0, -1)},
	}
	for _, c := range cases {
		xv, yv := quakeEdAxis(c.normal)
		vecNear(t, xv, c.xv, "axis xv")
		vecNear(t, yv, c.yv, "axis yv")
	}
}

func normalizedVec3(t *testing.T, v vec3) vec3 {
	t.Helper()
	n, _ := v3Normalize(v)
	if n == (vec3{}) {
		t.Fatal("zero normalize")
	}
	return n
}