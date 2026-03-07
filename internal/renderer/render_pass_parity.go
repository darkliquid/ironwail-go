package renderer

type worldBrushPassSelector int

const (
	worldBrushPassAll worldBrushPassSelector = iota
	worldBrushPassNonLiquid
	worldBrushPassLiquidOnly
	worldBrushPassSkyOnly
)

func normalizeWorldBrushPassSelector(selector worldBrushPassSelector) worldBrushPassSelector {
	switch selector {
	case worldBrushPassAll, worldBrushPassNonLiquid, worldBrushPassLiquidOnly, worldBrushPassSkyOnly:
		return selector
	default:
		return worldBrushPassAll
	}
}

func (selector worldBrushPassSelector) includesNonLiquid() bool {
	selector = normalizeWorldBrushPassSelector(selector)
	return selector == worldBrushPassAll || selector == worldBrushPassNonLiquid
}

func (selector worldBrushPassSelector) includesLiquid() bool {
	selector = normalizeWorldBrushPassSelector(selector)
	return selector == worldBrushPassAll || selector == worldBrushPassLiquidOnly
}

func (selector worldBrushPassSelector) includesSky() bool {
	selector = normalizeWorldBrushPassSelector(selector)
	return selector == worldBrushPassAll || selector == worldBrushPassSkyOnly
}

func isFullyOpaqueAlpha(alpha float32) bool {
	return alpha >= 1
}

func visibleAliasEntityAlpha(alpha float32) (float32, bool) {
	alpha = clamp01(alpha)
	return alpha, alpha > 0
}

func splitAliasEntitiesByAlpha(entities []AliasModelEntity) (opaque, translucent []AliasModelEntity) {
	if len(entities) == 0 {
		return nil, nil
	}
	opaque = make([]AliasModelEntity, 0, len(entities))
	translucent = make([]AliasModelEntity, 0, len(entities))
	for _, entity := range entities {
		alpha, visible := visibleAliasEntityAlpha(entity.Alpha)
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
