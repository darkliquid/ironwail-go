package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Enforcer frames
	ENF_FRAME_stand1 = iota
	ENF_FRAME_stand2
	ENF_FRAME_stand3
	ENF_FRAME_stand4
	ENF_FRAME_stand5
	ENF_FRAME_stand6
	ENF_FRAME_stand7

	ENF_FRAME_walk1
	ENF_FRAME_walk2
	ENF_FRAME_walk3
	ENF_FRAME_walk4
	ENF_FRAME_walk5
	ENF_FRAME_walk6
	ENF_FRAME_walk7
	ENF_FRAME_walk8
	ENF_FRAME_walk9
	ENF_FRAME_walk10
	ENF_FRAME_walk11
	ENF_FRAME_walk12
	ENF_FRAME_walk13
	ENF_FRAME_walk14
	ENF_FRAME_walk15
	ENF_FRAME_walk16

	ENF_FRAME_run1
	ENF_FRAME_run2
	ENF_FRAME_run3
	ENF_FRAME_run4
	ENF_FRAME_run5
	ENF_FRAME_run6
	ENF_FRAME_run7
	ENF_FRAME_run8

	ENF_FRAME_attack1
	ENF_FRAME_attack2
	ENF_FRAME_attack3
	ENF_FRAME_attack4
	ENF_FRAME_attack5
	ENF_FRAME_attack6
	ENF_FRAME_attack7
	ENF_FRAME_attack8
	ENF_FRAME_attack9
	ENF_FRAME_attack10

	ENF_FRAME_death1
	ENF_FRAME_death2
	ENF_FRAME_death3
	ENF_FRAME_death4
	ENF_FRAME_death5
	ENF_FRAME_death6
	ENF_FRAME_death7
	ENF_FRAME_death8
	ENF_FRAME_death9
	ENF_FRAME_death10
	ENF_FRAME_death11
	ENF_FRAME_death12
	ENF_FRAME_death13
	ENF_FRAME_death14

	ENF_FRAME_fdeath1
	ENF_FRAME_fdeath2
	ENF_FRAME_fdeath3
	ENF_FRAME_fdeath4
	ENF_FRAME_fdeath5
	ENF_FRAME_fdeath6
	ENF_FRAME_fdeath7
	ENF_FRAME_fdeath8
	ENF_FRAME_fdeath9
	ENF_FRAME_fdeath10
	ENF_FRAME_fdeath11

	ENF_FRAME_paina1
	ENF_FRAME_paina2
	ENF_FRAME_paina3
	ENF_FRAME_paina4

	ENF_FRAME_painb1
	ENF_FRAME_painb2
	ENF_FRAME_painb3
	ENF_FRAME_painb4
	ENF_FRAME_painb5

	ENF_FRAME_painc1
	ENF_FRAME_painc2
	ENF_FRAME_painc3
	ENF_FRAME_painc4
	ENF_FRAME_painc5
	ENF_FRAME_painc6
	ENF_FRAME_painc7
	ENF_FRAME_painc8

	ENF_FRAME_paind1
	ENF_FRAME_paind2
	ENF_FRAME_paind3
	ENF_FRAME_paind4
	ENF_FRAME_paind5
	ENF_FRAME_paind6
	ENF_FRAME_paind7
	ENF_FRAME_paind8
	ENF_FRAME_paind9
	ENF_FRAME_paind10
	ENF_FRAME_paind11
	ENF_FRAME_paind12
	ENF_FRAME_paind13
	ENF_FRAME_paind14
	ENF_FRAME_paind15
	ENF_FRAME_paind16
	ENF_FRAME_paind17
	ENF_FRAME_paind18
	ENF_FRAME_paind19
)

