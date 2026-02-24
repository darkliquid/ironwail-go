// Package qc provides QuakeC built-in functions.
//
// This file implements the built-in functions that QuakeC code can call.
package qc

import (
	"fmt"
	"math"
	"math/rand"
)

// RegisterBuiltins registers all QuakeC built-in functions with the VM.
func RegisterBuiltins(vm *VM) {
	// Builtin numbers match standard Quake numbering

	// Group 1-10: Vector/Math Operations
	vm.Builtins[1] = makevectors
	vm.Builtins[2] = vectoangles
	vm.Builtins[3] = vectoyaw
	vm.Builtins[4] = vlen
	vm.Builtins[5] = normalize
	vm.Builtins[6] = random

	// Group 11-20: Entity Management
	vm.Builtins[14] = spawn
	vm.Builtins[15] = remove
	vm.Builtins[16] = find
	vm.Builtins[17] = findfloat
	vm.Builtins[18] = nextent
	vm.Builtins[19] = findradius

	// Group 21-40: Physics/Movement
	vm.Builtins[22] = walkmove
	vm.Builtins[34] = droptofloor
	vm.Builtins[2] = setorigin
	vm.Builtins[4] = setsize
	vm.Builtins[3] = setmodel
	vm.Builtins[6] = movetogoal
	vm.Builtins[13] = changeyaw

	vm.NumBuiltins = 40
}

// ============================================================================
// Vector/Math Builtins (1-10)
// ============================================================================

// makevectors writes new values for v_forward, v_up, and v_right
// based on the entity's angles. This is used for vector math
// and directional operations in QuakeC.
//
// QuakeC signature: void(vector ang) makevectors
func makevectors(vm *VM) {
	angles := vm.GVector(OFSParm0) // Get input angles

	// Calculate yaw (angles[1] in Quake, which is Y axis rotation)
	yaw := float32(angles[1]) * float32(math.Pi) / 180.0

	sinYaw := math.Sin(float64(yaw))
	cosYaw := math.Cos(float64(yaw))
	pitch := float32(angles[0]) * float32(math.Pi) / 180.0
	sinPitch := math.Sin(float64(pitch))
	cosPitch := math.Cos(float64(pitch))

	// v_forward = direction entity is facing
	forward := [3]float32{
		float32(cosYaw * cosPitch),
		float32(sinYaw * cosPitch),
		float32(-sinPitch),
	}

	// v_right = strafe direction (perpendicular to forward)
	right := [3]float32{
		float32(-sinYaw),
		float32(cosYaw),
		0,
	}

	// v_up = always points up in world space
	up := [3]float32{0, 0, 1}

	vm.SetGVector(OFSGlobalVForward, forward)
	vm.SetGVector(OFSGlobalVRight, right)
	vm.SetGVector(OFSGlobalVUp, up)

	// Return void (G_FLOAT(OFSReturn))
	vm.SetGFloat(OFSReturn, 0)
}

// vectoangles converts a direction vector to Euler angles.
// This is the inverse of makevectors.
//
// QuakeC signature: vector(vector dir) vectoangles
func vectoangles(vm *VM) {
	dir := vm.GVector(OFSParm0)

	// Calculate yaw from forward direction
	yaw := math.Atan2(float64(dir[0]), float64(dir[1])) * 180.0 / math.Pi

	// Calculate pitch from up component and forward z
	forwardLen := math.Sqrt(float64(dir[0]*dir[0] + dir[1]*dir[1]))
	pitch := math.Atan2(float64(dir[2]), forwardLen) * 180.0 / math.Pi

	// Roll is always 0
	angles := [3]float32{
		-float32(pitch),
		float32(yaw),
		0,
	}

	vm.SetGVector(OFSReturn, angles)
}

// vectoyaw returns the yaw angle (Y-axis rotation) from a vector.
//
// QuakeC signature: float(vector vec) vectoyaw
func vectoyaw(vm *VM) {
	vec := vm.GVector(OFSParm0)
	yaw := math.Atan2(float64(vec[0]), float64(vec[1])) * 180.0 / math.Pi
	vm.SetGFloat(OFSReturn, float32(yaw))
}

// vlen returns the length (magnitude) of a vector.
//
// QuakeC signature: float(vector vec) vlen
func vlen(vm *VM) {
	vec := vm.GVector(OFSParm0)
	length := vm.VectorLength(vec)
	vm.SetGFloat(OFSReturn, length)
}

// normalize normalizes a vector to unit length and returns the original length.
//
// QuakeC signature: float(vector vec) normalize
func normalize(vm *VM) {
	vec := vm.GVector(OFSParm0)
	length := vm.VectorLength(vec)

	if length == 0 {
		vm.SetGVector(OFSReturn, [3]float32{0, 0, 0})
		vm.SetGFloat(OFSReturn, 0)
		return
	}

	// Normalize and return original length
	normalized := vm.VectorNormalize(vec)
	vm.SetGVector(OFSReturn, normalized)
	vm.SetGFloat(OFSReturn, length)
}

// random returns a random float in the range [0, 1].
//
// QuakeC signature: float() random
func random(vm *VM) {
	vm.SetGFloat(OFSReturn, rand.Float32())
}

// ============================================================================
// Entity Management Builtins (11-20)
// ============================================================================

