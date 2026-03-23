package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Chthon (Boss 1) frames
	BOSS1_FRAME_rise1 = iota
	BOSS1_FRAME_rise2
	BOSS1_FRAME_rise3
	BOSS1_FRAME_rise4
	BOSS1_FRAME_rise5
	BOSS1_FRAME_rise6
	BOSS1_FRAME_rise7
	BOSS1_FRAME_rise8
	BOSS1_FRAME_rise9
	BOSS1_FRAME_rise10
	BOSS1_FRAME_rise11
	BOSS1_FRAME_rise12
	BOSS1_FRAME_rise13
	BOSS1_FRAME_rise14
	BOSS1_FRAME_rise15
	BOSS1_FRAME_rise16
	BOSS1_FRAME_rise17

	BOSS1_FRAME_walk1
	BOSS1_FRAME_walk2
	BOSS1_FRAME_walk3
	BOSS1_FRAME_walk4
	BOSS1_FRAME_walk5
	BOSS1_FRAME_walk6
	BOSS1_FRAME_walk7
	BOSS1_FRAME_walk8
	BOSS1_FRAME_walk9
	BOSS1_FRAME_walk10
	BOSS1_FRAME_walk11
	BOSS1_FRAME_walk12
	BOSS1_FRAME_walk13
	BOSS1_FRAME_walk14
	BOSS1_FRAME_walk15
	BOSS1_FRAME_walk16
	BOSS1_FRAME_walk17
	BOSS1_FRAME_walk18
	BOSS1_FRAME_walk19
	BOSS1_FRAME_walk20
	BOSS1_FRAME_walk21
	BOSS1_FRAME_walk22
	BOSS1_FRAME_walk23
	BOSS1_FRAME_walk24
	BOSS1_FRAME_walk25
	BOSS1_FRAME_walk26
	BOSS1_FRAME_walk27
	BOSS1_FRAME_walk28
	BOSS1_FRAME_walk29
	BOSS1_FRAME_walk30
	BOSS1_FRAME_walk31

	BOSS1_FRAME_death1
	BOSS1_FRAME_death2
	BOSS1_FRAME_death3
	BOSS1_FRAME_death4
	BOSS1_FRAME_death5
	BOSS1_FRAME_death6
	BOSS1_FRAME_death7
	BOSS1_FRAME_death8
	BOSS1_FRAME_death9

	BOSS1_FRAME_attack1
	BOSS1_FRAME_attack2
	BOSS1_FRAME_attack3
	BOSS1_FRAME_attack4
	BOSS1_FRAME_attack5
	BOSS1_FRAME_attack6
	BOSS1_FRAME_attack7
	BOSS1_FRAME_attack8
	BOSS1_FRAME_attack9
	BOSS1_FRAME_attack10
	BOSS1_FRAME_attack11
	BOSS1_FRAME_attack12
	BOSS1_FRAME_attack13
	BOSS1_FRAME_attack14
	BOSS1_FRAME_attack15
	BOSS1_FRAME_attack16
	BOSS1_FRAME_attack17
	BOSS1_FRAME_attack18
	BOSS1_FRAME_attack19
	BOSS1_FRAME_attack20
	BOSS1_FRAME_attack21
	BOSS1_FRAME_attack22
	BOSS1_FRAME_attack23

	BOSS1_FRAME_shocka1
	BOSS1_FRAME_shocka2
	BOSS1_FRAME_shocka3
	BOSS1_FRAME_shocka4
	BOSS1_FRAME_shocka5
	BOSS1_FRAME_shocka6
	BOSS1_FRAME_shocka7
	BOSS1_FRAME_shocka8
	BOSS1_FRAME_shocka9
	BOSS1_FRAME_shocka10

	BOSS1_FRAME_shockb1
	BOSS1_FRAME_shockb2
	BOSS1_FRAME_shockb3
	BOSS1_FRAME_shockb4
	BOSS1_FRAME_shockb5
	BOSS1_FRAME_shockb6

	BOSS1_FRAME_shockc1
	BOSS1_FRAME_shockc2
	BOSS1_FRAME_shockc3
	BOSS1_FRAME_shockc4
	BOSS1_FRAME_shockc5
	BOSS1_FRAME_shockc6
	BOSS1_FRAME_shockc7
	BOSS1_FRAME_shockc8
	BOSS1_FRAME_shockc9
	BOSS1_FRAME_shockc10
)

