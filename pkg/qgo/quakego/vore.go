package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Vore (Shalrath) frames
	VORE_FRAME_attack1 = iota
	VORE_FRAME_attack2
	VORE_FRAME_attack3
	VORE_FRAME_attack4
	VORE_FRAME_attack5
	VORE_FRAME_attack6
	VORE_FRAME_attack7
	VORE_FRAME_attack8
	VORE_FRAME_attack9
	VORE_FRAME_attack10
	VORE_FRAME_attack11

	VORE_FRAME_pain1
	VORE_FRAME_pain2
	VORE_FRAME_pain3
	VORE_FRAME_pain4
	VORE_FRAME_pain5

	VORE_FRAME_death1
	VORE_FRAME_death2
	VORE_FRAME_death3
	VORE_FRAME_death4
	VORE_FRAME_death5
	VORE_FRAME_death6
	VORE_FRAME_death7

	VORE_FRAME_walk1
	VORE_FRAME_walk2
	VORE_FRAME_walk3
	VORE_FRAME_walk4
	VORE_FRAME_walk5
	VORE_FRAME_walk6
	VORE_FRAME_walk7
	VORE_FRAME_walk8
	VORE_FRAME_walk9
	VORE_FRAME_walk10
	VORE_FRAME_walk11
	VORE_FRAME_walk12
)

// Prototyped elsewhere
var shal_stand func()
var shal_run1 func()
var ShalMissile func()
var ShalHome func()
var ShalMissileTouch func()

func shal_stand_impl() { Self.Frame = float32(VORE_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = shal_stand; ai_stand() }

func shal_walk1() {
	Self.Frame = float32(VORE_FRAME_walk2)
	Self.NextThink = Time + 0.1
	Self.Think = shal_walk2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "shalrath/idle.wav", 1, ATTN_IDLE)
	}
	ai_walk(6)
}

