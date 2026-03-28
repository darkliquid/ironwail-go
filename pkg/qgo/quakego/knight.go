package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Knight frames
	KNIGHT_FRAME_stand1 = iota
	KNIGHT_FRAME_stand2
	KNIGHT_FRAME_stand3
	KNIGHT_FRAME_stand4
	KNIGHT_FRAME_stand5
	KNIGHT_FRAME_stand6
	KNIGHT_FRAME_stand7
	KNIGHT_FRAME_stand8
	KNIGHT_FRAME_stand9

	KNIGHT_FRAME_runb1
	KNIGHT_FRAME_runb2
	KNIGHT_FRAME_runb3
	KNIGHT_FRAME_runb4
	KNIGHT_FRAME_runb5
	KNIGHT_FRAME_runb6
	KNIGHT_FRAME_runb7
	KNIGHT_FRAME_runb8

	KNIGHT_FRAME_runattack1
	KNIGHT_FRAME_runattack2
	KNIGHT_FRAME_runattack3
	KNIGHT_FRAME_runattack4
	KNIGHT_FRAME_runattack5
	KNIGHT_FRAME_runattack6
	KNIGHT_FRAME_runattack7
	KNIGHT_FRAME_runattack8
	KNIGHT_FRAME_runattack9
	KNIGHT_FRAME_runattack10
	KNIGHT_FRAME_runattack11

	KNIGHT_FRAME_pain1
	KNIGHT_FRAME_pain2
	KNIGHT_FRAME_pain3

	KNIGHT_FRAME_painb1
	KNIGHT_FRAME_painb2
	KNIGHT_FRAME_painb3
	KNIGHT_FRAME_painb4
	KNIGHT_FRAME_painb5
	KNIGHT_FRAME_painb6
	KNIGHT_FRAME_painb7
	KNIGHT_FRAME_painb8
	KNIGHT_FRAME_painb9
	KNIGHT_FRAME_painb10
	KNIGHT_FRAME_painb11

	KNIGHT_FRAME_attackb1
	KNIGHT_FRAME_attackb2
	KNIGHT_FRAME_attackb3
	KNIGHT_FRAME_attackb4
	KNIGHT_FRAME_attackb5
	KNIGHT_FRAME_attackb6
	KNIGHT_FRAME_attackb7
	KNIGHT_FRAME_attackb8
	KNIGHT_FRAME_attackb9
	KNIGHT_FRAME_attackb10
	KNIGHT_FRAME_attackb11

	KNIGHT_FRAME_walk1
	KNIGHT_FRAME_walk2
	KNIGHT_FRAME_walk3
	KNIGHT_FRAME_walk4
	KNIGHT_FRAME_walk5
	KNIGHT_FRAME_walk6
	KNIGHT_FRAME_walk7
	KNIGHT_FRAME_walk8
	KNIGHT_FRAME_walk9
	KNIGHT_FRAME_walk10
	KNIGHT_FRAME_walk11
	KNIGHT_FRAME_walk12
	KNIGHT_FRAME_walk13
	KNIGHT_FRAME_walk14

	KNIGHT_FRAME_kneel1
	KNIGHT_FRAME_kneel2
	KNIGHT_FRAME_kneel3
	KNIGHT_FRAME_kneel4
	KNIGHT_FRAME_kneel5

	KNIGHT_FRAME_standing2
	KNIGHT_FRAME_standing3
	KNIGHT_FRAME_standing4
	KNIGHT_FRAME_standing5

	KNIGHT_FRAME_death1
	KNIGHT_FRAME_death2
	KNIGHT_FRAME_death3
	KNIGHT_FRAME_death4
	KNIGHT_FRAME_death5
	KNIGHT_FRAME_death6
	KNIGHT_FRAME_death7
	KNIGHT_FRAME_death8
	KNIGHT_FRAME_death9
	KNIGHT_FRAME_death10

	KNIGHT_FRAME_deathb1
	KNIGHT_FRAME_deathb2
	KNIGHT_FRAME_deathb3
	KNIGHT_FRAME_deathb4
	KNIGHT_FRAME_deathb5
	KNIGHT_FRAME_deathb6
	KNIGHT_FRAME_deathb7
	KNIGHT_FRAME_deathb8
	KNIGHT_FRAME_deathb9
	KNIGHT_FRAME_deathb10
	KNIGHT_FRAME_deathb11
)

