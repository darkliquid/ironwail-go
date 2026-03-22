package renderer

import "github.com/ironwail/ironwail-go/internal/model"

type worldBrushPassSelector int

const (
	worldBrushPassAll worldBrushPassSelector = iota
	worldBrushPassNonLiquid
	worldBrushPassLiquidOnly
	worldBrushPassLiquidOpaqueOnly
	worldBrushPassLiquidTranslucentOnly
	worldBrushPassSkyOnly
)

// normalizeWorldBrushPassSelector normalizes requested pass filters so parity checks compare equivalent selector forms.
func normalizeWorldBrushPassSelector(selector worldBrushPassSelector) worldBrushPassSelector {
	switch selector {
	case worldBrushPassAll, worldBrushPassNonLiquid, worldBrushPassLiquidOnly, worldBrushPassLiquidOpaqueOnly, worldBrushPassLiquidTranslucentOnly, worldBrushPassSkyOnly:
		return selector
	default:
		return worldBrushPassAll
	}
}

// includesNonLiquid reports whether a selector includes standard opaque/translucent non-liquid world passes.
func (selector worldBrushPassSelector) includesNonLiquid() bool {
	selector = normalizeWorldBrushPassSelector(selector)
	return selector == worldBrushPassAll || selector == worldBrushPassNonLiquid
}

// includesLiquidOpaque reports whether opaque liquid surfaces belong to the selected world pass set.
func (selector worldBrushPassSelector) includesLiquidOpaque() bool {
	selector = normalizeWorldBrushPassSelector(selector)
	return selector == worldBrushPassAll || selector == worldBrushPassLiquidOnly || selector == worldBrushPassLiquidOpaqueOnly
}

// includesLiquidTranslucent reports whether translucent liquid surfaces belong to the selected world pass set.
func (selector worldBrushPassSelector) includesLiquidTranslucent() bool {
	selector = normalizeWorldBrushPassSelector(selector)
	return selector == worldBrushPassAll || selector == worldBrushPassLiquidOnly || selector == worldBrushPassLiquidTranslucentOnly
}

// includesSky reports whether sky surfaces should be emitted for the selected pass selector.
func (selector worldBrushPassSelector) includesSky() bool {
	selector = normalizeWorldBrushPassSelector(selector)
	return selector == worldBrushPassAll || selector == worldBrushPassSkyOnly
}

// isFullyOpaqueAlpha classifies alpha values that can stay in the opaque pass, avoiding unnecessary blending work.
func isFullyOpaqueAlpha(alpha float32) bool {
	return alpha >= 1
}

// visibleEntityAlpha computes effective entity alpha after visibility and render-mode rules are applied.
func visibleEntityAlpha(alpha float32) (float32, bool) {
	alpha = clamp01(alpha)
	return alpha, alpha > 0
}

// splitAliasEntitiesByAlpha partitions alias entities into opaque and blended groups to preserve depth correctness and batching.
func splitAliasEntitiesByAlpha(entities []AliasModelEntity) (opaque, translucent []AliasModelEntity) {
	if len(entities) == 0 {
		return nil, nil
	}
	opaque = make([]AliasModelEntity, 0, len(entities))
	translucent = make([]AliasModelEntity, 0, len(entities))
	for _, entity := range entities {
		alpha, visible := visibleEntityAlpha(entity.Alpha)
		if !visible {
			continue
		}
		if isFullyOpaqueAlpha(alpha) {
			opaque = append(opaque, entity)
			continue
		}
		translucent = append(translucent, entity)
	}
	return opaque, translucent
}

// splitBrushEntitiesByAlpha performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func splitBrushEntitiesByAlpha(entities []BrushEntity) (opaque, translucent []BrushEntity) {
	if len(entities) == 0 {
		return nil, nil
	}
	opaque = make([]BrushEntity, 0, len(entities))
	translucent = make([]BrushEntity, 0, len(entities))
	for _, entity := range entities {
		alpha, visible := visibleEntityAlpha(entity.Alpha)
		if !visible {
			continue
		}
		if isFullyOpaqueAlpha(alpha) {
			opaque = append(opaque, entity)
			continue
		}
		translucent = append(translucent, entity)
	}
	return opaque, translucent
}

