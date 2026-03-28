package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Fiend (Demon) frames
	FIEND_FRAME_stand1 = iota
	FIEND_FRAME_stand2
	FIEND_FRAME_stand3
	FIEND_FRAME_stand4
	FIEND_FRAME_stand5
	FIEND_FRAME_stand6
	FIEND_FRAME_stand7
	FIEND_FRAME_stand8
	FIEND_FRAME_stand9
	FIEND_FRAME_stand10
	FIEND_FRAME_stand11
	FIEND_FRAME_stand12
	FIEND_FRAME_stand13

	FIEND_FRAME_walk1
	FIEND_FRAME_walk2
	FIEND_FRAME_walk3
	FIEND_FRAME_walk4
	FIEND_FRAME_walk5
	FIEND_FRAME_walk6
	FIEND_FRAME_walk7
	FIEND_FRAME_walk8

	FIEND_FRAME_run1
	FIEND_FRAME_run2
	FIEND_FRAME_run3
	FIEND_FRAME_run4
	FIEND_FRAME_run5
	FIEND_FRAME_run6

	FIEND_FRAME_leap1
	FIEND_FRAME_leap2
	FIEND_FRAME_leap3
	FIEND_FRAME_leap4
	FIEND_FRAME_leap5
	FIEND_FRAME_leap6
	FIEND_FRAME_leap7
	FIEND_FRAME_leap8
	FIEND_FRAME_leap9
	FIEND_FRAME_leap10
	FIEND_FRAME_leap11
	FIEND_FRAME_leap12

	FIEND_FRAME_pain1
	FIEND_FRAME_pain2
	FIEND_FRAME_pain3
	FIEND_FRAME_pain4
	FIEND_FRAME_pain5
	FIEND_FRAME_pain6

	FIEND_FRAME_death1
	FIEND_FRAME_death2
	FIEND_FRAME_death3
	FIEND_FRAME_death4
	FIEND_FRAME_death5
	FIEND_FRAME_death6
	FIEND_FRAME_death7
	FIEND_FRAME_death8
	FIEND_FRAME_death9

	FIEND_FRAME_attacka1
	FIEND_FRAME_attacka2
	FIEND_FRAME_attacka3
	FIEND_FRAME_attacka4
	FIEND_FRAME_attacka5
	FIEND_FRAME_attacka6
	FIEND_FRAME_attacka7
	FIEND_FRAME_attacka8
	FIEND_FRAME_attacka9
	FIEND_FRAME_attacka10
	FIEND_FRAME_attacka11
	FIEND_FRAME_attacka12
	FIEND_FRAME_attacka13
	FIEND_FRAME_attacka14
	FIEND_FRAME_attacka15
)

// Prototyped elsewhere
var Demon_JumpTouch func()

