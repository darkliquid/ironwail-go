package renderer

import "testing"

func TestSplitParticleVerticesByAlpha(t *testing.T) {
	vertices := []ParticleVertex{
		{Color: [4]byte{1, 2, 3, 255}},
		{Color: [4]byte{4, 5, 6, 254}},
		{Color: [4]byte{7, 8, 9, 255}},
	}

	opaque, translucent := splitParticleVerticesByAlpha(vertices)
	if len(opaque) != 2 {
		t.Fatalf("opaque count = %d, want 2", len(opaque))
	}
	if len(translucent) != 1 {
		t.Fatalf("translucent count = %d, want 1", len(translucent))
	}
	if translucent[0].Color[3] != 254 {
		t.Fatalf("translucent alpha = %d, want 254", translucent[0].Color[3])
	}
}
