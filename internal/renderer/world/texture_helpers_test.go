package world

import (
	"encoding/binary"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

func TestFaceFlagsMissingTextureFallsBackToSpecialSlot(t *testing.T) {
	textureData := make([]byte, 8)
	binary.LittleEndian.PutUint32(textureData[:4], 1)
	binary.LittleEndian.PutUint32(textureData[4:], uint32(0xffffffff))
	tree := &bsp.Tree{
		TextureData: textureData,
		Texinfo: []bsp.Texinfo{
			{Miptex: 0, Flags: bsp.TexSpecial},
		},
	}
	face := &bsp.TreeFace{Texinfo: 0}

	flags := FaceFlags([]TextureMeta{{Type: model.TexTypeTele}}, tree, face)
	if flags&model.SurfNoTexture == 0 {
		t.Fatalf("flags = %#x, want SurfNoTexture", flags)
	}
	if flags&model.SurfDrawTurb != 0 {
		t.Fatalf("flags = %#x, missing dummy texture must not be turbulent liquid", flags)
	}
}

func TestFaceTextureIndexMapsMissingToWhiteDummy(t *testing.T) {
	// Two textures: entry 0 present but an invalid offset, entry 1 present.
	textureData := make([]byte, 12)
	binary.LittleEndian.PutUint32(textureData[:4], 2)
	binary.LittleEndian.PutUint32(textureData[4:8], 8)  // miptex 0 at offset 8
	binary.LittleEndian.PutUint32(textureData[8:12], uint32(0xffffffff))
	tree := &bsp.Tree{
		TextureData: textureData,
		Texinfo:     []bsp.Texinfo{{Miptex: 0, Flags: 0}},
	}
	face := &bsp.TreeFace{Texinfo: 0}
	got := FaceTextureIndex(tree, face)
	// miptex 0 has offset 8 which is >= len(12)? No — 8 < 12 but parse fails on
	// 4 bytes (miptex needs 40+). So it's "missing" -> fallback slot for
	// non-special texinfo = textureCount = 2.
	if got != 2 {
		t.Fatalf("FaceTextureIndex = %d, want 2 (white dummy slot)", got)
	}
}

func TestTexCoordDoubleIsFloat64Precise(t *testing.T) {
	pos := [3]float32{10, 20, 30}
	vec := [4]float32{0.5, 0.25, 0.125, 4}
	got := TexCoordDouble(pos, vec)
	want := 10*0.5 + 20*0.25 + 30*0.125 + 4
	if got != want {
		t.Fatalf("TexCoordDouble = %v, want %v", got, want)
	}
}

func TestTextureCountMalformed(t *testing.T) {
	if _, ok := TextureCount(nil); ok {
		t.Fatal("TextureCount(nil) should be !ok")
	}
	if _, ok := TextureCount(&bsp.Tree{TextureData: []byte{1, 2, 3}}); ok {
		t.Fatal("short texture data should be !ok")
	}
}
