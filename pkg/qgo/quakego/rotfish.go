package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Rotfish frames
	FISH_FRAME_attack1 = iota
	FISH_FRAME_attack2
	FISH_FRAME_attack3
	FISH_FRAME_attack4
	FISH_FRAME_attack5
	FISH_FRAME_attack6
	FISH_FRAME_attack7
	FISH_FRAME_attack8
	FISH_FRAME_attack9
	FISH_FRAME_attack10
	FISH_FRAME_attack11
	FISH_FRAME_attack12
	FISH_FRAME_attack13
	FISH_FRAME_attack14
	FISH_FRAME_attack15
	FISH_FRAME_attack16
	FISH_FRAME_attack17
	FISH_FRAME_attack18

	FISH_FRAME_death1
	FISH_FRAME_death2
	FISH_FRAME_death3
	FISH_FRAME_death4
	FISH_FRAME_death5
	FISH_FRAME_death6
	FISH_FRAME_death7
	FISH_FRAME_death8
	FISH_FRAME_death9
	FISH_FRAME_death10
	FISH_FRAME_death11
	FISH_FRAME_death12
	FISH_FRAME_death13
	FISH_FRAME_death14
	FISH_FRAME_death15
	FISH_FRAME_death16
	FISH_FRAME_death17
	FISH_FRAME_death18
	FISH_FRAME_death19
	FISH_FRAME_death20
	FISH_FRAME_death21

	FISH_FRAME_swim1
	FISH_FRAME_swim2
	FISH_FRAME_swim3
	FISH_FRAME_swim4
	FISH_FRAME_swim5
	FISH_FRAME_swim6
	FISH_FRAME_swim7
	FISH_FRAME_swim8
	FISH_FRAME_swim9
	FISH_FRAME_swim10
	FISH_FRAME_swim11
	FISH_FRAME_swim12
	FISH_FRAME_swim13
	FISH_FRAME_swim14
	FISH_FRAME_swim15
	FISH_FRAME_swim16
	FISH_FRAME_swim17
	FISH_FRAME_swim18

	FISH_FRAME_pain1
	FISH_FRAME_pain2
	FISH_FRAME_pain3
	FISH_FRAME_pain4
	FISH_FRAME_pain5
	FISH_FRAME_pain6
	FISH_FRAME_pain7
	FISH_FRAME_pain8
	FISH_FRAME_pain9
)

func f_stand1()  { Self.Frame = float32(FISH_FRAME_swim1); Self.NextThink = Time + 0.1; Self.Think = f_stand2; ai_stand() }
func f_stand2()  { Self.Frame = float32(FISH_FRAME_swim2); Self.NextThink = Time + 0.1; Self.Think = f_stand3; ai_stand() }
func f_stand3()  { Self.Frame = float32(FISH_FRAME_swim3); Self.NextThink = Time + 0.1; Self.Think = f_stand4; ai_stand() }
func f_stand4()  { Self.Frame = float32(FISH_FRAME_swim4); Self.NextThink = Time + 0.1; Self.Think = f_stand5; ai_stand() }
func f_stand5()  { Self.Frame = float32(FISH_FRAME_swim5); Self.NextThink = Time + 0.1; Self.Think = f_stand6; ai_stand() }
func f_stand6()  { Self.Frame = float32(FISH_FRAME_swim6); Self.NextThink = Time + 0.1; Self.Think = f_stand7; ai_stand() }
func f_stand7()  { Self.Frame = float32(FISH_FRAME_swim7); Self.NextThink = Time + 0.1; Self.Think = f_stand8; ai_stand() }
func f_stand8()  { Self.Frame = float32(FISH_FRAME_swim8); Self.NextThink = Time + 0.1; Self.Think = f_stand9; ai_stand() }
func f_stand9()  { Self.Frame = float32(FISH_FRAME_swim9); Self.NextThink = Time + 0.1; Self.Think = f_stand10; ai_stand() }
func f_stand10() { Self.Frame = float32(FISH_FRAME_swim10); Self.NextThink = Time + 0.1; Self.Think = f_stand11; ai_stand() }
func f_stand11() { Self.Frame = float32(FISH_FRAME_swim11); Self.NextThink = Time + 0.1; Self.Think = f_stand12; ai_stand() }
func f_stand12() { Self.Frame = float32(FISH_FRAME_swim12); Self.NextThink = Time + 0.1; Self.Think = f_stand13; ai_stand() }
func f_stand13() { Self.Frame = float32(FISH_FRAME_swim13); Self.NextThink = Time + 0.1; Self.Think = f_stand14; ai_stand() }
func f_stand14() { Self.Frame = float32(FISH_FRAME_swim14); Self.NextThink = Time + 0.1; Self.Think = f_stand15; ai_stand() }
func f_stand15() { Self.Frame = float32(FISH_FRAME_swim15); Self.NextThink = Time + 0.1; Self.Think = f_stand16; ai_stand() }
func f_stand16() { Self.Frame = float32(FISH_FRAME_swim16); Self.NextThink = Time + 0.1; Self.Think = f_stand17; ai_stand() }
func f_stand17() { Self.Frame = float32(FISH_FRAME_swim17); Self.NextThink = Time + 0.1; Self.Think = f_stand18; ai_stand() }
func f_stand18() { Self.Frame = float32(FISH_FRAME_swim18); Self.NextThink = Time + 0.1; Self.Think = f_stand1; ai_stand() }

