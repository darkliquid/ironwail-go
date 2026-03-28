package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

func SUB_Null() {}

func SUB_Remove() {
	engine.Remove(Self)
}

func SetMovedir() {
	if Self.Angles == quake.MakeVec3(0, -1, 0) {
		Self.MoveDir = quake.MakeVec3(0, 0, 1)
	} else if Self.Angles == quake.MakeVec3(0, -2, 0) {
		Self.MoveDir = quake.MakeVec3(0, 0, -1)
	} else {
		engine.MakeVectors(Self.Angles)
		Self.MoveDir = VForward
	}

	Self.Angles = quake.MakeVec3(0, 0, 0)
}

func InitTrigger() {
	if Self.Angles != quake.MakeVec3(0, 0, 0) {
		SetMovedir()
	}

	Self.Solid = SOLID_TRIGGER
	engine.SetModel(Self, Self.Model)
	Self.MoveType = MOVETYPE_NONE
	Self.ModelIndex = 0
	Self.Model = StringNull
}

func SUB_CalcMoveEnt(ent *quake.Entity, tdest quake.Vec3, tspeed float32, fn quake.Func) {
	stemp := Self
	Self = ent

	SUB_CalcMove(tdest, tspeed, fn)
	Self = stemp
}

func SUB_CalcMove(tdest quake.Vec3, tspeed float32, fn quake.Func) {
	if tspeed == 0 {
		engine.ObjError("No speed is defined!")
	}

	Self.Think1 = fn
	Self.FinalDest = tdest
	Self.Think = SUB_CalcMoveDone

	if tdest == Self.Origin {
		Self.Velocity = quake.MakeVec3(0, 0, 0)
		Self.NextThink = Self.LTime + 0.1
		return
	}

	vdestdelta := tdest.Sub(Self.Origin)
	length := engine.Vlen(vdestdelta)
	traveltime := length / tspeed

	if traveltime < 0.1 {
		Self.Velocity = quake.MakeVec3(0, 0, 0)
		Self.NextThink = Self.LTime + 0.1
		return
	}

	Self.NextThink = Self.LTime + traveltime
	Self.Velocity = vdestdelta.Mul(1.0 / traveltime)
}

func SUB_CalcMoveDone() {
	engine.SetOrigin(Self, Self.FinalDest)
	Self.Velocity = quake.MakeVec3(0, 0, 0)
	Self.NextThink = -1

	if Self.Think1 != nil {
		Self.Think1()
	}
}

func SUB_CalcAngleMoveEnt(ent *quake.Entity, destangle quake.Vec3, tspeed float32, fn quake.Func) {
	stemp := Self
	Self = ent
	SUB_CalcAngleMove(destangle, tspeed, fn)
	Self = stemp
}

func SUB_CalcAngleMove(destangle quake.Vec3, tspeed float32, fn quake.Func) {
	if tspeed == 0 {
		engine.ObjError("No speed is defined!")
	}

	destdelta := destangle.Sub(Self.Angles)
	length := engine.Vlen(destdelta)
	traveltime := length / tspeed

	Self.NextThink = Self.LTime + traveltime
	Self.AVelocity = destdelta.Mul(1.0 / traveltime)

	Self.Think1 = fn
	Self.FinalAngle = destangle
	Self.Think = SUB_CalcAngleMoveDone
}

func SUB_CalcAngleMoveDone() {
	Self.Angles = Self.FinalAngle
	Self.AVelocity = quake.MakeVec3(0, 0, 0)
	Self.NextThink = -1

	if Self.Think1 != nil {
		Self.Think1()
	}
}

func DelayThink() {
	Activator = Self.Enemy
	SUB_UseTargets()
	engine.Remove(Self)
}

func SUB_UseTargets() {
	if Self.Delay != 0 {
		t := engine.Spawn()
		t.ClassName = "DelayedUse"
		t.NextThink = Time + Self.Delay
		t.Think = DelayThink
		t.Enemy = Activator
		t.Message = Self.Message
		t.KillTarget = Self.KillTarget
		t.Target = Self.Target
		return
	}

	if Activator.ClassName == "player" && Self.Message != StringNull {
		engine.Centerprint(Self.Message)

		if Self.Noise == "" {
			engine.Sound(Activator, int(CHAN_VOICE), "misc/talk.wav", 1, ATTN_NORM)
		}
	}

	if Self.KillTarget != StringNull {
		t := engine.Find(World, "targetname", Self.KillTarget)
		for t != nil {
			engine.Remove(t)
			t = engine.Find(t, "targetname", Self.KillTarget)
		}
	}

	if Self.Target != StringNull {
		act := Activator
		t := engine.Find(World, "targetname", Self.Target)
		for t != nil {
			stemp := Self
			otemp := Other
			Self = t
			Other = stemp
			if Self.Use != nil {
				Self.Use()
			}
			Self = stemp
			Other = otemp
			Activator = act
			t = engine.Find(t, "targetname", Self.Target)
		}
	}
}

func SUB_AttackFinished(normal float32) {
	Self.Cnt = 0
	Self.AttackFinished = Time + normal
}

func SUB_CheckRefire(thinkst quake.Func) {
	if Skill != 3 {
		return
	}

	if Self.Cnt == 1 {
		return
	}

	if visible(Self.Enemy) == 0 {
		return
	}

	Self.Cnt = 1
	Self.Think = thinkst
}
