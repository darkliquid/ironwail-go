package renderer

import (
	"math"

	aliasimpl "github.com/darkliquid/ironwail-go/internal/renderer/alias"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	LerpResetAnim  = aliasimpl.LerpResetAnim
	LerpResetAnim2 = aliasimpl.LerpResetAnim2
	LerpResetMove  = aliasimpl.LerpResetMove
	LerpMoveStep   = aliasimpl.LerpMoveStep
	LerpFinish     = aliasimpl.LerpFinish
)

const (
	ModNoLerp = aliasimpl.ModNoLerp
)

type AliasFrame = aliasimpl.AliasFrame
type AliasHeader = aliasimpl.AliasHeader
type AliasEntity = aliasimpl.AliasEntity
type LerpData = aliasimpl.LerpData

func SetupAliasFrame(e *AliasEntity, hdr *AliasHeader, timeSeconds float64, lerpModels bool, demoPlayback bool, demoSpeed float64) (LerpData, error) {
	return aliasimpl.SetupAliasFrame(e, hdr, timeSeconds, lerpModels, demoPlayback, demoSpeed)
}

func SetupEntityTransform(e *AliasEntity, timeSeconds float64, lerpMove bool, isViewEntity bool, chaseActive bool, demoPlayback bool, demoSpeed float64) (origin types.Vec3, angles types.Vec3) {
	return aliasimpl.SetupEntityTransform(e, timeSeconds, lerpMove, isViewEntity, chaseActive, demoPlayback, demoSpeed)
}

// clamp01 remains in renderer root because many non-alias systems use it directly.
func clamp01(v float32) float32 {
	if math.IsNaN(float64(v)) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
