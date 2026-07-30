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
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.ModelIndex
	}
	return 0
}
func (e *Edict) SetModelIndex(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldModelIndex, v)
	}
	if e.Vars != nil {
		e.Vars.ModelIndex = v
	}
}

func (e *Edict) LTime(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldLTime)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.LTime
	}
	return 0
}
func (e *Edict) SetLTime(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldLTime, v)
	}
	if e.Vars != nil {
		e.Vars.LTime = v
	}
}

func (e *Edict) MoveType(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldMoveType)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.MoveType
	}
	return 0
}
func (e *Edict) SetMoveType(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldMoveType, v)
	}
	if e.Vars != nil {
		e.Vars.MoveType = v
	}
}

func (e *Edict) Solid(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		return s.QCVM.EFloat(e.Num, qc.EntFieldSolid)
	}
	if e.Vars != nil {
		return e.Vars.Solid
	}
	return 0
}
func (e *Edict) SetSolid(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldSolid, v)
	}
	if e.Vars != nil {
		e.Vars.Solid = v
	}
}

func (e *Edict) Frame(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFrame)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Frame
	}
	return 0
}
func (e *Edict) SetFrame(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFrame, v)
	}
	if e.Vars != nil {
		e.Vars.Frame = v
	}
}

func (e *Edict) Skin(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldSkin)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Skin
	}
	return 0
}
func (e *Edict) SetSkin(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldSkin, v)
	}
	if e.Vars != nil {
		e.Vars.Skin = v
	}
}

func (e *Edict) Effects(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldEffects)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Effects
	}
	return 0
}
func (e *Edict) SetEffects(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldEffects, v)
	}
	if e.Vars != nil {
		e.Vars.Effects = v
	}
}

func (e *Edict) NextThink(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldNextThink)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.NextThink
	}
	return 0
}
func (e *Edict) SetNextThink(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldNextThink, v)
	}
	if e.Vars != nil {
		e.Vars.NextThink = v
	}
}

func (e *Edict) Health(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldHealth)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Health
	}
	return 0
}
func (e *Edict) SetHealth(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldHealth, v)
	}
	if e.Vars != nil {
		e.Vars.Health = v
	}
}

func (e *Edict) Frags(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFrags)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Frags
	}
	return 0
}
func (e *Edict) SetFrags(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFrags, v)
	}
	if e.Vars != nil {
		e.Vars.Frags = v
	}
}

func (e *Edict) Weapon(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWeapon)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Weapon
	}
	return 0
}
func (e *Edict) SetWeapon(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWeapon, v)
	}
	if e.Vars != nil {
		e.Vars.Weapon = v
	}
}

func (e *Edict) WeaponFrame(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWeaponFrame)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.WeaponFrame
	}
	return 0
}
func (e *Edict) SetWeaponFrame(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWeaponFrame, v)
	}
	if e.Vars != nil {
		e.Vars.WeaponFrame = v
	}
}

func (e *Edict) CurrentAmmo(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldCurrentAmmo)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.CurrentAmmo
	}
	return 0
}

func (e *Edict) AmmoShells(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoShells)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AmmoShells
	}
	return 0
}

func (e *Edict) AmmoNails(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoNails)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AmmoNails
	}
	return 0
}

func (e *Edict) AmmoRockets(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoRockets)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AmmoRockets
	}
	return 0
}

func (e *Edict) AmmoCells(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldAmmoCells)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AmmoCells
	}
	return 0
}

func (e *Edict) Items(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldItems)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Items
	}
	return 0
}
func (e *Edict) SetItems(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldItems, v)
	}
	if e.Vars != nil {
		e.Vars.Items = v
	}
}

func (e *Edict) TakeDamage(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldTakeDamage)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.TakeDamage
	}
	return 0
}
func (e *Edict) SetTakeDamage(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldTakeDamage, v)
	}
	if e.Vars != nil {
		e.Vars.TakeDamage = v
	}
}

func (e *Edict) DeadFlag(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldDeadFlag)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.DeadFlag
	}
	return 0
}
func (e *Edict) SetDeadFlag(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldDeadFlag, v)
	}
	if e.Vars != nil {
		e.Vars.DeadFlag = v
	}
}

func (e *Edict) Button0(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldButton0)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Button0
	}
	return 0
}
func (e *Edict) SetButton0(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldButton0, v)
	}
	if e.Vars != nil {
		e.Vars.Button0 = v
	}
}

func (e *Edict) Button1(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldButton1)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Button1
	}
	return 0
}
func (e *Edict) SetButton1(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldButton1, v)
	}
	if e.Vars != nil {
		e.Vars.Button1 = v
	}
}

