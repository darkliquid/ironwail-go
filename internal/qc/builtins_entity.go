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
	if serverBuiltinHooks.Spawn != nil {
		entNum, err := serverBuiltinHooks.Spawn(vm)
		if err != nil {
			vm.SetGFloat(OFSReturn, 0)
			return
		}
		vm.SetGFloat(OFSReturn, float32(entNum))
		return
	}

	if vm.NumEdicts == 0 {
		vm.NumEdicts = 1
	}
	if vm.MaxEdicts > 0 && vm.NumEdicts >= vm.MaxEdicts {
		vm.SetGFloat(OFSReturn, 0)
		return
	}

	entNum := vm.NumEdicts
	vm.NumEdicts++

	if data := vm.EdictData(entNum); data != nil {
		for i := range data {
			data[i] = 0
		}
	}

	vm.SetGFloat(OFSReturn, float32(entNum))
}

// traceline performs a line trace and stores the result in the trace globals.
//
// QuakeC signature: float(vector start, vector end, float nomonsters) traceline

// remove removes an entity from the game.
// The entity is deallocated and can be reused later.
//
// QuakeC signature: void(entity e) remove
func remove(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	if serverBuiltinHooks.Remove != nil {
		_ = serverBuiltinHooks.Remove(vm, entNum)
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
	startEnt := int(vm.GFloat(OFSParm0))
	fieldOfs := int(vm.GInt(OFSParm1))
	match := vm.GString(OFSParm2)

	if serverBuiltinHooks.Find != nil {
		vm.SetGFloat(OFSReturn, float32(serverBuiltinHooks.Find(vm, startEnt, fieldOfs, match)))
		return
	}

	for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
		if vm.GetString(vm.EString(entNum, fieldOfs)) == match {
			vm.SetGFloat(OFSReturn, float32(entNum))
			return
		}
	}

	vm.SetGFloat(OFSReturn, 0)
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
	startEnt := int(vm.GFloat(OFSParm0))
	fieldOfs := int(vm.GInt(OFSParm1))
	match := vm.GFloat(OFSParm2)

	if serverBuiltinHooks.FindFloat != nil {
		vm.SetGFloat(OFSReturn, float32(serverBuiltinHooks.FindFloat(vm, startEnt, fieldOfs, match)))
		return
	}

	for entNum := startEnt + 1; entNum < vm.NumEdicts; entNum++ {
		if vm.EFloat(entNum, fieldOfs) == match {
			vm.SetGFloat(OFSReturn, float32(entNum))
			return
		}
	}

	vm.SetGFloat(OFSReturn, 0)
}

// nextent returns the entity after the given one in the entity list.
//
// QuakeC signature: entity(entity e) nextent

// nextent returns the entity after the given one in the entity list.
//
// QuakeC signature: entity(entity e) nextent
func nextent(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	if serverBuiltinHooks.NextEnt != nil {
		vm.SetGFloat(OFSReturn, float32(serverBuiltinHooks.NextEnt(vm, entNum)))
		return
	}

	if entNum+1 > 0 && entNum+1 < vm.NumEdicts {
		vm.SetGFloat(OFSReturn, float32(entNum+1))
		return
	}

	vm.SetGFloat(OFSReturn, 0)
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

	if serverBuiltinHooks.FindRadius != nil {
		vm.SetGFloat(OFSReturn, float32(serverBuiltinHooks.FindRadius(vm, org, rad)))
		return
	}

	if rad < 0 {
		vm.SetGFloat(OFSReturn, 0)
		return
	}

	radSq := rad * rad
	for entNum := 1; entNum < vm.NumEdicts; entNum++ {
		entOrg := vm.EVector(entNum, EntFieldOrigin)
		dx := entOrg[0] - org[0]
		dy := entOrg[1] - org[1]
		dz := entOrg[2] - org[2]
		if dx*dx+dy*dy+dz*dz <= radSq {
			vm.SetGFloat(OFSReturn, float32(entNum))
			return
		}
	}

	vm.SetGFloat(OFSReturn, 0)
}

// precacheSound records a sound resource for later lookup.

// setorigin sets an entity's position directly.
// This bypasses physics - use with caution (teleports only).
//
// QuakeC signature: void(entity e, vector org) setorigin
func setorigin(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	org := vm.GVector(OFSParm1)
	if serverBuiltinHooks.SetOrigin != nil {
		serverBuiltinHooks.SetOrigin(vm, entNum, org)
		return
	}

	vm.SetEVector(entNum, EntFieldOrigin, org)

	mins := vm.EVector(entNum, EntFieldMins)
	maxs := vm.EVector(entNum, EntFieldMaxs)
	vm.SetEVector(entNum, EntFieldAbsMin, [3]float32{org[0] + mins[0], org[1] + mins[1], org[2] + mins[2]})
	vm.SetEVector(entNum, EntFieldAbsMax, [3]float32{org[0] + maxs[0], org[1] + maxs[1], org[2] + maxs[2]})
}

// setsize sets an entity's bounding box.
//
// QuakeC signature: void(entity e, vector min, vector max) setsize

// setsize sets an entity's bounding box.
//
// QuakeC signature: void(entity e, vector min, vector max) setsize
func setsize(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	mins := vm.GVector(OFSParm1)
	maxs := vm.GVector(OFSParm2)
	if serverBuiltinHooks.SetSize != nil {
		serverBuiltinHooks.SetSize(vm, entNum, mins, maxs)
		return
	}

	vm.SetEVector(entNum, EntFieldMins, mins)
	vm.SetEVector(entNum, EntFieldMaxs, maxs)

	size := [3]float32{
		maxs[0] - mins[0],
		maxs[1] - mins[1],
		maxs[2] - mins[2],
	}
	vm.SetEVector(entNum, EntFieldSize, size)

	origin := vm.EVector(entNum, EntFieldOrigin)
	vm.SetEVector(entNum, EntFieldAbsMin, [3]float32{origin[0] + mins[0], origin[1] + mins[1], origin[2] + mins[2]})
	vm.SetEVector(entNum, EntFieldAbsMax, [3]float32{origin[0] + maxs[0], origin[1] + maxs[1], origin[2] + maxs[2]})
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
	entNum := int(vm.GFloat(OFSParm0))
	modelName := vm.GString(OFSParm1)
	if serverBuiltinHooks.SetModel != nil {
		serverBuiltinHooks.SetModel(vm, entNum, modelName)
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
	entNum := int(vm.GFloat(OFSParm0))
	if serverBuiltinHooks.MakeStatic != nil {
		serverBuiltinHooks.MakeStatic(vm, entNum)
	}
	vm.SetGFloat(OFSReturn, 0)
}

func setspawnparms(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	if serverBuiltinHooks.SetSpawnParms != nil {
		serverBuiltinHooks.SetSpawnParms(vm, entNum)
	}
}
