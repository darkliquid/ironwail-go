package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Dog frames
	DOG_FRAME_attack1 = iota
	DOG_FRAME_attack2
	DOG_FRAME_attack3
	DOG_FRAME_attack4
	DOG_FRAME_attack5
	DOG_FRAME_attack6
	DOG_FRAME_attack7
	DOG_FRAME_attack8

	DOG_FRAME_death1
	DOG_FRAME_death2
	DOG_FRAME_death3
	DOG_FRAME_death4
	DOG_FRAME_death5
	DOG_FRAME_death6
	DOG_FRAME_death7
	DOG_FRAME_death8
	DOG_FRAME_death9

	DOG_FRAME_deathb1
	DOG_FRAME_deathb2
	DOG_FRAME_deathb3
	DOG_FRAME_deathb4
	DOG_FRAME_deathb5
	DOG_FRAME_deathb6
	DOG_FRAME_deathb7
	DOG_FRAME_deathb8
	DOG_FRAME_deathb9

	DOG_FRAME_pain1
	DOG_FRAME_pain2
	DOG_FRAME_pain3
	DOG_FRAME_pain4
	DOG_FRAME_pain5
	DOG_FRAME_pain6

	DOG_FRAME_painb1
	DOG_FRAME_painb2
	DOG_FRAME_painb3
	DOG_FRAME_painb4
	DOG_FRAME_painb5
	DOG_FRAME_painb6
	DOG_FRAME_painb7
	DOG_FRAME_painb8
	DOG_FRAME_painb9
	DOG_FRAME_painb10
	DOG_FRAME_painb11
	DOG_FRAME_painb12
	DOG_FRAME_painb13
	DOG_FRAME_painb14
	DOG_FRAME_painb15
	DOG_FRAME_painb16

	DOG_FRAME_run1
	DOG_FRAME_run2
	DOG_FRAME_run3
	DOG_FRAME_run4
	DOG_FRAME_run5
	DOG_FRAME_run6
	DOG_FRAME_run7
	DOG_FRAME_run8
	DOG_FRAME_run9
	DOG_FRAME_run10
	DOG_FRAME_run11
	DOG_FRAME_run12

	DOG_FRAME_leap1
	DOG_FRAME_leap2
	DOG_FRAME_leap3
	DOG_FRAME_leap4
	DOG_FRAME_leap5
	DOG_FRAME_leap6
	DOG_FRAME_leap7
	DOG_FRAME_leap8
	DOG_FRAME_leap9

	DOG_FRAME_stand1
	DOG_FRAME_stand2
	DOG_FRAME_stand3
	DOG_FRAME_stand4
	DOG_FRAME_stand5
	DOG_FRAME_stand6
	DOG_FRAME_stand7
	DOG_FRAME_stand8
	DOG_FRAME_stand9

	DOG_FRAME_walk1
	DOG_FRAME_walk2
	DOG_FRAME_walk3
	DOG_FRAME_walk4
	DOG_FRAME_walk5
	DOG_FRAME_walk6
	DOG_FRAME_walk7
	DOG_FRAME_walk8
)

// Prototyped elsewhere
var dog_leap1 func()
var dog_run1 func()
var Dog_JumpTouch func()

func dog_bite() {
	var delta quake.Vec3
	var ldmg float32

	if Self.Enemy == nil {
		return
	}

	ai_charge(10)

	if CanDamage(Self.Enemy, Self) == 0 {
		return
	}

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 100 {
		return
	}

	ldmg = (engine.Random() + engine.Random() + engine.Random()) * 8
	T_Damage(Self.Enemy, Self, Self, ldmg)
}

