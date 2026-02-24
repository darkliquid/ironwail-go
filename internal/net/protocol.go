// Package net implements Quake network protocol definitions.
//
// This file contains all network protocol constants and structures
// for client-server communication in Quake.
//
// Protocol versions:
//   - PROTOCOL_NETQUAKE (15): Standard Quake protocol
//   - PROTOCOL_FITZQUAKE (666): FitzQuake extensions
//   - PROTOCOL_RMQ (999): RMQ protocol
package net

// Protocol versions
const (
	PROTOCOL_NETQUAKE  = 15  // Standard Quake protocol
	PROTOCOL_FITZQUAKE = 666 // FitzQuake extensions
	PROTOCOL_RMQ       = 999 // RMQ protocol
)

// PROTOCOL_RMQ protocol flags
const (
	PRFL_SHORTANGLE  = 1 << 0
	PRFL_FLOATANGLE  = 1 << 2
	PRFL_24BITCOORD  = 1 << 3
	PRFL_FLOATCOORD  = 1 << 4
	PRFL_EDICTSCALE  = 1 << 5
	PRFL_ALPHASANITY = 1 << 6
	PRFL_INT32COORD  = 1 << 7
	PRFL_MOREFLAGS   = 1 << 31
)

// Server to client message types (svc_*)
const (
	SVCBad              = 0  // Invalid message
	SVCNop              = 1  // No operation
	SVCDisconnect       = 2  // Disconnect client
	SVCUpdateStat       = 3  // [byte] [long]
	SVCVersion          = 4  // [long] server version
	SVCSetView          = 5  // [short] entity number
	SVCSound            = 6  // <see code>
	SVCTime             = 7  // [float] server time
	SVCPrint            = 8  // [string] null terminated string
	SVCStuffText        = 9  // [string] stuffed into client's console buffer
	SVCSetAngle         = 10 // [angle3] set view angle to this absolute value
	SVCServerInfo       = 11 // [long] version, [string] signon, [string]..[0]models, [string]...[0]sounds
	SVCLightStyle       = 12 // [byte] [string]
	SVCUpdateName       = 13 // [byte] [string]
	SVCUpdateFrags      = 14 // [byte] [short]
	SVCClientData       = 15 // <shortbits + data>
	SVCStopSound        = 16 // <see code>
	SVCUpdateColors     = 17 // [byte] [byte]
	SVCParticle         = 18 // [vec3] <variable>
	SVCDamage           = 19
	SVCSpawnStatic      = 20
	SVCSpawnBaseline    = 22
	SVCTempEntity       = 23
	SVCSetPause         = 24 // [byte] on / off
	SVCSignOnNum        = 25 // [byte] used for signon sequence
	SVCCenterPrint      = 26 // [string] to put in center of screen
	SVCKillMonster      = 27
	SVCFoundSecret      = 28
	SVCSpawnStaticSound = 29 // [coord3] [byte] samp [byte] vol [byte] aten
	SVCIntermission     = 30 // [string] music
	SVCFinale           = 31 // [string] music [string] text
	SVCCDTrack          = 32 // [byte] track [byte] looptrack
	SVCSellScreen       = 33
	SVCCutScene         = 34

	// FitzQuake extensions
	SVCSkyBox            = 37 // [string] name
	SVCBF                = 40
	SVCFog               = 41 // [byte] density [byte] red [byte] green [byte] blue [float] time
	SVCSpawnBaseline2    = 42 // support for large modelindex, large framenum, alpha, using flags
	SVCSpawnStatic2      = 43 // support for large modelindex, large framenum, alpha, using flags
	SVCSpawnStaticSound2 = 44 // [coord3] [short] samp [byte] vol [byte] aten

	// 2021 re-release server messages
	SVCBotChat        = 38
	SVCSetViews       = 45
	SVCUpdatePing     = 46
	SVCUpdateSocial   = 47
	SVCUpdatePLInfo   = 48
	SVCRawPrint       = 49
	SVCServerVars     = 50
	SVCSeq            = 51
	SVCAchievement    = 52 // [string] id
	SVCChat           = 53
	SVCLevelCompleted = 54
	SVCBackToLobby    = 55
	SVCLocalSound     = 56
)

// Client to server message types (clc_*)
const (
	CLCBad        = 0 // Invalid message
	CLCNop        = 1
	CLCDisconnect = 2
	CLCMove       = 3 // [usercmd_t]
	CLCStringCmd  = 4 // [string] message
)

