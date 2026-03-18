package renderer

import "testing"

func TestCvarRLitWaterConstant(t *testing.T) {
	if CvarRLitWater != "r_litwater" {
		t.Fatalf("CvarRLitWater = %q, want %q", CvarRLitWater, "r_litwater")
	}
}
