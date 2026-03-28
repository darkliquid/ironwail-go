package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Zombie frames
	ZOMBIE_FRAME_stand1 = iota
	ZOMBIE_FRAME_stand2
	ZOMBIE_FRAME_stand3
	ZOMBIE_FRAME_stand4
	ZOMBIE_FRAME_stand5
	ZOMBIE_FRAME_stand6
	ZOMBIE_FRAME_stand7
	ZOMBIE_FRAME_stand8
	ZOMBIE_FRAME_stand9
	ZOMBIE_FRAME_stand10
	ZOMBIE_FRAME_stand11
	ZOMBIE_FRAME_stand12
	ZOMBIE_FRAME_stand13
	ZOMBIE_FRAME_stand14
	ZOMBIE_FRAME_stand15

	ZOMBIE_FRAME_walk1
	ZOMBIE_FRAME_walk2
	ZOMBIE_FRAME_walk3
	ZOMBIE_FRAME_walk4
	ZOMBIE_FRAME_walk5
	ZOMBIE_FRAME_walk6
	ZOMBIE_FRAME_walk7
	ZOMBIE_FRAME_walk8
	ZOMBIE_FRAME_walk9
	ZOMBIE_FRAME_walk10
	ZOMBIE_FRAME_walk11
	ZOMBIE_FRAME_walk12
	ZOMBIE_FRAME_walk13
	ZOMBIE_FRAME_walk14
	ZOMBIE_FRAME_walk15
	ZOMBIE_FRAME_walk16
	ZOMBIE_FRAME_walk17
	ZOMBIE_FRAME_walk18
	ZOMBIE_FRAME_walk19

	ZOMBIE_FRAME_run1
	ZOMBIE_FRAME_run2
	ZOMBIE_FRAME_run3
	ZOMBIE_FRAME_run4
	ZOMBIE_FRAME_run5
	ZOMBIE_FRAME_run6
	ZOMBIE_FRAME_run7
	ZOMBIE_FRAME_run8
	ZOMBIE_FRAME_run9
	ZOMBIE_FRAME_run10
	ZOMBIE_FRAME_run11
	ZOMBIE_FRAME_run12
	ZOMBIE_FRAME_run13
	ZOMBIE_FRAME_run14
	ZOMBIE_FRAME_run15
	ZOMBIE_FRAME_run16
	ZOMBIE_FRAME_run17
	ZOMBIE_FRAME_run18

	ZOMBIE_FRAME_atta1
	ZOMBIE_FRAME_atta2
	ZOMBIE_FRAME_atta3
	ZOMBIE_FRAME_atta4
	ZOMBIE_FRAME_atta5
	ZOMBIE_FRAME_atta6
	ZOMBIE_FRAME_atta7
	ZOMBIE_FRAME_atta8
	ZOMBIE_FRAME_atta9
	ZOMBIE_FRAME_atta10
	ZOMBIE_FRAME_atta11
	ZOMBIE_FRAME_atta12
	ZOMBIE_FRAME_atta13

	ZOMBIE_FRAME_attb1
	ZOMBIE_FRAME_attb2
	ZOMBIE_FRAME_attb3
	ZOMBIE_FRAME_attb4
	ZOMBIE_FRAME_attb5
	ZOMBIE_FRAME_attb6
	ZOMBIE_FRAME_attb7
	ZOMBIE_FRAME_attb8
	ZOMBIE_FRAME_attb9
	ZOMBIE_FRAME_attb10
	ZOMBIE_FRAME_attb11
	ZOMBIE_FRAME_attb12
	ZOMBIE_FRAME_attb13
	ZOMBIE_FRAME_attb14

	ZOMBIE_FRAME_attc1
	ZOMBIE_FRAME_attc2
	ZOMBIE_FRAME_attc3
	ZOMBIE_FRAME_attc4
	ZOMBIE_FRAME_attc5
	ZOMBIE_FRAME_attc6
	ZOMBIE_FRAME_attc7
	ZOMBIE_FRAME_attc8
	ZOMBIE_FRAME_attc9
	ZOMBIE_FRAME_attc10
	ZOMBIE_FRAME_attc11
	ZOMBIE_FRAME_attc12

	ZOMBIE_FRAME_paina1
	ZOMBIE_FRAME_paina2
	ZOMBIE_FRAME_paina3
	ZOMBIE_FRAME_paina4
	ZOMBIE_FRAME_paina5
	ZOMBIE_FRAME_paina6
	ZOMBIE_FRAME_paina7
	ZOMBIE_FRAME_paina8
	ZOMBIE_FRAME_paina9
	ZOMBIE_FRAME_paina10
	ZOMBIE_FRAME_paina11
	ZOMBIE_FRAME_paina12

	ZOMBIE_FRAME_painb1
	ZOMBIE_FRAME_painb2
	ZOMBIE_FRAME_painb3
	ZOMBIE_FRAME_painb4
	ZOMBIE_FRAME_painb5
	ZOMBIE_FRAME_painb6
	ZOMBIE_FRAME_painb7
	ZOMBIE_FRAME_painb8
	ZOMBIE_FRAME_painb9
	ZOMBIE_FRAME_painb10
	ZOMBIE_FRAME_painb11
	ZOMBIE_FRAME_painb12
	ZOMBIE_FRAME_painb13
	ZOMBIE_FRAME_painb14
	ZOMBIE_FRAME_painb15
	ZOMBIE_FRAME_painb16
	ZOMBIE_FRAME_painb17
	ZOMBIE_FRAME_painb18
	ZOMBIE_FRAME_painb19
	ZOMBIE_FRAME_painb20
	ZOMBIE_FRAME_painb21
	ZOMBIE_FRAME_painb22
	ZOMBIE_FRAME_painb23
	ZOMBIE_FRAME_painb24
	ZOMBIE_FRAME_painb25
	ZOMBIE_FRAME_painb26
	ZOMBIE_FRAME_painb27
	ZOMBIE_FRAME_painb28

	ZOMBIE_FRAME_painc1
	ZOMBIE_FRAME_painc2
	ZOMBIE_FRAME_painc3
	ZOMBIE_FRAME_painc4
	ZOMBIE_FRAME_painc5
	ZOMBIE_FRAME_painc6
	ZOMBIE_FRAME_painc7
	ZOMBIE_FRAME_painc8
	ZOMBIE_FRAME_painc9
	ZOMBIE_FRAME_painc10
	ZOMBIE_FRAME_painc11
	ZOMBIE_FRAME_painc12
	ZOMBIE_FRAME_painc13
	ZOMBIE_FRAME_painc14
	ZOMBIE_FRAME_painc15
	ZOMBIE_FRAME_painc16
	ZOMBIE_FRAME_painc17
	ZOMBIE_FRAME_painc18

	ZOMBIE_FRAME_paind1
	ZOMBIE_FRAME_paind2
	ZOMBIE_FRAME_paind3
	ZOMBIE_FRAME_paind4
	ZOMBIE_FRAME_paind5
	ZOMBIE_FRAME_paind6
	ZOMBIE_FRAME_paind7
	ZOMBIE_FRAME_paind8
	ZOMBIE_FRAME_paind9
	ZOMBIE_FRAME_paind10
	ZOMBIE_FRAME_paind11
	ZOMBIE_FRAME_paind12
	ZOMBIE_FRAME_paind13

	ZOMBIE_FRAME_paine1
	ZOMBIE_FRAME_paine2
	ZOMBIE_FRAME_paine3
	ZOMBIE_FRAME_paine4
	ZOMBIE_FRAME_paine5
	ZOMBIE_FRAME_paine6
	ZOMBIE_FRAME_paine7
	ZOMBIE_FRAME_paine8
	ZOMBIE_FRAME_paine9
	ZOMBIE_FRAME_paine10
	ZOMBIE_FRAME_paine11
	ZOMBIE_FRAME_paine12
	ZOMBIE_FRAME_paine13
	ZOMBIE_FRAME_paine14
	ZOMBIE_FRAME_paine15
	ZOMBIE_FRAME_paine16
	ZOMBIE_FRAME_paine17
	ZOMBIE_FRAME_paine18
	ZOMBIE_FRAME_paine19
	ZOMBIE_FRAME_paine20
	ZOMBIE_FRAME_paine21
	ZOMBIE_FRAME_paine22
	ZOMBIE_FRAME_paine23
	ZOMBIE_FRAME_paine24
	ZOMBIE_FRAME_paine25
	ZOMBIE_FRAME_paine26
	ZOMBIE_FRAME_paine27
	ZOMBIE_FRAME_paine28
	ZOMBIE_FRAME_paine29
	ZOMBIE_FRAME_paine30

	ZOMBIE_FRAME_cruc_1
	ZOMBIE_FRAME_cruc_2
	ZOMBIE_FRAME_cruc_3
	ZOMBIE_FRAME_cruc_4
	ZOMBIE_FRAME_cruc_5
	ZOMBIE_FRAME_cruc_6
)