func (e *Edict) Button2(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldButton2)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Button2
	}
	return 0
}
func (e *Edict) SetButton2(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldButton2, v)
	}
	if e.Vars != nil {
		e.Vars.Button2 = v
	}
}

func (e *Edict) Impulse(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldImpulse)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Impulse
	}
	return 0
}
func (e *Edict) SetImpulse(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldImpulse, v)
	}
	if e.Vars != nil {
		e.Vars.Impulse = v
	}
}

func (e *Edict) FixAngle(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFixAngle)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.FixAngle
	}
	return 0
}
func (e *Edict) SetFixAngle(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFixAngle, v)
	}
	if e.Vars != nil {
		e.Vars.FixAngle = v
	}
}

func (e *Edict) IdealPitch(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldIdealPitch)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.IdealPitch
	}
	return 0
}
func (e *Edict) SetIdealPitch(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldIdealPitch, v)
	}
	if e.Vars != nil {
		e.Vars.IdealPitch = v
	}
}

func (e *Edict) Flags(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldFlags)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Flags
	}
	return 0
}
func (e *Edict) SetFlags(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldFlags, v)
	}
	if e.Vars != nil {
		e.Vars.Flags = v
	}
}

func (e *Edict) Colormap(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldColormap)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Colormap
	}
	return 0
}
func (e *Edict) SetColormap(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldColormap, v)
	}
	if e.Vars != nil {
		e.Vars.Colormap = v
	}
}

func (e *Edict) Team(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldTeam)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Team
	}
	return 0
}
func (e *Edict) SetTeam(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldTeam, v)
	}
	if e.Vars != nil {
		e.Vars.Team = v
	}
}

func (e *Edict) MaxHealth(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldMaxHealth)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.MaxHealth
	}
	return 0
}
func (e *Edict) SetMaxHealth(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldMaxHealth, v)
	}
	if e.Vars != nil {
		e.Vars.MaxHealth = v
	}
}

func (e *Edict) TeleportTime(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldTeleportTime)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.TeleportTime
	}
	return 0
}
func (e *Edict) SetTeleportTime(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldTeleportTime, v)
	}
	if e.Vars != nil {
		e.Vars.TeleportTime = v
	}
}

func (e *Edict) ArmorType(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldArmorType)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.ArmorType
	}
	return 0
}

func (e *Edict) ArmorValue(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldArmorValue)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.ArmorValue
	}
	return 0
}

func (e *Edict) WaterLevel(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWaterLevel)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.WaterLevel
	}
	return 0
}
func (e *Edict) SetWaterLevel(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWaterLevel, v)
	}
	if e.Vars != nil {
		e.Vars.WaterLevel = v
	}
}

func (e *Edict) WaterType(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldWaterType)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.WaterType
	}
	return 0
}
func (e *Edict) SetWaterType(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldWaterType, v)
	}
	if e.Vars != nil {
		e.Vars.WaterType = v
	}
}

func (e *Edict) IdealYaw(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldIdealYaw)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.IdealYaw
	}
	return 0
}
func (e *Edict) SetIdealYaw(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldIdealYaw, v)
	}
	if e.Vars != nil {
		e.Vars.IdealYaw = v
	}
}

func (e *Edict) YawSpeed(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldYawSpeed)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.YawSpeed
	}
	return 0
}

func (e *Edict) SpawnFlags(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldSpawnFlags)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.SpawnFlags
	}
	return 0
}

func (e *Edict) DmgTake(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldDmgTake)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.DmgTake
	}
	return 0
}
func (e *Edict) SetDmgTake(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldDmgTake, v)
	}
	if e.Vars != nil {
		e.Vars.DmgTake = v
	}
}

func (e *Edict) DmgSave(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldDmgSave)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.DmgSave
	}
	return 0
}
func (e *Edict) SetDmgSave(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldDmgSave, v)
	}
	if e.Vars != nil {
		e.Vars.DmgSave = v
	}
}

func (e *Edict) Sounds(s *Server) float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EFloat(e.Num, qc.EntFieldSounds)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Sounds
	}
	return 0
}
func (e *Edict) SetSounds(s *Server, v float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldSounds, v)
	}
	if e.Vars != nil {
		e.Vars.Sounds = v
	}
}

func (e *Edict) SetYawSpeed(s *Server, v float32) {
	if s != nil && s.QCVM != nil {
		s.QCVM.SetEFloat(e.Num, qc.EntFieldYawSpeed, v)
	}
	if e.Vars != nil {
		e.Vars.YawSpeed = v
	}
}

