package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Running
	FRAME_axrun1 = 0
	FRAME_axrun2 = 1
	FRAME_axrun3 = 2
	FRAME_axrun4 = 3
	FRAME_axrun5 = 4
	FRAME_axrun6 = 5

	FRAME_rockrun1 = 6
	FRAME_rockrun2 = 7
	FRAME_rockrun3 = 8
	FRAME_rockrun4 = 9
	FRAME_rockrun5 = 10
	FRAME_rockrun6 = 11

	// Standing
	FRAME_stand1 = 12
	FRAME_stand2 = 13
	FRAME_stand3 = 14
	FRAME_stand4 = 15
	FRAME_stand5 = 16

	FRAME_axstnd1 = 17
	FRAME_axstnd2 = 18
	FRAME_axstnd3 = 19
	FRAME_axstnd4 = 20
	FRAME_axstnd5 = 21
	FRAME_axstnd6 = 22
	FRAME_axstnd7 = 23
	FRAME_axstnd8 = 24
	FRAME_axstnd9 = 25
	FRAME_axstnd10 = 26
	FRAME_axstnd11 = 27
	FRAME_axstnd12 = 28

	// Pain
	FRAME_axpain1 = 29
	FRAME_axpain2 = 30
	FRAME_axpain3 = 31
	FRAME_axpain4 = 32
	FRAME_axpain5 = 33
	FRAME_axpain6 = 34

	FRAME_pain1 = 35
	FRAME_pain2 = 36
	FRAME_pain3 = 37
	FRAME_pain4 = 38
	FRAME_pain5 = 39
	FRAME_pain6 = 40

	// Death
	FRAME_axdeth1 = 41
	FRAME_axdeth2 = 42
	FRAME_axdeth3 = 43
	FRAME_axdeth4 = 44
	FRAME_axdeth5 = 45
	FRAME_axdeth6 = 46
	FRAME_axdeth7 = 47
	FRAME_axdeth8 = 48
	FRAME_axdeth9 = 49

	FRAME_deatha1 = 50
	FRAME_deatha2 = 51
	FRAME_deatha3 = 52
	FRAME_deatha4 = 53
	FRAME_deatha5 = 54
	FRAME_deatha6 = 55
	FRAME_deatha7 = 56
	FRAME_deatha8 = 57
	FRAME_deatha9 = 58
	FRAME_deatha10 = 59
	FRAME_deatha11 = 60

	FRAME_deathb1 = 61
	FRAME_deathb2 = 62
	FRAME_deathb3 = 63
	FRAME_deathb4 = 64
	FRAME_deathb5 = 65
	FRAME_deathb6 = 66
	FRAME_deathb7 = 67
	FRAME_deathb8 = 68
	FRAME_deathb9 = 69

	FRAME_deathc1 = 70
	FRAME_deathc2 = 71
	FRAME_deathc3 = 72
	FRAME_deathc4 = 73
	FRAME_deathc5 = 74
	FRAME_deathc6 = 75
	FRAME_deathc7 = 76
	FRAME_deathc8 = 77
	FRAME_deathc9 = 78
	FRAME_deathc10 = 79
	FRAME_deathc11 = 80
	FRAME_deathc12 = 81
	FRAME_deathc13 = 82
	FRAME_deathc14 = 83
	FRAME_deathc15 = 84

	FRAME_deathd1 = 85
	FRAME_deathd2 = 86
	FRAME_deathd3 = 87
	FRAME_deathd4 = 88
	FRAME_deathd5 = 89
	FRAME_deathd6 = 90
	FRAME_deathd7 = 91
	FRAME_deathd8 = 92
	FRAME_deathd9 = 93

	FRAME_deathe1 = 94
	FRAME_deathe2 = 95
	FRAME_deathe3 = 96
	FRAME_deathe4 = 97
	FRAME_deathe5 = 98
	FRAME_deathe6 = 99
	FRAME_deathe7 = 100
	FRAME_deathe8 = 101
	FRAME_deathe9 = 102

	// Attacks
	FRAME_nailatt1 = 103
	FRAME_nailatt2 = 104

	FRAME_light1 = 105
	FRAME_light2 = 106

	FRAME_rockatt1 = 107
	FRAME_rockatt2 = 108
	FRAME_rockatt3 = 109
	FRAME_rockatt4 = 110
	FRAME_rockatt5 = 111
	FRAME_rockatt6 = 112

	FRAME_shotatt1 = 113
	FRAME_shotatt2 = 114
	FRAME_shotatt3 = 115
	FRAME_shotatt4 = 116
	FRAME_shotatt5 = 117
	FRAME_shotatt6 = 118

	FRAME_axatt1 = 119
	FRAME_axatt2 = 120
	FRAME_axatt3 = 121
	FRAME_axatt4 = 122
	FRAME_axatt5 = 123
	FRAME_axatt6 = 124

	FRAME_axattb1 = 125
	FRAME_axattb2 = 126
	FRAME_axattb3 = 127
	FRAME_axattb4 = 128
	FRAME_axattb5 = 129
	FRAME_axattb6 = 130

	FRAME_axattc1 = 131
	FRAME_axattc2 = 132
	FRAME_axattc3 = 133
	FRAME_axattc4 = 134
	FRAME_axattc5 = 135
	FRAME_axattc6 = 136

	FRAME_axattd1 = 137
	FRAME_axattd2 = 138
	FRAME_axattd3 = 139
	FRAME_axattd4 = 140
	FRAME_axattd5 = 141
	FRAME_axattd6 = 142
)

