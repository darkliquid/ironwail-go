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
	return s.QCVM.EFloat(e.Num, qc.EntFieldModelIndex)
}
func (e *Edict) SetModelIndex(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldModelIndex, v)
}

func (e *Edict) LTime(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldLTime)
}
func (e *Edict) SetLTime(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldLTime, v)
}

func (e *Edict) MoveType(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldMoveType)
}
func (e *Edict) SetMoveType(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldMoveType, v)
}

func (e *Edict) Solid(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldSolid)
}
func (e *Edict) SetSolid(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldSolid, v)
}

func (e *Edict) Frame(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldFrame)
}
func (e *Edict) SetFrame(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldFrame, v)
}

func (e *Edict) Skin(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldSkin)
}
func (e *Edict) SetSkin(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldSkin, v)
}

func (e *Edict) Effects(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldEffects)
}
func (e *Edict) SetEffects(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldEffects, v)
}

func (e *Edict) NextThink(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldNextThink)
}
func (e *Edict) SetNextThink(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldNextThink, v)
}

func (e *Edict) Health(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldHealth)
}
func (e *Edict) SetHealth(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldHealth, v)
}

func (e *Edict) Frags(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldFrags)
}
func (e *Edict) SetFrags(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldFrags, v)
}

func (e *Edict) Weapon(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldWeapon)
}
func (e *Edict) SetWeapon(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldWeapon, v)
}

func (e *Edict) WeaponFrame(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldWeaponFrame)
}
func (e *Edict) SetWeaponFrame(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldWeaponFrame, v)
}

func (e *Edict) CurrentAmmo(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldCurrentAmmo)
}

func (e *Edict) AmmoShells(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldAmmoShells)
}

func (e *Edict) AmmoNails(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldAmmoNails)
}

func (e *Edict) AmmoRockets(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldAmmoRockets)
}

func (e *Edict) AmmoCells(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldAmmoCells)
}

func (e *Edict) Items(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldItems)
}
func (e *Edict) SetItems(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldItems, v)
}

func (e *Edict) TakeDamage(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldTakeDamage)
}
func (e *Edict) SetTakeDamage(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldTakeDamage, v)
}

func (e *Edict) DeadFlag(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldDeadFlag)
}
func (e *Edict) SetDeadFlag(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldDeadFlag, v)
}

func (e *Edict) Button0(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldButton0)
}
func (e *Edict) SetButton0(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldButton0, v)
}

func (e *Edict) Button1(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldButton1)
}
func (e *Edict) SetButton1(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldButton1, v)
}

func (e *Edict) Button2(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldButton2)
}
func (e *Edict) SetButton2(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldButton2, v)
}

func (e *Edict) Impulse(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldImpulse)
}
func (e *Edict) SetImpulse(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldImpulse, v)
}

func (e *Edict) FixAngle(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldFixAngle)
}
func (e *Edict) SetFixAngle(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldFixAngle, v)
}

func (e *Edict) IdealPitch(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldIdealPitch)
}
func (e *Edict) SetIdealPitch(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldIdealPitch, v)
}

func (e *Edict) Flags(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldFlags)
}
func (e *Edict) SetFlags(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldFlags, v)
}

func (e *Edict) Colormap(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldColormap)
}
func (e *Edict) SetColormap(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldColormap, v)
}

func (e *Edict) Team(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldTeam)
}
func (e *Edict) SetTeam(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldTeam, v)
}

func (e *Edict) MaxHealth(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldMaxHealth)
}
func (e *Edict) SetMaxHealth(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldMaxHealth, v)
}

func (e *Edict) TeleportTime(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldTeleportTime)
}
func (e *Edict) SetTeleportTime(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldTeleportTime, v)
}

func (e *Edict) ArmorType(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldArmorType)
}

func (e *Edict) ArmorValue(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldArmorValue)
}

func (e *Edict) WaterLevel(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldWaterLevel)
}
func (e *Edict) SetWaterLevel(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldWaterLevel, v)
}

func (e *Edict) WaterType(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldWaterType)
}
func (e *Edict) SetWaterType(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldWaterType, v)
}

func (e *Edict) IdealYaw(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldIdealYaw)
}
func (e *Edict) SetIdealYaw(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldIdealYaw, v)
}

func (e *Edict) YawSpeed(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldYawSpeed)
}

func (e *Edict) SpawnFlags(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldSpawnFlags)
}

func (e *Edict) DmgTake(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldDmgTake)
}
func (e *Edict) SetDmgTake(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldDmgTake, v)
}

func (e *Edict) DmgSave(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldDmgSave)
}
func (e *Edict) SetDmgSave(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldDmgSave, v)
}

func (e *Edict) Sounds(s *Server) float32 {
	return s.QCVM.EFloat(e.Num, qc.EntFieldSounds)
}
func (e *Edict) SetSounds(s *Server, v float32) {
	s.QCVM.SetEFloat(e.Num, qc.EntFieldSounds, v)
}

