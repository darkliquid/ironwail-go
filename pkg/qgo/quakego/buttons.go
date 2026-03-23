package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

func button_wait() {
	Self.State = float32(STATE_TOP)
	Self.NextThink = Self.LTime + Self.Wait
	Self.Think = button_return
	Activator = Self.Enemy
	SUB_UseTargets()
	Self.Frame = 1 // use alternate textures
}

func button_done() {
	Self.State = float32(STATE_BOTTOM)
}

func button_return() {
	Self.State = float32(STATE_DOWN)
	SUB_CalcMove(Self.Pos1, Self.Speed, button_done)
	Self.Frame = 0 // use normal textures

	if Self.Health != 0 {
		Self.TakeDamage = DAMAGE_YES // can be shot again
	}
}

func button_blocked() {
	// do nothing, just don't come all the way back out
}

func button_fire() {
	if Self.State == float32(STATE_UP) || Self.State == float32(STATE_TOP) {
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), Self.Noise, 1, ATTN_NORM)

	Self.State = float32(STATE_UP)
	SUB_CalcMove(Self.Pos2, Self.Speed, button_wait)
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
