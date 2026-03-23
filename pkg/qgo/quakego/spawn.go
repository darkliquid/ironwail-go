package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Tarbaby (Spawn) frames
	TBABY_FRAME_walk1 = iota
	TBABY_FRAME_walk2
	TBABY_FRAME_walk3
	TBABY_FRAME_walk4
	TBABY_FRAME_walk5
	TBABY_FRAME_walk6
	TBABY_FRAME_walk7
	TBABY_FRAME_walk8
	TBABY_FRAME_walk9
	TBABY_FRAME_walk10
	TBABY_FRAME_walk11
	TBABY_FRAME_walk12
	TBABY_FRAME_walk13
	TBABY_FRAME_walk14
	TBABY_FRAME_walk15
	TBABY_FRAME_walk16
	TBABY_FRAME_walk17
	TBABY_FRAME_walk18
	TBABY_FRAME_walk19
	TBABY_FRAME_walk20
	TBABY_FRAME_walk21
	TBABY_FRAME_walk22
	TBABY_FRAME_walk23
	TBABY_FRAME_walk24
	TBABY_FRAME_walk25

	TBABY_FRAME_run1
	TBABY_FRAME_run2
	TBABY_FRAME_run3
	TBABY_FRAME_run4
	TBABY_FRAME_run5
	TBABY_FRAME_run6
	TBABY_FRAME_run7
	TBABY_FRAME_run8
	TBABY_FRAME_run9
	TBABY_FRAME_run10
	TBABY_FRAME_run11
	TBABY_FRAME_run12
	TBABY_FRAME_run13
	TBABY_FRAME_run14
	TBABY_FRAME_run15
	TBABY_FRAME_run16
	TBABY_FRAME_run17
	TBABY_FRAME_run18
	TBABY_FRAME_run19
	TBABY_FRAME_run20
	TBABY_FRAME_run21
	TBABY_FRAME_run22
	TBABY_FRAME_run23
	TBABY_FRAME_run24
	TBABY_FRAME_run25

	TBABY_FRAME_jump1
	TBABY_FRAME_jump2
	TBABY_FRAME_jump3
	TBABY_FRAME_jump4
	TBABY_FRAME_jump5
	TBABY_FRAME_jump6

	TBABY_FRAME_fly1
	TBABY_FRAME_fly2
	TBABY_FRAME_fly3
	TBABY_FRAME_fly4

	TBABY_FRAME_exp
)

// Prototyped elsewhere
var tbaby_stand1 func()
var tbaby_walk1 func()
var tbaby_run1 func()
var tbaby_jump1 func()
var tbaby_jump5 func()
var tbaby_fly1 func()

