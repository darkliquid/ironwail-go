// This file belongs to the Entity/QC subsystem: edict allocation, entity accessors, QuakeC field offsets, QC call tracing, and entity state types.

package server

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func (s *Server) syncEdictToQCVM(entNum int, ent *Edict) {
}

// syncEdictFromQCVM pulls one VM edict's fields back into the Go Edict struct.
// With accessor-based dual-write, QCVM data is already authoritative; this
// is now a no-op kept for call-site compatibility.
func (s *Server) syncEdictFromQCVM(entNum int, ent *Edict) {
}

func clearQCVMEdictData(vm *qc.VM, entNum int) {
	if vm == nil {
		return
	}
	data := vm.EdictData(entNum)
	if data == nil {
		return
	}
	clear(data)
}

// EdictClearQCVMFunc is the injected QCVM-clear dependency for the edict
// subpackage (edict.NewManager).
func EdictClearQCVMFunc(vm *qc.VM, entNum int) {
	clearQCVMEdictData(vm, entNum)
}

// EdictDefaultOffsets returns the default entvar field offset table used by
// map/savegame entity parsing in the edict subpackage.
func EdictDefaultOffsets() map[string]int {
	return defaultEntFieldOffsets
}

// ensureDefaultQCVMEdictStorage sets up QCVM edict storage with default
// parameters when no progs.dat has been loaded yet. This ensures accessor
// methods work during tests and early initialization. When progs.dat has
// already been loaded (EdictSize > 0), it still allocates the Edicts byte
// array if it hasn't been allocated yet — this happens in the production
// init order where LoadProgs is called before Init.
func (s *Server) ensureDefaultQCVMEdictStorage() {
	if s.QCVM == nil {
		return
	}
	if s.QCVM.EdictSize == 0 {
		s.QCVM.EntityFields = 128
		s.QCVM.EdictSize = 28 + s.QCVM.EntityFields*4
	}
	maxEdicts := s.MaxEdicts
	if maxEdicts < s.NumEdicts {
		maxEdicts = s.NumEdicts
	}
	if maxEdicts <= 0 {
		maxEdicts = s.NumEdicts
	}
	if s.QCVM.MaxEdicts < maxEdicts {
		s.QCVM.MaxEdicts = maxEdicts
	}
	if s.QCVM.NumEdicts < s.NumEdicts {
		s.QCVM.NumEdicts = s.NumEdicts
	}
	needed := s.QCVM.EdictSize * s.QCVM.MaxEdicts
	if needed > 0 && len(s.QCVM.Edicts) < needed {
		grown := make([]byte, needed)
		copy(grown, s.QCVM.Edicts)
		s.QCVM.Edicts = grown
	}
}

func (s *Server) syncSpawnedEdictsFromQCVM(startEntNum int) {
	if s == nil || s.QCVM == nil {
		return
	}
	if startEntNum < 0 {
		startEntNum = 0
	}
	limit := s.NumEdicts
	if limit > len(s.Edicts) {
		limit = len(s.Edicts)
	}
	for entNum := startEntNum; entNum < limit; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free {
			continue
		}
		if entNum == 0 || int(ent.Solid(s)) == int(SolidNot) {
			continue
		}
		s.LinkEdict(ent, false)
	}
}

func (s *Server) worldLeafIndex(leaf *bsp.TreeLeaf) int {
	if s == nil || s.WorldTree == nil || leaf == nil {
		return -1
	}
	for i := range s.WorldTree.Leafs {
		if &s.WorldTree.Leafs[i] == leaf {
			return i - 1
		}
	}
	return -1
}

func (s *Server) newCheckClient() int {
	if s == nil || s.Static == nil || len(s.Static.Clients) == 0 {
		s.checkClientPVS = nil
		return 0
	}
	maxClients := s.MaxClients()
	if maxClients <= 0 || maxClients > len(s.Static.Clients) {
		maxClients = len(s.Static.Clients)
	}
	if maxClients == 0 {
		s.checkClientPVS = nil
		return 0
	}
	check := s.checkClientSlot
	if check < 1 {
		check = 1
	}
	if check > maxClients {
		check = maxClients
	}
	i := 1
	if check != maxClients {
		i = check + 1
	}
	for {
		if i == maxClients+1 {
			i = 1
		}
		client := s.Static.Clients[i-1]
		if i == check {
			break
		}
		if client == nil || !client.Active || client.Edict == nil || client.Edict.Free {
			i++
			continue
		}
		if client.Edict.Health(s) <= 0 {
			i++
			continue
		}
		if uint32(client.Edict.Flags(s))&FlagNoTarget != 0 {
			i++
			continue
		}
		break
	}
	s.checkClientSlot = i
	s.checkClientPVS = nil
	if i < 1 || i > maxClients {
		return 0
	}
	client := s.Static.Clients[i-1]
	if client == nil || client.Edict == nil || client.Edict.Free || client.Edict.Health(s) <= 0 {
		return 0
	}
	if s.WorldTree != nil && len(s.WorldTree.Nodes) > 0 {
		org := client.Edict.Origin(s)
		viewOfs := client.Edict.ViewOfs(s)
		org = [3]float32{org[0] + viewOfs[0], org[1] + viewOfs[1], org[2] + viewOfs[2]}
		if leaf := s.WorldTree.PointInLeaf(org); leaf != nil {
			s.checkClientPVS = append(s.checkClientPVS[:0], s.WorldTree.LeafPVS(leaf)...)
		}
	}
	return s.NumForEdict(client.Edict)
}