func player_stand1() {
	Self.Frame = float32(FRAME_axstnd1)
	Self.NextThink = Time + 0.1
	Self.Think = player_stand1

	Self.WeaponFrame = 0

	if Self.Velocity[0] != 0 || Self.Velocity[1] != 0 {
		Self.WalkFrame = 0
		player_run()
		return
	}

	if int(Self.Weapon) == IT_AXE {
		if Self.WalkFrame >= 12 {
			Self.WalkFrame = 0
		}
		Self.Frame = float32(FRAME_axstnd1 + int(Self.WalkFrame))
	} else {
		if Self.WalkFrame >= 5 {
			Self.WalkFrame = 0
		}
		Self.Frame = float32(FRAME_stand1 + int(Self.WalkFrame))
	}

	Self.WalkFrame = Self.WalkFrame + 1
}

func player_run() {
	Self.Frame = float32(FRAME_rockrun1)
	Self.NextThink = Time + 0.1
	Self.Think = player_run

	Self.WeaponFrame = 0

	if Self.Velocity[0] == 0 && Self.Velocity[1] == 0 {
		Self.WalkFrame = 0
		player_stand1()
		return
	}

	if int(Self.Weapon) == IT_AXE {
		if Self.WalkFrame == 6 {
			Self.WalkFrame = 0
		}
		Self.Frame = float32(FRAME_axrun1 + int(Self.WalkFrame))
	} else {
		if Self.WalkFrame == 6 {
			Self.WalkFrame = 0
		}
		Self.Frame = float32(FRAME_rockrun1 + int(Self.WalkFrame))
	}

	Self.WalkFrame = Self.WalkFrame + 1
}

func player_shot1() {
	Self.Frame = float32(FRAME_shotatt1)
	Self.NextThink = Time + 0.1
	Self.Think = player_shot2
	Self.WeaponFrame = 1
	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
}
func player_shot2() { Self.Frame = float32(FRAME_shotatt2); Self.NextThink = Time + 0.1; Self.Think = player_shot3; Self.WeaponFrame = 2 }
func player_shot3() { Self.Frame = float32(FRAME_shotatt3); Self.NextThink = Time + 0.1; Self.Think = player_shot4; Self.WeaponFrame = 3 }
func player_shot4() { Self.Frame = float32(FRAME_shotatt4); Self.NextThink = Time + 0.1; Self.Think = player_shot5; Self.WeaponFrame = 4 }
func player_shot5() { Self.Frame = float32(FRAME_shotatt5); Self.NextThink = Time + 0.1; Self.Think = player_shot6; Self.WeaponFrame = 5 }
func player_shot6() { Self.Frame = float32(FRAME_shotatt6); Self.NextThink = Time + 0.1; Self.Think = player_run; Self.WeaponFrame = 6 }

func player_axe1() { Self.Frame = float32(FRAME_axatt1); Self.NextThink = Time + 0.1; Self.Think = player_axe2; Self.WeaponFrame = 1 }
func player_axe2() { Self.Frame = float32(FRAME_axatt2); Self.NextThink = Time + 0.1; Self.Think = player_axe3; Self.WeaponFrame = 2 }
func player_axe3() { Self.Frame = float32(FRAME_axatt3); Self.NextThink = Time + 0.1; Self.Think = player_axe4; Self.WeaponFrame = 3; W_FireAxe() }
func player_axe4() { Self.Frame = float32(FRAME_axatt4); Self.NextThink = Time + 0.1; Self.Think = player_run; Self.WeaponFrame = 4 }