func tbaby_stand1_impl() { Self.Frame = float32(TBABY_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = tbaby_stand1_impl; ai_stand() }
func tbaby_hang1()       { Self.Frame = float32(TBABY_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = tbaby_hang1; ai_stand() }

func tbaby_walk1_impl()  { Self.Frame = float32(TBABY_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk2; ai_turn() }
func tbaby_walk2()  { Self.Frame = float32(TBABY_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk3; ai_turn() }
func tbaby_walk3()  { Self.Frame = float32(TBABY_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk4; ai_turn() }
func tbaby_walk4()  { Self.Frame = float32(TBABY_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk5; ai_turn() }
func tbaby_walk5()  { Self.Frame = float32(TBABY_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk6; ai_turn() }
func tbaby_walk6()  { Self.Frame = float32(TBABY_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk7; ai_turn() }
func tbaby_walk7()  { Self.Frame = float32(TBABY_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk8; ai_turn() }
func tbaby_walk8()  { Self.Frame = float32(TBABY_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk9; ai_turn() }
func tbaby_walk9()  { Self.Frame = float32(TBABY_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk10; ai_turn() }
func tbaby_walk10() { Self.Frame = float32(TBABY_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk11; ai_turn() }
func tbaby_walk11() { Self.Frame = float32(TBABY_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk12; ai_walk(2) }
func tbaby_walk12() { Self.Frame = float32(TBABY_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk13; ai_walk(2) }
func tbaby_walk13() { Self.Frame = float32(TBABY_FRAME_walk13); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk14; ai_walk(2) }
func tbaby_walk14() { Self.Frame = float32(TBABY_FRAME_walk14); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk15; ai_walk(2) }
func tbaby_walk15() { Self.Frame = float32(TBABY_FRAME_walk15); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk16; ai_walk(2) }
func tbaby_walk16() { Self.Frame = float32(TBABY_FRAME_walk16); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk17; ai_walk(2) }
func tbaby_walk17() { Self.Frame = float32(TBABY_FRAME_walk17); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk18; ai_walk(2) }
func tbaby_walk18() { Self.Frame = float32(TBABY_FRAME_walk18); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk19; ai_walk(2) }
func tbaby_walk19() { Self.Frame = float32(TBABY_FRAME_walk19); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk20; ai_walk(2) }
func tbaby_walk20() { Self.Frame = float32(TBABY_FRAME_walk20); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk21; ai_walk(2) }
func tbaby_walk21() { Self.Frame = float32(TBABY_FRAME_walk21); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk22; ai_walk(2) }
func tbaby_walk22() { Self.Frame = float32(TBABY_FRAME_walk22); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk23; ai_walk(2) }
func tbaby_walk23() { Self.Frame = float32(TBABY_FRAME_walk23); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk24; ai_walk(2) }
func tbaby_walk24() { Self.Frame = float32(TBABY_FRAME_walk24); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk25; ai_walk(2) }
func tbaby_walk25() { Self.Frame = float32(TBABY_FRAME_walk25); Self.NextThink = Time + 0.1; Self.Think = tbaby_walk1_impl; ai_walk(2) }

func tbaby_run1_impl()  { Self.Frame = float32(TBABY_FRAME_run1); Self.NextThink = Time + 0.1; Self.Think = tbaby_run2; ai_face() }
func tbaby_run2()  { Self.Frame = float32(TBABY_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = tbaby_run3; ai_face() }
func tbaby_run3()  { Self.Frame = float32(TBABY_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = tbaby_run4; ai_face() }
func tbaby_run4()  { Self.Frame = float32(TBABY_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = tbaby_run5; ai_face() }
func tbaby_run5()  { Self.Frame = float32(TBABY_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = tbaby_run6; ai_face() }
func tbaby_run6()  { Self.Frame = float32(TBABY_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = tbaby_run7; ai_face() }
func tbaby_run7()  { Self.Frame = float32(TBABY_FRAME_run7); Self.NextThink = Time + 0.1; Self.Think = tbaby_run8; ai_face() }
func tbaby_run8()  { Self.Frame = float32(TBABY_FRAME_run8); Self.NextThink = Time + 0.1; Self.Think = tbaby_run9; ai_face() }
func tbaby_run9()  { Self.Frame = float32(TBABY_FRAME_run9); Self.NextThink = Time + 0.1; Self.Think = tbaby_run10; ai_face() }
func tbaby_run10() { Self.Frame = float32(TBABY_FRAME_run10); Self.NextThink = Time + 0.1; Self.Think = tbaby_run11; ai_face() }
func tbaby_run11() { Self.Frame = float32(TBABY_FRAME_run11); Self.NextThink = Time + 0.1; Self.Think = tbaby_run12; ai_run(2) }
func tbaby_run12() { Self.Frame = float32(TBABY_FRAME_run12); Self.NextThink = Time + 0.1; Self.Think = tbaby_run13; ai_run(2) }
func tbaby_run13() { Self.Frame = float32(TBABY_FRAME_run13); Self.NextThink = Time + 0.1; Self.Think = tbaby_run14; ai_run(2) }
func tbaby_run14() { Self.Frame = float32(TBABY_FRAME_run14); Self.NextThink = Time + 0.1; Self.Think = tbaby_run15; ai_run(2) }
func tbaby_run15() { Self.Frame = float32(TBABY_FRAME_run15); Self.NextThink = Time + 0.1; Self.Think = tbaby_run16; ai_run(2) }
func tbaby_run16() { Self.Frame = float32(TBABY_FRAME_run16); Self.NextThink = Time + 0.1; Self.Think = tbaby_run17; ai_run(2) }
func tbaby_run17() { Self.Frame = float32(TBABY_FRAME_run17); Self.NextThink = Time + 0.1; Self.Think = tbaby_run18; ai_run(2) }
func tbaby_run18() { Self.Frame = float32(TBABY_FRAME_run18); Self.NextThink = Time + 0.1; Self.Think = tbaby_run19; ai_run(2) }
func tbaby_run19() { Self.Frame = float32(TBABY_FRAME_run19); Self.NextThink = Time + 0.1; Self.Think = tbaby_run20; ai_run(2) }
func tbaby_run20() { Self.Frame = float32(TBABY_FRAME_run20); Self.NextThink = Time + 0.1; Self.Think = tbaby_run21; ai_run(2) }
func tbaby_run21() { Self.Frame = float32(TBABY_FRAME_run21); Self.NextThink = Time + 0.1; Self.Think = tbaby_run22; ai_run(2) }
func tbaby_run22() { Self.Frame = float32(TBABY_FRAME_run22); Self.NextThink = Time + 0.1; Self.Think = tbaby_run23; ai_run(2) }
func tbaby_run23() { Self.Frame = float32(TBABY_FRAME_run23); Self.NextThink = Time + 0.1; Self.Think = tbaby_run24; ai_run(2) }
func tbaby_run24() { Self.Frame = float32(TBABY_FRAME_run24); Self.NextThink = Time + 0.1; Self.Think = tbaby_run25; ai_run(2) }
func tbaby_run25() { Self.Frame = float32(TBABY_FRAME_run25); Self.NextThink = Time + 0.1; Self.Think = tbaby_run1_impl; ai_run(2) }

func Tar_JumpTouch() {
	var ldmg float32

	if Other.TakeDamage != 0 && Other.ClassName != Self.ClassName {
		if engine.Vlen(Self.Velocity) > 400 {
			ldmg = 10 + 10*engine.Random()
			T_Damage(Other, Self, Self, ldmg)
			engine.Sound(Self, int(CHAN_WEAPON), "blob/hit1.wav", 1, ATTN_NORM)
		}
	} else {
		engine.Sound(Self, int(CHAN_WEAPON), "blob/land1.wav", 1, ATTN_NORM)
	}

	if engine.CheckBottom(Self) == 0 {
		if (int(Self.Flags) & FL_ONGROUND) != 0 {
			Self.Touch = SUB_Null
			Self.Think = tbaby_run1_impl
			Self.MoveType = MOVETYPE_STEP
			Self.NextThink = Time + 0.1
		}
		return
	}

	Self.Touch = SUB_Null
	Self.Think = tbaby_jump1_impl
	Self.NextThink = Time + 0.1
}

func tbaby_fly1_impl() { Self.Frame = float32(TBABY_FRAME_fly1); Self.NextThink = Time + 0.1; Self.Think = tbaby_fly2 }
func tbaby_fly2() { Self.Frame = float32(TBABY_FRAME_fly2); Self.NextThink = Time + 0.1; Self.Think = tbaby_fly3 }
func tbaby_fly3() { Self.Frame = float32(TBABY_FRAME_fly3); Self.NextThink = Time + 0.1; Self.Think = tbaby_fly4 }
func tbaby_fly4() {
	Self.Frame = float32(TBABY_FRAME_fly4)
	Self.NextThink = Time + 0.1
	Self.Think = tbaby_fly1_impl
	Self.Cnt = Self.Cnt + 1

	if Self.Cnt == 4 {
		tbaby_jump5_impl()
	}
}

func tbaby_jump1_impl() { Self.Frame = float32(TBABY_FRAME_jump1); Self.NextThink = Time + 0.1; Self.Think = tbaby_jump2; ai_face() }
func tbaby_jump2() { Self.Frame = float32(TBABY_FRAME_jump2); Self.NextThink = Time + 0.1; Self.Think = tbaby_jump3; ai_face() }
func tbaby_jump3() { Self.Frame = float32(TBABY_FRAME_jump3); Self.NextThink = Time + 0.1; Self.Think = tbaby_jump4; ai_face() }
func tbaby_jump4() { Self.Frame = float32(TBABY_FRAME_jump4); Self.NextThink = Time + 0.1; Self.Think = tbaby_jump5_impl; ai_face() }
func tbaby_jump5_impl() {
	Self.Frame = float32(TBABY_FRAME_jump5)
	Self.NextThink = Time + 0.1
	Self.Think = tbaby_jump6
	Self.MoveType = MOVETYPE_BOUNCE
	Self.Touch = Tar_JumpTouch
	makevectorsfixed(Self.Angles)
	Self.Origin[2] = Self.Origin[2] + 1
	Self.Velocity = VForward.Mul(600).Add(quake.MakeVec3(0, 0, 200))
	Self.Velocity[2] = Self.Velocity[2] + engine.Random()*150

	if (int(Self.Flags) & FL_ONGROUND) != 0 {
		Self.Flags = Self.Flags - float32(FL_ONGROUND)
	}

	Self.Cnt = 0
}

func tbaby_jump6() { Self.Frame = float32(TBABY_FRAME_jump6); Self.NextThink = Time + 0.1; Self.Think = tbaby_fly1_impl }

func tbaby_die1() { Self.Frame = float32(TBABY_FRAME_exp); Self.NextThink = Time + 0.1; Self.Think = tbaby_die2; Self.TakeDamage = DAMAGE_NO }
func tbaby_die2() {
	Self.Frame = float32(TBABY_FRAME_exp)
	Self.NextThink = Time + 0.1
	Self.Think = tbaby_run1_impl
	T_RadiusDamage(Self, Self, 120, World)

	engine.Sound(Self, int(CHAN_VOICE), "blob/death1.wav", 1, ATTN_NORM)
	Self.Origin = Self.Origin.Sub(engine.Normalize(Self.Velocity).Mul(8))

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_TAREXPLOSION))
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])

	BecomeExplosion()
}

func init() {
	tbaby_stand1 = tbaby_stand1_impl
	tbaby_walk1 = tbaby_walk1_impl
	tbaby_run1 = tbaby_run1_impl
	tbaby_jump1 = tbaby_jump1_impl
	tbaby_jump5 = tbaby_jump5_impl
	tbaby_fly1 = tbaby_fly1_impl
}

func monster_tarbaby() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel2("progs/tarbaby.mdl")

	engine.PrecacheSound2("blob/death1.wav")
	engine.PrecacheSound2("blob/hit1.wav")
	engine.PrecacheSound2("blob/land1.wav")
	engine.PrecacheSound2("blob/sight1.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/tarbaby.mdl")

	Self.Noise = "blob/sight1.wav"
	Self.NetName = "$qc_spawn"
	Self.KillString = "$qc_ks_spawn"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 40))
	Self.Health = 80
	Self.MaxHealth = 80

	Self.ThStand = tbaby_stand1_impl
	Self.ThWalk = tbaby_walk1_impl
	Self.ThRun = tbaby_run1_impl
	Self.ThMissile = tbaby_jump1_impl
	Self.ThMelee = tbaby_jump1_impl
	Self.ThDie = tbaby_die1
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_MELEE

	walkmonster_start()
}
