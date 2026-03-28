package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

const (
	// Shub-Niggurath (Old One) frames
	SHUB_FRAME_old1 = iota
	SHUB_FRAME_old2
	SHUB_FRAME_old3
	SHUB_FRAME_old4
	SHUB_FRAME_old5
	SHUB_FRAME_old6
	SHUB_FRAME_old7
	SHUB_FRAME_old8
	SHUB_FRAME_old9
	SHUB_FRAME_old10
	SHUB_FRAME_old11
	SHUB_FRAME_old12
	SHUB_FRAME_old13
	SHUB_FRAME_old14
	SHUB_FRAME_old15
	SHUB_FRAME_old16
	SHUB_FRAME_old17
	SHUB_FRAME_old18
	SHUB_FRAME_old19
	SHUB_FRAME_old20
	SHUB_FRAME_old21
	SHUB_FRAME_old22
	SHUB_FRAME_old23
	SHUB_FRAME_old24
	SHUB_FRAME_old25
	SHUB_FRAME_old26
	SHUB_FRAME_old27
	SHUB_FRAME_old28
	SHUB_FRAME_old29
	SHUB_FRAME_old30
	SHUB_FRAME_old31
	SHUB_FRAME_old32
	SHUB_FRAME_old33
	SHUB_FRAME_old34
	SHUB_FRAME_old35
	SHUB_FRAME_old36
	SHUB_FRAME_old37
	SHUB_FRAME_old38
	SHUB_FRAME_old39
	SHUB_FRAME_old40
	SHUB_FRAME_old41
	SHUB_FRAME_old42
	SHUB_FRAME_old43
	SHUB_FRAME_old44
	SHUB_FRAME_old45
	SHUB_FRAME_old46

	SHUB_FRAME_shake1
	SHUB_FRAME_shake2
	SHUB_FRAME_shake3
	SHUB_FRAME_shake4
	SHUB_FRAME_shake5
	SHUB_FRAME_shake6
	SHUB_FRAME_shake7
	SHUB_FRAME_shake8
	SHUB_FRAME_shake9
	SHUB_FRAME_shake10
	SHUB_FRAME_shake11
	SHUB_FRAME_shake12
	SHUB_FRAME_shake13
	SHUB_FRAME_shake14
	SHUB_FRAME_shake15
	SHUB_FRAME_shake16
	SHUB_FRAME_shake17
	SHUB_FRAME_shake18
	SHUB_FRAME_shake19
	SHUB_FRAME_shake20
	SHUB_FRAME_shake21
)

var shub *quake.Entity

