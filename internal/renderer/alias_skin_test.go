package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/model"
)

func TestResolveAliasSkinSlotUsesGroupedSkinTimingAliasSkin(t *testing.T) {
	hdr := &model.AliasHeader{
		Skins: make([][]byte, 4),
		SkinDescs: []model.AliasSkinDesc{
			{FirstFrame: 0, NumFrames: 1},
			{FirstFrame: 1, NumFrames: 3, Intervals: []float32{0.1, 0.2, 0.3}},
		},
	}

	if got := resolveAliasSkinSlot(hdr, 1, 0.05, 4); got != 1 {
		t.Fatalf("slot at t=0.05 = %d, want 1", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.15, 4); got != 2 {
		t.Fatalf("slot at t=0.15 = %d, want 2", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.25, 4); got != 3 {
		t.Fatalf("slot at t=0.25 = %d, want 3", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.35, 4); got != 1 {
		t.Fatalf("slot at t=0.35 = %d, want 1", got)
	}
}

func TestApplyAliasNoLerpListFlags(t *testing.T) {
	if cvar.Get(CvarRNoLerpList) == nil {
		cvar.Register(CvarRNoLerpList, "", 0, "")
	}
	cvar.Set(CvarRNoLerpList, "progs/flame.mdl")

	flags := applyAliasNoLerpListFlags(0, "progs/flame.mdl")
	if flags&ModNoLerp == 0 {
		t.Fatalf("expected ModNoLerp bit from r_nolerp_list")
	}
	flags = applyAliasNoLerpListFlags(0, "progs/ogre.mdl")
	if flags&ModNoLerp != 0 {
		t.Fatalf("unexpected ModNoLerp bit for non-listed model")
	}
}
