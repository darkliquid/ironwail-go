package renderer

import "github.com/ironwail/ironwail-go/internal/model"

type worldRenderPass int

const (
	worldPassSky worldRenderPass = iota
	worldPassOpaque
	worldPassAlphaTest
	worldPassTranslucent
)

func worldFaceAlpha(flags int32, liquidAlpha worldLiquidAlphaSettings) float32 {
	if flags&model.SurfDrawTurb == 0 {
		return 1
	}
	if flags&model.SurfDrawLava != 0 {
		return liquidAlpha.lava
	}
	if flags&model.SurfDrawSlime != 0 {
		return liquidAlpha.slime
	}
	if flags&model.SurfDrawTele != 0 {
		return liquidAlpha.tele
	}
	if flags&model.SurfDrawWater != 0 {
		return liquidAlpha.water
	}
	return 1
}

func worldFaceUsesTurb(flags int32) bool {
	return flags&model.SurfDrawTurb != 0 && flags&model.SurfDrawSky == 0
}

func worldFaceIsLiquid(flags int32) bool {
	return flags&(model.SurfDrawLava|model.SurfDrawSlime|model.SurfDrawTele|model.SurfDrawWater) != 0
}

func worldFacePass(flags int32, alpha float32) worldRenderPass {
	switch {
	case flags&model.SurfDrawSky != 0:
		return worldPassSky
	case flags&model.SurfDrawFence != 0:
		return worldPassAlphaTest
	case alpha < 1:
		return worldPassTranslucent
	default:
		return worldPassOpaque
	}
}

func worldFaceDistanceSq(center [3]float32, camera CameraState) float32 {
	dx := center[0] - camera.Origin.X
	dy := center[1] - camera.Origin.Y
	dz := center[2] - camera.Origin.Z
	return dx*dx + dy*dy + dz*dz
}
