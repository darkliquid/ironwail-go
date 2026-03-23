package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Shambler frames
	SHAMBLER_FRAME_stand1 = iota
	SHAMBLER_FRAME_stand2
	SHAMBLER_FRAME_stand3
	SHAMBLER_FRAME_stand4
	SHAMBLER_FRAME_stand5
	SHAMBLER_FRAME_stand6
	SHAMBLER_FRAME_stand7
	SHAMBLER_FRAME_stand8
	SHAMBLER_FRAME_stand9
	SHAMBLER_FRAME_stand10
	SHAMBLER_FRAME_stand11
	SHAMBLER_FRAME_stand12
	SHAMBLER_FRAME_stand13
	SHAMBLER_FRAME_stand14
	SHAMBLER_FRAME_stand15
	SHAMBLER_FRAME_stand16
	SHAMBLER_FRAME_stand17

	SHAMBLER_FRAME_walk1
	SHAMBLER_FRAME_walk2
	SHAMBLER_FRAME_walk3
	SHAMBLER_FRAME_walk4
	SHAMBLER_FRAME_walk5
	SHAMBLER_FRAME_walk6
	SHAMBLER_FRAME_walk7
	SHAMBLER_FRAME_walk8
	SHAMBLER_FRAME_walk9
	SHAMBLER_FRAME_walk10
	SHAMBLER_FRAME_walk11
	SHAMBLER_FRAME_walk12

	SHAMBLER_FRAME_run1
	SHAMBLER_FRAME_run2
	SHAMBLER_FRAME_run3
	SHAMBLER_FRAME_run4
	SHAMBLER_FRAME_run5
	SHAMBLER_FRAME_run6

	SHAMBLER_FRAME_smash1
	SHAMBLER_FRAME_smash2
	SHAMBLER_FRAME_smash3
	SHAMBLER_FRAME_smash4
	SHAMBLER_FRAME_smash5
	SHAMBLER_FRAME_smash6
	SHAMBLER_FRAME_smash7
	SHAMBLER_FRAME_smash8
	SHAMBLER_FRAME_smash9
	SHAMBLER_FRAME_smash10
	SHAMBLER_FRAME_smash11
	SHAMBLER_FRAME_smash12

	SHAMBLER_FRAME_swingr1
	SHAMBLER_FRAME_swingr2
	SHAMBLER_FRAME_swingr3
	SHAMBLER_FRAME_swingr4
	SHAMBLER_FRAME_swingr5
	SHAMBLER_FRAME_swingr6
	SHAMBLER_FRAME_swingr7
	SHAMBLER_FRAME_swingr8
	SHAMBLER_FRAME_swingr9

	SHAMBLER_FRAME_swingl1
	SHAMBLER_FRAME_swingl2
	SHAMBLER_FRAME_swingl3
	SHAMBLER_FRAME_swingl4
	SHAMBLER_FRAME_swingl5
	SHAMBLER_FRAME_swingl6
	SHAMBLER_FRAME_swingl7
	SHAMBLER_FRAME_swingl8
	SHAMBLER_FRAME_swingl9

	SHAMBLER_FRAME_magic1
	SHAMBLER_FRAME_magic2
	SHAMBLER_FRAME_magic3
	SHAMBLER_FRAME_magic4
	SHAMBLER_FRAME_magic5
	SHAMBLER_FRAME_magic6
	SHAMBLER_FRAME_magic7
	SHAMBLER_FRAME_magic8
	SHAMBLER_FRAME_magic9
	SHAMBLER_FRAME_magic10
	SHAMBLER_FRAME_magic11
	SHAMBLER_FRAME_magic12

	SHAMBLER_FRAME_pain1
	SHAMBLER_FRAME_pain2
	SHAMBLER_FRAME_pain3
	SHAMBLER_FRAME_pain4
	SHAMBLER_FRAME_pain5
	SHAMBLER_FRAME_pain6

	SHAMBLER_FRAME_death1
	SHAMBLER_FRAME_death2
	SHAMBLER_FRAME_death3
	SHAMBLER_FRAME_death4
	SHAMBLER_FRAME_death5
	SHAMBLER_FRAME_death6
	SHAMBLER_FRAME_death7
	SHAMBLER_FRAME_death8
	SHAMBLER_FRAME_death9
	SHAMBLER_FRAME_death10
	SHAMBLER_FRAME_death11
)