func demon1_stand1()  { Self.Frame = float32(FIEND_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = demon1_stand2; ai_stand() }
func demon1_stand2()  { Self.Frame = float32(FIEND_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = demon1_stand3; ai_stand() }
func demon1_stand3()  { Self.Frame = float32(FIEND_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = demon1_stand4; ai_stand() }
func demon1_stand4()  { Self.Frame = float32(FIEND_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = demon1_stand5; ai_stand() }
func demon1_stand5()  { Self.Frame = float32(FIEND_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = demon1_stand6; ai_stand() }
func demon1_stand6()  { Self.Frame = float32(FIEND_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = demon1_stand7; ai_stand() }
func demon1_stand7()  { Self.Frame = float32(FIEND_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = demon1_stand8; ai_stand() }
func demon1_stand8()  { Self.Frame = float32(FIEND_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = demon1_stand9; ai_stand() }
func demon1_stand9()  { Self.Frame = float32(FIEND_FRAME_stand9); Self.NextThink = Time + 0.1; Self.Think = demon1_stand10; ai_stand() }
func demon1_stand10() { Self.Frame = float32(FIEND_FRAME_stand10); Self.NextThink = Time + 0.1; Self.Think = demon1_stand11; ai_stand() }
func demon1_stand11() { Self.Frame = float32(FIEND_FRAME_stand11); Self.NextThink = Time + 0.1; Self.Think = demon1_stand12; ai_stand() }
func demon1_stand12() { Self.Frame = float32(FIEND_FRAME_stand12); Self.NextThink = Time + 0.1; Self.Think = demon1_stand13; ai_stand() }
func demon1_stand13() { Self.Frame = float32(FIEND_FRAME_stand13); Self.NextThink = Time + 0.1; Self.Think = demon1_stand1; ai_stand() }

func demon1_walk1() {
	Self.Frame = float32(FIEND_FRAME_walk1)
	Self.NextThink = Time + 0.1
	Self.Think = demon1_walk2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "demon/idle1.wav", 1, ATTN_IDLE)
	}
	ai_walk(8)
}
func demon1_walk2() { Self.Frame = float32(FIEND_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = demon1_walk3; ai_walk(6) }
func demon1_walk3() { Self.Frame = float32(FIEND_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = demon1_walk4; ai_walk(6) }
func demon1_walk4() { Self.Frame = float32(FIEND_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = demon1_walk5; ai_walk(7) }
func demon1_walk5() { Self.Frame = float32(FIEND_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = demon1_walk6; ai_walk(4) }
func demon1_walk6() { Self.Frame = float32(FIEND_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = demon1_walk7; ai_walk(6) }
func demon1_walk7() { Self.Frame = float32(FIEND_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = demon1_walk8; ai_walk(10) }
func demon1_walk8() { Self.Frame = float32(FIEND_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = demon1_walk1; ai_walk(10) }

func demon1_run1() {
	Self.Frame = float32(FIEND_FRAME_run1)
	Self.NextThink = Time + 0.1
	Self.Think = demon1_run2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "demon/idle1.wav", 1, ATTN_IDLE)
	}
	ai_run(20)
}
func demon1_run2() { Self.Frame = float32(FIEND_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = demon1_run3; ai_run(15) }
func demon1_run3() { Self.Frame = float32(FIEND_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = demon1_run4; ai_run(36) }
func demon1_run4() { Self.Frame = float32(FIEND_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = demon1_run5; ai_run(20) }
func demon1_run5() { Self.Frame = float32(FIEND_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = demon1_run6; ai_run(15) }
func demon1_run6() { Self.Frame = float32(FIEND_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = demon1_run1; ai_run(36) }

func demon1_jump1() { Self.Frame = float32(FIEND_FRAME_leap1); Self.NextThink = Time + 0.1; Self.Think = demon1_jump2; ai_face() }
func demon1_jump2() { Self.Frame = float32(FIEND_FRAME_leap2); Self.NextThink = Time + 0.1; Self.Think = demon1_jump3; ai_face() }
func demon1_jump3() { Self.Frame = float32(FIEND_FRAME_leap3); Self.NextThink = Time + 0.1; Self.Think = demon1_jump4; ai_face() }
func demon1_jump4() {
	Self.Frame = float32(FIEND_FRAME_leap4)
	Self.NextThink = Time + 0.1
	Self.Think = demon1_jump5
	ai_face()

	Self.Touch = Demon_JumpTouch
	makevectorsfixed(Self.Angles)
	Self.Origin[2] = Self.Origin[2] + 1
	Self.Velocity = VForward.Mul(600).Add(quake.MakeVec3(0, 0, 250))

	if (int(Self.Flags) & FL_ONGROUND) != 0 {
		Self.Flags = Self.Flags - float32(FL_ONGROUND)
	}
}

func demon1_jump5()  { Self.Frame = float32(FIEND_FRAME_leap5); Self.NextThink = Time + 0.1; Self.Think = demon1_jump6 }
func demon1_jump6()  { Self.Frame = float32(FIEND_FRAME_leap6); Self.NextThink = Time + 0.1; Self.Think = demon1_jump7 }
func demon1_jump7()  { Self.Frame = float32(FIEND_FRAME_leap7); Self.NextThink = Time + 0.1; Self.Think = demon1_jump8 }
func demon1_jump8()  { Self.Frame = float32(FIEND_FRAME_leap8); Self.NextThink = Time + 0.1; Self.Think = demon1_jump9 }
func demon1_jump9()  { Self.Frame = float32(FIEND_FRAME_leap9); Self.NextThink = Time + 0.1; Self.Think = demon1_jump10 }
func demon1_jump10() {
	Self.Frame = float32(FIEND_FRAME_leap10)
	Self.NextThink = Time + 0.1
	Self.Think = demon1_jump1
	Self.NextThink = Time + 3 // if three seconds pass, assume demon is stuck and jump again
}

func demon1_jump11() { Self.Frame = float32(FIEND_FRAME_leap11); Self.NextThink = Time + 0.1; Self.Think = demon1_jump12 }
func demon1_jump12() { Self.Frame = float32(FIEND_FRAME_leap12); Self.NextThink = Time + 0.1; Self.Think = demon1_run1 }

func demon1_atta1()  { Self.Frame = float32(FIEND_FRAME_attacka1); Self.NextThink = Time + 0.1; Self.Think = demon1_atta2; ai_charge(4) }
func demon1_atta2()  { Self.Frame = float32(FIEND_FRAME_attacka2); Self.NextThink = Time + 0.1; Self.Think = demon1_atta3; ai_charge(0) }
func demon1_atta3()  { Self.Frame = float32(FIEND_FRAME_attacka3); Self.NextThink = Time + 0.1; Self.Think = demon1_atta4; ai_charge(0) }
func demon1_atta4()  { Self.Frame = float32(FIEND_FRAME_attacka4); Self.NextThink = Time + 0.1; Self.Think = demon1_atta5; ai_charge(1) }
func demon1_atta5()  { Self.Frame = float32(FIEND_FRAME_attacka5); Self.NextThink = Time + 0.1; Self.Think = demon1_atta6; ai_charge(2); Demon_Melee_impl(200) }
func demon1_atta6()  { Self.Frame = float32(FIEND_FRAME_attacka6); Self.NextThink = Time + 0.1; Self.Think = demon1_atta7; ai_charge(1) }
func demon1_atta7()  { Self.Frame = float32(FIEND_FRAME_attacka7); Self.NextThink = Time + 0.1; Self.Think = demon1_atta8; ai_charge(6) }
func demon1_atta8()  { Self.Frame = float32(FIEND_FRAME_attacka8); Self.NextThink = Time + 0.1; Self.Think = demon1_atta9; ai_charge(8) }
func demon1_atta9()  { Self.Frame = float32(FIEND_FRAME_attacka9); Self.NextThink = Time + 0.1; Self.Think = demon1_atta10; ai_charge(4) }
func demon1_atta10() { Self.Frame = float32(FIEND_FRAME_attacka10); Self.NextThink = Time + 0.1; Self.Think = demon1_atta11; ai_charge(2) }
func demon1_atta11() { Self.Frame = float32(FIEND_FRAME_attacka11); Self.NextThink = Time + 0.1; Self.Think = demon1_atta12; Demon_Melee_impl(-200) }
func demon1_atta12() { Self.Frame = float32(FIEND_FRAME_attacka12); Self.NextThink = Time + 0.1; Self.Think = demon1_atta13; ai_charge(5) }
func demon1_atta13() { Self.Frame = float32(FIEND_FRAME_attacka13); Self.NextThink = Time + 0.1; Self.Think = demon1_atta14; ai_charge(8) }
func demon1_atta14() { Self.Frame = float32(FIEND_FRAME_attacka14); Self.NextThink = Time + 0.1; Self.Think = demon1_atta15; ai_charge(4) }
func demon1_atta15() { Self.Frame = float32(FIEND_FRAME_attacka15); Self.NextThink = Time + 0.1; Self.Think = demon1_run1; ai_charge(4) }

func demon1_pain1() { Self.Frame = float32(FIEND_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = demon1_pain2 }
func demon1_pain2() { Self.Frame = float32(FIEND_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = demon1_pain3 }
func demon1_pain3() { Self.Frame = float32(FIEND_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = demon1_pain4 }
func demon1_pain4() { Self.Frame = float32(FIEND_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = demon1_pain5 }
func demon1_pain5() { Self.Frame = float32(FIEND_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = demon1_pain6 }
func demon1_pain6() { Self.Frame = float32(FIEND_FRAME_pain6); Self.NextThink = Time + 0.1; Self.Think = demon1_run1 }

// Note: To avoid redefining demon1_pain from monsters.go or fight.go
func demon1_pain_impl(attacker *quake.Entity, damage float32) {
	// Function comparison is tricky; checking if the current touch is Demon_JumpTouch
	// requires a workaround or direct pointer tracking, assuming jump touch sets it.
	// Since we can't reliably do `if Self.Touch == Demon_JumpTouch` easily in Go,
	// we'll rely on the PainFinished check and health state.
	// Alternatively, we can assume if it's jumping (in leap frames), it doesn't feel pain.

	if Self.Frame >= float32(FIEND_FRAME_leap1) && Self.Frame <= float32(FIEND_FRAME_leap12) {
		return
	}

	if Self.PainFinished > Time {
		return
	}

	Self.PainFinished = Time + 1

	if engine.Random()*200 > damage {
		return // didn't flinch
	}

	demon1_pain1()
}

func demon1_die1() {
	Self.Frame = float32(FIEND_FRAME_death1)
	Self.NextThink = Time + 0.1
	Self.Think = demon1_die2
	engine.Sound(Self, int(CHAN_VOICE), "demon/ddeath.wav", 1, ATTN_NORM)
}

func demon1_die2() { Self.Frame = float32(FIEND_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = demon1_die3 }
func demon1_die3() { Self.Frame = float32(FIEND_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = demon1_die4 }
func demon1_die4() { Self.Frame = float32(FIEND_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = demon1_die5 }
func demon1_die5() { Self.Frame = float32(FIEND_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = demon1_die6 }
func demon1_die6() {
	Self.Frame = float32(FIEND_FRAME_death6)
	Self.NextThink = Time + 0.1
	Self.Think = demon1_die7
	Self.Solid = SOLID_NOT
}
func demon1_die7() { Self.Frame = float32(FIEND_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = demon1_die8 }
func demon1_die8() { Self.Frame = float32(FIEND_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = demon1_die9 }
func demon1_die9() { Self.Frame = float32(FIEND_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = demon1_die9 }

func demon_die() {
	if Self.Health < -80 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_demon.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		return
	}

	demon1_die1()
}

func Demon_MeleeAttack() {
	demon1_atta1()
}

func monster_demon1() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/demon.mdl")
	engine.PrecacheModel("progs/h_demon.mdl")

	engine.PrecacheSound("demon/ddeath.wav")
	engine.PrecacheSound("demon/dhit2.wav")
	engine.PrecacheSound("demon/djump.wav")
	engine.PrecacheSound("demon/dpain1.wav")
	engine.PrecacheSound("demon/idle1.wav")
	engine.PrecacheSound("demon/sight2.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/demon.mdl")
	Self.Noise = "demon/sight2.wav"
	Self.KillString = "$qc_ks_fiend"
	Self.NetName = "$qc_fiend"

	engine.SetSize(Self, VEC_HULL2_MIN, VEC_HULL2_MAX)
	Self.Health = 300
	Self.MaxHealth = 300

	Self.ThStand = demon1_stand1
	Self.ThWalk = demon1_walk1
	Self.ThRun = demon1_run1
	Self.ThDie = demon_die
	Self.ThMelee = Demon_MeleeAttack
	Self.ThMissile = demon1_jump1
	Self.ThPain = demon1_pain_impl
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_MELEE

	walkmonster_start()
}

func CheckDemonMelee() float32 {
	if EnemyRange == float32(RANGE_MELEE) {
		Self.AttackState = AS_MELEE
		return TRUE
	}
	return FALSE
}

func CheckDemonJump() float32 {
	var dist quake.Vec3
	var d float32

	if Self.Origin[2]+Self.Mins[2] > Self.Enemy.Origin[2]+Self.Enemy.Mins[2]+0.75*Self.Enemy.Size[2] {
		return FALSE
	}

	if Self.Origin[2]+Self.Maxs[2] < Self.Enemy.Origin[2]+Self.Enemy.Mins[2]+0.25*Self.Enemy.Size[2] {
		return FALSE
	}

	dist = Self.Enemy.Origin.Sub(Self.Origin)
	dist[2] = 0

	d = engine.Vlen(dist)

	if d < 100 {
		return FALSE
	}

	if d > 200 {
		if engine.Random() < 0.9 {
			return FALSE
		}
	}

	return TRUE
}

func DemonCheckAttack_impl() float32 {
	if CheckDemonMelee() != 0 {
		Self.AttackState = AS_MELEE
		return TRUE
	}

	if CheckDemonJump() != 0 {
		Self.AttackState = AS_MISSILE
		return TRUE
	}

	return FALSE
}

func Demon_Melee_impl(side float32) {
	var ldmg float32
	var delta quake.Vec3

	ai_face()
	engine.WalkMove(Self.IdealYaw, 12)

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 100 {
		return
	}

	if CanDamage(Self.Enemy, Self) == 0 {
		return
	}

	ldmg = 10 + 5*engine.Random()
	T_Damage(Self.Enemy, Self, Self, ldmg)

	makevectorsfixed(Self.Angles)
	SpawnMeatSpray(Self.Origin.Add(VForward.Mul(16)), VRight.Mul(side))
}

func Demon_JumpTouch_impl() {
	var ldmg float32

	if Self.Health <= 0 {
		return
	}

	if Other.TakeDamage != 0 {
		if engine.Vlen(Self.Velocity) > 400 {
			ldmg = 40 + 10*engine.Random()
			T_Damage(Other, Self, Self, ldmg)
		}
	}

	if engine.CheckBottom(Self) == 0 {
		if (int(Self.Flags) & FL_ONGROUND) != 0 {
			Self.Touch = SUB_Null
			Self.Think = demon1_jump1
			Self.NextThink = Time + 0.1
		}
		return
	}

	Self.Touch = SUB_Null
	Self.Think = demon1_jump11
	Self.NextThink = Time + 0.1
}

func init() {
	DemonCheckAttack = DemonCheckAttack_impl
	Demon_Melee = Demon_Melee_impl
	Demon_JumpTouch = Demon_JumpTouch_impl
}
