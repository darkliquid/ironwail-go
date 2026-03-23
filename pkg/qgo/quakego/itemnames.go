package quakego

import (
)

func GetNetName(item_number float32) string {
	switch int(item_number) {
	case IT_AXE:
		return "Axe"
	case IT_SHOTGUN:
		return "Shotgun"
	case IT_SUPER_SHOTGUN:
		return "Super Shotgun"
	case IT_NAILGUN:
		return "Nailgun"
	case IT_SUPER_NAILGUN:
		return "Perforator"
	case IT_GRENADE_LAUNCHER:
		return "Grenade Launcher"
	case IT_ROCKET_LAUNCHER:
		return "Rocket Launcher"
	case IT_LIGHTNING:
		return "Lightning Gun"
	case IT_EXTRA_WEAPON:
		return "Extra Weapon"
	case IT_SHELLS:
		return "Shells"
	case IT_NAILS:
		return "Nails"
	case IT_ROCKETS:
		return "Rockets"
	case IT_CELLS:
		return "Cells"
	case IT_ARMOR1:
		return "Green Armor"
	case IT_ARMOR2:
		return "Yellow Armor"
	case IT_ARMOR3:
		return "Red Armor"
	case IT_SUPERHEALTH:
		return "Mega Health"
	case IT_KEY1:
		if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
			return "Silver key"
		} else if int(World.WorldType) == WORLDTYPE_METAL {
			return "Silver runkey"
		} else if int(World.WorldType) == WORLDTYPE_BASE {
			return "Silver keycard"
		}
	case IT_KEY2:
		if int(World.WorldType) == WORLDTYPE_MEDIEVAL {
			return "Gold key"
		} else if int(World.WorldType) == WORLDTYPE_METAL {
			return "Gold runkey"
		} else if int(World.WorldType) == WORLDTYPE_BASE {
			return "Gold keycard"
		}
	case IT_INVISIBILITY:
		return "Ring of Shadows"
	case IT_INVULNERABILITY:
		return "Pentagram of Protection"
	case IT_SUIT:
		return "Biohazard Suit"
	case IT_QUAD:
		return "Quad Damage"
	}
	return StringNull
}
