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
		name                     string
		selector                 worldBrushPassSelector
		wantIncludesLiquidOpaque bool
		wantIncludesLiquidTrans  bool
		wantIncludesOther        bool
		wantIncludesSky          bool
	}{
		{
			name:                     "all",
			selector:                 worldBrushPassAll,
			wantIncludesLiquidOpaque: true,
			wantIncludesLiquidTrans:  true,
			wantIncludesOther:        true,
			wantIncludesSky:          true,
		},
		{
			name:                     "non-liquid",
			selector:                 worldBrushPassNonLiquid,
			wantIncludesLiquidOpaque: false,
			wantIncludesLiquidTrans:  false,
			wantIncludesOther:        true,
			wantIncludesSky:          false,
		},
		{
			name:                     "liquid-only",
			selector:                 worldBrushPassLiquidOnly,
			wantIncludesLiquidOpaque: true,
			wantIncludesLiquidTrans:  true,
			wantIncludesOther:        false,
			wantIncludesSky:          false,
		},
		{
			name:                     "liquid-opaque-only",
			selector:                 worldBrushPassLiquidOpaqueOnly,
			wantIncludesLiquidOpaque: true,
			wantIncludesLiquidTrans:  false,
			wantIncludesOther:        false,
			wantIncludesSky:          false,
		},
		{
			name:                     "liquid-translucent-only",
			selector:                 worldBrushPassLiquidTranslucentOnly,
			wantIncludesLiquidOpaque: false,
			wantIncludesLiquidTrans:  true,
			wantIncludesOther:        false,
			wantIncludesSky:          false,
		},
		{
			name:                     "sky-only",
			selector:                 worldBrushPassSkyOnly,
			wantIncludesLiquidOpaque: false,
			wantIncludesLiquidTrans:  false,
			wantIncludesOther:        false,
			wantIncludesSky:          true,
		},
		{
			name:                     "invalid selector normalizes to all",
			selector:                 worldBrushPassSelector(99),
			wantIncludesLiquidOpaque: true,
			wantIncludesLiquidTrans:  true,
			wantIncludesOther:        true,
			wantIncludesSky:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.selector.includesLiquidOpaque(); got != tc.wantIncludesLiquidOpaque {
				t.Fatalf("includesLiquidOpaque() = %v, want %v", got, tc.wantIncludesLiquidOpaque)
			}
			if got := tc.selector.includesLiquidTranslucent(); got != tc.wantIncludesLiquidTrans {
				t.Fatalf("includesLiquidTranslucent() = %v, want %v", got, tc.wantIncludesLiquidTrans)
			}
			if got := tc.selector.includesNonLiquid(); got != tc.wantIncludesOther {
				t.Fatalf("includesNonLiquid() = %v, want %v", got, tc.wantIncludesOther)
			}
			if got := tc.selector.includesSky(); got != tc.wantIncludesSky {
				t.Fatalf("includesSky() = %v, want %v", got, tc.wantIncludesSky)
			}
		})
	}
}