func f_walk1()  { Self.Frame = float32(FISH_FRAME_swim1); Self.NextThink = Time + 0.1; Self.Think = f_walk2; ai_walk(8) }
func f_walk2()  { Self.Frame = float32(FISH_FRAME_swim2); Self.NextThink = Time + 0.1; Self.Think = f_walk3; ai_walk(8) }
func f_walk3()  { Self.Frame = float32(FISH_FRAME_swim3); Self.NextThink = Time + 0.1; Self.Think = f_walk4; ai_walk(8) }
func f_walk4()  { Self.Frame = float32(FISH_FRAME_swim4); Self.NextThink = Time + 0.1; Self.Think = f_walk5; ai_walk(8) }
func f_walk5()  { Self.Frame = float32(FISH_FRAME_swim5); Self.NextThink = Time + 0.1; Self.Think = f_walk6; ai_walk(8) }
func f_walk6()  { Self.Frame = float32(FISH_FRAME_swim6); Self.NextThink = Time + 0.1; Self.Think = f_walk7; ai_walk(8) }
func f_walk7()  { Self.Frame = float32(FISH_FRAME_swim7); Self.NextThink = Time + 0.1; Self.Think = f_walk8; ai_walk(8) }
func f_walk8()  { Self.Frame = float32(FISH_FRAME_swim8); Self.NextThink = Time + 0.1; Self.Think = f_walk9; ai_walk(8) }
func f_walk9()  { Self.Frame = float32(FISH_FRAME_swim9); Self.NextThink = Time + 0.1; Self.Think = f_walk10; ai_walk(8) }
func f_walk10() { Self.Frame = float32(FISH_FRAME_swim10); Self.NextThink = Time + 0.1; Self.Think = f_walk11; ai_walk(8) }
func f_walk11() { Self.Frame = float32(FISH_FRAME_swim11); Self.NextThink = Time + 0.1; Self.Think = f_walk12; ai_walk(8) }
func f_walk12() { Self.Frame = float32(FISH_FRAME_swim12); Self.NextThink = Time + 0.1; Self.Think = f_walk13; ai_walk(8) }
func f_walk13() { Self.Frame = float32(FISH_FRAME_swim13); Self.NextThink = Time + 0.1; Self.Think = f_walk14; ai_walk(8) }
func f_walk14() { Self.Frame = float32(FISH_FRAME_swim14); Self.NextThink = Time + 0.1; Self.Think = f_walk15; ai_walk(8) }
func f_walk15() { Self.Frame = float32(FISH_FRAME_swim15); Self.NextThink = Time + 0.1; Self.Think = f_walk16; ai_walk(8) }
func f_walk16() { Self.Frame = float32(FISH_FRAME_swim16); Self.NextThink = Time + 0.1; Self.Think = f_walk17; ai_walk(8) }
func f_walk17() { Self.Frame = float32(FISH_FRAME_swim17); Self.NextThink = Time + 0.1; Self.Think = f_walk18; ai_walk(8) }
func f_walk18() { Self.Frame = float32(FISH_FRAME_swim18); Self.NextThink = Time + 0.1; Self.Think = f_walk1; ai_walk(8) }

func f_run1() {
	Self.Frame = float32(FISH_FRAME_swim1)
	Self.NextThink = Time + 0.1
	Self.Think = f_run2
	ai_run(12)

	if engine.Random() < 0.5 {
		engine.Sound(Self, int(CHAN_VOICE), "fish/idle.wav", 1, ATTN_NORM)
	}
}

