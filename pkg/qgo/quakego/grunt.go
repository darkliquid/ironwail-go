package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Grunt (Soldier) frames
	GRUNT_FRAME_stand1 = iota
	GRUNT_FRAME_stand2
	GRUNT_FRAME_stand3
	GRUNT_FRAME_stand4
	GRUNT_FRAME_stand5
	GRUNT_FRAME_stand6
	GRUNT_FRAME_stand7
	GRUNT_FRAME_stand8

	GRUNT_FRAME_death1
	GRUNT_FRAME_death2
	GRUNT_FRAME_death3
	GRUNT_FRAME_death4
	GRUNT_FRAME_death5
	GRUNT_FRAME_death6
	GRUNT_FRAME_death7
	GRUNT_FRAME_death8
	GRUNT_FRAME_death9
	GRUNT_FRAME_death10

	GRUNT_FRAME_deathc1
	GRUNT_FRAME_deathc2
	GRUNT_FRAME_deathc3
	GRUNT_FRAME_deathc4
	GRUNT_FRAME_deathc5
	GRUNT_FRAME_deathc6
	GRUNT_FRAME_deathc7
	GRUNT_FRAME_deathc8
	GRUNT_FRAME_deathc9
	GRUNT_FRAME_deathc10
	GRUNT_FRAME_deathc11

	GRUNT_FRAME_load1
	GRUNT_FRAME_load2
	GRUNT_FRAME_load3
	GRUNT_FRAME_load4
	GRUNT_FRAME_load5
	GRUNT_FRAME_load6
	GRUNT_FRAME_load7
	GRUNT_FRAME_load8
	GRUNT_FRAME_load9
	GRUNT_FRAME_load10
	GRUNT_FRAME_load11

	GRUNT_FRAME_pain1
	GRUNT_FRAME_pain2
	GRUNT_FRAME_pain3
	GRUNT_FRAME_pain4
	GRUNT_FRAME_pain5
	GRUNT_FRAME_pain6

	GRUNT_FRAME_painb1
	GRUNT_FRAME_painb2
	GRUNT_FRAME_painb3
	GRUNT_FRAME_painb4
	GRUNT_FRAME_painb5
	GRUNT_FRAME_painb6
	GRUNT_FRAME_painb7
	GRUNT_FRAME_painb8
	GRUNT_FRAME_painb9
	GRUNT_FRAME_painb10
	GRUNT_FRAME_painb11
	GRUNT_FRAME_painb12
	GRUNT_FRAME_painb13
	GRUNT_FRAME_painb14

	GRUNT_FRAME_painc1
	GRUNT_FRAME_painc2
	GRUNT_FRAME_painc3
	GRUNT_FRAME_painc4
	GRUNT_FRAME_painc5
	GRUNT_FRAME_painc6
	GRUNT_FRAME_painc7
	GRUNT_FRAME_painc8
	GRUNT_FRAME_painc9
	GRUNT_FRAME_painc10
	GRUNT_FRAME_painc11
	GRUNT_FRAME_painc12
	GRUNT_FRAME_painc13

	GRUNT_FRAME_run1
	GRUNT_FRAME_run2
	GRUNT_FRAME_run3
	GRUNT_FRAME_run4
	GRUNT_FRAME_run5
	GRUNT_FRAME_run6
	GRUNT_FRAME_run7
	GRUNT_FRAME_run8

	GRUNT_FRAME_shoot1
	GRUNT_FRAME_shoot2
	GRUNT_FRAME_shoot3
	GRUNT_FRAME_shoot4
	GRUNT_FRAME_shoot5
	GRUNT_FRAME_shoot6
	GRUNT_FRAME_shoot7
	GRUNT_FRAME_shoot8
	GRUNT_FRAME_shoot9

	GRUNT_FRAME_prowl_1
	GRUNT_FRAME_prowl_2
	GRUNT_FRAME_prowl_3
	GRUNT_FRAME_prowl_4
	GRUNT_FRAME_prowl_5
	GRUNT_FRAME_prowl_6
	GRUNT_FRAME_prowl_7
	GRUNT_FRAME_prowl_8
	GRUNT_FRAME_prowl_9
	GRUNT_FRAME_prowl_10
	GRUNT_FRAME_prowl_11
	GRUNT_FRAME_prowl_12
	GRUNT_FRAME_prowl_13
	GRUNT_FRAME_prowl_14
	GRUNT_FRAME_prowl_15
	GRUNT_FRAME_prowl_16
	GRUNT_FRAME_prowl_17
	GRUNT_FRAME_prowl_18
	GRUNT_FRAME_prowl_19
	GRUNT_FRAME_prowl_20
	GRUNT_FRAME_prowl_21
	GRUNT_FRAME_prowl_22
	GRUNT_FRAME_prowl_23
	GRUNT_FRAME_prowl_24
)