// splitParticleVerticesByAlpha performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func splitParticleVerticesByAlpha(vertices []ParticleVertex) (opaque, translucent []ParticleVertex) {
	if len(vertices) == 0 {
		return nil, nil
	}
	opaque = make([]ParticleVertex, 0, len(vertices))
	translucent = make([]ParticleVertex, 0, len(vertices))
	for _, vertex := range vertices {
		if isFullyOpaqueAlpha(float32(vertex.Color[3]) / 255.0) {
			opaque = append(opaque, vertex)
			continue
		}
		translucent = append(translucent, vertex)
	}
	return opaque, translucent
}

type lateTranslucencyBlockInputs struct {
	drawWorld                   bool
	hasTranslucentWorld         bool
	drawEntities                bool
	hasSpriteEntities           bool
	drawParticles               bool
	hasDecalMarks               bool
	hasTranslucentBrushEntities bool
	hasTranslucentAliasEntities bool
}

// shouldRunLateTranslucencyBlock performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func shouldRunLateTranslucencyBlock(inputs lateTranslucencyBlockInputs) bool {
	if (inputs.drawWorld && inputs.hasTranslucentWorld) || inputs.hasDecalMarks || inputs.drawParticles {
		return true
	}
	if !inputs.drawEntities {
		return false
	}
	return inputs.hasTranslucentBrushEntities || inputs.hasTranslucentAliasEntities || inputs.hasSpriteEntities
}

type gogpuSharedDepthStencilClearInputs struct {
	drawWorld         bool
	drawEntities      bool
	hasBrushEntities  bool
	hasAliasEntities  bool
	hasSpriteEntities bool
	hasParticles      bool
	hasDecalMarks     bool
	hasViewModel      bool
}

// shouldClearGoGPUSharedDepthStencil performs its step in this part of the renderer; this helper exists to keep the no-world scene-pass behavior deterministic and aligned with the primary renderer.
func shouldClearGoGPUSharedDepthStencil(inputs gogpuSharedDepthStencilClearInputs) bool {
	if inputs.drawWorld {
		return false
	}
	if inputs.hasDecalMarks {
		return true
	}
	if inputs.hasParticles {
		return true
	}
	if !inputs.drawEntities {
		return false
	}
	return inputs.hasBrushEntities || inputs.hasAliasEntities || inputs.hasSpriteEntities || inputs.hasViewModel
}

type gogpuEntityPhase int

const (
	gogpuEntityPhaseOpaqueBrush gogpuEntityPhase = iota
	gogpuEntityPhaseOpaqueAlias
	gogpuEntityPhaseOpaqueParticles
	gogpuEntityPhaseSkyBrush
	gogpuEntityPhaseOpaqueLiquidBrush
	gogpuEntityPhaseTranslucentWorldLiquid
	gogpuEntityPhaseTranslucentLiquidBrush
	gogpuEntityPhaseTranslucentBrush
	gogpuEntityPhaseDecals
	gogpuEntityPhaseTranslucentAlias
	gogpuEntityPhaseSprites
	gogpuEntityPhaseTranslucentParticles
)

type gogpuOpaqueAliasStep int

const (
	gogpuOpaqueAliasStepEntities gogpuOpaqueAliasStep = iota
	gogpuOpaqueAliasStepShadows
)

type gogpuEntityDrawPlan struct {
	opaqueBrush      []BrushEntity
	skyBrush         []BrushEntity
	translucentBrush []BrushEntity
	opaqueAlias      []AliasModelEntity
	translucentAlias []AliasModelEntity
	phases           []gogpuEntityPhase
}

// visibleSkyBrushEntities performs its step in this part of the renderer; this helper exists to keep the fallback sky phase aligned with the primary renderer without widening into full geometry bucketing.
func visibleSkyBrushEntities(entities []BrushEntity) []BrushEntity {
	var sky []BrushEntity
	for _, entity := range entities {
		if entity.Alpha <= 0 {
			continue
		}
		sky = append(sky, entity)
	}
	return sky
}

// classifyGoGPUParticlePhase performs its step in this part of the renderer; this helper exists to keep the particle fallback scheduling deterministic and aligned with the primary renderer.
func classifyGoGPUParticlePhase(mode, activeParticles int) (gogpuEntityPhase, bool) {
	switch {
	case ShouldDrawParticles(mode, false, false, activeParticles):
		return gogpuEntityPhaseOpaqueParticles, true
	case ShouldDrawParticles(mode, true, false, activeParticles):
		return gogpuEntityPhaseTranslucentParticles, true
	default:
		return 0, false
	}
}

