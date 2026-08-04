// entity_accessors.go provides typed read/write accessors to QCVM entity data for Edict.
package types

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// VMProvider abstracts any type that can provide access to the QuakeC Virtual Machine.
type VMProvider interface {
	GetVM() *qc.VM
}

func getVM(vmp VMProvider) *qc.VM {
	if vmp == nil {
		return nil
	}
	return vmp.GetVM()
}

// ============================================================================
// float32 field accessors
// ============================================================================

func (e *Edict) ModelIndex(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldModelIndex)
	}
	return 0
}
func (e *Edict) SetModelIndex(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldModelIndex, v)
	}
}

func (e *Edict) LTime(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldLTime)
	}
	return 0
}
func (e *Edict) SetLTime(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldLTime, v)
	}
}

func (e *Edict) MoveType(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldMoveType)
	}
	return 0
}
func (e *Edict) SetMoveType(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldMoveType, v)
	}
}

func (e *Edict) Solid(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldSolid)
	}
	return 0
}
func (e *Edict) SetSolid(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldSolid, v)
	}
}

func (e *Edict) Frame(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldFrame)
	}
	return 0
}
func (e *Edict) SetFrame(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldFrame, v)
	}
}

func (e *Edict) Skin(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldSkin)
	}
	return 0
}
func (e *Edict) SetSkin(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldSkin, v)
	}
}

func (e *Edict) Effects(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldEffects)
	}
	return 0
}
func (e *Edict) SetEffects(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldEffects, v)
	}
}

func (e *Edict) NextThink(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldNextThink)
	}
	return 0
}
func (e *Edict) SetNextThink(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldNextThink, v)
	}
}

func (e *Edict) Health(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldHealth)
	}
	return 0
}
func (e *Edict) SetHealth(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldHealth, v)
	}
}

func (e *Edict) Frags(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldFrags)
	}
	return 0
}
func (e *Edict) SetFrags(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldFrags, v)
	}
}

func (e *Edict) Weapon(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldWeapon)
	}
	return 0
}
func (e *Edict) SetWeapon(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldWeapon, v)
	}
}

func (e *Edict) WeaponFrame(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldWeaponFrame)
	}
	return 0
}
func (e *Edict) SetWeaponFrame(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldWeaponFrame, v)
	}
}

func (e *Edict) CurrentAmmo(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldCurrentAmmo)
	}
	return 0
}
func (e *Edict) SetCurrentAmmo(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldCurrentAmmo, v)
	}
}

func (e *Edict) AmmoShells(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldAmmoShells)
	}
	return 0
}
func (e *Edict) SetAmmoShells(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldAmmoShells, v)
	}
}

func (e *Edict) AmmoNails(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldAmmoNails)
	}
	return 0
}
func (e *Edict) SetAmmoNails(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldAmmoNails, v)
	}
}

func (e *Edict) AmmoRockets(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldAmmoRockets)
	}
	return 0
}
func (e *Edict) SetAmmoRockets(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldAmmoRockets, v)
	}
}

func (e *Edict) AmmoCells(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldAmmoCells)
	}
	return 0
}
func (e *Edict) SetAmmoCells(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldAmmoCells, v)
	}
}

func (e *Edict) Items(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldItems)
	}
	return 0
}
func (e *Edict) SetItems(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldItems, v)
	}
}

func (e *Edict) TakeDamage(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldTakeDamage)
	}
	return 0
}
func (e *Edict) SetTakeDamage(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldTakeDamage, v)
	}
}

func (e *Edict) DeadFlag(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldDeadFlag)
	}
	return 0
}
func (e *Edict) SetDeadFlag(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldDeadFlag, v)
	}
}

func (e *Edict) Button0(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldButton0)
	}
	return 0
}
func (e *Edict) SetButton0(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldButton0, v)
	}
}

func (e *Edict) Button1(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldButton1)
	}
	return 0
}
func (e *Edict) SetButton1(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldButton1, v)
	}
}

func (e *Edict) Button2(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldButton2)
	}
	return 0
}
func (e *Edict) SetButton2(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldButton2, v)
	}
}

func (e *Edict) Impulse(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldImpulse)
	}
	return 0
}
func (e *Edict) SetImpulse(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldImpulse, v)
	}
}

func (e *Edict) FixAngle(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldFixAngle)
	}
	return 0
}
func (e *Edict) SetFixAngle(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldFixAngle, v)
	}
}

func (e *Edict) IdealPitch(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldIdealPitch)
	}
	return 0
}
func (e *Edict) SetIdealPitch(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldIdealPitch, v)
	}
}

func (e *Edict) Flags(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldFlags)
	}
	return 0
}
func (e *Edict) SetFlags(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldFlags, v)
	}
}

func (e *Edict) Colormap(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldColormap)
	}
	return 0
}
func (e *Edict) SetColormap(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldColormap, v)
	}
}

func (e *Edict) Team(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldTeam)
	}
	return 0
}
func (e *Edict) SetTeam(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldTeam, v)
	}
}

func (e *Edict) MaxHealth(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldMaxHealth)
	}
	return 0
}
func (e *Edict) SetMaxHealth(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldMaxHealth, v)
	}
}

func (e *Edict) TeleportTime(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldTeleportTime)
	}
	return 0
}
func (e *Edict) SetTeleportTime(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldTeleportTime, v)
	}
}

func (e *Edict) ArmorType(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldArmorType)
	}
	return 0
}
func (e *Edict) SetArmorType(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldArmorType, v)
	}
}

func (e *Edict) ArmorValue(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldArmorValue)
	}
	return 0
}
func (e *Edict) SetArmorValue(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldArmorValue, v)
	}
}

func (e *Edict) WaterLevel(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldWaterLevel)
	}
	return 0
}
func (e *Edict) SetWaterLevel(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldWaterLevel, v)
	}
}

func (e *Edict) WaterType(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldWaterType)
	}
	return 0
}
func (e *Edict) SetWaterType(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldWaterType, v)
	}
}

func (e *Edict) IdealYaw(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldIdealYaw)
	}
	return 0
}
func (e *Edict) SetIdealYaw(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldIdealYaw, v)
	}
}

func (e *Edict) YawSpeed(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldYawSpeed)
	}
	return 0
}
func (e *Edict) SetYawSpeed(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil {
		vm.SetEFloat(e.Num, qc.EntFieldYawSpeed, v)
	}
}

func (e *Edict) SpawnFlags(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldSpawnFlags)
	}
	return 0
}
func (e *Edict) SetSpawnFlags(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldSpawnFlags, v)
	}
}

func (e *Edict) DmgTake(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldDmgTake)
	}
	return 0
}
func (e *Edict) SetDmgTake(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldDmgTake, v)
	}
}

func (e *Edict) DmgSave(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldDmgSave)
	}
	return 0
}
func (e *Edict) SetDmgSave(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldDmgSave, v)
	}
}

func (e *Edict) Sounds(vmp VMProvider) float32 {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		return vm.EFloat(e.Num, qc.EntFieldSounds)
	}
	return 0
}
func (e *Edict) SetSounds(vmp VMProvider, v float32) {
	if vm := getVM(vmp); vm != nil && vm.EdictSize > 28 {
		vm.SetEFloat(e.Num, qc.EntFieldSounds, v)
	}
}