func player_axeb1() { Self.Frame = float32(FRAME_axattb1); Self.NextThink = Time + 0.1; Self.Think = player_axeb2; Self.WeaponFrame = 5 }
func player_axeb2() { Self.Frame = float32(FRAME_axattb2); Self.NextThink = Time + 0.1; Self.Think = player_axeb3; Self.WeaponFrame = 6 }
func player_axeb3() { Self.Frame = float32(FRAME_axattb3); Self.NextThink = Time + 0.1; Self.Think = player_axeb4; Self.WeaponFrame = 7; W_FireAxe() }
func player_axeb4() { Self.Frame = float32(FRAME_axattb4); Self.NextThink = Time + 0.1; Self.Think = player_run; Self.WeaponFrame = 8 }

func player_axec1() { Self.Frame = float32(FRAME_axattc1); Self.NextThink = Time + 0.1; Self.Think = player_axec2; Self.WeaponFrame = 1 }
func player_axec2() { Self.Frame = float32(FRAME_axattc2); Self.NextThink = Time + 0.1; Self.Think = player_axec3; Self.WeaponFrame = 2 }
func player_axec3() { Self.Frame = float32(FRAME_axattc3); Self.NextThink = Time + 0.1; Self.Think = player_axec4; Self.WeaponFrame = 3; W_FireAxe() }
func player_axec4() { Self.Frame = float32(FRAME_axattc4); Self.NextThink = Time + 0.1; Self.Think = player_run; Self.WeaponFrame = 4 }

func player_axed1() { Self.Frame = float32(FRAME_axattd1); Self.NextThink = Time + 0.1; Self.Think = player_axed2; Self.WeaponFrame = 5 }
func player_axed2() { Self.Frame = float32(FRAME_axattd2); Self.NextThink = Time + 0.1; Self.Think = player_axed3; Self.WeaponFrame = 6 }
func player_axed3() { Self.Frame = float32(FRAME_axattd3); Self.NextThink = Time + 0.1; Self.Think = player_axed4; Self.WeaponFrame = 7; W_FireAxe() }
func player_axed4() { Self.Frame = float32(FRAME_axattd4); Self.NextThink = Time + 0.1; Self.Think = player_run; Self.WeaponFrame = 8 }

func player_nail1() {
	Self.Frame = float32(FRAME_nailatt1)
	Self.NextThink = Time + 0.1
	Self.Think = player_nail2

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)

	if Self.Button0 == 0 {
		player_run()
		return
	}

	Self.WeaponFrame = Self.WeaponFrame + 1
	if Self.WeaponFrame == 9 {
		Self.WeaponFrame = 1
	}

	SuperDamageSound()
	W_FireSpikes(4)
	Self.AttackFinished = Time + 0.2
}

func player_nail2() {
	Self.Frame = float32(FRAME_nailatt2)
	Self.NextThink = Time + 0.1
	Self.Think = player_nail1

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)

	if Self.Button0 == 0 {
		player_run()
		return
	}

	Self.WeaponFrame = Self.WeaponFrame + 1
	if Self.WeaponFrame == 9 {
		Self.WeaponFrame = 1
	}

	SuperDamageSound()
	W_FireSpikes(-4)
	Self.AttackFinished = Time + 0.2
}

func player_light1() {
	Self.Frame = float32(FRAME_light1)
	Self.NextThink = Time + 0.1
	Self.Think = player_light2

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)

	if Self.Button0 == 0 {
		player_run()
		return
	}

	Self.WeaponFrame = Self.WeaponFrame + 1
	if Self.WeaponFrame == 5 {
		Self.WeaponFrame = 1
	}

	SuperDamageSound()
	W_FireLightning()
	Self.AttackFinished = Time + 0.2
}

func player_light2() {
	Self.Frame = float32(FRAME_light2)
	Self.NextThink = Time + 0.1
	Self.Think = player_light1

	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)

	if Self.Button0 == 0 {
		player_run()
		return
	}

	Self.WeaponFrame = Self.WeaponFrame + 1
	if Self.WeaponFrame == 5 {
		Self.WeaponFrame = 1
	}

	SuperDamageSound()
	W_FireLightning()
	Self.AttackFinished = Time + 0.2
}

