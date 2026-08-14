// clientdata.go implements the SVCClientData delta encoder: the per-frame
// player-centric payload (damage, view angles, ammo, items, weapon bits).
// Extracted from server_net_send.go behind narrow seams so it can be
// unit-tested without a live Server.
package net

import (
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// PrecacheReader abstracts the model precache lookups the clientdata encoder
// performs. Implemented by *server.Server.
type PrecacheReader interface {
	// FindModel returns the precache slot index for a model name (0 if absent).
	FindModel(name string) int
	// String resolves a QC string table index into its text ("" for 0/nil VM).
	String(idx int32) string
}

// ClientDataLogger abstracts the debug telemetry hooks used to trace
// clientdata serialization. Implemented by *server.DebugTelemetry.
type ClientDataLogger interface {
	ShouldLogEvent(kind srvdebug.DebugEventKind, vm *qc.VM, entNum int, ent *srvtypes.Edict) bool
	LogEventf(kind srvdebug.DebugEventKind, vm *qc.VM, entNum int, ent *srvtypes.Edict, format string, args ...any) bool
}

// ClientDataDeps bundles the server surfaces the clientdata encoder needs
// beyond the message buffer and edict field accessors.
type ClientDataDeps struct {
	// Handle provides the VM-backed edict field accessors (implemented by
	// *server.Server).
	Handle srvtypes.ServerHandle
	// Precacher resolves model precache indices (implemented by *server.Server).
	Precacher PrecacheReader
	// Logger traces serialization; may be nil (implemented by
	// *server.DebugTelemetry).
	Logger ClientDataLogger
	// SetIdealPitch computes and writes the idealpitch entvar from the
	// ground slope ahead of the player. Stays in the server root because it
	// needs the collision Move trace.
	SetIdealPitch func(ent *srvtypes.Edict)
	// EdictNum returns the edict at an index (may be nil).
	EdictNum func(n int) *srvtypes.Edict
	// NumForEdict returns the index of an edict.
	NumForEdict func(e *srvtypes.Edict) int
	// Protocol is the negotiated net protocol (15=NetQuake, 666=Fitz, 999=RMQ).
	Protocol int
	// StandardQuakeWeaponEncoding selects the weapon byte encoding
	// (standard bitmask vs mission-pack bit-number encoding).
	StandardQuakeWeaponEncoding bool
	// Flags are the protocol flags for coord/angle encoding.
	Flags uint32
}

// WriteClientData serializes player-centric data (damage, view, ammo, items)
// for one frame into msg. It mirrors server_net_send.go's
// WriteClientDataToMessage; the root keeps a 1:1 delegator.
func WriteClientData(deps ClientDataDeps, ent *srvtypes.Edict, msg *srvtypes.MessageBuffer) {
	flags := deps.Flags
	fixAngleSent := ent.FixAngle(deps.Handle) != 0
	dmgTake := ent.DmgTake(deps.Handle)
	dmgSave := ent.DmgSave(deps.Handle)
	if dmgTake != 0 || dmgSave != 0 {
		other := deps.EdictNum(int(ent.DmgInflictor(deps.Handle)))
		msg.PutByte(byte(inet.SVCDamage))
		msg.PutByte(byte(dmgSave))
		msg.PutByte(byte(dmgTake))
		if other != nil {
			oOrg := other.Origin(deps.Handle)
			oMins := other.Mins(deps.Handle)
			oMaxs := other.Maxs(deps.Handle)
			for i := 0; i < 3; i++ {
				msg.WriteCoord(oOrg[i]+0.5*(oMins[i]+oMaxs[i]), flags)
			}
		} else {
			for i := 0; i < 3; i++ {
				msg.WriteCoord(0, flags)
			}
		}
		ent.SetDmgTake(deps.Handle, 0)
		ent.SetDmgSave(deps.Handle, 0)
	}

	if deps.SetIdealPitch != nil {
		deps.SetIdealPitch(ent)
	}

	if ent.FixAngle(deps.Handle) != 0 {
		msg.PutByte(byte(inet.SVCSetAngle))
		vAng := ent.VAngle(deps.Handle)
		for i := 0; i < 3; i++ {
			msg.WriteAngle(vAng[i], flags)
		}
		ent.SetFixAngle(deps.Handle, 0)
	}

	bits := uint32(0)

	viewOfs := ent.ViewOfs(deps.Handle)
	idealPitch := ent.IdealPitch(deps.Handle)
	entFlags := ent.Flags(deps.Handle)
	waterLevel := ent.WaterLevel(deps.Handle)
	punchAngle := ent.PunchAngle(deps.Handle)
	velocity := ent.Velocity(deps.Handle)
	weaponFrame := ent.WeaponFrame(deps.Handle)
	armorValue := ent.ArmorValue(deps.Handle)
	weaponModel := int32(ent.WeaponModel(deps.Handle))
	currentAmmo := ent.CurrentAmmo(deps.Handle)
	ammoShells := ent.AmmoShells(deps.Handle)
	ammoNails := ent.AmmoNails(deps.Handle)
	ammoRockets := ent.AmmoRockets(deps.Handle)
	ammoCells := ent.AmmoCells(deps.Handle)
	itemsVal := ent.Items(deps.Handle)
	weaponVal := ent.Weapon(deps.Handle)
	healthVal := ent.Health(deps.Handle)

	if viewOfs[2] != srvtypes.ViewHeight {
		bits |= inet.SU_VIEWHEIGHT
	}
	if idealPitch != 0 {
		bits |= inet.SU_IDEALPITCH
	}
	bits |= inet.SU_ITEMS

	if uint32(entFlags)&srvtypes.FlagOnGround != 0 {
		bits |= inet.SU_ONGROUND
	}
	if waterLevel >= 2 {
		bits |= inet.SU_INWATER
	}
	for i := 0; i < 3; i++ {
		if punchAngle[i] != 0 {
			bits |= inet.SU_PUNCH1 << i
		}
		if velocity[i] != 0 {
			bits |= inet.SU_VELOCITY1 << i
		}
	}
	if weaponFrame != 0 {
		bits |= inet.SU_WEAPONFRAME
	}
	if armorValue != 0 {
		bits |= inet.SU_ARMOR
	}
	bits |= inet.SU_WEAPON

	// FitzQuake/RMQ extension bits — only for non-NetQuake protocols
	weaponModelIdx := deps.Precacher.FindModel(deps.Precacher.String(weaponModel))
	if deps.Protocol != ProtocolNetQuake {
		if bits&inet.SU_WEAPON != 0 && weaponModelIdx&0xFF00 != 0 {
			bits |= inet.SU_WEAPON2
		}
		if int(armorValue)&0xFF00 != 0 {
			bits |= inet.SU_ARMOR2
		}
		if int(currentAmmo)&0xFF00 != 0 {
			bits |= inet.SU_AMMO2
		}
		if int(ammoShells)&0xFF00 != 0 {
			bits |= inet.SU_SHELLS2
		}
		if int(ammoNails)&0xFF00 != 0 {
			bits |= inet.SU_NAILS2
		}
		if int(ammoRockets)&0xFF00 != 0 {
			bits |= inet.SU_ROCKETS2
		}
		if int(ammoCells)&0xFF00 != 0 {
			bits |= inet.SU_CELLS2
		}
		if bits&inet.SU_WEAPONFRAME != 0 && int(weaponFrame)&0xFF00 != 0 {
			bits |= inet.SU_WEAPONFRAME2
		}
		if bits&inet.SU_WEAPON != 0 && ent.Alpha != 0 { // weaponalpha = client entity alpha
			bits |= inet.SU_WEAPONALPHA
		}
		if bits >= 65536 {
			bits |= inet.SU_EXTEND1
		}
		if bits >= 16777216 {
			bits |= inet.SU_EXTEND2
		}
	}

	if entNum := deps.NumForEdict(ent); deps.Logger != nil &&
		deps.Logger.ShouldLogEvent(srvdebug.DebugEventPhysics, deps.Handle.GetVM(), entNum, ent) {
		weaponModelName := deps.Precacher.String(weaponModel)
		deps.Logger.LogEventf(srvdebug.DebugEventPhysics, deps.Handle.GetVM(), entNum, ent,
			"clientdata serialize bits=%#x onground=%t waterlevel=%d viewofs=(%.1f %.1f %.1f) idealpitch=%.1f vel=(%.1f %.1f %.1f) punch=(%.1f %.1f %.1f) fixangle_sent=%t ground=%d teleport=%.3f items=%#x weapon=%#x weaponmodel=%q weaponmodelidx=%d ammo=%d shells=%d",
			bits, uint32(entFlags)&srvtypes.FlagOnGround != 0, int(waterLevel),
			viewOfs[0], viewOfs[1], viewOfs[2],
			idealPitch,
			velocity[0], velocity[1], velocity[2],
			punchAngle[0], punchAngle[1], punchAngle[2],
			fixAngleSent, int(ent.GroundEntity(deps.Handle)), ent.TeleportTime(deps.Handle),
			uint32(itemsVal), uint32(weaponVal), weaponModelName, weaponModelIdx,
			int(currentAmmo), int(ammoShells))
	}

	msg.PutByte(byte(inet.SVCClientData))
	msg.WriteShort(int16(bits))

	if bits&inet.SU_EXTEND1 != 0 {
		msg.PutByte(byte(bits >> 16))
	}
	if bits&inet.SU_EXTEND2 != 0 {
		msg.PutByte(byte(bits >> 24))
	}

	if bits&inet.SU_VIEWHEIGHT != 0 {
		msg.WriteChar(int8(viewOfs[2]))
	}
	if bits&inet.SU_IDEALPITCH != 0 {
		msg.WriteChar(int8(idealPitch))
	}
	for i := 0; i < 3; i++ {
		if bits&(inet.SU_PUNCH1<<i) != 0 {
			msg.WriteChar(int8(punchAngle[i]))
		}
		if bits&(inet.SU_VELOCITY1<<i) != 0 {
			msg.WriteChar(int8(velocity[i] / 16))
		}
	}

	items := uint32(itemsVal)
	msg.WriteLong(int32(items))

	if bits&inet.SU_WEAPONFRAME != 0 {
		msg.PutByte(byte(weaponFrame))
	}
	if bits&inet.SU_ARMOR != 0 {
		msg.PutByte(byte(armorValue))
	}
	if bits&inet.SU_WEAPON != 0 {
		msg.PutByte(byte(weaponModelIdx))
	}

	msg.WriteShort(int16(healthVal))
	msg.PutByte(byte(currentAmmo))
	msg.PutByte(byte(ammoShells))
	msg.PutByte(byte(ammoNails))
	msg.PutByte(byte(ammoRockets))
	msg.PutByte(byte(ammoCells))

	weaponValue := int32(weaponVal)
	if deps.StandardQuakeWeaponEncoding {
		msg.PutByte(byte(weaponValue))
	} else {
		activeWeapon := byte(0)
		for i := 0; i < 32; i++ {
			if weaponValue&(1<<i) != 0 {
				activeWeapon = byte(i)
				break
			}
		}
		msg.PutByte(activeWeapon)
	}

	// FitzQuake extension data
	if bits&inet.SU_WEAPON2 != 0 {
		msg.PutByte(byte(weaponModelIdx >> 8))
	}
	if bits&inet.SU_ARMOR2 != 0 {
		msg.PutByte(byte(int(armorValue) >> 8))
	}
	if bits&inet.SU_AMMO2 != 0 {
		msg.PutByte(byte(int(currentAmmo) >> 8))
	}
	if bits&inet.SU_SHELLS2 != 0 {
		msg.PutByte(byte(int(ammoShells) >> 8))
	}
	if bits&inet.SU_NAILS2 != 0 {
		msg.PutByte(byte(int(ammoNails) >> 8))
	}
	if bits&inet.SU_ROCKETS2 != 0 {
		msg.PutByte(byte(int(ammoRockets) >> 8))
	}
	if bits&inet.SU_CELLS2 != 0 {
		msg.PutByte(byte(int(ammoCells) >> 8))
	}
	if bits&inet.SU_WEAPONFRAME2 != 0 {
		msg.PutByte(byte(int(weaponFrame) >> 8))
	}
	if bits&inet.SU_WEAPONALPHA != 0 {
		msg.PutByte(ent.Alpha) // weaponalpha = client entity alpha
	}

	// Compatibility hack from C Ironwail for Alkaline: the clientdata payload only
	// carries a byte for STAT_ACTIVEWEAPON, so resend the full 32-bit stat when the
	// QuakeC weapon bitmask does not fit in that byte.
	if uint32(byte(weaponValue)) != uint32(weaponValue) && msg.Len()+6 <= msg.Limit() {
		msg.PutByte(byte(inet.SVCUpdateStat))
		msg.PutByte(byte(inet.StatActiveWeapon))
		msg.WriteLong(weaponValue)
	}
}
