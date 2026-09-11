package qbsp

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	bspreader "github.com/darkliquid/ironwail-go/internal/bsp"
)

// miptexNameAt extracts the texture name of miptex table entry mi from a
// raw BSP29 miptex lump (4-byte count, count offsets, then records).
func miptexNameAt(data []byte, mi int32) string {
	if len(data) < 4 {
		return "?"
	}
	count := binary.LittleEndian.Uint32(data[0:4])
	if mi < 0 || uint32(mi) >= count {
		return "?"
	}
	off := binary.LittleEndian.Uint32(data[4+4*uint32(mi):])
	if int(off)+16 > len(data) {
		return "?"
	}
	var nb [16]byte
	copy(nb[:], data[off:off+16])
	if n := bytes.IndexByte(nb[:], 0); n >= 0 {
		return string(nb[:n])
	}
	return string(nb[:])
}

// TestSkipTexinfoFacesNotEmitted compiles a func_door with mixed
// skip+real faces and verifies the emitted BSP references no skip/hint
// texinfo from any face, matching ericw-tools ShouldOmitFace: skip faces
// are compiler annotations, never drawn.
func TestSkipTexinfoFacesNotEmitted(t *testing.T) {
	x0, y0, z0, x1, y1, z1, th := -128.0, -128.0, -16.0, 128.0, 128.0, 128.0, 16.0
	mapData := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(x0, y0, z0, x1, y1, z1, th) +
		"}\n" +
		"{\n\"classname\" \"func_door\"\n\"angle\" \"0\"\n" +
		prettySlabSkipSides(-32, -8, 0, 32, 8, 96, "mt_door") +
		"}\n" +
		"{\n\"classname\" \"info_player_start\"\n\"origin\" \"0 0 32\"\n}\n"
	m, err := ParseMap(strings.NewReader(mapData))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := Compile(m, Options{Log: func(string, ...any) {}})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	bf, err := bspreader.Load(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("bsp.Load: %v", err)
	}
	faces, ok := bf.Faces.([]bspreader.DSFace)
	if !ok {
		t.Fatalf("unexpected faces type %T", bf.Faces)
	}
	for i, f := range faces {
		if f.Texinfo < 0 || int(f.Texinfo) >= len(bf.Texinfo) {
			continue
		}
		name := miptexNameAt(bf.TextureData, bf.Texinfo[f.Texinfo].Miptex)
		if strings.EqualFold(name, "skip") || strings.EqualFold(name, "hint") {
			t.Fatalf("face %d (plane %d) references skip texinfo %q", i, f.PlaneNum, name)
		}
	}
}
