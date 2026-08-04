// entity_accessors_vec.go provides typed read/write vector and int32 field accessors for Edict.
package types

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// ServerHandle abstracts Server operations for entity field accessors.
type ServerHandle interface {
	VMProvider
	GetFieldAlpha() int
	GetFieldScale() int
	GetFieldGravity() int
	GetFieldItems2() int
	GetFieldState() int
	GetFieldWait() int
	GetFieldSpeed() int
	GetFieldCustomFlags() int
	GetFieldThCheckAttack() int
	GetFieldMap() int
	String(idx int32) string
}

// ============================================================================
// [3]float32 (vector) field accessors
// ============================================================================

func (e *Edict) AbsMin(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldAbsMin)
	}
	return [3]float32{}
}
func (e *Edict) SetAbsMin(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldAbsMin, v)
	}
}

func (e *Edict) AbsMax(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldAbsMax)
	}
	return [3]float32{}
}
func (e *Edict) SetAbsMax(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldAbsMax, v)
	}
}

func (e *Edict) Origin(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldOrigin)
	}
	return [3]float32{}
}
func (e *Edict) SetOrigin(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldOrigin, v)
	}
}

func (e *Edict) OldOrigin(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldOldOrigin)
	}
	return [3]float32{}
}
func (e *Edict) SetOldOrigin(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldOldOrigin, v)
	}
}

func (e *Edict) Velocity(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldVelocity)
	}
	return [3]float32{}
}
func (e *Edict) SetVelocity(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldVelocity, v)
	}
}

func (e *Edict) Angles(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldAngles)
	}
	return [3]float32{}
}
func (e *Edict) SetAngles(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldAngles, v)
	}
}

func (e *Edict) AVelocity(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldAVelocity)
	}
	return [3]float32{}
}
func (e *Edict) SetAVelocity(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldAVelocity, v)
	}
}

func (e *Edict) PunchAngle(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldPunchAngle)
	}
	return [3]float32{}
}
func (e *Edict) SetPunchAngle(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldPunchAngle, v)
	}
}

func (e *Edict) Mins(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldMins)
	}
	return [3]float32{}
}
func (e *Edict) SetMins(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldMins, v)
	}
}

func (e *Edict) Maxs(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldMaxs)
	}
	return [3]float32{}
}
func (e *Edict) SetMaxs(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldMaxs, v)
	}
}

func (e *Edict) Size(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldSize)
	}
	return [3]float32{}
}
func (e *Edict) SetSize(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldSize, v)
	}
}

func (e *Edict) ViewOfs(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldViewOfs)
	}
	return [3]float32{}
}
func (e *Edict) SetViewOfs(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldViewOfs, v)
	}
}

func (e *Edict) VAngle(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldVAngle)
	}
	return [3]float32{}
}
func (e *Edict) SetVAngle(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldVAngle, v)
	}
}

func (e *Edict) MoveDir(sh ServerHandle) [3]float32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EVector(e.Num, qc.EntFieldMoveDir)
	}
	return [3]float32{}
}
func (e *Edict) SetMoveDir(sh ServerHandle, v [3]float32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEVector(e.Num, qc.EntFieldMoveDir, v)
	}
}

// ============================================================================
// int32 field accessors (string indices, entity refs, function refs)
// ============================================================================

func (e *Edict) ClassName(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldClassName)
	}
	return 0
}
func (e *Edict) SetClassName(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldClassName, v)
	}
}

func (e *Edict) Model(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldModel)
	}
	return 0
}
func (e *Edict) SetModel(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldModel, v)
	}
}

func (e *Edict) Touch(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldTouch)
	}
	return 0
}
func (e *Edict) SetTouch(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldTouch, v)
	}
}

func (e *Edict) Use(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldUse)
	}
	return 0
}
func (e *Edict) SetUse(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldUse, v)
	}
}

func (e *Edict) Think(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldThink)
	}
	return 0
}
func (e *Edict) SetThink(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldThink, v)
	}
}

func (e *Edict) Blocked(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldBlocked)
	}
	return 0
}
func (e *Edict) SetBlocked(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldBlocked, v)
	}
}

func (e *Edict) GroundEntity(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldGroundEnt)
	}
	return 0
}
func (e *Edict) SetGroundEntity(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldGroundEnt, v)
	}
}

func (e *Edict) Chain(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldChain)
	}
	return 0
}
func (e *Edict) SetChain(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldChain, v)
	}
}

func (e *Edict) WeaponModel(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldWeaponModel)
	}
	return 0
}
func (e *Edict) SetWeaponModel(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldWeaponModel, v)
	}
}

func (e *Edict) NetName(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldNetName)
	}
	return 0
}
func (e *Edict) SetNetName(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldNetName, v)
	}
}

func (e *Edict) Enemy(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldEnemy)
	}
	return 0
}
func (e *Edict) SetEnemy(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldEnemy, v)
	}
}

func (e *Edict) AimEnt(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldAimEnt)
	}
	return 0
}
func (e *Edict) SetAimEnt(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldAimEnt, v)
	}
}

func (e *Edict) GoalEntity(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldGoalEntity)
	}
	return 0
}
func (e *Edict) SetGoalEntity(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldGoalEntity, v)
	}
}

func (e *Edict) Target(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldTarget)
	}
	return 0
}
func (e *Edict) SetTarget(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldTarget, v)
	}
}

