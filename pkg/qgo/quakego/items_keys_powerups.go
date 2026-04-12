package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

func key_touch() {
	if Other.ClassName != "player" {
		return
	}
	if Other.Health <= 0 {
		return
	}
	if (int(Other.Items) & int(Self.Items)) != 0 {
		return
	}
	engine.SPrint(Other, quake.Sprintf("$qc_got_item %s", Self.NetName))
	engine.Sound(Other, int(CHAN_ITEM), Self.Noise, 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")
	Other.Items = float32(int(Other.Items) | int(Self.Items))
	if Coop == 0 {
		Self.Solid = SOLID_NOT
		Self.Model = StringNull
	}
	Activator = Other
	SUB_UseTargets()
}

func key_setsounds() {
	if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
		engine.PrecacheSound("misc/medkey.wav")
		Self.Noise = "misc/medkey.wav"
	}
	if int(World.WorldType) == WORLDTYPE_METAL {
		engine.PrecacheSound("misc/runekey.wav")
		Self.Noise = "misc/runekey.wav"
	}
	if int(World.WorldType) == WORLDTYPE_BASE {
		engine.PrecacheSound("misc/basekey.wav")
		Self.Noise = "misc/basekey.wav"
	}
}

func item_key1() {
	if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
		engine.PrecacheModel("progs/w_s_key.mdl")
		engine.SetModel(Self, "progs/w_s_key.mdl")
		Self.NetName = "$qc_silver_key"
	} else if int(World.WorldType) == WORLDTYPE_METAL {
		engine.PrecacheModel("progs/m_s_key.mdl")
		engine.SetModel(Self, "progs/m_s_key.mdl")
		Self.NetName = "$qc_silver_runekey"
	} else if int(World.WorldType) == WORLDTYPE_BASE {
		engine.PrecacheModel("progs/b_s_key.mdl")
		engine.SetModel(Self, "progs/b_s_key.mdl")
		Self.NetName = "$qc_silver_keycard"
	}
	key_setsounds()
	Self.Touch = key_touch
	Self.Items = float32(IT_KEY1)
	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 32))
	StartItem()
}

func item_key2() {
	if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
		engine.PrecacheModel("progs/w_g_key.mdl")
		engine.SetModel(Self, "progs/w_g_key.mdl")
		Self.NetName = "$qc_gold_key"
	}
	if int(World.WorldType) == WORLDTYPE_METAL {
		engine.PrecacheModel("progs/m_g_key.mdl")
		engine.SetModel(Self, "progs/m_g_key.mdl")
		Self.NetName = "$qc_gold_runekey"
	}
	if int(World.WorldType) == WORLDTYPE_BASE {
		engine.PrecacheModel("progs/b_g_key.mdl")
		engine.SetModel(Self, "progs/b_g_key.mdl")
		Self.NetName = "$qc_gold_keycard"
	}
	key_setsounds()
	Self.Touch = key_touch
	Self.Items = float32(IT_KEY2)
	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 32))
	StartItem()
}

func sigil_touch() {
	if Other.ClassName != "player" {
		return
	}
	if Other.Health <= 0 {
		return
	}
	engine.Centerprint("$qc_got_rune")
	engine.Sound(Other, int(CHAN_ITEM), Self.Noise, 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")
	Self.Solid = SOLID_NOT
	Self.Model = StringNull
	ServerFlags = float32(int(ServerFlags) | (int(Self.SpawnFlags) & 15))
	Self.ClassName = StringNull
	Activator = Other
	SUB_UseTargets()
}

func item_sigil() {
	if Self.SpawnFlags == 0 {
		engine.ObjError("no spawnflags")
	}
	engine.PrecacheSound("misc/runekey.wav")
	Self.Noise = "misc/runekey.wav"
	if (int(Self.SpawnFlags) & 1) != 0 {
		engine.PrecacheModel("progs/end1.mdl")
		engine.SetModel(Self, "progs/end1.mdl")
	}
	if (int(Self.SpawnFlags) & 2) != 0 {
		engine.PrecacheModel("progs/end2.mdl")
		engine.SetModel(Self, "progs/end2.mdl")
	}
	if (int(Self.SpawnFlags) & 4) != 0 {
		engine.PrecacheModel("progs/end3.mdl")
		engine.SetModel(Self, "progs/end3.mdl")
	}
	if (int(Self.SpawnFlags) & 8) != 0 {
		engine.PrecacheModel("progs/end4.mdl")
		engine.SetModel(Self, "progs/end4.mdl")
	}
	Self.Touch = sigil_touch
	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 32))
	StartItem()
}

