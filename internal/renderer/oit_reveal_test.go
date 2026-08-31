package renderer

import (
	"math"
	"testing"
)

func TestOITRevealBlendingMath(t *testing.T) {
	// Let's test the mathematical behavior of revealage blending:
	// Initial: reveal = 1.0
	// Draw with alpha a:
	// Dst' = Dst * (1.0 - a)
	a := float32(0.35)
	reveal := float32(1.0)
	reveal = reveal * (1.0 - a)
	if math.Abs(float64(reveal-0.65)) > 1e-4 {
		t.Fatalf("Expected reveal 0.65, got %f", reveal)
	}
	t.Logf("1 draw: reveal = %f (unorm8 = %d)", reveal, uint8(reveal*255.0))

	// If 256 overlapping draws occurred:
	for i := 1; i < 256; i++ {
		reveal = reveal * (1.0 - a)
	}
	t.Logf("256 draws: reveal = %e (unorm8 = %d)", reveal, uint8(reveal*255.0))
}
