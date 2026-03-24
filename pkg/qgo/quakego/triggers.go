package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

type triggerEntity quake.Entity

func asTriggerEntity(ent *quake.Entity) *triggerEntity {
	return (*triggerEntity)(ent)
}

func (te *triggerEntity) entity() *quake.Entity {
	return (*quake.Entity)(te)
}

func trigger_reactivate() {
	asTriggerEntity(Self).reactivate()
}

func (te *triggerEntity) reactivate() {
	te.entity().Solid = SOLID_TRIGGER
}

var (
	SPAWNFLAG_NOMESSAGE float32 = 1
	SPAWNFLAG_NOTOUCH   float32 = 1
)

func multi_wait() {
	asTriggerEntity(Self).wait()
}

func (te *triggerEntity) wait() {
	self := te.entity()

	if self.MaxHealth != 0 {
		self.Health = self.MaxHealth
		self.TakeDamage = DAMAGE_YES
		self.Solid = SOLID_BBOX
	}
}

func multi_trigger() {
	asTriggerEntity(Self).trigger(Other)
}

func (te *triggerEntity) trigger(toucher *quake.Entity) {
	self := te.entity()

	if self.NextThink > Time {
		return
	}

	if self.ClassName == "trigger_secret" {
		if toucher.ClassName != "player" {
			return
		}
		FoundSecrets = FoundSecrets + 1
		engine.WriteByte(MSG_ALL, float32(SVC_FOUNDSECRET))

		MsgEntity = toucher
		engine.WriteByte(MSG_ONE, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ONE, "ACH_FIND_SECRET")
	}

	if self.Noise != StringNull {
		engine.Sound(self, int(CHAN_VOICE), self.Noise, 1, ATTN_NORM)
	}

	self.TakeDamage = DAMAGE_NO
	Activator = toucher

	SUB_UseTargets()

	if self.Wait > 0 {
		self.Think = te.wait
		self.NextThink = Time + self.Wait
	} else {
		self.Touch = SUB_Null
		self.NextThink = Time + 0.1
		self.Think = SUB_Remove
	}

	if Activator.ClassName == "player" && World.Model == "maps/e2m3.bsp" && self.Message == "$map_dopefish" {
		MsgEntity = Activator
		engine.WriteByte(MSG_ONE, float32(SVC_ACHIEVEMENT))
		engine.WriteString(MSG_ONE, "ACH_FIND_DOPEFISH")
	}
}

func multi_killed() {
	asTriggerEntity(Self).killed()
}

func (te *triggerEntity) killed() {
	Other = DamageAttacker
	te.trigger(Other)
}

func multi_use() {
	asTriggerEntity(Self).use()
}

func (te *triggerEntity) use() {
	Other = Activator
	te.trigger(Other)
}

func multi_touch() {
	asTriggerEntity(Self).touch(Other)
}

func (te *triggerEntity) touch(toucher *quake.Entity) {
	if toucher.ClassName != "player" {
		return
	}

	self := te.entity()
	if self.MoveDir != quake.MakeVec3(0, 0, 0) {
		engine.MakeVectors(toucher.Angles)
		if VForward.Dot(self.MoveDir) < 0 {
			return
		}
	}

	te.trigger(toucher)
}

func trigger_multiple() {
	asTriggerEntity(Self).setupMultiple()
}

func (te *triggerEntity) setupMultiple() {
	self := te.entity()

	if self.Sounds == 1 {
		engine.PrecacheSound("misc/secret.wav")
		self.Noise = "misc/secret.wav"
	} else if self.Sounds == 2 {
		engine.PrecacheSound("misc/talk.wav")
		self.Noise = "misc/talk.wav"
	} else if self.Sounds == 3 {
		engine.PrecacheSound("misc/trigger1.wav")
		self.Noise = "misc/trigger1.wav"
	}

	if self.Wait == 0 {
		self.Wait = 0.2
	}

	self.Use = te.use

	InitTrigger()

	if self.Health != 0 {
		if (int(self.SpawnFlags) & int(SPAWNFLAG_NOTOUCH)) != 0 {
			engine.ObjError("health and notouch don't make sense\n")
		}

		self.MaxHealth = self.Health
		self.ThDie = multi_killed
		self.TakeDamage = DAMAGE_YES
		self.Solid = SOLID_BBOX
		engine.SetOrigin(self, self.Origin)
	} else {
		if (int(self.SpawnFlags) & int(SPAWNFLAG_NOTOUCH)) == 0 {
			self.Touch = multi_touch
		}
	}
}

func trigger_once() {
	asTriggerEntity(Self).setupOnce()
}

func (te *triggerEntity) setupOnce() {
	te.entity().Wait = -1
	te.setupMultiple()
}

func trigger_relay() {
	Self.Use = SUB_UseTargets
}