// Temp entity events (TE_*)
const (
	TE_SPIKE        = 0
	TE_SUPERSPIKE   = 1
	TE_GUNSHOT      = 2
	TE_EXPLOSION    = 3
	TE_TAREXPLOSION = 4
	TE_LIGHTNING1   = 5
	TE_LIGHTNING2   = 6
	TE_WIZSPIKE     = 7
	TE_KNIGHTSPIKE  = 8
	TE_LIGHTNING3   = 9
	TE_LAVASPLASH   = 10
	TE_TELEPORT     = 11
	TE_EXPLOSION2   = 12
	TE_BEAM         = 13
)

// Sound flags
const (
	SND_VOLUME      = 1 << 0 // a byte
	SND_ATTENUATION = 1 << 1 // a byte
	SND_LOOPING     = 1 << 2 // a long

	SND_LARGEENTITY = 1 << 3 // a short + byte (instead of just a short)
	SND_LARGESOUND  = 1 << 4 // a short soundindex (instead of a byte)
)

const (
	DEFAULT_SOUND_PACKET_VOLUME      = 255
	DEFAULT_SOUND_PACKET_ATTENUATION = 1.0
)

// Baseline flags (B_*)
const (
	BLARGEMODEL = 1 << 0 // modelindex is short instead of byte
	BLARGEFRAME = 1 << 1 // frame is short instead of byte
	BALPHA      = 1 << 2 // 1 byte, uses ENTALPHA_ENCODE, not sent if ENTALPHA_DEFAULT
	BSCALE      = 1 << 3
)

// Alpha encoding constants
const (
	ENTALPHA_DEFAULT = 0   // entity's alpha is "default" (i.e. water obeys r_wateralpha) -- must be zero
	ENTALPHA_ZERO    = 1   // entity is invisible (lowest possible alpha)
	ENTALPHA_ONE     = 255 // entity is fully opaque (highest possible alpha)
)

// ENTALPHA_OPAQUE checks if entity is opaque (alpha==0 or alpha==255)
func ENTALPHA_OPAQUE(a byte) bool {
	return byte(a-1) >= 254
}

// ENTALPHA_ENCODE converts float alpha (0-1) to byte for transmission
func ENTALPHA_ENCODE(a float32) byte {
	if a == 0 {
		return ENTALPHA_DEFAULT
	}
	clamped := a
	if clamped < 0 {
		clamped = 0
	} else if clamped > 1 {
		clamped = 1
	}
	return byte(clamped*254.0 + 0.5)
}

// ENTALPHA_DECODE converts byte alpha back to float for rendering
func ENTALPHA_DECODE(a byte) float32 {
	if a == ENTALPHA_DEFAULT {
		return 1.0
	}
	return float32(a-1) / 254.0
}

// ENTALPHA_TOSAVE converts byte alpha to float for savegame
func ENTALPHA_TOSAVE(a byte) float32 {
	if a == ENTALPHA_DEFAULT {
		return 0.0
	}
	if a == ENTALPHA_ZERO {
		return -1.0
	}
	return float32(a-1) / 254.0
}

// Scale encoding constants
const (
	ENTSCALE_DEFAULT = 16 // Equivalent to float 1.0f due to byte packing
)

// ENTSCALE_ENCODE converts float scale to byte
func ENTSCALE_ENCODE(a float32) byte {
	if a == 0 {
		return ENTSCALE_DEFAULT
	}
	return byte(a * ENTSCALE_DEFAULT)
}

// ENTSCALE_DECODE converts byte scale to float for rendering
func ENTSCALE_DECODE(a byte) float32 {
	return float32(a) / float32(ENTSCALE_DEFAULT)
}

// Client info defaults
const (
	DEFAULT_VIEWHEIGHT = 22
)

// Game types sent by serverinfo
// These determine which intermission screen plays
const (
	GAME_COOP       = 0
	GAME_DEATHMATCH = 1
)

// EntityState represents entity baseline state sent to clients.
// Corresponds to entity_state_t in protocol.h
type EntityState struct {
	Origin     [3]float32 // Entity position
	Angles     [3]float32 // Entity orientation
	ModelIndex uint16     // Index into model cache (FitzQuake extension)
	Frame      uint16     // Animation frame number (FitzQuake extension)
	Colormap   uint8      // Player colormap
	Skin       uint8      // Model skin number
	Effects    int        // Visual effect flags
	Alpha      uint8      // Transparency value (FitzQuake extension)
	Scale      uint8      // Model scale (FitzQuake extension)
}

// UserCmd represents client input commands sent to server.
// Contains movement, angles, and impulse values for a single frame.
// Corresponds to usercmd_t in protocol.h
type UserCmd struct {
	ViewAngles  [3]float32 // Client view angles (pitch, yaw, roll)
	ForwardMove float32    // Forward/backward movement (-back, +forward)
	SideMove    float32    // Strafe movement (-left, +right)
	UpMove      float32    // Vertical movement (jump/swim)
	Buttons     uint8      // Button state (attack, jump, etc.)
	Impulse     uint8      // Weapon/impulse command
}

