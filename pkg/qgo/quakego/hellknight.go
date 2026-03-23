package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Hell Knight frames
	HKNIGHT_FRAME_stand1 = iota
	HKNIGHT_FRAME_stand2
	HKNIGHT_FRAME_stand3
	HKNIGHT_FRAME_stand4
	HKNIGHT_FRAME_stand5
	HKNIGHT_FRAME_stand6
	HKNIGHT_FRAME_stand7
	HKNIGHT_FRAME_stand8
	HKNIGHT_FRAME_stand9

	HKNIGHT_FRAME_walk1
	HKNIGHT_FRAME_walk2
	HKNIGHT_FRAME_walk3
	HKNIGHT_FRAME_walk4
	HKNIGHT_FRAME_walk5
	HKNIGHT_FRAME_walk6
	HKNIGHT_FRAME_walk7
	HKNIGHT_FRAME_walk8
	HKNIGHT_FRAME_walk9
	HKNIGHT_FRAME_walk10
	HKNIGHT_FRAME_walk11
	HKNIGHT_FRAME_walk12
	HKNIGHT_FRAME_walk13
	HKNIGHT_FRAME_walk14
	HKNIGHT_FRAME_walk15
	HKNIGHT_FRAME_walk16
	HKNIGHT_FRAME_walk17
	HKNIGHT_FRAME_walk18
	HKNIGHT_FRAME_walk19
	HKNIGHT_FRAME_walk20

	HKNIGHT_FRAME_run1
	HKNIGHT_FRAME_run2
	HKNIGHT_FRAME_run3
	HKNIGHT_FRAME_run4
	HKNIGHT_FRAME_run5
	HKNIGHT_FRAME_run6
	HKNIGHT_FRAME_run7
	HKNIGHT_FRAME_run8

	HKNIGHT_FRAME_pain1
	HKNIGHT_FRAME_pain2
	HKNIGHT_FRAME_pain3
	HKNIGHT_FRAME_pain4
	HKNIGHT_FRAME_pain5

	HKNIGHT_FRAME_death1
	HKNIGHT_FRAME_death2
	HKNIGHT_FRAME_death3
	HKNIGHT_FRAME_death4
	HKNIGHT_FRAME_death5
	HKNIGHT_FRAME_death6
	HKNIGHT_FRAME_death7
	HKNIGHT_FRAME_death8
	HKNIGHT_FRAME_death9
	HKNIGHT_FRAME_death10
	HKNIGHT_FRAME_death11
	HKNIGHT_FRAME_death12

	HKNIGHT_FRAME_deathb1
	HKNIGHT_FRAME_deathb2
	HKNIGHT_FRAME_deathb3
	HKNIGHT_FRAME_deathb4
	HKNIGHT_FRAME_deathb5
	HKNIGHT_FRAME_deathb6
	HKNIGHT_FRAME_deathb7
	HKNIGHT_FRAME_deathb8
	HKNIGHT_FRAME_deathb9

	HKNIGHT_FRAME_char_a1
	HKNIGHT_FRAME_char_a2
	HKNIGHT_FRAME_char_a3
	HKNIGHT_FRAME_char_a4
	HKNIGHT_FRAME_char_a5
	HKNIGHT_FRAME_char_a6
	HKNIGHT_FRAME_char_a7
	HKNIGHT_FRAME_char_a8
	HKNIGHT_FRAME_char_a9
	HKNIGHT_FRAME_char_a10
	HKNIGHT_FRAME_char_a11
	HKNIGHT_FRAME_char_a12
	HKNIGHT_FRAME_char_a13
	HKNIGHT_FRAME_char_a14
	HKNIGHT_FRAME_char_a15
	HKNIGHT_FRAME_char_a16

	HKNIGHT_FRAME_magica1
	HKNIGHT_FRAME_magica2
	HKNIGHT_FRAME_magica3
	HKNIGHT_FRAME_magica4
	HKNIGHT_FRAME_magica5
	HKNIGHT_FRAME_magica6
	HKNIGHT_FRAME_magica7
	HKNIGHT_FRAME_magica8
	HKNIGHT_FRAME_magica9
	HKNIGHT_FRAME_magica10
	HKNIGHT_FRAME_magica11
	HKNIGHT_FRAME_magica12
	HKNIGHT_FRAME_magica13
	HKNIGHT_FRAME_magica14

	HKNIGHT_FRAME_magicb1
	HKNIGHT_FRAME_magicb2
	HKNIGHT_FRAME_magicb3
	HKNIGHT_FRAME_magicb4
	HKNIGHT_FRAME_magicb5
	HKNIGHT_FRAME_magicb6
	HKNIGHT_FRAME_magicb7
	HKNIGHT_FRAME_magicb8
	HKNIGHT_FRAME_magicb9
	HKNIGHT_FRAME_magicb10
	HKNIGHT_FRAME_magicb11
	HKNIGHT_FRAME_magicb12
	HKNIGHT_FRAME_magicb13

	HKNIGHT_FRAME_char_b1
	HKNIGHT_FRAME_char_b2
	HKNIGHT_FRAME_char_b3
	HKNIGHT_FRAME_char_b4
	HKNIGHT_FRAME_char_b5
	HKNIGHT_FRAME_char_b6

	HKNIGHT_FRAME_slice1
	HKNIGHT_FRAME_slice2
	HKNIGHT_FRAME_slice3
	HKNIGHT_FRAME_slice4
	HKNIGHT_FRAME_slice5
	HKNIGHT_FRAME_slice6
	HKNIGHT_FRAME_slice7
	HKNIGHT_FRAME_slice8
	HKNIGHT_FRAME_slice9
	HKNIGHT_FRAME_slice10

	HKNIGHT_FRAME_smash1
	HKNIGHT_FRAME_smash2
	HKNIGHT_FRAME_smash3
	HKNIGHT_FRAME_smash4
	HKNIGHT_FRAME_smash5
	HKNIGHT_FRAME_smash6
	HKNIGHT_FRAME_smash7
	HKNIGHT_FRAME_smash8
	HKNIGHT_FRAME_smash9
	HKNIGHT_FRAME_smash10
	HKNIGHT_FRAME_smash11

	HKNIGHT_FRAME_w_attack1
	HKNIGHT_FRAME_w_attack2
	HKNIGHT_FRAME_w_attack3
	HKNIGHT_FRAME_w_attack4
	HKNIGHT_FRAME_w_attack5
	HKNIGHT_FRAME_w_attack6
	HKNIGHT_FRAME_w_attack7
	HKNIGHT_FRAME_w_attack8
	HKNIGHT_FRAME_w_attack9
	HKNIGHT_FRAME_w_attack10
	HKNIGHT_FRAME_w_attack11
	HKNIGHT_FRAME_w_attack12
	HKNIGHT_FRAME_w_attack13
	HKNIGHT_FRAME_w_attack14
	HKNIGHT_FRAME_w_attack15
	HKNIGHT_FRAME_w_attack16
	HKNIGHT_FRAME_w_attack17
	HKNIGHT_FRAME_w_attack18
	HKNIGHT_FRAME_w_attack19
	HKNIGHT_FRAME_w_attack20
	HKNIGHT_FRAME_w_attack21
	HKNIGHT_FRAME_w_attack22

	HKNIGHT_FRAME_magicc1
	HKNIGHT_FRAME_magicc2
	HKNIGHT_FRAME_magicc3
	HKNIGHT_FRAME_magicc4
	HKNIGHT_FRAME_magicc5
	HKNIGHT_FRAME_magicc6
	HKNIGHT_FRAME_magicc7
	HKNIGHT_FRAME_magicc8
	HKNIGHT_FRAME_magicc9
	HKNIGHT_FRAME_magicc10
	HKNIGHT_FRAME_magicc11
)

