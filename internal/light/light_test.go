package light

import (
	"bytes"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// floorFace is a 64x64 floor at z=0 with identity texinfo vectors.
func floorFace() Face {
	return Face{
		Index: 0,
		Poly: [][3]float64{
			{0, 0, 0}, {64, 0, 0}, {64, 64, 0}, {0, 64, 0},
		},
		Vecs: [2][4]float64{
			{1, 0, 0, 0},
			{0, 1, 0, 0},
		},
		Normal: [3]float64{0, 0, 1},
	}
}

func TestCalcExtents(t *testing.T) {
	e := CalcExtents(floorFace().Poly, floorFace().Vecs)
	if e.W != 5 || e.H != 5 {
		t.Fatalf("extents = %dx%d, want 5x5 (64 units / 16 + 1)", e.W, e.H)
	}
	if e.Mins[0] != 0 || e.Mins[1] != 0 || e.Maxs[0] != 64 || e.Maxs[1] != 64 {
		t.Errorf("extents mins/maxs = %v/%v, want 0/64", e.Mins, e.Maxs)
	}
}

func TestDirectLightFalloff(t *testing.T) {
	face := floorFace()
	// Light directly above the centre luxel (s=40, t=40), 8 units up.
	lights := []Light{{Origin: [3]float64{40, 40, 8}, Value: 1000}}
	res := Bake([]Face{face}, lights, nil)
	if res.LightOfs[0] < 0 {
		t.Fatal("face not lit")
	}
	samples := res.Lighting[res.LightOfs[0]:]
	// Centre sample (i=2,j=2) is directly under the light.
	center := samples[2*5+2]
	// Corner sample (i=0,j=0) is farther and grazing.
	corner := samples[0]
	if center <= corner {
		t.Errorf("centre sample %d should exceed corner %d (falloff)", center, corner)
	}
	if center == 0 {
		t.Error("centre sample is 0 (light should reach it)")
	}
}

func TestShadowBlocksLight(t *testing.T) {
	face := floorFace()
	lights := []Light{{Origin: [3]float64{40, 40, 8}, Value: 1000}}
	// Shadow everything: a trace that always reports blocked.
	blocked := Bake([]Face{face}, lights, func(from, to [3]float64) bool { return true })
	if blocked.LightOfs[0] < 0 {
		t.Fatal("face not lit")
	}
	for _, s := range blocked.Lighting[blocked.LightOfs[0]:] {
		if s != 0 {
			t.Errorf("shadowed sample = %d, want 0", s)
		}
	}
}

func TestWriteLitRoundTrip(t *testing.T) {
	face := floorFace()
	lights := []Light{{Origin: [3]float64{40, 40, 8}, Value: 1000}}
	res := Bake([]Face{face}, lights, nil)
	lit := WriteLit(res.Lighting)

	// ApplyLitFile must accept it against the mono lighting lump.
	tree := &bsp.Tree{Lighting: res.Lighting}
	if err := bsp.ApplyLitFile(tree, lit); err != nil {
		t.Fatalf("ApplyLitFile: %v", err)
	}
	if !tree.LightingRGB {
		t.Error("LightingRGB not set")
	}
	if !bytes.Equal(tree.Lighting, lit[8:]) {
		t.Error("lit RGB data differs from the sidecar payload")
	}
}

func TestSkyFaceUnlit(t *testing.T) {
	face := floorFace()
	face.Sky = true
	res := Bake([]Face{face}, []Light{{Origin: [3]float64{40, 40, 8}, Value: 1000}}, nil)
	if res.LightOfs[0] != -1 {
		t.Error("sky face should have no lightmap offset")
	}
}
// compileBoxWithLight builds a hollow box with a light entity and returns
// the compiled BSP bytes.
func compileBoxWithLight(t *testing.T) []byte {
	t.Helper()
	src := `{
"classname" "worldspawn"
{
( 64 0 0 ) ( 64 0 8 ) ( 64 64 0 ) mt_floor 0 0 0 1 1
( 0 64 0 ) ( 0 64 8 ) ( 0 0 8 ) mt_floor 0 0 0 1 1
( 0 64 0 ) ( 64 64 0 ) ( 0 64 8 ) mt_floor 0 0 0 1 1
( 0 0 0 ) ( 0 0 8 ) ( 64 0 0 ) mt_floor 0 0 0 1 1
( 0 0 8 ) ( 0 64 8 ) ( 64 0 8 ) mt_floor 0 0 0 1 1
( 0 0 0 ) ( 64 0 0 ) ( 0 64 0 ) mt_floor 0 0 0 1 1
}
{
( 64 0 56 ) ( 64 0 64 ) ( 64 64 56 ) mt_floor 0 0 0 1 1
( 0 64 56 ) ( 0 64 64 ) ( 0 0 64 ) mt_floor 0 0 0 1 1
( 0 64 56 ) ( 64 64 56 ) ( 0 64 64 ) mt_floor 0 0 0 1 1
( 0 0 56 ) ( 0 0 64 ) ( 64 0 56 ) mt_floor 0 0 0 1 1
( 0 0 64 ) ( 0 64 64 ) ( 64 0 64 ) mt_floor 0 0 0 1 1
( 0 0 56 ) ( 64 0 56 ) ( 0 64 56 ) mt_floor 0 0 0 1 1
}
{
( 8 0 8 ) ( 8 0 56 ) ( 8 64 8 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 0 64 56 ) ( 0 8 56 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 8 64 8 ) ( 0 64 56 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 0 8 56 ) ( 8 8 8 ) mt_wall 0 0 0 1 1
( 0 8 56 ) ( 0 64 56 ) ( 8 8 56 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 8 8 8 ) ( 0 64 8 ) mt_wall 0 0 0 1 1
}
{
( 64 0 8 ) ( 64 0 56 ) ( 64 64 8 ) mt_wall 0 0 0 1 1
( 56 64 8 ) ( 56 64 56 ) ( 56 8 56 ) mt_wall 0 0 0 1 1
( 56 64 8 ) ( 64 64 8 ) ( 56 64 56 ) mt_wall 0 0 0 1 1
( 56 8 8 ) ( 56 8 56 ) ( 64 8 8 ) mt_wall 0 0 0 1 1
( 56 8 56 ) ( 56 64 56 ) ( 64 8 56 ) mt_wall 0 0 0 1 1
( 56 8 8 ) ( 64 8 8 ) ( 56 64 8 ) mt_wall 0 0 0 1 1
}
{
( 64 8 8 ) ( 64 8 56 ) ( 64 0 8 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 0 8 56 ) ( 56 8 8 ) mt_wall 0 0 0 1 1
( 0 8 8 ) ( 64 8 8 ) ( 0 8 56 ) mt_wall 0 0 0 1 1
( 0 0 8 ) ( 0 0 56 ) ( 56 0 8 ) mt_wall 0 0 0 1 1
( 0 0 56 ) ( 0 8 56 ) ( 56 0 56 ) mt_wall 0 0 0 1 1
( 0 0 8 ) ( 56 0 8 ) ( 0 8 8 ) mt_wall 0 0 0 1 1
}
{
( 64 64 8 ) ( 64 64 56 ) ( 64 56 8 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 0 64 56 ) ( 56 64 8 ) mt_wall 0 0 0 1 1
( 0 64 8 ) ( 64 64 8 ) ( 0 64 56 ) mt_wall 0 0 0 1 1
( 0 56 8 ) ( 0 56 56 ) ( 56 56 8 ) mt_wall 0 0 0 1 1
( 0 56 56 ) ( 0 64 56 ) ( 56 56 56 ) mt_wall 0 0 0 1 1
( 0 56 8 ) ( 56 56 8 ) ( 0 64 8 ) mt_wall 0 0 0 1 1
}
}
{
"classname" "info_player_start"
"origin" "32 32 32"
}
{
"classname" "light"
"origin" "32 32 48"
"light" "1000"
}
`
	m, err := qbsp.ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Data
}

func TestBSPIntegrationBake(t *testing.T) {
	data := compileBoxWithLight(t)
	faces, err := ParseFaces(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) == 0 {
		t.Fatal("no faces parsed")
	}
	lights, err := ParseLights(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(lights) != 1 {
		t.Fatalf("parsed %d lights, want 1", len(lights))
	}
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	res := Bake(faces, lights, TreeTracer(tree))
	// The floor face (normal +z) should be lit.
	litFaces := 0
	for i := range faces {
		if res.LightOfs[i] >= 0 {
			litFaces++
		}
	}
	if litFaces == 0 {
		t.Fatal("no faces lit")
	}
	// The .lit sidecar must satisfy ApplyLitFile against the mono lump.
	tree.Lighting = res.Lighting
	if err := bsp.ApplyLitFile(tree, WriteLit(res.Lighting)); err != nil {
		t.Fatalf("ApplyLitFile: %v", err)
	}
}
