// This file belongs to the Entity/QC subsystem: edict allocation, entity accessors, QuakeC field offsets, QC call tracing, and entity state types.

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
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetAbsMin(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAbsMin, v)
	}
}

func (e *Edict) AbsMax(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldAbsMax)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetAbsMax(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAbsMax, v)
	}
}

func (e *Edict) Origin(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetOrigin(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldOrigin, v)
	}
}

func (e *Edict) OldOrigin(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldOldOrigin)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetOldOrigin(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldOldOrigin, v)
	}
}

func (e *Edict) Velocity(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldVelocity)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetVelocity(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldVelocity, v)
	}
}

func (e *Edict) Angles(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldAngles)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetAngles(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAngles, v)
	}
}

func (e *Edict) AVelocity(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldAVelocity)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetAVelocity(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldAVelocity, v)
	}
}

func (e *Edict) PunchAngle(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldPunchAngle)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetPunchAngle(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldPunchAngle, v)
	}
}

func (e *Edict) Mins(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldMins)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetMins(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldMins, v)
	}
}

func (e *Edict) Maxs(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldMaxs)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetMaxs(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldMaxs, v)
	}
}

func (e *Edict) Size(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldSize)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetSize(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldSize, v)
	}
}

func (e *Edict) ViewOfs(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldViewOfs)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetViewOfs(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldViewOfs, v)
	}
}

func (e *Edict) VAngle(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldVAngle)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetVAngle(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldVAngle, v)
	}
}

func (e *Edict) MoveDir(s *Server) [3]float32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EVector(e.Num, qc.EntFieldMoveDir)
		return v
	}
	return [3]float32{}
}
func (e *Edict) SetMoveDir(s *Server, v [3]float32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEVector(e.Num, qc.EntFieldMoveDir, v)
	}
}

// ============================================================================
// int32 field accessors (string indices, entity refs, function refs)
// ============================================================================

func (e *Edict) ClassName(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldClassName)
		return v
	}
	return 0
}
func (e *Edict) SetClassName(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldClassName, v)
	}
}

func (e *Edict) Model(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldModel)
		return v
	}
	return 0
}
func (e *Edict) SetModel(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldModel, v)
	}
}

func (e *Edict) Touch(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldTouch)
		return v
	}
	return 0
}
func (e *Edict) SetTouch(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldTouch, v)
	}
}

func (e *Edict) Use(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldUse)
		return v
	}
	return 0
}
func (e *Edict) SetUse(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldUse, v)
	}
}

func (e *Edict) Think(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldThink)
		return v
	}
	return 0
}
func (e *Edict) SetThink(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldThink, v)
	}
}

func (e *Edict) Blocked(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldBlocked)
		return v
	}
	return 0
}
func (e *Edict) SetBlocked(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldBlocked, v)
	}
}

func (e *Edict) GroundEntity(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldGroundEnt)
		return v
	}
	return 0
}
func (e *Edict) SetGroundEntity(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldGroundEnt, v)
	}
}

func (e *Edict) Chain(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldChain)
		return v
	}
	return 0
}
func (e *Edict) SetChain(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldChain, v)
	}
}

func (e *Edict) WeaponModel(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldWeaponModel)
		return v
	}
	return 0
}
func (e *Edict) SetWeaponModel(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldWeaponModel, v)
	}
}

func (e *Edict) NetName(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNetName)
		return v
	}
	return 0
}
func (e *Edict) SetNetName(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNetName, v)
	}
}

func (e *Edict) Enemy(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldEnemy)
		return v
	}
	return 0
}
func (e *Edict) SetEnemy(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldEnemy, v)
	}
}

func (e *Edict) AimEnt(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldAimEnt)
		return v
	}
	return 0
}
func (e *Edict) SetAimEnt(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldAimEnt, v)
	}
}

func (e *Edict) GoalEntity(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldGoalEntity)
		return v
	}
	return 0
}
func (e *Edict) SetGoalEntity(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldGoalEntity, v)
	}
}

func (e *Edict) Target(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldTarget)
		return v
	}
	return 0
}
func (e *Edict) SetTarget(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldTarget, v)
	}
}

func (e *Edict) TargetName(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldTargetName)
		return v
	}
	return 0
}
func (e *Edict) SetTargetName(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldTargetName, v)
	}
}

func (e *Edict) DmgInflictor(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldDmgInflictor)
		return v
	}
	return 0
}
func (e *Edict) SetDmgInflictor(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldDmgInflictor, v)
	}
}

func (e *Edict) Owner(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldOwner)
		return v
	}
	return 0
}
func (e *Edict) SetOwner(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldOwner, v)
	}
}

func (e *Edict) Message(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldMessage)
		return v
	}
	return 0
}
func (e *Edict) SetMessage(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldMessage, v)
	}
}

func (e *Edict) Noise(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise)
		return v
	}
	return 0
}
func (e *Edict) SetNoise(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise, v)
	}
}

func (e *Edict) Noise1(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise1)
		return v
	}
	return 0
}
func (e *Edict) SetNoise1(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise1, v)
	}
}

func (e *Edict) Noise2(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise2)
		return v
	}
	return 0
}
func (e *Edict) SetNoise2(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise2, v)
	}
}

func (e *Edict) Noise3(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		v := s.QCVM.EInt(e.Num, qc.EntFieldNoise3)
		return v
	}
	return 0
}
func (e *Edict) SetNoise3(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
		s.QCVM.SetEInt(e.Num, qc.EntFieldNoise3, v)
	}
}

func (e *Edict) Map(s *Server) int32 {
	if s != nil && s.QCVM != nil && s.QCFieldMap >= 0 {
		v := s.QCVM.EInt(e.Num, s.QCFieldMap)
		return v
	}
	return 0
}
func (e *Edict) SetMap(s *Server, v int32) {
	if s != nil && s.QCVM != nil && s.QCFieldMap >= 0 {
		s.QCVM.SetEInt(e.Num, s.QCFieldMap, v)
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

func (e *Edict) MapString(s *Server) string {
	return s.String(e.Map(s))
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