func player_rocket1() {
	Self.Frame = float32(FRAME_rockatt1)
	Self.NextThink = Time + 0.1
	Self.Think = player_rocket2
	Self.WeaponFrame = 1
	Self.Effects = float32(int(Self.Effects) | EF_MUZZLEFLASH)
}
func player_rocket2() { Self.Frame = float32(FRAME_rockatt2); Self.NextThink = Time + 0.1; Self.Think = player_rocket3; Self.WeaponFrame = 2 }
func player_rocket3() { Self.Frame = float32(FRAME_rockatt3); Self.NextThink = Time + 0.1; Self.Think = player_rocket4; Self.WeaponFrame = 3 }
func player_rocket4() { Self.Frame = float32(FRAME_rockatt4); Self.NextThink = Time + 0.1; Self.Think = player_rocket5; Self.WeaponFrame = 4 }
func player_rocket5() { Self.Frame = float32(FRAME_rockatt5); Self.NextThink = Time + 0.1; Self.Think = player_rocket6; Self.WeaponFrame = 5 }
func player_rocket6() { Self.Frame = float32(FRAME_rockatt6); Self.NextThink = Time + 0.1; Self.Think = player_run; Self.WeaponFrame = 6 }

func PainSound() {
	var rs float32

	if Self.Health < 0 {
		return
	}

	if DamageAttacker.ClassName == "teledeath" {
		engine.Sound(Self, int(CHAN_VOICE), "player/teledth1.wav", 1, ATTN_NONE)
		return
	}

	if int(Self.WaterType) == CONTENT_WATER && Self.WaterLevel == 3 {
		DeathBubbles(1)
		if engine.Random() > 0.5 {
			engine.Sound(Self, int(CHAN_VOICE), "player/drown1.wav", 1, ATTN_NORM)
		} else {
			engine.Sound(Self, int(CHAN_VOICE), "player/drown2.wav", 1, ATTN_NORM)
		}
		return
	}

	if int(Self.WaterType) == CONTENT_SLIME {
		if engine.Random() > 0.5 {
			engine.Sound(Self, int(CHAN_VOICE), "player/lburn1.wav", 1, ATTN_NORM)
		} else {
			engine.Sound(Self, int(CHAN_VOICE), "player/lburn2.wav", 1, ATTN_NORM)
		}
		return
	}

	if int(Self.WaterType) == CONTENT_LAVA {
		if engine.Random() > 0.5 {
			engine.Sound(Self, int(CHAN_VOICE), "player/lburn1.wav", 1, ATTN_NORM)
		} else {
			engine.Sound(Self, int(CHAN_VOICE), "player/lburn2.wav", 1, ATTN_NORM)
		}
		return
	}

	if Self.PainFinished > Time {
		Self.AxHitMe = 0
		return
	}

	Self.PainFinished = Time + 0.5

	if Self.AxHitMe == 1 {
		Self.AxHitMe = 0
		engine.Sound(Self, int(CHAN_VOICE), "player/axhit1.wav", 1, ATTN_NORM)
		return
	}

	rs = engine.RInt((engine.Random() * 5) + 1)
	Self.Noise = StringNull

	switch int(rs) {
	case 1:
		Self.Noise = "player/pain1.wav"
	case 2:
		Self.Noise = "player/pain2.wav"
	case 3:
		Self.Noise = "player/pain3.wav"
	case 4:
		Self.Noise = "player/pain4.wav"
	case 5:
		Self.Noise = "player/pain5.wav"
	case 6:
		Self.Noise = "player/pain6.wav"
	}

	engine.Sound(Self, int(CHAN_VOICE), Self.Noise, 1, ATTN_NORM)
}

func player_pain1() { Self.Frame = float32(FRAME_pain1); Self.NextThink = Time + 0.1; Self.Think = player_pain2; PainSound(); Self.WeaponFrame = 0 }
func player_pain2() { Self.Frame = float32(FRAME_pain2); Self.NextThink = Time + 0.1; Self.Think = player_pain3 }
func player_pain3() { Self.Frame = float32(FRAME_pain3); Self.NextThink = Time + 0.1; Self.Think = player_pain4 }
func player_pain4() { Self.Frame = float32(FRAME_pain4); Self.NextThink = Time + 0.1; Self.Think = player_pain5 }
func player_pain5() { Self.Frame = float32(FRAME_pain5); Self.NextThink = Time + 0.1; Self.Think = player_pain6 }
func player_pain6() { Self.Frame = float32(FRAME_pain6); Self.NextThink = Time + 0.1; Self.Think = player_run }

