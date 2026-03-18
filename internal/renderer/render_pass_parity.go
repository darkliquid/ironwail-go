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