// planGoGPUEntityDrawOrder keeps the GoGPU entity pass sequencing aligned with the
// current OpenGL ordering without pulling world-pass or translucency-block mechanics
// into the secondary backend.
func planGoGPUEntityDrawOrder(drawEntities bool, hasTranslucentWorld bool, brushEntities []BrushEntity, aliasEntities []AliasModelEntity, spriteEntities []SpriteEntity, decalMarks []DecalMarkEntity, particlePhase gogpuEntityPhase, hasParticlePhase bool) gogpuEntityDrawPlan {
	var (
		opaqueBrush      []BrushEntity
		skyBrush         []BrushEntity
		translucentBrush []BrushEntity
		opaqueAlias      []AliasModelEntity
		translucentAlias []AliasModelEntity
	)
	if drawEntities {
		opaqueBrush, translucentBrush = splitBrushEntitiesByAlpha(brushEntities)
		skyBrush = visibleSkyBrushEntities(brushEntities)
		opaqueAlias, translucentAlias = splitAliasEntitiesByAlpha(aliasEntities)
	} else {
		spriteEntities = nil
	}
	phases := make([]gogpuEntityPhase, 0, 8)
	if len(opaqueBrush) > 0 {
		phases = append(phases, gogpuEntityPhaseOpaqueBrush)
	}
	if len(opaqueAlias) > 0 {
		phases = append(phases, gogpuEntityPhaseOpaqueAlias)
	}
	if hasParticlePhase && particlePhase == gogpuEntityPhaseOpaqueParticles {
		phases = append(phases, gogpuEntityPhaseOpaqueParticles)
	}
	if len(skyBrush) > 0 {
		phases = append(phases, gogpuEntityPhaseSkyBrush)
	}
	if len(opaqueBrush) > 0 {
		phases = append(phases, gogpuEntityPhaseOpaqueLiquidBrush)
	}
	if hasTranslucentWorld {
		phases = append(phases, gogpuEntityPhaseTranslucentWorldLiquid)
	}
	if len(opaqueBrush) > 0 {
		phases = append(phases, gogpuEntityPhaseTranslucentLiquidBrush)
	}
	if len(translucentBrush) > 0 {
		phases = append(phases, gogpuEntityPhaseTranslucentBrush)
	}
	if len(decalMarks) > 0 {
		phases = append(phases, gogpuEntityPhaseDecals)
	}
	if len(translucentAlias) > 0 {
		phases = append(phases, gogpuEntityPhaseTranslucentAlias)
	}
	if len(spriteEntities) > 0 {
		phases = append(phases, gogpuEntityPhaseSprites)
	}
	if hasParticlePhase && particlePhase == gogpuEntityPhaseTranslucentParticles {
		phases = append(phases, gogpuEntityPhaseTranslucentParticles)
	}
	return gogpuEntityDrawPlan{
		opaqueBrush:      opaqueBrush,
		skyBrush:         skyBrush,
		translucentBrush: translucentBrush,
		opaqueAlias:      opaqueAlias,
		translucentAlias: translucentAlias,
		phases:           phases,
	}
}

// gogpuOpaqueAliasPassSteps performs its step in this part of the renderer; this helper exists to keep the phase-internal ordering deterministic and aligned with the primary renderer.
func gogpuOpaqueAliasPassSteps() []gogpuOpaqueAliasStep {
	return []gogpuOpaqueAliasStep{
		gogpuOpaqueAliasStepEntities,
		gogpuOpaqueAliasStepShadows,
	}
}

// worldLiquidFaceTypeMask performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func worldLiquidFaceTypeMask(faces []WorldFace) int32 {
	var mask int32
	for _, face := range faces {
		if face.Flags&model.SurfDrawTurb == 0 {
			continue
		}
		mask |= face.Flags & (model.SurfDrawLava | model.SurfDrawSlime | model.SurfDrawTele | model.SurfDrawWater)
	}
	return mask
}

// hasTranslucentWorldLiquidFaceType performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func hasTranslucentWorldLiquidFaceType(mask int32, liquidAlpha worldLiquidAlphaSettings) bool {
	if mask&model.SurfDrawLava != 0 && !isFullyOpaqueAlpha(liquidAlpha.lava) {
		return true
	}
	if mask&model.SurfDrawSlime != 0 && !isFullyOpaqueAlpha(liquidAlpha.slime) {
		return true
	}
	if mask&model.SurfDrawTele != 0 && !isFullyOpaqueAlpha(liquidAlpha.tele) {
		return true
	}
	if mask&model.SurfDrawWater != 0 && !isFullyOpaqueAlpha(liquidAlpha.water) {
		return true
	}
	return false
}
