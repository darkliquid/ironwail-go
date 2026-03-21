package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
)

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

func TestSplitBrushEntitiesByAlpha(t *testing.T) {
	entities := []BrushEntity{
		{SubmodelIndex: 1, Alpha: 0},
		{SubmodelIndex: 2, Alpha: 0.5},
		{SubmodelIndex: 3, Alpha: 1},
	}

	opaque, translucent := splitBrushEntitiesByAlpha(entities)
	if len(opaque) != 1 {
		t.Fatalf("opaque count = %d, want 1", len(opaque))
	}
	if opaque[0].SubmodelIndex != 3 {
		t.Fatalf("opaque entities = %#v, want full alpha brush", opaque)
	}
	if len(translucent) != 1 {
		t.Fatalf("translucent count = %d, want 1", len(translucent))
	}
	if translucent[0].SubmodelIndex != 2 {
		t.Fatalf("translucent entity = %#v, want translucent brush", translucent[0])
	}
	for _, entity := range append(opaque, translucent...) {
		if entity.SubmodelIndex == 1 {
			t.Fatalf("invisible brush should have been skipped: %#v", entity)
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

func TestShouldRunLateTranslucencyBlock(t *testing.T) {
	tests := []struct {
		name   string
		inputs lateTranslucencyBlockInputs
		want   bool
	}{
		{
			name:   "disabled when no late translucent work",
			inputs: lateTranslucencyBlockInputs{},
			want:   false,
		},
		{
			name: "world draw without translucent world work does not enable block",
			inputs: lateTranslucencyBlockInputs{
				drawWorld: true,
			},
			want: false,
		},
		{
			name: "world draw with translucent world work enables block",
			inputs: lateTranslucencyBlockInputs{
				drawWorld:           true,
				hasTranslucentWorld: true,
			},
			want: true,
		},
		{
			name: "particles draw enables block",
			inputs: lateTranslucencyBlockInputs{
				drawParticles: true,
			},
			want: true,
		},
		{
			name: "decal marks enable block",
			inputs: lateTranslucencyBlockInputs{
				hasDecalMarks: true,
			},
			want: true,
		},
		{
			name: "translucent entity slices require draw entities",
			inputs: lateTranslucencyBlockInputs{
				hasTranslucentBrushEntities: true,
			},
			want: false,
		},
		{
			name: "translucent brush entities with draw entities enabled",
			inputs: lateTranslucencyBlockInputs{
				drawEntities:                true,
				hasTranslucentBrushEntities: true,
			},
			want: true,
		},
		{
			name: "translucent alias entities with draw entities enabled",
			inputs: lateTranslucencyBlockInputs{
				drawEntities:                true,
				hasTranslucentAliasEntities: true,
			},
			want: true,
		},
		{
			name: "sprite entities require draw entities",
			inputs: lateTranslucencyBlockInputs{
				hasSpriteEntities: true,
			},
			want: false,
		},
		{
			name: "sprite entities with draw entities enabled",
			inputs: lateTranslucencyBlockInputs{
				drawEntities:      true,
				hasSpriteEntities: true,
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRunLateTranslucencyBlock(tc.inputs); got != tc.want {
				t.Fatalf("shouldRunLateTranslucencyBlock() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPlanGoGPUEntityDrawOrder(t *testing.T) {
	brushEntities := []BrushEntity{
		{SubmodelIndex: 1, Alpha: 0},
		{SubmodelIndex: 2, Alpha: 0.5},
		{SubmodelIndex: 3, Alpha: 1},
	}
	aliasEntities := []AliasModelEntity{
		{ModelID: "hidden", Alpha: 0},
		{ModelID: "ghost", Alpha: 0.5},
		{ModelID: "ogre", Alpha: 1},
	}
	spriteEntities := []SpriteEntity{{ModelID: "flame"}}
	decalMarks := []DecalMarkEntity{{Size: 16}}

	plan := planGoGPUEntityDrawOrder(brushEntities, aliasEntities, spriteEntities, decalMarks)
	if len(plan.opaqueBrush) != 1 || plan.opaqueBrush[0].SubmodelIndex != 3 {
		t.Fatalf("opaqueBrush = %#v, want only submodel 3", plan.opaqueBrush)
	}
	if len(plan.translucentBrush) != 1 || plan.translucentBrush[0].SubmodelIndex != 2 {
		t.Fatalf("translucentBrush = %#v, want only submodel 2", plan.translucentBrush)
	}
	if len(plan.opaqueAlias) != 1 || plan.opaqueAlias[0].ModelID != "ogre" {
		t.Fatalf("opaqueAlias = %#v, want only ogre", plan.opaqueAlias)
	}
	if len(plan.translucentAlias) != 1 || plan.translucentAlias[0].ModelID != "ghost" {
		t.Fatalf("translucentAlias = %#v, want only ghost", plan.translucentAlias)
	}
	want := []gogpuEntityPhase{
		gogpuEntityPhaseOpaqueBrush,
		gogpuEntityPhaseOpaqueAlias,
		gogpuEntityPhaseTranslucentBrush,
		gogpuEntityPhaseDecals,
		gogpuEntityPhaseTranslucentAlias,
		gogpuEntityPhaseSprites,
	}
	if len(plan.phases) != len(want) {
		t.Fatalf("phase count = %d, want %d (%v)", len(plan.phases), len(want), plan.phases)
	}
	for i, phase := range want {
		if plan.phases[i] != phase {
			t.Fatalf("phase[%d] = %v, want %v (all=%v)", i, plan.phases[i], phase, plan.phases)
		}
	}
}

func TestWorldLiquidFaceTypeMask(t *testing.T) {
	faces := []WorldFace{
		{Flags: model.SurfDrawWater},                       // non-turbulent should not count
		{Flags: model.SurfDrawTurb | model.SurfDrawWater},  // turbulent water counts
		{Flags: model.SurfDrawTurb | model.SurfDrawLava},   // turbulent lava counts
		{Flags: model.SurfDrawTurb | model.SurfDrawSky},    // non-liquid
		{Flags: model.SurfDrawTurb | model.SurfDrawSlime},  // turbulent slime counts
		{Flags: model.SurfDrawTurb | model.SurfDrawTele},   // turbulent tele counts
		{Flags: model.SurfDrawTurb | model.SurfDrawFence},  // non-liquid
		{Flags: model.SurfDrawWater | model.SurfDrawFence}, // still non-turbulent
	}
	got := worldLiquidFaceTypeMask(faces)
	want := int32(model.SurfDrawWater | model.SurfDrawLava | model.SurfDrawSlime | model.SurfDrawTele)
	if got != want {
		t.Fatalf("worldLiquidFaceTypeMask() = %#x, want %#x", got, want)
	}
}

func TestHasTranslucentWorldLiquidFaceType(t *testing.T) {
	mask := int32(model.SurfDrawWater | model.SurfDrawLava | model.SurfDrawSlime | model.SurfDrawTele)
	if got := hasTranslucentWorldLiquidFaceType(mask, worldLiquidAlphaSettings{water: 1, lava: 1, slime: 1, tele: 1}); got {
		t.Fatalf("all-opaque alpha should not be translucent")
	}
	if got := hasTranslucentWorldLiquidFaceType(mask, worldLiquidAlphaSettings{water: 1, lava: 0.5, slime: 1, tele: 1}); !got {
		t.Fatalf("translucent lava should be translucent")
	}
	if got := hasTranslucentWorldLiquidFaceType(int32(model.SurfDrawWater), worldLiquidAlphaSettings{water: 0.75, lava: 1, slime: 1, tele: 1}); !got {
		t.Fatalf("translucent water should be translucent")
	}
}