func f_run2() { Self.Frame = float32(FISH_FRAME_swim3); Self.NextThink = Time + 0.1; Self.Think = f_run3; ai_run(12) }
func f_run3() { Self.Frame = float32(FISH_FRAME_swim5); Self.NextThink = Time + 0.1; Self.Think = f_run4; ai_run(12) }
func f_run4() { Self.Frame = float32(FISH_FRAME_swim7); Self.NextThink = Time + 0.1; Self.Think = f_run5; ai_run(12) }
func f_run5() { Self.Frame = float32(FISH_FRAME_swim9); Self.NextThink = Time + 0.1; Self.Think = f_run6; ai_run(12) }
func f_run6() { Self.Frame = float32(FISH_FRAME_swim11); Self.NextThink = Time + 0.1; Self.Think = f_run7; ai_run(12) }
func f_run7() { Self.Frame = float32(FISH_FRAME_swim13); Self.NextThink = Time + 0.1; Self.Think = f_run8; ai_run(12) }
func f_run8() { Self.Frame = float32(FISH_FRAME_swim15); Self.NextThink = Time + 0.1; Self.Think = f_run9; ai_run(12) }
func f_run9() { Self.Frame = float32(FISH_FRAME_swim17); Self.NextThink = Time + 0.1; Self.Think = f_run1; ai_run(12) }

func fish_melee() {
	var delta quake.Vec3
	var ldmg float32

	if Self.Enemy == nil {
		return // removed before stroke
	}

	delta = Self.Enemy.Origin.Sub(Self.Origin)

	if engine.Vlen(delta) > 60 {
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), "fish/bite.wav", 1, ATTN_NORM)
	ldmg = (engine.Random() + engine.Random()) * 3
	T_Damage(Self.Enemy, Self, Self, ldmg)
}

func f_attack1()  { Self.Frame = float32(FISH_FRAME_attack1); Self.NextThink = Time + 0.1; Self.Think = f_attack2; ai_charge(10) }
func f_attack2()  { Self.Frame = float32(FISH_FRAME_attack2); Self.NextThink = Time + 0.1; Self.Think = f_attack3; ai_charge(10) }
func f_attack3()  { Self.Frame = float32(FISH_FRAME_attack3); Self.NextThink = Time + 0.1; Self.Think = f_attack4; fish_melee() }
func f_attack4()  { Self.Frame = float32(FISH_FRAME_attack4); Self.NextThink = Time + 0.1; Self.Think = f_attack5; ai_charge(10) }
func f_attack5()  { Self.Frame = float32(FISH_FRAME_attack5); Self.NextThink = Time + 0.1; Self.Think = f_attack6; ai_charge(10) }
func f_attack6()  { Self.Frame = float32(FISH_FRAME_attack6); Self.NextThink = Time + 0.1; Self.Think = f_attack7; ai_charge(10) }
func f_attack7()  { Self.Frame = float32(FISH_FRAME_attack7); Self.NextThink = Time + 0.1; Self.Think = f_attack8; ai_charge(10) }
func f_attack8()  { Self.Frame = float32(FISH_FRAME_attack8); Self.NextThink = Time + 0.1; Self.Think = f_attack9; ai_charge(10) }
func f_attack9()  { Self.Frame = float32(FISH_FRAME_attack9); Self.NextThink = Time + 0.1; Self.Think = f_attack10; fish_melee() }
func f_attack10() { Self.Frame = float32(FISH_FRAME_attack10); Self.NextThink = Time + 0.1; Self.Think = f_attack11; ai_charge(10) }
func f_attack11() { Self.Frame = float32(FISH_FRAME_attack11); Self.NextThink = Time + 0.1; Self.Think = f_attack12; ai_charge(10) }
func f_attack12() { Self.Frame = float32(FISH_FRAME_attack12); Self.NextThink = Time + 0.1; Self.Think = f_attack13; ai_charge(10) }
func f_attack13() { Self.Frame = float32(FISH_FRAME_attack13); Self.NextThink = Time + 0.1; Self.Think = f_attack14; ai_charge(10) }
func f_attack14() { Self.Frame = float32(FISH_FRAME_attack14); Self.NextThink = Time + 0.1; Self.Think = f_attack15; ai_charge(10) }
func f_attack15() { Self.Frame = float32(FISH_FRAME_attack15); Self.NextThink = Time + 0.1; Self.Think = f_attack16; fish_melee() }
func f_attack16() { Self.Frame = float32(FISH_FRAME_attack16); Self.NextThink = Time + 0.1; Self.Think = f_attack17; ai_charge(10) }
func f_attack17() { Self.Frame = float32(FISH_FRAME_attack17); Self.NextThink = Time + 0.1; Self.Think = f_attack18; ai_charge(10) }
func f_attack18() { Self.Frame = float32(FISH_FRAME_attack18); Self.NextThink = Time + 0.1; Self.Think = f_run1; ai_charge(10) }

func f_death1() {
	Self.Frame = float32(FISH_FRAME_death1)
	Self.NextThink = Time + 0.1
	Self.Think = f_death2
	engine.Sound(Self, int(CHAN_VOICE), "fish/death.wav", 1, ATTN_NORM)
}

func f_death2() {
	Self.Frame = float32(FISH_FRAME_death2)
	Self.NextThink = Time + 0.1
	Self.Think = f_death3
	Self.Solid = SOLID_NOT
}

