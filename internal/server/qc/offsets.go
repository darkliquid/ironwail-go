// offsets.go implements the QuakeC VM entvar field offset table.
package qc

import (
	"strings"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

var (
	normalizedNamesMu    sync.RWMutex
	normalizedNamesCache = make(map[string]string, 512)
)

// NormalizeFieldName strips underscores and lowercases the input string to
// produce a canonical form suitable for case-insensitive,
// underscore-insensitive field name matching.
func NormalizeFieldName(name string) string {
	normalizedNamesMu.RLock()
	if n, ok := normalizedNamesCache[name]; ok {
		normalizedNamesMu.RUnlock()
		return n
	}
	normalizedNamesMu.RUnlock()

	n := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	normalizedNamesMu.Lock()
	normalizedNamesCache[name] = n
	normalizedNamesMu.Unlock()
	return n
}

// DefaultEntFieldOffsets returns the default entvar field offset table used by
// map/savegame entity parsing in the edict subpackage.
func DefaultEntFieldOffsets() map[string]int {
	return defaultEntFieldOffsets
}

var defaultEntFieldOffsets = map[string]int{
	NormalizeFieldName("ModelIndex"):   qc.EntFieldModelIndex,
	NormalizeFieldName("AbsMin"):       qc.EntFieldAbsMin,
	NormalizeFieldName("AbsMax"):       qc.EntFieldAbsMax,
	NormalizeFieldName("LTime"):        qc.EntFieldLTime,
	NormalizeFieldName("MoveType"):     qc.EntFieldMoveType,
	NormalizeFieldName("Solid"):        qc.EntFieldSolid,
	NormalizeFieldName("Origin"):       qc.EntFieldOrigin,
	NormalizeFieldName("OldOrigin"):    qc.EntFieldOldOrigin,
	NormalizeFieldName("Velocity"):     qc.EntFieldVelocity,
	NormalizeFieldName("Angles"):       qc.EntFieldAngles,
	NormalizeFieldName("AVelocity"):    qc.EntFieldAVelocity,
	NormalizeFieldName("PunchAngle"):   qc.EntFieldPunchAngle,
	NormalizeFieldName("ClassName"):    qc.EntFieldClassName,
	NormalizeFieldName("Model"):        qc.EntFieldModel,
	NormalizeFieldName("Frame"):        qc.EntFieldFrame,
	NormalizeFieldName("Skin"):         qc.EntFieldSkin,
	NormalizeFieldName("Effects"):      qc.EntFieldEffects,
	NormalizeFieldName("Mins"):         qc.EntFieldMins,
	NormalizeFieldName("Maxs"):         qc.EntFieldMaxs,
	NormalizeFieldName("Size"):         qc.EntFieldSize,
	NormalizeFieldName("Touch"):        qc.EntFieldTouch,
	NormalizeFieldName("Use"):          qc.EntFieldUse,
	NormalizeFieldName("Think"):        qc.EntFieldThink,
	NormalizeFieldName("Blocked"):      qc.EntFieldBlocked,
	NormalizeFieldName("NextThink"):    qc.EntFieldNextThink,
	NormalizeFieldName("GroundEntity"): qc.EntFieldGroundEnt,
	NormalizeFieldName("Health"):       qc.EntFieldHealth,
	NormalizeFieldName("Frags"):        qc.EntFieldFrags,
	NormalizeFieldName("Weapon"):       qc.EntFieldWeapon,
	NormalizeFieldName("WeaponModel"):  qc.EntFieldWeaponModel,
	NormalizeFieldName("WeaponFrame"):  qc.EntFieldWeaponFrame,
	NormalizeFieldName("CurrentAmmo"):  qc.EntFieldCurrentAmmo,
	NormalizeFieldName("AmmoShells"):   qc.EntFieldAmmoShells,
	NormalizeFieldName("AmmoNails"):    qc.EntFieldAmmoNails,
	NormalizeFieldName("AmmoRockets"):  qc.EntFieldAmmoRockets,
	NormalizeFieldName("AmmoCells"):    qc.EntFieldAmmoCells,
	NormalizeFieldName("Items"):        qc.EntFieldItems,
	NormalizeFieldName("TakeDamage"):   qc.EntFieldTakeDamage,
	NormalizeFieldName("Chain"):        qc.EntFieldChain,
	NormalizeFieldName("DeadFlag"):     qc.EntFieldDeadFlag,
	NormalizeFieldName("ViewOfs"):      qc.EntFieldViewOfs,
	NormalizeFieldName("Button0"):      qc.EntFieldButton0,
	NormalizeFieldName("Button1"):      qc.EntFieldButton1,
	NormalizeFieldName("Button2"):      qc.EntFieldButton2,
	NormalizeFieldName("Impulse"):      qc.EntFieldImpulse,
	NormalizeFieldName("FixAngle"):     qc.EntFieldFixAngle,
	NormalizeFieldName("VAngle"):       qc.EntFieldVAngle,
	NormalizeFieldName("IdealPitch"):   qc.EntFieldIdealPitch,
	NormalizeFieldName("NetName"):      qc.EntFieldNetName,
	NormalizeFieldName("Enemy"):        qc.EntFieldEnemy,
	NormalizeFieldName("Flags"):        qc.EntFieldFlags,
	NormalizeFieldName("Colormap"):     qc.EntFieldColormap,
	NormalizeFieldName("Team"):         qc.EntFieldTeam,
	NormalizeFieldName("MaxHealth"):    qc.EntFieldMaxHealth,
	NormalizeFieldName("TeleportTime"): qc.EntFieldTeleportTime,
	NormalizeFieldName("ArmorType"):    qc.EntFieldArmorType,
	NormalizeFieldName("ArmorValue"):   qc.EntFieldArmorValue,
	NormalizeFieldName("WaterLevel"):   qc.EntFieldWaterLevel,
	NormalizeFieldName("WaterType"):    qc.EntFieldWaterType,
	NormalizeFieldName("IdealYaw"):     qc.EntFieldIdealYaw,
	NormalizeFieldName("YawSpeed"):     qc.EntFieldYawSpeed,
	NormalizeFieldName("AimEnt"):       qc.EntFieldAimEnt,
	NormalizeFieldName("GoalEntity"):   qc.EntFieldGoalEntity,
	NormalizeFieldName("SpawnFlags"):   qc.EntFieldSpawnFlags,
	NormalizeFieldName("Target"):       qc.EntFieldTarget,
	NormalizeFieldName("TargetName"):   qc.EntFieldTargetName,
	NormalizeFieldName("DmgTake"):      qc.EntFieldDmgTake,
	NormalizeFieldName("DmgSave"):      qc.EntFieldDmgSave,
	NormalizeFieldName("DmgInflictor"): qc.EntFieldDmgInflictor,
	NormalizeFieldName("Owner"):        qc.EntFieldOwner,
	NormalizeFieldName("MoveDir"):      qc.EntFieldMoveDir,
	NormalizeFieldName("Message"):      qc.EntFieldMessage,
	NormalizeFieldName("Sounds"):       qc.EntFieldSounds,
	NormalizeFieldName("Noise"):        qc.EntFieldNoise,
	NormalizeFieldName("Noise1"):       qc.EntFieldNoise1,
	NormalizeFieldName("Noise2"):       qc.EntFieldNoise2,
	NormalizeFieldName("Noise3"):       qc.EntFieldNoise3,
}