var hknight_type float32

func hk_idle_sound() {
	if engine.Random() < 0.2 {
		engine.Sound(Self, int(CHAN_VOICE), "hknight/idle.wav", 1, ATTN_NORM)
	}
}

func hknight_shot(offset float32) {
	var offang quake.Vec3
	var org, vec quake.Vec3

	offang = engine.VectoAngles(Self.Enemy.Origin.Sub(Self.Origin))
	offang[1] = offang[1] + offset*6

	engine.MakeVectors(offang)

	org = Self.Origin.Add(Self.Mins).Add(Self.Size.Mul(0.5)).Add(VForward.Mul(20))

	vec = engine.Normalize(VForward)
	vec[2] = 0 - vec[2] + (engine.Random()-0.5)*0.1

	launch_spike(org, vec)
	Newmis.ClassName = "knight_spike"
	engine.SetModel(Newmis, "progs/k_spike.mdl")
	engine.SetSize(Newmis, VEC_ORIGIN, VEC_ORIGIN)
	Newmis.Velocity = vec.Mul(300)
	if engine.Cvar("pr_checkextension") != 0 {
		if engine.CheckExtension("EX_EXTENDED_EF") != 0 {
			Newmis.Effects = float32(int(Newmis.Effects) | EF_CANDLELIGHT)
		}
	}
	engine.Sound(Self, int(CHAN_WEAPON), "hknight/attack1.wav", 1, ATTN_NORM)
}

func CheckForCharge() {
	if EnemyVisible == 0 {
		return
	}

	if Time < Self.AttackFinished {
		return
	}

	if engine.FAbs(Self.Origin[2]-Self.Enemy.Origin[2]) > 20 {
		return
	}

	if engine.Vlen(Self.Origin.Sub(Self.Enemy.Origin)) < 80 {
		return
	}

	SUB_AttackFinished(2)
	hknight_char_a1()
}

func CheckContinueCharge() {
	if Time > Self.AttackFinished {
		SUB_AttackFinished(3)
		hknight_run1()
		return
	}

	if engine.Random() > 0.5 {
		engine.Sound(Self, int(CHAN_WEAPON), "knight/sword2.wav", 1, ATTN_NORM)
	} else {
		engine.Sound(Self, int(CHAN_WEAPON), "knight/sword1.wav", 1, ATTN_NORM)
	}
}