var SPAWN_CRUCIFIED float32 = 1

func zombie_stand1()  { Self.Frame = float32(ZOMBIE_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = zombie_stand2; ai_stand() }
func zombie_stand2()  { Self.Frame = float32(ZOMBIE_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = zombie_stand3; ai_stand() }
func zombie_stand3()  { Self.Frame = float32(ZOMBIE_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = zombie_stand4; ai_stand() }
func zombie_stand4()  { Self.Frame = float32(ZOMBIE_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = zombie_stand5; ai_stand() }
func zombie_stand5()  { Self.Frame = float32(ZOMBIE_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = zombie_stand6; ai_stand() }
func zombie_stand6()  { Self.Frame = float32(ZOMBIE_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = zombie_stand7; ai_stand() }
func zombie_stand7()  { Self.Frame = float32(ZOMBIE_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = zombie_stand8; ai_stand() }
func zombie_stand8()  { Self.Frame = float32(ZOMBIE_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = zombie_stand9; ai_stand() }
func zombie_stand9()  { Self.Frame = float32(ZOMBIE_FRAME_stand9); Self.NextThink = Time + 0.1; Self.Think = zombie_stand10; ai_stand() }
func zombie_stand10() { Self.Frame = float32(ZOMBIE_FRAME_stand10); Self.NextThink = Time + 0.1; Self.Think = zombie_stand11; ai_stand() }
func zombie_stand11() { Self.Frame = float32(ZOMBIE_FRAME_stand11); Self.NextThink = Time + 0.1; Self.Think = zombie_stand12; ai_stand() }
func zombie_stand12() { Self.Frame = float32(ZOMBIE_FRAME_stand12); Self.NextThink = Time + 0.1; Self.Think = zombie_stand13; ai_stand() }
func zombie_stand13() { Self.Frame = float32(ZOMBIE_FRAME_stand13); Self.NextThink = Time + 0.1; Self.Think = zombie_stand14; ai_stand() }
func zombie_stand14() { Self.Frame = float32(ZOMBIE_FRAME_stand14); Self.NextThink = Time + 0.1; Self.Think = zombie_stand15; ai_stand() }
func zombie_stand15() { Self.Frame = float32(ZOMBIE_FRAME_stand15); Self.NextThink = Time + 0.1; Self.Think = zombie_stand1; ai_stand() }

func zombie_cruc1() {
	Self.Frame = float32(ZOMBIE_FRAME_cruc_1)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_cruc2
	if engine.Random() < 0.1 {
		engine.Sound(Self, int(CHAN_VOICE), "zombie/idle_w2.wav", 1, ATTN_STATIC)
	}
}
func zombie_cruc2() { Self.Frame = float32(ZOMBIE_FRAME_cruc_2); Self.NextThink = Time + 0.1 + engine.Random()*0.1; Self.Think = zombie_cruc3 }
func zombie_cruc3() { Self.Frame = float32(ZOMBIE_FRAME_cruc_3); Self.NextThink = Time + 0.1 + engine.Random()*0.1; Self.Think = zombie_cruc4 }
func zombie_cruc4() { Self.Frame = float32(ZOMBIE_FRAME_cruc_4); Self.NextThink = Time + 0.1 + engine.Random()*0.1; Self.Think = zombie_cruc5 }
func zombie_cruc5() { Self.Frame = float32(ZOMBIE_FRAME_cruc_5); Self.NextThink = Time + 0.1 + engine.Random()*0.1; Self.Think = zombie_cruc6 }
func zombie_cruc6() { Self.Frame = float32(ZOMBIE_FRAME_cruc_6); Self.NextThink = Time + 0.1 + engine.Random()*0.1; Self.Think = zombie_cruc1 }

func zombie_walk1()  { Self.Frame = float32(ZOMBIE_FRAME_walk1); Self.NextThink = Time + 0.1; Self.Think = zombie_walk2; ai_walk(0) }
func zombie_walk2()  { Self.Frame = float32(ZOMBIE_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = zombie_walk3; ai_walk(2) }
func zombie_walk3()  { Self.Frame = float32(ZOMBIE_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = zombie_walk4; ai_walk(3) }
func zombie_walk4()  { Self.Frame = float32(ZOMBIE_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = zombie_walk5; ai_walk(2) }
func zombie_walk5()  { Self.Frame = float32(ZOMBIE_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = zombie_walk6; ai_walk(1) }
func zombie_walk6()  { Self.Frame = float32(ZOMBIE_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = zombie_walk7; ai_walk(0) }
func zombie_walk7()  { Self.Frame = float32(ZOMBIE_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = zombie_walk8; ai_walk(0) }
func zombie_walk8()  { Self.Frame = float32(ZOMBIE_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = zombie_walk9; ai_walk(0) }
func zombie_walk9()  { Self.Frame = float32(ZOMBIE_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = zombie_walk10; ai_walk(0) }
func zombie_walk10() { Self.Frame = float32(ZOMBIE_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = zombie_walk11; ai_walk(0) }
func zombie_walk11() { Self.Frame = float32(ZOMBIE_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = zombie_walk12; ai_walk(2) }
func zombie_walk12() { Self.Frame = float32(ZOMBIE_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = zombie_walk13; ai_walk(2) }
func zombie_walk13() { Self.Frame = float32(ZOMBIE_FRAME_walk13); Self.NextThink = Time + 0.1; Self.Think = zombie_walk14; ai_walk(1) }
func zombie_walk14() { Self.Frame = float32(ZOMBIE_FRAME_walk14); Self.NextThink = Time + 0.1; Self.Think = zombie_walk15; ai_walk(0) }
func zombie_walk15() { Self.Frame = float32(ZOMBIE_FRAME_walk15); Self.NextThink = Time + 0.1; Self.Think = zombie_walk16; ai_walk(0) }
func zombie_walk16() { Self.Frame = float32(ZOMBIE_FRAME_walk16); Self.NextThink = Time + 0.1; Self.Think = zombie_walk17; ai_walk(0) }
func zombie_walk17() { Self.Frame = float32(ZOMBIE_FRAME_walk17); Self.NextThink = Time + 0.1; Self.Think = zombie_walk18; ai_walk(0) }
func zombie_walk18() { Self.Frame = float32(ZOMBIE_FRAME_walk18); Self.NextThink = Time + 0.1; Self.Think = zombie_walk19; ai_walk(0) }
func zombie_walk19() {
	Self.Frame = float32(ZOMBIE_FRAME_walk19)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_walk1
	ai_walk(0)
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "zombie/z_idle.wav", 1, ATTN_IDLE)
	}
}

func zombie_run1_impl() {
	Self.Frame = float32(ZOMBIE_FRAME_run1)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_run2
	ai_run(1)
	Self.InPain = 0
}

func zombie_run2()  { Self.Frame = float32(ZOMBIE_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = zombie_run3; ai_run(1) }
func zombie_run3()  { Self.Frame = float32(ZOMBIE_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = zombie_run4; ai_run(0) }
func zombie_run4()  { Self.Frame = float32(ZOMBIE_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = zombie_run5; ai_run(1) }
func zombie_run5()  { Self.Frame = float32(ZOMBIE_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = zombie_run6; ai_run(2) }
func zombie_run6()  { Self.Frame = float32(ZOMBIE_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = zombie_run7; ai_run(3) }
func zombie_run7()  { Self.Frame = float32(ZOMBIE_FRAME_run7); Self.NextThink = Time + 0.1; Self.Think = zombie_run8; ai_run(4) }
func zombie_run8()  { Self.Frame = float32(ZOMBIE_FRAME_run8); Self.NextThink = Time + 0.1; Self.Think = zombie_run9; ai_run(4) }
func zombie_run9()  { Self.Frame = float32(ZOMBIE_FRAME_run9); Self.NextThink = Time + 0.1; Self.Think = zombie_run10; ai_run(2) }
func zombie_run10() { Self.Frame = float32(ZOMBIE_FRAME_run10); Self.NextThink = Time + 0.1; Self.Think = zombie_run11; ai_run(0) }
func zombie_run11() { Self.Frame = float32(ZOMBIE_FRAME_run11); Self.NextThink = Time + 0.1; Self.Think = zombie_run12; ai_run(0) }
func zombie_run12() { Self.Frame = float32(ZOMBIE_FRAME_run12); Self.NextThink = Time + 0.1; Self.Think = zombie_run13; ai_run(0) }
func zombie_run13() { Self.Frame = float32(ZOMBIE_FRAME_run13); Self.NextThink = Time + 0.1; Self.Think = zombie_run14; ai_run(2) }
func zombie_run14() { Self.Frame = float32(ZOMBIE_FRAME_run14); Self.NextThink = Time + 0.1; Self.Think = zombie_run15; ai_run(4) }
func zombie_run15() { Self.Frame = float32(ZOMBIE_FRAME_run15); Self.NextThink = Time + 0.1; Self.Think = zombie_run16; ai_run(6) }
func zombie_run16() { Self.Frame = float32(ZOMBIE_FRAME_run16); Self.NextThink = Time + 0.1; Self.Think = zombie_run17; ai_run(7) }
func zombie_run17() { Self.Frame = float32(ZOMBIE_FRAME_run17); Self.NextThink = Time + 0.1; Self.Think = zombie_run18; ai_run(3) }
func zombie_run18() {
	Self.Frame = float32(ZOMBIE_FRAME_run18)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_run1_impl
	ai_run(8)
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "zombie/z_idle.wav", 1, ATTN_IDLE)
	}
	if engine.Random() > 0.8 {
		engine.Sound(Self, int(CHAN_VOICE), "zombie/z_idle1.wav", 1, ATTN_IDLE)
	}
}

func ZombieGrenadeTouch() {
	if Other == Self.Owner {
		return
	}

	if Other.TakeDamage != 0 {
		T_Damage(Other, Self, Self.Owner, 10)
		engine.Sound(Self, int(CHAN_WEAPON), "zombie/z_hit.wav", 1, ATTN_NORM)
		engine.Remove(Self)
		return
	}

	engine.Sound(Self, int(CHAN_WEAPON), "zombie/z_miss.wav", 1, ATTN_NORM)
	Self.Velocity = quake.MakeVec3(0, 0, 0)
	Self.AVelocity = quake.MakeVec3(0, 0, 0)
	Self.Touch = SUB_Remove
}

func ZombieFireGrenade(st quake.Vec3) {
	var missile *quake.Entity
	var org quake.Vec3

	engine.Sound(Self, int(CHAN_WEAPON), "zombie/z_shot1.wav", 1, ATTN_NORM)

	missile = engine.Spawn()
	missile.ClassName = "zombie_grenade"
	missile.Owner = Self
	missile.MoveType = MOVETYPE_BOUNCE
	missile.Solid = SOLID_BBOX

	engine.MakeVectors(Self.Angles)
	org = Self.Origin.Add(VForward.Mul(st[0])).Add(VRight.Mul(st[1])).Add(VUp.Mul(st[2] - 24))

	makevectorsfixed(Self.Angles)

	missile.Velocity = engine.Normalize(Self.Enemy.Origin.Sub(org))
	missile.Velocity = missile.Velocity.Mul(600)
	missile.Velocity[2] = 200

	missile.AVelocity = quake.MakeVec3(3000, 1000, 2000)

	missile.Touch = ZombieGrenadeTouch

	missile.NextThink = Time + 2.5
	missile.Think = SUB_Remove

	engine.SetModel(missile, "progs/zom_gib.mdl")
	engine.SetSize(missile, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	engine.SetOrigin(missile, org)
}

func zombie_atta1()  { Self.Frame = float32(ZOMBIE_FRAME_atta1); Self.NextThink = Time + 0.1; Self.Think = zombie_atta2; ai_face() }
func zombie_atta2()  { Self.Frame = float32(ZOMBIE_FRAME_atta2); Self.NextThink = Time + 0.1; Self.Think = zombie_atta3; ai_face() }
func zombie_atta3()  { Self.Frame = float32(ZOMBIE_FRAME_atta3); Self.NextThink = Time + 0.1; Self.Think = zombie_atta4; ai_face() }
func zombie_atta4()  { Self.Frame = float32(ZOMBIE_FRAME_atta4); Self.NextThink = Time + 0.1; Self.Think = zombie_atta5; ai_face() }
func zombie_atta5()  { Self.Frame = float32(ZOMBIE_FRAME_atta5); Self.NextThink = Time + 0.1; Self.Think = zombie_atta6; ai_face() }
func zombie_atta6()  { Self.Frame = float32(ZOMBIE_FRAME_atta6); Self.NextThink = Time + 0.1; Self.Think = zombie_atta7; ai_face() }
func zombie_atta7()  { Self.Frame = float32(ZOMBIE_FRAME_atta7); Self.NextThink = Time + 0.1; Self.Think = zombie_atta8; ai_face() }
func zombie_atta8()  { Self.Frame = float32(ZOMBIE_FRAME_atta8); Self.NextThink = Time + 0.1; Self.Think = zombie_atta9; ai_face() }
func zombie_atta9()  { Self.Frame = float32(ZOMBIE_FRAME_atta9); Self.NextThink = Time + 0.1; Self.Think = zombie_atta10; ai_face() }
func zombie_atta10() { Self.Frame = float32(ZOMBIE_FRAME_atta10); Self.NextThink = Time + 0.1; Self.Think = zombie_atta11; ai_face() }
func zombie_atta11() { Self.Frame = float32(ZOMBIE_FRAME_atta11); Self.NextThink = Time + 0.1; Self.Think = zombie_atta12; ai_face() }
func zombie_atta12() { Self.Frame = float32(ZOMBIE_FRAME_atta12); Self.NextThink = Time + 0.1; Self.Think = zombie_atta13; ai_face() }
func zombie_atta13() {
	Self.Frame = float32(ZOMBIE_FRAME_atta13)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_run1_impl
	ai_face()
	ZombieFireGrenade(quake.MakeVec3(-10, -22, 30))
}

func zombie_attb1()  { Self.Frame = float32(ZOMBIE_FRAME_attb1); Self.NextThink = Time + 0.1; Self.Think = zombie_attb2; ai_face() }
func zombie_attb2()  { Self.Frame = float32(ZOMBIE_FRAME_attb2); Self.NextThink = Time + 0.1; Self.Think = zombie_attb3; ai_face() }
func zombie_attb3()  { Self.Frame = float32(ZOMBIE_FRAME_attb3); Self.NextThink = Time + 0.1; Self.Think = zombie_attb4; ai_face() }
func zombie_attb4()  { Self.Frame = float32(ZOMBIE_FRAME_attb4); Self.NextThink = Time + 0.1; Self.Think = zombie_attb5; ai_face() }
func zombie_attb5()  { Self.Frame = float32(ZOMBIE_FRAME_attb5); Self.NextThink = Time + 0.1; Self.Think = zombie_attb6; ai_face() }
func zombie_attb6()  { Self.Frame = float32(ZOMBIE_FRAME_attb6); Self.NextThink = Time + 0.1; Self.Think = zombie_attb7; ai_face() }
func zombie_attb7()  { Self.Frame = float32(ZOMBIE_FRAME_attb7); Self.NextThink = Time + 0.1; Self.Think = zombie_attb8; ai_face() }
func zombie_attb8()  { Self.Frame = float32(ZOMBIE_FRAME_attb8); Self.NextThink = Time + 0.1; Self.Think = zombie_attb9; ai_face() }
func zombie_attb9()  { Self.Frame = float32(ZOMBIE_FRAME_attb9); Self.NextThink = Time + 0.1; Self.Think = zombie_attb10; ai_face() }
func zombie_attb10() { Self.Frame = float32(ZOMBIE_FRAME_attb10); Self.NextThink = Time + 0.1; Self.Think = zombie_attb11; ai_face() }
func zombie_attb11() { Self.Frame = float32(ZOMBIE_FRAME_attb11); Self.NextThink = Time + 0.1; Self.Think = zombie_attb12; ai_face() }
func zombie_attb12() { Self.Frame = float32(ZOMBIE_FRAME_attb12); Self.NextThink = Time + 0.1; Self.Think = zombie_attb13; ai_face() }
func zombie_attb13() { Self.Frame = float32(ZOMBIE_FRAME_attb13); Self.NextThink = Time + 0.1; Self.Think = zombie_attb14; ai_face() }
func zombie_attb14() {
	Self.Frame = float32(ZOMBIE_FRAME_attb13)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_run1_impl
	ai_face()
	ZombieFireGrenade(quake.MakeVec3(-10, -24, 29))
}

func zombie_attc1()  { Self.Frame = float32(ZOMBIE_FRAME_attc1); Self.NextThink = Time + 0.1; Self.Think = zombie_attc2; ai_face() }
func zombie_attc2()  { Self.Frame = float32(ZOMBIE_FRAME_attc2); Self.NextThink = Time + 0.1; Self.Think = zombie_attc3; ai_face() }
func zombie_attc3()  { Self.Frame = float32(ZOMBIE_FRAME_attc3); Self.NextThink = Time + 0.1; Self.Think = zombie_attc4; ai_face() }
func zombie_attc4()  { Self.Frame = float32(ZOMBIE_FRAME_attc4); Self.NextThink = Time + 0.1; Self.Think = zombie_attc5; ai_face() }
func zombie_attc5()  { Self.Frame = float32(ZOMBIE_FRAME_attc5); Self.NextThink = Time + 0.1; Self.Think = zombie_attc6; ai_face() }
func zombie_attc6()  { Self.Frame = float32(ZOMBIE_FRAME_attc6); Self.NextThink = Time + 0.1; Self.Think = zombie_attc7; ai_face() }
func zombie_attc7()  { Self.Frame = float32(ZOMBIE_FRAME_attc7); Self.NextThink = Time + 0.1; Self.Think = zombie_attc8; ai_face() }
func zombie_attc8()  { Self.Frame = float32(ZOMBIE_FRAME_attc8); Self.NextThink = Time + 0.1; Self.Think = zombie_attc9; ai_face() }
func zombie_attc9()  { Self.Frame = float32(ZOMBIE_FRAME_attc9); Self.NextThink = Time + 0.1; Self.Think = zombie_attc10; ai_face() }
func zombie_attc10() { Self.Frame = float32(ZOMBIE_FRAME_attc10); Self.NextThink = Time + 0.1; Self.Think = zombie_attc11; ai_face() }
func zombie_attc11() { Self.Frame = float32(ZOMBIE_FRAME_attc11); Self.NextThink = Time + 0.1; Self.Think = zombie_attc12; ai_face() }
func zombie_attc12() {
	Self.Frame = float32(ZOMBIE_FRAME_attc12)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_run1_impl
	ai_face()
	ZombieFireGrenade(quake.MakeVec3(-12, -19, 29))
}

func zombie_missile() {
	var r float32
	r = engine.Random()

	if r < 0.3 {
		zombie_atta1()
	} else if r < 0.6 {
		zombie_attb1()
	} else {
		zombie_attc1()
	}
}

func zombie_paina1() { Self.Frame = float32(ZOMBIE_FRAME_paina1); Self.NextThink = Time + 0.1; Self.Think = zombie_paina2; engine.Sound(Self, int(CHAN_VOICE), "zombie/z_pain.wav", 1, ATTN_NORM) }
func zombie_paina2() { Self.Frame = float32(ZOMBIE_FRAME_paina2); Self.NextThink = Time + 0.1; Self.Think = zombie_paina3; ai_painforward(3) }
func zombie_paina3() { Self.Frame = float32(ZOMBIE_FRAME_paina3); Self.NextThink = Time + 0.1; Self.Think = zombie_paina4; ai_painforward(1) }
func zombie_paina4() { Self.Frame = float32(ZOMBIE_FRAME_paina4); Self.NextThink = Time + 0.1; Self.Think = zombie_paina5; ai_pain(1) }
func zombie_paina5() { Self.Frame = float32(ZOMBIE_FRAME_paina5); Self.NextThink = Time + 0.1; Self.Think = zombie_paina6; ai_pain(3) }
func zombie_paina6() { Self.Frame = float32(ZOMBIE_FRAME_paina6); Self.NextThink = Time + 0.1; Self.Think = zombie_paina7; ai_pain(1) }
func zombie_paina7() { Self.Frame = float32(ZOMBIE_FRAME_paina7); Self.NextThink = Time + 0.1; Self.Think = zombie_paina8 }
func zombie_paina8() { Self.Frame = float32(ZOMBIE_FRAME_paina8); Self.NextThink = Time + 0.1; Self.Think = zombie_paina9 }
func zombie_paina9() { Self.Frame = float32(ZOMBIE_FRAME_paina9); Self.NextThink = Time + 0.1; Self.Think = zombie_paina10 }
func zombie_paina10() { Self.Frame = float32(ZOMBIE_FRAME_paina10); Self.NextThink = Time + 0.1; Self.Think = zombie_paina11 }
func zombie_paina11() { Self.Frame = float32(ZOMBIE_FRAME_paina11); Self.NextThink = Time + 0.1; Self.Think = zombie_paina12 }
func zombie_paina12() { Self.Frame = float32(ZOMBIE_FRAME_paina12); Self.NextThink = Time + 0.1; Self.Think = zombie_run1_impl }

func zombie_painb1() { Self.Frame = float32(ZOMBIE_FRAME_painb1); Self.NextThink = Time + 0.1; Self.Think = zombie_painb2; engine.Sound(Self, int(CHAN_VOICE), "zombie/z_pain1.wav", 1, ATTN_NORM) }
func zombie_painb2() { Self.Frame = float32(ZOMBIE_FRAME_painb2); Self.NextThink = Time + 0.1; Self.Think = zombie_painb3; ai_pain(2) }
func zombie_painb3() { Self.Frame = float32(ZOMBIE_FRAME_painb3); Self.NextThink = Time + 0.1; Self.Think = zombie_painb4; ai_pain(8) }
func zombie_painb4() { Self.Frame = float32(ZOMBIE_FRAME_painb4); Self.NextThink = Time + 0.1; Self.Think = zombie_painb5; ai_pain(6) }
func zombie_painb5() { Self.Frame = float32(ZOMBIE_FRAME_painb5); Self.NextThink = Time + 0.1; Self.Think = zombie_painb6; ai_pain(2) }
func zombie_painb6() { Self.Frame = float32(ZOMBIE_FRAME_painb6); Self.NextThink = Time + 0.1; Self.Think = zombie_painb7 }
func zombie_painb7() { Self.Frame = float32(ZOMBIE_FRAME_painb7); Self.NextThink = Time + 0.1; Self.Think = zombie_painb8 }
func zombie_painb8() { Self.Frame = float32(ZOMBIE_FRAME_painb8); Self.NextThink = Time + 0.1; Self.Think = zombie_painb9 }
func zombie_painb9() { Self.Frame = float32(ZOMBIE_FRAME_painb9); Self.NextThink = Time + 0.1; Self.Think = zombie_painb10; engine.Sound(Self, int(CHAN_BODY), "zombie/z_fall.wav", 1, ATTN_NORM) }
func zombie_painb10() { Self.Frame = float32(ZOMBIE_FRAME_painb10); Self.NextThink = Time + 0.1; Self.Think = zombie_painb11 }
func zombie_painb11() { Self.Frame = float32(ZOMBIE_FRAME_painb11); Self.NextThink = Time + 0.1; Self.Think = zombie_painb12 }
func zombie_painb12() { Self.Frame = float32(ZOMBIE_FRAME_painb12); Self.NextThink = Time + 0.1; Self.Think = zombie_painb13 }
func zombie_painb13() { Self.Frame = float32(ZOMBIE_FRAME_painb13); Self.NextThink = Time + 0.1; Self.Think = zombie_painb14 }
func zombie_painb14() { Self.Frame = float32(ZOMBIE_FRAME_painb14); Self.NextThink = Time + 0.1; Self.Think = zombie_painb15 }
func zombie_painb15() { Self.Frame = float32(ZOMBIE_FRAME_painb15); Self.NextThink = Time + 0.1; Self.Think = zombie_painb16 }
func zombie_painb16() { Self.Frame = float32(ZOMBIE_FRAME_painb16); Self.NextThink = Time + 0.1; Self.Think = zombie_painb17 }
func zombie_painb17() { Self.Frame = float32(ZOMBIE_FRAME_painb17); Self.NextThink = Time + 0.1; Self.Think = zombie_painb18 }
func zombie_painb18() { Self.Frame = float32(ZOMBIE_FRAME_painb18); Self.NextThink = Time + 0.1; Self.Think = zombie_painb19 }
func zombie_painb19() { Self.Frame = float32(ZOMBIE_FRAME_painb19); Self.NextThink = Time + 0.1; Self.Think = zombie_painb20 }
func zombie_painb20() { Self.Frame = float32(ZOMBIE_FRAME_painb20); Self.NextThink = Time + 0.1; Self.Think = zombie_painb21 }
func zombie_painb21() { Self.Frame = float32(ZOMBIE_FRAME_painb21); Self.NextThink = Time + 0.1; Self.Think = zombie_painb22 }
func zombie_painb22() { Self.Frame = float32(ZOMBIE_FRAME_painb22); Self.NextThink = Time + 0.1; Self.Think = zombie_painb23 }
func zombie_painb23() { Self.Frame = float32(ZOMBIE_FRAME_painb23); Self.NextThink = Time + 0.1; Self.Think = zombie_painb24 }
func zombie_painb24() { Self.Frame = float32(ZOMBIE_FRAME_painb24); Self.NextThink = Time + 0.1; Self.Think = zombie_painb25 }
func zombie_painb25() { Self.Frame = float32(ZOMBIE_FRAME_painb25); Self.NextThink = Time + 0.1; Self.Think = zombie_painb26; ai_painforward(1) }
func zombie_painb26() { Self.Frame = float32(ZOMBIE_FRAME_painb26); Self.NextThink = Time + 0.1; Self.Think = zombie_painb27 }
func zombie_painb27() { Self.Frame = float32(ZOMBIE_FRAME_painb27); Self.NextThink = Time + 0.1; Self.Think = zombie_painb28 }
func zombie_painb28() { Self.Frame = float32(ZOMBIE_FRAME_painb28); Self.NextThink = Time + 0.1; Self.Think = zombie_run1_impl }

func zombie_painc1() { Self.Frame = float32(ZOMBIE_FRAME_painc1); Self.NextThink = Time + 0.1; Self.Think = zombie_painc2; engine.Sound(Self, int(CHAN_VOICE), "zombie/z_pain1.wav", 1, ATTN_NORM) }
func zombie_painc2() { Self.Frame = float32(ZOMBIE_FRAME_painc2); Self.NextThink = Time + 0.1; Self.Think = zombie_painc3 }
func zombie_painc3() { Self.Frame = float32(ZOMBIE_FRAME_painc3); Self.NextThink = Time + 0.1; Self.Think = zombie_painc4; ai_pain(3) }
func zombie_painc4() { Self.Frame = float32(ZOMBIE_FRAME_painc4); Self.NextThink = Time + 0.1; Self.Think = zombie_painc5; ai_pain(1) }
func zombie_painc5() { Self.Frame = float32(ZOMBIE_FRAME_painc5); Self.NextThink = Time + 0.1; Self.Think = zombie_painc6 }
func zombie_painc6() { Self.Frame = float32(ZOMBIE_FRAME_painc6); Self.NextThink = Time + 0.1; Self.Think = zombie_painc7 }
func zombie_painc7() { Self.Frame = float32(ZOMBIE_FRAME_painc7); Self.NextThink = Time + 0.1; Self.Think = zombie_painc8 }
func zombie_painc8() { Self.Frame = float32(ZOMBIE_FRAME_painc8); Self.NextThink = Time + 0.1; Self.Think = zombie_painc9 }
func zombie_painc9() { Self.Frame = float32(ZOMBIE_FRAME_painc9); Self.NextThink = Time + 0.1; Self.Think = zombie_painc10 }
func zombie_painc10() { Self.Frame = float32(ZOMBIE_FRAME_painc10); Self.NextThink = Time + 0.1; Self.Think = zombie_painc11 }
func zombie_painc11() { Self.Frame = float32(ZOMBIE_FRAME_painc11); Self.NextThink = Time + 0.1; Self.Think = zombie_painc12; ai_painforward(1) }
func zombie_painc12() { Self.Frame = float32(ZOMBIE_FRAME_painc12); Self.NextThink = Time + 0.1; Self.Think = zombie_painc13; ai_painforward(1) }
func zombie_painc13() { Self.Frame = float32(ZOMBIE_FRAME_painc13); Self.NextThink = Time + 0.1; Self.Think = zombie_painc14 }
func zombie_painc14() { Self.Frame = float32(ZOMBIE_FRAME_painc14); Self.NextThink = Time + 0.1; Self.Think = zombie_painc15 }
func zombie_painc15() { Self.Frame = float32(ZOMBIE_FRAME_painc15); Self.NextThink = Time + 0.1; Self.Think = zombie_painc16 }
func zombie_painc16() { Self.Frame = float32(ZOMBIE_FRAME_painc16); Self.NextThink = Time + 0.1; Self.Think = zombie_painc17 }
func zombie_painc17() { Self.Frame = float32(ZOMBIE_FRAME_painc17); Self.NextThink = Time + 0.1; Self.Think = zombie_painc18 }
func zombie_painc18() { Self.Frame = float32(ZOMBIE_FRAME_painc18); Self.NextThink = Time + 0.1; Self.Think = zombie_run1_impl }

func zombie_paind1() { Self.Frame = float32(ZOMBIE_FRAME_paind1); Self.NextThink = Time + 0.1; Self.Think = zombie_paind2; engine.Sound(Self, int(CHAN_VOICE), "zombie/z_pain.wav", 1, ATTN_NORM) }
func zombie_paind2() { Self.Frame = float32(ZOMBIE_FRAME_paind2); Self.NextThink = Time + 0.1; Self.Think = zombie_paind3 }
func zombie_paind3() { Self.Frame = float32(ZOMBIE_FRAME_paind3); Self.NextThink = Time + 0.1; Self.Think = zombie_paind4 }
func zombie_paind4() { Self.Frame = float32(ZOMBIE_FRAME_paind4); Self.NextThink = Time + 0.1; Self.Think = zombie_paind5 }
func zombie_paind5() { Self.Frame = float32(ZOMBIE_FRAME_paind5); Self.NextThink = Time + 0.1; Self.Think = zombie_paind6 }
func zombie_paind6() { Self.Frame = float32(ZOMBIE_FRAME_paind6); Self.NextThink = Time + 0.1; Self.Think = zombie_paind7 }
func zombie_paind7() { Self.Frame = float32(ZOMBIE_FRAME_paind7); Self.NextThink = Time + 0.1; Self.Think = zombie_paind8 }
func zombie_paind8() { Self.Frame = float32(ZOMBIE_FRAME_paind8); Self.NextThink = Time + 0.1; Self.Think = zombie_paind9 }
func zombie_paind9() { Self.Frame = float32(ZOMBIE_FRAME_paind9); Self.NextThink = Time + 0.1; Self.Think = zombie_paind10; ai_pain(1) }
func zombie_paind10() { Self.Frame = float32(ZOMBIE_FRAME_paind10); Self.NextThink = Time + 0.1; Self.Think = zombie_paind11 }
func zombie_paind11() { Self.Frame = float32(ZOMBIE_FRAME_paind11); Self.NextThink = Time + 0.1; Self.Think = zombie_paind12 }
func zombie_paind12() { Self.Frame = float32(ZOMBIE_FRAME_paind12); Self.NextThink = Time + 0.1; Self.Think = zombie_paind13 }
func zombie_paind13() { Self.Frame = float32(ZOMBIE_FRAME_paind13); Self.NextThink = Time + 0.1; Self.Think = zombie_run1_impl }

func zombie_paine1() {
	Self.Frame = float32(ZOMBIE_FRAME_paine1)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_paine2
	engine.Sound(Self, int(CHAN_VOICE), "zombie/z_pain.wav", 1, ATTN_NORM)
	Self.Health = 60
}
func zombie_paine2() { Self.Frame = float32(ZOMBIE_FRAME_paine2); Self.NextThink = Time + 0.1; Self.Think = zombie_paine3; ai_pain(8) }
func zombie_paine3() { Self.Frame = float32(ZOMBIE_FRAME_paine3); Self.NextThink = Time + 0.1; Self.Think = zombie_paine4; ai_pain(5) }
func zombie_paine4() { Self.Frame = float32(ZOMBIE_FRAME_paine4); Self.NextThink = Time + 0.1; Self.Think = zombie_paine5; ai_pain(3) }
func zombie_paine5() { Self.Frame = float32(ZOMBIE_FRAME_paine5); Self.NextThink = Time + 0.1; Self.Think = zombie_paine6; ai_pain(1) }
func zombie_paine6() { Self.Frame = float32(ZOMBIE_FRAME_paine6); Self.NextThink = Time + 0.1; Self.Think = zombie_paine7; ai_pain(2) }
func zombie_paine7() { Self.Frame = float32(ZOMBIE_FRAME_paine7); Self.NextThink = Time + 0.1; Self.Think = zombie_paine8; ai_pain(1) }
func zombie_paine8() { Self.Frame = float32(ZOMBIE_FRAME_paine8); Self.NextThink = Time + 0.1; Self.Think = zombie_paine9; ai_pain(1) }
func zombie_paine9() { Self.Frame = float32(ZOMBIE_FRAME_paine9); Self.NextThink = Time + 0.1; Self.Think = zombie_paine10; ai_pain(2) }
func zombie_paine10() {
	Self.Frame = float32(ZOMBIE_FRAME_paine10)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_paine11
	engine.Sound(Self, int(CHAN_BODY), "zombie/z_fall.wav", 1, ATTN_NORM)
	Self.Solid = float32(SOLID_NOT)
}
func zombie_paine11() {
	Self.Frame = float32(ZOMBIE_FRAME_paine11)
	Self.NextThink = Time + 5
	Self.Think = zombie_paine12
	Self.Health = 60
}
func zombie_paine12() {
	Self.Frame = float32(ZOMBIE_FRAME_paine12)
	Self.NextThink = Time + 0.1
	Self.Think = zombie_paine13
	Self.Health = 60
	engine.Sound(Self, int(CHAN_VOICE), "zombie/z_idle.wav", 1, ATTN_IDLE)
	Self.Solid = float32(SOLID_SLIDEBOX)

	if engine.WalkMove(0, 0) == 0 {
		Self.Think = zombie_paine11
		Self.Solid = float32(SOLID_NOT)
		return
	}
}
func zombie_paine13() { Self.Frame = float32(ZOMBIE_FRAME_paine13); Self.NextThink = Time + 0.1; Self.Think = zombie_paine14 }
func zombie_paine14() { Self.Frame = float32(ZOMBIE_FRAME_paine14); Self.NextThink = Time + 0.1; Self.Think = zombie_paine15 }
func zombie_paine15() { Self.Frame = float32(ZOMBIE_FRAME_paine15); Self.NextThink = Time + 0.1; Self.Think = zombie_paine16 }
func zombie_paine16() { Self.Frame = float32(ZOMBIE_FRAME_paine16); Self.NextThink = Time + 0.1; Self.Think = zombie_paine17 }
func zombie_paine17() { Self.Frame = float32(ZOMBIE_FRAME_paine17); Self.NextThink = Time + 0.1; Self.Think = zombie_paine18 }
func zombie_paine18() { Self.Frame = float32(ZOMBIE_FRAME_paine18); Self.NextThink = Time + 0.1; Self.Think = zombie_paine19 }
func zombie_paine19() { Self.Frame = float32(ZOMBIE_FRAME_paine19); Self.NextThink = Time + 0.1; Self.Think = zombie_paine20 }
func zombie_paine20() { Self.Frame = float32(ZOMBIE_FRAME_paine20); Self.NextThink = Time + 0.1; Self.Think = zombie_paine21 }
func zombie_paine21() { Self.Frame = float32(ZOMBIE_FRAME_paine21); Self.NextThink = Time + 0.1; Self.Think = zombie_paine22 }
func zombie_paine22() { Self.Frame = float32(ZOMBIE_FRAME_paine22); Self.NextThink = Time + 0.1; Self.Think = zombie_paine23 }
func zombie_paine23() { Self.Frame = float32(ZOMBIE_FRAME_paine23); Self.NextThink = Time + 0.1; Self.Think = zombie_paine24 }
func zombie_paine24() { Self.Frame = float32(ZOMBIE_FRAME_paine24); Self.NextThink = Time + 0.1; Self.Think = zombie_paine25 }
func zombie_paine25() { Self.Frame = float32(ZOMBIE_FRAME_paine25); Self.NextThink = Time + 0.1; Self.Think = zombie_paine26; ai_painforward(5) }
func zombie_paine26() { Self.Frame = float32(ZOMBIE_FRAME_paine26); Self.NextThink = Time + 0.1; Self.Think = zombie_paine27; ai_painforward(3) }
func zombie_paine27() { Self.Frame = float32(ZOMBIE_FRAME_paine27); Self.NextThink = Time + 0.1; Self.Think = zombie_paine28; ai_painforward(1) }
func zombie_paine28() { Self.Frame = float32(ZOMBIE_FRAME_paine28); Self.NextThink = Time + 0.1; Self.Think = zombie_paine29; ai_pain(1) }
func zombie_paine29() { Self.Frame = float32(ZOMBIE_FRAME_paine29); Self.NextThink = Time + 0.1; Self.Think = zombie_paine30 }
func zombie_paine30() { Self.Frame = float32(ZOMBIE_FRAME_paine30); Self.NextThink = Time + 0.1; Self.Think = zombie_run1_impl }

func zombie_die() {
	engine.Sound(Self, int(CHAN_VOICE), "zombie/z_gib.wav", 1, ATTN_NORM)
	ThrowHead("progs/h_zombie.mdl", Self.Health)
	ThrowGib("progs/gib1.mdl", Self.Health)
	ThrowGib("progs/gib2.mdl", Self.Health)
	ThrowGib("progs/gib3.mdl", Self.Health)
}

func zombie_pain(attacker *quake.Entity, take float32) {
	var r float32

	Self.Health = 60 // always reset health

	if take < 9 {
		return // totally ignore
	}

	if Self.InPain == 2 {
		return // down on ground, so don't reset any counters
	}

	if take >= 25 {
		Self.InPain = 2
		zombie_paine1()
		return
	}

	if Self.InPain != 0 {
		Self.PainFinished = Time + 3
		return // currently going through an animation, don't change
	}

	if Self.PainFinished > Time {
		Self.InPain = 2
		zombie_paine1()
		return
	}

	Self.InPain = 1

	r = engine.Random()
	if r < 0.25 {
		zombie_paina1()
	} else if r < 0.5 {
		zombie_painb1()
	} else if r < 0.75 {
		zombie_painc1()
	} else {
		zombie_paind1()
	}
}

func monster_zombie() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel("progs/zombie.mdl")
	engine.PrecacheModel("progs/h_zombie.mdl")
	engine.PrecacheModel("progs/zom_gib.mdl")

	engine.PrecacheSound("zombie/z_idle.wav")
	engine.PrecacheSound("zombie/z_idle1.wav")
	engine.PrecacheSound("zombie/z_shot1.wav")
	engine.PrecacheSound("zombie/z_gib.wav")
	engine.PrecacheSound("zombie/z_pain.wav")
	engine.PrecacheSound("zombie/z_pain1.wav")
	engine.PrecacheSound("zombie/z_fall.wav")
	engine.PrecacheSound("zombie/z_miss.wav")
	engine.PrecacheSound("zombie/z_hit.wav")
	engine.PrecacheSound("zombie/idle_w2.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/zombie.mdl")

	Self.Noise = "zombie/z_idle.wav"
	Self.NetName = "$qc_zombie"
	Self.KillString = "$qc_ks_zombie"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 40))
	Self.Health = 60
	Self.MaxHealth = 60

	Self.ThStand = zombie_stand1
	Self.ThWalk = zombie_walk1
	Self.ThRun = zombie_run1_impl
	Self.ThPain = zombie_pain
	Self.ThDie = zombie_die
	Self.ThMissile = zombie_missile
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_RANGED

	if (int(Self.SpawnFlags) & int(SPAWN_CRUCIFIED)) != 0 {
		Self.MoveType = MOVETYPE_NONE
		zombie_cruc1()
	} else {
		walkmonster_start()
	}
}
