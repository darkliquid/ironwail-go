package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Scrag (Wizard) frames
	WIZ_FRAME_hover1 = iota
	WIZ_FRAME_hover2
	WIZ_FRAME_hover3
	WIZ_FRAME_hover4
	WIZ_FRAME_hover5
	WIZ_FRAME_hover6
	WIZ_FRAME_hover7
	WIZ_FRAME_hover8
	WIZ_FRAME_hover9
	WIZ_FRAME_hover10
	WIZ_FRAME_hover11
	WIZ_FRAME_hover12
	WIZ_FRAME_hover13
	WIZ_FRAME_hover14
	WIZ_FRAME_hover15

	WIZ_FRAME_fly1
	WIZ_FRAME_fly2
	WIZ_FRAME_fly3
	WIZ_FRAME_fly4
	WIZ_FRAME_fly5
	WIZ_FRAME_fly6
	WIZ_FRAME_fly7
	WIZ_FRAME_fly8
	WIZ_FRAME_fly9
	WIZ_FRAME_fly10
	WIZ_FRAME_fly11
	WIZ_FRAME_fly12
	WIZ_FRAME_fly13
	WIZ_FRAME_fly14

	WIZ_FRAME_magatt1
	WIZ_FRAME_magatt2
	WIZ_FRAME_magatt3
	WIZ_FRAME_magatt4
	WIZ_FRAME_magatt5
	WIZ_FRAME_magatt6
	WIZ_FRAME_magatt7
	WIZ_FRAME_magatt8
	WIZ_FRAME_magatt9
	WIZ_FRAME_magatt10
	WIZ_FRAME_magatt11
	WIZ_FRAME_magatt12
	WIZ_FRAME_magatt13

	WIZ_FRAME_pain1
	WIZ_FRAME_pain2
	WIZ_FRAME_pain3
	WIZ_FRAME_pain4

	WIZ_FRAME_death1
	WIZ_FRAME_death2
	WIZ_FRAME_death3
	WIZ_FRAME_death4
	WIZ_FRAME_death5
	WIZ_FRAME_death6
	WIZ_FRAME_death7
	WIZ_FRAME_death8
)

// Prototyped elsewhere
var wiz_run1 func()
var wiz_side1 func()

func LaunchMissile(missile *quake.Entity, mspeed, accuracy float32) {
	var vec, move quake.Vec3
	var fly float32

	makevectorsfixed(Self.Angles)

	vec = Self.Enemy.Origin.Add(Self.Enemy.Mins).Add(Self.Enemy.Size.Mul(0.7)).Sub(missile.Origin)

	fly = engine.Vlen(vec) / mspeed

	move = Self.Enemy.Velocity
	move[2] = 0

	vec = vec.Add(move.Mul(fly))

	vec = engine.Normalize(vec)
	vec = vec.Add(VUp.Mul(accuracy * (engine.Random() - 0.5))).Add(VRight.Mul(accuracy * (engine.Random() - 0.5)))

	missile.Velocity = vec.Mul(mspeed)

	missile.Angles = quake.MakeVec3(0, 0, 0)
	missile.Angles[1] = engine.Vectoyaw(missile.Velocity)

	missile.NextThink = Time + 5
	missile.Think = SUB_Remove
}

