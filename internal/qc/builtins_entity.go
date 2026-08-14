// Package qc provides QuakeC built-in functions.
//
// This file implements entity-management QuakeC built-ins.
package qc

// ============================================================================
// Entity Management Builtins (11-20)
// ============================================================================

// spawn creates a new entity and returns its index.
// The entity is initialized but not placed in the world yet.
//
// QuakeC signature: entity() spawn
func spawn(vm *VM) {
	if vm.ServerHooks.Spawn != nil {
		entNum, err := vm.ServerHooks.Spawn(vm)
		if err != nil {
			vm.SetGInt(OFSReturn, 0)
			return
		}
		vm.SetGInt(OFSReturn, int32(entNum))
		return
	}

	if vm.NumEdicts == 0 {
		vm.NumEdicts = 1
	}
	if vm.MaxEdicts > 0 && vm.NumEdicts >= vm.MaxEdicts {
		vm.SetGInt(OFSReturn, 0)
		return
	}

	entNum := vm.NumEdicts
	vm.NumEdicts++

	if data := vm.EdictData(entNum); data != nil {
		for i := range data {
			data[i] = 0
		}
	}

	vm.SetGInt(OFSReturn, int32(entNum))
}

// traceline performs a line trace and stores the result in the trace globals.
//
// QuakeC signature: float(vector start, vector end, float nomonsters) traceline

// remove removes an entity from the game.
// The entity is deallocated and can be reused later.
//
// QuakeC signature: void(entity e) remove
func remove(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	if vm.ServerHooks.Remove != nil {
		_ = vm.ServerHooks.Remove(vm, entNum)
		return
	}

	if entNum <= 0 || entNum >= vm.NumEdicts {
		return
	}
	if data := vm.EdictData(entNum); data != nil {
		for i := range data {
			data[i] = 0
		}
	}
}

// find searches for an entity with a matching field value.
// It starts searching from the entity after 'self'.
//
// QuakeC signature: entity(entity start, .string fld, string match) find

// find searches for an entity with a matching field value.
// It starts searching from the entity after 'self'.
//
// QuakeC signature: entity(entity start, .string fld, string match) find
func find(vm *VM) {
	startEnt := int(vm.GInt(OFSParm0))
	fieldOfs := int(vm.GInt(OFSParm1))
	match := vm.GString(OFSParm2)

	if vm.ServerHooks.Find != nil {
		vm.SetGInt(OFSReturn, int32(vm.ServerHooks.Find(vm, startEnt, fieldOfs, match)))
		return
	}

	for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
		if vm.String(vm.EString(entNum, fieldOfs)) == match {
			vm.SetGInt(OFSReturn, int32(entNum))
			return
		}
	}

	vm.SetGInt(OFSReturn, 0)
}

// findfloat searches for an entity with a matching float field value.
// Similar to find but for float fields.
//
// QuakeC signature: entity(entity start, .float fld, float match) findfloat

// findfloat searches for an entity with a matching float field value.
// Similar to find but for float fields.
//
// QuakeC signature: entity(entity start, .float fld, float match) findfloat
func findfloat(vm *VM) {
	startEnt := int(vm.GInt(OFSParm0))
	fieldOfs := int(vm.GInt(OFSParm1))
	match := vm.GFloat(OFSParm2)

	if vm.ServerHooks.FindFloat != nil {
		vm.SetGInt(OFSReturn, int32(vm.ServerHooks.FindFloat(vm, startEnt, fieldOfs, match)))
		return
	}

	for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
		if vm.EFloat(entNum, fieldOfs) == match {
			vm.SetGInt(OFSReturn, int32(entNum))
			return
		}
	}

	vm.SetGInt(OFSReturn, 0)
}

// nextent returns the entity after the given one in the entity list.
//
// QuakeC signature: entity(entity e) nextent

// nextent returns the entity after the given one in the entity list.
//
// QuakeC signature: entity(entity e) nextent
func nextent(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	if vm.ServerHooks.NextEnt != nil {
		vm.SetGInt(OFSReturn, int32(vm.ServerHooks.NextEnt(vm, entNum)))
		return
	}

	if entNum+1 > 0 && entNum+1 < vm.NumEdicts {
		vm.SetGInt(OFSReturn, int32(entNum+1))
		return
	}

	vm.SetGInt(OFSReturn, 0)
}

