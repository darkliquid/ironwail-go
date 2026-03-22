//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestSceneCompositeUniformBytes(t *testing.T) {
	got := sceneCompositeUniformBytes(true, 12.5)
	if len(got) != sceneCompositeUniformBufferSize {
		t.Fatalf("uniform byte len = %d, want %d", len(got), sceneCompositeUniformBufferSize)
	}
	values := [4]float32{}
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(got[i*4:]))
	}
	if values[0] != 1 || values[1] != 1 {
		t.Fatalf("uv scale = (%v,%v), want (1,1)", values[0], values[1])
	}
	if math.Abs(float64(values[2]-(1.0/256.0))) > 0.000001 {
		t.Fatalf("warp amp = %v, want %v", values[2], 1.0/256.0)
	}
	if values[3] != 12.5 {
		t.Fatalf("warp time = %v, want 12.5", values[3])
	}

	got = sceneCompositeUniformBytes(false, 3)
	values[2] = math.Float32frombits(binary.LittleEndian.Uint32(got[8:12]))
	if values[2] != 0 {
		t.Fatalf("warp amp without warp = %v, want 0", values[2])
	}
}