func WizardCheckAttack_impl() float32 {
	var spot1, spot2 quake.Vec3
	var targ *quake.Entity
	var chance float32

	if Time < Self.AttackFinished {
		return FALSE
	}

	if EnemyVisible == 0 {
		return FALSE
	}

	if EnemyRange == float32(RANGE_FAR) {
		if Self.AttackState != float32(AS_STRAIGHT) {
			Self.AttackState = float32(AS_STRAIGHT)
			wiz_run1()
		}
		return FALSE
	}

	targ = Self.Enemy

	spot1 = Self.Origin.Add(Self.ViewOfs)
	spot2 = targ.Origin.Add(targ.ViewOfs)

	engine.Traceline(spot1, spot2, FALSE, Self)

	if TraceEnt != targ {
		if Self.AttackState != float32(AS_STRAIGHT) {
			Self.AttackState = float32(AS_STRAIGHT)
			wiz_run1()
		}
		return FALSE
	}

	if EnemyRange == float32(RANGE_MELEE) {
		chance = 0.9
	} else if EnemyRange == float32(RANGE_NEAR) {
		chance = 0.6
	} else if EnemyRange == float32(RANGE_MID) {
		chance = 0.2
	} else {
		chance = 0
	}

	if engine.Random() < chance {
		Self.AttackState = float32(AS_MISSILE)
		return TRUE
	}

	if EnemyRange == float32(RANGE_MID) {
		if Self.AttackState != float32(AS_STRAIGHT) {
			Self.AttackState = float32(AS_STRAIGHT)
			wiz_run1()
		}
	} else {
		if Self.AttackState != float32(AS_SLIDING) {
			Self.AttackState = float32(AS_SLIDING)
			wiz_side1()
		}
	}

	return FALSE
}

func WizardAttackFinished() {
	if EnemyRange >= float32(RANGE_MID) || EnemyVisible == 0 {
		Self.AttackState = float32(AS_STRAIGHT)
		Self.Think = wiz_run1
	} else {
		Self.AttackState = float32(AS_SLIDING)
		Self.Think = wiz_side1
	}
}

func Wiz_FastFire() {
	var vec quake.Vec3
	var dst quake.Vec3

	if Self.Owner.Health > 0 {
		Self.Owner.Effects = float32(int(Self.Owner.Effects) | EF_MUZZLEFLASH)

		makevectorsfixed(Self.Enemy.Angles)
		dst = Self.Enemy.Origin.Sub(Self.MoveDir.Mul(13))

		vec = engine.Normalize(dst.Sub(Self.Origin))
		engine.Sound(Self, int(CHAN_WEAPON), "wizard/wattack.wav", 1, ATTN_NORM)
		launch_spike(Self.Origin, vec)
		Newmis.Velocity = vec.Mul(600)
		Newmis.Owner = Self.Owner
		Newmis.ClassName = "wizard_spike"
		engine.SetModel(Newmis, "progs/w_spike.mdl")
		engine.SetSize(Newmis, VEC_ORIGIN, VEC_ORIGIN)
	}

	engine.Remove(Self)
}

func Wiz_StartFast() {
	var missile *quake.Entity

	engine.Sound(Self, int(CHAN_WEAPON), "wizard/wattack.wav", 1, ATTN_NORM)
	Self.VAngle = Self.Angles
	makevectorsfixed(Self.Angles)

	missile = engine.Spawn()
	missile.Owner = Self
	missile.NextThink = Time + 0.6
	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(missile, Self.Origin.Add(quake.MakeVec3(0, 0, 30)).Add(VForward.Mul(14)).Add(VRight.Mul(14)))
	missile.Enemy = Self.Enemy
	missile.NextThink = Time + 0.8
	missile.Think = Wiz_FastFire
	missile.MoveDir = VRight

	missile = engine.Spawn()
	missile.Owner = Self
	missile.NextThink = Time + 1
	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(missile, Self.Origin.Add(quake.MakeVec3(0, 0, 30)).Add(VForward.Mul(14)).Add(VRight.Mul(-14)))
	missile.Enemy = Self.Enemy
	missile.NextThink = Time + 0.3
	missile.Think = Wiz_FastFire
	missile.MoveDir = VEC_ORIGIN.Sub(VRight)
}

func Wiz_idlesound() {
	var wr float32
	wr = engine.Random() * 5

	if Self.WaitMin < Time {
		Self.WaitMin = Time + 2
		if wr > 4.5 {
			engine.Sound(Self, int(CHAN_VOICE), "wizard/widle1.wav", 1, ATTN_IDLE)
		}
		if wr < 1.5 {
			engine.Sound(Self, int(CHAN_VOICE), "wizard/widle2.wav", 1, ATTN_IDLE)
		}
	}
}

