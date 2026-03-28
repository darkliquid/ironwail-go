package quakego

import "github.com/darkliquid/ironwail-go/pkg/qgo/quake"

const (
	FALSE = 0
	TRUE  = 1

	FL_FLY            = 1
	FL_SWIM           = 2
	FL_CLIENT         = 8
	FL_INWATER        = 16
	FL_MONSTER        = 32
	FL_GODMODE        = 64
	FL_NOTARGET       = 128
	FL_ITEM           = 256
	FL_ONGROUND       = 512
	FL_PARTIALGROUND  = 1024
	FL_WATERJUMP      = 2048
	FL_JUMPRELEASED   = 4096
	FL_ISBOT          = 8192
	FL_NO_PLAYERS     = 16384
	FL_NO_MONSTERS    = 32768
	FL_NO_BOTS        = 65536
	FL_OBJECTIVE      = 131072

	MOVETYPE_NONE       = 0
	MOVETYPE_WALK       = 3
	MOVETYPE_STEP       = 4
	MOVETYPE_FLY        = 5
	MOVETYPE_TOSS       = 6
	MOVETYPE_PUSH       = 7
	MOVETYPE_NOCLIP     = 8
	MOVETYPE_FLYMISSILE = 9
	MOVETYPE_BOUNCE     = 10
	MOVETYPE_GIB        = 11

	SOLID_NOT      = 0
	SOLID_TRIGGER  = 1
	SOLID_BBOX     = 2
	SOLID_SLIDEBOX = 3
	SOLID_BSP      = 4
	SOLID_CORPSE   = 5

	RANGE_MELEE = 0
	RANGE_NEAR  = 1
	RANGE_MID   = 2
	RANGE_FAR   = 3

	DEAD_NO          = 0
	DEAD_DYING       = 1
	DEAD_DEAD        = 2
	DEAD_RESPAWNABLE = 3

	DAMAGE_NO  = 0
	DAMAGE_YES = 1
	DAMAGE_AIM = 2

	IT_AXE              = 4096
	IT_SHOTGUN          = 1
	IT_SUPER_SHOTGUN    = 2
	IT_NAILGUN          = 4
	IT_SUPER_NAILGUN    = 8
	IT_GRENADE_LAUNCHER = 16
	IT_ROCKET_LAUNCHER  = 32
	IT_LIGHTNING        = 64
	IT_EXTRA_WEAPON     = 128
	IT_SHELLS           = 256
	IT_NAILS            = 512
	IT_ROCKETS          = 1024
	IT_CELLS            = 2048
	IT_ARMOR1           = 8192
	IT_ARMOR2           = 16384
	IT_ARMOR3           = 32768
	IT_SUPERHEALTH      = 65536
	IT_KEY1             = 131072
	IT_KEY2             = 262144
	IT_INVISIBILITY     = 524288
	IT_INVULNERABILITY  = 1048576
	IT_SUIT             = 2097152
	IT_QUAD             = 4194304

	CONTENT_EMPTY = -1
	CONTENT_SOLID = -2
	CONTENT_WATER = -3
	CONTENT_SLIME = -4
	CONTENT_LAVA  = -5
	CONTENT_SKY   = -6

	STATE_TOP    = 0
	STATE_BOTTOM = 1
	STATE_UP     = 2
	STATE_DOWN   = 3

	SVC_TEMPENTITY     = 23
	SVC_KILLEDMONSTER  = 27
	SVC_FOUNDSECRET    = 28
	SVC_INTERMISSION   = 30
	SVC_FINALE         = 31
	SVC_CDTRACK        = 32
	SVC_SELLSCREEN     = 33
	SVC_SPAWNEDMONSTER = 39
	SVC_ACHIEVEMENT    = 52
	SVC_CHAT           = 53
	SVC_LEVELCOMPLETED = 54
	SVC_BACKTOLOBBY    = 55
	SVC_LOCALSOUND     = 56
	SVC_PROMPT         = 57

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

	CHAN_AUTO   = 0
	CHAN_WEAPON = 1
	CHAN_VOICE  = 2
	CHAN_ITEM   = 3
	CHAN_BODY   = 4

	ATTN_NONE   = 0
	ATTN_NORM   = 1
	ATTN_IDLE   = 2
	ATTN_STATIC = 3

	UPDATE_GENERAL = 0
	UPDATE_STATIC  = 1
	UPDATE_BINARY  = 2
	UPDATE_TEMP    = 3

	EF_BRIGHTFIELD  = 1
	EF_MUZZLEFLASH  = 2
	EF_BRIGHTLIGHT  = 4
	EF_DIMLIGHT     = 8
	EF_QUADLIGHT    = 16
	EF_PENTALIGHT   = 32
	EF_CANDLELIGHT  = 64

	MSG_BROADCAST = 0
	MSG_ONE       = 1
	MSG_ALL       = 2
	MSG_INIT      = 3

	TEAM_NONE     = -1
	TEAM_MONSTERS = 0
	TEAM_HUMANS   = 1

	COLOR_RED    = 251
	COLOR_GREEN  = 184
	COLOR_BLUE   = 208
	COLOR_YELLOW = 192
	COLOR_WHITE  = 254
	COLOR_BLACK  = 0
	COLOR_CYAN   = 244
	COLOR_ORANGE = 95

	WORLDTYPE_MEDIEVAL = 0
	WORLDTYPE_METAL    = 1
	WORLDTYPE_BASE     = 2

	AS_STRAIGHT = 1
	AS_SLIDING  = 2
	AS_MELEE    = 3
	AS_MISSILE  = 4

	CS_NONE   = 0
	CS_RANGED = 1
	CS_MELEE  = 2
	CS_MIXED  = 3
)

