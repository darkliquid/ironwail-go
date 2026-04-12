package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

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
	LastSpawn                      *quake.Entity
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

func info_player_start()      {}
func info_player_start2()     {}
func testplayerstart()        {}
func info_player_deathmatch() {}
func info_player_coop()       {}