func Laser_Touch() {
	var org quake.Vec3

	if Other == Self.Owner {
		return
	}

	if engine.PointContents(Self.Origin) == float32(CONTENT_SKY) {
		engine.Remove(Self)
		return
	}

	engine.Sound(Self, int(CHAN_WEAPON), "enforcer/enfstop.wav", 1, ATTN_STATIC)
	org = Self.Origin.Sub(engine.Normalize(Self.Velocity).Mul(8))

	if Other.Health != 0 {
		SpawnBlood(org, Self.Velocity.Mul(0.2), 15)
		T_Damage(Other, Self, Self.Owner, 15)
	} else {
		engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
		engine.WriteByte(MSG_BROADCAST, float32(TE_GUNSHOT))
		engine.WriteCoord(MSG_BROADCAST, org[0])
		engine.WriteCoord(MSG_BROADCAST, org[1])
		engine.WriteCoord(MSG_BROADCAST, org[2])
	}

	engine.Remove(Self)
}

func LaunchLaser(org, vec quake.Vec3) {
	if Self.ClassName == "monster_enforcer" {
		engine.Sound(Self, int(CHAN_WEAPON), "enforcer/enfire.wav", 1, ATTN_NORM)
	}

	vec = engine.Normalize(vec)

	Newmis = engine.Spawn()
	Newmis.ClassName = "enforcer_laser"
	Newmis.Owner = Self
	Newmis.MoveType = MOVETYPE_FLY
	Newmis.Solid = SOLID_BBOX
	Newmis.Effects = float32(EF_DIMLIGHT)

	engine.SetModel(Newmis, "progs/laser.mdl")
	engine.SetSize(Newmis, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))

	engine.SetOrigin(Newmis, org)

	Newmis.Velocity = vec.Mul(600)
	Newmis.Angles = engine.VectoAngles(Newmis.Velocity)

	Newmis.NextThink = Time + 5
	Newmis.Think = SUB_Remove
	Newmis.Touch = Laser_Touch
}

func enforcer_fire() {
	var org quake.Vec3

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
	makevectorsfixed(Self.Angles)

	org = Self.Origin.Add(VForward.Mul(30)).Add(VRight.Mul(8.5)).Add(quake.MakeVec3(0, 0, 16))

	LaunchLaser(org, Self.Enemy.Origin.Sub(Self.Origin))
}