// Prototyped elsewhere
var sham_smash1 func()
var sham_swingr1 func()
var sham_swingl1 func()

func sham_stand1()  { Self.Frame = float32(SHAMBLER_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = sham_stand2; ai_stand() }
func sham_stand2()  { Self.Frame = float32(SHAMBLER_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = sham_stand3; ai_stand() }
func sham_stand3()  { Self.Frame = float32(SHAMBLER_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = sham_stand4; ai_stand() }
func sham_stand4()  { Self.Frame = float32(SHAMBLER_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = sham_stand5; ai_stand() }
func sham_stand5()  { Self.Frame = float32(SHAMBLER_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = sham_stand6; ai_stand() }
func sham_stand6()  { Self.Frame = float32(SHAMBLER_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = sham_stand7; ai_stand() }
func sham_stand7()  { Self.Frame = float32(SHAMBLER_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = sham_stand8; ai_stand() }
func sham_stand8()  { Self.Frame = float32(SHAMBLER_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = sham_stand9; ai_stand() }
func sham_stand9()  { Self.Frame = float32(SHAMBLER_FRAME_stand9); Self.NextThink = Time + 0.1; Self.Think = sham_stand10; ai_stand() }
func sham_stand10() { Self.Frame = float32(SHAMBLER_FRAME_stand10); Self.NextThink = Time + 0.1; Self.Think = sham_stand11; ai_stand() }
func sham_stand11() { Self.Frame = float32(SHAMBLER_FRAME_stand11); Self.NextThink = Time + 0.1; Self.Think = sham_stand12; ai_stand() }
func sham_stand12() { Self.Frame = float32(SHAMBLER_FRAME_stand12); Self.NextThink = Time + 0.1; Self.Think = sham_stand13; ai_stand() }
func sham_stand13() { Self.Frame = float32(SHAMBLER_FRAME_stand13); Self.NextThink = Time + 0.1; Self.Think = sham_stand14; ai_stand() }
func sham_stand14() { Self.Frame = float32(SHAMBLER_FRAME_stand14); Self.NextThink = Time + 0.1; Self.Think = sham_stand15; ai_stand() }
func sham_stand15() { Self.Frame = float32(SHAMBLER_FRAME_stand15); Self.NextThink = Time + 0.1; Self.Think = sham_stand16; ai_stand() }
func sham_stand16() { Self.Frame = float32(SHAMBLER_FRAME_stand16); Self.NextThink = Time + 0.1; Self.Think = sham_stand17; ai_stand() }
func sham_stand17() { Self.Frame = float32(SHAMBLER_FRAME_stand17); Self.NextThink = Time + 0.1; Self.Think = sham_stand1; ai_stand() }

func sham_walk1()  { Self.Frame = float32(SHAMBLER_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = sham_walk2; ai_walk(10) }
func sham_walk2()  { Self.Frame = float32(SHAMBLER_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = sham_walk3; ai_walk(9) }
func sham_walk3()  { Self.Frame = float32(SHAMBLER_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = sham_walk4; ai_walk(9) }
func sham_walk4()  { Self.Frame = float32(SHAMBLER_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = sham_walk5; ai_walk(5) }
func sham_walk5()  { Self.Frame = float32(SHAMBLER_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = sham_walk6; ai_walk(6) }
func sham_walk6()  { Self.Frame = float32(SHAMBLER_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = sham_walk7; ai_walk(12) }
func sham_walk7()  { Self.Frame = float32(SHAMBLER_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = sham_walk8; ai_walk(8) }
func sham_walk8()  { Self.Frame = float32(SHAMBLER_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = sham_walk9; ai_walk(3) }
func sham_walk9()  { Self.Frame = float32(SHAMBLER_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = sham_walk10; ai_walk(13) }
func sham_walk10() { Self.Frame = float32(SHAMBLER_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = sham_walk11; ai_walk(9) }
func sham_walk11() { Self.Frame = float32(SHAMBLER_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = sham_walk12; ai_walk(7) }
func sham_walk12() {
	Self.Frame = float32(SHAMBLER_FRAME_walk12)
	Self.NextThink = Time + 0.1
	Self.Think = sham_walk1
	ai_walk(7)

	if engine.Random() > 0.8 {
		engine.Sound(Self, int(CHAN_VOICE), "shambler/sidle.wav", 1, ATTN_IDLE)
	}
}

func sham_run1() { Self.Frame = float32(SHAMBLER_FRAME_run1); Self.NextThink = Time + 0.1; Self.Think = sham_run2; ai_run(20) }
func sham_run2() { Self.Frame = float32(SHAMBLER_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = sham_run3; ai_run(24) }
func sham_run3() { Self.Frame = float32(SHAMBLER_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = sham_run4; ai_run(20) }
func sham_run4() { Self.Frame = float32(SHAMBLER_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = sham_run5; ai_run(20) }
func sham_run5() { Self.Frame = float32(SHAMBLER_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = sham_run6; ai_run(24) }
func sham_run6() {
	Self.Frame = float32(SHAMBLER_FRAME_run6)
	Self.NextThink = Time + 0.1
	Self.Think = sham_run1
	ai_run(20)

	if engine.Random() > 0.8 {
		engine.Sound(Self, int(CHAN_VOICE), "shambler/sidle.wav", 1, ATTN_IDLE)
	}
}

func sham_smash1_impl() {
	Self.Frame = float32(SHAMBLER_FRAME_smash1)
	Self.NextThink = Time + 0.1
	Self.Think = sham_smash2
	engine.Sound(Self, int(CHAN_VOICE), "shambler/melee1.wav", 1, ATTN_NORM)
	ai_charge(2)
}
func sham_smash2() { Self.Frame = float32(SHAMBLER_FRAME_smash2); Self.NextThink = Time + 0.1; Self.Think = sham_smash3; ai_charge(6) }
func sham_smash3() { Self.Frame = float32(SHAMBLER_FRAME_smash3); Self.NextThink = Time + 0.1; Self.Think = sham_smash4; ai_charge(6) }
func sham_smash4() { Self.Frame = float32(SHAMBLER_FRAME_smash4); Self.NextThink = Time + 0.1; Self.Think = sham_smash5; ai_charge(5) }
func sham_smash5() { Self.Frame = float32(SHAMBLER_FRAME_smash5); Self.NextThink = Time + 0.1; Self.Think = sham_smash6; ai_charge(4) }
func sham_smash6() { Self.Frame = float32(SHAMBLER_FRAME_smash6); Self.NextThink = Time + 0.1; Self.Think = sham_smash7; ai_charge(1) }
func sham_smash7() { Self.Frame = float32(SHAMBLER_FRAME_smash7); Self.NextThink = Time + 0.1; Self.Think = sham_smash8; ai_charge(0) }
func sham_smash8() { Self.Frame = float32(SHAMBLER_FRAME_smash8); Self.NextThink = Time + 0.1; Self.Think = sham_smash9; ai_charge(0) }
func sham_smash9() { Self.Frame = float32(SHAMBLER_FRAME_smash9); Self.NextThink = Time + 0.1; Self.Think = sham_smash10; ai_charge(0) }
func sham_smash10() {
	Self.Frame = float32(SHAMBLER_FRAME_smash10)
	Self.NextThink = Time + 0.1
	Self.Think = sham_smash11

	var delta quake.Vec3
	var ldmg float32

	if Self.Enemy == nil {
		return
	}

	ai_charge(0)

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 100 {
		return
	}

	if CanDamage(Self.Enemy, Self) == 0 {
		return
	}

	ldmg = (engine.Random() + engine.Random() + engine.Random()) * 40
	T_Damage(Self.Enemy, Self, Self, ldmg)
	engine.Sound(Self, int(CHAN_VOICE), "shambler/smack.wav", 1, ATTN_NORM)

	SpawnMeatSpray(Self.Origin.Add(VForward.Mul(16)), VRight.Mul(crandom()*100))
	SpawnMeatSpray(Self.Origin.Add(VForward.Mul(16)), VRight.Mul(crandom()*100))
}

func sham_smash11() { Self.Frame = float32(SHAMBLER_FRAME_smash11); Self.NextThink = Time + 0.1; Self.Think = sham_smash12; ai_charge(5) }
func sham_smash12() { Self.Frame = float32(SHAMBLER_FRAME_smash12); Self.NextThink = Time + 0.1; Self.Think = sham_run1; ai_charge(4) }

func ShamClaw(side float32) {
	var delta quake.Vec3
	var ldmg float32

	if Self.Enemy == nil {
		return
	}

	ai_charge(10)

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 100 {
		return
	}

	ldmg = (engine.Random() + engine.Random() + engine.Random()) * 20
	T_Damage(Self.Enemy, Self, Self, ldmg)
	engine.Sound(Self, int(CHAN_VOICE), "shambler/smack.wav", 1, ATTN_NORM)

	if side != 0 {
		makevectorsfixed(Self.Angles)
		SpawnMeatSpray(Self.Origin.Add(VForward.Mul(16)), VRight.Mul(side))
	}
}

func sham_swingl1_impl() {
	Self.Frame = float32(SHAMBLER_FRAME_swingl1)
	Self.NextThink = Time + 0.1
	Self.Think = sham_swingl2
	engine.Sound(Self, int(CHAN_VOICE), "shambler/melee2.wav", 1, ATTN_NORM)
	ai_charge(5)
}

func sham_swingl2() { Self.Frame = float32(SHAMBLER_FRAME_swingl2); Self.NextThink = Time + 0.1; Self.Think = sham_swingl3; ai_charge(3) }
func sham_swingl3() { Self.Frame = float32(SHAMBLER_FRAME_swingl3); Self.NextThink = Time + 0.1; Self.Think = sham_swingl4; ai_charge(7) }
func sham_swingl4() { Self.Frame = float32(SHAMBLER_FRAME_swingl4); Self.NextThink = Time + 0.1; Self.Think = sham_swingl5; ai_charge(3) }
func sham_swingl5() { Self.Frame = float32(SHAMBLER_FRAME_swingl5); Self.NextThink = Time + 0.1; Self.Think = sham_swingl6; ai_charge(7) }
func sham_swingl6() { Self.Frame = float32(SHAMBLER_FRAME_swingl6); Self.NextThink = Time + 0.1; Self.Think = sham_swingl7; ai_charge(9) }
func sham_swingl7() { Self.Frame = float32(SHAMBLER_FRAME_swingl7); Self.NextThink = Time + 0.1; Self.Think = sham_swingl8; ai_charge(5); ShamClaw(250) }
func sham_swingl8() { Self.Frame = float32(SHAMBLER_FRAME_swingl8); Self.NextThink = Time + 0.1; Self.Think = sham_swingl9; ai_charge(4) }
func sham_swingl9() {
	Self.Frame = float32(SHAMBLER_FRAME_swingl9)
	Self.NextThink = Time + 0.1
	Self.Think = sham_run1
	ai_charge(8)

	if engine.Random() < 0.5 {
		Self.Think = sham_swingr1
	}
}

func sham_swingr1_impl() {
	Self.Frame = float32(SHAMBLER_FRAME_swingr1)
	Self.NextThink = Time + 0.1
	Self.Think = sham_swingr2
	engine.Sound(Self, int(CHAN_VOICE), "shambler/melee1.wav", 1, ATTN_NORM)
	ai_charge(1)
}

func sham_swingr2() { Self.Frame = float32(SHAMBLER_FRAME_swingr2); Self.NextThink = Time + 0.1; Self.Think = sham_swingr3; ai_charge(8) }
func sham_swingr3() { Self.Frame = float32(SHAMBLER_FRAME_swingr3); Self.NextThink = Time + 0.1; Self.Think = sham_swingr4; ai_charge(14) }
func sham_swingr4() { Self.Frame = float32(SHAMBLER_FRAME_swingr4); Self.NextThink = Time + 0.1; Self.Think = sham_swingr5; ai_charge(7) }
func sham_swingr5() { Self.Frame = float32(SHAMBLER_FRAME_swingr5); Self.NextThink = Time + 0.1; Self.Think = sham_swingr6; ai_charge(3) }
func sham_swingr6() { Self.Frame = float32(SHAMBLER_FRAME_swingr6); Self.NextThink = Time + 0.1; Self.Think = sham_swingr7; ai_charge(6) }
func sham_swingr7() { Self.Frame = float32(SHAMBLER_FRAME_swingr7); Self.NextThink = Time + 0.1; Self.Think = sham_swingr8; ai_charge(6); ShamClaw(-250) }
func sham_swingr8() { Self.Frame = float32(SHAMBLER_FRAME_swingr8); Self.NextThink = Time + 0.1; Self.Think = sham_swingr9; ai_charge(3) }
func sham_swingr9() {
	Self.Frame = float32(SHAMBLER_FRAME_swingr9)
	Self.NextThink = Time + 0.1
	Self.Think = sham_run1
	ai_charge(1)
	ai_charge(10)

	if engine.Random() < 0.5 {
		Self.Think = sham_swingl1
	}
}

func sham_melee() {
	var chance float32
	chance = engine.Random()

	if chance > 0.6 || Self.Health == 600 {
		sham_smash1_impl()
	} else if chance > 0.3 {
		sham_swingr1()
	} else {
		sham_swingl1()
	}
}

func CastLightning() {
	var org, dir quake.Vec3

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)

	ai_face()

	org = Self.Origin.Add(quake.MakeVec3(0, 0, 40))

	dir = Self.Enemy.Origin.Add(quake.MakeVec3(0, 0, 16)).Sub(org)
	dir = engine.Normalize(dir)

	engine.Traceline(org, Self.Origin.Add(dir.Mul(600)), TRUE, Self)

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_LIGHTNING1))
	engine.WriteEntity(MSG_BROADCAST, Self)
	engine.WriteCoord(MSG_BROADCAST, org[0])
	engine.WriteCoord(MSG_BROADCAST, org[1])
	engine.WriteCoord(MSG_BROADCAST, org[2])
	engine.WriteCoord(MSG_BROADCAST, TraceEndPos[0])
	engine.WriteCoord(MSG_BROADCAST, TraceEndPos[1])
	engine.WriteCoord(MSG_BROADCAST, TraceEndPos[2])

	LightningDamage(org, TraceEndPos, Self, 10)
}

func sham_magic1() {
	Self.Frame = float32(SHAMBLER_FRAME_magic1)
	Self.NextThink = Time + 0.1
	Self.Think = sham_magic2
	ai_face()
	engine.Sound(Self, int(CHAN_WEAPON), "shambler/sattck1.wav", 1, ATTN_NORM)
}

func sham_magic2() { Self.Frame = float32(SHAMBLER_FRAME_magic2); Self.NextThink = Time + 0.1; Self.Think = sham_magic3; ai_face() }
func sham_magic3() {
	Self.Frame = float32(SHAMBLER_FRAME_magic3)
	Self.NextThink = Time + 0.1
	Self.Think = sham_magic4
	ai_face()
	Self.NextThink = Self.NextThink + 0.2
	var o *quake.Entity

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
	ai_face()
	Self.Owner = engine.Spawn()
	o = Self.Owner
	engine.SetModel(o, "progs/s_light.mdl")
	engine.SetOrigin(o, Self.Origin)
	o.Angles = Self.Angles
	o.NextThink = Time + 0.7
	o.Think = SUB_Remove
}

func sham_magic4() {
	Self.Frame = float32(SHAMBLER_FRAME_magic4)
	Self.NextThink = Time + 0.1
	Self.Think = sham_magic5
	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
	Self.Owner.Frame = 1
}

func sham_magic5() {
	Self.Frame = float32(SHAMBLER_FRAME_magic5)
	Self.NextThink = Time + 0.1
	Self.Think = sham_magic6
	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
	Self.Owner.Frame = 2
}

func sham_magic6() {
	Self.Frame = float32(SHAMBLER_FRAME_magic6)
	Self.NextThink = Time + 0.1
	Self.Think = sham_magic9
	engine.Remove(Self.Owner)
	CastLightning()
	engine.Sound(Self, int(CHAN_WEAPON), "shambler/sboom.wav", 1, ATTN_NORM)
}

func sham_magic9()  { Self.Frame = float32(SHAMBLER_FRAME_magic9); Self.NextThink = Time + 0.1; Self.Think = sham_magic10; CastLightning() }
func sham_magic10() { Self.Frame = float32(SHAMBLER_FRAME_magic10); Self.NextThink = Time + 0.1; Self.Think = sham_magic11; CastLightning() }
func sham_magic11() { Self.Frame = float32(SHAMBLER_FRAME_magic11); Self.NextThink = Time + 0.1; Self.Think = sham_magic12 }
func sham_magic12() { Self.Frame = float32(SHAMBLER_FRAME_magic12); Self.NextThink = Time + 0.1; Self.Think = sham_run1 }

func sham_pain1() { Self.Frame = float32(SHAMBLER_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = sham_pain2 }
func sham_pain2() { Self.Frame = float32(SHAMBLER_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = sham_pain3 }
func sham_pain3() { Self.Frame = float32(SHAMBLER_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = sham_pain4 }
func sham_pain4() { Self.Frame = float32(SHAMBLER_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = sham_pain5 }
func sham_pain5() { Self.Frame = float32(SHAMBLER_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = sham_pain6 }
func sham_pain6() { Self.Frame = float32(SHAMBLER_FRAME_pain6); Self.NextThink = Time + 0.1; Self.Think = sham_run1 }

func sham_pain(attacker *quake.Entity, damage float32) {
	engine.Sound(Self, int(CHAN_VOICE), "shambler/shurt2.wav", 1, ATTN_NORM)

	if damage >= Self.Health && attacker.ClassName == "player" && int(attacker.Weapon) == IT_AXE {
		MsgEntity = attacker
		engine.WriteByte(MSG_ONE, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ONE, "ACH_CLOSE_SHAVE")
	}

	if Self.Health <= 0 {
		return // already dying, don't go into pain frame
	}

	if engine.Random()*400 > damage {
		return // didn't flinch
	}

	if Self.PainFinished > Time {
		return
	}

	Self.PainFinished = Time + 2
	sham_pain1()
}

func sham_death1() { Self.Frame = float32(SHAMBLER_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = sham_death2 }
func sham_death2() { Self.Frame = float32(SHAMBLER_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = sham_death3 }
func sham_death3() { Self.Frame = float32(SHAMBLER_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = sham_death4; Self.Solid = SOLID_NOT }
func sham_death4() { Self.Frame = float32(SHAMBLER_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = sham_death5 }
func sham_death5() { Self.Frame = float32(SHAMBLER_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = sham_death6 }
func sham_death6() { Self.Frame = float32(SHAMBLER_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = sham_death7 }
func sham_death7() { Self.Frame = float32(SHAMBLER_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = sham_death8 }
func sham_death8() { Self.Frame = float32(SHAMBLER_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = sham_death9 }
func sham_death9() { Self.Frame = float32(SHAMBLER_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = sham_death10 }
func sham_death10() { Self.Frame = float32(SHAMBLER_FRAME_death10); Self.NextThink = Time + 0.1; Self.Think = sham_death11 }
func sham_death11() { Self.Frame = float32(SHAMBLER_FRAME_death11); Self.NextThink = Time + 0.1; Self.Think = sham_death11 }

func sham_die() {
	if Self.Health < -60 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_shams.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "shambler/sdeath.wav", 1, ATTN_NORM)
	sham_death1()
}

func init() {
	sham_swingl1 = sham_swingl1_impl
	sham_swingr1 = sham_swingr1_impl
	sham_smash1 = sham_smash1_impl
}

func monster_shambler() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/shambler.mdl")
	engine.PrecacheModel("progs/s_light.mdl")
	engine.PrecacheModel("progs/h_shams.mdl")
	engine.PrecacheModel("progs/bolt.mdl")

	engine.PrecacheSound("shambler/sattck1.wav")
	engine.PrecacheSound("shambler/sboom.wav")
	engine.PrecacheSound("shambler/sdeath.wav")
	engine.PrecacheSound("shambler/shurt2.wav")
	engine.PrecacheSound("shambler/sidle.wav")
	engine.PrecacheSound("shambler/ssight.wav")
	engine.PrecacheSound("shambler/melee1.wav")
	engine.PrecacheSound("shambler/melee2.wav")
	engine.PrecacheSound("shambler/smack.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP
	engine.SetModel(Self, "progs/shambler.mdl")

	Self.Noise = "shambler/ssight.wav"
	Self.NetName = "$qc_shambler"
	Self.KillString = "$qc_ks_shambler"

	engine.SetSize(Self, VEC_HULL2_MIN, VEC_HULL2_MAX)
	Self.Health = 600
	Self.MaxHealth = 600

	Self.ThStand = sham_stand1
	Self.ThWalk = sham_walk1
	Self.ThRun = sham_run1
	Self.ThDie = sham_die
	Self.ThMelee = sham_melee
	Self.ThMissile = sham_magic1
	Self.ThPain = sham_pain
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_MIXED

	walkmonster_start()
}
