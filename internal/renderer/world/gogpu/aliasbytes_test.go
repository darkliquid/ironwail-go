package gogpu

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestAppendVertexBytesMatchesVertexBytes(t *testing.T) {
	vertices := []worldimpl.WorldVertex{
		{
			Position:      [3]float32{1, 2, 3},
			TexCoord:      [2]float32{0.5, 0.25},
			LightmapCoord: [2]float32{0.1, 0.9},
			Normal:        [3]float32{0, 1, 0},
			LightmapLayer: 2,
			MaterialID:    7,
		},
		{
			Position:      [3]float32{4, 5, 6},
			LightmapLayer: 3,
			MaterialID:    8,
		},
	}
	// Append into an existing buffer with prefix to verify appending works.
	prefix := []byte{0xAA, 0xBB}
	got := AppendVertexBytes(prefix, vertices)
	if len(got) != len(prefix)+len(vertices)*48 {
		t.Fatalf("len = %d, want %d", len(got), len(prefix)+len(vertices)*48)
	}
	if !bytes.Equal(got[:2], prefix) {
		t.Fatal("prefix clobbered")
	}
	payload := got[2:]
	if !bytes.Equal(payload, VertexBytes(vertices)) {
		t.Fatalf("AppendVertexBytes payload != VertexBytes:\n got %v\nwant %v", payload, VertexBytes(vertices))
	}
}

func TestAppendIndexBytesPacksLittleEndian(t *testing.T) {
	got := AppendIndexBytes([]byte{0xCC}, []uint32{1, 258, 65539})
	if len(got) != 1+3*4 {
		t.Fatalf("len = %d, want %d", len(got), 1+3*4)
	}
	if got[0] != 0xCC {
		t.Fatal("prefix clobbered")
	}
	payload := got[1:]
	if !bytes.Equal(payload, IndexBytes([]uint32{1, 258, 65539})) {
		t.Fatalf("AppendIndexBytes payload mismatch: %v", payload)
	}
}

func TestAppendAliasSceneUniformBytesLayout(t *testing.T) {
	vp := types.IdentityMatrix()
	origin := [3]float32{1, 2, 3}
	// Use a fog density of 0 so FogUniformDensity is identity-ish; the
	// encoding rounds through the world helper, so just verify offsets.
	dst := AppendAliasSceneUniformBytes([]byte{}, 0, vp, origin, 0.5, [3]float32{0.1, 0.2, 0.3}, 0)
	// The buffer is padded to WorldUniformAlign; the uniform occupies [0,96).
	if len(dst) != WorldUniformAlign {
		t.Fatalf("len = %d, want %d (uniform align)", len(dst), WorldUniformAlign)
	}
	// Verify the uniform payload (first AliasSceneUniformBufferSize bytes).
	// First 64 bytes: column-major matrix (identity) in little-endian floats.
	if v := math.Float32frombits(binary.LittleEndian.Uint32(dst[0:4])); v != 1 {
		t.Fatalf("matrix[0] = %v, want 1", v)
	}
	// Camera origin at 64:76.
	if v := math.Float32frombits(binary.LittleEndian.Uint32(dst[64:68])); v != 1 {
		t.Fatalf("origin[0] = %v, want 1", v)
	}
	// Fog density at 76:80 (0 -> 0 via FogUniformDensity).
	if v := math.Float32frombits(binary.LittleEndian.Uint32(dst[76:80])); v != 0 {
		t.Fatalf("fogDensity uniform = %v, want 0", v)
	}
	// Alpha at 92:96.
	if v := math.Float32frombits(binary.LittleEndian.Uint32(dst[92:96])); v != 0.5 {
		t.Fatalf("alpha = %v, want 0.5", v)
	}
}

func TestAppendAliasVertexBytesStride(t *testing.T) {
	vertices := []worldimpl.WorldVertex{
		{Position: [3]float32{1, 2, 3}, MaterialID: 42},
	}
	got := AliasVertexBytes(vertices)
	if len(got) != AliasVertexStride {
		t.Fatalf("len = %d, want %d", len(got), AliasVertexStride)
	}
	if v := binary.LittleEndian.Uint32(got[44:48]); v != 42 {
		t.Fatalf("materialID = %d, want 42", v)
	}
	if !bytes.Equal(got, AliasVertexBytesInto(nil, vertices)) {
		t.Fatal("AliasVertexBytes and AliasVertexBytesInto disagree")
	}
}