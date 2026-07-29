package server

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// ============================================================================
// [3]float32 (vector) field accessors
// ============================================================================

func (e *Edict) AbsMin(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldAbsMin)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AbsMin
	}
	return [3]float32{}
}
func (e *Edict) SetAbsMin(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAbsMin, v)
	}
	if e.Vars != nil {
		e.Vars.AbsMin = v
	}
}

func (e *Edict) AbsMax(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldAbsMax)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AbsMax
	}
	return [3]float32{}
}
func (e *Edict) SetAbsMax(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAbsMax, v)
	}
	if e.Vars != nil {
		e.Vars.AbsMax = v
	}
}

func (e *Edict) Origin(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Origin
	}
	return [3]float32{}
}
func (e *Edict) SetOrigin(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldOrigin, v)
	}
	if e.Vars != nil {
		e.Vars.Origin = v
	}
}

func (e *Edict) OldOrigin(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldOldOrigin)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.OldOrigin
	}
	return [3]float32{}
}
func (e *Edict) SetOldOrigin(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldOldOrigin, v)
	}
	if e.Vars != nil {
		e.Vars.OldOrigin = v
	}
}

func (e *Edict) Velocity(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldVelocity)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Velocity
	}
	return [3]float32{}
}
func (e *Edict) SetVelocity(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldVelocity, v)
	}
	if e.Vars != nil {
		e.Vars.Velocity = v
	}
}

func (e *Edict) Angles(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldAngles)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Angles
	}
	return [3]float32{}
}
func (e *Edict) SetAngles(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAngles, v)
	}
	if e.Vars != nil {
		e.Vars.Angles = v
	}
}

func (e *Edict) AVelocity(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldAVelocity)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AVelocity
	}
	return [3]float32{}
}
func (e *Edict) SetAVelocity(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAVelocity, v)
	}
	if e.Vars != nil {
		e.Vars.AVelocity = v
	}
}

func (e *Edict) PunchAngle(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldPunchAngle)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.PunchAngle
	}
	return [3]float32{}
}
func (e *Edict) SetPunchAngle(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldPunchAngle, v)
	}
	if e.Vars != nil {
		e.Vars.PunchAngle = v
	}
}

func (e *Edict) Mins(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldMins)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Mins
	}
	return [3]float32{}
}
func (e *Edict) SetMins(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldMins, v)
	}
	if e.Vars != nil {
		e.Vars.Mins = v
	}
}

func (e *Edict) Maxs(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldMaxs)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Maxs
	}
	return [3]float32{}
}
func (e *Edict) SetMaxs(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldMaxs, v)
	}
	if e.Vars != nil {
		e.Vars.Maxs = v
	}
}

func (e *Edict) Size(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldSize)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Size
	}
	return [3]float32{}
}
func (e *Edict) SetSize(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldSize, v)
	}
	if e.Vars != nil {
		e.Vars.Size = v
	}
}

func (e *Edict) ViewOfs(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldViewOfs)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.ViewOfs
	}
	return [3]float32{}
}
func (e *Edict) SetViewOfs(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldViewOfs, v)
	}
	if e.Vars != nil {
		e.Vars.ViewOfs = v
	}
}

func (e *Edict) VAngle(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldVAngle)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.VAngle
	}
	return [3]float32{}
}
func (e *Edict) SetVAngle(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldVAngle, v)
	}
	if e.Vars != nil {
		e.Vars.VAngle = v
	}
}

func (e *Edict) MoveDir(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldMoveDir)
		if v != [3]float32{} || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.MoveDir
	}
	return [3]float32{}
}
func (e *Edict) SetMoveDir(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldMoveDir, v)
	}
	if e.Vars != nil {
		e.Vars.MoveDir = v
	}
}

// ============================================================================
// int32 field accessors (string indices, entity refs, function refs)
// ============================================================================

func (e *Edict) ClassName(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldClassName)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.ClassName
	}
	return 0
}
func (e *Edict) SetClassName(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldClassName, v)
	}
	if e.Vars != nil {
		e.Vars.ClassName = v
	}
}

func (e *Edict) Model(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldModel)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Model
	}
	return 0
}
func (e *Edict) SetModel(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldModel, v)
	}
	if e.Vars != nil {
		e.Vars.Model = v
	}
}

func (e *Edict) Touch(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldTouch)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Touch
	}
	return 0
}
func (e *Edict) SetTouch(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldTouch, v)
	}
	if e.Vars != nil {
		e.Vars.Touch = v
	}
}

func (e *Edict) Use(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldUse)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Use
	}
	return 0
}
func (e *Edict) SetUse(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldUse, v)
	}
	if e.Vars != nil {
		e.Vars.Use = v
	}
}

func (e *Edict) Think(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldThink)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Think
	}
	return 0
}
func (e *Edict) SetThink(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldThink, v)
	}
	if e.Vars != nil {
		e.Vars.Think = v
	}
}

func (e *Edict) Blocked(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldBlocked)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Blocked
	}
	return 0
}
func (e *Edict) SetBlocked(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldBlocked, v)
	}
	if e.Vars != nil {
		e.Vars.Blocked = v
	}
}

func (e *Edict) GroundEntity(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldGroundEnt)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.GroundEntity
	}
	return 0
}
func (e *Edict) SetGroundEntity(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldGroundEnt, v)
	}
	if e.Vars != nil {
		e.Vars.GroundEntity = v
	}
}

func (e *Edict) Chain(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldChain)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Chain
	}
	return 0
}
func (e *Edict) SetChain(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldChain, v)
	}
	if e.Vars != nil {
		e.Vars.Chain = v
	}
}

