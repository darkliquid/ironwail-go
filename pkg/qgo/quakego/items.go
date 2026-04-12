package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

func SUB_regen() {
	Self.Model = Self.Mdl
	Self.Solid = SOLID_TRIGGER
	engine.Sound(Self, int(CHAN_VOICE), "items/itembk2.wav", 1, ATTN_NORM)
	engine.SetOrigin(Self, Self.Origin)
}

func noclass() {
	engine.Dprint("noclass spawned at")
	engine.Dprint(engine.Vtos(Self.Origin))
	engine.Dprint("\n")
	engine.Remove(Self)
}

func PlaceItem() {
	if Self.NetName == StringNull {
		if Self.Items != 0 {
			Self.NetName = GetNetName(Self.Items)
		} else if Self.Weapon != 0 {
			Self.NetName = GetNetName(Self.Weapon)
		}
	}

	Self.Mdl = Self.Model
	Self.Flags = float32(int(Self.Flags) | FL_ITEM)
	Self.Solid = SOLID_TRIGGER
	Self.MoveType = MOVETYPE_TOSS
	Self.Velocity = quake.MakeVec3(0, 0, 0)
	Self.Origin[2] = Self.Origin[2] + 6

	if engine.DropToFloor() == 0 {
		engine.Dprint("Bonus item fell out of level at ")
		engine.Dprint(engine.Vtos(Self.Origin))
		engine.Dprint("\n")
		engine.Remove(Self)
		return
	}
}

func StartItem() {
	Self.NextThink = Time + 0.2
	Self.Think = PlaceItem
}

func T_Heal(e *quake.Entity, healamount, ignore float32) float32 {
	return e.Heal(healamount, ignore)
}

const (
	healthFlagRotten = 1 << iota
	healthFlagMega
)

func item_health() {
	Self.Touch = health_touch

	if (int(Self.SpawnFlags) & healthFlagRotten) != 0 {
		engine.PrecacheModel("maps/b_bh10.bsp")
		engine.PrecacheSound("items/r_item1.wav")
		engine.SetModel(Self, "maps/b_bh10.bsp")
		Self.Noise = "items/r_item1.wav"
		Self.HealAmount = 15
		Self.HealType = 0
	} else if (int(Self.SpawnFlags) & healthFlagMega) != 0 {
		engine.PrecacheModel("maps/b_bh100.bsp")
		engine.PrecacheSound("items/r_item2.wav")
		engine.SetModel(Self, "maps/b_bh100.bsp")
		Self.Noise = "items/r_item2.wav"
		Self.HealAmount = 100
		Self.HealType = 2
	} else {
		engine.PrecacheModel("maps/b_bh25.bsp")
		engine.PrecacheSound("items/health1.wav")
		engine.SetModel(Self, "maps/b_bh25.bsp")
		Self.Noise = "items/health1.wav"
		Self.HealAmount = 25
		Self.HealType = 1
	}

	engine.SetSize(Self, quake.MakeVec3(0, 0, 0), quake.MakeVec3(32, 32, 56))
	StartItem()
}

func health_touch() {
	if Other.ClassName != "player" {
		return
	}

	if Self.HealType == 2 {
		if Other.Health >= 250 {
			return
		}
		if T_Heal(Other, Self.HealAmount, 1) == 0 {
			return
		}
	} else {
		if T_Heal(Other, Self.HealAmount, 0) == 0 {
			return
		}
	}

	engine.SPrint(Other, quake.Sprintf("$qc_item_health %s", engine.Ftos(Self.HealAmount)))

	engine.Sound(Other, int(CHAN_ITEM), Self.Noise, 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")

	Self.Model = StringNull
	Self.Solid = SOLID_NOT

	if Deathmatch != 0 && Deathmatch != 2 {
		if Self.HealType == 2 {
			Self.NextThink = Time + 120
		} else {
			Self.NextThink = Time + 20
		}
	}

	Self.Think = SUB_regen
	Activator = Other
	SUB_UseTargets()
}

func armor_touch() {
	var typ, value, bit float32

	typ = 0.3
	value = 100
	bit = float32(IT_ARMOR1)

	if Other.Health <= 0 {
		return
	}

	if Other.ClassName != "player" {
		return
	}

	if Self.ClassName == "item_armor1" {
		typ = 0.3
		value = 100
		bit = float32(IT_ARMOR1)
	}

	if Self.ClassName == "item_armor2" {
		typ = 0.6
		value = 150
		bit = float32(IT_ARMOR2)
	}

	if Self.ClassName == "item_armorInv" {
		typ = 0.8
		value = 200
		bit = float32(IT_ARMOR3)
	}

	if Other.ArmorType*Other.ArmorValue >= typ*value {
		return
	}

	Other.ArmorType = typ
	Other.ArmorValue = value
	Other.Items = float32(int(Other.Items) - (int(Other.Items) & (IT_ARMOR1 | IT_ARMOR2 | IT_ARMOR3)) + int(bit))

	Self.Solid = SOLID_NOT
	Self.Model = StringNull

	if Deathmatch != 0 && Deathmatch != 2 {
		Self.NextThink = Time + 20
	}

	Self.Think = SUB_regen
	engine.SPrint(Other, "$qc_item_armor")
	engine.Sound(Other, int(CHAN_ITEM), "items/armor1.wav", 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")

	Activator = Other
	SUB_UseTargets()
}

func item_armor1() {
	Self.Touch = armor_touch
	Self.ArmorType = 0.3
	Self.ArmorValue = 100
	engine.PrecacheModel("progs/armor.mdl")
	engine.SetModel(Self, "progs/armor.mdl")
	Self.Skin = 0
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func item_armor2() {
	Self.Touch = armor_touch
	Self.ArmorType = 0.6
	Self.ArmorValue = 150
	engine.PrecacheModel("progs/armor.mdl")
	engine.SetModel(Self, "progs/armor.mdl")
	Self.Skin = 1
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func item_armorInv() {
	Self.Touch = armor_touch
	Self.ArmorType = 0.8
	Self.ArmorValue = 200
	engine.PrecacheModel("progs/armor.mdl")
	engine.SetModel(Self, "progs/armor.mdl")
	Self.Skin = 2
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}
