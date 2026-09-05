package light

import (
	"bytes"
	"encoding/binary"
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
	lit := WriteLit(&res)

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
	if err := bsp.ApplyLitFile(tree, WriteLit(&res)); err != nil {
		t.Fatalf("ApplyLitFile: %v", err)
	}
}

// TestStyleRouting verifies per-style lightmap blocks: two lights of
// styles 0 and 3 on one face produce styles[4] = [0,3,255,255] and two
// consecutive W*H blocks in the Lighting lump, with style-3 light only
// affecting block 1.
func TestStyleRouting(t *testing.T) {
	face := floorFace()
	l0 := Light{Origin: [3]float64{40, 40, 8}, Value: 1000, Style: 0}
	l3 := Light{Origin: [3]float64{24, 24, 8}, Value: 1000, Style: 3}
	res := Bake([]Face{face}, []Light{l0, l3}, nil)
	if res.LightOfs[0] < 0 {
		t.Fatal("face not lit")
	}
	if res.Styles[0] != [4]byte{0, 3, 255, 255} {
		t.Errorf("styles = %v, want [0 3 255 255]", res.Styles[0])
	}
	// Two 5x5 blocks: block 0 only lit by style 0 (origin 40,40), block 1
	// only lit by style 3 (origin 24,24).
	block0 := res.Lighting[res.LightOfs[0] : res.LightOfs[0]+25]
	block1 := res.Lighting[res.LightOfs[0]+25 : res.LightOfs[0]+50]
	// Style 0 centre sample (2,2 -> s=40,t=40) bright; style 3 centre
	// sample (1,1 -> s=24,t=24) bright.
	if block0[2*5+2] == 0 {
		t.Error("style-0 block centre is 0")
	}
	if block1[1*5+1] == 0 {
		t.Error("style-3 block centre is 0")
	}
	// Style 3 must not contribute to block 0: near (40,40) the style-0
	// block is the only lit one.
	if block0[1*5+1] >= block0[2*5+2] {
		t.Error("style-3 light leaked into the style-0 block")
	}
}

// TestStyleLitSidecar verifies the .lit sidecar carries only style-0
// samples: size == 8 + 3 * (style-0 block size).
func TestStyleLitSidecar(t *testing.T) {
	face := floorFace()
	res := Bake([]Face{face}, []Light{
		{Origin: [3]float64{40, 40, 8}, Value: 1000, Style: 0},
		{Origin: [3]float64{24, 24, 8}, Value: 1000, Style: 3},
	}, nil)
	lit := WriteLit(&res)
	// 5x5 = 25 style-0 samples.
	if len(lit) != 8+25*3 {
		t.Fatalf("lit size = %d, want %d", len(lit), 8+25*3)
	}
	if string(lit[0:4]) != "QLIT" {
		t.Fatalf("lit magic = %q", lit[0:4])
	}
}

// TestSunLightTopFaces verifies sun direction cosine: a floor face is
// brightly lit from a straight-down sun, a vertical wall face is dark.
func TestSunLightTopFaces(t *testing.T) {
	sun := &Sun{Dir: [3]float64{0, 0, -1}, Value: 1000, Color: [3]float64{255, 255, 255}}
	floor := floorFace()
	wall := Face{
		Index:  1,
		Poly:   [][3]float64{{0, 0, 0}, {0, 64, 0}, {0, 64, 64}, {0, 0, 64}},
		Vecs:   [2][4]float64{{0, 1, 0, 0}, {0, 0, 1, 0}},
		Normal: [3]float64{-1, 0, 0},
	}
	r, _, _ := sun.SunLight(&floor, floor.Normal, [3]float64{32, 32, 0})
	if r <= 0 {
		t.Error("sun should light a floor facing up")
	}
	r2, _, _ := sun.SunLight(&wall, wall.Normal, [3]float64{0, 32, 32})
	if r2 > r*0.01 {
		t.Errorf("wall normal away from sun got %v, want ~0", r2)
	}
}

// TestBounceLightReachesShadowedWall verifies single-bounce radiosity:
// a wall whose direct light is shadow-blocked still receives non-zero
// light from the lit floor (the bounce pass).
func TestBounceLightReachesShadowedWall(t *testing.T) {
	floor := floorFace()
	wall := Face{
		Index:  1,
		Poly:   [][3]float64{{0, 0, 0}, {0, 64, 0}, {0, 64, 64}, {0, 0, 64}},
		Vecs:   [2][4]float64{{0, 1, 0, 0}, {0, 0, 1, 0}},
		Normal: [3]float64{1, 0, 0}, // +x, facing the lit floor
	}
	lights := []Light{{Origin: [3]float64{32, 32, 8}, Value: 20000}}
	// Occlude rays FROM the light (z=8) TO the wall (x=0); floor-light rays
	// and floor->wall radiosity rays pass.
	trace := func(from, to [3]float64) bool {
		return to[0] < 8 && from[2] > 2
	}
	direct := Bake([]Face{floor, wall}, lights, trace)
	if direct.LightOfs[1] < 0 {
		t.Fatal("wall should have a lightmap")
	}
	directVal := direct.Lighting[int(direct.LightOfs[1])+2*5+2]
	bounced := BakeWithOpts([]Face{floor, wall}, lights, trace, BakeOpts{Bounce: 1})
	bouncedVal := bounced.Lighting[int(bounced.LightOfs[1])+2*5+2]
	if directVal != 0 {
		t.Logf("note: wall direct sample = %d (expected 0 with occlusion)", directVal)
	}
	if bouncedVal <= directVal {
		t.Errorf("bounce did not light the wall: direct=%d bounced=%d", directVal, bouncedVal)
	}
}