func old_idle1()  { Self.Frame = float32(SHUB_FRAME_old1); Self.NextThink = Time + 0.1; Self.Think = old_idle2 }
func old_idle2()  { Self.Frame = float32(SHUB_FRAME_old2); Self.NextThink = Time + 0.1; Self.Think = old_idle3 }
func old_idle3()  { Self.Frame = float32(SHUB_FRAME_old3); Self.NextThink = Time + 0.1; Self.Think = old_idle4 }
func old_idle4()  { Self.Frame = float32(SHUB_FRAME_old4); Self.NextThink = Time + 0.1; Self.Think = old_idle5 }
func old_idle5()  { Self.Frame = float32(SHUB_FRAME_old5); Self.NextThink = Time + 0.1; Self.Think = old_idle6 }
func old_idle6()  { Self.Frame = float32(SHUB_FRAME_old6); Self.NextThink = Time + 0.1; Self.Think = old_idle7 }
func old_idle7()  { Self.Frame = float32(SHUB_FRAME_old7); Self.NextThink = Time + 0.1; Self.Think = old_idle8 }
func old_idle8()  { Self.Frame = float32(SHUB_FRAME_old8); Self.NextThink = Time + 0.1; Self.Think = old_idle9 }
func old_idle9()  { Self.Frame = float32(SHUB_FRAME_old9); Self.NextThink = Time + 0.1; Self.Think = old_idle10 }
func old_idle10() { Self.Frame = float32(SHUB_FRAME_old10); Self.NextThink = Time + 0.1; Self.Think = old_idle11 }
func old_idle11() { Self.Frame = float32(SHUB_FRAME_old11); Self.NextThink = Time + 0.1; Self.Think = old_idle12 }
func old_idle12() { Self.Frame = float32(SHUB_FRAME_old12); Self.NextThink = Time + 0.1; Self.Think = old_idle13 }
func old_idle13() { Self.Frame = float32(SHUB_FRAME_old13); Self.NextThink = Time + 0.1; Self.Think = old_idle14 }
func old_idle14() { Self.Frame = float32(SHUB_FRAME_old14); Self.NextThink = Time + 0.1; Self.Think = old_idle15 }
func old_idle15() { Self.Frame = float32(SHUB_FRAME_old15); Self.NextThink = Time + 0.1; Self.Think = old_idle16 }
func old_idle16() { Self.Frame = float32(SHUB_FRAME_old16); Self.NextThink = Time + 0.1; Self.Think = old_idle17 }
func old_idle17() { Self.Frame = float32(SHUB_FRAME_old17); Self.NextThink = Time + 0.1; Self.Think = old_idle18 }
func old_idle18() { Self.Frame = float32(SHUB_FRAME_old18); Self.NextThink = Time + 0.1; Self.Think = old_idle19 }
func old_idle19() { Self.Frame = float32(SHUB_FRAME_old19); Self.NextThink = Time + 0.1; Self.Think = old_idle20 }
func old_idle20() { Self.Frame = float32(SHUB_FRAME_old20); Self.NextThink = Time + 0.1; Self.Think = old_idle21 }
func old_idle21() { Self.Frame = float32(SHUB_FRAME_old21); Self.NextThink = Time + 0.1; Self.Think = old_idle22 }
func old_idle22() { Self.Frame = float32(SHUB_FRAME_old22); Self.NextThink = Time + 0.1; Self.Think = old_idle23 }
func old_idle23() { Self.Frame = float32(SHUB_FRAME_old23); Self.NextThink = Time + 0.1; Self.Think = old_idle24 }
func old_idle24() { Self.Frame = float32(SHUB_FRAME_old24); Self.NextThink = Time + 0.1; Self.Think = old_idle25 }
func old_idle25() { Self.Frame = float32(SHUB_FRAME_old25); Self.NextThink = Time + 0.1; Self.Think = old_idle26 }
func old_idle26() { Self.Frame = float32(SHUB_FRAME_old26); Self.NextThink = Time + 0.1; Self.Think = old_idle27 }
func old_idle27() { Self.Frame = float32(SHUB_FRAME_old27); Self.NextThink = Time + 0.1; Self.Think = old_idle28 }
func old_idle28() { Self.Frame = float32(SHUB_FRAME_old28); Self.NextThink = Time + 0.1; Self.Think = old_idle29 }
func old_idle29() { Self.Frame = float32(SHUB_FRAME_old29); Self.NextThink = Time + 0.1; Self.Think = old_idle30 }
func old_idle30() { Self.Frame = float32(SHUB_FRAME_old30); Self.NextThink = Time + 0.1; Self.Think = old_idle31 }
func old_idle31() { Self.Frame = float32(SHUB_FRAME_old31); Self.NextThink = Time + 0.1; Self.Think = old_idle32 }
func old_idle32() { Self.Frame = float32(SHUB_FRAME_old32); Self.NextThink = Time + 0.1; Self.Think = old_idle33 }
func old_idle33() { Self.Frame = float32(SHUB_FRAME_old33); Self.NextThink = Time + 0.1; Self.Think = old_idle34 }
func old_idle34() { Self.Frame = float32(SHUB_FRAME_old34); Self.NextThink = Time + 0.1; Self.Think = old_idle35 }
func old_idle35() { Self.Frame = float32(SHUB_FRAME_old35); Self.NextThink = Time + 0.1; Self.Think = old_idle36 }
func old_idle36() { Self.Frame = float32(SHUB_FRAME_old36); Self.NextThink = Time + 0.1; Self.Think = old_idle37 }
func old_idle37() { Self.Frame = float32(SHUB_FRAME_old37); Self.NextThink = Time + 0.1; Self.Think = old_idle38 }
func old_idle38() { Self.Frame = float32(SHUB_FRAME_old38); Self.NextThink = Time + 0.1; Self.Think = old_idle39 }
func old_idle39() { Self.Frame = float32(SHUB_FRAME_old39); Self.NextThink = Time + 0.1; Self.Think = old_idle40 }
func old_idle40() { Self.Frame = float32(SHUB_FRAME_old40); Self.NextThink = Time + 0.1; Self.Think = old_idle41 }
func old_idle41() { Self.Frame = float32(SHUB_FRAME_old41); Self.NextThink = Time + 0.1; Self.Think = old_idle42 }
func old_idle42() { Self.Frame = float32(SHUB_FRAME_old42); Self.NextThink = Time + 0.1; Self.Think = old_idle43 }
func old_idle43() { Self.Frame = float32(SHUB_FRAME_old43); Self.NextThink = Time + 0.1; Self.Think = old_idle44 }
func old_idle44() { Self.Frame = float32(SHUB_FRAME_old44); Self.NextThink = Time + 0.1; Self.Think = old_idle45 }
func old_idle45() { Self.Frame = float32(SHUB_FRAME_old45); Self.NextThink = Time + 0.1; Self.Think = old_idle46 }
func old_idle46() { Self.Frame = float32(SHUB_FRAME_old46); Self.NextThink = Time + 0.1; Self.Think = old_idle1 }

