// This file belongs to the Entity/QC subsystem: edict allocation, entity accessors, QuakeC field offsets, QC call tracing, and entity state types.
//
// DeadFlag, TakeDamage, EntityFlags, and EntityEffects have been moved to
// internal/server/types. Aliases below preserve backward compatibility.
package server

import srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"

// Type aliases for types moved to the types sub-package.
type (
	DeadFlag   = srvtypes.DeadFlag
	TakeDamage = srvtypes.TakeDamage
)

// DeadFlag constants
const (
	DeadNo          = srvtypes.DeadNo
	DeadDying       = srvtypes.DeadDying
	DeadDead        = srvtypes.DeadDead
	DeadRespawnable = srvtypes.DeadRespawnable
)

// TakeDamage constants
const (
	DamageNo  = srvtypes.DamageNo
	DamageYes = srvtypes.DamageYes
	DamageAim = srvtypes.DamageAim
)

// EntityFlags constants
const (
	FlagFly           = srvtypes.FlagFly
	FlagSwim          = srvtypes.FlagSwim
	FlagConveyor      = srvtypes.FlagConveyor
	FlagClient        = srvtypes.FlagClient
	FlagInWater       = srvtypes.FlagInWater
	FlagMonster       = srvtypes.FlagMonster
	FlagGodMode       = srvtypes.FlagGodMode
	FlagNoTarget      = srvtypes.FlagNoTarget
	FlagItem          = srvtypes.FlagItem
	FlagOnGround      = srvtypes.FlagOnGround
	FlagPartialGround = srvtypes.FlagPartialGround
	FlagWaterJump     = srvtypes.FlagWaterJump
	FlagJumpReleased  = srvtypes.FlagJumpReleased
)

// EntityEffects constants
const (
	EffectBrightField = srvtypes.EffectBrightField
	EffectMuzzleFlash = srvtypes.EffectMuzzleFlash
	EffectBrightLight = srvtypes.EffectBrightLight
	EffectDimLight    = srvtypes.EffectDimLight
	EffectQuadLight   = srvtypes.EffectQuadLight
	EffectPentaLight  = srvtypes.EffectPentaLight
	EffectCandleLight = srvtypes.EffectCandleLight
)