func enf_stand1() { Self.Frame = float32(ENF_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = enf_stand2; ai_stand() }
func enf_stand2() { Self.Frame = float32(ENF_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = enf_stand3; ai_stand() }
func enf_stand3() { Self.Frame = float32(ENF_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = enf_stand4; ai_stand() }
func enf_stand4() { Self.Frame = float32(ENF_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = enf_stand5; ai_stand() }
func enf_stand5() { Self.Frame = float32(ENF_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = enf_stand6; ai_stand() }
func enf_stand6() { Self.Frame = float32(ENF_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = enf_stand7; ai_stand() }
func enf_stand7() { Self.Frame = float32(ENF_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = enf_stand1; ai_stand() }

func enf_walk1() {
	Self.Frame = float32(ENF_FRAME_walk1)
	Self.NextThink = Time + 0.1
	Self.Think = enf_walk2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "enforcer/idle1.wav", 1, ATTN_IDLE)
	}
	ai_walk(2)
}

func enf_walk2()  { Self.Frame = float32(ENF_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = enf_walk3; ai_walk(4) }
func enf_walk3()  { Self.Frame = float32(ENF_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = enf_walk4; ai_walk(4) }
func enf_walk4()  { Self.Frame = float32(ENF_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = enf_walk5; ai_walk(3) }
func enf_walk5()  { Self.Frame = float32(ENF_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = enf_walk6; ai_walk(1) }
func enf_walk6()  { Self.Frame = float32(ENF_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = enf_walk7; ai_walk(2) }
func enf_walk7()  { Self.Frame = float32(ENF_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = enf_walk8; ai_walk(2) }
func enf_walk8()  { Self.Frame = float32(ENF_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = enf_walk9; ai_walk(1) }
func enf_walk9()  { Self.Frame = float32(ENF_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = enf_walk10; ai_walk(2) }
func enf_walk10() { Self.Frame = float32(ENF_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = enf_walk11; ai_walk(4) }
func enf_walk11() { Self.Frame = float32(ENF_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = enf_walk12; ai_walk(4) }
func enf_walk12() { Self.Frame = float32(ENF_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = enf_walk13; ai_walk(1) }
func enf_walk13() { Self.Frame = float32(ENF_FRAME_walk13); Self.NextThink = Time + 0.1; Self.Think = enf_walk14; ai_walk(2) }
func enf_walk14() { Self.Frame = float32(ENF_FRAME_walk14); Self.NextThink = Time + 0.1; Self.Think = enf_walk15; ai_walk(3) }
func enf_walk15() { Self.Frame = float32(ENF_FRAME_walk15); Self.NextThink = Time + 0.1; Self.Think = enf_walk16; ai_walk(4) }
func enf_walk16() { Self.Frame = float32(ENF_FRAME_walk16); Self.NextThink = Time + 0.1; Self.Think = enf_walk1; ai_walk(2) }

func enf_run1() {
	Self.Frame = float32(ENF_FRAME_run1)
	Self.NextThink = Time + 0.1
	Self.Think = enf_run2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "enforcer/idle1.wav", 1, ATTN_IDLE)
	}
	ai_run(18)
}

func enf_run2() { Self.Frame = float32(ENF_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = enf_run3; ai_run(14) }
func enf_run3() { Self.Frame = float32(ENF_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = enf_run4; ai_run(7) }
func enf_run4() { Self.Frame = float32(ENF_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = enf_run5; ai_run(12) }
func enf_run5() { Self.Frame = float32(ENF_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = enf_run6; ai_run(14) }
func enf_run6() { Self.Frame = float32(ENF_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = enf_run7; ai_run(14) }
func enf_run7() { Self.Frame = float32(ENF_FRAME_run7); Self.NextThink = Time + 0.1; Self.Think = enf_run8; ai_run(7) }
func enf_run8() { Self.Frame = float32(ENF_FRAME_run8); Self.NextThink = Time + 0.1; Self.Think = enf_run1; ai_run(11) }

func enf_atk1() { Self.Frame = float32(ENF_FRAME_attack1); Self.NextThink = Time + 0.1; Self.Think = enf_atk2; ai_face() }
func enf_atk2() { Self.Frame = float32(ENF_FRAME_attack2); Self.NextThink = Time + 0.1; Self.Think = enf_atk3; ai_face() }
func enf_atk3() { Self.Frame = float32(ENF_FRAME_attack3); Self.NextThink = Time + 0.1; Self.Think = enf_atk4; ai_face() }
func enf_atk4() { Self.Frame = float32(ENF_FRAME_attack4); Self.NextThink = Time + 0.1; Self.Think = enf_atk5; ai_face() }
func enf_atk5() { Self.Frame = float32(ENF_FRAME_attack5); Self.NextThink = Time + 0.1; Self.Think = enf_atk6; ai_face() }
func enf_atk6() { Self.Frame = float32(ENF_FRAME_attack6); Self.NextThink = Time + 0.1; Self.Think = enf_atk7; enforcer_fire() }
func enf_atk7() { Self.Frame = float32(ENF_FRAME_attack7); Self.NextThink = Time + 0.1; Self.Think = enf_atk8; ai_face() }
func enf_atk8() { Self.Frame = float32(ENF_FRAME_attack8); Self.NextThink = Time + 0.1; Self.Think = enf_atk9; ai_face() }
func enf_atk9() { Self.Frame = float32(ENF_FRAME_attack5); Self.NextThink = Time + 0.1; Self.Think = enf_atk10; ai_face() }
func enf_atk10() { Self.Frame = float32(ENF_FRAME_attack6); Self.NextThink = Time + 0.1; Self.Think = enf_atk11; enforcer_fire() }
func enf_atk11() { Self.Frame = float32(ENF_FRAME_attack7); Self.NextThink = Time + 0.1; Self.Think = enf_atk12; ai_face() }
func enf_atk12() { Self.Frame = float32(ENF_FRAME_attack8); Self.NextThink = Time + 0.1; Self.Think = enf_atk13; ai_face() }
func enf_atk13() { Self.Frame = float32(ENF_FRAME_attack9); Self.NextThink = Time + 0.1; Self.Think = enf_atk14; ai_face() }
func enf_atk14() {
	Self.Frame = float32(ENF_FRAME_attack10)
	Self.NextThink = Time + 0.1
	Self.Think = enf_run1
	ai_face()
	SUB_CheckRefire(enf_atk1)
}

func enf_paina1() { Self.Frame = float32(ENF_FRAME_paina1); Self.NextThink = Time + 0.1; Self.Think = enf_paina2 }
func enf_paina2() { Self.Frame = float32(ENF_FRAME_paina2); Self.NextThink = Time + 0.1; Self.Think = enf_paina3 }
func enf_paina3() { Self.Frame = float32(ENF_FRAME_paina3); Self.NextThink = Time + 0.1; Self.Think = enf_paina4 }
func enf_paina4() { Self.Frame = float32(ENF_FRAME_paina4); Self.NextThink = Time + 0.1; Self.Think = enf_run1 }

func enf_painb1() { Self.Frame = float32(ENF_FRAME_painb1); Self.NextThink = Time + 0.1; Self.Think = enf_painb2 }
func enf_painb2() { Self.Frame = float32(ENF_FRAME_painb2); Self.NextThink = Time + 0.1; Self.Think = enf_painb3 }
func enf_painb3() { Self.Frame = float32(ENF_FRAME_painb3); Self.NextThink = Time + 0.1; Self.Think = enf_painb4 }
func enf_painb4() { Self.Frame = float32(ENF_FRAME_painb4); Self.NextThink = Time + 0.1; Self.Think = enf_painb5 }
func enf_painb5() { Self.Frame = float32(ENF_FRAME_painb5); Self.NextThink = Time + 0.1; Self.Think = enf_run1 }

func enf_painc1() { Self.Frame = float32(ENF_FRAME_painc1); Self.NextThink = Time + 0.1; Self.Think = enf_painc2 }
func enf_painc2() { Self.Frame = float32(ENF_FRAME_painc2); Self.NextThink = Time + 0.1; Self.Think = enf_painc3 }
func enf_painc3() { Self.Frame = float32(ENF_FRAME_painc3); Self.NextThink = Time + 0.1; Self.Think = enf_painc4 }
func enf_painc4() { Self.Frame = float32(ENF_FRAME_painc4); Self.NextThink = Time + 0.1; Self.Think = enf_painc5 }
func enf_painc5() { Self.Frame = float32(ENF_FRAME_painc5); Self.NextThink = Time + 0.1; Self.Think = enf_painc6 }
func enf_painc6() { Self.Frame = float32(ENF_FRAME_painc6); Self.NextThink = Time + 0.1; Self.Think = enf_painc7 }
func enf_painc7() { Self.Frame = float32(ENF_FRAME_painc7); Self.NextThink = Time + 0.1; Self.Think = enf_painc8 }
func enf_painc8() { Self.Frame = float32(ENF_FRAME_painc8); Self.NextThink = Time + 0.1; Self.Think = enf_run1 }

func enf_paind1()  { Self.Frame = float32(ENF_FRAME_paind1); Self.NextThink = Time + 0.1; Self.Think = enf_paind2 }
func enf_paind2()  { Self.Frame = float32(ENF_FRAME_paind2); Self.NextThink = Time + 0.1; Self.Think = enf_paind3 }
func enf_paind3()  { Self.Frame = float32(ENF_FRAME_paind3); Self.NextThink = Time + 0.1; Self.Think = enf_paind4 }
func enf_paind4()  { Self.Frame = float32(ENF_FRAME_paind4); Self.NextThink = Time + 0.1; Self.Think = enf_paind5; ai_painforward(2) }
func enf_paind5()  { Self.Frame = float32(ENF_FRAME_paind5); Self.NextThink = Time + 0.1; Self.Think = enf_paind6; ai_painforward(1) }
func enf_paind6()  { Self.Frame = float32(ENF_FRAME_paind6); Self.NextThink = Time + 0.1; Self.Think = enf_paind7 }
func enf_paind7()  { Self.Frame = float32(ENF_FRAME_paind7); Self.NextThink = Time + 0.1; Self.Think = enf_paind8 }
func enf_paind8()  { Self.Frame = float32(ENF_FRAME_paind8); Self.NextThink = Time + 0.1; Self.Think = enf_paind9 }
func enf_paind9()  { Self.Frame = float32(ENF_FRAME_paind9); Self.NextThink = Time + 0.1; Self.Think = enf_paind10 }
func enf_paind10() { Self.Frame = float32(ENF_FRAME_paind10); Self.NextThink = Time + 0.1; Self.Think = enf_paind11 }
func enf_paind11() { Self.Frame = float32(ENF_FRAME_paind11); Self.NextThink = Time + 0.1; Self.Think = enf_paind12; ai_painforward(1) }
func enf_paind12() { Self.Frame = float32(ENF_FRAME_paind12); Self.NextThink = Time + 0.1; Self.Think = enf_paind13; ai_painforward(1) }
func enf_paind13() { Self.Frame = float32(ENF_FRAME_paind13); Self.NextThink = Time + 0.1; Self.Think = enf_paind14; ai_painforward(1) }
func enf_paind14() { Self.Frame = float32(ENF_FRAME_paind14); Self.NextThink = Time + 0.1; Self.Think = enf_paind15 }
func enf_paind15() { Self.Frame = float32(ENF_FRAME_paind15); Self.NextThink = Time + 0.1; Self.Think = enf_paind16 }
func enf_paind16() { Self.Frame = float32(ENF_FRAME_paind16); Self.NextThink = Time + 0.1; Self.Think = enf_paind17; ai_pain(1) }
func enf_paind17() { Self.Frame = float32(ENF_FRAME_paind17); Self.NextThink = Time + 0.1; Self.Think = enf_paind18; ai_pain(1) }
func enf_paind18() { Self.Frame = float32(ENF_FRAME_paind18); Self.NextThink = Time + 0.1; Self.Think = enf_paind19 }
func enf_paind19() { Self.Frame = float32(ENF_FRAME_paind19); Self.NextThink = Time + 0.1; Self.Think = enf_run1 }

func enf_pain(attacker *quake.Entity, damage float32) {
	var r float32

	r = engine.Random()
	if Self.PainFinished > Time {
		return
	}

	if r < 0.5 {
		engine.Sound(Self, int(CHAN_VOICE), "enforcer/pain1.wav", 1, ATTN_NORM)
	} else {
		engine.Sound(Self, int(CHAN_VOICE), "enforcer/pain2.wav", 1, ATTN_NORM)
	}

	if r < 0.2 {
		Self.PainFinished = Time + 1
		enf_paina1()
	} else if r < 0.4 {
		Self.PainFinished = Time + 1
		enf_painb1()
	} else if r < 0.7 {
		Self.PainFinished = Time + 1
		enf_painc1()
	} else {
		Self.PainFinished = Time + 2
		enf_paind1()
	}
}

func enf_die1() { Self.Frame = float32(ENF_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = enf_die2 }
func enf_die2() { Self.Frame = float32(ENF_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = enf_die3 }
func enf_die3() {
	Self.Frame = float32(ENF_FRAME_death3)
	Self.NextThink = Time + 0.1
	Self.Think = enf_die4
	Self.Solid = SOLID_NOT
	Self.AmmoCells = 5
	DropBackpack()
}
func enf_die4()  { Self.Frame = float32(ENF_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = enf_die5; ai_forward(14) }
func enf_die5()  { Self.Frame = float32(ENF_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = enf_die6; ai_forward(2) }
func enf_die6()  { Self.Frame = float32(ENF_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = enf_die7 }
func enf_die7()  { Self.Frame = float32(ENF_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = enf_die8 }
func enf_die8()  { Self.Frame = float32(ENF_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = enf_die9 }
func enf_die9()  { Self.Frame = float32(ENF_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = enf_die10; ai_forward(3) }
func enf_die10() { Self.Frame = float32(ENF_FRAME_death10); Self.NextThink = Time + 0.1; Self.Think = enf_die11; ai_forward(5) }
func enf_die11() { Self.Frame = float32(ENF_FRAME_death11); Self.NextThink = Time + 0.1; Self.Think = enf_die12; ai_forward(5) }
func enf_die12() { Self.Frame = float32(ENF_FRAME_death12); Self.NextThink = Time + 0.1; Self.Think = enf_die13; ai_forward(5) }
func enf_die13() { Self.Frame = float32(ENF_FRAME_death13); Self.NextThink = Time + 0.1; Self.Think = enf_die14 }
func enf_die14() { Self.Frame = float32(ENF_FRAME_death14); Self.NextThink = Time + 0.1; Self.Think = enf_die14 }

func enf_fdie1() { Self.Frame = float32(ENF_FRAME_fdeath1); Self.NextThink = Time + 0.1; Self.Think = enf_fdie2 }
func enf_fdie2() { Self.Frame = float32(ENF_FRAME_fdeath2); Self.NextThink = Time + 0.1; Self.Think = enf_fdie3 }
func enf_fdie3() {
	Self.Frame = float32(ENF_FRAME_fdeath3)
	Self.NextThink = Time + 0.1
	Self.Think = enf_fdie4
	Self.Solid = SOLID_NOT
	Self.AmmoCells = 5
	DropBackpack()
}
func enf_fdie4()  { Self.Frame = float32(ENF_FRAME_fdeath4); Self.NextThink = Time + 0.1; Self.Think = enf_fdie5 }
func enf_fdie5()  { Self.Frame = float32(ENF_FRAME_fdeath5); Self.NextThink = Time + 0.1; Self.Think = enf_fdie6 }
func enf_fdie6()  { Self.Frame = float32(ENF_FRAME_fdeath6); Self.NextThink = Time + 0.1; Self.Think = enf_fdie7 }
func enf_fdie7()  { Self.Frame = float32(ENF_FRAME_fdeath7); Self.NextThink = Time + 0.1; Self.Think = enf_fdie8 }
func enf_fdie8()  { Self.Frame = float32(ENF_FRAME_fdeath8); Self.NextThink = Time + 0.1; Self.Think = enf_fdie9 }
func enf_fdie9()  { Self.Frame = float32(ENF_FRAME_fdeath9); Self.NextThink = Time + 0.1; Self.Think = enf_fdie10 }
func enf_fdie10() { Self.Frame = float32(ENF_FRAME_fdeath10); Self.NextThink = Time + 0.1; Self.Think = enf_fdie11 }
func enf_fdie11() { Self.Frame = float32(ENF_FRAME_fdeath11); Self.NextThink = Time + 0.1; Self.Think = enf_fdie11 }

func enf_die() {
	if Self.Health < -35 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_mega.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "enforcer/death1.wav", 1, ATTN_NORM)

	if engine.Random() > 0.5 {
		enf_die1()
	} else {
		enf_fdie1()
	}
}

func monster_enforcer() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel2("progs/enforcer.mdl")
	engine.PrecacheModel2("progs/h_mega.mdl")
	engine.PrecacheModel2("progs/laser.mdl")

	engine.PrecacheSound2("enforcer/death1.wav")
	engine.PrecacheSound2("enforcer/enfire.wav")
	engine.PrecacheSound2("enforcer/enfstop.wav")
	engine.PrecacheSound2("enforcer/idle1.wav")
	engine.PrecacheSound2("enforcer/pain1.wav")
	engine.PrecacheSound2("enforcer/pain2.wav")
	engine.PrecacheSound2("enforcer/sight1.wav")
	engine.PrecacheSound2("enforcer/sight2.wav")
	engine.PrecacheSound2("enforcer/sight3.wav")
	engine.PrecacheSound2("enforcer/sight4.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/enforcer.mdl")

	Self.NetName = "$qc_enforcer"
	Self.KillString = "$qc_ks_enforcer"
	Self.Noise = "enforcer/sight1.wav"
	Self.Noise1 = "enforcer/sight2.wav"
	Self.Noise2 = "enforcer/sight3.wav"
	Self.Noise3 = "enforcer/sight4.wav"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 40))
	Self.Health = 80
	Self.MaxHealth = 80

	Self.ThStand = enf_stand1
	Self.ThWalk = enf_walk1
	Self.ThRun = enf_run1
	Self.ThPain = enf_pain
	Self.ThDie = enf_die
	Self.ThMissile = enf_atk1
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_RANGED

	walkmonster_start()
}
