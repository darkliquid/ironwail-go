package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

const (
	doorFlagStartOpen = 1 << iota
	_
	doorFlagDontLink
	doorFlagGoldKey
	doorFlagSilverKey
	doorFlagToggle
)

func door_blocked() {
	T_Damage(Other, Self, Self, Self.Dmg)

	if Self.Wait >= 0 {
		if Self.State == float32(STATE_DOWN) {
			door_go_up()
		} else {
			door_go_down()
		}
	}
}

func door_hit_top() {
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise1, 1, ATTN_NORM)
	Self.State = float32(STATE_TOP)

	if (int(Self.SpawnFlags) & doorFlagToggle) != 0 {
		return
	}

	Self.Think = door_go_down
	Self.NextThink = Self.LTime + Self.Wait
}

func door_hit_bottom() {
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise1, 1, ATTN_NORM)
	Self.State = float32(STATE_BOTTOM)
}

func door_go_down_impl() {
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise2, 1, ATTN_NORM)

	if Self.MaxHealth != 0 {
		Self.TakeDamage = DAMAGE_YES
		Self.Health = Self.MaxHealth
	}

	Self.State = float32(STATE_DOWN)
	SUB_CalcMove(Self.Pos1, Self.Speed, door_hit_bottom)
}

func door_go_up_impl() {
	if Self.State == float32(STATE_UP) {
		return
	}

	if Self.State == float32(STATE_TOP) {
		Self.NextThink = Self.LTime + Self.Wait
		return
	}

	engine.Sound(Self, int(CHAN_VOICE), Self.Noise2, 1, ATTN_NORM)
	Self.State = float32(STATE_UP)
	SUB_CalcMove(Self.Pos2, Self.Speed, door_hit_top)

	SUB_UseTargets()
}

func init() {
	door_go_down = door_go_down_impl
	door_go_up = door_go_up_impl
}

func door_fire() {
	var oself *quake.Entity
	var starte *quake.Entity

	if Self.Owner != Self {
		engine.ObjError("door_fire: self.owner != self")
	}

	if Self.Items != 0 {
		engine.Sound(Self, int(CHAN_ITEM), Self.Noise4, 1, ATTN_NORM)
	}

	Self.Message = StringNull
	oself = Self

	if (int(Self.SpawnFlags) & doorFlagToggle) != 0 {
		if Self.State == float32(STATE_UP) || Self.State == float32(STATE_TOP) {
			starte = Self
			for {
				door_go_down()
				Self = Self.Enemy
				if Self == starte || Self == nil {
					break
				}
			}
			Self = oself
			return
		}
	}

	starte = Self
	for {
		door_go_up()
		Self = Self.Enemy
		if Self == starte || Self == nil {
			break
		}
	}

	Self = oself
}

func door_use() {
	oself := Self

	Self.Message = StringNull
	Self.Owner.Message = StringNull
	Self.Enemy.Message = StringNull
	Self = Self.Owner
	door_fire()
	Self = oself
}

func door_trigger_touch() {
	if Other.Health <= 0 {
		return
	}

	if Time < Self.AttackFinished {
		return
	}

	Self.AttackFinished = Time + 1
	Activator = Other

	Self = Self.Owner
	door_use()
}

func door_killed() {
	oself := Self

	Self = Self.Owner
	Self.Health = Self.MaxHealth
	Self.TakeDamage = DAMAGE_NO
	door_use()
	Self = oself
}

func door_touch() {
	if Other.ClassName != "player" {
		return
	}

	if Self.Owner.AttackFinished > Time {
		return
	}

	Self.Owner.AttackFinished = Time + 2

	if Self.Owner.Message != StringNull {
		engine.Centerprint(Self.Owner.Message)
		engine.Sound(Other, int(CHAN_VOICE), "misc/talk.wav", 1, ATTN_NORM)
	}

	if Self.Items == 0 {
		return
	}

	if (int(Self.Items) & int(Other.Items)) != int(Self.Items) {
		if Self.Owner.Items == float32(IT_KEY1) {
			if int(World.WorldType) == WORLDTYPE_BASE {
				engine.Centerprint("$qc_need_silver_keycard")
				engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
			} else if int(World.WorldType) == WORLDTYPE_METAL {
				engine.Centerprint("$qc_need_silver_runekey")
				engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
			} else if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
				engine.Centerprint("$qc_need_silver_key")
				engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
			}
		} else {
			if int(World.WorldType) == WORLDTYPE_BASE {
				engine.Centerprint("$qc_need_gold_keycard")
				engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
			} else if int(World.WorldType) == WORLDTYPE_METAL {
				engine.Centerprint("$qc_need_gold_runekey")
				engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
			} else if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
				engine.Centerprint("$qc_need_gold_key")
				engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
			}
		}
		return
	}

	Other.Items = float32(int(Other.Items) - int(Self.Items))
	Self.Touch = SUB_Null

	if Self.Enemy != nil {
		Self.Enemy.Touch = SUB_Null
	}

	door_use()
}