// ============================================================================
// [3]float32 (vector) field accessors
// ============================================================================

func (e *Edict) AbsMin(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldAbsMin)
}
func (e *Edict) SetAbsMin(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldAbsMin, v)
}

func (e *Edict) AbsMax(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldAbsMax)
}
func (e *Edict) SetAbsMax(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldAbsMax, v)
}

func (e *Edict) Origin(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
}
func (e *Edict) SetOrigin(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldOrigin, v)
}

func (e *Edict) OldOrigin(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldOldOrigin)
}
func (e *Edict) SetOldOrigin(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldOldOrigin, v)
}

func (e *Edict) Velocity(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldVelocity)
}
func (e *Edict) SetVelocity(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldVelocity, v)
}

func (e *Edict) Angles(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldAngles)
}
func (e *Edict) SetAngles(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldAngles, v)
}

func (e *Edict) AVelocity(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldAVelocity)
}
func (e *Edict) SetAVelocity(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldAVelocity, v)
}

func (e *Edict) PunchAngle(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldPunchAngle)
}
func (e *Edict) SetPunchAngle(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldPunchAngle, v)
}

func (e *Edict) Mins(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldMins)
}
func (e *Edict) SetMins(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldMins, v)
}

func (e *Edict) Maxs(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldMaxs)
}
func (e *Edict) SetMaxs(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldMaxs, v)
}

func (e *Edict) Size(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldSize)
}
func (e *Edict) SetSize(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldSize, v)
}

func (e *Edict) ViewOfs(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldViewOfs)
}
func (e *Edict) SetViewOfs(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldViewOfs, v)
}

func (e *Edict) VAngle(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldVAngle)
}
func (e *Edict) SetVAngle(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldVAngle, v)
}

func (e *Edict) MoveDir(s *Server) [3]float32 {
	return s.QCVM.EVector(e.Num, qc.EntFieldMoveDir)
}
func (e *Edict) SetMoveDir(s *Server, v [3]float32) {
	s.QCVM.SetEVector(e.Num, qc.EntFieldMoveDir, v)
}

// ============================================================================
// int32 field accessors (string indices, entity refs, function refs)
// ============================================================================

func (e *Edict) ClassName(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldClassName)
}
func (e *Edict) SetClassName(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldClassName, v)
}

func (e *Edict) Model(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldModel)
}
func (e *Edict) SetModel(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldModel, v)
}

func (e *Edict) Touch(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldTouch)
}
func (e *Edict) SetTouch(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldTouch, v)
}

func (e *Edict) Use(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldUse)
}
func (e *Edict) SetUse(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldUse, v)
}

func (e *Edict) Think(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldThink)
}
func (e *Edict) SetThink(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldThink, v)
}

func (e *Edict) Blocked(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldBlocked)
}
func (e *Edict) SetBlocked(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldBlocked, v)
}

func (e *Edict) GroundEntity(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldGroundEnt)
}
func (e *Edict) SetGroundEntity(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldGroundEnt, v)
}

func (e *Edict) Chain(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldChain)
}
func (e *Edict) SetChain(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldChain, v)
}

func (e *Edict) WeaponModel(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldWeaponModel)
}
func (e *Edict) SetWeaponModel(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldWeaponModel, v)
}

func (e *Edict) NetName(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldNetName)
}
func (e *Edict) SetNetName(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldNetName, v)
}

func (e *Edict) Enemy(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldEnemy)
}
func (e *Edict) SetEnemy(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldEnemy, v)
}

func (e *Edict) AimEnt(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldAimEnt)
}
func (e *Edict) SetAimEnt(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldAimEnt, v)
}

func (e *Edict) GoalEntity(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldGoalEntity)
}
func (e *Edict) SetGoalEntity(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldGoalEntity, v)
}

func (e *Edict) Target(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldTarget)
}
func (e *Edict) SetTarget(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldTarget, v)
}

func (e *Edict) TargetName(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldTargetName)
}
func (e *Edict) SetTargetName(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldTargetName, v)
}

func (e *Edict) DmgInflictor(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldDmgInflictor)
}

func (e *Edict) Owner(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldOwner)
}
func (e *Edict) SetOwner(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldOwner, v)
}

func (e *Edict) Message(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldMessage)
}
func (e *Edict) SetMessage(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldMessage, v)
}

func (e *Edict) Noise(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldNoise)
}
func (e *Edict) SetNoise(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldNoise, v)
}

func (e *Edict) Noise1(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldNoise1)
}
func (e *Edict) SetNoise1(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldNoise1, v)
}

func (e *Edict) Noise2(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldNoise2)
}
func (e *Edict) SetNoise2(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldNoise2, v)
}

func (e *Edict) Noise3(s *Server) int32 {
	return s.QCVM.EInt(e.Num, qc.EntFieldNoise3)
}
func (e *Edict) SetNoise3(s *Server, v int32) {
	s.QCVM.SetEInt(e.Num, qc.EntFieldNoise3, v)
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
