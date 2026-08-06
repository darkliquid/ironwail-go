package gogpu

import (
	"bytes"
	"encoding/binary"
	"testing"

	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

// TestEncodeIndexBytes verifies the pure little-endian index packing matches
// the expected byte layout.
func TestEncodeIndexBytes(t *testing.T) {
	indices := []uint32{1, 258, 65539}
	got := EncodeIndexBytes(indices)
	want := make([]byte, 3*4)
	binary.LittleEndian.PutUint32(want[0:4], 1)
	binary.LittleEndian.PutUint32(want[4:8], 258)
	binary.LittleEndian.PutUint32(want[8:12], 65539)
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeIndexBytes mismatch:\n got %v\nwant %v", got, want)
	}
}

// TestCreateWorldVertexBufferPacking verifies the vertex-byte packing used by
// the world pipeline matches the canonical VertexBytes layout (48-byte
// stride). This protects parity between the delegator path and the shared
// packing helper.
func TestCreateWorldVertexBufferPacking(t *testing.T) {
	v := worldimpl.WorldVertex{
		Position:      [3]float32{1, 2, 3},
		TexCoord:      [2]float32{0.5, 0.25},
		LightmapCoord: [2]float32{0.1, 0.9},
		Normal:        [3]float32{0, 1, 0},
		LightmapLayer: 2,
		MaterialID:    7,
	}
	got := VertexBytes([]worldimpl.WorldVertex{v})
	if len(got) != 48 {
		t.Fatalf("expected 48-byte vertex payload, got %d", len(got))
	}
	if v := binary.LittleEndian.Uint32(got[44:48]); v != 7 {
		t.Fatalf("materialID mismatch: got %d want 7", v)
	}
}