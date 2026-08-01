package server

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// Entity field accessors provide typed read/write access to QCVM entity data
// via the byte array, eliminating the need for the EntVars sync layer. These
// methods read/write directly to s.QCVM.Edicts[] — the single source of truth
// — matching C Ironwail's shared-memory architecture where the engine and QC
// bytecode access the same entity field storage.
//
// Standard fields use compile-time EntField* constants for offsets.
// Extension fields (state, speed, wait, etc.) use cached offsets looked up
// from the progs.dat FieldDefs table at load time.

// ============================================================================
// float32 field accessors
// ============================================================================

func (e *Edict) ModelIndex(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldModelIndex)
		return v
	}
	return 0
}
func (e *Edict) SetModelIndex(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldModelIndex, v)
	}
}

func (e *Edict) LTime(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldLTime)
		return v
	}
	return 0
}
func (e *Edict) SetLTime(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldLTime, v)
	}
}

func (e *Edict) MoveType(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldMoveType)
		return v
	}
	return 0
}
func (e *Edict) SetMoveType(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldMoveType, v)
	}
}

func (e *Edict) Solid(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		return s.QCVM.EFloat(e.Num, qc.EntFieldSolid)
	}
	return 0
}
func (e *Edict) SetSolid(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldSolid, v)
	}
}

func (e *Edict) Frame(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFrame)
		return v
	}
	return 0
}
func (e *Edict) SetFrame(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFrame, v)
	}
}

func (e *Edict) Skin(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldSkin)
		return v
	}
	return 0
}
func (e *Edict) SetSkin(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldSkin, v)
	}
}

func (e *Edict) Effects(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldEffects)
		return v
	}
	return 0
}
func (e *Edict) SetEffects(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldEffects, v)
	}
}

func (e *Edict) NextThink(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldNextThink)
		return v
	}
	return 0
}
func (e *Edict) SetNextThink(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldNextThink, v)
	}
}

func (e *Edict) Health(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldHealth)
		return v
	}
	return 0
}
func (e *Edict) SetHealth(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldHealth, v)
	}
}

func (e *Edict) Frags(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFrags)
		return v
	}
	return 0
}
func (e *Edict) SetFrags(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFrags, v)
	}
}

func (e *Edict) Weapon(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWeapon)
		return v
	}
	return 0
}
func (e *Edict) SetWeapon(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWeapon, v)
	}
}

func (e *Edict) WeaponFrame(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWeaponFrame)
		return v
	}
	return 0
}
func (e *Edict) SetWeaponFrame(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWeaponFrame, v)
	}
}

func (e *Edict) CurrentAmmo(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldCurrentAmmo)
		return v
	}
	return 0
}
func (e *Edict) SetCurrentAmmo(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldCurrentAmmo, v)
	}
}

func (e *Edict) AmmoShells(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoShells)
		return v
	}
	return 0
}
func (e *Edict) SetAmmoShells(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldAmmoShells, v)
	}
}

func (e *Edict) AmmoNails(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoNails)
		return v
	}
	return 0
}
func (e *Edict) SetAmmoNails(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldAmmoNails, v)
	}
}

func (e *Edict) AmmoRockets(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoRockets)
		return v
	}
	return 0
}
func (e *Edict) SetAmmoRockets(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldAmmoRockets, v)
	}
}

func (e *Edict) AmmoCells(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoCells)
		return v
	}
	return 0
}
func (e *Edict) SetAmmoCells(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldAmmoCells, v)
	}
}

func (e *Edict) Items(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldItems)
		return v
	}
	return 0
}
func (e *Edict) SetItems(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldItems, v)
	}
}

func (e *Edict) TakeDamage(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldTakeDamage)
		return v
	}
	return 0
}
func (e *Edict) SetTakeDamage(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldTakeDamage, v)
	}
}

func (e *Edict) DeadFlag(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldDeadFlag)
		return v
	}
	return 0
}
func (e *Edict) SetDeadFlag(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldDeadFlag, v)
	}
}

func (e *Edict) Button0(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldButton0)
		return v
	}
	return 0
}
func (e *Edict) SetButton0(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldButton0, v)
	}
}

func (e *Edict) Button1(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldButton1)
		return v
	}
	return 0
}
func (e *Edict) SetButton1(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldButton1, v)
	}
}

func (e *Edict) Button2(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldButton2)
		return v
	}
	return 0
}
func (e *Edict) SetButton2(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldButton2, v)
	}
}

func (e *Edict) Impulse(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldImpulse)
		return v
	}
	return 0
}
func (e *Edict) SetImpulse(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldImpulse, v)
	}
}

func (e *Edict) FixAngle(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFixAngle)
		return v
	}
	return 0
}
func (e *Edict) SetFixAngle(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFixAngle, v)
	}
}

func (e *Edict) IdealPitch(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldIdealPitch)
		return v
	}
	return 0
}
func (e *Edict) SetIdealPitch(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldIdealPitch, v)
	}
}

func (e *Edict) Flags(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFlags)
		return v
	}
	return 0
}
func (e *Edict) SetFlags(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFlags, v)
	}
}

func (e *Edict) Colormap(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldColormap)
		return v
	}
	return 0
}
func (e *Edict) SetColormap(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldColormap, v)
	}
}

func (e *Edict) Team(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldTeam)
		return v
	}
	return 0
}
func (e *Edict) SetTeam(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldTeam, v)
	}
}

func (e *Edict) MaxHealth(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldMaxHealth)
		return v
	}
	return 0
}
func (e *Edict) SetMaxHealth(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldMaxHealth, v)
	}
}

func (e *Edict) TeleportTime(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldTeleportTime)
		return v
	}
	return 0
}
func (e *Edict) SetTeleportTime(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldTeleportTime, v)
	}
}

func (e *Edict) ArmorType(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldArmorType)
		return v
	}
	return 0
}
func (e *Edict) SetArmorType(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldArmorType, v)
	}
}

func (e *Edict) ArmorValue(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldArmorValue)
		return v
	}
	return 0
}
func (e *Edict) SetArmorValue(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldArmorValue, v)
	}
}

func (e *Edict) WaterLevel(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWaterLevel)
		return v
	}
	return 0
}
func (e *Edict) SetWaterLevel(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWaterLevel, v)
	}
}

func (e *Edict) WaterType(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWaterType)
		return v
	}
	return 0
}
func (e *Edict) SetWaterType(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWaterType, v)
	}
}

func (e *Edict) IdealYaw(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldIdealYaw)
		return v
	}
	return 0
}
func (e *Edict) SetIdealYaw(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldIdealYaw, v)
	}
}

func (e *Edict) YawSpeed(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldYawSpeed)
		return v
	}
	return 0
}

func (e *Edict) SpawnFlags(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldSpawnFlags)
		return v
	}
	return 0
}
func (e *Edict) SetSpawnFlags(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldSpawnFlags, v)
	}
}

func (e *Edict) DmgTake(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldDmgTake)
		return v
	}
	return 0
}
func (e *Edict) SetDmgTake(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldDmgTake, v)
	}
}

func (e *Edict) DmgSave(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldDmgSave)
		return v
	}
	return 0
}
func (e *Edict) SetDmgSave(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldDmgSave, v)
	}
}

func (e *Edict) Sounds(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldSounds)
		return v
	}
	return 0
}
func (e *Edict) SetSounds(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldSounds, v)
	}
}

func (e *Edict) SetYawSpeed(s *Server, v float32) {
	if s != nil && s.QCVM != nil {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldYawSpeed, v)
	}
}