func spawn_field(fmins, fmaxs quake.Vec3) *quake.Entity {
	trigger := engine.Spawn()
	trigger.MoveType = MOVETYPE_NONE
	trigger.Solid = SOLID_TRIGGER
	trigger.Owner = Self
	trigger.Touch = door_trigger_touch

	engine.SetSize(trigger, fmins.Sub(quake.MakeVec3(60, 60, 8)), fmaxs.Add(quake.MakeVec3(60, 60, 8)))
	return trigger
}

func EntitiesTouching(e1, e2 *quake.Entity) float32 {
	if e1.Mins[0] > e2.Maxs[0] {
		return FALSE
	}
	if e1.Mins[1] > e2.Maxs[1] {
		return FALSE
	}
	if e1.Mins[2] > e2.Maxs[2] {
		return FALSE
	}
	if e1.Maxs[0] < e2.Mins[0] {
		return FALSE
	}
	if e1.Maxs[1] < e2.Mins[1] {
		return FALSE
	}
	if e1.Maxs[2] < e2.Mins[2] {
		return FALSE
	}

	return TRUE
}

func LinkDoors() {
	var t, starte *quake.Entity
	var cmins, cmaxs quake.Vec3

	if Self.Enemy != nil {
		return
	}

	if (int(Self.SpawnFlags) & doorFlagDontLink) != 0 {
		Self.Owner = Self
		Self.Enemy = Self
		return
	}

	cmins = Self.Mins
	cmaxs = Self.Maxs

	starte = Self
	t = Self

	for {
		Self.Owner = starte

		if Self.Health != 0 {
			starte.Health = Self.Health
		}

		if Self.TargetName != StringNull {
			starte.TargetName = Self.TargetName
		}

		if Self.Message != StringNull {
			starte.Message = Self.Message
		}

		t = engine.Find(t, "classname", Self.ClassName)
		if t == nil {
			Self.Enemy = starte
			Self = Self.Owner

			if Self.Health != 0 {
				return
			}
			if Self.TargetName != StringNull {
				return
			}
			if Self.Items != 0 {
				return
			}

			Self.Owner.TriggerField = spawn_field(cmins, cmaxs)
			return
		}

		if EntitiesTouching(Self, t) != 0 {
			if t.Enemy != nil {
				engine.ObjError("cross connected doors")
			}

			Self.Enemy = t
			Self = t

			if t.Mins[0] < cmins[0] {
				cmins[0] = t.Mins[0]
			}
			if t.Mins[1] < cmins[1] {
				cmins[1] = t.Mins[1]
			}
			if t.Mins[2] < cmins[2] {
				cmins[2] = t.Mins[2]
			}
			if t.Maxs[0] > cmaxs[0] {
				cmaxs[0] = t.Maxs[0]
			}
			if t.Maxs[1] > cmaxs[1] {
				cmaxs[1] = t.Maxs[1]
			}
			if t.Maxs[2] > cmaxs[2] {
				cmaxs[2] = t.Maxs[2]
			}
		}
	}
}