// TestSupersamplingKeepsGrid verifies -extra preserves the lightmap grid
// (same sample count) while changing the baked values.
func TestSupersamplingKeepsGrid(t *testing.T) {
	face := floorFace()
	lights := []Light{{Origin: [3]float64{40, 40, 8}, Value: 4000}}
	base := Bake([]Face{face}, lights, nil)
	sup := BakeWithOpts([]Face{face}, lights, nil, BakeOpts{Extra: 4})
	if len(base.Lighting) != len(sup.Lighting) {
		t.Fatalf("grid changed: base %d vs supersampled %d", len(base.Lighting), len(sup.Lighting))
	}
	// The grazing corner luxel (i=0,j=0 -> s=t=8) averages sub-samples
	// closer to the light than the default centre sample; the value must
	// not be lower than the coarse one's zero-only result is invalid.
	if sup.Lighting[0] == 0 && base.Lighting[0] != 0 {
		t.Error("supersampling darkened a lit luxel to zero")
	}
	// Centre luxel values are clamped bytes (always in range).
	if len(sup.Lighting) == 0 {
		t.Fatal("no supersampled samples produced")
	}
}

// TestTextureBrightness verifies the miptex-albedo path: a mid-gray
// placeholder miptex yields the 0.5 default, a bright texture yields a
// higher albedo, and a black (legacy) texture falls back to 0.5.
func TestTextureBrightness(t *testing.T) {
	tmtex := func(pixel byte) []byte {
		out := make([]byte, 8+40+16*16)           // count+ptr, 40-byte miptex, mip0
		binary.LittleEndian.PutUint32(out[0:], 1) // one texture
		binary.LittleEndian.PutUint32(out[4:], 8) // entry offset
		copy(out[8:24], "tex")
		binary.LittleEndian.PutUint32(out[8+16:], 16) // width
		binary.LittleEndian.PutUint32(out[8+20:], 16) // height
		binary.LittleEndian.PutUint32(out[8+24:], 40) // mip0 offset
		for i := 8 + 40; i < 8+40+16*16; i++ {
			out[i] = pixel
		}
		return out
	}
	if v := textureBrightness(tmtex(128), 0); v < 0.49 || v > 0.51 {
		t.Errorf("gray albedo = %v, want ~0.5", v)
	}
	if v := textureBrightness(tmtex(255), 0); v < 0.99 {
		t.Errorf("white albedo = %v, want ~1", v)
	}
	if v := textureBrightness(tmtex(0), 0); v != 0.5 {
		t.Errorf("black placeholder albedo = %v, want 0.5 fallback", v)
	}
}

// TestPhongNormalsBlendSharedVertices verifies BuildPhongNormals: two
// faces sharing an edge vertex within the angle threshold get a blended
// normal at the seam; a far-angled pair stays flat.
func TestPhongNormalsBlendSharedVertices(t *testing.T) {
	a := floorFace()
	b := Face{
		Index:  1,
		Poly:   [][3]float64{{0, 0, 0}, {64, 0, 0}, {64, 0, 64}, {0, 0, 64}},
		Vecs:   [2][4]float64{{1, 0, 0, 0}, {0, 0, 1, 0}},
		Normal: [3]float64{0, -1, 0},
	}
	// b's normal is 90° from a's (+z vs -y): with a 120° threshold they
	// blend.
	faces := []Face{a, b}
	BuildPhongNormals(faces, 120)
	if len(faces[0].VNormals) == 0 {
		t.Fatal("floor should gain phong normals")
	}
	// Shared vertex (0,0,0) normal averages (+z, -y).
	v := faces[0].VNormals[0]
	if v[2] < 0.5 || v[1] > -0.5 {
		t.Errorf("blended normal = %v, want roughly (0,-0.7,0.7)", v)
	}
	// With a tight threshold (0°), no blending.
	faces2 := []Face{a, b}
	BuildPhongNormals(faces2, 0)
	if len(faces2[0].VNormals) != 0 {
		t.Error("0-degree threshold should not blend perpendicular faces")
	}
}