var (
	// System Globals
	Self           *quake.Entity //qgo:self
	Other          *quake.Entity //qgo:other
	World          *quake.Entity //qgo:world
	Time           float32       //qgo:time
	Frametime      float32       //qgo:frametime
	ForceRetouch   float32       //qgo:force_retouch
	MapName        string        //qgo:mapname
	Deathmatch     float32       //qgo:deathmatch
	Coop           float32       //qgo:coop
	Teamplay       float32       //qgo:teamplay
	ServerFlags    float32       //qgo:serverflags
	TotalSecrets   float32       //qgo:total_secrets
	TotalMonsters  float32       //qgo:total_monsters
	FoundSecrets   float32       //qgo:found_secrets
	KilledMonsters float32      //qgo:killed_monsters

	Parm1 float32 //qgo:parm1
	Parm2 float32 //qgo:parm2
	Parm3 float32 //qgo:parm3
	Parm4 float32 //qgo:parm4
	Parm5 float32 //qgo:parm5
	Parm6 float32 //qgo:parm6
	Parm7 float32 //qgo:parm7
	Parm8 float32 //qgo:parm8
	Parm9 float32 //qgo:parm9
	Parm10 float32 //qgo:parm10
	Parm11 float32 //qgo:parm11
	Parm12 float32 //qgo:parm12
	Parm13 float32 //qgo:parm13
	Parm14 float32 //qgo:parm14
	Parm15 float32 //qgo:parm15
	Parm16 float32 //qgo:parm16

	VForward quake.Vec3 //qgo:v_forward
	VUp      quake.Vec3 //qgo:v_up
	VRight   quake.Vec3 //qgo:v_right

	TraceAllSolid     float32    //qgo:trace_allsolid
	TraceStartSolid   float32    //qgo:trace_startsolid
	TraceFraction     float32    //qgo:trace_fraction
	TraceEndPos       quake.Vec3 //qgo:trace_endpos
	TracePlaneNormal  quake.Vec3 //qgo:trace_plane_normal
	TracePlaneDist    float32    //qgo:trace_plane_dist
	TraceEnt          *quake.Entity //qgo:trace_ent
	TraceInOpen       float32    //qgo:trace_inopen
	TraceInWater      float32    //qgo:trace_inwater
	MsgEntity         *quake.Entity //qgo:msg_entity

	// Game Globals
	Movedist float32
	Gameover float32

	StringNull string //qgo:string_null

	Newmis          *quake.Entity
	Activator       *quake.Entity
	DamageAttacker  *quake.Entity
	Framecount      float32
	Skill           float32
	CampaignValid   float32
	Campaign        float32
	CheatsAllowed   float32

	ModelIndexPlayer float32
	ModelIndexEyes   float32

	BodyqueueHead *quake.Entity

	VEC_ORIGIN    = quake.Vec3{0, 0, 0}

	VEC_HULL_MIN  = quake.MakeVec3(-16, -16, -24)
	VEC_HULL_MAX  = quake.MakeVec3(16, 16, 32)
	VEC_HULL2_MIN = quake.MakeVec3(-32, -32, -24)
	VEC_HULL2_MAX = quake.MakeVec3(32, 32, 64)
)