func player_axpain1() { Self.Frame = float32(FRAME_axpain1); Self.NextThink = Time + 0.1; Self.Think = player_axpain2; PainSound(); Self.WeaponFrame = 0 }
func player_axpain2() { Self.Frame = float32(FRAME_axpain2); Self.NextThink = Time + 0.1; Self.Think = player_axpain3 }
func player_axpain3() { Self.Frame = float32(FRAME_axpain3); Self.NextThink = Time + 0.1; Self.Think = player_axpain4 }
func player_axpain4() { Self.Frame = float32(FRAME_axpain4); Self.NextThink = Time + 0.1; Self.Think = player_axpain5 }
func player_axpain5() { Self.Frame = float32(FRAME_axpain5); Self.NextThink = Time + 0.1; Self.Think = player_axpain6 }
func player_axpain6() { Self.Frame = float32(FRAME_axpain6); Self.NextThink = Time + 0.1; Self.Think = player_run }

func player_pain(attacker *quake.Entity, damage float32) {
	if Self.WeaponFrame != 0 {
		return
	}

	if Self.InvisibleFinished > Time {
		return
	}

	if int(Self.Weapon) == IT_AXE {
		player_axpain1()
	} else {
		player_pain1()
	}
}

func DeathBubblesSpawn() {
	if Self.Owner.WaterLevel != 3 {
		return
	}

	bubble := engine.Spawn()
	engine.SetModel(bubble, "progs/s_bubble.spr")
	engine.SetOrigin(bubble, Self.Owner.Origin.Add(quake.MakeVec3(0, 0, 24)))
	bubble.MoveType = MOVETYPE_NOCLIP
	bubble.Solid = SOLID_NOT
	bubble.Velocity = quake.MakeVec3(0, 0, 15)
	bubble.NextThink = Time + 0.5
	bubble.Think = bubble_bob
	bubble.ClassName = "bubble"
	bubble.Frame = 0
	bubble.Cnt = 0
	engine.SetSize(bubble, quake.MakeVec3(-8, -8, -8), quake.MakeVec3(8, 8, 8))
	Self.NextThink = Time + 0.1
	Self.Think = DeathBubblesSpawn
	Self.AirFinished = Self.AirFinished + 1

	if Self.AirFinished >= Self.BubbleCount {
		engine.Remove(Self)
	}
}

func DeathBubbles(num_bubbles float32) {
	bubble_spawner := engine.Spawn()
	engine.SetOrigin(bubble_spawner, Self.Origin)
	bubble_spawner.MoveType = MOVETYPE_NONE
	bubble_spawner.Solid = SOLID_NOT
	bubble_spawner.NextThink = Time + 0.1
	bubble_spawner.Think = DeathBubblesSpawn
	bubble_spawner.AirFinished = 0
	bubble_spawner.Owner = Self
	bubble_spawner.BubbleCount = num_bubbles
}

func DeathSound() {
	var rs float32

	if Self.WaterLevel == 3 {
		DeathBubbles(20)
		engine.Sound(Self, int(CHAN_VOICE), "player/h2odeath.wav", 1, ATTN_NONE)
		return
	}

	rs = engine.RInt((engine.Random() * 4) + 1)

	switch int(rs) {
	case 1:
		Self.Noise = "player/death1.wav"
	case 2:
		Self.Noise = "player/death2.wav"
	case 3:
		Self.Noise = "player/death3.wav"
	case 4:
		Self.Noise = "player/death4.wav"
	case 5:
		Self.Noise = "player/death5.wav"
	}

	engine.Sound(Self, int(CHAN_VOICE), Self.Noise, 1, ATTN_NONE)
}

func PlayerDead() {
	Self.NextThink = -1
	Self.DeadFlag = DEAD_DEAD
}

func VelocityForDamage(dm float32) quake.Vec3 {
	var v quake.Vec3

	v[0] = 100 * crandom()
	v[1] = 100 * crandom()
	v[2] = 200 + 100*engine.Random()

	if dm > -50 {
		v = v.Mul(0.7)
	} else if dm > -200 {
		v = v.Mul(2)
	} else {
		v = v.Mul(10)
	}

	return v
}

func ThrowGib(gibname string, dm float32) {
	newGib := engine.Spawn()
	newGib.Origin = Self.Origin
	engine.SetModel(newGib, gibname)
	engine.SetSize(newGib, quake.MakeVec3(0, 0, 0), quake.MakeVec3(0, 0, 0))
	newGib.Velocity = VelocityForDamage(dm)
	newGib.MoveType = MOVETYPE_BOUNCE
	if engine.Cvar("pr_checkextension") != 0 {
		if engine.CheckExtension("EX_MOVETYPE_GIB") != 0 {
			newGib.MoveType = 11 // MOVETYPE_GIB
		}
	}
	newGib.Solid = SOLID_NOT
	newGib.AVelocity[0] = engine.Random() * 600
	newGib.AVelocity[1] = engine.Random() * 600
	newGib.AVelocity[2] = engine.Random() * 600
	newGib.Think = SUB_Remove
	newGib.LTime = Time
	newGib.NextThink = Time + 10 + engine.Random()*10
	newGib.Frame = 0
	newGib.Flags = 0
}

