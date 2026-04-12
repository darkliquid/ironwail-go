package quakego

import (
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake"
	"github.com/darkliquid/ironwail-go/pkg/qgo/quake/engine"
)

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