func f_death3()  { Self.Frame = float32(FISH_FRAME_death3); Self.NextThink = Time + 0.1; Self.Think = f_death4 }
func f_death4()  { Self.Frame = float32(FISH_FRAME_death4); Self.NextThink = Time + 0.1; Self.Think = f_death5 }
func f_death5()  { Self.Frame = float32(FISH_FRAME_death5); Self.NextThink = Time + 0.1; Self.Think = f_death6 }
func f_death6()  { Self.Frame = float32(FISH_FRAME_death6); Self.NextThink = Time + 0.1; Self.Think = f_death7 }
func f_death7()  { Self.Frame = float32(FISH_FRAME_death7); Self.NextThink = Time + 0.1; Self.Think = f_death8 }
func f_death8()  { Self.Frame = float32(FISH_FRAME_death8); Self.NextThink = Time + 0.1; Self.Think = f_death9 }
func f_death9()  { Self.Frame = float32(FISH_FRAME_death9); Self.NextThink = Time + 0.1; Self.Think = f_death10 }
func f_death10() { Self.Frame = float32(FISH_FRAME_death10); Self.NextThink = Time + 0.1; Self.Think = f_death11 }
func f_death11() { Self.Frame = float32(FISH_FRAME_death11); Self.NextThink = Time + 0.1; Self.Think = f_death12 }
func f_death12() { Self.Frame = float32(FISH_FRAME_death12); Self.NextThink = Time + 0.1; Self.Think = f_death13 }
func f_death13() { Self.Frame = float32(FISH_FRAME_death13); Self.NextThink = Time + 0.1; Self.Think = f_death14 }
func f_death14() { Self.Frame = float32(FISH_FRAME_death14); Self.NextThink = Time + 0.1; Self.Think = f_death15 }
func f_death15() { Self.Frame = float32(FISH_FRAME_death15); Self.NextThink = Time + 0.1; Self.Think = f_death16 }
func f_death16() { Self.Frame = float32(FISH_FRAME_death16); Self.NextThink = Time + 0.1; Self.Think = f_death17 }
func f_death17() { Self.Frame = float32(FISH_FRAME_death17); Self.NextThink = Time + 0.1; Self.Think = f_death18 }
func f_death18() { Self.Frame = float32(FISH_FRAME_death18); Self.NextThink = Time + 0.1; Self.Think = f_death19 }
func f_death19() { Self.Frame = float32(FISH_FRAME_death19); Self.NextThink = Time + 0.1; Self.Think = f_death20 }
func f_death20() { Self.Frame = float32(FISH_FRAME_death20); Self.NextThink = Time + 0.1; Self.Think = f_death21 }
func f_death21() { Self.Frame = float32(FISH_FRAME_death21); Self.NextThink = Time + 0.1; Self.Think = f_death21 }

func f_pain1() { Self.Frame = float32(FISH_FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = f_pain2 }
func f_pain2() { Self.Frame = float32(FISH_FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = f_pain3; ai_pain(6) }
func f_pain3() { Self.Frame = float32(FISH_FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = f_pain4; ai_pain(6) }
func f_pain4() { Self.Frame = float32(FISH_FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = f_pain5; ai_pain(6) }
func f_pain5() { Self.Frame = float32(FISH_FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = f_pain6; ai_pain(6) }
func f_pain6() { Self.Frame = float32(FISH_FRAME_pain6); Self.NextThink = Time + 0.1; Self.Think = f_pain7; ai_pain(6) }
func f_pain7() { Self.Frame = float32(FISH_FRAME_pain7); Self.NextThink = Time + 0.1; Self.Think = f_pain8; ai_pain(6) }
func f_pain8() { Self.Frame = float32(FISH_FRAME_pain8); Self.NextThink = Time + 0.1; Self.Think = f_pain9; ai_pain(6) }
func f_pain9() { Self.Frame = float32(FISH_FRAME_pain9); Self.NextThink = Time + 0.1; Self.Think = f_run1; ai_pain(6) }

func fish_pain(attacker *quake.Entity, damage float32) {
	// fish always do pain frames
	f_pain1()
}

func monster_fish() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel2("progs/fish.mdl")

	engine.PrecacheSound2("fish/death.wav")
	engine.PrecacheSound2("fish/bite.wav")
	engine.PrecacheSound2("fish/idle.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/fish.mdl")

	Self.NetName = "$qc_rotfish"
	Self.Noise = "fish/idle.wav"
	Self.KillString = "$qc_ks_rotfish"

	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 24))
	Self.Health = 25
	Self.MaxHealth = 25

	Self.ThStand = f_stand1
	Self.ThWalk = f_walk1
	Self.ThRun = f_run1
	Self.ThDie = f_death1
	Self.ThPain = fish_pain
	Self.ThMelee = f_attack1

	swimmonster_start()
}