// Prototyped elsewhere
var finale_4 func()

func old_thrash1_impl() {
	Self.Frame = float32(SHUB_FRAME_shake1)
	Self.NextThink = Time + 0.1
	Self.Think = old_thrash2
	engine.LightStyle(0, "m")
}
func old_thrash2()  { Self.Frame = float32(SHUB_FRAME_shake2); Self.NextThink = Time + 0.1; Self.Think = old_thrash3; engine.LightStyle(0, "k") }
func old_thrash3()  { Self.Frame = float32(SHUB_FRAME_shake3); Self.NextThink = Time + 0.1; Self.Think = old_thrash4; engine.LightStyle(0, "k") }
func old_thrash4()  { Self.Frame = float32(SHUB_FRAME_shake4); Self.NextThink = Time + 0.1; Self.Think = old_thrash5; engine.LightStyle(0, "i") }
func old_thrash5()  { Self.Frame = float32(SHUB_FRAME_shake5); Self.NextThink = Time + 0.1; Self.Think = old_thrash6; engine.LightStyle(0, "g") }
func old_thrash6()  { Self.Frame = float32(SHUB_FRAME_shake6); Self.NextThink = Time + 0.1; Self.Think = old_thrash7; engine.LightStyle(0, "e") }
func old_thrash7()  { Self.Frame = float32(SHUB_FRAME_shake7); Self.NextThink = Time + 0.1; Self.Think = old_thrash8; engine.LightStyle(0, "c") }
func old_thrash8()  { Self.Frame = float32(SHUB_FRAME_shake8); Self.NextThink = Time + 0.1; Self.Think = old_thrash9; engine.LightStyle(0, "a") }
func old_thrash9()  { Self.Frame = float32(SHUB_FRAME_shake9); Self.NextThink = Time + 0.1; Self.Think = old_thrash10; engine.LightStyle(0, "c") }
func old_thrash10() { Self.Frame = float32(SHUB_FRAME_shake10); Self.NextThink = Time + 0.1; Self.Think = old_thrash11; engine.LightStyle(0, "e") }
func old_thrash11() { Self.Frame = float32(SHUB_FRAME_shake11); Self.NextThink = Time + 0.1; Self.Think = old_thrash12; engine.LightStyle(0, "g") }
func old_thrash12() { Self.Frame = float32(SHUB_FRAME_shake12); Self.NextThink = Time + 0.1; Self.Think = old_thrash13; engine.LightStyle(0, "i") }
func old_thrash13() { Self.Frame = float32(SHUB_FRAME_shake13); Self.NextThink = Time + 0.1; Self.Think = old_thrash14; engine.LightStyle(0, "k") }
func old_thrash14() { Self.Frame = float32(SHUB_FRAME_shake14); Self.NextThink = Time + 0.1; Self.Think = old_thrash15; engine.LightStyle(0, "m") }
func old_thrash15() {
	Self.Frame = float32(SHUB_FRAME_shake15)
	Self.NextThink = Time + 0.1
	Self.Think = old_thrash16
	engine.LightStyle(0, "m")
	Self.Cnt = Self.Cnt + 1

	if Self.Cnt != 3 {
		Self.Think = old_thrash1_impl
	}
}

