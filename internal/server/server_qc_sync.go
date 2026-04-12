package server

import (
	"bytes"
	"reflect"
	"strconv"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func syncEdictToQCVM(vm *qc.VM, entNum int, ent *Edict) {
	if vm == nil || ent == nil || ent.Vars == nil || entNum < 0 || entNum >= vm.NumEdicts {
		return
	}
	cache := qcSyncCacheForVM(vm)
	syncEntVarsToQC(vm, entNum, ent.Vars, cache.entVarBindings)
	if ent.Free && cache.modelIndexOfs >= 0 {
		vm.SetEFloat(entNum, cache.modelIndexOfs, 0)
	}
}

// syncEdictFromQCVM pulls one VM edict's fields back into the Go Edict struct.
// It is used after QC mutates fields so server physics/network code can continue
// from the updated authoritative values produced by QuakeC logic.
func syncEdictFromQCVM(vm *qc.VM, entNum int, ent *Edict) {
	if vm == nil || ent == nil || ent.Vars == nil || entNum < 0 || entNum >= vm.NumEdicts {
		return
	}
	cache := qcSyncCacheForVM(vm)
	syncEntVarsFromQC(vm, entNum, ent.Vars, cache.entVarBindings)
	if ent.Vars.Model == 0 || vm.GetString(ent.Vars.Model) == "" {
		ent.Vars.ModelIndex = 0
	}
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
		if ent.Vars == nil {
			ent.Vars = &EntVars{}
		}
		syncEdictFromQCVM(s.QCVM, entNum, ent)
		if entNum == 0 || int(ent.Vars.Solid) == int(SolidNot) {
			continue
		}
		s.LinkEdict(ent, false)
	}
}

type qcVMEdictSnapshot struct {
	entNum int
	data   []byte
}

func (s *Server) captureNonPusherQCVMEdictSnapshots() []qcVMEdictSnapshot {
	if s == nil || s.QCVM == nil {
		return nil
	}
	limit := s.NumEdicts
	if limit > len(s.Edicts) {
		limit = len(s.Edicts)
	}
	snapshots := make([]qcVMEdictSnapshot, 0, limit)
	for entNum := 0; entNum < limit; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if MoveType(ent.Vars.MoveType) == MoveTypePush {
			continue
		}
		data := s.QCVM.EdictData(entNum)
		if data == nil {
			continue
		}
		snapshots = append(snapshots, qcVMEdictSnapshot{
			entNum: entNum,
			data:   append([]byte(nil), data...),
		})
	}
	return snapshots
}

func (s *Server) syncMutatedNonPushersFromQCVM(snapshots []qcVMEdictSnapshot) {
	if s == nil || s.QCVM == nil {
		return
	}
	for _, snapshot := range snapshots {
		if snapshot.entNum < 0 || snapshot.entNum >= s.NumEdicts || snapshot.entNum >= len(s.Edicts) {
			continue
		}
		ent := s.Edicts[snapshot.entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if MoveType(ent.Vars.MoveType) == MoveTypePush {
			continue
		}
		current := s.QCVM.EdictData(snapshot.entNum)
		if current == nil || bytes.Equal(current, snapshot.data) {
			continue
		}
		oldOrigin := ent.Vars.Origin
		oldSolid := ent.Vars.Solid
		oldMins := ent.Vars.Mins
		oldMaxs := ent.Vars.Maxs
		oldModel := ent.Vars.Model
		oldModelIndex := ent.Vars.ModelIndex
		syncEdictFromQCVM(s.QCVM, snapshot.entNum, ent)
		if snapshot.entNum != 0 && (ent.Vars.Origin != oldOrigin || ent.Vars.Solid != oldSolid || ent.Vars.Mins != oldMins || ent.Vars.Maxs != oldMaxs || ent.Vars.Model != oldModel || ent.Vars.ModelIndex != oldModelIndex) {
			s.LinkEdict(ent, false)
		}
	}
}

func (s *Server) syncPushersToQCVM() {
	if s == nil || s.QCVM == nil {
		return
	}
	limit := s.NumEdicts
	if limit > len(s.Edicts) {
		limit = len(s.Edicts)
	}
	for entNum := 0; entNum < limit; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if MoveType(ent.Vars.MoveType) != MoveTypePush {
			continue
		}
		syncEdictToQCVM(s.QCVM, entNum, ent)
	}
}

type pusherSnapshot struct {
	entNum int
	vars   EntVars
}

func (s *Server) capturePusherSnapshots() []pusherSnapshot {
	if s == nil || s.QCVM == nil {
		return nil
	}
	limit := s.NumEdicts
	if limit > len(s.Edicts) {
		limit = len(s.Edicts)
	}
	snapshots := make([]pusherSnapshot, 0, limit)
	for entNum := 0; entNum < limit; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if MoveType(ent.Vars.MoveType) != MoveTypePush {
			continue
		}
		snapshots = append(snapshots, pusherSnapshot{
			entNum: entNum,
			vars:   *ent.Vars,
		})
	}
	return snapshots
}