var defaultEntFieldOffsets = map[string]int{
	normalizeFieldName("ModelIndex"):   qc.EntFieldModelIndex,
	normalizeFieldName("AbsMin"):       qc.EntFieldAbsMin,
	normalizeFieldName("AbsMax"):       qc.EntFieldAbsMax,
	normalizeFieldName("LTime"):        qc.EntFieldLTime,
	normalizeFieldName("MoveType"):     qc.EntFieldMoveType,
	normalizeFieldName("Solid"):        qc.EntFieldSolid,
	normalizeFieldName("Origin"):       qc.EntFieldOrigin,
	normalizeFieldName("OldOrigin"):    qc.EntFieldOldOrigin,
	normalizeFieldName("Velocity"):     qc.EntFieldVelocity,
	normalizeFieldName("Angles"):       qc.EntFieldAngles,
	normalizeFieldName("AVelocity"):    qc.EntFieldAVelocity,
	normalizeFieldName("PunchAngle"):   qc.EntFieldPunchAngle,
	normalizeFieldName("ClassName"):    qc.EntFieldClassName,
	normalizeFieldName("Model"):        qc.EntFieldModel,
	normalizeFieldName("Frame"):        qc.EntFieldFrame,
	normalizeFieldName("Skin"):         qc.EntFieldSkin,
	normalizeFieldName("Effects"):      qc.EntFieldEffects,
	normalizeFieldName("Mins"):         qc.EntFieldMins,
	normalizeFieldName("Maxs"):         qc.EntFieldMaxs,
	normalizeFieldName("Size"):         qc.EntFieldSize,
	normalizeFieldName("Touch"):        qc.EntFieldTouch,
	normalizeFieldName("Use"):          qc.EntFieldUse,
	normalizeFieldName("Think"):        qc.EntFieldThink,
	normalizeFieldName("Blocked"):      qc.EntFieldBlocked,
	normalizeFieldName("NextThink"):    qc.EntFieldNextThink,
	normalizeFieldName("GroundEntity"): qc.EntFieldGroundEnt,
	normalizeFieldName("Health"):       qc.EntFieldHealth,
	normalizeFieldName("Frags"):        qc.EntFieldFrags,
	normalizeFieldName("Weapon"):       qc.EntFieldWeapon,
	normalizeFieldName("WeaponModel"):  qc.EntFieldWeaponModel,
	normalizeFieldName("WeaponFrame"):  qc.EntFieldWeaponFrame,
	normalizeFieldName("CurrentAmmo"):  qc.EntFieldCurrentAmmo,
	normalizeFieldName("AmmoShells"):   qc.EntFieldAmmoShells,
	normalizeFieldName("AmmoNails"):    qc.EntFieldAmmoNails,
	normalizeFieldName("AmmoRockets"):  qc.EntFieldAmmoRockets,
	normalizeFieldName("AmmoCells"):    qc.EntFieldAmmoCells,
	normalizeFieldName("Items"):        qc.EntFieldItems,
	normalizeFieldName("TakeDamage"):   qc.EntFieldTakeDamage,
	normalizeFieldName("Chain"):        qc.EntFieldChain,
	normalizeFieldName("DeadFlag"):     qc.EntFieldDeadFlag,
	normalizeFieldName("ViewOfs"):      qc.EntFieldViewOfs,
	normalizeFieldName("Button0"):      qc.EntFieldButton0,
	normalizeFieldName("Button1"):      qc.EntFieldButton1,
	normalizeFieldName("Button2"):      qc.EntFieldButton2,
	normalizeFieldName("Impulse"):      qc.EntFieldImpulse,
	normalizeFieldName("FixAngle"):     qc.EntFieldFixAngle,
	normalizeFieldName("VAngle"):       qc.EntFieldVAngle,
	normalizeFieldName("IdealPitch"):   qc.EntFieldIdealPitch,
	normalizeFieldName("NetName"):      qc.EntFieldNetName,
	normalizeFieldName("Enemy"):        qc.EntFieldEnemy,
	normalizeFieldName("Flags"):        qc.EntFieldFlags,
	normalizeFieldName("Colormap"):     qc.EntFieldColormap,
	normalizeFieldName("Team"):         qc.EntFieldTeam,
	normalizeFieldName("MaxHealth"):    qc.EntFieldMaxHealth,
	normalizeFieldName("TeleportTime"): qc.EntFieldTeleportTime,
	normalizeFieldName("ArmorType"):    qc.EntFieldArmorType,
	normalizeFieldName("ArmorValue"):   qc.EntFieldArmorValue,
	normalizeFieldName("WaterLevel"):   qc.EntFieldWaterLevel,
	normalizeFieldName("WaterType"):    qc.EntFieldWaterType,
	normalizeFieldName("IdealYaw"):     qc.EntFieldIdealYaw,
	normalizeFieldName("YawSpeed"):     qc.EntFieldYawSpeed,
	normalizeFieldName("AimEnt"):       qc.EntFieldAimEnt,
	normalizeFieldName("GoalEntity"):   qc.EntFieldGoalEntity,
	normalizeFieldName("SpawnFlags"):   qc.EntFieldSpawnFlags,
	normalizeFieldName("Target"):       qc.EntFieldTarget,
	normalizeFieldName("TargetName"):   qc.EntFieldTargetName,
	normalizeFieldName("DmgTake"):      qc.EntFieldDmgTake,
	normalizeFieldName("DmgSave"):      qc.EntFieldDmgSave,
	normalizeFieldName("DmgInflictor"): qc.EntFieldDmgInflictor,
	normalizeFieldName("Owner"):        qc.EntFieldOwner,
	normalizeFieldName("MoveDir"):      qc.EntFieldMoveDir,
	normalizeFieldName("Message"):      qc.EntFieldMessage,
	normalizeFieldName("Sounds"):       qc.EntFieldSounds,
	normalizeFieldName("Noise"):        qc.EntFieldNoise,
	normalizeFieldName("Noise1"):       qc.EntFieldNoise1,
	normalizeFieldName("Noise2"):       qc.EntFieldNoise2,
	normalizeFieldName("Noise3"):       qc.EntFieldNoise3,
}

