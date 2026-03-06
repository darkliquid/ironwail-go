// Package qc provides bytecode execution for the QuakeC Virtual Machine.
//
// This file implements the main execution loop for QuakeC bytecode.
// The interpreter processes bytecode statements sequentially, dispatching
// to opcode handlers for each instruction type.
//
// # Execution Model
//
// QuakeC bytecode is a stack-based virtual machine with the following characteristics:
//
//   - All values are stored as float32 in a flat globals array
//   - Entity fields are stored separately in the Edicts byte array
//   - Functions can call other functions or built-ins (Go functions)
//   - Control flow uses conditional/unconditional jumps
//   - Up to 8 parameters can be passed to functions
//
// # Opcode Categories
//
//   - Arithmetic: OPAddF, OPSubF, OPMulF, OPDivF, etc.
//   - Comparison: OPEqF, OPNeF, OPLE, OPLT, OPGE, OPGT
//   - Logic: OPAnd, OPOr, OPNotF, OPBitAnd, OPBitOr
//   - Control flow: OPIF, OPIFNot, OPGoto, OPReturn, OPDone
//   - Function calls: OPCall0-OPCall8
//   - Memory: OPLoad*, OPStore*, OPAddress
//   - Entity state: OPState
package qc

import (
	"fmt"
	"math"
)

