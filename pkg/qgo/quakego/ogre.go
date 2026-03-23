package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Ogre frames
	OGRE_FRAME_stand1 = iota
	OGRE_FRAME_stand2
	OGRE_FRAME_stand3
	OGRE_FRAME_stand4
	OGRE_FRAME_stand5
	OGRE_FRAME_stand6
	OGRE_FRAME_stand7
	OGRE_FRAME_stand8
	OGRE_FRAME_stand9

	OGRE_FRAME_walk1
	OGRE_FRAME_walk2
	OGRE_FRAME_walk3
	OGRE_FRAME_walk4
	OGRE_FRAME_walk5
	OGRE_FRAME_walk6
	OGRE_FRAME_walk7
	OGRE_FRAME_walk8
	OGRE_FRAME_walk9
	OGRE_FRAME_walk10
	OGRE_FRAME_walk11
	OGRE_FRAME_walk12
	OGRE_FRAME_walk13
	OGRE_FRAME_walk14
	OGRE_FRAME_walk15
	OGRE_FRAME_walk16

	OGRE_FRAME_run1
	OGRE_FRAME_run2
	OGRE_FRAME_run3
	OGRE_FRAME_run4
	OGRE_FRAME_run5
	OGRE_FRAME_run6
	OGRE_FRAME_run7
	OGRE_FRAME_run8

	OGRE_FRAME_swing1
	OGRE_FRAME_swing2
	OGRE_FRAME_swing3
	OGRE_FRAME_swing4
	OGRE_FRAME_swing5
	OGRE_FRAME_swing6
	OGRE_FRAME_swing7
	OGRE_FRAME_swing8
	OGRE_FRAME_swing9
	OGRE_FRAME_swing10
	OGRE_FRAME_swing11
	OGRE_FRAME_swing12
	OGRE_FRAME_swing13
	OGRE_FRAME_swing14

	OGRE_FRAME_smash1
	OGRE_FRAME_smash2
	OGRE_FRAME_smash3
	OGRE_FRAME_smash4
	OGRE_FRAME_smash5
	OGRE_FRAME_smash6
	OGRE_FRAME_smash7
	OGRE_FRAME_smash8
	OGRE_FRAME_smash9
	OGRE_FRAME_smash10
	OGRE_FRAME_smash11
	OGRE_FRAME_smash12
	OGRE_FRAME_smash13
	OGRE_FRAME_smash14

	OGRE_FRAME_shoot1
	OGRE_FRAME_shoot2
	OGRE_FRAME_shoot3
	OGRE_FRAME_shoot4
	OGRE_FRAME_shoot5
	OGRE_FRAME_shoot6

	OGRE_FRAME_pain1
	OGRE_FRAME_pain2
	OGRE_FRAME_pain3
	OGRE_FRAME_pain4
	OGRE_FRAME_pain5

	OGRE_FRAME_painb1
	OGRE_FRAME_painb2
	OGRE_FRAME_painb3

	OGRE_FRAME_painc1
	OGRE_FRAME_painc2
	OGRE_FRAME_painc3
	OGRE_FRAME_painc4
	OGRE_FRAME_painc5
	OGRE_FRAME_painc6

	OGRE_FRAME_paind1
	OGRE_FRAME_paind2
	OGRE_FRAME_paind3
	OGRE_FRAME_paind4
	OGRE_FRAME_paind5
	OGRE_FRAME_paind6
	OGRE_FRAME_paind7
	OGRE_FRAME_paind8
	OGRE_FRAME_paind9
	OGRE_FRAME_paind10
	OGRE_FRAME_paind11
	OGRE_FRAME_paind12
	OGRE_FRAME_paind13
	OGRE_FRAME_paind14
	OGRE_FRAME_paind15
	OGRE_FRAME_paind16

	OGRE_FRAME_paine1
	OGRE_FRAME_paine2
	OGRE_FRAME_paine3
	OGRE_FRAME_paine4
	OGRE_FRAME_paine5
	OGRE_FRAME_paine6
	OGRE_FRAME_paine7
	OGRE_FRAME_paine8
	OGRE_FRAME_paine9
	OGRE_FRAME_paine10
	OGRE_FRAME_paine11
	OGRE_FRAME_paine12
	OGRE_FRAME_paine13
	OGRE_FRAME_paine14
	OGRE_FRAME_paine15

	OGRE_FRAME_death1
	OGRE_FRAME_death2
	OGRE_FRAME_death3
	OGRE_FRAME_death4
	OGRE_FRAME_death5
	OGRE_FRAME_death6
	OGRE_FRAME_death7
	OGRE_FRAME_death8
	OGRE_FRAME_death9
	OGRE_FRAME_death10
	OGRE_FRAME_death11
	OGRE_FRAME_death12
	OGRE_FRAME_death13
	OGRE_FRAME_death14

	OGRE_FRAME_bdeath1
	OGRE_FRAME_bdeath2
	OGRE_FRAME_bdeath3
	OGRE_FRAME_bdeath4
	OGRE_FRAME_bdeath5
	OGRE_FRAME_bdeath6
	OGRE_FRAME_bdeath7
	OGRE_FRAME_bdeath8
	OGRE_FRAME_bdeath9
	OGRE_FRAME_bdeath10

	OGRE_FRAME_pull1
	OGRE_FRAME_pull2
	OGRE_FRAME_pull3
	OGRE_FRAME_pull4
	OGRE_FRAME_pull5
	OGRE_FRAME_pull6
	OGRE_FRAME_pull7
	OGRE_FRAME_pull8
	OGRE_FRAME_pull9
	OGRE_FRAME_pull10
	OGRE_FRAME_pull11
)