func knight_stand1() { Self.Frame = float32(KNIGHT_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = knight_stand2; ai_stand() }
func knight_stand2() { Self.Frame = float32(KNIGHT_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = knight_stand3; ai_stand() }
func knight_stand3() { Self.Frame = float32(KNIGHT_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = knight_stand4; ai_stand() }
func knight_stand4() { Self.Frame = float32(KNIGHT_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = knight_stand5; ai_stand() }
func knight_stand5() { Self.Frame = float32(KNIGHT_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = knight_stand6; ai_stand() }
func knight_stand6() { Self.Frame = float32(KNIGHT_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = knight_stand7; ai_stand() }
func knight_stand7() { Self.Frame = float32(KNIGHT_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = knight_stand8; ai_stand() }
func knight_stand8() { Self.Frame = float32(KNIGHT_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = knight_stand9; ai_stand() }
func knight_stand9() { Self.Frame = float32(KNIGHT_FRAME_stand9); Self.NextThink = Time + 0.1; Self.Think = knight_stand1; ai_stand() }

func knight_walk1() {
	Self.Frame = float32(KNIGHT_FRAME_walk1)
	Self.NextThink = Time + 0.1
	Self.Think = knight_walk2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "knight/idle.wav", 1, ATTN_IDLE)
	}
	ai_walk(3)
}
func knight_walk2()  { Self.Frame = float32(KNIGHT_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = knight_walk3; ai_walk(2) }
func knight_walk3()  { Self.Frame = float32(KNIGHT_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = knight_walk4; ai_walk(3) }
func knight_walk4()  { Self.Frame = float32(KNIGHT_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = knight_walk5; ai_walk(4) }
func knight_walk5()  { Self.Frame = float32(KNIGHT_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = knight_walk6; ai_walk(3) }
func knight_walk6()  { Self.Frame = float32(KNIGHT_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = knight_walk7; ai_walk(3) }
func knight_walk7()  { Self.Frame = float32(KNIGHT_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = knight_walk8; ai_walk(3) }
func knight_walk8()  { Self.Frame = float32(KNIGHT_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = knight_walk9; ai_walk(4) }
func knight_walk9()  { Self.Frame = float32(KNIGHT_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = knight_walk10; ai_walk(3) }
func knight_walk10() { Self.Frame = float32(KNIGHT_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = knight_walk11; ai_walk(3) }
func knight_walk11() { Self.Frame = float32(KNIGHT_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = knight_walk12; ai_walk(2) }
func knight_walk12() { Self.Frame = float32(KNIGHT_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = knight_walk13; ai_walk(3) }
func knight_walk13() { Self.Frame = float32(KNIGHT_FRAME_walk13); Self.NextThink = Time + 0.1; Self.Think = knight_walk14; ai_walk(4) }
func knight_walk14() { Self.Frame = float32(KNIGHT_FRAME_walk14); Self.NextThink = Time + 0.1; Self.Think = knight_walk1; ai_walk(3) }

func knight_run1() {
	Self.Frame = float32(KNIGHT_FRAME_runb1)
	Self.NextThink = Time + 0.1
	Self.Think = knight_run2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "knight/idle.wav", 1, ATTN_IDLE)
	}
	ai_run(16)
}
func knight_run2() { Self.Frame = float32(KNIGHT_FRAME_runb2); Self.NextThink = Time + 0.1; Self.Think = knight_run3; ai_run(20) }
func knight_run3() { Self.Frame = float32(KNIGHT_FRAME_runb3); Self.NextThink = Time + 0.1; Self.Think = knight_run4; ai_run(13) }
func knight_run4() { Self.Frame = float32(KNIGHT_FRAME_runb4); Self.NextThink = Time + 0.1; Self.Think = knight_run5; ai_run(7) }
func knight_run5() { Self.Frame = float32(KNIGHT_FRAME_runb5); Self.NextThink = Time + 0.1; Self.Think = knight_run6; ai_run(16) }
func knight_run6() { Self.Frame = float32(KNIGHT_FRAME_runb6); Self.NextThink = Time + 0.1; Self.Think = knight_run7; ai_run(20) }
func knight_run7() { Self.Frame = float32(KNIGHT_FRAME_runb7); Self.NextThink = Time + 0.1; Self.Think = knight_run8; ai_run(14) }
func knight_run8() { Self.Frame = float32(KNIGHT_FRAME_runb8); Self.NextThink = Time + 0.1; Self.Think = knight_run1; ai_run(6) }

func knight_runatk1_impl() {
	Self.Frame = float32(KNIGHT_FRAME_runattack1)
	Self.NextThink = Time + 0.1
	Self.Think = knight_runatk2

	if engine.Random() > 0.5 {
		engine.Sound(Self, int(CHAN_WEAPON), "knight/sword2.wav", 1, ATTN_NORM)
	} else {
		engine.Sound(Self, int(CHAN_WEAPON), "knight/sword1.wav", 1, ATTN_NORM)
	}
	ai_charge(20)
}

func knight_runatk2()  { Self.Frame = float32(KNIGHT_FRAME_runattack2); Self.NextThink = Time + 0.1; Self.Think = knight_runatk3; ai_charge_side() }
func knight_runatk3()  { Self.Frame = float32(KNIGHT_FRAME_runattack3); Self.NextThink = Time + 0.1; Self.Think = knight_runatk4; ai_charge_side() }
func knight_runatk4()  { Self.Frame = float32(KNIGHT_FRAME_runattack4); Self.NextThink = Time + 0.1; Self.Think = knight_runatk5; ai_charge_side() }
func knight_runatk5()  { Self.Frame = float32(KNIGHT_FRAME_runattack5); Self.NextThink = Time + 0.1; Self.Think = knight_runatk6; ai_melee_side() }
func knight_runatk6()  { Self.Frame = float32(KNIGHT_FRAME_runattack6); Self.NextThink = Time + 0.1; Self.Think = knight_runatk7; ai_melee_side() }
func knight_runatk7()  { Self.Frame = float32(KNIGHT_FRAME_runattack7); Self.NextThink = Time + 0.1; Self.Think = knight_runatk8; ai_melee_side() }
func knight_runatk8()  { Self.Frame = float32(KNIGHT_FRAME_runattack8); Self.NextThink = Time + 0.1; Self.Think = knight_runatk9; ai_melee_side() }
func knight_runatk9()  { Self.Frame = float32(KNIGHT_FRAME_runattack9); Self.NextThink = Time + 0.1; Self.Think = knight_runatk10; ai_melee_side() }
func knight_runatk10() { Self.Frame = float32(KNIGHT_FRAME_runattack10); Self.NextThink = Time + 0.1; Self.Think = knight_runatk11; ai_charge_side() }
func knight_runatk11() { Self.Frame = float32(KNIGHT_FRAME_runattack11); Self.NextThink = Time + 0.1; Self.Think = knight_run1; ai_charge(10) }

func knight_atk1_impl() {
	Self.Frame = float32(KNIGHT_FRAME_attackb1)
	Self.NextThink = Time + 0.1
	Self.Think = knight_atk2
	engine.Sound(Self, int(CHAN_WEAPON), "knight/sword1.wav", 1, ATTN_NORM)
	ai_charge(0)
}

func knight_atk2()  { Self.Frame = float32(KNIGHT_FRAME_attackb2); Self.NextThink = Time + 0.1; Self.Think = knight_atk3; ai_charge(7) }
func knight_atk3()  { Self.Frame = float32(KNIGHT_FRAME_attackb3); Self.NextThink = Time + 0.1; Self.Think = knight_atk4; ai_charge(4) }
func knight_atk4()  { Self.Frame = float32(KNIGHT_FRAME_attackb4); Self.NextThink = Time + 0.1; Self.Think = knight_atk5; ai_charge(0) }
func knight_atk5()  { Self.Frame = float32(KNIGHT_FRAME_attackb5); Self.NextThink = Time + 0.1; Self.Think = knight_atk6; ai_charge(3) }
func knight_atk6()  { Self.Frame = float32(KNIGHT_FRAME_attackb6); Self.NextThink = Time + 0.1; Self.Think = knight_atk7; ai_charge(4); ai_melee() }
func knight_atk7()  { Self.Frame = float32(KNIGHT_FRAME_attackb7); Self.NextThink = Time + 0.1; Self.Think = knight_atk8; ai_charge(1); ai_melee() }
func knight_atk8()  { Self.Frame = float32(KNIGHT_FRAME_attackb8); Self.NextThink = Time + 0.1; Self.Think = knight_atk9; ai_charge(3); ai_melee() }
func knight_atk9()  { Self.Frame = float32(KNIGHT_FRAME_attackb9); Self.NextThink = Time + 0.1; Self.Think = knight_atk10; ai_charge(1) }
func knight_atk10() { Self.Frame = float32(KNIGHT_FRAME_attackb10); Self.NextThink = Time + 0.1; Self.Think = knight_run1; ai_charge(5) }

func knight_pain1() { Self.Frame = float32(KNIGHT_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = knight_pain2 }
func knight_pain2() { Self.Frame = float32(KNIGHT_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = knight_pain3 }
func knight_pain3() { Self.Frame = float32(KNIGHT_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = knight_run1 }

func knight_painb1()  { Self.Frame = float32(KNIGHT_FRAME_painb1); Self.NextThink = Time + 0.1; Self.Think = knight_painb2; ai_painforward(0) }
func knight_painb2()  { Self.Frame = float32(KNIGHT_FRAME_painb2); Self.NextThink = Time + 0.1; Self.Think = knight_painb3; ai_painforward(3) }
func knight_painb3()  { Self.Frame = float32(KNIGHT_FRAME_painb3); Self.NextThink = Time + 0.1; Self.Think = knight_painb4 }
func knight_painb4()  { Self.Frame = float32(KNIGHT_FRAME_painb4); Self.NextThink = Time + 0.1; Self.Think = knight_painb5 }
func knight_painb5()  { Self.Frame = float32(KNIGHT_FRAME_painb5); Self.NextThink = Time + 0.1; Self.Think = knight_painb6; ai_painforward(2) }
func knight_painb6()  { Self.Frame = float32(KNIGHT_FRAME_painb6); Self.NextThink = Time + 0.1; Self.Think = knight_painb7; ai_painforward(4) }
func knight_painb7()  { Self.Frame = float32(KNIGHT_FRAME_painb7); Self.NextThink = Time + 0.1; Self.Think = knight_painb8; ai_painforward(2) }
func knight_painb8()  { Self.Frame = float32(KNIGHT_FRAME_painb8); Self.NextThink = Time + 0.1; Self.Think = knight_painb9; ai_painforward(5) }
func knight_painb9()  { Self.Frame = float32(KNIGHT_FRAME_painb9); Self.NextThink = Time + 0.1; Self.Think = knight_painb10; ai_painforward(5) }
func knight_painb10() { Self.Frame = float32(KNIGHT_FRAME_painb10); Self.NextThink = Time + 0.1; Self.Think = knight_painb11; ai_painforward(0) }
func knight_painb11() { Self.Frame = float32(KNIGHT_FRAME_painb11); Self.NextThink = Time + 0.1; Self.Think = knight_run1 }

func knight_pain(attacker *quake.Entity, damage float32) {
	var r float32

	if Self.PainFinished > Time {
		return
	}

	r = engine.Random()

	engine.Sound(Self, int(CHAN_VOICE), "knight/khurt.wav", 1, ATTN_NORM)
	if r < 0.85 {
		knight_pain1()
		Self.PainFinished = Time + 1
	} else {
		knight_painb1()
		Self.PainFinished = Time + 1
	}
}

func knight_bow1()  { Self.Frame = float32(KNIGHT_FRAME_kneel1); Self.NextThink = Time + 0.1; Self.Think = knight_bow2; ai_turn() }
func knight_bow2()  { Self.Frame = float32(KNIGHT_FRAME_kneel2); Self.NextThink = Time + 0.1; Self.Think = knight_bow3; ai_turn() }
func knight_bow3()  { Self.Frame = float32(KNIGHT_FRAME_kneel3); Self.NextThink = Time + 0.1; Self.Think = knight_bow4; ai_turn() }
func knight_bow4()  { Self.Frame = float32(KNIGHT_FRAME_kneel4); Self.NextThink = Time + 0.1; Self.Think = knight_bow5; ai_turn() }
func knight_bow5()  { Self.Frame = float32(KNIGHT_FRAME_kneel5); Self.NextThink = Time + 0.1; Self.Think = knight_bow5; ai_turn() }
func knight_bow6()  { Self.Frame = float32(KNIGHT_FRAME_kneel4); Self.NextThink = Time + 0.1; Self.Think = knight_bow7; ai_turn() }
func knight_bow7()  { Self.Frame = float32(KNIGHT_FRAME_kneel3); Self.NextThink = Time + 0.1; Self.Think = knight_bow8; ai_turn() }
func knight_bow8()  { Self.Frame = float32(KNIGHT_FRAME_kneel2); Self.NextThink = Time + 0.1; Self.Think = knight_bow9; ai_turn() }
func knight_bow9()  { Self.Frame = float32(KNIGHT_FRAME_kneel1); Self.NextThink = Time + 0.1; Self.Think = knight_bow10; ai_turn() }
func knight_bow10() { Self.Frame = float32(KNIGHT_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = knight_walk1; ai_turn() }

func knight_die1()  { Self.Frame = float32(KNIGHT_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = knight_die2 }
func knight_die2()  { Self.Frame = float32(KNIGHT_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = knight_die3 }
func knight_die3()  { Self.Frame = float32(KNIGHT_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = knight_die4; Self.Solid = SOLID_NOT }
func knight_die4()  { Self.Frame = float32(KNIGHT_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = knight_die5 }
func knight_die5()  { Self.Frame = float32(KNIGHT_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = knight_die6 }
func knight_die6()  { Self.Frame = float32(KNIGHT_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = knight_die7 }
func knight_die7()  { Self.Frame = float32(KNIGHT_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = knight_die8 }
func knight_die8()  { Self.Frame = float32(KNIGHT_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = knight_die9 }
func knight_die9()  { Self.Frame = float32(KNIGHT_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = knight_die10 }
func knight_die10() { Self.Frame = float32(KNIGHT_FRAME_death10); Self.NextThink = Time + 0.1; Self.Think = knight_die10 }

func knight_dieb1()  { Self.Frame = float32(KNIGHT_FRAME_deathb1); Self.NextThink = Time + 0.1; Self.Think = knight_dieb2 }
func knight_dieb2()  { Self.Frame = float32(KNIGHT_FRAME_deathb2); Self.NextThink = Time + 0.1; Self.Think = knight_dieb3 }
func knight_dieb3()  { Self.Frame = float32(KNIGHT_FRAME_deathb3); Self.NextThink = Time + 0.1; Self.Think = knight_dieb4; Self.Solid = SOLID_NOT }
func knight_dieb4()  { Self.Frame = float32(KNIGHT_FRAME_deathb4); Self.NextThink = Time + 0.1; Self.Think = knight_dieb5 }
func knight_dieb5()  { Self.Frame = float32(KNIGHT_FRAME_deathb5); Self.NextThink = Time + 0.1; Self.Think = knight_dieb6 }
func knight_dieb6()  { Self.Frame = float32(KNIGHT_FRAME_deathb6); Self.NextThink = Time + 0.1; Self.Think = knight_dieb7 }
func knight_dieb7()  { Self.Frame = float32(KNIGHT_FRAME_deathb7); Self.NextThink = Time + 0.1; Self.Think = knight_dieb8 }
func knight_dieb8()  { Self.Frame = float32(KNIGHT_FRAME_deathb8); Self.NextThink = Time + 0.1; Self.Think = knight_dieb9 }
func knight_dieb9()  { Self.Frame = float32(KNIGHT_FRAME_deathb9); Self.NextThink = Time + 0.1; Self.Think = knight_dieb10 }
func knight_dieb10() { Self.Frame = float32(KNIGHT_FRAME_deathb10); Self.NextThink = Time + 0.1; Self.Think = knight_dieb11 }
func knight_dieb11() { Self.Frame = float32(KNIGHT_FRAME_deathb11); Self.NextThink = Time + 0.1; Self.Think = knight_dieb11 }

func knight_die() {
	if Self.Health < -40 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_knight.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "knight/kdeath.wav", 1, ATTN_NORM)

	if engine.Random() < 0.5 {
		knight_die1()
	} else {
		knight_dieb1()
	}
}

func init() {
	knight_atk1 = knight_atk1_impl
	knight_runatk1 = knight_runatk1_impl
}

func monster_knight() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/knight.mdl")
	engine.PrecacheModel("progs/h_knight.mdl")

	engine.PrecacheSound("knight/kdeath.wav")
	engine.PrecacheSound("knight/khurt.wav")
	engine.PrecacheSound("knight/ksight.wav")
	engine.PrecacheSound("knight/sword1.wav")
	engine.PrecacheSound("knight/sword2.wav")
	engine.PrecacheSound("knight/idle.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/knight.mdl")

	Self.Noise = "knight/ksight.wav"
	Self.NetName = "$qc_knight"
	Self.KillString = "$qc_ks_knight"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 40))
	Self.Health = 75
	Self.MaxHealth = 75

	Self.ThStand = knight_stand1
	Self.ThWalk = knight_walk1
	Self.ThRun = knight_run1
	Self.ThMelee = knight_atk1_impl
	Self.ThPain = knight_pain
	Self.ThDie = knight_die
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_MELEE

	walkmonster_start()
}
