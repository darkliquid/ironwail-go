// Package qc provides QuakeC built-in functions.
//
// This file implements world/physics QuakeC built-ins.
package qc

import (
	"github.com/darkliquid/ironwail-go/internal/console"
)

// traceline performs a line trace and stores the result in the trace globals.
//
// QuakeC signature: float(vector start, vector end, float nomonsters, entity passent) traceline
func traceline(vm *VM) {
	start := vm.GVector(OFSParm0)
	end := vm.GVector(OFSParm1)
	noMonsters := vm.GFloat(OFSParm2) != 0
	passEnt := int(vm.GInt(OFSParm3))

	trace := BuiltinTraceResult{Fraction: 1, EndPos: end, EntNum: 0, InOpen: true}
	if serverBuiltinHooks.Traceline != nil {
		trace = serverBuiltinHooks.Traceline(vm, start, end, noMonsters, passEnt)
	}
	setTraceGlobals(vm, trace)
	vm.SetGFloat(OFSReturn, trace.Fraction)
}

func checkclient(vm *VM) {
	if serverBuiltinHooks.CheckClient != nil {
		vm.SetGInt(OFSReturn, int32(serverBuiltinHooks.CheckClient(vm)))
		return
	}
	vm.SetGInt(OFSReturn, 0)
}

func sound(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	channel := int(vm.GFloat(OFSParm1))
	sample := vm.GString(OFSParm2)
	volume := 255
	if len(vm.Globals) > OFSParm3 {
		if v := int(vm.GFloat(OFSParm3) * 255); v != 0 {
			volume = v
		}
	}
	attenuation := float32(1)
	if len(vm.Globals) > OFSParm4 {
		attenuation = vm.GFloat(OFSParm4)
	}
	if serverBuiltinHooks.Sound != nil {
		serverBuiltinHooks.Sound(vm, entNum, channel, sample, volume, attenuation)
	}
}

// remove removes an entity from the game.
// The entity is deallocated and can be reused later.
//
// QuakeC signature: void(entity e) remove

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
	if serverBuiltinHooks.WalkMove != nil {
		if serverBuiltinHooks.WalkMove(vm, yaw, dist) {
			vm.SetGFloat(OFSReturn, 1)
		} else {
			vm.SetGFloat(OFSReturn, 0)
		}
		return
	}

	vm.SetGFloat(OFSReturn, 0)
}

// checkbottom verifies that the entity is supported by the floor.

// checkbottom verifies that the entity is supported by the floor.
func checkbottom(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	if serverBuiltinHooks.CheckBottom != nil {
		if serverBuiltinHooks.CheckBottom(vm, entNum) {
			vm.SetGFloat(OFSReturn, 1)
		} else {
			vm.SetGFloat(OFSReturn, 0)
		}
		return
	}
	vm.SetGFloat(OFSReturn, 0)
}

// pointcontents reports BSP contents at a point.

// pointcontents reports BSP contents at a point.
func pointcontents(vm *VM) {
	point := vm.GVector(OFSParm0)
	if serverBuiltinHooks.PointContents != nil {
		vm.SetGFloat(OFSReturn, float32(serverBuiltinHooks.PointContents(vm, point)))
		return
	}
	vm.SetGFloat(OFSReturn, 0)
}

func aimBuiltin(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	speed := vm.GFloat(OFSParm1)
	if serverBuiltinHooks.Aim != nil {
		vm.SetGVector(OFSReturn, serverBuiltinHooks.Aim(vm, entNum, speed))
		return
	}
	vm.SetGVector(OFSReturn, vm.GVector(OFSGlobalVForward))
}

func lightstyle(vm *VM) {
	style := int(vm.GFloat(OFSParm0))
	value := vm.GString(OFSParm1)
	if serverBuiltinHooks.LightStyle != nil {
		serverBuiltinHooks.LightStyle(vm, style, value)
	}
}

func particle(vm *VM) {
	org := vm.GVector(OFSParm0)
	dir := vm.GVector(OFSParm1)
	color := int(vm.GFloat(OFSParm2))
	count := int(vm.GFloat(OFSParm3))
	if serverBuiltinHooks.Particle != nil {
		serverBuiltinHooks.Particle(vm, org, dir, color, count)
	}
}

// droptofloor drops an entity down until it hits the floor.
// Used for spawning items that fall to the ground.
//
// QuakeC signature: float() droptofloor

// droptofloor drops an entity down until it hits the floor.
// Used for spawning items that fall to the ground.
//
// QuakeC signature: float() droptofloor
func droptofloor(vm *VM) {
	if serverBuiltinHooks.DropToFloor != nil {
		if serverBuiltinHooks.DropToFloor(vm) {
			vm.SetGFloat(OFSReturn, 1)
		} else {
			vm.SetGFloat(OFSReturn, 0)
		}
		return
	}

	vm.SetGFloat(OFSReturn, 0)
}

// setorigin sets an entity's position directly.
// This bypasses physics - use with caution (teleports only).
//
// QuakeC signature: void(entity e, vector org) setorigin

// movetogoal moves an entity towards its goal.
// Used for AI navigation.
//
// QuakeC signature: void(float dist) movetogoal
func movetogoal(vm *VM) {
	dist := vm.GFloat(OFSParm0)
	if serverBuiltinHooks.MoveToGoal != nil {
		serverBuiltinHooks.MoveToGoal(vm, dist)
	}
}

func ambientsound(vm *VM) {
	org := vm.GVector(OFSParm0)
	sample := vm.GString(OFSParm1)
	volume := int(vm.GFloat(OFSParm2))
	attenuation := vm.GFloat(OFSParm3)
	if serverBuiltinHooks.AmbientSound != nil {
		serverBuiltinHooks.AmbientSound(vm, org, sample, volume, attenuation)
	}
	vm.SetGFloat(OFSReturn, 0)
}

// changeyaw smoothly rotates an entity toward its ideal yaw.
// Used for AI turning animation.
//
// QuakeC signature: void() changeyaw

// changeyaw smoothly rotates an entity toward its ideal yaw.
// Used for AI turning animation.
//
// QuakeC signature: void() changeyaw
func changeyaw(vm *VM) {
	if serverBuiltinHooks.ChangeYaw != nil {
		serverBuiltinHooks.ChangeYaw(vm)
	}
}

// centerprint prints a centered message, currently falling back to console output.

// centerprint prints a centered message, currently falling back to console output.
func centerprint(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	msg := localizedTextMessage(vm.GString(OFSParm1))
	if serverBuiltinHooks.CenterPrint != nil {
		serverBuiltinHooks.CenterPrint(vm, entNum, msg)
		return
	}
	console.CenterPrintf(40, "%s", msg)
}

func localsound(vm *VM) {
	entNum := int(vm.GInt(OFSParm0))
	sample := vm.GString(OFSParm1)
	if serverBuiltinHooks.LocalSound != nil {
		serverBuiltinHooks.LocalSound(vm, entNum, sample)
	}
}