func OgreGrenadeExplode() {
	T_RadiusDamage(Self, Self.Owner, 40, World)
	engine.Sound(Self, int(CHAN_VOICE), "weapons/r_exp3.wav", 1, ATTN_NORM)

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_EXPLOSION))
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])

	Self.Velocity = quake.MakeVec3(0, 0, 0)
	Self.Touch = SUB_Null
	engine.SetModel(Self, "progs/s_explod.spr")
	Self.Solid = SOLID_NOT
	s_explode1()
}

func OgreGrenadeTouch() {
	if Other == Self.Owner {
		return
	}

	if Other.TakeDamage == float32(DAMAGE_AIM) {
		OgreGrenadeExplode()
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "weapons/bounce.wav", 1, ATTN_NORM)

	if Self.Velocity == quake.MakeVec3(0, 0, 0) {
		Self.AVelocity = quake.MakeVec3(0, 0, 0)
	}
}

func OgreFireGrenade() {
	var missile *quake.Entity

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
	engine.Sound(Self, int(CHAN_WEAPON), "weapons/grenade.wav", 1, ATTN_NORM)

	missile = engine.Spawn()
	missile.ClassName = "ogre_grenade"
	missile.Owner = Self
	missile.MoveType = MOVETYPE_BOUNCE
	missile.Solid = SOLID_BBOX

	makevectorsfixed(Self.Angles)

	missile.Velocity = engine.Normalize(Self.Enemy.Origin.Sub(Self.Origin))
	missile.Velocity = missile.Velocity.Mul(600)
	missile.Velocity[2] = 200

	missile.AVelocity = quake.MakeVec3(300, 300, 300)
	missile.Angles = engine.VectoAngles(missile.Velocity)
	missile.Touch = OgreGrenadeTouch

	if engine.Cvar("pr_checkextension") != 0 {
		if engine.CheckExtension("EX_EXTENDED_EF") != 0 {
			missile.Effects = float32(int(missile.Effects) | EF_CANDLELIGHT)
		}
	}

	missile.NextThink = Time + 2.5
	missile.Think = OgreGrenadeExplode

	engine.SetModel(missile, "progs/grenade.mdl")
	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(missile, Self.Origin)
}

func ogre_chainsaw(side float32) {
	var delta quake.Vec3
	var ldmg float32

	if Self.Enemy == nil {
		return
	}

	if CanDamage(Self.Enemy, Self) == 0 {
		return
	}

	ai_charge(10)

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 100 {
		return
	}

	ldmg = (engine.Random() + engine.Random() + engine.Random()) * 4
	T_Damage(Self.Enemy, Self, Self, ldmg)

	if side != 0 {
		makevectorsfixed(Self.Angles)
		if side == 1 {
			SpawnMeatSpray(Self.Origin.Add(VForward.Mul(16)), VRight.Mul(crandom()*100))
		} else {
			SpawnMeatSpray(Self.Origin.Add(VForward.Mul(16)), VRight.Mul(side))
		}
	}
}

