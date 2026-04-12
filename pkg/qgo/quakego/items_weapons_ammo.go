package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

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