func ThrowHead(gibname string, dm float32) {
	engine.SetModel(Self, gibname)
	Self.Frame = 0
	Self.NextThink = -1
	Self.MoveType = MOVETYPE_BOUNCE
	if engine.Cvar("pr_checkextension") != 0 {
		if engine.CheckExtension("EX_MOVETYPE_GIB") != 0 {
			Self.MoveType = 11 // MOVETYPE_GIB
		}
	}
	Self.TakeDamage = DAMAGE_NO
	Self.Solid = SOLID_NOT
	Self.ViewOfs = quake.MakeVec3(0, 0, 8)
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	Self.Velocity = VelocityForDamage(dm)
	Self.Origin[2] = Self.Origin[2] - 24
	Self.Flags = Self.Flags - float32(int(Self.Flags)&FL_ONGROUND)
	Self.AVelocity = quake.MakeVec3(0, crandom()*600, 0)
}

func GibPlayer() {
	ThrowHead("progs/h_player.mdl", Self.Health)
	ThrowGib("progs/gib1.mdl", Self.Health)
	ThrowGib("progs/gib2.mdl", Self.Health)
	ThrowGib("progs/gib3.mdl", Self.Health)

	Self.DeadFlag = DEAD_DEAD

	if DamageAttacker.ClassName == "teledeath" {
		engine.Sound(Self, int(CHAN_VOICE), "player/teledth1.wav", 1, ATTN_NONE)
		return
	}

	if DamageAttacker.ClassName == "teledeath2" {
		engine.Sound(Self, int(CHAN_VOICE), "player/teledth1.wav", 1, ATTN_NONE)
		return
	}

	if engine.Random() < 0.5 {
		engine.Sound(Self, int(CHAN_VOICE), "player/gib.wav", 1, ATTN_NONE)
	} else {
		engine.Sound(Self, int(CHAN_VOICE), "player/udeath.wav", 1, ATTN_NONE)
	}
}

func PlayerDie() {
	var i float32

	Self.Items = float32(int(Self.Items) - (int(Self.Items) & (IT_INVISIBILITY | IT_INVULNERABILITY | IT_SUIT | IT_QUAD)))
	Self.InvisibleFinished = 0
	Self.InvincibleFinished = 0
	Self.SuperDamageFinished = 0
	Self.RadSuitFinished = 0
	Self.Effects = 0
	Self.ModelIndex = ModelIndexPlayer

	if Deathmatch != 0 || Coop != 0 {
		DropBackpack()
	}

	Self.WeaponModel = StringNull
	Self.ViewOfs = quake.MakeVec3(0, 0, -8)
	Self.DeadFlag = DEAD_DYING
	Self.Solid = SOLID_NOT
	Self.Flags = Self.Flags - float32(int(Self.Flags)&FL_ONGROUND)
	Self.MoveType = MOVETYPE_TOSS

	if Self.Velocity[2] < 10 {
		Self.Velocity[2] = Self.Velocity[2] + engine.Random()*300
	}

	if Self.Health < -40 {
		GibPlayer()
		return
	}

	DeathSound()

	Self.Angles[0] = 0
	Self.Angles[2] = 0

	if int(Self.Weapon) == IT_AXE {
		player_die_ax1()
		return
	}

	i = 1 + engine.Floor(engine.Random()*6)

	switch int(i) {
	case 1:
		player_diea1()
	case 2:
		player_dieb1()
	case 3:
		player_diec1()
	case 4:
		player_died1()
	default:
		player_diee1()
	}
}

func set_suicide_frame() {
	if Self.Model != "progs/player.mdl" {
		return
	}

	Self.Frame = float32(FRAME_deatha11)
	Self.Solid = SOLID_NOT
	Self.MoveType = MOVETYPE_TOSS
	Self.DeadFlag = DEAD_DEAD
	Self.NextThink = -1
}