func func_door() {
	if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
		engine.PrecacheSound("doors/medtry.wav")
		engine.PrecacheSound("doors/meduse.wav")
		Self.Noise3 = "doors/medtry.wav"
		Self.Noise4 = "doors/meduse.wav"
	} else if int(World.WorldType) == WORLDTYPE_METAL {
		engine.PrecacheSound("doors/runetry.wav")
		engine.PrecacheSound("doors/runeuse.wav")
		Self.Noise3 = "doors/runetry.wav"
		Self.Noise4 = "doors/runeuse.wav"
	} else if int(World.WorldType) == WORLDTYPE_BASE {
		engine.PrecacheSound("doors/basetry.wav")
		engine.PrecacheSound("doors/baseuse.wav")
		Self.Noise3 = "doors/basetry.wav"
		Self.Noise4 = "doors/baseuse.wav"
	} else {
		engine.Dprint("no worldtype set!\n")
	}

	if Self.Sounds == 0 {
		engine.PrecacheSound("misc/null.wav")
		Self.Noise1 = "misc/null.wav"
		Self.Noise2 = "misc/null.wav"
	}

	if Self.Sounds == 1 {
		engine.PrecacheSound("doors/drclos4.wav")
		engine.PrecacheSound("doors/doormv1.wav")
		Self.Noise1 = "doors/drclos4.wav"
		Self.Noise2 = "doors/doormv1.wav"
	}

	if Self.Sounds == 2 {
		engine.PrecacheSound("doors/hydro1.wav")
		engine.PrecacheSound("doors/hydro2.wav")
		Self.Noise2 = "doors/hydro1.wav"
		Self.Noise1 = "doors/hydro2.wav"
	}

	if Self.Sounds == 3 {
		engine.PrecacheSound("doors/stndr1.wav")
		engine.PrecacheSound("doors/stndr2.wav")
		Self.Noise2 = "doors/stndr1.wav"
		Self.Noise1 = "doors/stndr2.wav"
	}

	if Self.Sounds == 4 {
		engine.PrecacheSound("doors/ddoor1.wav")
		engine.PrecacheSound("doors/ddoor2.wav")
		Self.Noise1 = "doors/ddoor2.wav"
		Self.Noise2 = "doors/ddoor1.wav"
	}

	SetMovedir()

	Self.MaxHealth = Self.Health
	Self.Solid = SOLID_BSP
	Self.MoveType = MOVETYPE_PUSH
	engine.SetOrigin(Self, Self.Origin)
	engine.SetModel(Self, Self.Model)
	Self.ClassName = "func_door"

	Self.Blocked = door_blocked
	Self.Use = door_use

	if (int(Self.SpawnFlags) & doorFlagSilverKey) != 0 {
		Self.Items = float32(IT_KEY1)
	}

	if (int(Self.SpawnFlags) & doorFlagGoldKey) != 0 {
		Self.Items = float32(IT_KEY2)
	}

	if Self.Speed == 0 {
		Self.Speed = 100
	}

	if Self.Wait == 0 {
		Self.Wait = 3
	}

	if Self.Lip == 0 {
		Self.Lip = 8
	}

	if Self.Dmg == 0 {
		Self.Dmg = 2
	}

	Self.Pos1 = Self.Origin
	movedir_fabs := quake.MakeVec3(engine.FAbs(Self.MoveDir[0]), engine.FAbs(Self.MoveDir[1]), engine.FAbs(Self.MoveDir[2]))

	dot := movedir_fabs.Dot(Self.Size)
	Self.Pos2 = Self.Pos1.Add(Self.MoveDir.Mul(dot - Self.Lip))

	if (int(Self.SpawnFlags) & doorFlagStartOpen) != 0 {
		engine.SetOrigin(Self, Self.Pos2)
		Self.Pos2 = Self.Pos1
		Self.Pos1 = Self.Origin
	}

	Self.State = float32(STATE_BOTTOM)

	if Self.Health != 0 {
		Self.TakeDamage = DAMAGE_YES
		Self.ThDie = door_killed
	}

	if Self.Items != 0 {
		Self.Wait = -1
	}

	Self.Touch = door_touch

	Self.Think = LinkDoors
	Self.NextThink = Self.LTime + 0.1
}

const (
	SECRET_OPEN_ONCE = 1 << iota
	SECRET_1ST_LEFT
	SECRET_1ST_DOWN
	SECRET_NO_SHOOT
	SECRET_YES_SHOOT
)

func SUB_Null_Pain(attacker *quake.Entity, damage float32)      {}
func fd_secret_use_pain(attacker *quake.Entity, damage float32) { fd_secret_use() }

func fd_secret_use() {
	var temp float32

	Self.Health = 10000

	if Self.Origin != Self.OldOrigin {
		return
	}

	Self.Message = StringNull

	SUB_UseTargets()

	if (int(Self.SpawnFlags) & int(SECRET_NO_SHOOT)) == 0 {
		Self.ThPain = SUB_Null_Pain
		Self.TakeDamage = DAMAGE_NO
	}

	Self.Velocity = quake.MakeVec3(0, 0, 0)

	engine.Sound(Self, int(CHAN_VOICE), Self.Noise1, 1, ATTN_NORM)
	Self.NextThink = Self.LTime + 0.1

	temp = 1 - float32(int(Self.SpawnFlags)&int(SECRET_1ST_LEFT)) // 1 or -1
	engine.MakeVectors(Self.Mangle)

	if Self.TWidth == 0 {
		if (int(Self.SpawnFlags) & int(SECRET_1ST_DOWN)) != 0 {
			Self.TWidth = engine.FAbs(VUp.Dot(Self.Size))
		} else {
			Self.TWidth = engine.FAbs(VRight.Dot(Self.Size))
		}
	}

	if Self.TLength == 0 {
		Self.TLength = engine.FAbs(VForward.Dot(Self.Size))
	}

	if (int(Self.SpawnFlags) & int(SECRET_1ST_DOWN)) != 0 {
		Self.Dest1 = Self.Origin.Sub(VUp.Mul(Self.TWidth))
	} else {
		Self.Dest1 = Self.Origin.Add(VRight.Mul(Self.TWidth * temp))
	}

	Self.Dest2 = Self.Dest1.Add(VForward.Mul(Self.TLength))
	SUB_CalcMove(Self.Dest1, Self.Speed, fd_secret_move1)
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise2, 1, ATTN_NORM)
}