// ServerFrame represents server game state snapshot
type ServerFrame struct {
	Num      int     // Frame number
	MaxEdict int     // Maximum edict number
	Entities []byte  // Entity update data
	SendTime float32 // When this frame was generated
}

// ClientFrame represents client state snapshot
type ClientFrame struct {
	Num      int     // Frame number
	SendTime float32 // When this frame was sent
	// Note: Client frame structure is more complex in full implementation
}

// Update flags for SVCClientData (SU_*)
const (
	SU_VIEWHEIGHT   = 1 << 0
	SU_IDEALPITCH   = 1 << 1
	SU_PUNCH1       = 1 << 2
	SU_PUNCH2       = 1 << 3
	SU_PUNCH3       = 1 << 4
	SU_VELOCITY1    = 1 << 5
	SU_VELOCITY2    = 1 << 6
	SU_VELOCITY3    = 1 << 7
	SU_UNUSED8      = 1 << 8 // AVAILABLE BIT
	SU_ITEMS        = 1 << 9
	SU_ONGROUND     = 1 << 10 // no data follows, bit is it
	SU_INWATER      = 1 << 11 // no data follows, bit is it
	SU_WEAPONFRAME  = 1 << 12
	SU_ARMOR        = 1 << 13
	SU_WEAPON       = 1 << 14
	SU_EXTEND1      = 1 << 15 // another byte to follow
	SU_WEAPON2      = 1 << 16 // 1 byte, this is .weaponmodel & 0xFF00 (second byte)
	SU_ARMOR2       = 1 << 17 // 1 byte, this is .armorvalue & 0xFF00 (second byte)
	SU_AMMO2        = 1 << 18 // 1 byte, this is .currentammo & 0xFF00 (second byte)
	SU_SHELLS2      = 1 << 19 // 1 byte, this is .ammo_shells & 0xFF00 (second byte)
	SU_NAILS2       = 1 << 20 // 1 byte, this is .ammo_nails & 0xFF00 (second byte)
	SU_ROCKETS2     = 1 << 21 // 1 byte, this is .ammo_rockets & 0xFF00 (second byte)
	SU_CELLS2       = 1 << 22 // 1 byte, this is .ammo_cells & 0xFF00 (second byte)
	SU_EXTEND2      = 1 << 23 // another byte to follow
	SU_WEAPONFRAME2 = 1 << 24 // 1 byte, this is .weaponframe & 0xFF00 (second byte)
	SU_WEAPONALPHA  = 1 << 25 // 1 byte, this is alpha for weaponmodel, uses ENTALPHA_ENCODE, not sent if ENTALPHA_DEFAULT
	SU_UNUSED26     = 1 << 26
	SU_UNUSED27     = 1 << 27
	SU_UNUSED28     = 1 << 28
	SU_UNUSED29     = 1 << 29
	SU_UNUSED30     = 1 << 30
	SU_EXTEND3      = 1 << 31 // another byte to follow, future expansion
)

// Update flags for SVCUpdate (U_*)
const (
	U_MOREBITS   = 1 << 0
	U_ORIGIN1    = 1 << 1
	U_ORIGIN2    = 1 << 2
	U_ORIGIN3    = 1 << 3
	U_ANGLE2     = 1 << 4
	U_STEP       = 1 << 5
	U_FRAME      = 1 << 6
	U_SIGNAL     = 1 << 7
	U_ANGLE1     = 1 << 8
	U_ANGLE3     = 1 << 9
	U_MODEL      = 1 << 10
	U_COLORMAP   = 1 << 11
	U_SKIN       = 1 << 12
	U_EFFECTS    = 1 << 13
	U_LONGENTITY = 1 << 14
	U_EXTEND1    = 1 << 15
	U_ALPHA      = 1 << 16 // 1 byte, uses ENTALPHA_ENCODE, not sent if equal to baseline
	U_FRAME2     = 1 << 17 // 1 byte, this is .frame & 0xFF00 (second byte)
	U_MODEL2     = 1 << 18 // 1 byte, this is .modelindex & 0xFF00 (second byte)
	U_LERPFINISH = 1 << 19 // 1 byte, 0.0-1.0 maps to 0-255, not sent if exactly 0.1, this is ent->v.nextthink - sv.time
	U_SCALE      = 1 << 20 // 1 byte, for PROTOCOL_RMQ PRFL_EDICTSCALE
	U_UNUSED21   = 1 << 21
	U_UNUSED22   = 1 << 22
	U_EXTEND2    = 1 << 23 // another byte to follow, future expansion
)
