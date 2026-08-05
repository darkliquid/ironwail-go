// This file belongs to the Network/Protocol subsystem: server-to-client message encoding, client management, and protocol types.
//
// ClientState, SignonStage, NetMessageType, ServerNetMessage, and Max*
// constants have been moved to internal/server/types. Aliases below
// preserve backward compatibility.
package server

import srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"

// Type aliases for types moved to the types sub-package.
type (
	ClientState      = srvtypes.ClientState
	SignonStage      = srvtypes.SignonStage
	NetMessageType   = srvtypes.NetMessageType
	ServerNetMessage = srvtypes.ServerNetMessage
)

// ClientState constants
const (
	ClientStateDisconnected = srvtypes.ClientStateDisconnected
	ClientStateConnected    = srvtypes.ClientStateConnected
	ClientStateSpawned      = srvtypes.ClientStateSpawned
)

// SignonStage constants
const (
	SignonNone       = srvtypes.SignonNone
	SignonPrespawn   = srvtypes.SignonPrespawn
	SignonSignonBufs = srvtypes.SignonSignonBufs
	SignonSignonMsg  = srvtypes.SignonSignonMsg
	SignonFlush      = srvtypes.SignonFlush
	SignonDone       = srvtypes.SignonDone
)

// NetMessageType constants
const (
	CLCNop        = srvtypes.CLCNop
	CLCDisconnect = srvtypes.CLCDisconnect
	CLCMove       = srvtypes.CLCMove
	CLCStringCmd  = srvtypes.CLCStringCmd
)

// ServerNetMessage constants
const (
	SVCNop                  = srvtypes.SVCNop
	SVCDamage               = srvtypes.SVCDamage
	SVCDisplayDisconnect    = srvtypes.SVCDisplayDisconnect
	SVCLevelName            = srvtypes.SVCLevelName
	SVCLoaded               = srvtypes.SVCLoaded
	SVCMove                 = srvtypes.SVCMove
	SVCEnterServer          = srvtypes.SVCEnterServer
	SVCSound                = srvtypes.SVCSound
	SVCPrint                = srvtypes.SVCPrint
	SVCSinglePrecisionFrame = srvtypes.SVCSinglePrecisionFrame
	SVCDoublePrecisionFrame = srvtypes.SVCDoublePrecisionFrame
	SVCCreateBaseline       = srvtypes.SVCCreateBaseline
	SVCCreateBaseline2      = srvtypes.SVCCreateBaseline2
	SVCLightStyle           = srvtypes.SVCLightStyle
	SVCTempEntity           = srvtypes.SVCTempEntity
	SVCCenterPrint          = srvtypes.SVCCenterPrint
	SVCKillMonster          = srvtypes.SVCKillMonster
	SVCSpawnBaseline        = srvtypes.SVCSpawnBaseline
	SVCSpawnBaseline2       = srvtypes.SVCSpawnBaseline2
	SVCSpawnStatic          = srvtypes.SVCSpawnStatic
	SVCSpawnStatic2         = srvtypes.SVCSpawnStatic2
	SVCSpawnStaticSound     = srvtypes.SVCSpawnStaticSound
	SVCSpawnStaticSound2    = srvtypes.SVCSpawnStaticSound2
	SVCClientData           = srvtypes.SVCClientData
	SVCDownload             = srvtypes.SVCDownload
	SVCUpdatePing           = srvtypes.SVCUpdatePing
	SVCUpdateFrags          = srvtypes.SVCUpdateFrags
	SVCUpdateStat           = srvtypes.SVCUpdateStat
	SVCParticle             = srvtypes.SVCParticle
	SVCCDTrack              = srvtypes.SVCCDTrack
	SVCLocalSound           = srvtypes.SVCLocalSound
	SVCSetAngle             = srvtypes.SVCSetAngle
	SVCSetView              = srvtypes.SVCSetView
	SVCUpdateUserInfo       = srvtypes.SVCUpdateUserInfo
	SVCSignOnNum            = srvtypes.SVCSignOnNum
	SVCStuffText            = srvtypes.SVCStuffText
	SVCTime                 = srvtypes.SVCTime
	SVCSetInfo              = srvtypes.SVCSetInfo
	SVCServerInfo           = srvtypes.SVCServerInfo
	SVCUpdateEnt            = srvtypes.SVCUpdateEnt
	SVCLocalSound2          = srvtypes.SVCLocalSound2
)

// Max constants for server limits.
const (
	MaxClients       = srvtypes.MaxClients
	MaxModels        = srvtypes.MaxModels
	MaxSounds        = srvtypes.MaxSounds
	MaxEdicts        = srvtypes.MaxEdicts
	MaxDatagram      = srvtypes.MaxDatagram
	MaxSignonBuffers = srvtypes.MaxSignonBuffers
	MaxEntityLeafs   = srvtypes.MaxEntityLeafs
)
