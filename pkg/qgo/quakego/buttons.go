package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

type buttonEntity quake.Entity

func asButtonEntity(ent *quake.Entity) *buttonEntity {
	return (*buttonEntity)(ent)
}

func (be *buttonEntity) entity() *quake.Entity {
	return (*quake.Entity)(be)
}

func button_wait() {
	asButtonEntity(Self).wait()
}

func (be *buttonEntity) wait() {
	self := be.entity()

	self.State = float32(STATE_TOP)
	self.NextThink = self.LTime + self.Wait
	self.Think = button_return
	Activator = self.Enemy
	SUB_UseTargets()
	self.Frame = 1 // use alternate textures
}

func button_done() {
	asButtonEntity(Self).done()
}

func (be *buttonEntity) done() {
	be.entity().State = float32(STATE_BOTTOM)
}

func button_return() {
	asButtonEntity(Self).returnToStart()
}

func (be *buttonEntity) returnToStart() {
	self := be.entity()

	self.State = float32(STATE_DOWN)
	SUB_CalcMove(self.Pos1, self.Speed, button_done)
	self.Frame = 0 // use normal textures

	if self.Health != 0 {
		self.TakeDamage = DAMAGE_YES // can be shot again
	}
}

func button_blocked() {
	// do nothing, just don't come all the way back out
}

func button_fire() {
	asButtonEntity(Self).fire()
}

func (be *buttonEntity) fire() {
	self := be.entity()

	if self.State == float32(STATE_UP) || self.State == float32(STATE_TOP) {
		return
	}

	engine.Sound(self, int(CHAN_VOICE), self.Noise, 1, ATTN_NORM)

	self.State = float32(STATE_UP)
	SUB_CalcMove(self.Pos2, self.Speed, button_wait)
}

func button_use() {
	Self.Enemy = Activator
	button_fire()
}

func button_touch() {
	if Other.ClassName != "player" {
		return
	}

	Self.Enemy = Other
	button_fire()
}

func button_killed() {
	Self.Enemy = DamageAttacker
	Self.Health = Self.MaxHealth
	Self.TakeDamage = DAMAGE_NO // will be reset upon return
	button_fire()
}

func func_button() {
	if Self.Sounds == 0 {
		engine.PrecacheSound("buttons/airbut1.wav")
		Self.Noise = "buttons/airbut1.wav"
	}

	if Self.Sounds == 1 {
		engine.PrecacheSound("buttons/switch21.wav")
		Self.Noise = "buttons/switch21.wav"
	}

	if Self.Sounds == 2 {
		engine.PrecacheSound("buttons/switch02.wav")
		Self.Noise = "buttons/switch02.wav"
	}

	if Self.Sounds == 3 {
		engine.PrecacheSound("buttons/switch04.wav")
		Self.Noise = "buttons/switch04.wav"
	}

	SetMovedir()

	Self.ClassName = "func_button"
	Self.MoveType = MOVETYPE_PUSH
	Self.Solid = SOLID_BSP
	engine.SetModel(Self, Self.Model)

	Self.Blocked = button_blocked
	Self.Use = button_use

	if Self.Health != 0 {
		Self.MaxHealth = Self.Health
		Self.ThDie = button_killed
		Self.TakeDamage = DAMAGE_YES
	} else {
		Self.Touch = button_touch
	}

	if Self.Speed == 0 {
		Self.Speed = 40
	}

	if Self.Wait == 0 {
		Self.Wait = 1
	}

	if Self.Lip == 0 {
		Self.Lip = 4
	}

	Self.State = float32(STATE_BOTTOM)

	Self.Pos1 = Self.Origin
	movedir_fabs := quake.MakeVec3(engine.FAbs(Self.MoveDir[0]), engine.FAbs(Self.MoveDir[1]), engine.FAbs(Self.MoveDir[2]))

	// self.pos2 = self.pos1 + ((movedir_fabs * self.size) - self.lip) * self.movedir;
	dot := movedir_fabs.Dot(Self.Size)
	Self.Pos2 = Self.Pos1.Add(Self.MoveDir.Mul(dot - Self.Lip))
}