// spawn creates a new entity and returns its index.
// The entity is initialized but not placed in the world yet.
//
// QuakeC signature: entity() spawn
func spawn(vm *VM) {
	// TODO: Call server's entity allocator ED_Alloc()
	// This requires access to the server's EntityManager
	fmt.Printf("TODO: spawn - allocate new entity\n")

	// For now, return 0 (should never be entity 0)
	vm.SetGFloat(OFSReturn, 0)
}

// remove removes an entity from the game.
// The entity is deallocated and can be reused later.
//
// QuakeC signature: void(entity e) remove
func remove(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))

	// TODO: Call server's ED_Free(entNum)
	// This requires access to the server's EntityManager
	fmt.Printf("TODO: remove - free entity %d\n", entNum)
}

// find searches for an entity with a matching field value.
// It starts searching from the entity after 'self'.
//
// QuakeC signature: entity(entity start, .string fld, string match) find
func find(vm *VM) {
	// TODO: Implement entity search
	// Requires iterating through entities and checking field values
	fmt.Printf("TODO: find - search entities\n")
	vm.SetGFloat(OFSReturn, 0)
}

// findfloat searches for an entity with a matching float field value.
// Similar to find but for float fields.
//
// QuakeC signature: entity(entity start, .float fld, float match) findfloat
func findfloat(vm *VM) {
	// TODO: Implement entity search for float fields
	fmt.Printf("TODO: findfloat - search entities by float\n")
	vm.SetGFloat(OFSReturn, 0)
}

// nextent returns the entity after the given one in the entity list.
//
// QuakeC signature: entity(entity e) nextent
func nextent(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))

	// TODO: Return next entity in the array
	// Requires iterating from entNum+1 to max_edicts
	fmt.Printf("TODO: nextent - get next entity after %d\n", entNum)
	vm.SetGFloat(OFSReturn, 0)
}

// findradius finds entities within a certain radius.
// This is used for area of effect queries.
//
// QuakeC signature: entity(vector org, float rad) findradius
func findradius(vm *VM) {
	// TODO: Find entities within radius of origin
	// Requires spatial partitioning or linear search
	fmt.Printf("TODO: findradius - search entities by radius\n")
	vm.SetGFloat(OFSReturn, 0)
}

// ============================================================================
// Physics/Movement Builtins (21-40)
// ============================================================================

// walkmove moves an entity forward with collision detection.
// Used for player and monster movement.
//
// QuakeC signature: float(float yaw, float dist) walkmove
func walkmove(vm *VM) {
	yaw := vm.GFloat(OFSParm0)
	dist := vm.GFloat(OFSParm1)

	// TODO: Implement walk movement with collision
	// This requires physics system integration
	fmt.Printf("TODO: walkmove yaw=%f dist=%f\n", yaw, dist)

	// Return result: 1 = success, 0 = blocked
	vm.SetGFloat(OFSReturn, 1)
}

// droptofloor drops an entity down until it hits the floor.
// Used for spawning items that fall to the ground.
//
// QuakeC signature: float() droptofloor
func droptofloor(vm *VM) {
	// TODO: Implement floor detection
	// Requires BSP collision detection
	fmt.Printf("TODO: droptofloor\n")

	vm.SetGFloat(OFSReturn, 0)
}

// setorigin sets an entity's position directly.
// This bypasses physics - use with caution (teleports only).
//
// QuakeC signature: void(entity e, vector org) setorigin
func setorigin(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	org := vm.GVector(OFSParm1)

	// Set entity origin
	vm.SetEVector(entNum, EntFieldOrigin, org)

	// TODO: Relink entity in world BSP tree
	fmt.Printf("TODO: setorigin - relink entity %d\n", entNum)
}

// setsize sets an entity's bounding box.
//
// QuakeC signature: void(entity e, vector min, vector max) setsize
func setsize(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	mins := vm.GVector(OFSParm1)
	maxs := vm.GVector(OFSParm2)

	// Set entity mins/maxs
	vm.SetEVector(entNum, EntFieldMins, mins)
	vm.SetEVector(entNum, EntFieldMaxs, maxs)

	// Calculate size
	size := [3]float32{
		maxs[0] - mins[0],
		maxs[1] - mins[1],
		maxs[2] - mins[2],
	}
	vm.SetEVector(entNum, EntFieldSize, size)

	// TODO: Relink entity in world BSP tree
	fmt.Printf("TODO: setsize - relink entity %d\n", entNum)
}

// setmodel sets the model for an entity.
// Also sets the model index.
//
// QuakeC signature: void(entity e, string model) setmodel
func setmodel(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))
	modelName := vm.GString(OFSParm1)

	// TODO: Precache model and set model index
	fmt.Printf("TODO: setmodel - set model to %s\n", modelName)

	vm.SetEInt(entNum, EntFieldModel, vm.AllocString(modelName))
}

// movetogoal moves an entity towards its goal.
// Used for AI navigation.
//
// QuakeC signature: void(entity ent) movetogoal
func movetogoal(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))

	// TODO: Implement goal-directed movement
	// Requires pathfinding or simple "move toward" logic
	fmt.Printf("TODO: movetogoal - entity %d\n", entNum)
}

// changeyaw smoothly rotates an entity toward its ideal yaw.
// Used for AI turning animation.
//
// QuakeC signature: void(entity ent) changeyaw
func changeyaw(vm *VM) {
	entNum := int(vm.GFloat(OFSParm0))

	// TODO: Implement yaw smoothing
	fmt.Printf("TODO: changeyaw - entity %d\n", entNum)
}