func (e *Edict) TargetName(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldTargetName)
	}
	return 0
}
func (e *Edict) SetTargetName(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldTargetName, v)
	}
}

func (e *Edict) DmgInflictor(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldDmgInflictor)
	}
	return 0
}
func (e *Edict) SetDmgInflictor(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldDmgInflictor, v)
	}
}

func (e *Edict) Owner(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldOwner)
	}
	return 0
}
func (e *Edict) SetOwner(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldOwner, v)
	}
}

func (e *Edict) Message(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldMessage)
	}
	return 0
}
func (e *Edict) SetMessage(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldMessage, v)
	}
}

func (e *Edict) Noise(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldNoise)
	}
	return 0
}
func (e *Edict) SetNoise(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldNoise, v)
	}
}

func (e *Edict) Noise1(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldNoise1)
	}
	return 0
}
func (e *Edict) SetNoise1(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldNoise1, v)
	}
}

func (e *Edict) Noise2(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldNoise2)
	}
	return 0
}
func (e *Edict) SetNoise2(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldNoise2, v)
	}
}

func (e *Edict) Noise3(sh ServerHandle) int32 {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		return vm.EInt(e.Num, qc.EntFieldNoise3)
	}
	return 0
}
func (e *Edict) SetNoise3(sh ServerHandle, v int32) {
	if vm := getVM(sh); vm != nil && vm.EdictSize > 28 {
		vm.SetEInt(e.Num, qc.EntFieldNoise3, v)
	}
}

func (e *Edict) Map(sh ServerHandle) int32 {
	if sh != nil && sh.GetVM() != nil && sh.GetFieldMap() >= 0 {
		return sh.GetVM().EInt(e.Num, sh.GetFieldMap())
	}
	return 0
}
func (e *Edict) SetMap(sh ServerHandle, v int32) {
	if sh != nil && sh.GetVM() != nil && sh.GetFieldMap() >= 0 {
		sh.GetVM().SetEInt(e.Num, sh.GetFieldMap(), v)
	}
}

// ============================================================================
// String resolution helpers (resolve QC string table index → Go string)
// ============================================================================

func (e *Edict) ClassNameString(sh ServerHandle) string {
	if vm := getVM(sh); vm != nil {
		return vm.String(e.ClassName(sh))
	}
	return ""
}

func (e *Edict) TargetString(sh ServerHandle) string {
	if vm := getVM(sh); vm != nil {
		return vm.String(e.Target(sh))
	}
	return ""
}

func (e *Edict) TargetNameString(sh ServerHandle) string {
	if vm := getVM(sh); vm != nil {
		return vm.String(e.TargetName(sh))
	}
	return ""
}

func (e *Edict) ModelString(sh ServerHandle) string {
	if vm := getVM(sh); vm != nil {
		return vm.String(e.Model(sh))
	}
	return ""
}

func (e *Edict) MessageString(sh ServerHandle) string {
	if vm := getVM(sh); vm != nil {
		return vm.String(e.Message(sh))
	}
	return ""
}

func (e *Edict) NoiseString(sh ServerHandle) string {
	if vm := getVM(sh); vm != nil {
		return vm.String(e.Noise(sh))
	}
	return ""
}

func (e *Edict) Noise1String(sh ServerHandle) string {
	if vm := getVM(sh); vm != nil {
		return vm.String(e.Noise1(sh))
	}
	return ""
}

func (e *Edict) MapString(sh ServerHandle) string {
	if sh != nil {
		return sh.String(e.Map(sh))
	}
	return ""
}

// ============================================================================
// Extension field accessors (cached offsets from progs.dat FieldDefs)
// ============================================================================

func (e *Edict) State(sh ServerHandle) float32 {
	if sh == nil || sh.GetFieldState() < 0 {
		return 0
	}
	if vm := sh.GetVM(); vm != nil {
		return vm.EFloat(e.Num, sh.GetFieldState())
	}
	return 0
}
func (e *Edict) SetState(sh ServerHandle, v float32) {
	if sh != nil && sh.GetFieldState() >= 0 {
		if vm := sh.GetVM(); vm != nil {
			vm.SetEFloat(e.Num, sh.GetFieldState(), v)
		}
	}
}

func (e *Edict) Wait(sh ServerHandle) float32 {
	if sh == nil || sh.GetFieldWait() < 0 {
		return 0
	}
	if vm := sh.GetVM(); vm != nil {
		return vm.EFloat(e.Num, sh.GetFieldWait())
	}
	return 0
}

func (e *Edict) Speed(sh ServerHandle) float32 {
	if sh == nil || sh.GetFieldSpeed() < 0 {
		return 0
	}
	if vm := sh.GetVM(); vm != nil {
		return vm.EFloat(e.Num, sh.GetFieldSpeed())
	}
	return 0
}

func (e *Edict) CustomFlags(sh ServerHandle) float32 {
	if sh == nil || sh.GetFieldCustomFlags() < 0 {
		return 0
	}
	if vm := sh.GetVM(); vm != nil {
		return vm.EFloat(e.Num, sh.GetFieldCustomFlags())
	}
	return 0
}

func (e *Edict) ThCheckAttack(sh ServerHandle) int32 {
	if sh == nil || sh.GetFieldThCheckAttack() < 0 {
		return 0
	}
	if vm := sh.GetVM(); vm != nil {
		return vm.EInt(e.Num, sh.GetFieldThCheckAttack())
	}
	return 0
}