func hknight_stand1() { Self.Frame = float32(HKNIGHT_FRAME_stand1); Self.NextThink = Time + 0.1; Self.Think = hknight_stand2; ai_stand() }
func hknight_stand2() { Self.Frame = float32(HKNIGHT_FRAME_stand2); Self.NextThink = Time + 0.1; Self.Think = hknight_stand3; ai_stand() }
func hknight_stand3() { Self.Frame = float32(HKNIGHT_FRAME_stand3); Self.NextThink = Time + 0.1; Self.Think = hknight_stand4; ai_stand() }
func hknight_stand4() { Self.Frame = float32(HKNIGHT_FRAME_stand4); Self.NextThink = Time + 0.1; Self.Think = hknight_stand5; ai_stand() }
func hknight_stand5() { Self.Frame = float32(HKNIGHT_FRAME_stand5); Self.NextThink = Time + 0.1; Self.Think = hknight_stand6; ai_stand() }
func hknight_stand6() { Self.Frame = float32(HKNIGHT_FRAME_stand6); Self.NextThink = Time + 0.1; Self.Think = hknight_stand7; ai_stand() }
func hknight_stand7() { Self.Frame = float32(HKNIGHT_FRAME_stand7); Self.NextThink = Time + 0.1; Self.Think = hknight_stand8; ai_stand() }
func hknight_stand8() { Self.Frame = float32(HKNIGHT_FRAME_stand8); Self.NextThink = Time + 0.1; Self.Think = hknight_stand9; ai_stand() }
func hknight_stand9() { Self.Frame = float32(HKNIGHT_FRAME_stand9); Self.NextThink = Time + 0.1; Self.Think = hknight_stand1; ai_stand() }

func hknight_walk1() {
	Self.Frame = float32(HKNIGHT_FRAME_walk1)
	Self.NextThink = Time + 0.1
	Self.Think = hknight_walk2
	hk_idle_sound()
	ai_walk(2)
}
func hknight_walk2()  { Self.Frame = float32(HKNIGHT_FRAME_walk2); Self.NextThink = Time + 0.1; Self.Think = hknight_walk3; ai_walk(5) }
func hknight_walk3()  { Self.Frame = float32(HKNIGHT_FRAME_walk3); Self.NextThink = Time + 0.1; Self.Think = hknight_walk4; ai_walk(5) }
func hknight_walk4()  { Self.Frame = float32(HKNIGHT_FRAME_walk4); Self.NextThink = Time + 0.1; Self.Think = hknight_walk5; ai_walk(4) }
func hknight_walk5()  { Self.Frame = float32(HKNIGHT_FRAME_walk5); Self.NextThink = Time + 0.1; Self.Think = hknight_walk6; ai_walk(4) }
func hknight_walk6()  { Self.Frame = float32(HKNIGHT_FRAME_walk6); Self.NextThink = Time + 0.1; Self.Think = hknight_walk7; ai_walk(2) }
func hknight_walk7()  { Self.Frame = float32(HKNIGHT_FRAME_walk7); Self.NextThink = Time + 0.1; Self.Think = hknight_walk8; ai_walk(2) }
func hknight_walk8()  { Self.Frame = float32(HKNIGHT_FRAME_walk8); Self.NextThink = Time + 0.1; Self.Think = hknight_walk9; ai_walk(3) }
func hknight_walk9()  { Self.Frame = float32(HKNIGHT_FRAME_walk9); Self.NextThink = Time + 0.1; Self.Think = hknight_walk10; ai_walk(3) }
func hknight_walk10() { Self.Frame = float32(HKNIGHT_FRAME_walk10); Self.NextThink = Time + 0.1; Self.Think = hknight_walk11; ai_walk(4) }
func hknight_walk11() { Self.Frame = float32(HKNIGHT_FRAME_walk11); Self.NextThink = Time + 0.1; Self.Think = hknight_walk12; ai_walk(3) }
func hknight_walk12() { Self.Frame = float32(HKNIGHT_FRAME_walk12); Self.NextThink = Time + 0.1; Self.Think = hknight_walk13; ai_walk(4) }
func hknight_walk13() { Self.Frame = float32(HKNIGHT_FRAME_walk13); Self.NextThink = Time + 0.1; Self.Think = hknight_walk14; ai_walk(6) }
func hknight_walk14() { Self.Frame = float32(HKNIGHT_FRAME_walk14); Self.NextThink = Time + 0.1; Self.Think = hknight_walk15; ai_walk(2) }
func hknight_walk15() { Self.Frame = float32(HKNIGHT_FRAME_walk15); Self.NextThink = Time + 0.1; Self.Think = hknight_walk16; ai_walk(2) }
func hknight_walk16() { Self.Frame = float32(HKNIGHT_FRAME_walk16); Self.NextThink = Time + 0.1; Self.Think = hknight_walk17; ai_walk(4) }
func hknight_walk17() { Self.Frame = float32(HKNIGHT_FRAME_walk17); Self.NextThink = Time + 0.1; Self.Think = hknight_walk18; ai_walk(3) }
func hknight_walk18() { Self.Frame = float32(HKNIGHT_FRAME_walk18); Self.NextThink = Time + 0.1; Self.Think = hknight_walk19; ai_walk(3) }
func hknight_walk19() { Self.Frame = float32(HKNIGHT_FRAME_walk19); Self.NextThink = Time + 0.1; Self.Think = hknight_walk20; ai_walk(3) }
func hknight_walk20() { Self.Frame = float32(HKNIGHT_FRAME_walk20); Self.NextThink = Time + 0.1; Self.Think = hknight_walk1; ai_walk(2) }

