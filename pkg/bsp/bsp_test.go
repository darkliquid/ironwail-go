package bsp_test

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestBSPLumpsRoundTripAndPatch(t *testing.T) {
	// Create 15 sample lumps
	lumps := make([][]byte, 15)
	for i := range lumps {
		lumps[i] = []byte(strings.Repeat(string(rune('A'+i)), (i+1)*16))
	}

	// 1. WriteBSP
	data, err := bsp.WriteBSP(lumps, bsp.BSPVersion)
	if err != nil {
		t.Fatalf("WriteBSP failed: %v", err)
	}

	// 2. ReadLumps
	ver, readLumps, err := bsp.ReadLumps(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ReadLumps failed: %v", err)
	}
	if ver != bsp.BSPVersion {
		t.Errorf("version = %d, want %d", ver, bsp.BSPVersion)
	}
	if len(readLumps) != 15 {
		t.Fatalf("readLumps count = %d, want 15", len(readLumps))
	}
	for i := range lumps {
		if !bytes.Equal(readLumps[i], lumps[i]) {
			t.Errorf("lump %d mismatch: got %d bytes, want %d bytes", i, len(readLumps[i]), len(lumps[i]))
		}
	}

	// 3. PatchLump: replace lump 0 (entities) with new content
	newEnts := []byte("{\n\"classname\" \"worldspawn\"\n\"sky\" \"night\"\n}\n\x00")
	var patchedBuf bytes.Buffer
	if err := bsp.PatchLump(bytes.NewReader(data), &patchedBuf, bsp.LumpEntities, newEnts); err != nil {
		t.Fatalf("PatchLump failed: %v", err)
	}

	ver2, patchedLumps, err := bsp.ReadLumps(bytes.NewReader(patchedBuf.Bytes()))
	if err != nil {
		t.Fatalf("ReadLumps on patched failed: %v", err)
	}
	if ver2 != bsp.BSPVersion {
		t.Errorf("patched version = %d, want %d", ver2, bsp.BSPVersion)
	}
	if !bytes.Equal(patchedLumps[bsp.LumpEntities], newEnts) {
		t.Errorf("patched lump 0 mismatch")
	}
	// Verify remaining lumps are intact
	for i := 1; i < 15; i++ {
		if !bytes.Equal(patchedLumps[i], lumps[i]) {
			t.Errorf("patched lump %d corrupted", i)
		}
	}
}

