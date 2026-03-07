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

func TestSplitAliasEntitiesByAlpha(t *testing.T) {
	entities := []AliasModelEntity{
		{ModelID: "invisible", Alpha: 0},
		{ModelID: "translucent", Alpha: 0.5},
		{ModelID: "opaque-full", Alpha: 1},
	}

	opaque, translucent := splitAliasEntitiesByAlpha(entities)
	if len(opaque) != 1 {
		t.Fatalf("opaque count = %d, want 1", len(opaque))
	}
	if opaque[0].ModelID != "opaque-full" {
		t.Fatalf("opaque entities = %#v, want full alpha alias", opaque)
	}
	if len(translucent) != 1 {
		t.Fatalf("translucent count = %d, want 1", len(translucent))
	}
	if translucent[0].ModelID != "translucent" {
		t.Fatalf("translucent entity = %#v, want translucent alias", translucent[0])
	}
	for _, entity := range append(opaque, translucent...) {
		if entity.ModelID == "invisible" {
			t.Fatalf("invisible alias should have been skipped: %#v", entity)
		}
	}
}

func TestWorldBrushPassSelector(t *testing.T) {
	tests := []struct {
		name               string
		selector           worldBrushPassSelector
		wantIncludesLiquid bool
		wantIncludesOther  bool
	}{
		{
			name:               "all",
			selector:           worldBrushPassAll,
			wantIncludesLiquid: true,
			wantIncludesOther:  true,
		},
		{
			name:               "non-liquid",
			selector:           worldBrushPassNonLiquid,
			wantIncludesLiquid: false,
			wantIncludesOther:  true,
		},
		{
			name:               "liquid-only",
			selector:           worldBrushPassLiquidOnly,
			wantIncludesLiquid: true,
			wantIncludesOther:  false,
		},
		{
			name:               "invalid selector normalizes to all",
			selector:           worldBrushPassSelector(99),
			wantIncludesLiquid: true,
			wantIncludesOther:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.selector.includesLiquid(); got != tc.wantIncludesLiquid {
				t.Fatalf("includesLiquid() = %v, want %v", got, tc.wantIncludesLiquid)
			}
			if got := tc.selector.includesNonLiquid(); got != tc.wantIncludesOther {
				t.Fatalf("includesNonLiquid() = %v, want %v", got, tc.wantIncludesOther)
			}
		})
	}
}