// ExecuteProgram runs a QuakeC function by its number.
//
// If the function is a built-in (negative FirstStatement), the corresponding
// Go function is called directly. Otherwise, the bytecode interpreter
// executes statements until OPDone or OPReturn unwinds the stack completely.
//
// Parameters are passed via the OFSParm* globals before calling.
// Return values appear in OFSReturn after execution.
//
// Returns an error for:
//   - Invalid function number
//   - Unknown built-in
//   - Stack overflow/underflow
//   - Unknown opcode
//   - Division by zero
func (vm *VM) ExecuteProgram(fnum int) error {
	if fnum < 0 || fnum >= len(vm.Functions) {
		return fmt.Errorf("invalid function number: %d", fnum)
	}

	f := &vm.Functions[fnum]
	if f.FirstStatement < 0 {
		builtin := int(-f.FirstStatement)
		if builtin >= len(vm.Builtins) || vm.Builtins[builtin] == nil {
			return fmt.Errorf("unknown builtin function: %d", builtin)
		}
		vm.Builtins[builtin](vm)
		return nil
	}

	if err := vm.EnterFunction(f); err != nil {
		return err
	}

	vm.XStatement = int(f.FirstStatement)

	for {
		if vm.XStatement < 0 || vm.XStatement >= len(vm.Statements) {
			return fmt.Errorf("statement out of bounds: %d", vm.XStatement)
		}

		st := &vm.Statements[vm.XStatement]
		op := Opcode(st.Op)

		switch op {
		case OPDone, OPReturn:
			if err := vm.LeaveFunction(); err != nil {
				return err
			}
			if vm.Depth == 0 {
				return nil
			}
			vm.XStatement++
			continue

		case OPCall0, OPCall1, OPCall2, OPCall3, OPCall4, OPCall5, OPCall6, OPCall7, OPCall8:
			argc := int(op - OPCall0)
			if err := vm.callFunction(int(vm.GFunction(int(st.A))), argc); err != nil {
				return err
			}

		case OPIF:
			if vm.GFloat(int(st.A)) != 0 {
				vm.XStatement += int(int16(st.B))
				continue
			}

		case OPIFNot:
			if vm.GFloat(int(st.A)) == 0 {
				vm.XStatement += int(int16(st.B))
				continue
			}

		case OPGoto:
			vm.XStatement += int(int16(st.A))
			continue

		case OPMulF:
			vm.SetGFloat(int(st.C), vm.GFloat(int(st.A))*vm.GFloat(int(st.B)))

		case OPMulV:
			a := vm.GVector(int(st.A))
			b := vm.GVector(int(st.B))
			vm.SetGFloat(int(st.C), a[0]*b[0]+a[1]*b[1]+a[2]*b[2])

		case OPMulFV:
			fv := vm.GFloat(int(st.A))
			v := vm.GVector(int(st.B))
			vm.SetGVector(int(st.C), [3]float32{fv * v[0], fv * v[1], fv * v[2]})

		case OPMulVF:
			v := vm.GVector(int(st.A))
			fv := vm.GFloat(int(st.B))
			vm.SetGVector(int(st.C), [3]float32{v[0] * fv, v[1] * fv, v[2] * fv})

		case OPDivF:
			b := vm.GFloat(int(st.B))
			if b == 0 {
				return fmt.Errorf("division by zero")
			}
			vm.SetGFloat(int(st.C), vm.GFloat(int(st.A))/b)

		case OPAddF:
			vm.SetGFloat(int(st.C), vm.GFloat(int(st.A))+vm.GFloat(int(st.B)))

		case OPAddV:
			a := vm.GVector(int(st.A))
			b := vm.GVector(int(st.B))
			vm.SetGVector(int(st.C), [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]})

		case OPSubF:
			vm.SetGFloat(int(st.C), vm.GFloat(int(st.A))-vm.GFloat(int(st.B)))

		case OPSubV:
			a := vm.GVector(int(st.A))
			b := vm.GVector(int(st.B))
			vm.SetGVector(int(st.C), [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]})

		case OPEqF:
			if vm.GFloat(int(st.A)) == vm.GFloat(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPEqV:
			a := vm.GVector(int(st.A))
			b := vm.GVector(int(st.B))
			if a == b {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPEqS:
			if vm.GString(int(st.A)) == vm.GString(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPEqE, OPEqFNC:
			if vm.GInt(int(st.A)) == vm.GInt(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNeF:
			if vm.GFloat(int(st.A)) != vm.GFloat(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNeV:
			a := vm.GVector(int(st.A))
			b := vm.GVector(int(st.B))
			if a != b {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNeS:
			if vm.GString(int(st.A)) != vm.GString(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNeE, OPNeFNC:
			if vm.GInt(int(st.A)) != vm.GInt(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPLE:
			if vm.GFloat(int(st.A)) <= vm.GFloat(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPGE:
			if vm.GFloat(int(st.A)) >= vm.GFloat(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPLT:
			if vm.GFloat(int(st.A)) < vm.GFloat(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPGT:
			if vm.GFloat(int(st.A)) > vm.GFloat(int(st.B)) {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNotF:
			if vm.GFloat(int(st.A)) == 0 {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNotV:
			v := vm.GVector(int(st.A))
			if v[0] == 0 && v[1] == 0 && v[2] == 0 {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNotS:
			s := vm.GString(int(st.A))
			if len(s) == 0 {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPNotEnt, OPNotFNC:
			if vm.GInt(int(st.A)) == 0 {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPAnd:
			if vm.GFloat(int(st.A)) != 0 && vm.GFloat(int(st.B)) != 0 {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPOr:
			if vm.GFloat(int(st.A)) != 0 || vm.GFloat(int(st.B)) != 0 {
				vm.SetGFloat(int(st.C), 1)
			} else {
				vm.SetGFloat(int(st.C), 0)
			}

		case OPBitAnd:
			vm.SetGFloat(int(st.C), float32(int(vm.GFloat(int(st.A)))&int(vm.GFloat(int(st.B)))))

		case OPBitOr:
			vm.SetGFloat(int(st.C), float32(int(vm.GFloat(int(st.A)))|int(vm.GFloat(int(st.B)))))

		case OPStoreF:
			vm.SetGFloat(int(st.B), vm.GFloat(int(st.A)))

		case OPStoreV:
			v := vm.GVector(int(st.A))
			vm.SetGVector(int(st.B), v)

		case OPStoreS:
			vm.SetGInt(int(st.B), vm.GInt(int(st.A)))

		case OPStoreEnt, OPStoreFld, OPStoreFNC:
			vm.SetGInt(int(st.B), vm.GInt(int(st.A)))

		case OPAddress:
			edictNum := int(vm.GInt(int(st.A)))
			maxEdicts := 0
			if vm.EdictSize > 0 {
				maxEdicts = len(vm.Edicts) / vm.EdictSize
			}
			if edictNum < 0 || edictNum >= maxEdicts {
				return fmt.Errorf("OPAddress invalid edict: %d", edictNum)
			}
			if edictNum == 0 {
				return fmt.Errorf("OPAddress assignment to world entity")
			}
			ptr := edictNum*vm.EdictSize + 28 + int(st.B)*4
			if ptr < 0 || ptr+4 > len(vm.Edicts) {
				return fmt.Errorf("OPAddress pointer out of bounds: %d", ptr)
			}
			vm.SetGInt(int(st.C), int32(ptr))

		case OPLoadF:
			edictNum := int(vm.GInt(int(st.A)))
			vm.SetGFloat(int(st.C), vm.EFloat(edictNum, int(st.B)))

		case OPLoadV:
			edictNum := int(vm.GInt(int(st.A)))
			vm.SetGVector(int(st.C), vm.EVector(edictNum, int(st.B)))

		case OPLoadS, OPLoadEnt, OPLoadFld, OPLoadFNC:
			edictNum := int(vm.GInt(int(st.A)))
			vm.SetGInt(int(st.C), vm.EInt(edictNum, int(st.B)))

		case OPStorePF:
			ptr := int(vm.GInt(int(st.B)))
			edictNum, fieldOfs, ok := vm.pointerToField(ptr)
			if !ok {
				rel := -1
				calcEdict := -1
				if vm.EdictSize > 0 {
					calcEdict = ptr / vm.EdictSize
					rel = ptr - calcEdict*vm.EdictSize - 28
				}
				return fmt.Errorf("OPStorePF pointer out of bounds: ptr=%d edict=%d rel=%d edictSize=%d entityFields=%d edictsLen=%d", ptr, calcEdict, rel, vm.EdictSize, vm.EntityFields, len(vm.Edicts))
			}
			vm.SetEFloat(edictNum, fieldOfs, vm.GFloat(int(st.A)))

		case OPStorePV:
			ptr := int(vm.GInt(int(st.B)))
			edictNum, fieldOfs, ok := vm.pointerToField(ptr)
			if !ok {
				return fmt.Errorf("OPStorePV pointer out of bounds: %d", ptr)
			}
			vm.SetEVector(edictNum, fieldOfs, vm.GVector(int(st.A)))

		case OPStorePS, OPStorePEnt, OPStorePFld, OPStorePFNC:
			ptr := int(vm.GInt(int(st.B)))
			edictNum, fieldOfs, ok := vm.pointerToField(ptr)
			if !ok {
				return fmt.Errorf("OPStoreP pointer out of bounds: %d", ptr)
			}
			vm.SetEInt(edictNum, fieldOfs, vm.GInt(int(st.A)))

		case OPState:
			// OPState sets entity animation frame and think function.
			// This is commonly used in QuakeC for monster AI state machines.
			// Equivalent to:
			//   self.frame = st.A
			//   self.nextthink = time + 0.1
			//   self.think = st.B
			selfEdict := int(vm.GInt(OFSSelf)) // self entity number
			frame := vm.GFloat(int(st.A))
			thinkFunc := vm.GInt(int(st.B))

			// Set entity fields using the defined constants
			vm.SetEFloat(selfEdict, EntFieldFrame, frame)
			vm.SetEFunction(selfEdict, EntFieldThink, thinkFunc)
			vm.SetEFloat(selfEdict, EntFieldNextThink, float32(vm.Time)+0.1)

		default:
			return fmt.Errorf("unknown opcode: %d", op)
		}

		vm.XStatement++
	}
}

func (vm *VM) callFunction(fnum int, argc int) error {
	if fnum < 0 {
		builtin := -fnum
		if builtin >= len(vm.Builtins) || vm.Builtins[builtin] == nil {
			return fmt.Errorf("unknown builtin: %d", builtin)
		}
		vm.ArgC = argc
		vm.Builtins[builtin](vm)
		return nil
	}
	if fnum >= len(vm.Functions) {
		return fmt.Errorf("invalid function number: %d", fnum)
	}

	f := &vm.Functions[fnum]
	if f.FirstStatement < 0 {
		builtin := int(-f.FirstStatement)
		if builtin >= len(vm.Builtins) || vm.Builtins[builtin] == nil {
			return fmt.Errorf("unknown builtin: %d", builtin)
		}
		vm.ArgC = argc
		vm.Builtins[builtin](vm)
		return nil
	}

	if err := vm.EnterFunction(f); err != nil {
		return err
	}

	vm.XStatement = int(f.FirstStatement)
	return nil
}

func (vm *VM) VectorLength(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

func (vm *VM) VectorNormalize(v [3]float32) [3]float32 {
	l := vm.VectorLength(v)
	if l == 0 {
		return [3]float32{0, 0, 0}
	}
	return [3]float32{v[0] / l, v[1] / l, v[2] / l}
}

func (vm *VM) VectorScale(v [3]float32, scale float32) [3]float32 {
	return [3]float32{v[0] * scale, v[1] * scale, v[2] * scale}
}

func (vm *VM) pointerToField(ptr int) (edictNum int, fieldOfs int, ok bool) {
	if ptr < 28 || ptr >= len(vm.Edicts) || vm.EdictSize <= 0 {
		return 0, 0, false
	}
	edictNum = ptr / vm.EdictSize
	maxEdicts := len(vm.Edicts) / vm.EdictSize
	if edictNum < 0 || edictNum >= maxEdicts {
		return 0, 0, false
	}
	rel := ptr - edictNum*vm.EdictSize - 28
	if rel < 0 || rel%4 != 0 {
		return 0, 0, false
	}
	fieldOfs = rel / 4
	if fieldOfs < 0 || fieldOfs >= vm.EntityFields {
		return 0, 0, false
	}
	return edictNum, fieldOfs, true
}