var (
	le1            *quake.Entity
	le2            *quake.Entity
	lightning_end  float32
)

// Prototyped elsewhere
var boss_missile1 func()
var boss_idle1 func()

func boss_face() {
	var target *quake.Entity
	var i float32 = 0

	if Self.Enemy.Health <= 0 || engine.Random() < 0.02 {
		target = Self.Enemy

		for i < 4 {
			target = engine.Find(target, "classname", "player")
			if target != nil && target.Health > 0 {
				Self.Enemy = target
				break
			}
			i = i + 1
		}
	}

	ai_face()
}

func boss_rise1() {
	Self.Frame = float32(BOSS1_FRAME_rise1)
	Self.NextThink = Time + 0.1
	Self.Think = boss_rise2
	engine.Sound(Self, int(CHAN_WEAPON), "boss1/out1.wav", 1, ATTN_NORM)
}

func boss_rise2() {
	Self.Frame = float32(BOSS1_FRAME_rise2)
	Self.NextThink = Time + 0.1
	Self.Think = boss_rise3
	engine.Sound(Self, int(CHAN_VOICE), "boss1/sight1.wav", 1, ATTN_NORM)
}

func boss_rise3()  { Self.Frame = float32(BOSS1_FRAME_rise3); Self.NextThink = Time + 0.1; Self.Think = boss_rise4 }
func boss_rise4()  { Self.Frame = float32(BOSS1_FRAME_rise4); Self.NextThink = Time + 0.1; Self.Think = boss_rise5 }
func boss_rise5()  { Self.Frame = float32(BOSS1_FRAME_rise5); Self.NextThink = Time + 0.1; Self.Think = boss_rise6 }
func boss_rise6()  { Self.Frame = float32(BOSS1_FRAME_rise6); Self.NextThink = Time + 0.1; Self.Think = boss_rise7 }
func boss_rise7()  { Self.Frame = float32(BOSS1_FRAME_rise7); Self.NextThink = Time + 0.1; Self.Think = boss_rise8 }
func boss_rise8()  { Self.Frame = float32(BOSS1_FRAME_rise8); Self.NextThink = Time + 0.1; Self.Think = boss_rise9 }
func boss_rise9()  { Self.Frame = float32(BOSS1_FRAME_rise9); Self.NextThink = Time + 0.1; Self.Think = boss_rise10 }
func boss_rise10() { Self.Frame = float32(BOSS1_FRAME_rise10); Self.NextThink = Time + 0.1; Self.Think = boss_rise11 }
func boss_rise11() { Self.Frame = float32(BOSS1_FRAME_rise11); Self.NextThink = Time + 0.1; Self.Think = boss_rise12 }
func boss_rise12() { Self.Frame = float32(BOSS1_FRAME_rise12); Self.NextThink = Time + 0.1; Self.Think = boss_rise13 }
func boss_rise13() { Self.Frame = float32(BOSS1_FRAME_rise13); Self.NextThink = Time + 0.1; Self.Think = boss_rise14 }
func boss_rise14() { Self.Frame = float32(BOSS1_FRAME_rise14); Self.NextThink = Time + 0.1; Self.Think = boss_rise15 }
func boss_rise15() { Self.Frame = float32(BOSS1_FRAME_rise15); Self.NextThink = Time + 0.1; Self.Think = boss_rise16 }
func boss_rise16() { Self.Frame = float32(BOSS1_FRAME_rise16); Self.NextThink = Time + 0.1; Self.Think = boss_rise17 }
func boss_rise17() { Self.Frame = float32(BOSS1_FRAME_rise17); Self.NextThink = Time + 0.1; Self.Think = boss_missile1 }

func boss_idle1_impl() {
	Self.Frame = float32(BOSS1_FRAME_walk1)
	Self.NextThink = Time + 0.1
	Self.Think = boss_idle2

	if Self.Enemy.Health > 0.0 {
		boss_missile1()
	} else {
		boss_face()
	}
}