func trigger_secret() {
	TotalSecrets = TotalSecrets + 1
	Self.Wait = -1

	if Self.Message == StringNull {
		Self.Message = "$qc_found_secret"
	}

	if Self.Sounds == 0 {
		Self.Sounds = 1
	}

	if Self.Sounds == 1 {
		engine.PrecacheSound("misc/secret.wav")
		Self.Noise = "misc/secret.wav"
	} else if Self.Sounds == 2 {
		engine.PrecacheSound("misc/talk.wav")
		Self.Noise = "misc/talk.wav"
	}

	trigger_multiple()
}

func counter_use() {
	Self.Count = Self.Count - 1

	if Self.Count < 0 {
		return
	}

	if Self.Count != 0 {
		if Activator.ClassName == "player" && (int(Self.SpawnFlags)&int(SPAWNFLAG_NOMESSAGE)) == 0 {
			if Self.Count >= 4 {
				engine.Centerprint("$qc_more_go")
			} else if Self.Count == 3 {
				engine.Centerprint("$qc_three_more")
			} else if Self.Count == 2 {
				engine.Centerprint("$qc_two_more")
			} else {
				engine.Centerprint("$qc_one_more")
			}
		}
		return
	}

	if Activator.ClassName == "player" && (int(Self.SpawnFlags)&int(SPAWNFLAG_NOMESSAGE)) == 0 {
		engine.Centerprint("$qc_sequence_completed")
	}

	Other = Activator
	multi_trigger()
}

func trigger_counter() {
	Self.Wait = -1
	if Self.Count == 0 {
		Self.Count = 2
	}
	Self.Use = counter_use
}

var (
	PLAYER_ONLY float32 = 1
	SILENT      float32 = 2
)

func play_teleport() {
	var v float32
	var tmpstr string

	v = engine.Random() * 5

	if v < 1 {
		tmpstr = "misc/r_tele1.wav"
	} else if v < 2 {
		tmpstr = "misc/r_tele2.wav"
	} else if v < 3 {
		tmpstr = "misc/r_tele3.wav"
	} else if v < 4 {
		tmpstr = "misc/r_tele4.wav"
	} else {
		tmpstr = "misc/r_tele5.wav"
	}

	engine.Sound(Self, int(CHAN_VOICE), tmpstr, 1, ATTN_NORM)
	engine.Remove(Self)
}

func spawn_tfog(org quake.Vec3) {
	s := engine.Spawn()
	s.Origin = org
	s.NextThink = Time + 0.2
	s.Think = play_teleport

	engine.WriteByte(MSG_BROADCAST, float32(SVC_TEMPENTITY))
	engine.WriteByte(MSG_BROADCAST, float32(TE_TELEPORT))
	engine.WriteCoord(MSG_BROADCAST, org[0])
	engine.WriteCoord(MSG_BROADCAST, org[1])
	engine.WriteCoord(MSG_BROADCAST, org[2])
}

func tdeath_touch() {
	if Other == Self.Owner {
		return
	}

	if Other.ClassName == "player" {
		if Other.InvincibleFinished > Time {
			Self.ClassName = "teledeath2"
		}

		if Self.Owner.ClassName != "player" {
			T_Damage(Self.Owner, Self, Self, 50000)
			return
		}
	}

	if Other.Health != 0 {
		T_Damage(Other, Self, Self, 50000)
	}
}

func spawn_tdeath(org quake.Vec3, death_owner *quake.Entity) {
	death := engine.Spawn()
	death.ClassName = "teledeath"
	death.MoveType = MOVETYPE_NONE
	death.Solid = SOLID_TRIGGER
	death.Angles = quake.MakeVec3(0, 0, 0)
	engine.SetSize(death, death_owner.Mins.Sub(quake.MakeVec3(1, 1, 1)), death_owner.Maxs.Add(quake.MakeVec3(1, 1, 1)))
	engine.SetOrigin(death, org)
	death.Touch = tdeath_touch
	death.NextThink = Time + 0.2
	death.Think = SUB_Remove
	death.Owner = death_owner

	ForceRetouch = 2
}

func teleport_touch() {
	var t *quake.Entity
	var org quake.Vec3

	if Self.TargetName != StringNull {
		if Self.NextThink < Time {
			return
		}
	}

	if (int(Self.SpawnFlags) & int(PLAYER_ONLY)) != 0 {
		if Other.ClassName != "player" {
			return
		}
	}

	if Other.Health <= 0 || Other.Solid != float32(SOLID_SLIDEBOX) {
		return
	}

	SUB_UseTargets()

	spawn_tfog(Other.Origin)

	t = engine.Find(World, "targetname", Self.Target)

	if t == nil {
		engine.ObjError("couldn't find target")
	}

	engine.MakeVectors(t.Mangle)
	org = t.Origin.Add(VForward.Mul(32))

	spawn_tfog(org)
	spawn_tdeath(t.Origin, Other)

	if Other.Health == 0 {
		Other.Origin = t.Origin
		Other.Velocity = VForward.Mul(Other.Velocity[0]).Add(VForward.Mul(Other.Velocity[1]))
		return
	}

	engine.SetOrigin(Other, t.Origin)
	Other.Angles = t.Mangle

	if Other.ClassName == "player" {
		Other.FixAngle = 1
		Other.TeleportTime = Time + 0.7

		if (int(Other.Flags) & FL_ONGROUND) != 0 {
			Other.Flags = Other.Flags - float32(FL_ONGROUND)
		}

		Other.Velocity = VForward.Mul(300)
	}

	Other.Flags = Other.Flags - float32(int(Other.Flags)&FL_ONGROUND)
}