func old_thrash16() { Self.Frame = float32(SHUB_FRAME_shake16); Self.NextThink = Time + 0.1; Self.Think = old_thrash17; engine.LightStyle(0, "g") }
func old_thrash17() { Self.Frame = float32(SHUB_FRAME_shake17); Self.NextThink = Time + 0.1; Self.Think = old_thrash18; engine.LightStyle(0, "c") }
func old_thrash18() { Self.Frame = float32(SHUB_FRAME_shake18); Self.NextThink = Time + 0.1; Self.Think = old_thrash19; engine.LightStyle(0, "b") }
func old_thrash19() { Self.Frame = float32(SHUB_FRAME_shake19); Self.NextThink = Time + 0.1; Self.Think = old_thrash20; engine.LightStyle(0, "a") }
func old_thrash20() { Self.Frame = float32(SHUB_FRAME_shake20); Self.NextThink = Time + 0.1; Self.Think = old_thrash20; finale_4() }

// Prototyped elsewhere
var finale_2 func()
var finale_3 func()
var finale_5 func()
var finale_6 func()

func finale_1() {
	var pos, pl *quake.Entity
	var timer *quake.Entity

	IntermissionExitTime = Time + 10000000 // never allow exit
	IntermissionRunning = 1

	pos = engine.Find(World, "classname", "info_intermission")

	if pos == nil {
		engine.Error("no info_intermission")
	}

	pl = engine.Find(World, "classname", "misc_teleporttrain")

	if pl == nil {
		engine.Error("no teleporttrain")
	}
	engine.Remove(pl)

	engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
	engine.WriteString(MSG_ALL, StringNull)

	pl = engine.Find(World, "classname", "player")

	for pl != nil {
		pl.ViewOfs = quake.MakeVec3(0, 0, 0)
		pl.Angles = pos.Mangle
		Other.VAngle = pos.Mangle
		pl.FixAngle = 1 // turn this way immediately
		pl.Map = Self.Map
		pl.NextThink = Time + 0.5
		pl.TakeDamage = DAMAGE_NO
		pl.Solid = SOLID_NOT
		pl.MoveType = MOVETYPE_NONE
		pl.ModelIndex = 0
		engine.SetOrigin(pl, pos.Origin)
		pl = engine.Find(pl, "classname", "player")
	}

	timer = engine.Spawn()
	timer.NextThink = Time + 1
	timer.Think = finale_2

	if Campaign != 0 && World.Model == "maps/end.bsp" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_DEFEAT_SHUB")
		if Skill == 3 {
			engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
			engine.WriteString(MSG_ALL, "ACH_DEFEAT_SHUB_NIGHTMARE")
		}
	}
}

