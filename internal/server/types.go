// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.
//
// Type definitions have been moved to internal/server/types. Aliases below
// preserve backward compatibility so all existing references (server.MoveType,
// server.SolidType, etc.) continue to resolve without changes.
package server

import srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"

// Type aliases re-exporting shared types from the types sub-package.
// StaticSound, UserCmd, DeadFlag, TakeDamage, ClientState, SignonStage,
// NetMessageType, and ServerNetMessage are aliased in their respective
// domain files (types_entities.go, types_flags.go, types_protocol.go).
type (
	Edict           = srvtypes.Edict
	TraceResult     = srvtypes.TraceResult
	MoveType        = srvtypes.MoveType
	SolidType       = srvtypes.SolidType
	ServerState     = srvtypes.ServerState
	EntityState     = srvtypes.EntityState
	CollisionModel  = srvtypes.CollisionModel
	AreaNode        = srvtypes.AreaNode
)


const (
	DistEpsilon = srvtypes.DistEpsilon
	AreaDepth   = srvtypes.AreaDepth
	AreaNodes   = srvtypes.AreaNodes
)





// MoveType constants
const (
	MoveTypeNone       = srvtypes.MoveTypeNone
	MoveTypeAngleNoClip = srvtypes.MoveTypeAngleNoClip
	MoveTypeAngleClip  = srvtypes.MoveTypeAngleClip
	MoveTypeWalk       = srvtypes.MoveTypeWalk
	MoveTypeStep       = srvtypes.MoveTypeStep
	MoveTypeFly        = srvtypes.MoveTypeFly
	MoveTypeToss       = srvtypes.MoveTypeToss
	MoveTypePush       = srvtypes.MoveTypePush
	MoveTypeNoClip     = srvtypes.MoveTypeNoClip
	MoveTypeFlyMissile = srvtypes.MoveTypeFlyMissile
	MoveTypeBounce     = srvtypes.MoveTypeBounce
	MoveTypeGib        = srvtypes.MoveTypeGib

	MoveNormal     = srvtypes.MoveNormal
	MoveNoMonsters = srvtypes.MoveNoMonsters
	MoveMissile    = srvtypes.MoveMissile
)

// SolidType constants
const (
	SolidNot      = srvtypes.SolidNot
	SolidTrigger  = srvtypes.SolidTrigger
	SolidBBox     = srvtypes.SolidBBox
	SolidSlideBox = srvtypes.SolidSlideBox
	SolidBSP      = srvtypes.SolidBSP
)

// ServerState constants
const (
	ServerStateLoading = srvtypes.ServerStateLoading
	ServerStateActive  = srvtypes.ServerStateActive
)

// Physics constants
const (
	MoveEpsilon  = srvtypes.MoveEpsilon
	StopEpsilon  = srvtypes.StopEpsilon
)

// Default physics/sound constants
const (
	DefaultSoundVolume      = srvtypes.DefaultSoundVolume
	DefaultSoundAttenuation = srvtypes.DefaultSoundAttenuation
	ViewHeight              = srvtypes.ViewHeight
	OneEpsilon              = srvtypes.OneEpsilon
)

// Vector math helper functions re-exported from the types sub-package.
func VecAdd(a, b [3]float32) [3]float32    { return srvtypes.VecAdd(a, b) }
func VecSub(a, b [3]float32) [3]float32    { return srvtypes.VecSub(a, b) }
func VecScale(v [3]float32, s float32) [3]float32 { return srvtypes.VecScale(v, s) }
func VecLen(v [3]float32) float32          { return srvtypes.VecLen(v) }
func VecNormalize(v *[3]float32) float32   { return srvtypes.VecNormalize(v) }
func VecDot(a, b [3]float32) float32       { return srvtypes.VecDot(a, b) }
func VecCopy(src [3]float32, dst *[3]float32) { srvtypes.VecCopy(src, dst) }
func VecCross(a, b [3]float32) [3]float32 { return srvtypes.VecCross(a, b) }

