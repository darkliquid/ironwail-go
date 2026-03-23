package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

// Prototyped elsewhere
var (
)

var (
	IntermissionRunning float32
	IntermissionExitTime float32
	NextMap             string
	ResetFlag           float32
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

var StartingServerFlags float32

func respawn() {
	if Coop != 0 {
		CopyToBodyQueue(Self)
		engine.SetSpawnParms(Self)
		PutClientInServer()
	} else if Deathmatch != 0 {
		CopyToBodyQueue(Self)
		SetNewParms()
		PutClientInServer()
	} else {
		ServerFlags = StartingServerFlags
		ResetFlag = TRUE
		engine.LocalCmd("changelevel " + MapName + "\n")
	}
}

func ClientKill() {
	engine.Bprint(quake.Sprintf("$qc_suicides %s", Self.NetName))
	set_suicide_frame()
	Self.ModelIndex = ModelIndexPlayer
	Self.Frags = Self.Frags - 2
	respawn()
}

func PlayerVisibleToSpawnPoint(point *quake.Entity) float32 {
	var spot1, spot2 quake.Vec3
	player := engine.Find(World, "classname", "player")
	for player != nil {
		if player.Health > 0 {
			spot1 = point.Origin.Add(player.ViewOfs)
			spot2 = player.Origin.Add(player.ViewOfs)

			engine.Traceline(spot1, spot2, TRUE, point)
			if TraceFraction >= 1.0 {
				return TRUE
			}
		}
		player = engine.Find(player, "classname", "player")
	}
	return FALSE
}

var (
	IDEAL_DIST_FROM_DM_SPAWN_POINT float32 = 384
	MIN_DIST_FROM_DM_SPAWN_POINT   float32 = 84
	LastSpawn                     *quake.Entity
)

func SelectSpawnPoint(forceSpawn float32) *quake.Entity {
	var spot, thing *quake.Entity
	var numspots, totalspots float32
	var pcount float32
	var spots *quake.Entity

	numspots = 0
	totalspots = 0

	spot = engine.Find(World, "classname", "testplayerstart")
	if spot != nil {
		return spot
	}

	if Coop != 0 {
		LastSpawn = engine.Find(LastSpawn, "classname", "info_player_coop")
		if LastSpawn == nil {
			LastSpawn = engine.Find(nil, "classname", "info_player_start")
		}
		if LastSpawn != nil {
			return LastSpawn
		}
	} else if Deathmatch != 0 {
		spots = nil
		spot = engine.Find(World, "classname", "info_player_deathmatch")

		for spot != nil {
			totalspots = totalspots + 1
			thing = engine.FindRadius(spot.Origin, IDEAL_DIST_FROM_DM_SPAWN_POINT)
			pcount = 0

			for thing != nil {
				if thing.ClassName == "player" && thing.Health > 0 {
					pcount = pcount + 1
				}
				thing = thing.Chain
			}

			if pcount == 0 {
				if PlayerVisibleToSpawnPoint(spot) != 0 {
					pcount = pcount + 1
				}
			}

			if pcount == 0 {
				spot.GoalEntity = spots
				spots = spot
				numspots = numspots + 1
			}
			spot = engine.Find(spot, "classname", "info_player_deathmatch")
		}

		totalspots = totalspots - 1

		if numspots == 0 {
			spot = engine.Find(World, "classname", "info_player_deathmatch")
			for spot != nil {
				thing = engine.FindRadius(spot.Origin, MIN_DIST_FROM_DM_SPAWN_POINT)
				pcount = 0
				for thing != nil {
					if thing.ClassName == "player" && thing.Health > 0 {
						pcount = pcount + 1
					}
					thing = thing.Chain
				}
				if pcount == 0 {
					spot.GoalEntity = spots
					spots = spot
					numspots = numspots + 1
				}
				spot = engine.Find(spot, "classname", "info_player_deathmatch")
			}
		}

		if numspots == 0 {
			if forceSpawn == FALSE {
				return nil
			}
			totalspots = engine.RInt(engine.Random() * totalspots)
			spot = engine.Find(World, "classname", "info_player_deathmatch")
			for totalspots > 0 {
				totalspots = totalspots - 1
				spot = engine.Find(spot, "classname", "info_player_deathmatch")
			}
			return spot
		}

		numspots = numspots - 1
		numspots = engine.RInt(engine.Random() * numspots)
		spot = spots
		for numspots > 0 {
			spot = spot.GoalEntity
			numspots = numspots - 1
		}
		return spot
	}

	if ServerFlags != 0 {
		spot = engine.Find(World, "classname", "info_player_start2")
		if spot != nil {
			return spot
		}
	}

	spot = engine.Find(World, "classname", "info_player_start")
	if spot == nil {
		engine.Error("PutClientInServer: no info_player_start on level")
	}

	return spot
}

func PutClientInServer() {
	var spot *quake.Entity

	Self.ClassName = "player"
	if Skill == 3 && Deathmatch == 0 {
		Self.Health = 50
	} else {
		Self.Health = 100
	}
	Self.TakeDamage = DAMAGE_AIM
	Self.Solid = SOLID_SLIDEBOX
	Self.MoveType = MOVETYPE_WALK
	Self.ShowHostile = 0
	if Skill == 3 && Deathmatch == 0 {
		Self.MaxHealth = 50
	} else {
		Self.MaxHealth = 100
	}
	Self.Flags = float32(FL_CLIENT)
	Self.AirFinished = Time + 12
	Self.Dmg = 2
	Self.SuperDamageFinished = 0
	Self.RadSuitFinished = 0
	Self.InvisibleFinished = 0
	Self.InvincibleFinished = 0
	Self.Effects = 0
	Self.InvincibleTime = 0
	Self.HealthRotNextCheck = 0
	Self.FiredWeapon = 0
	Self.TookDamage = 0
	Self.Team = TEAM_NONE

	if Coop != 0 {
		Self.Team = TEAM_HUMANS
	}

	DecodeLevelParms()
	W_SetCurrentAmmo()

	Self.AttackFinished = Time
	Self.ThPain = player_pain
	Self.ThDie = PlayerDie

	Self.DeadFlag = DEAD_NO
	Self.PauseTime = 0

	var shouldTelefrag float32
	if Self.SpawnDeferred > 0 && Time >= Self.SpawnDeferred {
		engine.Dprint("forcing telefrag on this spawn\n")
		shouldTelefrag = TRUE
	} else {
		shouldTelefrag = FALSE
	}

	spot = SelectSpawnPoint(shouldTelefrag)
	if spot == nil {
		Self.TakeDamage = DAMAGE_NO
		Self.Solid = SOLID_NOT
		Self.MoveType = MOVETYPE_NONE
		Self.DeadFlag = DEAD_DEAD
		engine.SetModel(Self, "")
		Self.ViewOfs = quake.MakeVec3(0, 0, 1)
		Self.Velocity = quake.MakeVec3(0, 0, 0)

		if Self.SpawnDeferred == 0 {
			engine.Dprint("no spawns available! deferring\n")
			Self.SpawnDeferred = Time + 5
		}

		spot = FindIntermission()
		Self.Angles = spot.Mangle
		Self.VAngle = spot.Mangle
		Self.FixAngle = 1
		Self.Origin = spot.Origin
		Self.WeaponModel = StringNull
		Self.WeaponFrame = 0
		Self.Weapon = 0
		return
	}

	Self.SpawnDeferred = 0
	engine.SetOrigin(Self, spot.Origin.Add(quake.MakeVec3(0, 0, 1)))
	Self.Angles = spot.Angles
	Self.FixAngle = 1

	engine.SetModel(Self, "progs/eyes.mdl")
	ModelIndexEyes = Self.ModelIndex

	engine.SetModel(Self, "progs/player.mdl")
	ModelIndexPlayer = Self.ModelIndex

	engine.SetSize(Self, VEC_HULL_MIN, VEC_HULL_MAX)
	Self.ViewOfs = quake.MakeVec3(0, 0, 22)
	Self.Velocity = quake.MakeVec3(0, 0, 0)

	player_stand1()

	if Deathmatch != 0 || Coop != 0 {
		makevectorsfixed(Self.Angles)
		spawn_tfog(Self.Origin.Add(VForward.Mul(20)))
	}

	spawn_tdeath(Self.Origin, Self)
	engine.StuffCmd(Self, "-attack\n")
}

func info_player_start() {}
func info_player_start2() {}
func testplayerstart() {}
func info_player_deathmatch() {}
func info_player_coop() {}

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

func PlayerDeathThink() {
	var forward float32

	if (int(Self.Flags) & FL_ONGROUND) != 0 {
		forward = engine.Vlen(Self.Velocity)
		forward = forward - 20
		if forward <= 0 {
			Self.Velocity = quake.MakeVec3(0, 0, 0)
		} else {
			Self.Velocity = engine.Normalize(Self.Velocity).Mul(forward)
		}
	}

	if Self.SpawnDeferred != 0 {
		spot := SelectSpawnPoint(FALSE)
		if spot != nil || Time >= Self.SpawnDeferred {
			respawn()
		}
		return
	}

	if Self.DeadFlag == float32(DEAD_DEAD) {
		if Self.Button2 != 0 || Self.Button1 != 0 || Self.Button0 != 0 {
			return
		}
		Self.DeadFlag = float32(DEAD_RESPAWNABLE)
		return
	}

	if Self.Button2 == 0 && Self.Button1 == 0 && Self.Button0 == 0 {
		return
	}

	Self.Button0 = 0
	Self.Button1 = 0
	Self.Button2 = 0
	respawn()
}

func PlayerJump() {
	if (int(Self.Flags) & FL_WATERJUMP) != 0 {
		return
	}

	if Self.WaterLevel >= 2 {
		if int(Self.WaterType) == CONTENT_WATER {
			Self.Velocity[2] = 100
		} else if int(Self.WaterType) == CONTENT_SLIME {
			Self.Velocity[2] = 80
		} else {
			Self.Velocity[2] = 50
		}

		if Self.SwimFlag < Time {
			Self.SwimFlag = Time + 1
			if engine.Random() < 0.5 {
				engine.Sound(Self, int(CHAN_BODY), "misc/water1.wav", 1, ATTN_NORM)
			} else {
				engine.Sound(Self, int(CHAN_BODY), "misc/water2.wav", 1, ATTN_NORM)
			}
		}
		return
	}

	if (int(Self.Flags) & FL_ONGROUND) == 0 {
		return
	}

	if (int(Self.Flags) & FL_JUMPRELEASED) == 0 {
		return
	}

	Self.Flags = Self.Flags - float32(int(Self.Flags)&FL_JUMPRELEASED)
	Self.Flags = Self.Flags - float32(FL_ONGROUND)

	Self.Button2 = 0
	engine.Sound(Self, int(CHAN_BODY), "player/plyrjmp8.wav", 1, ATTN_NORM)
	Self.Velocity[2] = Self.Velocity[2] + 270
}

func WaterMove() {
	if int(Self.MoveType) == MOVETYPE_NOCLIP {
		return
	}

	if Self.Health < 0 {
		return
	}

	if Self.WaterLevel != 3 {
		if Self.AirFinished < Time {
			engine.Sound(Self, int(CHAN_VOICE), "player/gasp2.wav", 1, ATTN_NORM)
		} else if Self.AirFinished < Time+9 {
			engine.Sound(Self, int(CHAN_VOICE), "player/gasp1.wav", 1, ATTN_NORM)
		}
		Self.AirFinished = Time + 12
		Self.Dmg = 2
	} else if Self.AirFinished < Time {
		if Self.PainFinished < Time {
			Self.Dmg = Self.Dmg + 2
			if Self.Dmg > 15 {
				Self.Dmg = 10
			}
			T_Damage(Self, World, World, Self.Dmg)
			Self.PainFinished = Time + 1
		}
	}

	if Self.WaterLevel == 0 {
		if (int(Self.Flags) & FL_INWATER) != 0 {
			engine.Sound(Self, int(CHAN_BODY), "misc/outwater.wav", 1, ATTN_NORM)
			Self.Flags = Self.Flags - float32(FL_INWATER)
		}
		return
	}

	if int(Self.WaterType) == CONTENT_LAVA {
		if Self.DmgTime < Time {
			if Self.RadSuitFinished > Time {
				Self.DmgTime = Time + 1
			} else {
				Self.DmgTime = Time + 0.2
			}
			T_Damage(Self, World, World, 10*Self.WaterLevel)
		}
	} else if int(Self.WaterType) == CONTENT_SLIME {
		if Self.DmgTime < Time && Self.RadSuitFinished < Time {
			Self.DmgTime = Time + 1
			T_Damage(Self, World, World, 4*Self.WaterLevel)
		}
	}

	if (int(Self.Flags) & FL_INWATER) == 0 {
		if int(Self.WaterType) == CONTENT_LAVA {
			engine.Sound(Self, int(CHAN_BODY), "player/inlava.wav", 1, ATTN_NORM)
		}
		if int(Self.WaterType) == CONTENT_WATER {
			engine.Sound(Self, int(CHAN_BODY), "player/inh2o.wav", 1, ATTN_NORM)
		}
		if int(Self.WaterType) == CONTENT_SLIME {
			engine.Sound(Self, int(CHAN_BODY), "player/slimbrn2.wav", 1, ATTN_NORM)
		}
		Self.Flags = Self.Flags + float32(FL_INWATER)
		Self.DmgTime = 0
	}

	if (int(Self.Flags) & FL_WATERJUMP) == 0 {
		Self.Velocity = Self.Velocity.Sub(Self.Velocity.Mul(0.8 * Self.WaterLevel * Frametime))
	}
}

func CheckWaterJump() {
	var start, end quake.Vec3

	makevectorsfixed(Self.Angles)
	start = Self.Origin
	start[2] = start[2] + 8
	VForward[2] = 0
	engine.Normalize(VForward)
	end = start.Add(VForward.Mul(24))
	engine.Traceline(start, end, TRUE, Self)

	if TraceFraction < 1 {
		start[2] = start[2] + Self.Maxs[2] - 8
		end = start.Add(VForward.Mul(24))
		Self.MoveDir = TracePlaneNormal.Mul(-50)
		engine.Traceline(start, end, TRUE, Self)

		if TraceFraction == 1 {
			Self.Flags = float32(int(Self.Flags) | FL_WATERJUMP)
			Self.Velocity[2] = 225
			Self.Flags = Self.Flags - float32(int(Self.Flags)&FL_JUMPRELEASED)
			Self.TeleportTime = Time + 2
			return
		}
	}
}

func PlayerPreThink() {
	if IntermissionRunning != 0 {
		IntermissionThink()
		return
	}

	if Self.ViewOfs == quake.MakeVec3(0, 0, 0) {
		return
	}

	engine.MakeVectors(Self.VAngle)

	if Deathmatch != 0 || Coop != 0 {
		CheckRules()
	}

	WaterMove()

	if Self.WaterLevel == 2 {
		CheckWaterJump()
	}

	if Self.DeadFlag >= float32(DEAD_DEAD) {
		PlayerDeathThink()
		return
	}

	if Self.DeadFlag == float32(DEAD_DYING) {
		return
	}

	if Self.Button2 != 0 {
		PlayerJump()
	} else {
		Self.Flags = float32(int(Self.Flags) | FL_JUMPRELEASED)
	}

	if Time < Self.PauseTime {
		Self.Velocity = quake.MakeVec3(0, 0, 0)
	}

	if Time > Self.AttackFinished && Self.CurrentAmmo == 0 && int(Self.Weapon) != IT_AXE {
		Self.Weapon = W_BestWeapon()
		W_SetCurrentAmmo()
	}
}

func CheckPowerups() {
	if Self.Health <= 0 {
		return
	}

	if Self.InvisibleFinished != 0 {
		if Self.InvisibleSound < Time {
			engine.Sound(Self, int(CHAN_AUTO), "items/inv3.wav", 0.5, ATTN_IDLE)
			Self.InvisibleSound = Time + ((engine.Random() * 3) + 1)
		}

		if Self.InvisibleFinished < Time+3 {
			if Self.InvisibleTime == 1 {
				engine.SPrint(Self, "$qc_ring_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/inv2.wav", 1, ATTN_NORM)
				Self.InvisibleTime = Time + 1
			}
			if Self.InvisibleTime < Time {
				Self.InvisibleTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.InvisibleFinished < Time {
			Self.Items = Self.Items - float32(IT_INVISIBILITY)
			Self.InvisibleFinished = 0
			Self.InvisibleTime = 0
		}

		Self.Frame = 0
		Self.ModelIndex = ModelIndexEyes
	} else {
		Self.ModelIndex = ModelIndexPlayer
	}

	if Self.InvincibleFinished != 0 {
		if Self.InvincibleFinished < Time+3 {
			if Self.InvincibleTime == 1 {
				engine.SPrint(Self, "$qc_protection_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/protect2.wav", 1, ATTN_NORM)
				Self.InvincibleTime = Time + 1
			}
			if Self.InvincibleTime < Time {
				Self.InvincibleTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.InvincibleFinished < Time {
			Self.Items = Self.Items - float32(IT_INVULNERABILITY)
			Self.InvincibleTime = 0
			Self.InvincibleFinished = 0
		}
		if Self.InvincibleFinished > Time {
			Self.Effects = float32(int(Self.Effects) | EF_PENTALIGHT)
		} else {
			Self.Effects = float32(int(Self.Effects) - (int(Self.Effects) & EF_PENTALIGHT))
		}
	}

	if Self.SuperDamageFinished != 0 {
		if Self.SuperDamageFinished < Time+3 {
			if Self.SuperTime == 1 {
				engine.SPrint(Self, "$qc_quad_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/damage2.wav", 1, ATTN_NORM)
				Self.SuperTime = Time + 1
			}
			if Self.SuperTime < Time {
				Self.SuperTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.SuperDamageFinished < Time {
			Self.Items = Self.Items - float32(IT_QUAD)
			Self.SuperDamageFinished = 0
			Self.SuperTime = 0
		}
		if Self.SuperDamageFinished > Time {
			Self.Effects = float32(int(Self.Effects) | EF_QUADLIGHT)
		} else {
			Self.Effects = float32(int(Self.Effects) - (int(Self.Effects) & EF_QUADLIGHT))
		}
	}

	if Self.RadSuitFinished != 0 {
		Self.AirFinished = Time + 12
		if Self.RadSuitFinished < Time+3 {
			if Self.RadTime == 1 {
				engine.SPrint(Self, "$qc_biosuit_fade")
				engine.StuffCmd(Self, "bf\n")
				engine.Sound(Self, int(CHAN_AUTO), "items/suit2.wav", 1, ATTN_NORM)
				Self.RadTime = Time + 1
			}
			if Self.RadTime < Time {
				Self.RadTime = Time + 1
				engine.StuffCmd(Self, "bf\n")
			}
		}

		if Self.RadSuitFinished < Time {
			Self.Items = Self.Items - float32(IT_SUIT)
			Self.RadTime = 0
			Self.RadSuitFinished = 0
		}
	}
}

func CheckHealthRot() {
	if (int(Self.Items) & IT_SUPERHEALTH) == 0 {
		return
	}

	if Self.HealthRotNextCheck > Time {
		return
	}

	if Self.Health > Self.MaxHealth {
		Self.Health = Self.Health - 1
		Self.HealthRotNextCheck = Time + 1
		return
	}

	Self.Items = Self.Items - float32(int(Self.Items)&IT_SUPERHEALTH)
	Self.HealthRotNextCheck = 0
}

func PlayerPostThink() {
	if Self.ViewOfs == quake.MakeVec3(0, 0, 0) {
		return
	}

	if Self.DeadFlag != 0 {
		return
	}

	W_WeaponFrame()

	if (Self.JumpFlag < -300) && (int(Self.Flags)&FL_ONGROUND) != 0 && (Self.Health > 0) {
		if int(Self.WaterType) == CONTENT_WATER {
			engine.Sound(Self, int(CHAN_BODY), "player/h2ojump.wav", 1, ATTN_NORM)
		} else if Self.JumpFlag < -650 {
			T_Damage(Self, World, World, 5)
			engine.Sound(Self, int(CHAN_VOICE), "player/land2.wav", 1, ATTN_NORM)
			if Self.Health <= 5 {
				Self.DeathType = "falling"
			}
		} else {
			engine.Sound(Self, int(CHAN_VOICE), "player/land.wav", 1, ATTN_NORM)
		}
		Self.JumpFlag = 0
	}

	if (int(Self.Flags) & FL_ONGROUND) == 0 {
		Self.JumpFlag = Self.Velocity[2]
	}

	CheckPowerups()
	CheckHealthRot()
}

func ClientConnect() {
	engine.Bprint(quake.Sprintf("$qc_entered %s", Self.NetName))
	if IntermissionRunning != 0 {
		ExitIntermission()
	}
}

func ClientDisconnect() {
	if Gameover != 0 {
		return
	}

	engine.Bprint(quake.Sprintf("$qc_left_game %s %s", Self.NetName, engine.Ftos(Self.Frags)))
	engine.Sound(Self, int(CHAN_BODY), "player/tornoff2.wav", 1, ATTN_NONE)
	Self.Effects = 0
	set_suicide_frame()
}