func ogre_stand1() { Self.Frame = float32(OGRE_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = ogre_stand2; ai_stand() }
func ogre_stand2() { Self.Frame = float32(OGRE_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = ogre_stand3; ai_stand() }
func ogre_stand3() { Self.Frame = float32(OGRE_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = ogre_stand4; ai_stand() }
func ogre_stand4() { Self.Frame = float32(OGRE_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = ogre_stand5; ai_stand() }
func ogre_stand5() {
	Self.Frame = float32(OGRE_FRAME_stand5)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_stand6
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "ogre/ogidle.wav", 1, ATTN_IDLE)
	}
	ai_stand()
}
func ogre_stand6() { Self.Frame = float32(OGRE_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = ogre_stand7; ai_stand() }
func ogre_stand7() { Self.Frame = float32(OGRE_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = ogre_stand8; ai_stand() }
func ogre_stand8() { Self.Frame = float32(OGRE_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = ogre_stand9; ai_stand() }
func ogre_stand9() { Self.Frame = float32(OGRE_FRAME_stand9); Self.NextThink = Time + 0.1; Self.Think = ogre_stand1; ai_stand() }

func ogre_walk1() { Self.Frame = float32(OGRE_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = ogre_walk2; ai_walk(3) }
func ogre_walk2() { Self.Frame = float32(OGRE_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = ogre_walk3; ai_walk(2) }
func ogre_walk3() {
	Self.Frame = float32(OGRE_FRAME_walk3)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_walk4
	ai_walk(2)
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "ogre/ogidle.wav", 1, ATTN_IDLE)
	}
}
func ogre_walk4() { Self.Frame = float32(OGRE_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = ogre_walk5; ai_walk(2) }
func ogre_walk5() { Self.Frame = float32(OGRE_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = ogre_walk6; ai_walk(2) }
func ogre_walk6() {
	Self.Frame = float32(OGRE_FRAME_walk6)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_walk7
	ai_walk(5)
	if engine.Random() < 0.1 {
		engine.Sound(Self, int(CHAN_VOICE), "ogre/ogdrag.wav", 1, ATTN_IDLE)
	}
}
func ogre_walk7()  { Self.Frame = float32(OGRE_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = ogre_walk8; ai_walk(3) }
func ogre_walk8()  { Self.Frame = float32(OGRE_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = ogre_walk9; ai_walk(2) }
func ogre_walk9()  { Self.Frame = float32(OGRE_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = ogre_walk10; ai_walk(3) }
func ogre_walk10() { Self.Frame = float32(OGRE_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = ogre_walk11; ai_walk(1) }
func ogre_walk11() { Self.Frame = float32(OGRE_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = ogre_walk12; ai_walk(2) }
func ogre_walk12() { Self.Frame = float32(OGRE_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = ogre_walk13; ai_walk(3) }
func ogre_walk13() { Self.Frame = float32(OGRE_FRAME_walk13); Self.NextThink = Time + 0.1; Self.Think = ogre_walk14; ai_walk(3) }
func ogre_walk14() { Self.Frame = float32(OGRE_FRAME_walk14); Self.NextThink = Time + 0.1; Self.Think = ogre_walk15; ai_walk(3) }
func ogre_walk15() { Self.Frame = float32(OGRE_FRAME_walk15); Self.NextThink = Time + 0.1; Self.Think = ogre_walk16; ai_walk(3) }
func ogre_walk16() { Self.Frame = float32(OGRE_FRAME_walk16); Self.NextThink = Time + 0.1; Self.Think = ogre_walk1; ai_walk(4) }

func ogre_run1() {
	Self.Frame = float32(OGRE_FRAME_run1)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_run2
	ai_run(9)
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "ogre/ogidle2.wav", 1, ATTN_IDLE)
	}
}
func ogre_run2() { Self.Frame = float32(OGRE_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = ogre_run3; ai_run(12) }
func ogre_run3() { Self.Frame = float32(OGRE_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = ogre_run4; ai_run(8) }
func ogre_run4() { Self.Frame = float32(OGRE_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = ogre_run5; ai_run(22) }
func ogre_run5() { Self.Frame = float32(OGRE_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = ogre_run6; ai_run(16) }
func ogre_run6() { Self.Frame = float32(OGRE_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = ogre_run7; ai_run(4) }
func ogre_run7() { Self.Frame = float32(OGRE_FRAME_run7); Self.NextThink = Time + 0.1; Self.Think = ogre_run8; ai_run(13) }
func ogre_run8() { Self.Frame = float32(OGRE_FRAME_run8); Self.NextThink = Time + 0.1; Self.Think = ogre_run1; ai_run(24) }

func ogre_swing1() {
	Self.Frame = float32(OGRE_FRAME_swing1)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing2
	ai_charge(11)
	engine.Sound(Self, int(CHAN_WEAPON), "ogre/ogsawatk.wav", 1, ATTN_NORM)
}
func ogre_swing2() { Self.Frame = float32(OGRE_FRAME_swing2); Self.NextThink = Time + 0.1; Self.Think = ogre_swing3; ai_charge(1) }
func ogre_swing3() { Self.Frame = float32(OGRE_FRAME_swing3); Self.NextThink = Time + 0.1; Self.Think = ogre_swing4; ai_charge(4) }
func ogre_swing4() { Self.Frame = float32(OGRE_FRAME_swing4); Self.NextThink = Time + 0.1; Self.Think = ogre_swing5; ai_charge(13) }
func ogre_swing5() {
	Self.Frame = float32(OGRE_FRAME_swing5)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing6
	ai_charge(9)
	ogre_chainsaw(0)
	Self.Angles[1] = Self.Angles[1] + engine.Random()*25
}
func ogre_swing6() {
	Self.Frame = float32(OGRE_FRAME_swing6)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing7
	ogre_chainsaw(200)
	Self.Angles[1] = Self.Angles[1] + engine.Random()*25
}
func ogre_swing7() {
	Self.Frame = float32(OGRE_FRAME_swing7)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing8
	ogre_chainsaw(0)
	Self.Angles[1] = Self.Angles[1] + engine.Random()*25
}
func ogre_swing8() {
	Self.Frame = float32(OGRE_FRAME_swing8)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing9
	ogre_chainsaw(0)
	Self.Angles[1] = Self.Angles[1] + engine.Random()*25
}
func ogre_swing9() {
	Self.Frame = float32(OGRE_FRAME_swing9)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing10
	ogre_chainsaw(0)
	Self.Angles[1] = Self.Angles[1] + engine.Random()*25
}
func ogre_swing10() {
	Self.Frame = float32(OGRE_FRAME_swing10)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing11
	ogre_chainsaw(-200)
	Self.Angles[1] = Self.Angles[1] + engine.Random()*25
}
func ogre_swing11() {
	Self.Frame = float32(OGRE_FRAME_swing11)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_swing12
	ogre_chainsaw(0)
	Self.Angles[1] = Self.Angles[1] + engine.Random()*25
}
func ogre_swing12() { Self.Frame = float32(OGRE_FRAME_swing12); Self.NextThink = Time + 0.1; Self.Think = ogre_swing13; ai_charge(3) }
func ogre_swing13() { Self.Frame = float32(OGRE_FRAME_swing13); Self.NextThink = Time + 0.1; Self.Think = ogre_swing14; ai_charge(8) }
func ogre_swing14() { Self.Frame = float32(OGRE_FRAME_swing14); Self.NextThink = Time + 0.1; Self.Think = ogre_run1; ai_charge(9) }

func ogre_smash1_impl() {
	Self.Frame = float32(OGRE_FRAME_smash1)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_smash2
	ai_charge(6)
	engine.Sound(Self, int(CHAN_WEAPON), "ogre/ogsawatk.wav", 1, ATTN_NORM)
}
func ogre_smash2() { Self.Frame = float32(OGRE_FRAME_smash2); Self.NextThink = Time + 0.1; Self.Think = ogre_smash3; ai_charge(0) }
func ogre_smash3() { Self.Frame = float32(OGRE_FRAME_smash3); Self.NextThink = Time + 0.1; Self.Think = ogre_smash4; ai_charge(0) }
func ogre_smash4() { Self.Frame = float32(OGRE_FRAME_smash4); Self.NextThink = Time + 0.1; Self.Think = ogre_smash5; ai_charge(1) }
func ogre_smash5() { Self.Frame = float32(OGRE_FRAME_smash5); Self.NextThink = Time + 0.1; Self.Think = ogre_smash6; ai_charge(4) }
func ogre_smash6() { Self.Frame = float32(OGRE_FRAME_smash6); Self.NextThink = Time + 0.1; Self.Think = ogre_smash7; ai_charge(4); ogre_chainsaw(0) }
func ogre_smash7() { Self.Frame = float32(OGRE_FRAME_smash7); Self.NextThink = Time + 0.1; Self.Think = ogre_smash8; ai_charge(4); ogre_chainsaw(0) }
func ogre_smash8() { Self.Frame = float32(OGRE_FRAME_smash8); Self.NextThink = Time + 0.1; Self.Think = ogre_smash9; ai_charge(10); ogre_chainsaw(0) }
func ogre_smash9() { Self.Frame = float32(OGRE_FRAME_smash9); Self.NextThink = Time + 0.1; Self.Think = ogre_smash10; ai_charge(13); ogre_chainsaw(0) }
func ogre_smash10() { Self.Frame = float32(OGRE_FRAME_smash10); Self.NextThink = Time + 0.1; Self.Think = ogre_smash11; ogre_chainsaw(1) }
func ogre_smash11() {
	Self.Frame = float32(OGRE_FRAME_smash11)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_smash12
	ai_charge(2)
	ogre_chainsaw(0)
	Self.NextThink = Self.NextThink + engine.Random()*0.2
}
func ogre_smash12() { Self.Frame = float32(OGRE_FRAME_smash12); Self.NextThink = Time + 0.1; Self.Think = ogre_smash13; ai_charge(0) }
func ogre_smash13() { Self.Frame = float32(OGRE_FRAME_smash13); Self.NextThink = Time + 0.1; Self.Think = ogre_smash14; ai_charge(4) }
func ogre_smash14() { Self.Frame = float32(OGRE_FRAME_smash14); Self.NextThink = Time + 0.1; Self.Think = ogre_run1; ai_charge(12) }

func ogre_nail1() { Self.Frame = float32(OGRE_FRAME_shoot1); Self.NextThink = Time + 0.1; Self.Think = ogre_nail2; ai_face() }
func ogre_nail2() { Self.Frame = float32(OGRE_FRAME_shoot2); Self.NextThink = Time + 0.1; Self.Think = ogre_nail3; ai_face() }
func ogre_nail3() { Self.Frame = float32(OGRE_FRAME_shoot2); Self.NextThink = Time + 0.1; Self.Think = ogre_nail4; ai_face() }
func ogre_nail4() { Self.Frame = float32(OGRE_FRAME_shoot3); Self.NextThink = Time + 0.1; Self.Think = ogre_nail5; ai_face(); OgreFireGrenade() }
func ogre_nail5() { Self.Frame = float32(OGRE_FRAME_shoot4); Self.NextThink = Time + 0.1; Self.Think = ogre_nail6; ai_face() }
func ogre_nail6() { Self.Frame = float32(OGRE_FRAME_shoot5); Self.NextThink = Time + 0.1; Self.Think = ogre_nail7; ai_face() }
func ogre_nail7() { Self.Frame = float32(OGRE_FRAME_shoot6); Self.NextThink = Time + 0.1; Self.Think = ogre_run1; ai_face() }

func ogre_pain1() { Self.Frame = float32(OGRE_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = ogre_pain2 }
func ogre_pain2() { Self.Frame = float32(OGRE_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = ogre_pain3 }
func ogre_pain3() { Self.Frame = float32(OGRE_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = ogre_pain4 }
func ogre_pain4() { Self.Frame = float32(OGRE_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = ogre_pain5 }
func ogre_pain5() { Self.Frame = float32(OGRE_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = ogre_run1 }

func ogre_painb1() { Self.Frame = float32(OGRE_FRAME_painb1); Self.NextThink = Time + 0.1; Self.Think = ogre_painb2 }
func ogre_painb2() { Self.Frame = float32(OGRE_FRAME_painb2); Self.NextThink = Time + 0.1; Self.Think = ogre_painb3 }
func ogre_painb3() { Self.Frame = float32(OGRE_FRAME_painb3); Self.NextThink = Time + 0.1; Self.Think = ogre_run1 }

func ogre_painc1() { Self.Frame = float32(OGRE_FRAME_painc1); Self.NextThink = Time + 0.1; Self.Think = ogre_painc2 }
func ogre_painc2() { Self.Frame = float32(OGRE_FRAME_painc2); Self.NextThink = Time + 0.1; Self.Think = ogre_painc3 }
func ogre_painc3() { Self.Frame = float32(OGRE_FRAME_painc3); Self.NextThink = Time + 0.1; Self.Think = ogre_painc4 }
func ogre_painc4() { Self.Frame = float32(OGRE_FRAME_painc4); Self.NextThink = Time + 0.1; Self.Think = ogre_painc5 }
func ogre_painc5() { Self.Frame = float32(OGRE_FRAME_painc5); Self.NextThink = Time + 0.1; Self.Think = ogre_painc6 }
func ogre_painc6() { Self.Frame = float32(OGRE_FRAME_painc6); Self.NextThink = Time + 0.1; Self.Think = ogre_run1 }

func ogre_paind1()  { Self.Frame = float32(OGRE_FRAME_paind1); Self.NextThink = Time + 0.1; Self.Think = ogre_paind2 }
func ogre_paind2()  { Self.Frame = float32(OGRE_FRAME_paind2); Self.NextThink = Time + 0.1; Self.Think = ogre_paind3; ai_pain(10) }
func ogre_paind3()  { Self.Frame = float32(OGRE_FRAME_paind3); Self.NextThink = Time + 0.1; Self.Think = ogre_paind4; ai_pain(9) }
func ogre_paind4()  { Self.Frame = float32(OGRE_FRAME_paind4); Self.NextThink = Time + 0.1; Self.Think = ogre_paind5; ai_pain(4) }
func ogre_paind5()  { Self.Frame = float32(OGRE_FRAME_paind5); Self.NextThink = Time + 0.1; Self.Think = ogre_paind6 }
func ogre_paind6()  { Self.Frame = float32(OGRE_FRAME_paind6); Self.NextThink = Time + 0.1; Self.Think = ogre_paind7 }
func ogre_paind7()  { Self.Frame = float32(OGRE_FRAME_paind7); Self.NextThink = Time + 0.1; Self.Think = ogre_paind8 }
func ogre_paind8()  { Self.Frame = float32(OGRE_FRAME_paind8); Self.NextThink = Time + 0.1; Self.Think = ogre_paind9 }
func ogre_paind9()  { Self.Frame = float32(OGRE_FRAME_paind9); Self.NextThink = Time + 0.1; Self.Think = ogre_paind10 }
func ogre_paind10() { Self.Frame = float32(OGRE_FRAME_paind10); Self.NextThink = Time + 0.1; Self.Think = ogre_paind11 }
func ogre_paind11() { Self.Frame = float32(OGRE_FRAME_paind11); Self.NextThink = Time + 0.1; Self.Think = ogre_paind12 }
func ogre_paind12() { Self.Frame = float32(OGRE_FRAME_paind12); Self.NextThink = Time + 0.1; Self.Think = ogre_paind13 }
func ogre_paind13() { Self.Frame = float32(OGRE_FRAME_paind13); Self.NextThink = Time + 0.1; Self.Think = ogre_paind14 }
func ogre_paind14() { Self.Frame = float32(OGRE_FRAME_paind14); Self.NextThink = Time + 0.1; Self.Think = ogre_paind15 }
func ogre_paind15() { Self.Frame = float32(OGRE_FRAME_paind15); Self.NextThink = Time + 0.1; Self.Think = ogre_paind16 }
func ogre_paind16() { Self.Frame = float32(OGRE_FRAME_paind16); Self.NextThink = Time + 0.1; Self.Think = ogre_run1 }

func ogre_paine1()  { Self.Frame = float32(OGRE_FRAME_paine1); Self.NextThink = Time + 0.1; Self.Think = ogre_paine2 }
func ogre_paine2()  { Self.Frame = float32(OGRE_FRAME_paine2); Self.NextThink = Time + 0.1; Self.Think = ogre_paine3; ai_pain(10) }
func ogre_paine3()  { Self.Frame = float32(OGRE_FRAME_paine3); Self.NextThink = Time + 0.1; Self.Think = ogre_paine4; ai_pain(9) }
func ogre_paine4()  { Self.Frame = float32(OGRE_FRAME_paine4); Self.NextThink = Time + 0.1; Self.Think = ogre_paine5; ai_pain(4) }
func ogre_paine5()  { Self.Frame = float32(OGRE_FRAME_paine5); Self.NextThink = Time + 0.1; Self.Think = ogre_paine6 }
func ogre_paine6()  { Self.Frame = float32(OGRE_FRAME_paine6); Self.NextThink = Time + 0.1; Self.Think = ogre_paine7 }
func ogre_paine7()  { Self.Frame = float32(OGRE_FRAME_paine7); Self.NextThink = Time + 0.1; Self.Think = ogre_paine8 }
func ogre_paine8()  { Self.Frame = float32(OGRE_FRAME_paine8); Self.NextThink = Time + 0.1; Self.Think = ogre_paine9 }
func ogre_paine9()  { Self.Frame = float32(OGRE_FRAME_paine9); Self.NextThink = Time + 0.1; Self.Think = ogre_paine10 }
func ogre_paine10() { Self.Frame = float32(OGRE_FRAME_paine10); Self.NextThink = Time + 0.1; Self.Think = ogre_paine11 }
func ogre_paine11() { Self.Frame = float32(OGRE_FRAME_paine11); Self.NextThink = Time + 0.1; Self.Think = ogre_paine12 }
func ogre_paine12() { Self.Frame = float32(OGRE_FRAME_paine12); Self.NextThink = Time + 0.1; Self.Think = ogre_paine13 }
func ogre_paine13() { Self.Frame = float32(OGRE_FRAME_paine13); Self.NextThink = Time + 0.1; Self.Think = ogre_paine14 }
func ogre_paine14() { Self.Frame = float32(OGRE_FRAME_paine14); Self.NextThink = Time + 0.1; Self.Think = ogre_paine15 }
func ogre_paine15() { Self.Frame = float32(OGRE_FRAME_paine15); Self.NextThink = Time + 0.1; Self.Think = ogre_run1 }

func ogre_pain(attacker *quake.Entity, damage float32) {
	var r float32

	if Self.PainFinished > Time {
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "ogre/ogpain1.wav", 1, ATTN_NORM)

	r = engine.Random()

	if r < 0.25 {
		ogre_pain1()
		Self.PainFinished = Time + 1
	} else if r < 0.5 {
		ogre_painb1()
		Self.PainFinished = Time + 1
	} else if r < 0.75 {
		ogre_painc1()
		Self.PainFinished = Time + 1
	} else if r < 0.88 {
		ogre_paind1()
		Self.PainFinished = Time + 2
	} else {
		ogre_paine1()
		Self.PainFinished = Time + 2
	}
}

func ogre_die1() { Self.Frame = float32(OGRE_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = ogre_die2 }
func ogre_die2() { Self.Frame = float32(OGRE_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = ogre_die3 }
func ogre_die3() {
	Self.Frame = float32(OGRE_FRAME_death3)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_die4
	Self.Solid = SOLID_NOT
	Self.AmmoRockets = 2
	DropBackpack()
}
func ogre_die4()  { Self.Frame = float32(OGRE_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = ogre_die5 }
func ogre_die5()  { Self.Frame = float32(OGRE_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = ogre_die6 }
func ogre_die6()  { Self.Frame = float32(OGRE_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = ogre_die7 }
func ogre_die7()  { Self.Frame = float32(OGRE_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = ogre_die8 }
func ogre_die8()  { Self.Frame = float32(OGRE_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = ogre_die9 }
func ogre_die9()  { Self.Frame = float32(OGRE_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = ogre_die10 }
func ogre_die10() { Self.Frame = float32(OGRE_FRAME_death10); Self.NextThink = Time + 0.1; Self.Think = ogre_die11 }
func ogre_die11() { Self.Frame = float32(OGRE_FRAME_death11); Self.NextThink = Time + 0.1; Self.Think = ogre_die12 }
func ogre_die12() { Self.Frame = float32(OGRE_FRAME_death12); Self.NextThink = Time + 0.1; Self.Think = ogre_die13 }
func ogre_die13() { Self.Frame = float32(OGRE_FRAME_death13); Self.NextThink = Time + 0.1; Self.Think = ogre_die14 }
func ogre_die14() { Self.Frame = float32(OGRE_FRAME_death14); Self.NextThink = Time + 0.1; Self.Think = ogre_die14 }

func ogre_bdie1() { Self.Frame = float32(OGRE_FRAME_bdeath1); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie2 }
func ogre_bdie2() { Self.Frame = float32(OGRE_FRAME_bdeath2); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie3; ai_forward(5) }
func ogre_bdie3() {
	Self.Frame = float32(OGRE_FRAME_bdeath3)
	Self.NextThink = Time + 0.1
	Self.Think = ogre_bdie4
	Self.Solid = SOLID_NOT
	Self.AmmoRockets = 2
	DropBackpack()
}
func ogre_bdie4()  { Self.Frame = float32(OGRE_FRAME_bdeath4); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie5; ai_forward(1) }
func ogre_bdie5()  { Self.Frame = float32(OGRE_FRAME_bdeath5); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie6; ai_forward(3) }
func ogre_bdie6()  { Self.Frame = float32(OGRE_FRAME_bdeath6); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie7; ai_forward(7) }
func ogre_bdie7()  { Self.Frame = float32(OGRE_FRAME_bdeath7); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie8; ai_forward(25) }
func ogre_bdie8()  { Self.Frame = float32(OGRE_FRAME_bdeath8); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie9 }
func ogre_bdie9()  { Self.Frame = float32(OGRE_FRAME_bdeath9); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie10 }
func ogre_bdie10() { Self.Frame = float32(OGRE_FRAME_bdeath10); Self.NextThink = Time + 0.1; Self.Think = ogre_bdie10 }

func ogre_die() {
	if Self.Health < -80 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_ogre.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "ogre/ogdth.wav", 1, ATTN_NORM)

	if engine.Random() < 0.5 {
		ogre_die1()
	} else {
		ogre_bdie1()
	}
}

func ogre_melee_impl() {
	if engine.Random() > 0.5 {
		ogre_smash1_impl()
	} else {
		ogre_swing1()
	}
}

func init() {
	ogre_smash1 = ogre_smash1_impl
}

func monster_ogre() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/ogre.mdl")
	engine.PrecacheModel("progs/h_ogre.mdl")
	engine.PrecacheModel("progs/grenade.mdl")

	engine.PrecacheSound("ogre/ogdrag.wav")
	engine.PrecacheSound("ogre/ogdth.wav")
	engine.PrecacheSound("ogre/ogidle.wav")
	engine.PrecacheSound("ogre/ogidle2.wav")
	engine.PrecacheSound("ogre/ogpain1.wav")
	engine.PrecacheSound("ogre/ogsawatk.wav")
	engine.PrecacheSound("ogre/ogwake.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/ogre.mdl")

	Self.Noise = "ogre/ogwake.wav"
	Self.NetName = "$qc_ogre"
	Self.KillString = "$qc_ks_ogre"

	engine.SetSize(Self, VEC_HULL2_MIN, VEC_HULL2_MAX)
	Self.Health = 200
	Self.MaxHealth = 200

	Self.ThStand = ogre_stand1
	Self.ThWalk = ogre_walk1
	Self.ThRun = ogre_run1
	Self.ThDie = ogre_die
	Self.ThMelee = ogre_melee_impl
	Self.ThMissile = ogre_nail1
	Self.ThPain = ogre_pain
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_MIXED

	walkmonster_start()
}

func monster_ogre_marksman() {
	monster_ogre()
}
