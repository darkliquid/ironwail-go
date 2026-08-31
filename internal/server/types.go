// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.
//
// Type definitions have been moved to internal/server/types. Aliases below
// preserve backward compatibility so all existing references (server.MoveType,
// server.SolidType, etc.) continue to resolve without changes.
package server

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// Type aliases re-exporting shared types from the types sub-package.
// StaticSound, UserCmd, DeadFlag, TakeDamage, ClientState, SignonStage,
// NetMessageType, and ServerNetMessage are aliased in their respective
// domain files (types_entities.go, types_flags.go, types_protocol.go).
type (
	Edict          = srvtypes.Edict
	Client         = srvtypes.Client
	TraceResult    = srvtypes.TraceResult
	MoveType       = srvtypes.MoveType
	SolidType      = srvtypes.SolidType
	ServerState    = srvtypes.ServerState
	EntityState    = srvtypes.EntityState
	CollisionModel = srvtypes.CollisionModel
	AreaNode       = srvtypes.AreaNode
	MessageBuffer  = srvtypes.MessageBuffer
	ProtocolFlags  = srvtypes.ProtocolFlags
)

const (
	ProtocolFlagShortAngle  = srvtypes.ProtocolFlagShortAngle
	ProtocolFlagFloatAngle  = srvtypes.ProtocolFlagFloatAngle
	ProtocolFlag24BitCoord  = srvtypes.ProtocolFlag24BitCoord
	ProtocolFlagFloatCoord  = srvtypes.ProtocolFlagFloatCoord
	ProtocolFlagEdictScale  = srvtypes.ProtocolFlagEdictScale
	ProtocolFlagAlphaSanity = srvtypes.ProtocolFlagAlphaSanity
	ProtocolFlagInt32Coord  = srvtypes.ProtocolFlagInt32Coord
)

const (
	DistEpsilon = srvtypes.DistEpsilon
	AreaDepth   = srvtypes.AreaDepth
	AreaNodes   = srvtypes.AreaNodes
)

// MoveType constants
const (
	MoveTypeNone        = srvtypes.MoveTypeNone
	MoveTypeAngleNoClip = srvtypes.MoveTypeAngleNoClip
	MoveTypeAngleClip   = srvtypes.MoveTypeAngleClip
	MoveTypeWalk        = srvtypes.MoveTypeWalk
	MoveTypeStep        = srvtypes.MoveTypeStep
	MoveTypeFly         = srvtypes.MoveTypeFly
	MoveTypeToss        = srvtypes.MoveTypeToss
	MoveTypePush        = srvtypes.MoveTypePush
	MoveTypeNoClip      = srvtypes.MoveTypeNoClip
	MoveTypeFlyMissile  = srvtypes.MoveTypeFlyMissile
	MoveTypeBounce      = srvtypes.MoveTypeBounce
	MoveTypeGib         = srvtypes.MoveTypeGib
)

// Solid constants
const (
	SolidNot      = srvtypes.SolidNot
	SolidTrigger  = srvtypes.SolidTrigger
	SolidBBox     = srvtypes.SolidBBox
	SolidSlideBox = srvtypes.SolidSlideBox
	SolidBSP      = srvtypes.SolidBSP
)

// Hull move type constants
const (
	MoveNormal     = srvtypes.MoveNormal
	MoveNoMonsters = srvtypes.MoveNoMonsters
	MoveMissile    = srvtypes.MoveMissile
)

// ServerState constants
const (
	ServerStateLoading = srvtypes.ServerStateLoading
	ServerStateActive  = srvtypes.ServerStateActive
)

// Physics constants
const (
	MoveEpsilon = srvtypes.MoveEpsilon
	StopEpsilon = srvtypes.StopEpsilon
)

// Default physics/sound constants
const (
	DefaultSoundVolume      = srvtypes.DefaultSoundVolume
	DefaultSoundAttenuation = srvtypes.DefaultSoundAttenuation
	ViewHeight              = srvtypes.ViewHeight
	OneEpsilon              = srvtypes.OneEpsilon
)

// Vector math helper functions re-exported from the types sub-package.
func VecAdd(a, b qtypes.Vec3) qtypes.Vec3           { return srvtypes.VecAdd(a, b) }
func VecSub(a, b qtypes.Vec3) qtypes.Vec3           { return srvtypes.VecSub(a, b) }
func VecScale(v qtypes.Vec3, s float32) qtypes.Vec3 { return srvtypes.VecScale(v, s) }
func VecLen(v qtypes.Vec3) float32                  { return srvtypes.VecLen(v) }
func VecNormalize(v *qtypes.Vec3) float32           { return srvtypes.VecNormalize(v) }
func VecDot(a, b qtypes.Vec3) float32               { return srvtypes.VecDot(a, b) }
func VecCopy(src qtypes.Vec3, dst *qtypes.Vec3)     { srvtypes.VecCopy(src, dst) }
func VecCross(a, b qtypes.Vec3) qtypes.Vec3         { return srvtypes.VecCross(a, b) }
