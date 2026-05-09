package world

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

func TestReadLiquidAlphaSettingsDefaultsTeleportersOpaque(t *testing.T) {
	got := ResolveLiquidAlphaSettings(0.5, 0, 0, 1, LiquidAlphaOverrides{}, &bsp.Tree{})
	if got.Water != 0.5 || got.Tele != 1 {
		t.Fatalf("liquid alpha = %+v, want water 0.5 and tele 1", got)
	}
}
