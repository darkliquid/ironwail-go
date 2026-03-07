package renderer

type worldBrushPassSelector int

const (
	worldBrushPassAll worldBrushPassSelector = iota
	worldBrushPassNonLiquid
	worldBrushPassLiquidOnly
)

func normalizeWorldBrushPassSelector(selector worldBrushPassSelector) worldBrushPassSelector {
	switch selector {
	case worldBrushPassAll, worldBrushPassNonLiquid, worldBrushPassLiquidOnly:
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

func isFullyOpaqueAlpha(alpha float32) bool {
	return alpha >= 1
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