// ensureQCVMEdictStorage grows VM edict backing storage to match server edict capacity.
// QuakeC addresses entities by index into a flat byte block; this guarantees indexes
// the server hands to QC are always valid before any builtin or script executes.
func (s *Server) ensureQCVMEdictStorage() {
	if s.QCVM == nil || s.QCVM.EdictSize <= 0 {
		return
	}
	maxEdicts := s.MaxEdicts
	if maxEdicts < s.NumEdicts {
		maxEdicts = s.NumEdicts
	}
	if maxEdicts <= 0 {
		maxEdicts = s.NumEdicts
	}
	if s.QCVM.MaxEdicts < maxEdicts {
		s.QCVM.MaxEdicts = maxEdicts
	}
	needed := s.QCVM.EdictSize * s.QCVM.MaxEdicts
	if len(s.QCVM.Edicts) < needed {
		grown := make([]byte, needed)
		copy(grown, s.QCVM.Edicts)
		s.QCVM.Edicts = grown
	}
	if s.QCVM.NumEdicts < s.NumEdicts {
		s.QCVM.NumEdicts = s.NumEdicts
	}
}

// syncQCVMState publishes core server globals and all live edicts into the QC VM.
// This is called at key boundaries (e.g. map spawn/load) so QuakeC starts from an
// accurate world snapshot before executing functions like worldspawn or client logic.
func (s *Server) syncQCVMState() {
	if s.QCVM == nil {
		return
	}
	s.ensureQCVMEdictStorage()
	s.syncQCVMGlobals()

	for entNum := 0; entNum < s.NumEdicts; entNum++ {
		s.syncEdictToQCVM(entNum, s.EdictNum(entNum))
	}
}

// syncQCVMGlobals publishes core server globals (time, world, mapname,
// serverflags, coop, deathmatch) into the QC VM without touching per-entity
// storage. This is the per-frame equivalent of C's pr_global_struct->time =
// sv.time before StartFrame runs. It must NOT sync per-entity fields because
// that would overwrite QC bytecode mutations (nextthink, think, velocity,
// etc.) that were written via OP_STORE_* during the previous frame's QC
// callbacks.
func (s *Server) syncQCVMGlobals() {
	if s.QCVM == nil {
		return
	}
	s.ensureQCVMEdictStorage()
	s.QCVM.SetGlobal("world", 0)
	s.QCVM.SetGlobal("mapname", s.QCVM.AllocString(s.Name))
	s.QCVM.SetGlobalFloat("time", s.Time)
	if s.Static != nil {
		s.QCVM.SetGlobalInt32("serverflags", int32(s.Static.ServerFlags))
	}

	coopVal := s.CVar.FloatValue("coop")
	dmVal := s.CVar.FloatValue("deathmatch")
	if coopVal != 0 {
		s.QCVM.SetGlobalFloat("coop", float32(coopVal))
	} else {
		s.QCVM.SetGlobalFloat("deathmatch", float32(dmVal))
	}
}

func (s *Server) setQCTimeGlobal(time float32) {
	if s.QCVM == nil {
		return
	}
	s.QCVM.Time = float64(time)
	s.QCVM.SetGlobalFloat("time", time)
}

// NewServer creates a new server instance.