func player_diea1()  { Self.Frame = float32(FRAME_deatha1); Self.NextThink = Time + 0.1; Self.Think = player_diea2 }
func player_diea2()  { Self.Frame = float32(FRAME_deatha2); Self.NextThink = Time + 0.1; Self.Think = player_diea3 }
func player_diea3()  { Self.Frame = float32(FRAME_deatha3); Self.NextThink = Time + 0.1; Self.Think = player_diea4 }
func player_diea4()  { Self.Frame = float32(FRAME_deatha4); Self.NextThink = Time + 0.1; Self.Think = player_diea5 }
func player_diea5()  { Self.Frame = float32(FRAME_deatha5); Self.NextThink = Time + 0.1; Self.Think = player_diea6 }
func player_diea6()  { Self.Frame = float32(FRAME_deatha6); Self.NextThink = Time + 0.1; Self.Think = player_diea7 }
func player_diea7()  { Self.Frame = float32(FRAME_deatha7); Self.NextThink = Time + 0.1; Self.Think = player_diea8 }
func player_diea8()  { Self.Frame = float32(FRAME_deatha8); Self.NextThink = Time + 0.1; Self.Think = player_diea9 }
func player_diea9()  { Self.Frame = float32(FRAME_deatha9); Self.NextThink = Time + 0.1; Self.Think = player_diea10 }
func player_diea10() { Self.Frame = float32(FRAME_deatha10); Self.NextThink = Time + 0.1; Self.Think = player_diea11 }
func player_diea11() { Self.Frame = float32(FRAME_deatha11); Self.NextThink = Time + 0.1; Self.Think = player_diea11; PlayerDead() }

func player_dieb1() { Self.Frame = float32(FRAME_deathb1); Self.NextThink = Time + 0.1; Self.Think = player_dieb2 }
func player_dieb2() { Self.Frame = float32(FRAME_deathb2); Self.NextThink = Time + 0.1; Self.Think = player_dieb3 }
func player_dieb3() { Self.Frame = float32(FRAME_deathb3); Self.NextThink = Time + 0.1; Self.Think = player_dieb4 }
func player_dieb4() { Self.Frame = float32(FRAME_deathb4); Self.NextThink = Time + 0.1; Self.Think = player_dieb5 }
func player_dieb5() { Self.Frame = float32(FRAME_deathb5); Self.NextThink = Time + 0.1; Self.Think = player_dieb6 }
func player_dieb6() { Self.Frame = float32(FRAME_deathb6); Self.NextThink = Time + 0.1; Self.Think = player_dieb7 }
func player_dieb7() { Self.Frame = float32(FRAME_deathb7); Self.NextThink = Time + 0.1; Self.Think = player_dieb8 }
func player_dieb8() { Self.Frame = float32(FRAME_deathb8); Self.NextThink = Time + 0.1; Self.Think = player_dieb9 }
func player_dieb9() { Self.Frame = float32(FRAME_deathb9); Self.NextThink = Time + 0.1; Self.Think = player_dieb9; PlayerDead() }

func player_diec1()  { Self.Frame = float32(FRAME_deathc1); Self.NextThink = Time + 0.1; Self.Think = player_diec2 }
func player_diec2()  { Self.Frame = float32(FRAME_deathc2); Self.NextThink = Time + 0.1; Self.Think = player_diec3 }
func player_diec3()  { Self.Frame = float32(FRAME_deathc3); Self.NextThink = Time + 0.1; Self.Think = player_diec4 }
func player_diec4()  { Self.Frame = float32(FRAME_deathc4); Self.NextThink = Time + 0.1; Self.Think = player_diec5 }
func player_diec5()  { Self.Frame = float32(FRAME_deathc5); Self.NextThink = Time + 0.1; Self.Think = player_diec6 }
func player_diec6()  { Self.Frame = float32(FRAME_deathc6); Self.NextThink = Time + 0.1; Self.Think = player_diec7 }
func player_diec7()  { Self.Frame = float32(FRAME_deathc7); Self.NextThink = Time + 0.1; Self.Think = player_diec8 }
func player_diec8()  { Self.Frame = float32(FRAME_deathc8); Self.NextThink = Time + 0.1; Self.Think = player_diec9 }
func player_diec9()  { Self.Frame = float32(FRAME_deathc9); Self.NextThink = Time + 0.1; Self.Think = player_diec10 }
func player_diec10() { Self.Frame = float32(FRAME_deathc10); Self.NextThink = Time + 0.1; Self.Think = player_diec11 }
func player_diec11() { Self.Frame = float32(FRAME_deathc11); Self.NextThink = Time + 0.1; Self.Think = player_diec12 }
func player_diec12() { Self.Frame = float32(FRAME_deathc12); Self.NextThink = Time + 0.1; Self.Think = player_diec13 }
func player_diec13() { Self.Frame = float32(FRAME_deathc13); Self.NextThink = Time + 0.1; Self.Think = player_diec14 }
func player_diec14() { Self.Frame = float32(FRAME_deathc14); Self.NextThink = Time + 0.1; Self.Think = player_diec15 }
func player_diec15() { Self.Frame = float32(FRAME_deathc15); Self.NextThink = Time + 0.1; Self.Think = player_diec15; PlayerDead() }