func finale_2_impl() {
	var o quake.Vec3

	o = shub.Origin.Sub(quake.MakeVec3(0, 100, 0))
	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_TELEPORT))
	engine.WriteCoord(MSG_BROADCAST, o[0])
	engine.WriteCoord(MSG_BROADCAST, o[1])
	engine.WriteCoord(MSG_BROADCAST, o[2])

	engine.Sound(shub, int(CHAN_VOICE), "misc/r_tele1.wav", 1, ATTN_NORM)

	Self.NextThink = Time + 2
	Self.Think = finale_3
}

func finale_3_impl() {
	shub.Think = old_thrash1_impl
	engine.Sound(shub, int(CHAN_VOICE), "boss2/death.wav", 1, ATTN_NORM)
	engine.LightStyle(0, "abcdefghijklmlkjihgfedcb")
}

func finale_4_impl() {
	var oldo quake.Vec3
	var x, y, z float32
	var r float32
	var n *quake.Entity

	engine.Sound(Self, int(CHAN_VOICE), "boss2/pop2.wav", 1, ATTN_NORM)

	oldo = Self.Origin

	z = 16
	for z <= 144 {
		x = -64
		for x <= 64 {
			y = -64
			for y <= 64 {
				Self.Origin[0] = oldo[0] + x
				Self.Origin[1] = oldo[1] + y
				Self.Origin[2] = oldo[2] + z

				r = engine.Random()

				if r < 0.3 {
					ThrowGib("progs/gib1.mdl", -999)
				} else if r < 0.6 {
					ThrowGib("progs/gib2.mdl", -999)
				} else {
					ThrowGib("progs/gib3.mdl", -999)
				}

				y = y + 32
			}
			x = x + 32
		}
		z = z + 96
	}

	engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
	engine.WriteString(MSG_ALL, "$qc_finale_end")

	n = engine.Spawn()
	engine.SetModel(n, "progs/player.mdl")
	oldo = oldo.Sub(quake.MakeVec3(32, 264, 0))
	engine.SetOrigin(n, oldo)
	n.Angles = quake.MakeVec3(0, 290, 0)
	n.Frame = 1

	engine.Remove(Self)

	engine.WriteByte(MSG_ALL, float32(SVC_CDTRACK))
	engine.WriteByte(MSG_ALL, 3)
	engine.WriteByte(MSG_ALL, 3)
	engine.LightStyle(0, "m")

	timer := engine.Spawn()
	timer.NextThink = Time + 1
	timer.Think = finale_5
}

func finale_5_impl() {
	if engine.FinaleFinished() != 0 {
		Self.NextThink = Time + 5
		Self.Think = finale_6
	} else {
		Self.NextThink = Time + 0.1
	}
}

func finale_6_impl() {
	if Coop == 0 {
		engine.LocalCmd("menu_credits\n")
		engine.LocalCmd("disconnect\n")
	} else {
		engine.Changelevel("start")
	}
}

func init() {
	finale_2 = finale_2_impl
	finale_3 = finale_3_impl
	finale_4 = finale_4_impl
	finale_5 = finale_5_impl
	finale_6 = finale_6_impl
}

func monster_oldone() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	engine.PrecacheModel2("progs/oldone.mdl")

	engine.PrecacheSound2("boss2/death.wav")
	engine.PrecacheSound2("boss2/idle.wav")
	engine.PrecacheSound2("boss2/sight.wav")
	engine.PrecacheSound2("boss2/pop2.wav")

	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_STEP

	engine.SetModel(Self, "progs/oldone.mdl")
	Self.NetName = "$qc_shub"
	Self.KillString = "$qc_ks_shub"

	engine.SetSize(Self, quake.MakeVec3(-160, -128, -24), quake.MakeVec3(160, 128, 256))

	Self.Health = 40000    // kill by telefrag
	Self.MaxHealth = 40000 // kill by telefrag
	Self.Think = old_idle1
	Self.NextThink = Time + 0.1
	Self.TakeDamage = DAMAGE_YES
	Self.ThPain = SUB_Null_Pain
	Self.ThDie = finale_1
	shub = Self

	TotalMonsters = TotalMonsters + 1
}