func shal_walk2()  { Self.Frame = float32(VORE_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = shal_walk3; ai_walk(4) }
func shal_walk3()  { Self.Frame = float32(VORE_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = shal_walk4; ai_walk(0) }
func shal_walk4()  { Self.Frame = float32(VORE_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = shal_walk5; ai_walk(0) }
func shal_walk5()  { Self.Frame = float32(VORE_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = shal_walk6; ai_walk(0) }
func shal_walk6()  { Self.Frame = float32(VORE_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = shal_walk7; ai_walk(0) }
func shal_walk7()  { Self.Frame = float32(VORE_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = shal_walk8; ai_walk(5) }
func shal_walk8()  { Self.Frame = float32(VORE_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = shal_walk9; ai_walk(6) }
func shal_walk9()  { Self.Frame = float32(VORE_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = shal_walk10; ai_walk(5) }
func shal_walk10() { Self.Frame = float32(VORE_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = shal_walk11; ai_walk(0) }
func shal_walk11() { Self.Frame = float32(VORE_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = shal_walk12; ai_walk(4) }
func shal_walk12() { Self.Frame = float32(VORE_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = shal_walk1; ai_walk(5) }

func shal_run1_impl() {
	Self.Frame = float32(VORE_FRAME_walk2)
	Self.NextThink = Time + 0.1
	Self.Think = shal_run2
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "shalrath/idle.wav", 1, ATTN_IDLE)
	}
	ai_run(6)
}

func shal_run2()  { Self.Frame = float32(VORE_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = shal_run3; ai_run(4) }
func shal_run3()  { Self.Frame = float32(VORE_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = shal_run4; ai_run(0) }
func shal_run4()  { Self.Frame = float32(VORE_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = shal_run5; ai_run(0) }
func shal_run5()  { Self.Frame = float32(VORE_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = shal_run6; ai_run(0) }
func shal_run6()  { Self.Frame = float32(VORE_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = shal_run7; ai_run(0) }
func shal_run7()  { Self.Frame = float32(VORE_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = shal_run8; ai_run(5) }
func shal_run8()  { Self.Frame = float32(VORE_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = shal_run9; ai_run(6) }
func shal_run9()  { Self.Frame = float32(VORE_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = shal_run10; ai_run(5) }
func shal_run10() { Self.Frame = float32(VORE_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = shal_run11; ai_run(0) }
func shal_run11() { Self.Frame = float32(VORE_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = shal_run12; ai_run(4) }
func shal_run12() { Self.Frame = float32(VORE_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = shal_run1_impl; ai_run(5) }

func shal_attack1() {
	Self.Frame = float32(VORE_FRAME_attack1)
	Self.NextThink = Time + 0.1
	Self.Think = shal_attack2
	engine.Sound(Self, int(CHAN_VOICE), "shalrath/attack.wav", 1, ATTN_NORM)
	ai_face()
}

func shal_attack2()  { Self.Frame = float32(VORE_FRAME_attack2); Self.NextThink = Time + 0.1; Self.Think = shal_attack3; ai_face() }
func shal_attack3()  { Self.Frame = float32(VORE_FRAME_attack3); Self.NextThink = Time + 0.1; Self.Think = shal_attack4; ai_face() }
func shal_attack4()  { Self.Frame = float32(VORE_FRAME_attack4); Self.NextThink = Time + 0.1; Self.Think = shal_attack5; ai_face() }
func shal_attack5()  { Self.Frame = float32(VORE_FRAME_attack5); Self.NextThink = Time + 0.1; Self.Think = shal_attack6; ai_face() }
func shal_attack6()  { Self.Frame = float32(VORE_FRAME_attack6); Self.NextThink = Time + 0.1; Self.Think = shal_attack7; ai_face() }
func shal_attack7()  { Self.Frame = float32(VORE_FRAME_attack7); Self.NextThink = Time + 0.1; Self.Think = shal_attack8; ai_face() }
func shal_attack8()  { Self.Frame = float32(VORE_FRAME_attack8); Self.NextThink = Time + 0.1; Self.Think = shal_attack9; ai_face() }
func shal_attack9()  { Self.Frame = float32(VORE_FRAME_attack9); Self.NextThink = Time + 0.1; Self.Think = shal_attack10; ShalMissile() }
func shal_attack10() { Self.Frame = float32(VORE_FRAME_attack10); Self.NextThink = Time + 0.1; Self.Think = shal_attack11; ai_face() }
func shal_attack11() { Self.Frame = float32(VORE_FRAME_attack11); Self.NextThink = Time + 0.1; Self.Think = shal_run1_impl }

func shal_pain1() { Self.Frame = float32(VORE_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = shal_pain2 }
func shal_pain2() { Self.Frame = float32(VORE_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = shal_pain3 }
func shal_pain3() { Self.Frame = float32(VORE_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = shal_pain4 }
func shal_pain4() { Self.Frame = float32(VORE_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = shal_pain5 }
func shal_pain5() { Self.Frame = float32(VORE_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = shal_run1_impl }

func shal_death1() { Self.Frame = float32(VORE_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = shal_death2 }
func shal_death2() { Self.Frame = float32(VORE_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = shal_death3 }
func shal_death3() { Self.Frame = float32(VORE_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = shal_death4 }
func shal_death4() { Self.Frame = float32(VORE_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = shal_death5 }
func shal_death5() { Self.Frame = float32(VORE_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = shal_death6 }
func shal_death6() { Self.Frame = float32(VORE_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = shal_death7 }
func shal_death7() { Self.Frame = float32(VORE_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = shal_death7 }

func shalrath_pain(attacker *quake.Entity, damage float32) {
	if Self.PainFinished > Time {
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "shalrath/pain.wav", 1, ATTN_NORM)
	shal_pain1()
	Self.PainFinished = Time + 3
}

func shalrath_die() {
	if Self.Health < -90 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_shal.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "shalrath/death.wav", 1, ATTN_NORM)
	shal_death1()
	Self.Solid = SOLID_NOT
}

func ShalMissile_impl() {
	var missile *quake.Entity
	var dir quake.Vec3
	var dist, flytime float32

	dir = engine.Normalize(Self.Enemy.Origin.Add(quake.MakeVec3(0, 0, 10)).Sub(Self.Origin))
	dist = engine.Vlen(Self.Enemy.Origin.Sub(Self.Origin))
	flytime = dist * 0.002

	if flytime < 0.1 {
		flytime = 0.1
	}

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
	engine.Sound(Self, int(CHAN_WEAPON), "shalrath/attack2.wav", 1, ATTN_NORM)

	missile = engine.Spawn()
	missile.ClassName = "vore_ball"
	missile.Owner = Self

	missile.Solid = SOLID_BBOX
	missile.MoveType = MOVETYPE_FLYMISSILE
	engine.SetModel(missile, "progs/v_spike.mdl")

	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))

	missile.Origin = Self.Origin.Add(quake.MakeVec3(0, 0, 10))
	missile.Velocity = dir.Mul(400)
	missile.AVelocity = quake.MakeVec3(300, 300, 300)
	missile.NextThink = flytime + Time
	missile.Think = ShalHome
	missile.Enemy = Self.Enemy
	missile.Touch = ShalMissileTouch
}

func ShalHome_impl() {
	var dir, vtemp quake.Vec3
	vtemp = Self.Enemy.Origin.Add(quake.MakeVec3(0, 0, 10))

	if Self.Enemy.Health < 1 {
		engine.Remove(Self)
		return
	}

	dir = engine.Normalize(vtemp.Sub(Self.Origin))
	Self.Velocity = dir.Mul(250)
	Self.NextThink = Time + 0.2
	Self.Think = ShalHome
}

func ShalMissileTouch_impl() {
	if Other == Self.Owner {
		return
	}

	if Other.ClassName == "monster_zombie" {
		T_Damage(Other, Self, Self, 110)
	}

	T_RadiusDamage(Self, Self.Owner, 40, World)
	engine.Sound(Self, int(CHAN_WEAPON), "weapons/r_exp3.wav", 1, ATTN_NORM)

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

func init() {
	shal_stand = shal_stand_impl
	shal_run1 = shal_run1_impl
	ShalMissile = ShalMissile_impl
	ShalHome = ShalHome_impl
	ShalMissileTouch = ShalMissileTouch_impl
}

func monster_shalrath() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel2("progs/shalrath.mdl")
	engine.PrecacheModel2("progs/h_shal.mdl")
	engine.PrecacheModel2("progs/v_spike.mdl")

	engine.PrecacheSound2("shalrath/attack.wav")
	engine.PrecacheSound2("shalrath/attack2.wav")
	engine.PrecacheSound2("shalrath/death.wav")
	engine.PrecacheSound2("shalrath/idle.wav")
	engine.PrecacheSound2("shalrath/pain.wav")
	engine.PrecacheSound2("shalrath/sight.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/shalrath.mdl")

	Self.Noise = "shalrath/sight.wav"
	Self.NetName = "$qc_vore"
	Self.KillString = "$qc_ks_vore"

	engine.SetSize(Self, VEC_HULL2_MIN, VEC_HULL2_MAX)
	Self.Health = 400
	Self.MaxHealth = 400

	Self.ThStand = shal_stand
	Self.ThWalk = shal_walk1
	Self.ThRun = shal_run1
	Self.ThDie = shalrath_die
	Self.ThPain = shalrath_pain
	Self.ThMissile = shal_attack1
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_RANGED

	Self.Think = walkmonster_start
	Self.NextThink = Time + 0.1 + engine.Random()*0.1
}
