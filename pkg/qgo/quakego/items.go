package quakego

import (
	"github.com/ironwail/ironwail-go/pkg/qgo/quake"
	"github.com/ironwail/ironwail-go/pkg/qgo/quake/engine"
)

// Prototyped elsewhere
var (
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
	if e.Health <= 0 {
		return 0
	}

	if (ignore == 0) && (e.Health >= e.MaxHealth) {
		return 0
	}

	healamount = engine.Ceil(healamount)
	e.Health = e.Health + healamount

	if (ignore == 0) && (e.Health >= e.MaxHealth) {
		e.Health = e.MaxHealth
	}

	if e.Health > 250 {
		e.Health = 250
	}

	return 1
}

const (
	H_ROTTEN = 1
	H_MEGA   = 2
)

func item_health() {
	Self.Touch = health_touch

	if (int(Self.SpawnFlags) & H_ROTTEN) != 0 {
		engine.PrecacheModel("maps/b_bh10.bsp")
		engine.PrecacheSound("items/r_item1.wav")
		engine.SetModel(Self, "maps/b_bh10.bsp")
		Self.Noise = "items/r_item1.wav"
		Self.HealAmount = 15
		Self.HealType = 0
	} else if (int(Self.SpawnFlags) & H_MEGA) != 0 {
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

func bound_other_ammo() {
	if Other.AmmoShells > 100 {
		Other.AmmoShells = 100
	}
	if Other.AmmoNails > 200 {
		Other.AmmoNails = 200
	}
	if Other.AmmoRockets > 100 {
		Other.AmmoRockets = 100
	}
	if Other.AmmoCells > 100 {
		Other.AmmoCells = 100
	}
}

func RankForWeapon(w float32) float32 {
	switch int(w) {
	case IT_LIGHTNING:
		return 1
	case IT_ROCKET_LAUNCHER:
		return 2
	case IT_SUPER_NAILGUN:
		return 3
	case IT_GRENADE_LAUNCHER:
		return 4
	case IT_SUPER_SHOTGUN:
		return 5
	case IT_NAILGUN:
		return 6
	}
	return 7
}

func WeaponCode(w float32) float32 {
	switch int(w) {
	case IT_SUPER_SHOTGUN:
		return 3
	case IT_NAILGUN:
		return 4
	case IT_SUPER_NAILGUN:
		return 5
	case IT_GRENADE_LAUNCHER:
		return 6
	case IT_ROCKET_LAUNCHER:
		return 7
	case IT_LIGHTNING:
		return 8
	}
	return 1
}

func Deathmatch_Weapon(old, new float32) {
	if (int(Self.Flags) & FL_ISBOT) != 0 {
		return
	}

	or := RankForWeapon(Self.Weapon)
	nr := RankForWeapon(new)

	if nr < or {
		Self.Weapon = new
	}
}

func weapon_touch() {
	var new_it, old_it, leave float32
	var stemp *quake.Entity

	new_it = Other.Items

	if (int(Other.Flags) & FL_CLIENT) == 0 {
		return
	}

	stemp = Self
	Self = Other
	Self = stemp

	if Coop != 0 || Deathmatch == 2 || Deathmatch == 3 || Deathmatch == 5 {
		leave = 1
	} else {
		leave = 0
	}

	if Self.ClassName == "weapon_nailgun" {
		if leave != 0 && (int(Other.Items)&IT_NAILGUN) != 0 {
			return
		}
		new_it = float32(IT_NAILGUN)
		Other.AmmoNails = Other.AmmoNails + 30
	} else if Self.ClassName == "weapon_supernailgun" {
		if leave != 0 && (int(Other.Items)&IT_SUPER_NAILGUN) != 0 {
			return
		}
		new_it = float32(IT_SUPER_NAILGUN)
		Other.AmmoNails = Other.AmmoNails + 30
	} else if Self.ClassName == "weapon_supershotgun" {
		if leave != 0 && (int(Other.Items)&IT_SUPER_SHOTGUN) != 0 {
			return
		}
		new_it = float32(IT_SUPER_SHOTGUN)
		Other.AmmoShells = Other.AmmoShells + 5
	} else if Self.ClassName == "weapon_rocketlauncher" {
		if leave != 0 && (int(Other.Items)&IT_ROCKET_LAUNCHER) != 0 {
			return
		}
		new_it = float32(IT_ROCKET_LAUNCHER)
		Other.AmmoRockets = Other.AmmoRockets + 5
	} else if Self.ClassName == "weapon_grenadelauncher" {
		if leave != 0 && (int(Other.Items)&IT_GRENADE_LAUNCHER) != 0 {
			return
		}
		new_it = float32(IT_GRENADE_LAUNCHER)
		Other.AmmoRockets = Other.AmmoRockets + 5
	} else if Self.ClassName == "weapon_lightning" {
		if leave != 0 && (int(Other.Items)&IT_LIGHTNING) != 0 {
			return
		}
		new_it = float32(IT_LIGHTNING)
		Other.AmmoCells = Other.AmmoCells + 15
	} else {
		engine.ObjError("weapon_touch: unknown classname")
	}

	engine.SPrint(Other, quake.Sprintf("$qc_got_item %s", Self.NetName))
	engine.Sound(Other, int(CHAN_ITEM), "weapons/pkup.wav", 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")

	bound_other_ammo()

	old_it = Other.Items
	Other.Items = float32(int(Other.Items) | int(new_it))

	stemp = Self
	Self = Other

	if W_WantsToChangeWeapon(Other, old_it, Other.Items) == 1 {
		if Deathmatch == 0 {
			Self.Weapon = new_it
		} else {
			Deathmatch_Weapon(old_it, new_it)
		}
	}

	W_SetCurrentAmmo()
	Self = stemp

	if leave != 0 {
		return
	}

	Self.Model = StringNull
	Self.Solid = SOLID_NOT

	if Deathmatch != 0 && Deathmatch != 2 {
		Self.NextThink = Time + 30
	}

	Self.Think = SUB_regen
	Activator = Other
	SUB_UseTargets()
}

func weapon_supershotgun() {
	engine.PrecacheModel("progs/g_shot.mdl")
	engine.SetModel(Self, "progs/g_shot.mdl")
	Self.Weapon = float32(IT_SUPER_SHOTGUN)
	Self.NetName = "$qc_double_shotgun"
	Self.Touch = weapon_touch
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func weapon_nailgun() {
	engine.PrecacheModel("progs/g_nail.mdl")
	engine.SetModel(Self, "progs/g_nail.mdl")
	Self.Weapon = float32(IT_NAILGUN)
	Self.NetName = "$qc_nailgun"
	Self.Touch = weapon_touch
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func weapon_supernailgun() {
	engine.PrecacheModel("progs/g_nail2.mdl")
	engine.SetModel(Self, "progs/g_nail2.mdl")
	Self.Weapon = float32(IT_SUPER_NAILGUN)
	Self.NetName = "$qc_super_nailgun"
	Self.Touch = weapon_touch
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func weapon_grenadelauncher() {
	engine.PrecacheModel("progs/g_rock.mdl")
	engine.SetModel(Self, "progs/g_rock.mdl")
	Self.Weapon = float32(IT_GRENADE_LAUNCHER)
	Self.NetName = "$qc_grenade_launcher"
	Self.Touch = weapon_touch
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func weapon_rocketlauncher() {
	engine.PrecacheModel("progs/g_rock2.mdl")
	engine.SetModel(Self, "progs/g_rock2.mdl")
	Self.Weapon = float32(IT_ROCKET_LAUNCHER)
	Self.NetName = "$qc_rocket_launcher"
	Self.Touch = weapon_touch
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func weapon_lightning() {
	engine.PrecacheModel("progs/g_light.mdl")
	engine.SetModel(Self, "progs/g_light.mdl")
	Self.Weapon = float32(IT_LIGHTNING)
	Self.NetName = "$qc_thunderbolt"
	Self.Touch = weapon_touch
	engine.SetSize(Self, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	StartItem()
}

func ammo_touch() {
	var stemp *quake.Entity
	var best float32

	if Other.ClassName != "player" {
		return
	}

	if Other.Health <= 0 {
		return
	}

	stemp = Self
	Self = Other
	Self = stemp

	if Self.Weapon == 1 {
		if Other.AmmoShells >= 100 {
			return
		}
		Other.AmmoShells = Other.AmmoShells + Self.AFlag
	}

	if Self.Weapon == 2 {
		if Other.AmmoNails >= 200 {
			return
		}
		Other.AmmoNails = Other.AmmoNails + Self.AFlag
	}

	if Self.Weapon == 3 {
		if Other.AmmoRockets >= 100 {
			return
		}
		Other.AmmoRockets = Other.AmmoRockets + Self.AFlag
	}

	if Self.Weapon == 4 {
		if Other.AmmoCells >= 100 {
			return
		}
		Other.AmmoCells = Other.AmmoCells + Self.AFlag
	}

	bound_other_ammo()
	engine.SPrint(Other, quake.Sprintf("$qc_got_item %s", Self.NetName))
	engine.Sound(Other, int(CHAN_ITEM), "weapons/lock4.wav", 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")

	if Other.Weapon == best && W_WantsToChangeWeapon(Other, 0, 1) == 1 {
		stemp = Self
		Self = Other
		Self.Weapon = W_BestWeapon()
		W_SetCurrentAmmo()
		Self = stemp
	}

	stemp = Self
	Self = Other
	W_SetCurrentAmmo()
	Self = stemp

	Self.Model = StringNull
	Self.Solid = SOLID_NOT

	if Deathmatch != 0 {
		if Deathmatch == 3 || Deathmatch == 5 {
			Self.NextThink = Time + 15
		} else if Deathmatch != 2 {
			Self.NextThink = Time + 30
		}
	}

	Self.Think = SUB_regen
	Activator = Other
	SUB_UseTargets()
}

var WEAPON_BIG2 float32 = 1

func item_shells() {
	Self.Touch = ammo_touch
	if (int(Self.SpawnFlags) & int(WEAPON_BIG2)) != 0 {
		engine.PrecacheModel("maps/b_shell1.bsp")
		engine.SetModel(Self, "maps/b_shell1.bsp")
		Self.AFlag = 40
	} else {
		engine.PrecacheModel("maps/b_shell0.bsp")
		engine.SetModel(Self, "maps/b_shell0.bsp")
		Self.AFlag = 20
	}
	Self.Weapon = 1
	Self.NetName = "$qc_shells"
	engine.SetSize(Self, quake.MakeVec3(0, 0, 0), quake.MakeVec3(32, 32, 56))
	StartItem()
}

func item_spikes() {
	Self.Touch = ammo_touch
	if (int(Self.SpawnFlags) & int(WEAPON_BIG2)) != 0 {
		engine.PrecacheModel("maps/b_nail1.bsp")
		engine.SetModel(Self, "maps/b_nail1.bsp")
		Self.AFlag = 50
	} else {
		engine.PrecacheModel("maps/b_nail0.bsp")
		engine.SetModel(Self, "maps/b_nail0.bsp")
		Self.AFlag = 25
	}
	Self.Weapon = 2
	Self.NetName = "$qc_nails"
	engine.SetSize(Self, quake.MakeVec3(0, 0, 0), quake.MakeVec3(32, 32, 56))
	StartItem()
}

func item_rockets() {
	Self.Touch = ammo_touch
	if (int(Self.SpawnFlags) & int(WEAPON_BIG2)) != 0 {
		engine.PrecacheModel("maps/b_rock1.bsp")
		engine.SetModel(Self, "maps/b_rock1.bsp")
		Self.AFlag = 10
	} else {
		engine.PrecacheModel("maps/b_rock0.bsp")
		engine.SetModel(Self, "maps/b_rock0.bsp")
		Self.AFlag = 5
	}
	Self.Weapon = 3
	Self.NetName = "$qc_rockets"
	engine.SetSize(Self, quake.MakeVec3(0, 0, 0), quake.MakeVec3(32, 32, 56))
	StartItem()
}

func item_cells() {
	Self.Touch = ammo_touch
	if (int(Self.SpawnFlags) & int(WEAPON_BIG2)) != 0 {
		engine.PrecacheModel("maps/b_batt1.bsp")
		engine.SetModel(Self, "maps/b_batt1.bsp")
		Self.AFlag = 12
	} else {
		engine.PrecacheModel("maps/b_batt0.bsp")
		engine.SetModel(Self, "maps/b_batt0.bsp")
		Self.AFlag = 6
	}
	Self.Weapon = 4
	Self.NetName = "$qc_cells"
	engine.SetSize(Self, quake.MakeVec3(0, 0, 0), quake.MakeVec3(32, 32, 56))
	StartItem()
}

var (
	WEAPON_SHOTGUN float32 = 1
	WEAPON_ROCKET  float32 = 2
	WEAPON_SPIKES  float32 = 4
	WEAPON_BIG     float32 = 8
)

func item_weapon() {
	Self.Touch = ammo_touch
	if (int(Self.SpawnFlags) & int(WEAPON_SHOTGUN)) != 0 {
		if (int(Self.SpawnFlags) & int(WEAPON_BIG)) != 0 {
			engine.PrecacheModel("maps/b_shell1.bsp")
			engine.SetModel(Self, "maps/b_shell1.bsp")
			Self.AFlag = 40
		} else {
			engine.PrecacheModel("maps/b_shell0.bsp")
			engine.SetModel(Self, "maps/b_shell0.bsp")
			Self.AFlag = 20
		}
		Self.Weapon = 1
		Self.NetName = "$qc_shells"
	}
	if (int(Self.SpawnFlags) & int(WEAPON_SPIKES)) != 0 {
		if (int(Self.SpawnFlags) & int(WEAPON_BIG)) != 0 {
			engine.PrecacheModel("maps/b_nail1.bsp")
			engine.SetModel(Self, "maps/b_nail1.bsp")
			Self.AFlag = 40
		} else {
			engine.PrecacheModel("maps/b_nail0.bsp")
			engine.SetModel(Self, "maps/b_nail0.bsp")
			Self.AFlag = 20
		}
		Self.Weapon = 2
		Self.NetName = "$qc_spikes"
	}
	if (int(Self.SpawnFlags) & int(WEAPON_ROCKET)) != 0 {
		if (int(Self.SpawnFlags) & int(WEAPON_BIG)) != 0 {
			engine.PrecacheModel("maps/b_rock1.bsp")
			engine.SetModel(Self, "maps/b_rock1.bsp")
			Self.AFlag = 10
		} else {
			engine.PrecacheModel("maps/b_rock0.bsp")
			engine.SetModel(Self, "maps/b_rock0.bsp")
			Self.AFlag = 5
		}
		Self.Weapon = 3
		Self.NetName = "$qc_rockets"
	}
	engine.SetSize(Self, quake.MakeVec3(0, 0, 0), quake.MakeVec3(32, 32, 56))
	StartItem()
}

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

func BackpackTouch() {
	var old_it, new_it, acount float32
	var stemp *quake.Entity

	if Other.ClassName != "player" {
		return
	}
	if Other.Health <= 0 {
		return
	}
	acount = 0
	engine.SPrint(Other, "$qc_backpack_got")
	if Self.Items != 0 {
		if (int(Other.Items) & int(Self.Items)) == 0 {
			acount = 1
			engine.SPrint(Other, Self.NetName)
		}
	}
	stemp = Self
	Self = Other
	Self = stemp

	Other.AmmoShells = Other.AmmoShells + Self.AmmoShells
	Other.AmmoNails = Other.AmmoNails + Self.AmmoNails
	Other.AmmoRockets = Other.AmmoRockets + Self.AmmoRockets
	Other.AmmoCells = Other.AmmoCells + Self.AmmoCells

	new_it = Self.Items
	if new_it == 0 {
		new_it = Other.Weapon
	}
	old_it = Other.Items
	Other.Items = float32(int(Other.Items) | int(Self.Items))
	bound_other_ammo()

	if Self.AmmoShells != 0 {
		if acount != 0 {
			engine.SPrint(Other, ", ")
		}
		acount = 1
		engine.SPrint(Other, quake.Sprintf("$qc_backpack_shells %s", engine.Ftos(Self.AmmoShells)))
	}
	if Self.AmmoNails != 0 {
		if acount != 0 {
			engine.SPrint(Other, ", ")
		}
		acount = 1
		engine.SPrint(Other, quake.Sprintf("$qc_backpack_nails %s", engine.Ftos(Self.AmmoNails)))
	}
	if Self.AmmoRockets != 0 {
		if acount != 0 {
			engine.SPrint(Other, ", ")
		}
		acount = 1
		engine.SPrint(Other, quake.Sprintf("$qc_backpack_rockets %s", engine.Ftos(Self.AmmoRockets)))
	}
	if Self.AmmoCells != 0 {
		if acount != 0 {
			engine.SPrint(Other, ", ")
		}
		acount = 1
		engine.SPrint(Other, quake.Sprintf("$qc_backpack_cells %s", engine.Ftos(Self.AmmoCells)))
	}
	engine.SPrint(Other, "\n")
	engine.Sound(Other, int(CHAN_ITEM), "weapons/lock4.wav", 1, ATTN_NORM)
	engine.StuffCmd(Other, "bf\n")
	engine.Remove(Self)
	Self = Other

	if W_WantsToChangeWeapon(Other, old_it, Other.Items) == 1 {
		if (int(Self.Flags) & FL_INWATER) != 0 {
			if int(new_it) != IT_LIGHTNING {
				Deathmatch_Weapon(old_it, new_it)
			}
		} else {
			Deathmatch_Weapon(old_it, new_it)
		}
	}
	W_SetCurrentAmmo()
}

func DropBackpack() {
	if (Self.AmmoShells + Self.AmmoNails + Self.AmmoRockets + Self.AmmoCells) == 0 {
		return
	}
	item := engine.Spawn()
	item.Origin = Self.Origin.Sub(quake.MakeVec3(0, 0, 24))
	item.Items = Self.Weapon
	item.ClassName = "item_backpack"

	switch int(item.Items) {
	case IT_AXE:
		item.NetName = "$qc_axe"
	case IT_SHOTGUN:
		item.NetName = "$qc_shotgun"
	case IT_SUPER_SHOTGUN:
		item.NetName = "$qc_double_shotgun"
	case IT_NAILGUN:
		item.NetName = "$qc_nailgun"
	case IT_SUPER_NAILGUN:
		item.NetName = "$qc_super_nailgun"
	case IT_GRENADE_LAUNCHER:
		item.NetName = "$qc_grenade_launcher"
	case IT_ROCKET_LAUNCHER:
		item.NetName = "$qc_rocket_launcher"
	case IT_LIGHTNING:
		item.NetName = "$qc_thunderbolt"
	}

	item.AmmoShells = Self.AmmoShells
	item.AmmoNails = Self.AmmoNails
	item.AmmoRockets = Self.AmmoRockets
	item.AmmoCells = Self.AmmoCells

	if item.AmmoShells < 5 && (int(item.Items) == IT_SHOTGUN || int(item.Items) == IT_SUPER_SHOTGUN) {
		item.AmmoShells = 5
	}
	if item.AmmoNails < 20 && (int(item.Items) == IT_NAILGUN || int(item.Items) == IT_SUPER_NAILGUN) {
		item.AmmoNails = 20
	}
	if item.AmmoRockets < 5 && (int(item.Items) == IT_GRENADE_LAUNCHER || int(item.Items) == IT_ROCKET_LAUNCHER) {
		item.AmmoRockets = 5
	}
	if item.AmmoCells < 15 && int(item.Items) == IT_LIGHTNING {
		item.AmmoCells = 15
	}

	item.Velocity[2] = 300
	item.Velocity[0] = -100 + (engine.Random() * 200)
	item.Velocity[1] = -100 + (engine.Random() * 200)

	item.Flags = float32(FL_ITEM)
	item.Solid = SOLID_TRIGGER
	item.MoveType = MOVETYPE_TOSS
	engine.SetModel(item, "progs/backpack.mdl")
	engine.SetSize(item, quake.MakeVec3(-16, -16, 0), quake.MakeVec3(16, 16, 56))
	item.Touch = BackpackTouch
	item.NextThink = Time + 120
	item.Think = SUB_Remove
}