// findradius finds entities within a certain radius.
// This is used for area of effect queries.
//
// QuakeC signature: entity(vector org, float rad) findradius

// findradius finds entities within a certain radius.
// This is used for area of effect queries.
//
// QuakeC signature: entity(vector org, float rad) findradius
func findradius(vm *VM) {
	org := vm.GVector(OFSParm0)
	rad := vm.GFloat(OFSParm1)

	if vm.ServerHooks.FindRadius != nil {
		vm.SetGInt(OFSReturn, int32(vm.ServerHooks.FindRadius(vm, org, rad)))
		return
	}

	if rad < 0 {
		vm.SetGInt(OFSReturn, 0)
		return
	}

	radSq := rad * rad
	for entNum := 1; entNum < vm.NumEdicts; entNum++ {
		entOrg := vm.EVector(entNum, EntFieldOrigin)
		if entOrg.DistanceSq(org) <= radSq {
			vm.SetGInt(OFSReturn, int32(entNum))
			return
		}
	}

	vm.SetGInt(OFSReturn, 0)
}

// precacheSound records a sound resource for later lookup.

// setorigin sets an entity's position directly.
// This bypasses physics - use with caution (teleports only).
//
// QuakeC signature: void(entity e, vector org) setorigin
func setorigin(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	org := vm.GVector(OFSParm1)
	if vm.ServerHooks.SetOrigin != nil {
		vm.ServerHooks.SetOrigin(vm, entNum, org)
		return
	}

	vm.SetEVector(entNum, EntFieldOrigin, org)

	mins := vm.EVector(entNum, EntFieldMins)
	maxs := vm.EVector(entNum, EntFieldMaxs)
	vm.SetEVector(entNum, EntFieldAbsMin, org.Add(mins))
	vm.SetEVector(entNum, EntFieldAbsMax, org.Add(maxs))
}

// setsize sets an entity's bounding box.
//
// QuakeC signature: void(entity e, vector min, vector max) setsize
func setsize(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	mins := vm.GVector(OFSParm1)
	maxs := vm.GVector(OFSParm2)
	if vm.ServerHooks.SetSize != nil {
		vm.ServerHooks.SetSize(vm, entNum, mins, maxs)
		return
	}

	vm.SetEVector(entNum, EntFieldMins, mins)
	vm.SetEVector(entNum, EntFieldMaxs, maxs)
	vm.SetEVector(entNum, EntFieldSize, maxs.Sub(mins))

	origin := vm.EVector(entNum, EntFieldOrigin)
	vm.SetEVector(entNum, EntFieldAbsMin, origin.Add(mins))
	vm.SetEVector(entNum, EntFieldAbsMax, origin.Add(maxs))
}

// setmodel sets the model for an entity.
// Also sets the model index.
//
// QuakeC signature: void(entity e, string model) setmodel

// setmodel sets the model for an entity.
// Also sets the model index.
//
// QuakeC signature: void(entity e, string model) setmodel
func setmodel(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	modelName := vm.GString(OFSParm1)
	if vm.ServerHooks.SetModel != nil {
		vm.ServerHooks.SetModel(vm, entNum, modelName)
		return
	}

	vm.SetEInt(entNum, EntFieldModel, vm.AllocString(modelName))
	if modelName != "" {
		vm.SetEFloat(entNum, EntFieldModelIndex, 1)
	} else {
		vm.SetEFloat(entNum, EntFieldModelIndex, 0)
	}
}

// movetogoal moves an entity towards its goal.
// Used for AI navigation.
//
// QuakeC signature: void(float dist) movetogoal

func makestatic(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	if vm.ServerHooks.MakeStatic != nil {
		vm.ServerHooks.MakeStatic(vm, entNum)
	}
	vm.SetGFloat(OFSReturn, 0)
}

func setspawnparms(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	if vm.ServerHooks.SetSpawnParms != nil {
		vm.ServerHooks.SetSpawnParms(vm, entNum)
	}
}