func fd_secret_move1_impl() {
	Self.NextThink = Self.LTime + 1.0
	Self.Think = fd_secret_move2
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
}

func fd_secret_move2_impl() {
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise2, 1, ATTN_NORM)
	SUB_CalcMove(Self.Dest2, Self.Speed, fd_secret_move3)
}

func fd_secret_move3_impl() {
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)

	if (int(Self.SpawnFlags) & int(SECRET_OPEN_ONCE)) == 0 {
		Self.NextThink = Self.LTime + Self.Wait
		Self.Think = fd_secret_move4
	}
}

func fd_secret_move4_impl() {
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise2, 1, ATTN_NORM)
	SUB_CalcMove(Self.Dest1, Self.Speed, fd_secret_move5)
}

func fd_secret_move5_impl() {
	Self.NextThink = Self.LTime + 1.0
	Self.Think = fd_secret_move6
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
}

func fd_secret_move6_impl() {
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise2, 1, ATTN_NORM)
	SUB_CalcMove(Self.OldOrigin, Self.Speed, fd_secret_done)
}

func fd_secret_done_impl() {
	if Self.TargetName == StringNull || (int(Self.SpawnFlags)&int(SECRET_YES_SHOOT)) != 0 {
		Self.Health = 10000
		Self.TakeDamage = DAMAGE_YES
		Self.ThPain = fd_secret_use_pain
		Self.ThDie = fd_secret_use
	}
	engine.Sound(Self, int(CHAN_VOICE), Self.Noise3, 1, ATTN_NORM)
}

func init() {
	fd_secret_move1 = fd_secret_move1_impl
	fd_secret_move2 = fd_secret_move2_impl
	fd_secret_move3 = fd_secret_move3_impl
	fd_secret_move4 = fd_secret_move4_impl
	fd_secret_move5 = fd_secret_move5_impl
	fd_secret_move6 = fd_secret_move6_impl
	fd_secret_done = fd_secret_done_impl
}

func secret_blocked() {
	if Time < Self.AttackFinished {
		return
	}

	Self.AttackFinished = Time + 0.5
	T_Damage(Other, Self, Self, Self.Dmg)
}

func secret_touch() {
	if Other.ClassName != "player" {
		return
	}

	if Self.AttackFinished > Time {
		return
	}

	Self.AttackFinished = Time + 2

	if Self.Message != StringNull {
		engine.Centerprint(Self.Message)
		engine.Sound(Other, int(CHAN_BODY), "misc/talk.wav", 1, ATTN_NORM)
	}
}

func func_door_secret() {
	if Self.Sounds == 0 {
		Self.Sounds = 3
	}

	if Self.Sounds == 1 {
		engine.PrecacheSound("doors/latch2.wav")
		engine.PrecacheSound("doors/winch2.wav")
		engine.PrecacheSound("doors/drclos4.wav")
		Self.Noise1 = "doors/latch2.wav"
		Self.Noise2 = "doors/winch2.wav"
		Self.Noise3 = "doors/drclos4.wav"
	}

	if Self.Sounds == 2 {
		engine.PrecacheSound("doors/airdoor1.wav")
		engine.PrecacheSound("doors/airdoor2.wav")
		Self.Noise2 = "doors/airdoor1.wav"
		Self.Noise1 = "doors/airdoor2.wav"
		Self.Noise3 = "doors/airdoor2.wav"
	}

	if Self.Sounds == 3 {
		engine.PrecacheSound("doors/basesec1.wav")
		engine.PrecacheSound("doors/basesec2.wav")
		Self.Noise2 = "doors/basesec1.wav"
		Self.Noise1 = "doors/basesec2.wav"
		Self.Noise3 = "doors/basesec2.wav"
	}

	if Self.Dmg == 0 {
		Self.Dmg = 2
	}

	Self.Mangle = Self.Angles
	Self.Angles = quake.MakeVec3(0, 0, 0)
	Self.Solid = SOLID_BSP
	Self.MoveType = MOVETYPE_PUSH
	Self.ClassName = "func_door_secret"
	engine.SetModel(Self, Self.Model)
	engine.SetOrigin(Self, Self.Origin)

	Self.Touch = secret_touch
	Self.Blocked = secret_blocked
	Self.Speed = 50
	Self.Use = fd_secret_use

	if Self.TargetName == StringNull || (int(Self.SpawnFlags)&int(SECRET_YES_SHOOT)) != 0 {
		Self.Health = 10000
		Self.TakeDamage = DAMAGE_YES
		Self.ThPain = fd_secret_use_pain
		Self.ThDie = fd_secret_use
	}

	Self.OldOrigin = Self.Origin

	if Self.Wait == 0 {
		Self.Wait = 5
	}
}