func wiz_stand1()  { Self.Frame = float32(WIZ_FRAME_hover1); Self.NextThink = Time + 0.1; Self.Think = wiz_stand2; ai_stand() }
func wiz_stand2()  { Self.Frame = float32(WIZ_FRAME_hover2); Self.NextThink = Time + 0.1; Self.Think = wiz_stand3; ai_stand() }
func wiz_stand3()  { Self.Frame = float32(WIZ_FRAME_hover3); Self.NextThink = Time + 0.1; Self.Think = wiz_stand4; ai_stand() }
func wiz_stand4()  { Self.Frame = float32(WIZ_FRAME_hover4); Self.NextThink = Time + 0.1; Self.Think = wiz_stand5; ai_stand() }
func wiz_stand5()  { Self.Frame = float32(WIZ_FRAME_hover5); Self.NextThink = Time + 0.1; Self.Think = wiz_stand6; ai_stand() }
func wiz_stand6()  { Self.Frame = float32(WIZ_FRAME_hover6); Self.NextThink = Time + 0.1; Self.Think = wiz_stand7; ai_stand() }
func wiz_stand7()  { Self.Frame = float32(WIZ_FRAME_hover7); Self.NextThink = Time + 0.1; Self.Think = wiz_stand8; ai_stand() }
func wiz_stand8()  { Self.Frame = float32(WIZ_FRAME_hover8); Self.NextThink = Time + 0.1; Self.Think = wiz_stand1; ai_stand() }

func wiz_walk1() {
	Self.Frame = float32(WIZ_FRAME_hover1)
	Self.NextThink = Time + 0.1
	Self.Think = wiz_walk2
	ai_walk(8)
	Wiz_idlesound()
}

func wiz_walk2() { Self.Frame = float32(WIZ_FRAME_hover2); Self.NextThink = Time + 0.1; Self.Think = wiz_walk3; ai_walk(8) }
func wiz_walk3() { Self.Frame = float32(WIZ_FRAME_hover3); Self.NextThink = Time + 0.1; Self.Think = wiz_walk4; ai_walk(8) }
func wiz_walk4() { Self.Frame = float32(WIZ_FRAME_hover4); Self.NextThink = Time + 0.1; Self.Think = wiz_walk5; ai_walk(8) }
func wiz_walk5() { Self.Frame = float32(WIZ_FRAME_hover5); Self.NextThink = Time + 0.1; Self.Think = wiz_walk6; ai_walk(8) }
func wiz_walk6() { Self.Frame = float32(WIZ_FRAME_hover6); Self.NextThink = Time + 0.1; Self.Think = wiz_walk7; ai_walk(8) }
func wiz_walk7() { Self.Frame = float32(WIZ_FRAME_hover7); Self.NextThink = Time + 0.1; Self.Think = wiz_walk8; ai_walk(8) }
func wiz_walk8() { Self.Frame = float32(WIZ_FRAME_hover8); Self.NextThink = Time + 0.1; Self.Think = wiz_walk1; ai_walk(8) }

func wiz_side1_impl() {
	Self.Frame = float32(WIZ_FRAME_hover1)
	Self.NextThink = Time + 0.1
	Self.Think = wiz_side2
	ai_run(8)
	Wiz_idlesound()
}

func wiz_side2() { Self.Frame = float32(WIZ_FRAME_hover2); Self.NextThink = Time + 0.1; Self.Think = wiz_side3; ai_run(8) }
func wiz_side3() { Self.Frame = float32(WIZ_FRAME_hover3); Self.NextThink = Time + 0.1; Self.Think = wiz_side4; ai_run(8) }
func wiz_side4() { Self.Frame = float32(WIZ_FRAME_hover4); Self.NextThink = Time + 0.1; Self.Think = wiz_side5; ai_run(8) }
func wiz_side5() { Self.Frame = float32(WIZ_FRAME_hover5); Self.NextThink = Time + 0.1; Self.Think = wiz_side6; ai_run(8) }
func wiz_side6() { Self.Frame = float32(WIZ_FRAME_hover6); Self.NextThink = Time + 0.1; Self.Think = wiz_side7; ai_run(8) }
func wiz_side7() { Self.Frame = float32(WIZ_FRAME_hover7); Self.NextThink = Time + 0.1; Self.Think = wiz_side8; ai_run(8) }
func wiz_side8() { Self.Frame = float32(WIZ_FRAME_hover8); Self.NextThink = Time + 0.1; Self.Think = wiz_side1_impl; ai_run(8) }