// Prototyped elsewhere
var army_atk1 func()

func army_stand1() { Self.Frame = float32(GRUNT_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = army_stand2; ai_stand() }
func army_stand2() { Self.Frame = float32(GRUNT_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = army_stand3; ai_stand() }
func army_stand3() { Self.Frame = float32(GRUNT_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = army_stand4; ai_stand() }
func army_stand4() { Self.Frame = float32(GRUNT_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = army_stand5; ai_stand() }
func army_stand5() { Self.Frame = float32(GRUNT_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = army_stand6; ai_stand() }
func army_stand6() { Self.Frame = float32(GRUNT_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = army_stand7; ai_stand() }
func army_stand7() { Self.Frame = float32(GRUNT_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = army_stand8; ai_stand() }
func army_stand8() { Self.Frame = float32(GRUNT_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = army_stand1; ai_stand() }

func army_walk1() {
	Self.Frame = float32(GRUNT_FRAME_prowl_1)
	Self.NextThink = Time + 0.1
	Self.Think = army_walk2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "soldier/idle.wav", 1, ATTN_IDLE)
	}
	ai_walk(1)
}

func army_walk2()  { Self.Frame = float32(GRUNT_FRAME_prowl_2); Self.NextThink = Time + 0.1; Self.Think = army_walk3; ai_walk(1) }
func army_walk3()  { Self.Frame = float32(GRUNT_FRAME_prowl_3); Self.NextThink = Time + 0.1; Self.Think = army_walk4; ai_walk(1) }
func army_walk4()  { Self.Frame = float32(GRUNT_FRAME_prowl_4); Self.NextThink = Time + 0.1; Self.Think = army_walk5; ai_walk(1) }
func army_walk5()  { Self.Frame = float32(GRUNT_FRAME_prowl_5); Self.NextThink = Time + 0.1; Self.Think = army_walk6; ai_walk(2) }
func army_walk6()  { Self.Frame = float32(GRUNT_FRAME_prowl_6); Self.NextThink = Time + 0.1; Self.Think = army_walk7; ai_walk(3) }
func army_walk7()  { Self.Frame = float32(GRUNT_FRAME_prowl_7); Self.NextThink = Time + 0.1; Self.Think = army_walk8; ai_walk(4) }
func army_walk8()  { Self.Frame = float32(GRUNT_FRAME_prowl_8); Self.NextThink = Time + 0.1; Self.Think = army_walk9; ai_walk(4) }
func army_walk9()  { Self.Frame = float32(GRUNT_FRAME_prowl_9); Self.NextThink = Time + 0.1; Self.Think = army_walk10; ai_walk(2) }
func army_walk10() { Self.Frame = float32(GRUNT_FRAME_prowl_10); Self.NextThink = Time + 0.1; Self.Think = army_walk11; ai_walk(2) }
func army_walk11() { Self.Frame = float32(GRUNT_FRAME_prowl_11); Self.NextThink = Time + 0.1; Self.Think = army_walk12; ai_walk(2) }
func army_walk12() { Self.Frame = float32(GRUNT_FRAME_prowl_12); Self.NextThink = Time + 0.1; Self.Think = army_walk13; ai_walk(1) }
func army_walk13() { Self.Frame = float32(GRUNT_FRAME_prowl_13); Self.NextThink = Time + 0.1; Self.Think = army_walk14; ai_walk(0) }
func army_walk14() { Self.Frame = float32(GRUNT_FRAME_prowl_14); Self.NextThink = Time + 0.1; Self.Think = army_walk15; ai_walk(1) }
func army_walk15() { Self.Frame = float32(GRUNT_FRAME_prowl_15); Self.NextThink = Time + 0.1; Self.Think = army_walk16; ai_walk(1) }
func army_walk16() { Self.Frame = float32(GRUNT_FRAME_prowl_16); Self.NextThink = Time + 0.1; Self.Think = army_walk17; ai_walk(1) }
func army_walk17() { Self.Frame = float32(GRUNT_FRAME_prowl_17); Self.NextThink = Time + 0.1; Self.Think = army_walk18; ai_walk(3) }
func army_walk18() { Self.Frame = float32(GRUNT_FRAME_prowl_18); Self.NextThink = Time + 0.1; Self.Think = army_walk19; ai_walk(3) }
func army_walk19() { Self.Frame = float32(GRUNT_FRAME_prowl_19); Self.NextThink = Time + 0.1; Self.Think = army_walk20; ai_walk(3) }
func army_walk20() { Self.Frame = float32(GRUNT_FRAME_prowl_20); Self.NextThink = Time + 0.1; Self.Think = army_walk21; ai_walk(3) }
func army_walk21() { Self.Frame = float32(GRUNT_FRAME_prowl_21); Self.NextThink = Time + 0.1; Self.Think = army_walk22; ai_walk(2) }
func army_walk22() { Self.Frame = float32(GRUNT_FRAME_prowl_22); Self.NextThink = Time + 0.1; Self.Think = army_walk23; ai_walk(1) }
func army_walk23() { Self.Frame = float32(GRUNT_FRAME_prowl_23); Self.NextThink = Time + 0.1; Self.Think = army_walk24; ai_walk(1) }
func army_walk24() { Self.Frame = float32(GRUNT_FRAME_prowl_24); Self.NextThink = Time + 0.1; Self.Think = army_walk1; ai_walk(1) }

func army_run1() {
	Self.Frame = float32(GRUNT_FRAME_run1)
	Self.NextThink = Time + 0.1
	Self.Think = army_run2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "soldier/idle.wav", 1, ATTN_IDLE)
	}
	ai_run(11)
}

func army_run2() { Self.Frame = float32(GRUNT_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = army_run3; ai_run(15) }
func army_run3() { Self.Frame = float32(GRUNT_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = army_run4; ai_run(10) }
func army_run4() { Self.Frame = float32(GRUNT_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = army_run5; ai_run(10) }
func army_run5() { Self.Frame = float32(GRUNT_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = army_run6; ai_run(8) }
func army_run6() { Self.Frame = float32(GRUNT_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = army_run7; ai_run(15) }
func army_run7() { Self.Frame = float32(GRUNT_FRAME_run7); Self.NextThink = Time + 0.1; Self.Think = army_run8; ai_run(10) }
func army_run8() { Self.Frame = float32(GRUNT_FRAME_run8); Self.NextThink = Time + 0.1; Self.Think = army_run1; ai_run(8) }

func army_fire() {
	var dir quake.Vec3
	var en *quake.Entity

	ai_face()

	engine.Sound(Self, int(CHAN_WEAPON), "soldier/sattck1.wav", 1, ATTN_NORM)

	en = Self.Enemy

	dir = en.Origin.Sub(en.Velocity.Mul(0.2))
	dir = engine.Normalize(dir.Sub(Self.Origin))

	FireBullets(4, dir, quake.MakeVec3(0.1, 0.1, 0))
}

func army_atk1_impl() { Self.Frame = float32(GRUNT_FRAME_shoot1); Self.NextThink = Time + 0.1; Self.Think = army_atk2; ai_face() }
func army_atk2() { Self.Frame = float32(GRUNT_FRAME_shoot2); Self.NextThink = Time + 0.1; Self.Think = army_atk3; ai_face() }
func army_atk3() { Self.Frame = float32(GRUNT_FRAME_shoot3); Self.NextThink = Time + 0.1; Self.Think = army_atk4; ai_face() }
func army_atk4() { Self.Frame = float32(GRUNT_FRAME_shoot4); Self.NextThink = Time + 0.1; Self.Think = army_atk5; ai_face() }
func army_atk5() {
	Self.Frame = float32(GRUNT_FRAME_shoot5)
	Self.NextThink = Time + 0.1
	Self.Think = army_atk6
	ai_face()
	army_fire()
	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
}

func army_atk6() { Self.Frame = float32(GRUNT_FRAME_shoot6); Self.NextThink = Time + 0.1; Self.Think = army_atk7; ai_face() }
func army_atk7() {
	Self.Frame = float32(GRUNT_FRAME_shoot7)
	Self.NextThink = Time + 0.1
	Self.Think = army_atk8
	ai_face()
	SUB_CheckRefire(army_atk1_impl)
}
func army_atk8() { Self.Frame = float32(GRUNT_FRAME_shoot8); Self.NextThink = Time + 0.1; Self.Think = army_atk9; ai_face() }
func army_atk9() { Self.Frame = float32(GRUNT_FRAME_shoot9); Self.NextThink = Time + 0.1; Self.Think = army_run1; ai_face() }

func army_pain1() { Self.Frame = float32(GRUNT_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = army_pain2 }
func army_pain2() { Self.Frame = float32(GRUNT_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = army_pain3 }
func army_pain3() { Self.Frame = float32(GRUNT_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = army_pain4 }
func army_pain4() { Self.Frame = float32(GRUNT_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = army_pain5 }
func army_pain5() { Self.Frame = float32(GRUNT_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = army_pain6 }
func army_pain6() { Self.Frame = float32(GRUNT_FRAME_pain6); Self.NextThink = Time + 0.1; Self.Think = army_run1; ai_pain(1) }

func army_painb1()  { Self.Frame = float32(GRUNT_FRAME_painb1); Self.NextThink = Time + 0.1; Self.Think = army_painb2 }
func army_painb2()  { Self.Frame = float32(GRUNT_FRAME_painb2); Self.NextThink = Time + 0.1; Self.Think = army_painb3; ai_painforward(13) }
func army_painb3()  { Self.Frame = float32(GRUNT_FRAME_painb3); Self.NextThink = Time + 0.1; Self.Think = army_painb4; ai_painforward(9) }
func army_painb4()  { Self.Frame = float32(GRUNT_FRAME_painb4); Self.NextThink = Time + 0.1; Self.Think = army_painb5 }
func army_painb5()  { Self.Frame = float32(GRUNT_FRAME_painb5); Self.NextThink = Time + 0.1; Self.Think = army_painb6 }
func army_painb6()  { Self.Frame = float32(GRUNT_FRAME_painb6); Self.NextThink = Time + 0.1; Self.Think = army_painb7 }
func army_painb7()  { Self.Frame = float32(GRUNT_FRAME_painb7); Self.NextThink = Time + 0.1; Self.Think = army_painb8 }
func army_painb8()  { Self.Frame = float32(GRUNT_FRAME_painb8); Self.NextThink = Time + 0.1; Self.Think = army_painb9 }
func army_painb9()  { Self.Frame = float32(GRUNT_FRAME_painb9); Self.NextThink = Time + 0.1; Self.Think = army_painb10 }
func army_painb10() { Self.Frame = float32(GRUNT_FRAME_painb10); Self.NextThink = Time + 0.1; Self.Think = army_painb11 }
func army_painb11() { Self.Frame = float32(GRUNT_FRAME_painb11); Self.NextThink = Time + 0.1; Self.Think = army_painb12 }
func army_painb12() { Self.Frame = float32(GRUNT_FRAME_painb12); Self.NextThink = Time + 0.1; Self.Think = army_painb13; ai_pain(2) }
func army_painb13() { Self.Frame = float32(GRUNT_FRAME_painb13); Self.NextThink = Time + 0.1; Self.Think = army_painb14 }
func army_painb14() { Self.Frame = float32(GRUNT_FRAME_painb14); Self.NextThink = Time + 0.1; Self.Think = army_run1 }

func army_painc1()  { Self.Frame = float32(GRUNT_FRAME_painc1); Self.NextThink = Time + 0.1; Self.Think = army_painc2 }
func army_painc2()  { Self.Frame = float32(GRUNT_FRAME_painc2); Self.NextThink = Time + 0.1; Self.Think = army_painc3; ai_pain(1) }
func army_painc3()  { Self.Frame = float32(GRUNT_FRAME_painc3); Self.NextThink = Time + 0.1; Self.Think = army_painc4 }
func army_painc4()  { Self.Frame = float32(GRUNT_FRAME_painc4); Self.NextThink = Time + 0.1; Self.Think = army_painc5 }
func army_painc5()  { Self.Frame = float32(GRUNT_FRAME_painc5); Self.NextThink = Time + 0.1; Self.Think = army_painc6; ai_painforward(1) }
func army_painc6()  { Self.Frame = float32(GRUNT_FRAME_painc6); Self.NextThink = Time + 0.1; Self.Think = army_painc7; ai_painforward(1) }
func army_painc7()  { Self.Frame = float32(GRUNT_FRAME_painc7); Self.NextThink = Time + 0.1; Self.Think = army_painc8 }
func army_painc8()  { Self.Frame = float32(GRUNT_FRAME_painc8); Self.NextThink = Time + 0.1; Self.Think = army_painc9; ai_pain(1) }
func army_painc9()  { Self.Frame = float32(GRUNT_FRAME_painc9); Self.NextThink = Time + 0.1; Self.Think = army_painc10; ai_painforward(4) }
func army_painc10() { Self.Frame = float32(GRUNT_FRAME_painc10); Self.NextThink = Time + 0.1; Self.Think = army_painc11; ai_painforward(3) }
func army_painc11() { Self.Frame = float32(GRUNT_FRAME_painc11); Self.NextThink = Time + 0.1; Self.Think = army_painc12; ai_painforward(6) }
func army_painc12() { Self.Frame = float32(GRUNT_FRAME_painc12); Self.NextThink = Time + 0.1; Self.Think = army_painc13; ai_painforward(8) }
func army_painc13() { Self.Frame = float32(GRUNT_FRAME_painc13); Self.NextThink = Time + 0.1; Self.Think = army_run1 }

func army_pain(attacker *quake.Entity, damage float32) {
	var r float32

	if Self.PainFinished > Time {
		return
	}

	r = engine.Random()

	if r < 0.2 {
		Self.PainFinished = Time + 0.6
		army_pain1()
		engine.Sound(Self, int(CHAN_VOICE), "soldier/pain1.wav", 1, ATTN_NORM)
	} else if r < 0.6 {
		Self.PainFinished = Time + 1.1
		army_painb1()
		engine.Sound(Self, int(CHAN_VOICE), "soldier/pain2.wav", 1, ATTN_NORM)
	} else {
		Self.PainFinished = Time + 1.1
		army_painc1()
		engine.Sound(Self, int(CHAN_VOICE), "soldier/pain2.wav", 1, ATTN_NORM)
	}
}

func army_die1() { Self.Frame = float32(GRUNT_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = army_die2 }
func army_die2() { Self.Frame = float32(GRUNT_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = army_die3 }
func army_die3() {
	Self.Frame = float32(GRUNT_FRAME_death3)
	Self.NextThink = Time + 0.1
	Self.Think = army_die4
	Self.Solid = SOLID_NOT
	Self.AmmoShells = 5
	DropBackpack()
}
func army_die4()  { Self.Frame = float32(GRUNT_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = army_die5 }
func army_die5()  { Self.Frame = float32(GRUNT_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = army_die6 }
func army_die6()  { Self.Frame = float32(GRUNT_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = army_die7 }
func army_die7()  { Self.Frame = float32(GRUNT_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = army_die8 }
func army_die8()  { Self.Frame = float32(GRUNT_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = army_die9 }
func army_die9()  { Self.Frame = float32(GRUNT_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = army_die10 }
func army_die10() { Self.Frame = float32(GRUNT_FRAME_death10); Self.NextThink = Time + 0.1; Self.Think = army_die10 }

func army_cdie1() { Self.Frame = float32(GRUNT_FRAME_deathc1); Self.NextThink = Time + 0.1; Self.Think = army_cdie2 }
func army_cdie2() { Self.Frame = float32(GRUNT_FRAME_deathc2); Self.NextThink = Time + 0.1; Self.Think = army_cdie3; ai_back(5) }
func army_cdie3() {
	Self.Frame = float32(GRUNT_FRAME_deathc3)
	Self.NextThink = Time + 0.1
	Self.Think = army_cdie4
	Self.Solid = SOLID_NOT
	Self.AmmoShells = 5
	DropBackpack()
	ai_back(4)
}
func army_cdie4()  { Self.Frame = float32(GRUNT_FRAME_deathc4); Self.NextThink = Time + 0.1; Self.Think = army_cdie5; ai_back(13) }
func army_cdie5()  { Self.Frame = float32(GRUNT_FRAME_deathc5); Self.NextThink = Time + 0.1; Self.Think = army_cdie6; ai_back(3) }
func army_cdie6()  { Self.Frame = float32(GRUNT_FRAME_deathc6); Self.NextThink = Time + 0.1; Self.Think = army_cdie7; ai_back(4) }
func army_cdie7()  { Self.Frame = float32(GRUNT_FRAME_deathc7); Self.NextThink = Time + 0.1; Self.Think = army_cdie8 }
func army_cdie8()  { Self.Frame = float32(GRUNT_FRAME_deathc8); Self.NextThink = Time + 0.1; Self.Think = army_cdie9 }
func army_cdie9()  { Self.Frame = float32(GRUNT_FRAME_deathc9); Self.NextThink = Time + 0.1; Self.Think = army_cdie10 }
func army_cdie10() { Self.Frame = float32(GRUNT_FRAME_deathc10); Self.NextThink = Time + 0.1; Self.Think = army_cdie11 }
func army_cdie11() { Self.Frame = float32(GRUNT_FRAME_deathc11); Self.NextThink = Time + 0.1; Self.Think = army_cdie11 }

func army_die() {
	if Self.Health < -35 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_guard.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "soldier/death1.wav", 1, ATTN_NORM)
	if engine.Random() < 0.5 {
		army_die1()
	} else {
		army_cdie1()
	}
}

func init() {
	army_atk1 = army_atk1_impl
}

func monster_army() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/soldier.mdl")
	engine.PrecacheModel("progs/h_guard.mdl")
	engine.PrecacheModel("progs/gib1.mdl")
	engine.PrecacheModel("progs/gib2.mdl")
	engine.PrecacheModel("progs/gib3.mdl")

	engine.PrecacheSound("soldier/death1.wav")
	engine.PrecacheSound("soldier/idle.wav")
	engine.PrecacheSound("soldier/pain1.wav")
	engine.PrecacheSound("soldier/pain2.wav")
	engine.PrecacheSound("soldier/sattck1.wav")
	engine.PrecacheSound("soldier/sight1.wav")
	engine.PrecacheSound("player/udeath.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/soldier.mdl")

	Self.Noise = "soldier/sight1.wav"
	Self.NetName = "$qc_grunt"
	Self.KillString = "$qc_ks_grunt"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 40))
	Self.Health = 30
	Self.MaxHealth = 30

	Self.ThStand = army_stand1
	Self.ThWalk = army_walk1
	Self.ThRun = army_run1
	Self.ThMissile = army_atk1_impl
	Self.ThPain = army_pain
	Self.ThDie = army_die
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_RANGED

	walkmonster_start()
}