func TestEntityParser(t *testing.T) {
	entData := `
// This is a comment
{
"classname" "worldspawn"
"message" "Test Map"
"_wateralpha" "0.6"
"lavaalpha" "0.8"
"_fog" "0.1 0.2 0.3 0.05"
"light" "300"
"sounds" "2"
"spawnflags" "1"
"flag_bool" "yes"
"flag_false" "0"
"origin" "128 -64 32"
}
// Another comment between entities
{
"classname" "light"
"origin" "0 0 64"
"light" "250"
}
`

	entities, err := bsp.ParseEntities(entData)
	if err != nil {
		t.Fatalf("ParseEntities failed: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("parsed %d entities, want 2", len(entities))
	}

	world := entities[0]
	if got := world.String("classname"); got != "worldspawn" {
		t.Errorf("classname = %q, want 'worldspawn'", got)
	}
	if got := world.String("message"); got != "Test Map" {
		t.Errorf("message = %q, want 'Test Map'", got)
	}

	// Floats with leading underscore handling
	if got := world.Float("wateralpha", 1.0); math.Abs(float64(got-0.6)) > 1e-4 {
		t.Errorf("wateralpha = %f, want 0.6", got)
	}
	if got := world.Float("lavaalpha", 1.0); math.Abs(float64(got-0.8)) > 1e-4 {
		t.Errorf("lavaalpha = %f, want 0.8", got)
	}
	if got := world.Float("nonexistent", 0.5); got != 0.5 {
		t.Errorf("nonexistent float = %f, want 0.5", got)
	}

	// Integers
	if got := world.Int("light", 0); got != 300 {
		t.Errorf("light = %d, want 300", got)
	}
	if got := world.Int("sounds", 0); got != 2 {
		t.Errorf("sounds = %d, want 2", got)
	}
	if got := world.Int("missing", 42); got != 42 {
		t.Errorf("missing int = %d, want 42", got)
	}

	// Booleans
	if got := world.Bool("flag_bool", false); !got {
		t.Errorf("flag_bool = false, want true")
	}
	if got := world.Bool("flag_false", true); got {
		t.Errorf("flag_false = true, want false")
	}

	// Val methods with boolean success indicators
	if f, ok := world.FloatVal("wateralpha"); !ok || math.Abs(float64(f-0.6)) > 1e-4 {
		t.Errorf("FloatVal(wateralpha) = (%f, %v), want (0.6, true)", f, ok)
	}
	if _, ok := world.FloatVal("nonexistent"); ok {
		t.Errorf("FloatVal(nonexistent) returned ok=true")
	}
	if i, ok := world.IntVal("light"); !ok || i != 300 {
		t.Errorf("IntVal(light) = (%d, %v), want (300, true)", i, ok)
	}
	if _, ok := world.IntVal("missing"); ok {
		t.Errorf("IntVal(missing) returned ok=true")
	}
	if b, ok := world.BoolVal("flag_bool"); !ok || !b {
		t.Errorf("BoolVal(flag_bool) = (%v, %v), want (true, true)", b, ok)
	}
	if b, ok := world.BoolVal("flag_false"); !ok || b {
		t.Errorf("BoolVal(flag_false) = (%v, %v), want (false, true)", b, ok)
	}
	if _, ok := world.BoolVal("missing"); ok {
		t.Errorf("BoolVal(missing) returned ok=true")
	}

	// Vec3
	org, ok := world.Vec3("origin")
	if !ok {
		t.Fatalf("origin Vec3 not found")
	}
	wantOrg := types.Vec3{X: 128, Y: -64, Z: 32}
	if org != wantOrg {
		t.Errorf("origin = %v, want %v", org, wantOrg)
	}

	// ParseFirstEntity helper
	first, ok := bsp.ParseFirstEntity(entData)
	if !ok {
		t.Fatalf("ParseFirstEntity failed")
	}
	if got := first.String("classname"); got != "worldspawn" {
		t.Errorf("first entity classname = %q, want 'worldspawn'", got)
	}
}

func TestPortalFileRoundTrip(t *testing.T) {
	pf := &bsp.PortalFile{
		LeafCount: 3,
		Portals: []bsp.Portal{
			{
				Leafs: [2]int{1, 2},
				Points: []types.Vec3d{
					{X: 0, Y: 0, Z: 0},
					{X: 64, Y: 0, Z: 0},
					{X: 64, Y: 64, Z: 0},
					{X: 0, Y: 64, Z: 0},
				},
			},
			{
				Leafs: [2]int{2, 3},
				Points: []types.Vec3d{
					{X: 64, Y: 0, Z: 0},
					{X: 128, Y: 0, Z: 0},
					{X: 128, Y: 64, Z: 0},
				},
			},
		},
	}

	data := pf.Serialize()
	if len(data) == 0 {
		t.Fatalf("Serialize returned empty bytes")
	}

	parsed, err := bsp.ParsePortalFile(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParsePortalFile failed: %v", err)
	}

	if parsed.LeafCount != pf.LeafCount {
		t.Errorf("LeafCount = %d, want %d", parsed.LeafCount, pf.LeafCount)
	}
	if len(parsed.Portals) != len(pf.Portals) {
		t.Fatalf("len(Portals) = %d, want %d", len(parsed.Portals), len(pf.Portals))
	}
	for i := range pf.Portals {
		if parsed.Portals[i].Leafs != pf.Portals[i].Leafs {
			t.Errorf("portal %d leafs = %v, want %v", i, parsed.Portals[i].Leafs, pf.Portals[i].Leafs)
		}
		if len(parsed.Portals[i].Points) != len(pf.Portals[i].Points) {
			t.Errorf("portal %d points len = %d, want %d", i, len(parsed.Portals[i].Points), len(pf.Portals[i].Points))
		}
		for j := range pf.Portals[i].Points {
			p1 := pf.Portals[i].Points[j]
			p2 := parsed.Portals[i].Points[j]
			if p1.Distance(p2) > 1e-4 {
				t.Errorf("portal %d point %d = %v, want %v", i, j, p2, p1)
			}
		}
	}
}

func TestLitFileValidation(t *testing.T) {
	tree := &bsp.Tree{
		Lighting: make([]byte, 100),
	}

	// 1. Valid .lit file
	validLit := make([]byte, 8+100*3)
	copy(validLit[0:4], "QLIT")
	validLit[4] = 1 // version 1
	for i := 8; i < len(validLit); i++ {
		validLit[i] = byte(i % 256)
	}

	if err := bsp.ApplyLitFile(tree, validLit); err != nil {
		t.Fatalf("ApplyLitFile on valid data failed: %v", err)
	}
	if !tree.LightingRGB {
		t.Errorf("tree.LightingRGB = false, want true")
	}
	if len(tree.Lighting) != 300 {
		t.Errorf("tree.Lighting len = %d, want 300", len(tree.Lighting))
	}

	// 2. Corrupt magic
	corruptMagic := append([]byte(nil), validLit...)
	copy(corruptMagic[0:4], "NOPE")
	if err := bsp.ApplyLitFile(tree, corruptMagic); err == nil {
		t.Errorf("expected error for bad magic")
	}

	// 3. Outdated length
	badLen := validLit[:len(validLit)-10]
	if err := bsp.ApplyLitFile(tree, badLen); err == nil {
		t.Errorf("expected error for size mismatch")
	}
}

func TestSyntheticTreeAndPointInLeaf(t *testing.T) {
	lumps := make([][]byte, 15)

	// Planes: 1 plane at z = 0
	planeBuf := make([]byte, 20)
	binary.LittleEndian.PutUint32(planeBuf[0:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(planeBuf[4:], math.Float32bits(0))
	binary.LittleEndian.PutUint32(planeBuf[8:], math.Float32bits(1)) // Normal.Z = 1
	binary.LittleEndian.PutUint32(planeBuf[12:], math.Float32bits(0)) // Dist = 0
	binary.LittleEndian.PutUint32(planeBuf[16:], 2)                   // Type = PlaneZ (2)
	lumps[bsp.LumpPlanes] = planeBuf

	// Textures: 4 bytes (num textures = 0)
	lumps[bsp.LumpTextures] = make([]byte, 4)

	putI32 := func(b []byte, off int, v int32) {
		binary.LittleEndian.PutUint32(b[off:], uint32(v))
	}

	// Leafs: leaf 0 (solid) and leaf 1 (empty)
	leafBuf := make([]byte, 28*2)
	// Leaf 0: Solid
	putI32(leafBuf, 0, bsp.ContentsSolid)
	putI32(leafBuf, 4, -1) // no vis
	// Leaf 1: Empty
	putI32(leafBuf, 28+0, bsp.ContentsEmpty)
	putI32(leafBuf, 28+4, -1) // no vis
	lumps[bsp.LumpLeafs] = leafBuf

	putI16 := func(b []byte, off int, v int16) {
		binary.LittleEndian.PutUint16(b[off:], uint16(v))
	}

	// Nodes: 1 node (child 0 -> leaf 1 (65534), child 1 -> leaf 0 (65535))
	nodeBuf := make([]byte, 24)
	binary.LittleEndian.PutUint32(nodeBuf[0:], 0) // PlaneNum = 0
	binary.LittleEndian.PutUint16(nodeBuf[4:], 65534) // Child 0 -> Leaf 1
	binary.LittleEndian.PutUint16(nodeBuf[6:], 65535) // Child 1 -> Leaf 0
	putI16(nodeBuf, 8, -512)
	putI16(nodeBuf, 10, -512)
	putI16(nodeBuf, 12, -512)
	putI16(nodeBuf, 14, 512)
	putI16(nodeBuf, 16, 512)
	putI16(nodeBuf, 18, 512)
	lumps[bsp.LumpNodes] = nodeBuf

	// Models: 1 model (world)
	modelBuf := make([]byte, 64)
	binary.LittleEndian.PutUint32(modelBuf[36:], 0) // HeadNode[0] = 0
	binary.LittleEndian.PutUint32(modelBuf[52:], 1) // VisLeafs = 1
	lumps[bsp.LumpModels] = modelBuf

	bspData, err := bsp.WriteBSP(lumps, bsp.BSPVersion)
	if err != nil {
		t.Fatalf("WriteBSP failed: %v", err)
	}

	// Test LoadTree
	tree, err := bsp.LoadTree(bytes.NewReader(bspData))
	if err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	if len(tree.Leafs) != 2 {
		t.Fatalf("tree.Leafs count = %d, want 2", len(tree.Leafs))
	}
	if len(tree.Nodes) != 1 {
		t.Fatalf("tree.Nodes count = %d, want 1", len(tree.Nodes))
	}

	// Point above z=0 should land in Leaf 1 (empty)
	pAbove := types.Vec3{X: 0, Y: 0, Z: 50}
	leafAbove := tree.PointInLeaf(pAbove)
	if leafAbove == nil {
		t.Fatalf("PointInLeaf(above) returned nil")
	}
	if leafAbove.Contents != bsp.ContentsEmpty {
		t.Errorf("leafAbove.Contents = %d, want ContentsEmpty (%d)", leafAbove.Contents, bsp.ContentsEmpty)
	}
	if idx := tree.PointInLeafIndex(pAbove); idx != 1 {
		t.Errorf("PointInLeafIndex(above) = %d, want 1", idx)
	}

	// Point below z=0 should land in Leaf 0 (solid)
	pBelow := types.Vec3{X: 0, Y: 0, Z: -50}
	leafBelow := tree.PointInLeaf(pBelow)
	if leafBelow == nil {
		t.Fatalf("PointInLeaf(below) returned nil")
	}
	if leafBelow.Contents != bsp.ContentsSolid {
		t.Errorf("leafBelow.Contents = %d, want ContentsSolid (%d)", leafBelow.Contents, bsp.ContentsSolid)
	}
	if idx := tree.PointInLeafIndex(pBelow); idx != 0 {
		t.Errorf("PointInLeafIndex(below) = %d, want 0", idx)
	}

	// Test low-level File Load
	file, err := bsp.Load(bytes.NewReader(bspData))
	if err != nil {
		t.Fatalf("bsp.Load failed: %v", err)
	}
	if len(file.Planes) != 1 {
		t.Errorf("file.Planes len = %d, want 1", len(file.Planes))
	}
	if len(file.Models) != 1 {
		t.Errorf("file.Models len = %d, want 1", len(file.Models))
	}
}

