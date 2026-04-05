//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"bytes"
	"testing"

	worldgogpu "github.com/darkliquid/ironwail-go/internal/renderer/world/gogpu"
)

func TestAppendGoGPUWorldVertexBytesMatchesWorldGoGPUVertexBytes(t *testing.T) {
	vertices := []WorldVertex{
		{
			Position:      [3]float32{1, 2, 3},
			TexCoord:      [2]float32{4, 5},
			LightmapCoord: [2]float32{6, 7},
			Normal:        [3]float32{8, 9, 10},
		},
		{
			Position:      [3]float32{11, 12, 13},
			TexCoord:      [2]float32{14, 15},
			LightmapCoord: [2]float32{16, 17},
			Normal:        [3]float32{18, 19, 20},
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
