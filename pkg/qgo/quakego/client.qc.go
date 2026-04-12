package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

var (
	IntermissionRunning  float32
	IntermissionExitTime float32
	NextMap              string
	ResetFlag            float32
)

func info_intermission() {}

func SetChangeParms() {
	if ResetFlag != 0 {
		engine.SetSpawnParms(Self)
		return
	}

	if Self.Health <= 0 || Deathmatch != 0 {
		SetNewParms()
		return
	}

	// remove items
	Self.Items = float32(int(Self.Items) - (int(Self.Items) & (IT_KEY1 | IT_KEY2 | IT_INVISIBILITY | IT_INVULNERABILITY | IT_SUIT | IT_QUAD)))

	// cap super health
	if Self.Health > Self.MaxHealth {
		Self.Health = Self.MaxHealth
	}

	if Self.Health < Self.MaxHealth/2 {
		Self.Health = Self.MaxHealth / 2
	}

	Parm1 = Self.Items
	Parm2 = Self.Health
	Parm3 = Self.ArmorValue

	if Self.AmmoShells < 25 {
		Parm4 = 25
	} else {
		Parm4 = Self.AmmoShells
	}

	Parm5 = Self.AmmoNails
	Parm6 = Self.AmmoRockets
	Parm7 = Self.AmmoCells
	Parm8 = Self.Weapon
	Parm9 = Self.ArmorType * 100
}

func SetNewParms() {
	Parm1 = float32(IT_SHOTGUN | IT_AXE)
	if Skill == 3 && Deathmatch == 0 {
		Parm2 = 50
	} else {
		Parm2 = 100
	}
	Parm3 = 0
	Parm4 = 25
	Parm5 = 0
	Parm6 = 0
	Parm7 = 0
	Parm8 = 1
	Parm9 = 0
}

func DecodeLevelParms() {
	if ServerFlags != 0 {
		if World.Model == "maps/start.bsp" {
			SetNewParms()
		}
	}

	Self.Items = Parm1
	Self.Health = Parm2
	Self.ArmorValue = Parm3
	Self.AmmoShells = Parm4
	Self.AmmoNails = Parm5
	Self.AmmoRockets = Parm6
	Self.AmmoCells = Parm7
	Self.Weapon = Parm8
	Self.ArmorType = Parm9 * 0.01
}

func FindIntermission() *quake.Entity {
	var spot *quake.Entity
	var cyc float32

	spot = engine.Find(World, "classname", "info_intermission")
	if spot != nil {
		cyc = engine.Random() * 4
		for cyc > 1 {
			spot = engine.Find(spot, "classname", "info_intermission")
			if spot == nil {
				spot = engine.Find(World, "classname", "info_intermission")
			}
			cyc = cyc - 1
		}
		return spot
	}

	spot = engine.Find(World, "classname", "info_player_start")
	if spot != nil {
		return spot
	}

	spot = engine.Find(World, "classname", "testplayerstart")
	if spot != nil {
		return spot
	}

	engine.ObjError("FindIntermission: no spot")
	return World
}

func GotoNextMap() {
	if engine.Cvar("samelevel") != 0 {
		engine.Changelevel(MapName)
	} else {
		engine.Changelevel(NextMap)
	}
}

func ExitIntermission() {
	if Deathmatch != 0 {
		GotoNextMap()
		return
	}

	IntermissionExitTime = Time + 1
	IntermissionRunning = IntermissionRunning + 1

	if IntermissionRunning == 2 {
		if World.Model == "maps/e1m7.bsp" {
			engine.WriteByte(MSG_ALL, float32(SVC_CDTRACK))
			engine.WriteByte(MSG_ALL, 2)
			engine.WriteByte(MSG_ALL, 3)

			if engine.Cvar("registered") == 0 {
				engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
				engine.WriteString(MSG_ALL, "$qc_finale_e1_shareware")
			} else {
				engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
				engine.WriteString(MSG_ALL, "$qc_finale_e1")
			}
			return
		} else if World.Model == "maps/e2m6.bsp" {
			engine.WriteByte(MSG_ALL, float32(SVC_CDTRACK))
			engine.WriteByte(MSG_ALL, 2)
			engine.WriteByte(MSG_ALL, 3)
			engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
			engine.WriteString(MSG_ALL, "$qc_finale_e2")
			return
		} else if World.Model == "maps/e3m6.bsp" {
			engine.WriteByte(MSG_ALL, float32(SVC_CDTRACK))
			engine.WriteByte(MSG_ALL, 2)
			engine.WriteByte(MSG_ALL, 3)
			engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
			engine.WriteString(MSG_ALL, "$qc_finale_e3")
			return
		} else if World.Model == "maps/e4m7.bsp" {
			engine.WriteByte(MSG_ALL, float32(SVC_CDTRACK))
			engine.WriteByte(MSG_ALL, 2)
			engine.WriteByte(MSG_ALL, 3)
			engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
			engine.WriteString(MSG_ALL, "$qc_finale_e4")
			return
		}
		GotoNextMap()
	}

	if IntermissionRunning == 3 {
		if engine.Cvar("registered") == 0 {
			engine.WriteByte(MSG_ALL, float32(SVC_SELLSCREEN))
			return
		}

		if (int(ServerFlags) & 15) == 15 {
			engine.WriteByte(MSG_ALL, float32(SVC_FINALE))
			engine.WriteString(MSG_ALL, "$qc_finale_all_runes")
			return
		}
	}

	GotoNextMap()
}