func info_teleport_destination() {
	Self.Mangle = Self.Angles
	Self.Angles = quake.MakeVec3(0, 0, 0)
	Self.Model = StringNull
	Self.Origin[2] = Self.Origin[2] + 27

	if Self.TargetName == StringNull {
		engine.ObjError("no targetname")
	}
}

func teleport_use() {
	Self.NextThink = Time + 0.2
	ForceRetouch = 2
	Self.Think = SUB_Null
}

func trigger_teleport() {
	var o quake.Vec3

	InitTrigger()
	Self.Touch = teleport_touch

	if Self.Target == StringNull {
		engine.ObjError("no target")
	}
	Self.Use = teleport_use

	Self.NetName = "trigger_teleport"

	if (int(Self.SpawnFlags) & int(SILENT)) == 0 {
		engine.PrecacheSound("ambience/hum1.wav")
		o = Self.Mins.Add(Self.Maxs).Mul(0.5)
		engine.Ambientsound(o, "ambience/hum1.wav", 0.5, ATTN_STATIC)
	}
}

func trigger_skill_touch() {
	if Other.ClassName != "player" {
		return
	}

	engine.CvarSet("skill", Self.Message)
}

func trigger_setskill() {
	InitTrigger()
	Self.Touch = trigger_skill_touch
}

func trigger_onlyregistered_touch() {
	if Other.ClassName != "player" {
		return
	}

	if Self.AttackFinished > Time {
		return
	}

	Self.AttackFinished = Time + 2

	if engine.Cvar("registered") != 0 {
		Self.Message = StringNull
		SUB_UseTargets()
		engine.Remove(Self)
	} else {
		if Self.Message != StringNull {
			engine.Centerprint(Self.Message)
			engine.Sound(Other, int(CHAN_BODY), "misc/talk.wav", 1, ATTN_NORM)
		}
	}
}

func trigger_onlyregistered() {
	engine.PrecacheSound("misc/talk.wav")
	InitTrigger()
	Self.Touch = trigger_onlyregistered_touch
}

func hurt_on() {
	Self.Solid = SOLID_TRIGGER
	Self.NextThink = -1
}

func hurt_touch() {
	if Other.TakeDamage != 0 {
		Self.Solid = SOLID_NOT
		T_Damage(Other, Self, Self, Self.Dmg)
		Self.Think = hurt_on
		Self.NextThink = Time + 1
	}
}

func trigger_hurt() {
	InitTrigger()
	Self.Touch = hurt_touch

	if Self.Dmg == 0 {
		Self.Dmg = 5
	}
}

var PUSH_ONCE float32 = 1

func trigger_push_touch() {
	if Other.ClassName == "grenade" {
		Other.Velocity = Self.MoveDir.Mul(Self.Speed * 10)
	} else if Other.Health > 0 {
		Other.Velocity = Self.MoveDir.Mul(Self.Speed * 10)

		if Other.ClassName == "player" {
			if Other.FlySound < Time {
				Other.FlySound = Time + 1.5
				engine.Sound(Other, int(CHAN_AUTO), "ambience/windfly.wav", 1, ATTN_NORM)
			}
		}
	}

	if (int(Self.SpawnFlags) & int(PUSH_ONCE)) != 0 {
		engine.Remove(Self)
	}
}

func trigger_push() {
	InitTrigger()
	engine.PrecacheSound("ambience/windfly.wav")
	Self.Touch = trigger_push_touch

	Self.NetName = "trigger_push"

	if Self.Speed == 0 {
		Self.Speed = 1000
	}
}

func trigger_monsterjump_touch() {
	if (int(Other.Flags) & (FL_MONSTER | FL_FLY | FL_SWIM)) != FL_MONSTER {
		return
	}

	Other.Velocity[0] = Self.MoveDir[0] * Self.Speed
	Other.Velocity[1] = Self.MoveDir[1] * Self.Speed

	if (int(Other.Flags) & FL_ONGROUND) == 0 {
		return
	}

	Other.Flags = Other.Flags - float32(FL_ONGROUND)
	Other.Velocity[2] = Self.Height
}

func trigger_monsterjump() {
	if Self.Speed == 0 {
		Self.Speed = 200
	}

	if Self.Height == 0 {
		Self.Height = 200
	}

	if Self.Angles == quake.MakeVec3(0, 0, 0) {
		Self.Angles = quake.MakeVec3(0, 360, 0)
	}

	InitTrigger()
	Self.Touch = trigger_monsterjump_touch
}