func wiz_run1_impl() {
	Self.Frame = float32(WIZ_FRAME_fly1)
	Self.NextThink = Time + 0.1
	Self.Think = wiz_run2
	ai_run(16)
	Wiz_idlesound()
}

func wiz_run2()  { Self.Frame = float32(WIZ_FRAME_fly2); Self.NextThink = Time + 0.1; Self.Think = wiz_run3; ai_run(16) }
func wiz_run3()  { Self.Frame = float32(WIZ_FRAME_fly3); Self.NextThink = Time + 0.1; Self.Think = wiz_run4; ai_run(16) }
func wiz_run4()  { Self.Frame = float32(WIZ_FRAME_fly4); Self.NextThink = Time + 0.1; Self.Think = wiz_run5; ai_run(16) }
func wiz_run5()  { Self.Frame = float32(WIZ_FRAME_fly5); Self.NextThink = Time + 0.1; Self.Think = wiz_run6; ai_run(16) }
func wiz_run6()  { Self.Frame = float32(WIZ_FRAME_fly6); Self.NextThink = Time + 0.1; Self.Think = wiz_run7; ai_run(16) }
func wiz_run7()  { Self.Frame = float32(WIZ_FRAME_fly7); Self.NextThink = Time + 0.1; Self.Think = wiz_run8; ai_run(16) }
func wiz_run8()  { Self.Frame = float32(WIZ_FRAME_fly8); Self.NextThink = Time + 0.1; Self.Think = wiz_run9; ai_run(16) }
func wiz_run9()  { Self.Frame = float32(WIZ_FRAME_fly9); Self.NextThink = Time + 0.1; Self.Think = wiz_run10; ai_run(16) }
func wiz_run10() { Self.Frame = float32(WIZ_FRAME_fly10); Self.NextThink = Time + 0.1; Self.Think = wiz_run11; ai_run(16) }
func wiz_run11() { Self.Frame = float32(WIZ_FRAME_fly11); Self.NextThink = Time + 0.1; Self.Think = wiz_run12; ai_run(16) }
func wiz_run12() { Self.Frame = float32(WIZ_FRAME_fly12); Self.NextThink = Time + 0.1; Self.Think = wiz_run13; ai_run(16) }
func wiz_run13() { Self.Frame = float32(WIZ_FRAME_fly13); Self.NextThink = Time + 0.1; Self.Think = wiz_run14; ai_run(16) }
func wiz_run14() { Self.Frame = float32(WIZ_FRAME_fly14); Self.NextThink = Time + 0.1; Self.Think = wiz_run1_impl; ai_run(16) }

func wiz_fast1() {
	Self.Frame = float32(WIZ_FRAME_magatt1)
	Self.NextThink = Time + 0.1
	Self.Think = wiz_fast2
	ai_face()
	Wiz_StartFast()
}

func wiz_fast2() { Self.Frame = float32(WIZ_FRAME_magatt2); Self.NextThink = Time + 0.1; Self.Think = wiz_fast3; ai_face() }
func wiz_fast3() { Self.Frame = float32(WIZ_FRAME_magatt3); Self.NextThink = Time + 0.1; Self.Think = wiz_fast4; ai_face() }
func wiz_fast4() { Self.Frame = float32(WIZ_FRAME_magatt4); Self.NextThink = Time + 0.1; Self.Think = wiz_fast5; ai_face() }
func wiz_fast5() { Self.Frame = float32(WIZ_FRAME_magatt5); Self.NextThink = Time + 0.1; Self.Think = wiz_fast6; ai_face() }
func wiz_fast6() { Self.Frame = float32(WIZ_FRAME_magatt6); Self.NextThink = Time + 0.1; Self.Think = wiz_fast7; ai_face() }
func wiz_fast7() { Self.Frame = float32(WIZ_FRAME_magatt5); Self.NextThink = Time + 0.1; Self.Think = wiz_fast8; ai_face() }
func wiz_fast8() { Self.Frame = float32(WIZ_FRAME_magatt4); Self.NextThink = Time + 0.1; Self.Think = wiz_fast9; ai_face() }
func wiz_fast9() { Self.Frame = float32(WIZ_FRAME_magatt3); Self.NextThink = Time + 0.1; Self.Think = wiz_fast10; ai_face() }
func wiz_fast10() {
	Self.Frame = float32(WIZ_FRAME_magatt2)
	Self.NextThink = Time + 0.1
	Self.Think = wiz_run1_impl
	ai_face()
	SUB_AttackFinished(2)
	WizardAttackFinished()
}