func IntermissionThink() {
	if Time < IntermissionExitTime {
		return
	}

	if Self.Button0 == 0 && Self.Button1 == 0 && Self.Button2 == 0 {
		return
	}

	ExitIntermission()
}

func execute_changelevel() {
	var pos *quake.Entity

	IntermissionRunning = 1

	if Deathmatch != 0 {
		IntermissionExitTime = Time + 5
	} else {
		IntermissionExitTime = Time + 2
	}

	engine.WriteByte(MSG_ALL, float32(SVC_CDTRACK))
	engine.WriteByte(MSG_ALL, 3)
	engine.WriteByte(MSG_ALL, 3)

	pos = FindIntermission()

	Other = engine.Find(World, "classname", "player")

	for Other != nil {
		Other.ViewOfs = quake.MakeVec3(0, 0, 0)
		Other.Angles = pos.Mangle
		Other.VAngle = pos.Mangle
		Other.FixAngle = 1
		Other.NextThink = Time + 0.5
		Other.TakeDamage = DAMAGE_NO
		Other.Solid = SOLID_NOT
		Other.MoveType = MOVETYPE_NONE
		Other.ModelIndex = 0
		engine.SetOrigin(Other, pos.Origin)

		if Skill == 3 {
			if Other.FiredWeapon == 0 && World.Model == "maps/e1m1.bsp" {
				MsgEntity = Other
				engine.WriteByte(MSG_ONE, float32(SVC_ACHIEVEMENT))
				engine.WriteString(MSG_ONE, "ACH_PACIFIST")
			}

			if Other.TookDamage == 0 && World.Model == "maps/e4m6.bsp" {
				MsgEntity = Other
				engine.WriteByte(MSG_ONE, float32(SVC_ACHIEVEMENT))
				engine.WriteString(MSG_ONE, "ACH_PAINLESS_MAZE")
			}
		}

		Other = engine.Find(Other, "classname", "player")
	}

	engine.WriteByte(MSG_ALL, float32(SVC_INTERMISSION))

	if Campaign != 0 && World.Model == "maps/e1m7.bsp" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_COMPLETE_E1M7")
	} else if Campaign != 0 && World.Model == "maps/e2m6.bsp" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_COMPLETE_E2M6")
	} else if Campaign != 0 && World.Model == "maps/e3m6.bsp" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_COMPLETE_E3M6")
	} else if Campaign != 0 && World.Model == "maps/e4m7.bsp" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_COMPLETE_E4M7")
	}

	if World.Model == "maps/e1m4.bsp" && NextMap == "e1m8" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_FIND_E1M8")
	} else if World.Model == "maps/e2m3.bsp" && NextMap == "e2m7" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_FIND_E2M7")
	} else if World.Model == "maps/e3m4.bsp" && NextMap == "e3m7" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_FIND_E3M7")
	} else if World.Model == "maps/e4m5.bsp" && NextMap == "e4m8" {
		engine.WriteByte(MSG_ALL, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ALL, "ACH_FIND_E4M8")
	}
}

func changelevel_touch() {
	if Other.ClassName != "player" {
		return
	}

	if (engine.Cvar("noexit") == 1) || (engine.Cvar("noexit") == 2 && MapName != "start") {
		T_Damage(Other, Self, Self, 50000)
		return
	}

	if Coop != 0 || Deathmatch != 0 {
		engine.Bprint(quake.Sprintf("$qc_exited %s", Other.NetName))
	}

	NextMap = Self.Map

	SUB_UseTargets()

	if (int(Self.SpawnFlags)&1) != 0 && Deathmatch == 0 {
		GotoNextMap()
		return
	}

	Self.Touch = SUB_Null
	Self.Think = execute_changelevel
	Self.NextThink = Time + 0.1
}

func trigger_changelevel() {
	if Self.Map == StringNull {
		engine.ObjError("changelevel trigger doesn't have map")
	}

	Self.NetName = "changelevel"
	Self.KillString = "$qc_ks_tried_leave"

	InitTrigger()
	Self.Touch = changelevel_touch
}

func NextLevel() {
	var o *quake.Entity

	if NextMap != StringNull {
		return
	}

	if MapName == "start" {
		if engine.Cvar("registered") == 0 {
			MapName = "e1m1"
		} else if (int(ServerFlags) & 1) == 0 {
			MapName = "e1m1"
			ServerFlags = float32(int(ServerFlags) | 1)
		} else if (int(ServerFlags) & 2) == 0 {
			MapName = "e2m1"
			ServerFlags = float32(int(ServerFlags) | 2)
		} else if (int(ServerFlags) & 4) == 0 {
			MapName = "e3m1"
			ServerFlags = float32(int(ServerFlags) | 4)
		} else if (int(ServerFlags) & 8) == 0 {
			MapName = "e4m1"
			ServerFlags = ServerFlags - 7
		}
		o = engine.Spawn()
		o.Map = MapName
	} else {
		o = engine.Find(World, "classname", "trigger_changelevel")
		if o == nil || MapName == "start" {
			o = engine.Spawn()
			o.Map = MapName
		}
	}

	NextMap = o.Map
	Gameover = TRUE

	if o.NextThink < Time {
		o.Think = execute_changelevel
		o.NextThink = Time + 0.1
	}
}

func CheckRules() {
	var timelimit, fraglimit float32

	if Gameover != 0 {
		return
	}

	timelimit = engine.Cvar("timelimit") * 60
	fraglimit = engine.Cvar("fraglimit")

	if timelimit != 0 && Time >= timelimit {
		NextLevel()
		return
	}

	if fraglimit != 0 && Self.Frags >= fraglimit {
		NextLevel()
		return
	}
}
