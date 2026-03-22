//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/pkg/types"
)

func TestProjectWorldPointToScreenCenter(t *testing.T) {
	vp := types.IdentityMatrix()

	x, y, ok := projectWorldPointToScreen([3]float32{0, 0, 0}, vp, 801, 601)
	if !ok {
		t.Fatal("projectWorldPointToScreen returned not visible for center point")
	}
	if x != 400 || y != 300 {
		t.Fatalf("projectWorldPointToScreen center = (%d,%d), want (400,300)", x, y)
	}
}

func TestProjectWorldPointToScreenRejectsOutOfClip(t *testing.T) {
	vp := types.IdentityMatrix()

	if _, _, ok := projectWorldPointToScreen([3]float32{2, 0, 0}, vp, 800, 600); ok {
		t.Fatal("projectWorldPointToScreen accepted point outside clip space")
	}
}

func TestProjectWorldPointToScreenRejectsNonPositiveW(t *testing.T) {
	var vp types.Mat4 = types.IdentityMatrix()
	vp[3] = 0
	vp[7] = 0
	vp[11] = -1
	vp[15] = 0

	if _, _, ok := projectWorldPointToScreen([3]float32{0, 0, 1}, vp, 800, 600); ok {
		t.Fatal("projectWorldPointToScreen accepted point with non-positive clip W")
	}
}

func TestProjectParticleMarkersSkipsNonVisibleParticles(t *testing.T) {
	vp := types.IdentityMatrix()
	particles := []Particle{
		{Org: [3]float32{0, 0, 0}, Color: 5},
		{Org: [3]float32{2, 0, 0}, Color: 9},
	}
	verts := []ParticleVertex{
		{Pos: [3]float32{0, 0, 0}},
		{Pos: [3]float32{2, 0, 0}},
	}

	markers := projectParticleMarkers(particles, verts, vp, 801, 601)
	if len(markers) != 1 {
		t.Fatalf("marker count = %d, want 1", len(markers))
	}
	if markers[0].x != 400 || markers[0].y != 300 {
		t.Fatalf("marker position = (%d,%d), want (400,300)", markers[0].x, markers[0].y)
	}
	if markers[0].color != 5 {
		t.Fatalf("marker color = %d, want 5", markers[0].color)
	}
}

func TestShouldDrawGoGPUParticlesHonorsParticleMode(t *testing.T) {
	if shouldDrawGoGPUParticles(0, 4) {
		t.Fatal("mode 0 should disable gogpu particle fallback")
	}
	if !shouldDrawGoGPUParticles(1, 4) {
		t.Fatal("mode 1 should allow gogpu particle fallback")
	}
	if !shouldDrawGoGPUParticles(2, 4) {
		t.Fatal("mode 2 should allow gogpu particle fallback")
	}
	if shouldDrawGoGPUParticles(1, 0) {
		t.Fatal("zero active particles should disable gogpu particle fallback")
	}
}