func boss_idle2()  { Self.Frame = float32(BOSS1_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = boss_idle3; boss_face() }
func boss_idle3()  { Self.Frame = float32(BOSS1_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = boss_idle4; boss_face() }
func boss_idle4()  { Self.Frame = float32(BOSS1_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = boss_idle5; boss_face() }
func boss_idle5()  { Self.Frame = float32(BOSS1_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = boss_idle6; boss_face() }
func boss_idle6()  { Self.Frame = float32(BOSS1_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = boss_idle7; boss_face() }
func boss_idle7()  { Self.Frame = float32(BOSS1_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = boss_idle8; boss_face() }
func boss_idle8()  { Self.Frame = float32(BOSS1_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = boss_idle9; boss_face() }
func boss_idle9()  { Self.Frame = float32(BOSS1_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = boss_idle10; boss_face() }
func boss_idle10() { Self.Frame = float32(BOSS1_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = boss_idle11; boss_face() }
func boss_idle11() { Self.Frame = float32(BOSS1_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = boss_idle12; boss_face() }
func boss_idle12() { Self.Frame = float32(BOSS1_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = boss_idle13; boss_face() }
func boss_idle13() { Self.Frame = float32(BOSS1_FRAME_walk13); Self.NextThink = Time + 0.1; Self.Think = boss_idle14; boss_face() }
func boss_idle14() { Self.Frame = float32(BOSS1_FRAME_walk14); Self.NextThink = Time + 0.1; Self.Think = boss_idle15; boss_face() }
func boss_idle15() { Self.Frame = float32(BOSS1_FRAME_walk15); Self.NextThink = Time + 0.1; Self.Think = boss_idle16; boss_face() }
func boss_idle16() { Self.Frame = float32(BOSS1_FRAME_walk16); Self.NextThink = Time + 0.1; Self.Think = boss_idle17; boss_face() }
func boss_idle17() { Self.Frame = float32(BOSS1_FRAME_walk17); Self.NextThink = Time + 0.1; Self.Think = boss_idle18; boss_face() }
func boss_idle18() { Self.Frame = float32(BOSS1_FRAME_walk18); Self.NextThink = Time + 0.1; Self.Think = boss_idle19; boss_face() }
func boss_idle19() { Self.Frame = float32(BOSS1_FRAME_walk19); Self.NextThink = Time + 0.1; Self.Think = boss_idle20; boss_face() }
func boss_idle20() { Self.Frame = float32(BOSS1_FRAME_walk20); Self.NextThink = Time + 0.1; Self.Think = boss_idle21; boss_face() }
func boss_idle21() { Self.Frame = float32(BOSS1_FRAME_walk21); Self.NextThink = Time + 0.1; Self.Think = boss_idle22; boss_face() }
func boss_idle22() { Self.Frame = float32(BOSS1_FRAME_walk22); Self.NextThink = Time + 0.1; Self.Think = boss_idle23; boss_face() }
func boss_idle23() { Self.Frame = float32(BOSS1_FRAME_walk23); Self.NextThink = Time + 0.1; Self.Think = boss_idle24; boss_face() }
func boss_idle24() { Self.Frame = float32(BOSS1_FRAME_walk24); Self.NextThink = Time + 0.1; Self.Think = boss_idle25; boss_face() }
func boss_idle25() { Self.Frame = float32(BOSS1_FRAME_walk25); Self.NextThink = Time + 0.1; Self.Think = boss_idle26; boss_face() }
func boss_idle26() { Self.Frame = float32(BOSS1_FRAME_walk26); Self.NextThink = Time + 0.1; Self.Think = boss_idle27; boss_face() }
func boss_idle27() { Self.Frame = float32(BOSS1_FRAME_walk27); Self.NextThink = Time + 0.1; Self.Think = boss_idle28; boss_face() }
func boss_idle28() { Self.Frame = float32(BOSS1_FRAME_walk28); Self.NextThink = Time + 0.1; Self.Think = boss_idle29; boss_face() }
func boss_idle29() { Self.Frame = float32(BOSS1_FRAME_walk29); Self.NextThink = Time + 0.1; Self.Think = boss_idle30; boss_face() }
func boss_idle30() { Self.Frame = float32(BOSS1_FRAME_walk30); Self.NextThink = Time + 0.1; Self.Think = boss_idle31; boss_face() }
func boss_idle31() { Self.Frame = float32(BOSS1_FRAME_walk31); Self.NextThink = Time + 0.1; Self.Think = boss_idle1_impl; boss_face() }

func boss_missile1_impl() { Self.Frame = float32(BOSS1_FRAME_attack1); Self.NextThink = Time + 0.1; Self.Think = boss_missile2; boss_face() }
func boss_missile2() { Self.Frame = float32(BOSS1_FRAME_attack2); Self.NextThink = Time + 0.1; Self.Think = boss_missile3; boss_face() }
func boss_missile3() { Self.Frame = float32(BOSS1_FRAME_attack3); Self.NextThink = Time + 0.1; Self.Think = boss_missile4; boss_face() }
func boss_missile4() { Self.Frame = float32(BOSS1_FRAME_attack4); Self.NextThink = Time + 0.1; Self.Think = boss_missile5; boss_face() }
func boss_missile5() { Self.Frame = float32(BOSS1_FRAME_attack5); Self.NextThink = Time + 0.1; Self.Think = boss_missile6; boss_face() }
func boss_missile6() { Self.Frame = float32(BOSS1_FRAME_attack6); Self.NextThink = Time + 0.1; Self.Think = boss_missile7; boss_face() }
func boss_missile7() { Self.Frame = float32(BOSS1_FRAME_attack7); Self.NextThink = Time + 0.1; Self.Think = boss_missile8; boss_face() }
func boss_missile8() { Self.Frame = float32(BOSS1_FRAME_attack8); Self.NextThink = Time + 0.1; Self.Think = boss_missile9; boss_face() }
func boss_missile9() {
	Self.Frame = float32(BOSS1_FRAME_attack9)
	Self.NextThink = Time + 0.1
	Self.Think = boss_missile10
	boss_missile(quake.MakeVec3(100, 100, 200))
}
func boss_missile10() { Self.Frame = float32(BOSS1_FRAME_attack10); Self.NextThink = Time + 0.1; Self.Think = boss_missile11; boss_face() }
func boss_missile11() { Self.Frame = float32(BOSS1_FRAME_attack11); Self.NextThink = Time + 0.1; Self.Think = boss_missile12; boss_face() }
func boss_missile12() { Self.Frame = float32(BOSS1_FRAME_attack12); Self.NextThink = Time + 0.1; Self.Think = boss_missile13; boss_face() }
func boss_missile13() { Self.Frame = float32(BOSS1_FRAME_attack13); Self.NextThink = Time + 0.1; Self.Think = boss_missile14; boss_face() }
func boss_missile14() { Self.Frame = float32(BOSS1_FRAME_attack14); Self.NextThink = Time + 0.1; Self.Think = boss_missile15; boss_face() }
func boss_missile15() { Self.Frame = float32(BOSS1_FRAME_attack15); Self.NextThink = Time + 0.1; Self.Think = boss_missile16; boss_face() }
func boss_missile16() { Self.Frame = float32(BOSS1_FRAME_attack16); Self.NextThink = Time + 0.1; Self.Think = boss_missile17; boss_face() }
func boss_missile17() { Self.Frame = float32(BOSS1_FRAME_attack17); Self.NextThink = Time + 0.1; Self.Think = boss_missile18; boss_face() }
func boss_missile18() { Self.Frame = float32(BOSS1_FRAME_attack18); Self.NextThink = Time + 0.1; Self.Think = boss_missile19; boss_face() }
func boss_missile19() { Self.Frame = float32(BOSS1_FRAME_attack19); Self.NextThink = Time + 0.1; Self.Think = boss_missile20; boss_face() }
func boss_missile20() {
	Self.Frame = float32(BOSS1_FRAME_attack20)
	Self.NextThink = Time + 0.1
	Self.Think = boss_missile21
	boss_missile(quake.MakeVec3(100, -100, 200))
}
func boss_missile21() { Self.Frame = float32(BOSS1_FRAME_attack21); Self.NextThink = Time + 0.1; Self.Think = boss_missile22; boss_face() }
func boss_missile22() { Self.Frame = float32(BOSS1_FRAME_attack22); Self.NextThink = Time + 0.1; Self.Think = boss_missile23; boss_face() }
func boss_missile23() { Self.Frame = float32(BOSS1_FRAME_attack23); Self.NextThink = Time + 0.1; Self.Think = boss_missile1_impl; boss_face() }

func boss_shocka1()  { Self.Frame = float32(BOSS1_FRAME_shocka1); Self.NextThink = Time + 0.1; Self.Think = boss_shocka2 }
func boss_shocka2()  { Self.Frame = float32(BOSS1_FRAME_shocka2); Self.NextThink = Time + 0.1; Self.Think = boss_shocka3 }
func boss_shocka3()  { Self.Frame = float32(BOSS1_FRAME_shocka3); Self.NextThink = Time + 0.1; Self.Think = boss_shocka4 }
func boss_shocka4()  { Self.Frame = float32(BOSS1_FRAME_shocka4); Self.NextThink = Time + 0.1; Self.Think = boss_shocka5 }
func boss_shocka5()  { Self.Frame = float32(BOSS1_FRAME_shocka5); Self.NextThink = Time + 0.1; Self.Think = boss_shocka6 }
func boss_shocka6()  { Self.Frame = float32(BOSS1_FRAME_shocka6); Self.NextThink = Time + 0.1; Self.Think = boss_shocka7 }
func boss_shocka7()  { Self.Frame = float32(BOSS1_FRAME_shocka7); Self.NextThink = Time + 0.1; Self.Think = boss_shocka8 }
func boss_shocka8()  { Self.Frame = float32(BOSS1_FRAME_shocka8); Self.NextThink = Time + 0.1; Self.Think = boss_shocka9 }
func boss_shocka9()  { Self.Frame = float32(BOSS1_FRAME_shocka9); Self.NextThink = Time + 0.1; Self.Think = boss_shocka10 }
func boss_shocka10() { Self.Frame = float32(BOSS1_FRAME_shocka10); Self.NextThink = Time + 0.1; Self.Think = boss_missile1_impl }

func boss_shockb1()  { Self.Frame = float32(BOSS1_FRAME_shockb1); Self.NextThink = Time + 0.1; Self.Think = boss_shockb2 }
func boss_shockb2()  { Self.Frame = float32(BOSS1_FRAME_shockb2); Self.NextThink = Time + 0.1; Self.Think = boss_shockb3 }
func boss_shockb3()  { Self.Frame = float32(BOSS1_FRAME_shockb3); Self.NextThink = Time + 0.1; Self.Think = boss_shockb4 }
func boss_shockb4()  { Self.Frame = float32(BOSS1_FRAME_shockb4); Self.NextThink = Time + 0.1; Self.Think = boss_shockb5 }
func boss_shockb5()  { Self.Frame = float32(BOSS1_FRAME_shockb5); Self.NextThink = Time + 0.1; Self.Think = boss_shockb6 }
func boss_shockb6()  { Self.Frame = float32(BOSS1_FRAME_shockb6); Self.NextThink = Time + 0.1; Self.Think = boss_shockb7 }
func boss_shockb7()  { Self.Frame = float32(BOSS1_FRAME_shockb1); Self.NextThink = Time + 0.1; Self.Think = boss_shockb8 }
func boss_shockb8()  { Self.Frame = float32(BOSS1_FRAME_shockb2); Self.NextThink = Time + 0.1; Self.Think = boss_shockb9 }
func boss_shockb9()  { Self.Frame = float32(BOSS1_FRAME_shockb3); Self.NextThink = Time + 0.1; Self.Think = boss_shockb10 }
func boss_shockb10() { Self.Frame = float32(BOSS1_FRAME_shockb4); Self.NextThink = Time + 0.1; Self.Think = boss_missile1_impl }

func boss_shockc1()  { Self.Frame = float32(BOSS1_FRAME_shockc1); Self.NextThink = Time + 0.1; Self.Think = boss_shockc2 }
func boss_shockc2()  { Self.Frame = float32(BOSS1_FRAME_shockc2); Self.NextThink = Time + 0.1; Self.Think = boss_shockc3 }
func boss_shockc3()  { Self.Frame = float32(BOSS1_FRAME_shockc3); Self.NextThink = Time + 0.1; Self.Think = boss_shockc4 }
func boss_shockc4()  { Self.Frame = float32(BOSS1_FRAME_shockc4); Self.NextThink = Time + 0.1; Self.Think = boss_shockc5 }
func boss_shockc5()  { Self.Frame = float32(BOSS1_FRAME_shockc5); Self.NextThink = Time + 0.1; Self.Think = boss_shockc6 }
func boss_shockc6()  { Self.Frame = float32(BOSS1_FRAME_shockc6); Self.NextThink = Time + 0.1; Self.Think = boss_shockc7 }
func boss_shockc7()  { Self.Frame = float32(BOSS1_FRAME_shockc7); Self.NextThink = Time + 0.1; Self.Think = boss_shockc8 }
func boss_shockc8()  { Self.Frame = float32(BOSS1_FRAME_shockc8); Self.NextThink = Time + 0.1; Self.Think = boss_shockc9 }
func boss_shockc9()  { Self.Frame = float32(BOSS1_FRAME_shockc9); Self.NextThink = Time + 0.1; Self.Think = boss_shockc10 }
func boss_shockc10() { Self.Frame = float32(BOSS1_FRAME_shockc10); Self.NextThink = Time + 0.1; Self.Think = boss_death1 }

func boss_death1() {
	Self.Frame = float32(BOSS1_FRAME_death1)
	Self.NextThink = Time + 0.1
	Self.Think = boss_death2
	engine.Sound(Self, int(CHAN_VOICE), "boss1/death.wav", 1, ATTN_NORM)
}

func boss_death2() { Self.Frame = float32(BOSS1_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = boss_death3 }
func boss_death3() { Self.Frame = float32(BOSS1_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = boss_death4 }
func boss_death4() { Self.Frame = float32(BOSS1_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = boss_death5 }
func boss_death5() { Self.Frame = float32(BOSS1_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = boss_death6 }
func boss_death6() { Self.Frame = float32(BOSS1_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = boss_death7 }
func boss_death7() { Self.Frame = float32(BOSS1_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = boss_death8 }
func boss_death8() { Self.Frame = float32(BOSS1_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = boss_death9 }
func boss_death9() {
	Self.Frame = float32(BOSS1_FRAME_death9)
	Self.NextThink = Time + 0.1
	Self.Think = boss_death10
	engine.Sound(Self, int(CHAN_BODY), "boss1/out1.wav", 1, ATTN_NORM)
	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_LAVASPLASH))
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])
}

func boss_death10() {
	Self.Frame = float32(BOSS1_FRAME_death9)
	Self.NextThink = Time + 0.1
	Self.Think = boss_death10
	KilledMonsters = KilledMonsters + 1
	engine.WriteByte(MSG_ALL, float32(SVC_KILLEDMONSTER)) // FIXME: reliable broadcast
	SUB_UseTargets()
	engine.Remove(Self)
}

func boss_missile(p quake.Vec3) {
	var offang quake.Vec3
	var org, vec, d quake.Vec3
	var t float32

	offang = engine.VectoAngles(Self.Enemy.Origin.Sub(Self.Origin))
	engine.MakeVectors(offang)

	org = Self.Origin.Add(VForward.Mul(p[0])).Add(VRight.Mul(p[1])).Add(quake.MakeVec3(0, 0, 1).Mul(p[2]))

	if Skill > 1 {
		t = engine.Vlen(Self.Enemy.Origin.Sub(org)) / 300
		vec = Self.Enemy.Velocity
		vec[2] = 0
		d = Self.Enemy.Origin.Add(vec.Mul(t))
	} else {
		d = Self.Enemy.Origin
	}

	vec = engine.Normalize(d.Sub(org))

	launch_spike(org, vec)
	Newmis.ClassName = "chthon_lavaball"
	engine.SetModel(Newmis, "progs/lavaball.mdl")
	Newmis.AVelocity = quake.MakeVec3(200, 100, 300)
	engine.SetSize(Newmis, VEC_ORIGIN, VEC_ORIGIN)
	Newmis.Velocity = vec.Mul(300)
	Newmis.Touch = T_MissileTouch
	engine.Sound(Self, int(CHAN_WEAPON), "boss1/throw.wav", 1, ATTN_NORM)

	if Self.Enemy.Health <= 0 {
		boss_idle1()
	}
}

func boss_awake() {
	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP
	Self.TakeDamage = DAMAGE_NO

	engine.SetModel(Self, "progs/boss.mdl")

	Self.NetName = "$qc_chthon"
	Self.KillString = "$qc_ks_chthon"

	engine.SetSize(Self, quake.MakeVec3(-128, -128, -24), quake.MakeVec3(128, 128, 256))

	if Skill == 0 {
		Self.Health = 1
	} else {
		Self.Health = 3
	}

	Self.Enemy = Activator

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_LAVASPLASH))
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[0])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[1])
	engine.WriteCoord(MSG_BROADCAST, Self.Origin[2])

	Self.YawSpeed = 20
	boss_rise1()
}

func monster_boss() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/boss.mdl")
	engine.PrecacheModel("progs/lavaball.mdl")

	engine.PrecacheSound("weapons/rocket1i.wav")
	engine.PrecacheSound("boss1/out1.wav")
	engine.PrecacheSound("boss1/sight1.wav")
	engine.PrecacheSound("misc/power.wav")
	engine.PrecacheSound("boss1/throw.wav")
	engine.PrecacheSound("boss1/pain.wav")
	engine.PrecacheSound("boss1/death.wav")

	TotalMonsters = TotalMonsters + 1

	Self.Use = boss_awake
}

func lightning_fire() {
	var p1, p2 quake.Vec3

	if Time >= lightning_end {
		stemp := Self
		Self = le1
		door_go_down()
		Self = le2
		door_go_down()
		Self = stemp
		return
	}

	p1 = le1.Mins.Add(le1.Maxs).Mul(0.5)
	p1[2] = le1.AbsMin[2] - 16

	p2 = le2.Mins.Add(le2.Maxs).Mul(0.5)
	p2[2] = le2.AbsMin[2] - 16

	p2 = p2.Sub(engine.Normalize(p2.Sub(p1)).Mul(100))

	Self.NextThink = Time + 0.1
	Self.Think = lightning_fire

	engine.WriteByte(MSG_ALL, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_ALL, float32(TE_LIGHTNING3))
	engine.WriteEntity(MSG_ALL, World)
	engine.WriteCoord(MSG_ALL, p1[0])
	engine.WriteCoord(MSG_ALL, p1[1])
	engine.WriteCoord(MSG_ALL, p1[2])
	engine.WriteCoord(MSG_ALL, p2[0])
	engine.WriteCoord(MSG_ALL, p2[1])
	engine.WriteCoord(MSG_ALL, p2[2])
}

func lightning_use() {
	if lightning_end >= Time+1 {
		return
	}

	le1 = engine.Find(World, "target", "lightning")
	le2 = engine.Find(le1, "target", "lightning")

	if le1 == nil || le2 == nil {
		engine.Dprint("missing lightning targets\n")
		return
	}

	if (le1.State != float32(STATE_TOP) && le1.State != float32(STATE_BOTTOM)) || (le2.State != float32(STATE_TOP) && le2.State != float32(STATE_BOTTOM)) || (le1.State != le2.State) {
		return
	}

	le1.NextThink = -1
	le2.NextThink = -1
	lightning_end = Time + 1

	engine.Sound(Self, int(CHAN_VOICE), "misc/power.wav", 1, ATTN_NORM)
	lightning_fire()

	stemp := Self
	Self = engine.Find(World, "classname", "monster_boss")

	if Self == nil {
		Self = stemp
		return
	}

	Self.Enemy = Activator

	if le1.State == float32(STATE_TOP) && Self.Health > 0 {
		engine.Sound(Self, int(CHAN_VOICE), "boss1/pain.wav", 1, ATTN_NORM)
		Self.Health = Self.Health - 1

		if Self.Health >= 2 {
			boss_shocka1()
		} else if Self.Health == 1 {
			boss_shockb1()
		} else if Self.Health == 0 {
			boss_shockc1()
		}
	}
	Self = stemp
}

func event_lightning() {
	Self.Use = lightning_use
}

func init() {
	boss_missile1 = boss_missile1_impl
	boss_idle1 = boss_idle1_impl
}