func (s *Server) syncPushersFromQCVM() {
	if s == nil || s.QCVM == nil {
		return
	}
	limit := s.NumEdicts
	if limit > len(s.Edicts) {
		limit = len(s.Edicts)
	}
	for entNum := 0; entNum < limit; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if MoveType(ent.Vars.MoveType) != MoveTypePush {
			continue
		}
		oldOrigin := ent.Vars.Origin
		oldSolid := ent.Vars.Solid
		oldMins := ent.Vars.Mins
		oldMaxs := ent.Vars.Maxs
		oldModel := ent.Vars.Model
		oldModelIndex := ent.Vars.ModelIndex
		syncEdictFromQCVM(s.QCVM, entNum, ent)
		if ent.Vars.Origin != oldOrigin || ent.Vars.Solid != oldSolid || ent.Vars.Mins != oldMins || ent.Vars.Maxs != oldMaxs || ent.Vars.Model != oldModel || ent.Vars.ModelIndex != oldModelIndex {
			s.LinkEdict(ent, false)
		}
	}
}

func (s *Server) syncMutatedPushersFromQCVM(snapshots []pusherSnapshot) {
	if s == nil || s.QCVM == nil {
		return
	}
	for _, snapshot := range snapshots {
		if snapshot.entNum < 0 || snapshot.entNum >= s.NumEdicts || snapshot.entNum >= len(s.Edicts) {
			continue
		}
		ent := s.Edicts[snapshot.entNum]
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		scratch := Edict{Vars: &EntVars{}}
		syncEdictFromQCVM(s.QCVM, snapshot.entNum, &scratch)
		if *scratch.Vars == snapshot.vars {
			continue
		}
		oldOrigin := ent.Vars.Origin
		oldSolid := ent.Vars.Solid
		oldMins := ent.Vars.Mins
		oldMaxs := ent.Vars.Maxs
		oldModel := ent.Vars.Model
		oldModelIndex := ent.Vars.ModelIndex
		*ent.Vars = *scratch.Vars
		if ent.Vars.Origin != oldOrigin || ent.Vars.Solid != oldSolid || ent.Vars.Mins != oldMins || ent.Vars.Maxs != oldMaxs || ent.Vars.Model != oldModel || ent.Vars.ModelIndex != oldModelIndex {
			s.LinkEdict(ent, false)
		}
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
	maxClients := s.GetMaxClients()
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
		if client.Edict.Vars.Health <= 0 {
			i++
			continue
		}
		if uint32(client.Edict.Vars.Flags)&FlagNoTarget != 0 {
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
	if client == nil || client.Edict == nil || client.Edict.Free || client.Edict.Vars.Health <= 0 {
		return 0
	}
	if s.WorldTree != nil && len(s.WorldTree.Nodes) > 0 {
		org := client.Edict.Vars.Origin
		org = [3]float32{org[0] + client.Edict.Vars.ViewOfs[0], org[1] + client.Edict.Vars.ViewOfs[1], org[2] + client.Edict.Vars.ViewOfs[2]}
		if leaf := s.WorldTree.PointInLeaf(org); leaf != nil {
			s.checkClientPVS = append(s.checkClientPVS[:0], s.WorldTree.LeafPVS(leaf)...)
		}
	}
	return s.NumForEdict(client.Edict)
}

// qcFieldOffsets builds a normalized field-name → VM offset table for entvars.
// QuakeC field layouts are data-driven from progs.dat; this lookup lets reflection
// code map Go struct field names onto runtime VM offsets safely.
func buildQCFieldOffsets(vm *qc.VM) map[string]int {
	offsets := make(map[string]int, len(defaultEntFieldOffsets)+len(vm.FieldDefs))
	for key, ofs := range defaultEntFieldOffsets {
		offsets[key] = ofs
	}
	for _, def := range vm.FieldDefs {
		name := vm.GetString(def.Name)
		if name == "" {
			continue
		}
		offsets[normalizeFieldName(name)] = int(def.Ofs)
	}
	return offsets
}

func qcSyncCacheForVM(vm *qc.VM) *qcSyncCache {
	if vm == nil {
		return nil
	}
	if cached, ok := qcSyncCaches.Load(vm); ok {
		return cached.(*qcSyncCache)
	}

	fieldOffsets := buildQCFieldOffsets(vm)
	rt := reflect.TypeOf(EntVars{})
	bindings := make([]entFieldBinding, 0, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		ofs, ok := fieldOffsets[normalizeFieldName(field.Name)]
		if !ok {
			continue
		}
		switch field.Type.Kind() {
		case reflect.Float32:
			bindings = append(bindings, entFieldBinding{fieldIndex: i, ofs: ofs, kind: entFieldFloat32})
		case reflect.Int32:
			bindings = append(bindings, entFieldBinding{fieldIndex: i, ofs: ofs, kind: entFieldInt32})
		case reflect.Array:
			if field.Type.Len() == 3 && field.Type.Elem().Kind() == reflect.Float32 {
				bindings = append(bindings, entFieldBinding{fieldIndex: i, ofs: ofs, kind: entFieldVec3})
			}
		}
	}

	cache := &qcSyncCache{
		fieldOffsets:   fieldOffsets,
		entVarBindings: bindings,
		modelIndexOfs:  -1,
	}
	if ofs, ok := fieldOffsets[normalizeFieldName("ModelIndex")]; ok {
		cache.modelIndexOfs = ofs
	}
	if existing, loaded := qcSyncCaches.LoadOrStore(vm, cache); loaded {
		return existing.(*qcSyncCache)
	}
	return cache
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

// syncEntVarsToQC reflects over EntVars and writes each mapped field into VM edict memory.
// This generic reflection pass avoids hand-writing dozens of assignments and keeps
// the Go-side EntVars schema synchronized with QuakeC-visible entity fields.
func syncEntVarsToQC(vm *qc.VM, entNum int, vars *EntVars, bindings []entFieldBinding) {
	if vm == nil || vars == nil {
		return
	}
	rv := reflect.ValueOf(vars).Elem()
	for _, binding := range bindings {
		value := rv.Field(binding.fieldIndex)
		switch binding.kind {
		case entFieldFloat32:
			vm.SetEFloat(entNum, binding.ofs, float32(value.Float()))
		case entFieldInt32:
			vm.SetEInt(entNum, binding.ofs, int32(value.Int()))
		case entFieldVec3:
			vm.SetEVector(entNum, binding.ofs, [3]float32{
				float32(value.Index(0).Float()),
				float32(value.Index(1).Float()),
				float32(value.Index(2).Float()),
			})
		}
	}
}

// syncEntVarsFromQC reflects over EntVars and imports values from VM edict memory.
// It is the inverse of syncEntVarsToQC and keeps engine systems (physics, networking,
// savegames) in lockstep with whatever game DLL logic changed in QuakeC this frame.
func syncEntVarsFromQC(vm *qc.VM, entNum int, vars *EntVars, bindings []entFieldBinding) {
	if vm == nil || vars == nil {
		return
	}
	rv := reflect.ValueOf(vars).Elem()
	for _, binding := range bindings {
		value := rv.Field(binding.fieldIndex)
		switch binding.kind {
		case entFieldFloat32:
			value.SetFloat(float64(vm.EFloat(entNum, binding.ofs)))
		case entFieldInt32:
			value.SetInt(int64(vm.EInt(entNum, binding.ofs)))
		case entFieldVec3:
			vec := vm.EVector(entNum, binding.ofs)
			value.Index(0).SetFloat(float64(vec[0]))
			value.Index(1).SetFloat(float64(vec[1]))
			value.Index(2).SetFloat(float64(vec[2]))
		}
	}
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
		s.QCVM.Edicts = make([]byte, needed)
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
	skill := 1
	if skillCV := cvar.Get("skill"); skillCV != nil {
		skill = int(skillCV.Float + 0.5)
		if skill < 0 {
			skill = 0
		} else if skill > 3 {
			skill = 3
		}
		cvar.Set("skill", strconv.Itoa(skill))
	}
	s.ensureQCVMEdictStorage()
	s.QCVM.SetGlobal("world", 0)
	s.QCVM.SetGlobal("mapname", s.QCVM.AllocString(s.Name))
	s.QCVM.SetGlobal("skill", skill)
	s.QCVM.SetGlobal("time", s.Time)
	if s.Static != nil {
		s.QCVM.SetGlobal("serverflags", s.Static.ServerFlags)
	}

	// C Ironwail sets coop/deathmatch globals before ED_LoadFromFile so
	// QC spawn functions can branch on game mode.
	coopVal := cvar.FloatValue("coop")
	dmVal := cvar.FloatValue("deathmatch")
	if coopVal != 0 {
		s.QCVM.SetGlobal("coop", float32(coopVal))
	} else {
		s.QCVM.SetGlobal("deathmatch", float32(dmVal))
	}

	for entNum := 0; entNum < s.NumEdicts; entNum++ {
		syncEdictToQCVM(s.QCVM, entNum, s.EdictNum(entNum))
	}
}

func (s *Server) setQCTimeGlobal(time float32) {
	if s.QCVM == nil {
		return
	}
	s.QCVM.Time = float64(time)
	s.QCVM.SetGlobal("time", time)
}

// NewServer creates a new server instance.