func Dog_JumpTouch_impl() {
	var ldmg float32

	if Self.Health <= 0 {
		return
	}

	if Other.TakeDamage != 0 {
		if engine.Vlen(Self.Velocity) > 300 {
			ldmg = 10 + 10*engine.Random()
			T_Damage(Other, Self, Self, ldmg)
		}
	}

	if engine.CheckBottom(Self) == 0 {
		if (int(Self.Flags) & FL_ONGROUND) != 0 {
			Self.Touch = SUB_Null
			Self.Think = dog_leap1
			Self.NextThink = Time + 0.1
		}
		return
	}

	Self.Touch = SUB_Null
	Self.Think = dog_run1
	Self.NextThink = Time + 0.1
}

func dog_stand1() { Self.Frame = float32(DOG_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = dog_stand2; ai_stand() }
func dog_stand2() { Self.Frame = float32(DOG_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = dog_stand3; ai_stand() }
func dog_stand3() { Self.Frame = float32(DOG_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = dog_stand4; ai_stand() }
func dog_stand4() { Self.Frame = float32(DOG_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = dog_stand5; ai_stand() }
func dog_stand5() { Self.Frame = float32(DOG_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = dog_stand6; ai_stand() }
func dog_stand6() { Self.Frame = float32(DOG_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = dog_stand7; ai_stand() }
func dog_stand7() { Self.Frame = float32(DOG_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = dog_stand8; ai_stand() }
func dog_stand8() { Self.Frame = float32(DOG_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = dog_stand9; ai_stand() }
func dog_stand9() { Self.Frame = float32(DOG_FRAME_stand9); Self.NextThink = Time + 0.1; Self.Think = dog_stand1; ai_stand() }

func dog_walk1() {
	Self.Frame = float32(DOG_FRAME_walk1)
	Self.NextThink = Time + 0.1
	Self.Think = dog_walk2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "dog/idle.wav", 1, ATTN_IDLE)
	}
	ai_walk(8)
}

func dog_walk2() { Self.Frame = float32(DOG_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = dog_walk3; ai_walk(8) }
func dog_walk3() { Self.Frame = float32(DOG_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = dog_walk4; ai_walk(8) }
func dog_walk4() { Self.Frame = float32(DOG_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = dog_walk5; ai_walk(8) }
func dog_walk5() { Self.Frame = float32(DOG_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = dog_walk6; ai_walk(8) }
func dog_walk6() { Self.Frame = float32(DOG_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = dog_walk7; ai_walk(8) }
func dog_walk7() { Self.Frame = float32(DOG_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = dog_walk8; ai_walk(8) }
func dog_walk8() { Self.Frame = float32(DOG_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = dog_walk1; ai_walk(8) }

func dog_run1_impl() {
	Self.Frame = float32(DOG_FRAME_run1)
	Self.NextThink = Time + 0.1
	Self.Think = dog_run2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "dog/idle.wav", 1, ATTN_IDLE)
	}
	ai_run(16)
}

func dog_run2()  { Self.Frame = float32(DOG_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = dog_run3; ai_run(32) }
func dog_run3()  { Self.Frame = float32(DOG_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = dog_run4; ai_run(32) }
func dog_run4()  { Self.Frame = float32(DOG_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = dog_run5; ai_run(20) }
func dog_run5()  { Self.Frame = float32(DOG_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = dog_run6; ai_run(64) }
func dog_run6()  { Self.Frame = float32(DOG_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = dog_run7; ai_run(32) }
func dog_run7()  { Self.Frame = float32(DOG_FRAME_run7); Self.NextThink = Time + 0.1; Self.Think = dog_run8; ai_run(16) }
func dog_run8()  { Self.Frame = float32(DOG_FRAME_run8); Self.NextThink = Time + 0.1; Self.Think = dog_run9; ai_run(32) }
func dog_run9()  { Self.Frame = float32(DOG_FRAME_run9); Self.NextThink = Time + 0.1; Self.Think = dog_run10; ai_run(32) }
func dog_run10() { Self.Frame = float32(DOG_FRAME_run10); Self.NextThink = Time + 0.1; Self.Think = dog_run11; ai_run(20) }
func dog_run11() { Self.Frame = float32(DOG_FRAME_run11); Self.NextThink = Time + 0.1; Self.Think = dog_run12; ai_run(64) }
func dog_run12() { Self.Frame = float32(DOG_FRAME_run12); Self.NextThink = Time + 0.1; Self.Think = dog_run1_impl; ai_run(32) }

func dog_atta1() { Self.Frame = float32(DOG_FRAME_attack1); Self.NextThink = Time + 0.1; Self.Think = dog_atta2; ai_charge(10) }
func dog_atta2() { Self.Frame = float32(DOG_FRAME_attack2); Self.NextThink = Time + 0.1; Self.Think = dog_atta3; ai_charge(10) }
func dog_atta3() { Self.Frame = float32(DOG_FRAME_attack3); Self.NextThink = Time + 0.1; Self.Think = dog_atta4; ai_charge(10) }
func dog_atta4() {
	Self.Frame = float32(DOG_FRAME_attack4)
	Self.NextThink = Time + 0.1
	Self.Think = dog_atta5
	engine.Sound(Self, int(CHAN_VOICE), "dog/dattack1.wav", 1, ATTN_NORM)
	dog_bite()
}

func dog_atta5() { Self.Frame = float32(DOG_FRAME_attack5); Self.NextThink = Time + 0.1; Self.Think = dog_atta6; ai_charge(10) }
func dog_atta6() { Self.Frame = float32(DOG_FRAME_attack6); Self.NextThink = Time + 0.1; Self.Think = dog_atta7; ai_charge(10) }
func dog_atta7() { Self.Frame = float32(DOG_FRAME_attack7); Self.NextThink = Time + 0.1; Self.Think = dog_atta8; ai_charge(10) }
func dog_atta8() { Self.Frame = float32(DOG_FRAME_attack8); Self.NextThink = Time + 0.1; Self.Think = dog_run1_impl; ai_charge(10) }

func dog_leap1_impl() { Self.Frame = float32(DOG_FRAME_leap1); Self.NextThink = Time + 0.1; Self.Think = dog_leap2; ai_face() }
func dog_leap2() {
	Self.Frame = float32(DOG_FRAME_leap2)
	Self.NextThink = Time + 0.1
	Self.Think = dog_leap3
	ai_face()

	Self.Touch = Dog_JumpTouch
	makevectorsfixed(Self.Angles)
	Self.Origin[2] = Self.Origin[2] + 1
	Self.Velocity = VForward.Mul(300).Add(quake.MakeVec3(0, 0, 200))

	if (int(Self.Flags) & FL_ONGROUND) != 0 {
		Self.Flags = Self.Flags - float32(FL_ONGROUND)
	}
}

func dog_leap3() { Self.Frame = float32(DOG_FRAME_leap3); Self.NextThink = Time + 0.1; Self.Think = dog_leap4 }
func dog_leap4() { Self.Frame = float32(DOG_FRAME_leap4); Self.NextThink = Time + 0.1; Self.Think = dog_leap5 }
func dog_leap5() { Self.Frame = float32(DOG_FRAME_leap5); Self.NextThink = Time + 0.1; Self.Think = dog_leap6 }
func dog_leap6() { Self.Frame = float32(DOG_FRAME_leap6); Self.NextThink = Time + 0.1; Self.Think = dog_leap7 }
func dog_leap7() { Self.Frame = float32(DOG_FRAME_leap7); Self.NextThink = Time + 0.1; Self.Think = dog_leap8 }
func dog_leap8() { Self.Frame = float32(DOG_FRAME_leap8); Self.NextThink = Time + 0.1; Self.Think = dog_leap9 }
func dog_leap9() { Self.Frame = float32(DOG_FRAME_leap9); Self.NextThink = Time + 0.1; Self.Think = dog_leap9 }

func dog_pain1() { Self.Frame = float32(DOG_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = dog_pain2 }
func dog_pain2() { Self.Frame = float32(DOG_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = dog_pain3 }
func dog_pain3() { Self.Frame = float32(DOG_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = dog_pain4 }
func dog_pain4() { Self.Frame = float32(DOG_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = dog_pain5 }
func dog_pain5() { Self.Frame = float32(DOG_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = dog_pain6 }
func dog_pain6() { Self.Frame = float32(DOG_FRAME_pain6); Self.NextThink = Time + 0.1; Self.Think = dog_run1_impl }

func dog_painb1()  { Self.Frame = float32(DOG_FRAME_painb1); Self.NextThink = Time + 0.1; Self.Think = dog_painb2 }
func dog_painb2()  { Self.Frame = float32(DOG_FRAME_painb2); Self.NextThink = Time + 0.1; Self.Think = dog_painb3 }
func dog_painb3()  { Self.Frame = float32(DOG_FRAME_painb3); Self.NextThink = Time + 0.1; Self.Think = dog_painb4; ai_pain(4) }
func dog_painb4()  { Self.Frame = float32(DOG_FRAME_painb4); Self.NextThink = Time + 0.1; Self.Think = dog_painb5; ai_pain(12) }
func dog_painb5()  { Self.Frame = float32(DOG_FRAME_painb5); Self.NextThink = Time + 0.1; Self.Think = dog_painb6; ai_pain(12) }
func dog_painb6()  { Self.Frame = float32(DOG_FRAME_painb6); Self.NextThink = Time + 0.1; Self.Think = dog_painb7; ai_pain(2) }
func dog_painb7()  { Self.Frame = float32(DOG_FRAME_painb7); Self.NextThink = Time + 0.1; Self.Think = dog_painb8 }
func dog_painb8()  { Self.Frame = float32(DOG_FRAME_painb8); Self.NextThink = Time + 0.1; Self.Think = dog_painb9; ai_pain(4) }
func dog_painb9()  { Self.Frame = float32(DOG_FRAME_painb9); Self.NextThink = Time + 0.1; Self.Think = dog_painb10 }
func dog_painb10() { Self.Frame = float32(DOG_FRAME_painb10); Self.NextThink = Time + 0.1; Self.Think = dog_painb11; ai_pain(10) }
func dog_painb11() { Self.Frame = float32(DOG_FRAME_painb11); Self.NextThink = Time + 0.1; Self.Think = dog_painb12 }
func dog_painb12() { Self.Frame = float32(DOG_FRAME_painb12); Self.NextThink = Time + 0.1; Self.Think = dog_painb13 }
func dog_painb13() { Self.Frame = float32(DOG_FRAME_painb13); Self.NextThink = Time + 0.1; Self.Think = dog_painb14 }
func dog_painb14() { Self.Frame = float32(DOG_FRAME_painb14); Self.NextThink = Time + 0.1; Self.Think = dog_painb15 }
func dog_painb15() { Self.Frame = float32(DOG_FRAME_painb15); Self.NextThink = Time + 0.1; Self.Think = dog_painb16 }
func dog_painb16() { Self.Frame = float32(DOG_FRAME_painb16); Self.NextThink = Time + 0.1; Self.Think = dog_run1_impl }

func dog_pain(attacker *quake.Entity, damage float32) {
	engine.Sound(Self, int(CHAN_VOICE), "dog/dpain1.wav", 1, ATTN_NORM)

	if engine.Random() > 0.5 {
		dog_pain1()
	} else {
		dog_painb1()
	}
}

func dog_die1() { Self.Frame = float32(DOG_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = dog_die2 }
func dog_die2() { Self.Frame = float32(DOG_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = dog_die3 }
func dog_die3() { Self.Frame = float32(DOG_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = dog_die4 }
func dog_die4() { Self.Frame = float32(DOG_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = dog_die5 }
func dog_die5() { Self.Frame = float32(DOG_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = dog_die6 }
func dog_die6() { Self.Frame = float32(DOG_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = dog_die7 }
func dog_die7() { Self.Frame = float32(DOG_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = dog_die8 }
func dog_die8() { Self.Frame = float32(DOG_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = dog_die9 }
func dog_die9() { Self.Frame = float32(DOG_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = dog_die9 }

func dog_dieb1() { Self.Frame = float32(DOG_FRAME_deathb1); Self.NextThink = Time + 0.1; Self.Think = dog_dieb2 }
func dog_dieb2() { Self.Frame = float32(DOG_FRAME_deathb2); Self.NextThink = Time + 0.1; Self.Think = dog_dieb3 }
func dog_dieb3() { Self.Frame = float32(DOG_FRAME_deathb3); Self.NextThink = Time + 0.1; Self.Think = dog_dieb4 }
func dog_dieb4() { Self.Frame = float32(DOG_FRAME_deathb4); Self.NextThink = Time + 0.1; Self.Think = dog_dieb5 }
func dog_dieb5() { Self.Frame = float32(DOG_FRAME_deathb5); Self.NextThink = Time + 0.1; Self.Think = dog_dieb6 }
func dog_dieb6() { Self.Frame = float32(DOG_FRAME_deathb6); Self.NextThink = Time + 0.1; Self.Think = dog_dieb7 }
func dog_dieb7() { Self.Frame = float32(DOG_FRAME_deathb7); Self.NextThink = Time + 0.1; Self.Think = dog_dieb8 }
func dog_dieb8() { Self.Frame = float32(DOG_FRAME_deathb8); Self.NextThink = Time + 0.1; Self.Think = dog_dieb9 }
func dog_dieb9() { Self.Frame = float32(DOG_FRAME_deathb9); Self.NextThink = Time + 0.1; Self.Think = dog_dieb9 }

func dog_die() {
	if Self.Health < -35 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowGib("progs/gib3.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		ThrowHead("progs/h_dog.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "dog/ddeath.wav", 1, ATTN_NORM)
	Self.Solid = SOLID_NOT

	if engine.Random() > 0.5 {
		dog_die1()
	} else {
		dog_dieb1()
	}
}

func CheckDogMelee() float32 {
	if EnemyRange == float32(RANGE_MELEE) {
		Self.AttackState = AS_MELEE
		return TRUE
	}

	return FALSE
}

func CheckDogJump() float32 {
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

	if d < 80 {
		return FALSE
	}

	if d > 150 {
		return FALSE
	}

	return TRUE
}

func DogCheckAttack_impl() float32 {
	if CheckDogMelee() != 0 {
		Self.AttackState = AS_MELEE
		return TRUE
	}

	if CheckDogJump() != 0 {
		Self.AttackState = AS_MISSILE
		return TRUE
	}

	return FALSE
}

func init() {
	DogCheckAttack = DogCheckAttack_impl
	dog_leap1 = dog_leap1_impl
	dog_run1 = dog_run1_impl
	Dog_JumpTouch = Dog_JumpTouch_impl
}

func monster_dog() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/h_dog.mdl")
	engine.PrecacheModel("progs/dog.mdl")

	engine.PrecacheSound("dog/dattack1.wav")
	engine.PrecacheSound("dog/ddeath.wav")
	engine.PrecacheSound("dog/dpain1.wav")
	engine.PrecacheSound("dog/dsight.wav")
	engine.PrecacheSound("dog/idle.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/dog.mdl")

	Self.Noise = "dog/dsight.wav"
	Self.NetName = "$qc_rottweiler"
	Self.KillString = "$qc_ks_rottweiler"

	engine.SetSize(Self, quake.MakeVec3(-32, -32, -24), quake.MakeVec3(32, 32, 40))
	Self.Health = 25
	Self.MaxHealth = 25

	Self.ThStand = dog_stand1
	Self.ThWalk = dog_walk1
	Self.ThRun = dog_run1_impl
	Self.ThPain = dog_pain
	Self.ThDie = dog_die
	Self.ThMelee = dog_atta1
	Self.ThMissile = dog_leap1_impl
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_MELEE

	walkmonster_start()
}