func player_died1() { Self.Frame = float32(FRAME_deathd1); Self.NextThink = Time + 0.1; Self.Think = player_died2 }
func player_died2() { Self.Frame = float32(FRAME_deathd2); Self.NextThink = Time + 0.1; Self.Think = player_died3 }
func player_died3() { Self.Frame = float32(FRAME_deathd3); Self.NextThink = Time + 0.1; Self.Think = player_died4 }
func player_died4() { Self.Frame = float32(FRAME_deathd4); Self.NextThink = Time + 0.1; Self.Think = player_died5 }
func player_died5() { Self.Frame = float32(FRAME_deathd5); Self.NextThink = Time + 0.1; Self.Think = player_died6 }
func player_died6() { Self.Frame = float32(FRAME_deathd6); Self.NextThink = Time + 0.1; Self.Think = player_died7 }
func player_died7() { Self.Frame = float32(FRAME_deathd7); Self.NextThink = Time + 0.1; Self.Think = player_died8 }
func player_died8() { Self.Frame = float32(FRAME_deathd8); Self.NextThink = Time + 0.1; Self.Think = player_died9 }
func player_died9() { Self.Frame = float32(FRAME_deathd9); Self.NextThink = Time + 0.1; Self.Think = player_died9; PlayerDead() }

func player_diee1() { Self.Frame = float32(FRAME_deathe1); Self.NextThink = Time + 0.1; Self.Think = player_diee2 }
func player_diee2() { Self.Frame = float32(FRAME_deathe2); Self.NextThink = Time + 0.1; Self.Think = player_diee3 }
func player_diee3() { Self.Frame = float32(FRAME_deathe3); Self.NextThink = Time + 0.1; Self.Think = player_diee4 }
func player_diee4() { Self.Frame = float32(FRAME_deathe4); Self.NextThink = Time + 0.1; Self.Think = player_diee5 }
func player_diee5() { Self.Frame = float32(FRAME_deathe5); Self.NextThink = Time + 0.1; Self.Think = player_diee6 }
func player_diee6() { Self.Frame = float32(FRAME_deathe6); Self.NextThink = Time + 0.1; Self.Think = player_diee7 }
func player_diee7() { Self.Frame = float32(FRAME_deathe7); Self.NextThink = Time + 0.1; Self.Think = player_diee8 }
func player_diee8() { Self.Frame = float32(FRAME_deathe8); Self.NextThink = Time + 0.1; Self.Think = player_diee9 }
func player_diee9() { Self.Frame = float32(FRAME_deathe9); Self.NextThink = Time + 0.1; Self.Think = player_diee9; PlayerDead() }

func player_die_ax1() { Self.Frame = float32(FRAME_axdeth1); Self.NextThink = Time + 0.1; Self.Think = player_die_ax2 }
func player_die_ax2() { Self.Frame = float32(FRAME_axdeth2); Self.NextThink = Time + 0.1; Self.Think = player_die_ax3 }
func player_die_ax3() { Self.Frame = float32(FRAME_axdeth3); Self.NextThink = Time + 0.1; Self.Think = player_die_ax4 }
func player_die_ax4() { Self.Frame = float32(FRAME_axdeth4); Self.NextThink = Time + 0.1; Self.Think = player_die_ax5 }
func player_die_ax5() { Self.Frame = float32(FRAME_axdeth5); Self.NextThink = Time + 0.1; Self.Think = player_die_ax6 }
func player_die_ax6() { Self.Frame = float32(FRAME_axdeth6); Self.NextThink = Time + 0.1; Self.Think = player_die_ax7 }
func player_die_ax7() { Self.Frame = float32(FRAME_axdeth7); Self.NextThink = Time + 0.1; Self.Think = player_die_ax8 }
func player_die_ax8() { Self.Frame = float32(FRAME_axdeth8); Self.NextThink = Time + 0.1; Self.Think = player_die_ax9 }
func player_die_ax9() { Self.Frame = float32(FRAME_axdeth9); Self.NextThink = Time + 0.1; Self.Think = player_die_ax9; PlayerDead() }