func wiz_pain1() { Self.Frame = float32(WIZ_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = wiz_pain2 }
func wiz_pain2() { Self.Frame = float32(WIZ_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = wiz_pain3 }
func wiz_pain3() { Self.Frame = float32(WIZ_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = wiz_pain4 }
func wiz_pain4() { Self.Frame = float32(WIZ_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = wiz_run1_impl }

func wiz_death1() {
	Self.Frame = float32(WIZ_FRAME_death1)
	Self.NextThink = Time + 0.1
	Self.Think = wiz_death2

	Self.Velocity[0] = -200 + 400*engine.Random()
	Self.Velocity[1] = -200 + 400*engine.Random()
	Self.Velocity[2] = 100 + 100*engine.Random()
	Self.Flags = Self.Flags - float32(int(Self.Flags)&FL_ONGROUND)
	engine.Sound(Self, int(CHAN_VOICE), "wizard/wdeath.wav", 1, ATTN_NORM)
}

func wiz_death2() { Self.Frame = float32(WIZ_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = wiz_death3 }
func wiz_death3() { Self.Frame = float32(WIZ_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = wiz_death4; Self.Solid = SOLID_NOT }
func wiz_death4() { Self.Frame = float32(WIZ_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = wiz_death5 }
func wiz_death5() { Self.Frame = float32(WIZ_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = wiz_death6 }
func wiz_death6() { Self.Frame = float32(WIZ_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = wiz_death7 }
func wiz_death7() { Self.Frame = float32(WIZ_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = wiz_death8 }
func wiz_death8() { Self.Frame = float32(WIZ_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = wiz_death8 }

func wiz_die() {
	if Self.Health < -40 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_wizard.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		return
	}

	wiz_death1()
}

func Wiz_Pain(attacker *quake.Entity, damage float32) {
	engine.Sound(Self, int(CHAN_VOICE), "wizard/wpain.wav", 1, ATTN_NORM)
	if engine.Random()*70 > damage {
		return // didn't flinch
	}

	wiz_pain1()
}

func Wiz_Missile() {
	wiz_fast1()
}

func init() {
	WizardCheckAttack = WizardCheckAttack_impl
	wiz_run1 = wiz_run1_impl
	wiz_side1 = wiz_side1_impl
}

func monster_wizard() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/wizard.mdl")
	engine.PrecacheModel("progs/h_wizard.mdl")
	engine.PrecacheModel("progs/w_spike.mdl")

	engine.PrecacheSound("wizard/hit.wav") // used by c code
	engine.PrecacheSound("wizard/wattack.wav")
	engine.PrecacheSound("wizard/wdeath.wav")
	engine.PrecacheSound("wizard/widle1.wav")
	engine.PrecacheSound("wizard/widle2.wav")
	engine.PrecacheSound("wizard/wpain.wav")
	engine.PrecacheSound("wizard/wsight.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/wizard.mdl")

	Self.Noise = "wizard/wsight.wav"
	Self.NetName = "$qc_scrag"
	Self.KillString = "$qc_ks_scrag"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 40))
	Self.Health = 80
	Self.MaxHealth = 80

	Self.ThStand = wiz_stand1
	Self.ThWalk = wiz_walk1
	Self.ThRun = wiz_run1_impl
	Self.ThMissile = Wiz_Missile
	Self.ThPain = Wiz_Pain
	Self.ThDie = wiz_die
	Self.CombatStyle = CS_RANGED

	flymonster_start()
}