func hknight_run1() {
	Self.Frame = float32(HKNIGHT_FRAME_run1)
	Self.NextThink = Time + 0.1
	Self.Think = hknight_run2
	hk_idle_sound()
	ai_run(20)
	CheckForCharge()
}
func hknight_run2() { Self.Frame = float32(HKNIGHT_FRAME_run2); Self.NextThink = Time + 0.1; Self.Think = hknight_run3; ai_run(25) }
func hknight_run3() { Self.Frame = float32(HKNIGHT_FRAME_run3); Self.NextThink = Time + 0.1; Self.Think = hknight_run4; ai_run(18) }
func hknight_run4() { Self.Frame = float32(HKNIGHT_FRAME_run4); Self.NextThink = Time + 0.1; Self.Think = hknight_run5; ai_run(16) }
func hknight_run5() { Self.Frame = float32(HKNIGHT_FRAME_run5); Self.NextThink = Time + 0.1; Self.Think = hknight_run6; ai_run(14) }
func hknight_run6() { Self.Frame = float32(HKNIGHT_FRAME_run6); Self.NextThink = Time + 0.1; Self.Think = hknight_run7; ai_run(25) }
func hknight_run7() { Self.Frame = float32(HKNIGHT_FRAME_run7); Self.NextThink = Time + 0.1; Self.Think = hknight_run8; ai_run(21) }
func hknight_run8() { Self.Frame = float32(HKNIGHT_FRAME_run8); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; ai_run(13) }

