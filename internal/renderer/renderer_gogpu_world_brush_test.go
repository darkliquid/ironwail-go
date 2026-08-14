package renderer

import (
	"bytes"
	"testing"

	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestAppendGoGPUWorldVertexBytesMatchesWorldGoGPUVertexBytes(t *testing.T) {
	vertices := []WorldVertex{
		{
			Position:      types.Vec3{X: 1, Y: 2, Z: 3},
			TexCoord:      [2]float32{4, 5},
			LightmapCoord: [2]float32{6, 7},
			Normal:        types.Vec3{X: 8, Y: 9, Z: 10},
		},
		{
			Position:      types.Vec3{X: 11, Y: 12, Z: 13},
			TexCoord:      [2]float32{14, 15},
			LightmapCoord: [2]float32{16, 17},
			Normal:        types.Vec3{X: 18, Y: 19, Z: 20},
		},
	}
	prefix := []byte{0xaa, 0xbb}
	got := appendGoGPUWorldVertexBytes(append([]byte(nil), prefix...), vertices)
	want := append(append([]byte(nil), prefix...), worldgogpu.VertexBytes(vertices)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("packed vertex bytes mismatch\ngot:  %v\nwant: %v", got, want)
	}
}

func TestAppendGoGPUWorldIndexBytesMatchesWorldGoGPUIndexBytes(t *testing.T) {
	indices := []uint32{0, 1, 2, 17, 1024}
	prefix := []byte{0xcc}
	got := appendGoGPUWorldIndexBytes(append([]byte(nil), prefix...), indices)
	want := append(append([]byte(nil), prefix...), worldgogpu.IndexBytes(indices)...)
	if !bytes.Equal(got, want) {
		t.Fatalf("packed index bytes mismatch\ngot:  %v\nwant: %v", got, want)
	}
}