func powerup_touch() {
	if Other.ClassName != "player" {
		return
	}
	if Other.Health <= 0 {
		return
	}
	engine.SPrint(Other, quake.Sprintf("$qc_got_item %s", Self.NetName))
	if Deathmatch != 0 {
		Self.Mdl = Self.Model
		if Self.ClassName == "item_artifact_invulnerability" || Self.ClassName == "item_artifact_invisibility" {
			Self.NextThink = Time + 300
		} else {
			Self.NextThink = Time + 60
		}
		Self.Think = SUB_regen
	}
	engine.Sound(Other, int(CHAN_VOICE), Self.Noise, 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")
	Self.Solid = SOLID_NOT
	Other.Items = float32(int(Other.Items) | int(Self.Items))
	Self.Model = StringNull

	if Self.ClassName == "item_artifact_envirosuit" {
		Other.RadTime = 1
		// Other.RadsuitFinished = Time + 30 // Field was removed from Entity
	}
	if Self.ClassName == "item_artifact_invulnerability" {
		Other.InvincibleTime = 1
		Other.InvincibleFinished = Time + 30
	}
	if Self.ClassName == "item_artifact_invisibility" {
		Other.InvisibleTime = 1
		Other.InvisibleFinished = Time + 30
	}
	if Self.ClassName == "item_artifact_super_damage" {
		Other.SuperTime = 1
		Other.SuperDamageFinished = Time + 30
	}
	Activator = Other
	SUB_UseTargets()
}

func item_artifact_invulnerability() {
	Self.Touch = powerup_touch
	engine.PrecacheModel("progs/invulner.mdl")
	engine.PrecacheSound("items/protect.wav")
	engine.PrecacheSound("items/protect2.wav")
	engine.PrecacheSound("items/protect3.wav")
	Self.Noise = "items/protect.wav"
	engine.SetModel(Self, "progs/invulner.mdl")
	Self.NetName = "$qc_pentagram_of_protection"
	Self.Items = float32(IT_INVULNERABILITY)
	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 32))
	StartItem()
}

func item_artifact_envirosuit() {
	Self.Touch = powerup_touch
	engine.PrecacheModel("progs/suit.mdl")
	engine.PrecacheSound("items/suit.wav")
	engine.PrecacheSound("items/suit2.wav")
	Self.Noise = "items/suit.wav"
	engine.SetModel(Self, "progs/suit.mdl")
	Self.NetName = "$qc_biosuit"
	Self.Items = float32(IT_SUIT)
	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 32))
	StartItem()
}

func item_artifact_invisibility() {
	Self.Touch = powerup_touch
	engine.PrecacheModel("progs/invisibl.mdl")
	engine.PrecacheSound("items/inv1.wav")
	engine.PrecacheSound("items/inv2.wav")
	engine.PrecacheSound("items/inv3.wav")
	Self.Noise = "items/inv1.wav"
	engine.SetModel(Self, "progs/invisibl.mdl")
	Self.NetName = "$qc_ring_of_shadows"
	Self.Items = float32(IT_INVISIBILITY)
	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 32))
	StartItem()
}

func item_artifact_super_damage() {
	Self.Touch = powerup_touch
	engine.PrecacheModel("progs/quaddama.mdl")
	engine.PrecacheSound("items/damage.wav")
	engine.PrecacheSound("items/damage2.wav")
	engine.PrecacheSound("items/damage3.wav")
	Self.Noise = "items/damage.wav"
	engine.SetModel(Self, "progs/quaddama.mdl")
	Self.NetName = "$qc_quad_damage"
	Self.Items = float32(IT_QUAD)
	engine.SetSize(Self, quake.MakeVec3(-16, -16, -24), quake.MakeVec3(16, 16, 32))
	StartItem()
}