func (e *Edict) WeaponModel(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldWeaponModel)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.WeaponModel
	}
	return 0
}
func (e *Edict) SetWeaponModel(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldWeaponModel, v)
	}
	if e.Vars != nil {
		e.Vars.WeaponModel = v
	}
}

func (e *Edict) NetName(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNetName)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.NetName
	}
	return 0
}
func (e *Edict) SetNetName(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNetName, v)
	}
	if e.Vars != nil {
		e.Vars.NetName = v
	}
}

func (e *Edict) Enemy(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldEnemy)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Enemy
	}
	return 0
}
func (e *Edict) SetEnemy(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldEnemy, v)
	}
	if e.Vars != nil {
		e.Vars.Enemy = v
	}
}

func (e *Edict) AimEnt(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldAimEnt)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.AimEnt
	}
	return 0
}
func (e *Edict) SetAimEnt(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldAimEnt, v)
	}
	if e.Vars != nil {
		e.Vars.AimEnt = v
	}
}

func (e *Edict) GoalEntity(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldGoalEntity)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.GoalEntity
	}
	return 0
}
func (e *Edict) SetGoalEntity(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldGoalEntity, v)
	}
	if e.Vars != nil {
		e.Vars.GoalEntity = v
	}
}

func (e *Edict) Target(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldTarget)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Target
	}
	return 0
}
func (e *Edict) SetTarget(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldTarget, v)
	}
	if e.Vars != nil {
		e.Vars.Target = v
	}
}

func (e *Edict) TargetName(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldTargetName)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.TargetName
	}
	return 0
}
func (e *Edict) SetTargetName(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldTargetName, v)
	}
	if e.Vars != nil {
		e.Vars.TargetName = v
	}
}

func (e *Edict) DmgInflictor(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldDmgInflictor)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.DmgInflictor
	}
	return 0
}

func (e *Edict) Owner(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldOwner)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Owner
	}
	return 0
}
func (e *Edict) SetOwner(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldOwner, v)
	}
	if e.Vars != nil {
		e.Vars.Owner = v
	}
}

func (e *Edict) Message(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldMessage)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Message
	}
	return 0
}
func (e *Edict) SetMessage(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldMessage, v)
	}
	if e.Vars != nil {
		e.Vars.Message = v
	}
}

func (e *Edict) Noise(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Noise
	}
	return 0
}
func (e *Edict) SetNoise(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise, v)
	}
	if e.Vars != nil {
		e.Vars.Noise = v
	}
}

func (e *Edict) Noise1(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise1)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Noise1
	}
	return 0
}
func (e *Edict) SetNoise1(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise1, v)
	}
	if e.Vars != nil {
		e.Vars.Noise1 = v
	}
}

func (e *Edict) Noise2(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise2)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Noise2
	}
	return 0
}
func (e *Edict) SetNoise2(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise2, v)
	}
	if e.Vars != nil {
		e.Vars.Noise2 = v
	}
}

func (e *Edict) Noise3(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise3)
		if v != 0 || e.Vars == nil {
			return v
		}
	}
	if e.Vars != nil {
		return e.Vars.Noise3
	}
	return 0
}
func (e *Edict) SetNoise3(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise3, v)
	}
	if e.Vars != nil {
		e.Vars.Noise3 = v
	}
}

// ============================================================================
// String resolution helpers (resolve QC string table index → Go string)
// ============================================================================

func (e *Edict) ClassNameString(s *Server) string {
	return s.QCVM.String(e.ClassName(s))
}

func (e *Edict) TargetString(s *Server) string {
	return s.QCVM.String(e.Target(s))
}

func (e *Edict) TargetNameString(s *Server) string {
	return s.QCVM.String(e.TargetName(s))
}

func (e *Edict) ModelString(s *Server) string {
	return s.QCVM.String(e.Model(s))
}

func (e *Edict) MessageString(s *Server) string {
	return s.QCVM.String(e.Message(s))
}

func (e *Edict) NoiseString(s *Server) string {
	return s.QCVM.String(e.Noise(s))
}

func (e *Edict) Noise1String(s *Server) string {
	return s.QCVM.String(e.Noise1(s))
}

// ============================================================================
// Extension field accessors (cached offsets from progs.dat FieldDefs)
// ============================================================================

// State returns the entity's state field (STATE_BOTTOM/TOP/UP/DOWN etc.)
func (e *Edict) State(s *Server) float32 {
	if s.QCFieldState < 0 {
		return 0
	}
	return s.QCVM.EFloat(e.Num, s.QCFieldState)
}
func (e *Edict) SetState(s *Server, v float32) {
	if s.QCFieldState >= 0 {
		s.QCVM.SetEFloat(e.Num, s.QCFieldState, v)
	}
}

// Wait returns the entity's wait time (trigger/door/plat debounce)
func (e *Edict) Wait(s *Server) float32 {
	if s.QCFieldWait < 0 {
		return 0
	}
	return s.QCVM.EFloat(e.Num, s.QCFieldWait)
}

// Speed returns the entity's movement speed
func (e *Edict) Speed(s *Server) float32 {
	if s.QCFieldSpeed < 0 {
		return 0
	}
	return s.QCVM.EFloat(e.Num, s.QCFieldSpeed)
}

// CustomFlags returns the entity's custom flags (CFL_LOCKED etc.)
func (e *Edict) CustomFlags(s *Server) float32 {
	if s.QCFieldCustomFlags < 0 {
		return 0
	}
	return s.QCVM.EFloat(e.Num, s.QCFieldCustomFlags)
}

// ThCheckAttack returns the entity's th_checkattack function index
func (e *Edict) ThCheckAttack(s *Server) int32 {
	if s.QCFieldThCheckAttack < 0 {
		return 0
	}
	return s.QCVM.EInt(e.Num, s.QCFieldThCheckAttack)
}