func hknight_pain1() {
	Self.Frame = float32(HKNIGHT_FRAME_pain1)
	Self.NextThink = Time + 0.1
	Self.Think = hknight_pain2
	engine.Sound(Self, int(CHAN_VOICE), "hknight/pain1.wav", 1, ATTN_NORM)
}
func hknight_pain2() { Self.Frame = float32(HKNIGHT_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = hknight_pain3 }
func hknight_pain3() { Self.Frame = float32(HKNIGHT_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = hknight_pain4 }
func hknight_pain4() { Self.Frame = float32(HKNIGHT_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = hknight_pain5 }
func hknight_pain5() { Self.Frame = float32(HKNIGHT_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = hknight_run1 }

func hknight_die1() { Self.Frame = float32(HKNIGHT_FRAME_death1); Self.NextThink = Time + 0.1; Self.Think = hknight_die2; ai_forward(10) }
func hknight_die2() { Self.Frame = float32(HKNIGHT_FRAME_death2); Self.NextThink = Time + 0.1; Self.Think = hknight_die3; ai_forward(8) }
func hknight_die3() {
	Self.Frame = float32(HKNIGHT_FRAME_death3)
	Self.NextThink = Time + 0.1
	Self.Think = hknight_die4
	Self.Solid = SOLID_NOT
	ai_forward(7)
}
func hknight_die4()  { Self.Frame = float32(HKNIGHT_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = hknight_die5 }
func hknight_die5()  { Self.Frame = float32(HKNIGHT_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = hknight_die6 }
func hknight_die6()  { Self.Frame = float32(HKNIGHT_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = hknight_die7 }
func hknight_die7()  { Self.Frame = float32(HKNIGHT_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = hknight_die8 }
func hknight_die8()  { Self.Frame = float32(HKNIGHT_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = hknight_die9; ai_forward(10) }
func hknight_die9()  { Self.Frame = float32(HKNIGHT_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = hknight_die10; ai_forward(11) }
func hknight_die10() { Self.Frame = float32(HKNIGHT_FRAME_death10); Self.NextThink = Time + 0.1; Self.Think = hknight_die11 }
func hknight_die11() { Self.Frame = float32(HKNIGHT_FRAME_death11); Self.NextThink = Time + 0.1; Self.Think = hknight_die12 }
func hknight_die12() { Self.Frame = float32(HKNIGHT_FRAME_death12); Self.NextThink = Time + 0.1; Self.Think = hknight_die12 }

func hknight_dieb1() { Self.Frame = float32(HKNIGHT_FRAME_deathb1); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb2 }
func hknight_dieb2() { Self.Frame = float32(HKNIGHT_FRAME_deathb2); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb3 }
func hknight_dieb3() {
	Self.Frame = float32(HKNIGHT_FRAME_deathb3)
	Self.NextThink = Time + 0.1
	Self.Think = hknight_dieb4
	Self.Solid = SOLID_NOT
}
func hknight_dieb4() { Self.Frame = float32(HKNIGHT_FRAME_deathb4); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb5 }
func hknight_dieb5() { Self.Frame = float32(HKNIGHT_FRAME_deathb5); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb6 }
func hknight_dieb6() { Self.Frame = float32(HKNIGHT_FRAME_deathb6); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb7 }
func hknight_dieb7() { Self.Frame = float32(HKNIGHT_FRAME_deathb7); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb8 }
func hknight_dieb8() { Self.Frame = float32(HKNIGHT_FRAME_deathb8); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb9 }
func hknight_dieb9() { Self.Frame = float32(HKNIGHT_FRAME_deathb9); Self.NextThink = Time + 0.1; Self.Think = hknight_dieb9 }

func hknight_die() {
	if Self.Health < -40 {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NORM)
		ThrowHead("progs/h_hellkn.mdl", Self.Health)
		ThrowGib("progs/gib1.mdl", Self.Health)
		ThrowGib("progs/gib2.mdl", Self.Health)
		ThrowGib("progs/gib3.mdl", Self.Health)
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "hknight/death1.wav", 1, ATTN_NORM)

	if engine.Random() > 0.5 {
		hknight_die1()
	} else {
		hknight_dieb1()
	}
}

func hknight_magica1()  { Self.Frame = float32(HKNIGHT_FRAME_magica1); Self.NextThink = Time + 0.1; Self.Think = hknight_magica2; ai_face() }
func hknight_magica2()  { Self.Frame = float32(HKNIGHT_FRAME_magica2); Self.NextThink = Time + 0.1; Self.Think = hknight_magica3; ai_face() }
func hknight_magica3()  { Self.Frame = float32(HKNIGHT_FRAME_magica3); Self.NextThink = Time + 0.1; Self.Think = hknight_magica4; ai_face() }
func hknight_magica4()  { Self.Frame = float32(HKNIGHT_FRAME_magica4); Self.NextThink = Time + 0.1; Self.Think = hknight_magica5; ai_face() }
func hknight_magica5()  { Self.Frame = float32(HKNIGHT_FRAME_magica5); Self.NextThink = Time + 0.1; Self.Think = hknight_magica6; ai_face() }
func hknight_magica6()  { Self.Frame = float32(HKNIGHT_FRAME_magica6); Self.NextThink = Time + 0.1; Self.Think = hknight_magica7; ai_face() }
func hknight_magica7()  { Self.Frame = float32(HKNIGHT_FRAME_magica7); Self.NextThink = Time + 0.1; Self.Think = hknight_magica8; hknight_shot(-2) }
func hknight_magica8()  { Self.Frame = float32(HKNIGHT_FRAME_magica8); Self.NextThink = Time + 0.1; Self.Think = hknight_magica9; hknight_shot(-1) }
func hknight_magica9()  { Self.Frame = float32(HKNIGHT_FRAME_magica9); Self.NextThink = Time + 0.1; Self.Think = hknight_magica10; hknight_shot(0) }
func hknight_magica10() { Self.Frame = float32(HKNIGHT_FRAME_magica10); Self.NextThink = Time + 0.1; Self.Think = hknight_magica11; hknight_shot(1) }
func hknight_magica11() { Self.Frame = float32(HKNIGHT_FRAME_magica11); Self.NextThink = Time + 0.1; Self.Think = hknight_magica12; hknight_shot(2) }
func hknight_magica12() { Self.Frame = float32(HKNIGHT_FRAME_magica12); Self.NextThink = Time + 0.1; Self.Think = hknight_magica13; hknight_shot(3) }
func hknight_magica13() { Self.Frame = float32(HKNIGHT_FRAME_magica13); Self.NextThink = Time + 0.1; Self.Think = hknight_magica14; ai_face() }
func hknight_magica14() { Self.Frame = float32(HKNIGHT_FRAME_magica14); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; ai_face() }

func hknight_magicb1()  { Self.Frame = float32(HKNIGHT_FRAME_magicb1); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb2; ai_face() }
func hknight_magicb2()  { Self.Frame = float32(HKNIGHT_FRAME_magicb2); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb3; ai_face() }
func hknight_magicb3()  { Self.Frame = float32(HKNIGHT_FRAME_magicb3); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb4; ai_face() }
func hknight_magicb4()  { Self.Frame = float32(HKNIGHT_FRAME_magicb4); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb5; ai_face() }
func hknight_magicb5()  { Self.Frame = float32(HKNIGHT_FRAME_magicb5); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb6; ai_face() }
func hknight_magicb6()  { Self.Frame = float32(HKNIGHT_FRAME_magicb6); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb7; ai_face() }
func hknight_magicb7()  { Self.Frame = float32(HKNIGHT_FRAME_magicb7); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb8; hknight_shot(-2) }
func hknight_magicb8()  { Self.Frame = float32(HKNIGHT_FRAME_magicb8); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb9; hknight_shot(-1) }
func hknight_magicb9()  { Self.Frame = float32(HKNIGHT_FRAME_magicb9); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb10; hknight_shot(0) }
func hknight_magicb10() { Self.Frame = float32(HKNIGHT_FRAME_magicb10); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb11; hknight_shot(1) }
func hknight_magicb11() { Self.Frame = float32(HKNIGHT_FRAME_magicb11); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb12; hknight_shot(2) }
func hknight_magicb12() { Self.Frame = float32(HKNIGHT_FRAME_magicb12); Self.NextThink = Time + 0.1; Self.Think = hknight_magicb13; hknight_shot(3) }
func hknight_magicb13() { Self.Frame = float32(HKNIGHT_FRAME_magicb13); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; ai_face() }

func hknight_magicc1()  { Self.Frame = float32(HKNIGHT_FRAME_magicc1); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc2; ai_face() }
func hknight_magicc2()  { Self.Frame = float32(HKNIGHT_FRAME_magicc2); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc3; ai_face() }
func hknight_magicc3()  { Self.Frame = float32(HKNIGHT_FRAME_magicc3); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc4; ai_face() }
func hknight_magicc4()  { Self.Frame = float32(HKNIGHT_FRAME_magicc4); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc5; ai_face() }
func hknight_magicc5()  { Self.Frame = float32(HKNIGHT_FRAME_magicc5); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc6; ai_face() }
func hknight_magicc6()  { Self.Frame = float32(HKNIGHT_FRAME_magicc6); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc7; hknight_shot(-2) }
func hknight_magicc7()  { Self.Frame = float32(HKNIGHT_FRAME_magicc7); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc8; hknight_shot(-1) }
func hknight_magicc8()  { Self.Frame = float32(HKNIGHT_FRAME_magicc8); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc9; hknight_shot(0) }
func hknight_magicc9()  { Self.Frame = float32(HKNIGHT_FRAME_magicc9); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc10; hknight_shot(1) }
func hknight_magicc10() { Self.Frame = float32(HKNIGHT_FRAME_magicc10); Self.NextThink = Time + 0.1; Self.Think = hknight_magicc11; hknight_shot(2) }
func hknight_magicc11() { Self.Frame = float32(HKNIGHT_FRAME_magicc11); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; hknight_shot(3) }

func hknight_char_a1() { Self.Frame = float32(HKNIGHT_FRAME_char_a1); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a2; ai_charge(20) }
func hknight_char_a2() { Self.Frame = float32(HKNIGHT_FRAME_char_a2); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a3; ai_charge(25) }
func hknight_char_a3() { Self.Frame = float32(HKNIGHT_FRAME_char_a3); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a4; ai_charge(18) }
func hknight_char_a4() { Self.Frame = float32(HKNIGHT_FRAME_char_a4); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a5; ai_charge(16) }
func hknight_char_a5() { Self.Frame = float32(HKNIGHT_FRAME_char_a5); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a6; ai_charge(14) }
func hknight_char_a6() { Self.Frame = float32(HKNIGHT_FRAME_char_a6); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a7; ai_charge(20); ai_melee() }
func hknight_char_a7() { Self.Frame = float32(HKNIGHT_FRAME_char_a7); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a8; ai_charge(21); ai_melee() }
func hknight_char_a8() { Self.Frame = float32(HKNIGHT_FRAME_char_a8); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a9; ai_charge(13); ai_melee() }
func hknight_char_a9() { Self.Frame = float32(HKNIGHT_FRAME_char_a9); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a10; ai_charge(20); ai_melee() }
func hknight_char_a10() { Self.Frame = float32(HKNIGHT_FRAME_char_a10); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a11; ai_charge(20); ai_melee() }
func hknight_char_a11() { Self.Frame = float32(HKNIGHT_FRAME_char_a11); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a12; ai_charge(18); ai_melee() }
func hknight_char_a12() { Self.Frame = float32(HKNIGHT_FRAME_char_a12); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a13; ai_charge(16) }
func hknight_char_a13() { Self.Frame = float32(HKNIGHT_FRAME_char_a13); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a14; ai_charge(14) }
func hknight_char_a14() { Self.Frame = float32(HKNIGHT_FRAME_char_a14); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a15; ai_charge(25) }
func hknight_char_a15() { Self.Frame = float32(HKNIGHT_FRAME_char_a15); Self.NextThink = Time + 0.1; Self.Think = hknight_char_a16; ai_charge(21) }
func hknight_char_a16() { Self.Frame = float32(HKNIGHT_FRAME_char_a16); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; ai_charge(13) }

func hknight_char_b1() {
	Self.Frame = float32(HKNIGHT_FRAME_char_b1)
	Self.NextThink = Time + 0.1
	Self.Think = hknight_char_b2
	CheckContinueCharge()
	ai_charge(23)
	ai_melee()
}
func hknight_char_b2() { Self.Frame = float32(HKNIGHT_FRAME_char_b2); Self.NextThink = Time + 0.1; Self.Think = hknight_char_b3; ai_charge(17); ai_melee() }
func hknight_char_b3() { Self.Frame = float32(HKNIGHT_FRAME_char_b3); Self.NextThink = Time + 0.1; Self.Think = hknight_char_b4; ai_charge(12); ai_melee() }
func hknight_char_b4() { Self.Frame = float32(HKNIGHT_FRAME_char_b4); Self.NextThink = Time + 0.1; Self.Think = hknight_char_b5; ai_charge(22); ai_melee() }
func hknight_char_b5() { Self.Frame = float32(HKNIGHT_FRAME_char_b5); Self.NextThink = Time + 0.1; Self.Think = hknight_char_b6; ai_charge(18); ai_melee() }
func hknight_char_b6() { Self.Frame = float32(HKNIGHT_FRAME_char_b6); Self.NextThink = Time + 0.1; Self.Think = hknight_char_b1; ai_charge(8); ai_melee() }

func hknight_slice1()  { Self.Frame = float32(HKNIGHT_FRAME_slice1); Self.NextThink = Time + 0.1; Self.Think = hknight_slice2; ai_charge(9) }
func hknight_slice2()  { Self.Frame = float32(HKNIGHT_FRAME_slice2); Self.NextThink = Time + 0.1; Self.Think = hknight_slice3; ai_charge(6) }
func hknight_slice3()  { Self.Frame = float32(HKNIGHT_FRAME_slice3); Self.NextThink = Time + 0.1; Self.Think = hknight_slice4; ai_charge(13) }
func hknight_slice4()  { Self.Frame = float32(HKNIGHT_FRAME_slice4); Self.NextThink = Time + 0.1; Self.Think = hknight_slice5; ai_charge(4) }
func hknight_slice5()  { Self.Frame = float32(HKNIGHT_FRAME_slice5); Self.NextThink = Time + 0.1; Self.Think = hknight_slice6; ai_charge(7); ai_melee() }
func hknight_slice6()  { Self.Frame = float32(HKNIGHT_FRAME_slice6); Self.NextThink = Time + 0.1; Self.Think = hknight_slice7; ai_charge(15); ai_melee() }
func hknight_slice7()  { Self.Frame = float32(HKNIGHT_FRAME_slice7); Self.NextThink = Time + 0.1; Self.Think = hknight_slice8; ai_charge(8); ai_melee() }
func hknight_slice8()  { Self.Frame = float32(HKNIGHT_FRAME_slice8); Self.NextThink = Time + 0.1; Self.Think = hknight_slice9; ai_charge(2); ai_melee() }
func hknight_slice9()  { Self.Frame = float32(HKNIGHT_FRAME_slice9); Self.NextThink = Time + 0.1; Self.Think = hknight_slice10; ai_melee() }
func hknight_slice10() { Self.Frame = float32(HKNIGHT_FRAME_slice10); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; ai_charge(3) }

func hknight_smash1() { Self.Frame = float32(HKNIGHT_FRAME_smash1); Self.NextThink = Time + 0.1; Self.Think = hknight_smash2; ai_charge(1) }
func hknight_smash2() { Self.Frame = float32(HKNIGHT_FRAME_smash2); Self.NextThink = Time + 0.1; Self.Think = hknight_smash3; ai_charge(13) }
func hknight_smash3() { Self.Frame = float32(HKNIGHT_FRAME_smash3); Self.NextThink = Time + 0.1; Self.Think = hknight_smash4; ai_charge(9) }
func hknight_smash4() { Self.Frame = float32(HKNIGHT_FRAME_smash4); Self.NextThink = Time + 0.1; Self.Think = hknight_smash5; ai_charge(11) }
func hknight_smash5() { Self.Frame = float32(HKNIGHT_FRAME_smash5); Self.NextThink = Time + 0.1; Self.Think = hknight_smash6; ai_charge(10); ai_melee() }
func hknight_smash6() { Self.Frame = float32(HKNIGHT_FRAME_smash6); Self.NextThink = Time + 0.1; Self.Think = hknight_smash7; ai_charge(7); ai_melee() }
func hknight_smash7() { Self.Frame = float32(HKNIGHT_FRAME_smash7); Self.NextThink = Time + 0.1; Self.Think = hknight_smash8; ai_charge(12); ai_melee() }
func hknight_smash8() { Self.Frame = float32(HKNIGHT_FRAME_smash8); Self.NextThink = Time + 0.1; Self.Think = hknight_smash9; ai_charge(2); ai_melee() }
func hknight_smash9() { Self.Frame = float32(HKNIGHT_FRAME_smash9); Self.NextThink = Time + 0.1; Self.Think = hknight_smash10; ai_charge(3); ai_melee() }
func hknight_smash10() { Self.Frame = float32(HKNIGHT_FRAME_smash10); Self.NextThink = Time + 0.1; Self.Think = hknight_smash11; ai_charge(0) }
func hknight_smash11() { Self.Frame = float32(HKNIGHT_FRAME_smash11); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; ai_charge(0) }

func hknight_watk1() { Self.Frame = float32(HKNIGHT_FRAME_w_attack1); Self.NextThink = Time + 0.1; Self.Think = hknight_watk2; ai_charge(2) }
func hknight_watk2() { Self.Frame = float32(HKNIGHT_FRAME_w_attack2); Self.NextThink = Time + 0.1; Self.Think = hknight_watk3; ai_charge(0) }
func hknight_watk3() { Self.Frame = float32(HKNIGHT_FRAME_w_attack3); Self.NextThink = Time + 0.1; Self.Think = hknight_watk4; ai_charge(0) }
func hknight_watk4() { Self.Frame = float32(HKNIGHT_FRAME_w_attack4); Self.NextThink = Time + 0.1; Self.Think = hknight_watk5; ai_melee() }
func hknight_watk5() { Self.Frame = float32(HKNIGHT_FRAME_w_attack5); Self.NextThink = Time + 0.1; Self.Think = hknight_watk6; ai_melee() }
func hknight_watk6() { Self.Frame = float32(HKNIGHT_FRAME_w_attack6); Self.NextThink = Time + 0.1; Self.Think = hknight_watk7; ai_melee() }
func hknight_watk7() { Self.Frame = float32(HKNIGHT_FRAME_w_attack7); Self.NextThink = Time + 0.1; Self.Think = hknight_watk8; ai_charge(1) }
func hknight_watk8() { Self.Frame = float32(HKNIGHT_FRAME_w_attack8); Self.NextThink = Time + 0.1; Self.Think = hknight_watk9; ai_charge(4) }
func hknight_watk9() { Self.Frame = float32(HKNIGHT_FRAME_w_attack9); Self.NextThink = Time + 0.1; Self.Think = hknight_watk10; ai_charge(5) }
func hknight_watk10() { Self.Frame = float32(HKNIGHT_FRAME_w_attack10); Self.NextThink = Time + 0.1; Self.Think = hknight_watk11; ai_charge(3); ai_melee() }
func hknight_watk11() { Self.Frame = float32(HKNIGHT_FRAME_w_attack11); Self.NextThink = Time + 0.1; Self.Think = hknight_watk12; ai_charge(2); ai_melee() }
func hknight_watk12() { Self.Frame = float32(HKNIGHT_FRAME_w_attack12); Self.NextThink = Time + 0.1; Self.Think = hknight_watk13; ai_charge(2); ai_melee() }
func hknight_watk13() { Self.Frame = float32(HKNIGHT_FRAME_w_attack13); Self.NextThink = Time + 0.1; Self.Think = hknight_watk14; ai_charge(0) }
func hknight_watk14() { Self.Frame = float32(HKNIGHT_FRAME_w_attack14); Self.NextThink = Time + 0.1; Self.Think = hknight_watk15; ai_charge(0) }
func hknight_watk15() { Self.Frame = float32(HKNIGHT_FRAME_w_attack15); Self.NextThink = Time + 0.1; Self.Think = hknight_watk16; ai_charge(0) }
func hknight_watk16() { Self.Frame = float32(HKNIGHT_FRAME_w_attack16); Self.NextThink = Time + 0.1; Self.Think = hknight_watk17; ai_charge(1) }
func hknight_watk17() { Self.Frame = float32(HKNIGHT_FRAME_w_attack17); Self.NextThink = Time + 0.1; Self.Think = hknight_watk18; ai_charge(1); ai_melee() }
func hknight_watk18() { Self.Frame = float32(HKNIGHT_FRAME_w_attack18); Self.NextThink = Time + 0.1; Self.Think = hknight_watk19; ai_charge(3); ai_melee() }
func hknight_watk19() { Self.Frame = float32(HKNIGHT_FRAME_w_attack19); Self.NextThink = Time + 0.1; Self.Think = hknight_watk20; ai_charge(4); ai_melee() }
func hknight_watk20() { Self.Frame = float32(HKNIGHT_FRAME_w_attack20); Self.NextThink = Time + 0.1; Self.Think = hknight_watk21; ai_charge(6) }
func hknight_watk21() { Self.Frame = float32(HKNIGHT_FRAME_w_attack21); Self.NextThink = Time + 0.1; Self.Think = hknight_watk22; ai_charge(7) }
func hknight_watk22() { Self.Frame = float32(HKNIGHT_FRAME_w_attack22); Self.NextThink = Time + 0.1; Self.Think = hknight_run1; ai_charge(3) }

func hknight_pain(attacker *quake.Entity, damage float32) {
	if Self.PainFinished > Time {
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "hknight/pain1.wav", 1, ATTN_NORM)

	if Time-Self.PainFinished > 5 { // always go into pain frame if it has been a while
		hknight_pain1()
		Self.PainFinished = Time + 1
		return
	}

	if engine.Random()*30 > damage {
		return // didn't flinch
	}

	Self.PainFinished = Time + 1
	hknight_pain1()
}

func hknight_melee() {
	hknight_type = hknight_type + 1

	engine.Sound(Self, int(CHAN_WEAPON), "hknight/slash1.wav", 1, ATTN_NORM)
	if hknight_type == 1 {
		hknight_slice1()
	} else if hknight_type == 2 {
		hknight_smash1()
	} else if hknight_type == 3 {
		hknight_watk1()
		hknight_type = 0
	}
}

func monster_hell_knight() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel2("progs/hknight.mdl")
	engine.PrecacheModel2("progs/k_spike.mdl")
	engine.PrecacheModel2("progs/h_hellkn.mdl")

	engine.PrecacheSound2("hknight/attack1.wav")
	engine.PrecacheSound2("hknight/death1.wav")
	engine.PrecacheSound2("hknight/pain1.wav")
	engine.PrecacheSound2("hknight/sight1.wav")
	engine.PrecacheSound("hknight/hit.wav") // used by C code, so don't sound2
	engine.PrecacheSound2("hknight/slash1.wav")
	engine.PrecacheSound2("hknight/idle.wav")
	engine.PrecacheSound2("hknight/grunt.wav")

	engine.PrecacheSound("knight/sword1.wav")
	engine.PrecacheSound("knight/sword2.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/hknight.mdl")

	Self.Noise = "hknight/sight1.wav"
	Self.NetName = "$qc_death_knight"
	Self.KillString = "$qc_ks_deathknight"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 40))
	Self.Health = 250
	Self.MaxHealth = 250

	Self.ThStand = hknight_stand1
	Self.ThWalk = hknight_walk1
	Self.ThRun = hknight_run1
	Self.ThMelee = hknight_melee
	Self.ThMissile = hknight_magicc1
	Self.ThPain = hknight_pain
	Self.ThDie = hknight_die
	Self.AllowPathFind = TRUE
	Self.CombatStyle = CS_MIXED

	walkmonster_start()
}
