package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

func monster_use() {
	if Self.Enemy != nil {
		return
	}

	if Self.Health <= 0 {
		return
	}

	if (int(Activator.Items) & IT_INVISIBILITY) != 0 {
		return
	}

	if (int(Activator.Flags) & FL_NOTARGET) != 0 {
		return
	}

	if Activator.ClassName != "player" {
		return
	}

	Self.Enemy = Activator
	Self.NextThink = Time + 0.1
	Self.Think = FoundTarget
}

func monster_death_use() {
	if (int(Self.Flags) & FL_FLY) != 0 {
		Self.Flags = Self.Flags - float32(FL_FLY)
	}

	if (int(Self.Flags) & FL_SWIM) != 0 {
		Self.Flags = Self.Flags - float32(FL_SWIM)
	}

	if Self.Target == StringNull {
		return
	}

	Activator = Self.Enemy
	SUB_UseTargets()
}

func walkmonster_start_go() {
	Self.Origin[2] = Self.Origin[2] + 1
	engine.DropToFloor()

	if engine.WalkMove(0, 0) == 0 {
		engine.Dprint("walkmonster in wall at: ")
		engine.Dprint(engine.Vtos(Self.Origin))
		engine.Dprint("\n")
	}

	Self.TakeDamage = DAMAGE_AIM
	Self.IdealYaw = Self.Angles[1]

	if Self.YawSpeed == 0 {
		Self.YawSpeed = 20
	}

	Self.ViewOfs = quake.MakeVec3(0, 0, 25)
	Self.Use = monster_use
	Self.Team = TEAM_MONSTERS
	Self.Flags = float32(int(Self.Flags) | FL_MONSTER)

	if Self.Target != StringNull {
		Self.GoalEntity = engine.Find(World, "targetname", Self.Target)
		Self.MoveTarget = Self.GoalEntity
		Self.IdealYaw = engine.Vectoyaw(Self.GoalEntity.Origin.Sub(Self.Origin))

		if Self.MoveTarget == nil {
			engine.Dprint("Monster can't find target at ")
			engine.Dprint(engine.Vtos(Self.Origin))
			engine.Dprint("\n")
		}

		if Self.MoveTarget != nil && Self.MoveTarget.ClassName == "path_corner" {
			if Self.ThWalk != nil {
				Self.ThWalk()
			}
		} else {
			Self.PauseTime = 99999999
			if Self.ThStand != nil {
				Self.ThStand()
			}
		}
	} else {
		Self.PauseTime = 99999999
		if Self.ThStand != nil {
			Self.ThStand()
		}
	}

	Self.NextThink = Self.NextThink + engine.Random()*0.5
}

func walkmonster_start() {
	Self.NextThink = Self.NextThink + engine.Random()*0.5
	Self.Think = walkmonster_start_go
	TotalMonsters = TotalMonsters + 1
}

func flymonster_start_go() {
	Self.TakeDamage = DAMAGE_AIM
	Self.IdealYaw = Self.Angles[1]

	if Self.YawSpeed == 0 {
		Self.YawSpeed = 10
	}

	Self.ViewOfs = quake.MakeVec3(0, 0, 25)
	Self.Use = monster_use
	Self.Team = TEAM_MONSTERS
	Self.Flags = float32(int(Self.Flags) | FL_FLY)
	Self.Flags = float32(int(Self.Flags) | FL_MONSTER)

	if engine.WalkMove(0, 0) == 0 {
		engine.Dprint("flymonster in wall at: ")
		engine.Dprint(engine.Vtos(Self.Origin))
		engine.Dprint("\n")
	}

	if Self.Target != StringNull {
		Self.GoalEntity = engine.Find(World, "targetname", Self.Target)
		Self.MoveTarget = Self.GoalEntity

		if Self.MoveTarget == nil {
			engine.Dprint("Monster can't find target at ")
			engine.Dprint(engine.Vtos(Self.Origin))
			engine.Dprint("\n")
		}

		if Self.MoveTarget != nil && Self.MoveTarget.ClassName == "path_corner" {
			if Self.ThWalk != nil {
				Self.ThWalk()
			}
		} else {
			Self.PauseTime = 99999999
			if Self.ThStand != nil {
				Self.ThStand()
			}
		}
	} else {
		Self.PauseTime = 99999999
		if Self.ThStand != nil {
			Self.ThStand()
		}
	}
}

func flymonster_start() {
	Self.NextThink = Self.NextThink + engine.Random()*0.5
	Self.Think = flymonster_start_go
	TotalMonsters = TotalMonsters + 1
}

func swimmonster_start_go() {
	if Deathmatch != 0 {
		engine.Remove(Self)
		return
	}

	Self.TakeDamage = DAMAGE_AIM
	Self.IdealYaw = Self.Angles[1]

	if Self.YawSpeed == 0 {
		Self.YawSpeed = 10
	}

	Self.ViewOfs = quake.MakeVec3(0, 0, 10)
	Self.Use = monster_use
	Self.Team = TEAM_MONSTERS
	Self.Flags = float32(int(Self.Flags) | FL_SWIM)
	Self.Flags = float32(int(Self.Flags) | FL_MONSTER)

	if Self.Target != StringNull {
		Self.GoalEntity = engine.Find(World, "targetname", Self.Target)
		Self.MoveTarget = Self.GoalEntity

		if Self.MoveTarget == nil {
			engine.Dprint("Monster can't find target at ")
			engine.Dprint(engine.Vtos(Self.Origin))
			engine.Dprint("\n")
		}

		if Self.GoalEntity != nil {
			Self.IdealYaw = engine.Vectoyaw(Self.GoalEntity.Origin.Sub(Self.Origin))
		}

		if Self.ThWalk != nil {
			Self.ThWalk()
		}
	} else {
		Self.PauseTime = 99999999
		if Self.ThStand != nil {
			Self.ThStand()
		}
	}

	Self.NextThink = Self.NextThink + engine.Random()*0.5
}

func swimmonster_start() {
	Self.NextThink = Self.NextThink + engine.Random()*0.5
	Self.Think = swimmonster_start_go
	TotalMonsters = TotalMonsters + 1
}
