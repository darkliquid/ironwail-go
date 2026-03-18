package qc

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

func (vm *VM) LoadProgs(r io.ReadSeeker) error {
	var header DProgs
	if err := binary.Read(r, binary.LittleEndian, &header); err != nil {
		return fmt.Errorf("failed to read progs header: %w", err)
	}

	if header.Version != ProgVersion {
		return fmt.Errorf("progs version mismatch: got %d, expected %d", header.Version, ProgVersion)
	}

	vm.Progs = &header
	vm.CRC = uint16(header.CRC)

	if _, err := r.Seek(int64(header.Statements), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to statements: %w", err)
	}
	vm.Statements = make([]DStatement, header.NumStatements)
	if err := binary.Read(r, binary.LittleEndian, &vm.Statements); err != nil {
		return fmt.Errorf("failed to read statements: %w", err)
	}

	if _, err := r.Seek(int64(header.GlobalDefs), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to global defs: %w", err)
	}
	vm.GlobalDefs = make([]DDef, header.NumGlobalDefs)
	if err := binary.Read(r, binary.LittleEndian, &vm.GlobalDefs); err != nil {
		return fmt.Errorf("failed to read global defs: %w", err)
	}

	if _, err := r.Seek(int64(header.FieldDefs), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to field defs: %w", err)
	}
	vm.FieldDefs = make([]DDef, header.NumFieldDefs)
	if err := binary.Read(r, binary.LittleEndian, &vm.FieldDefs); err != nil {
		return fmt.Errorf("failed to read field defs: %w", err)
	}

	if _, err := r.Seek(int64(header.Functions), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to functions: %w", err)
	}
	vm.Functions = make([]DFunction, header.NumFunctions)
	if err := binary.Read(r, binary.LittleEndian, &vm.Functions); err != nil {
		return fmt.Errorf("failed to read functions: %w", err)
	}

	if _, err := r.Seek(int64(header.Strings), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to strings: %w", err)
	}
	vm.Strings = make([]byte, header.NumStrings)
	if _, err := io.ReadFull(r, vm.Strings); err != nil {
		return fmt.Errorf("failed to read strings: %w", err)
	}

	if _, err := r.Seek(int64(header.Globals), io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek to globals: %w", err)
	}
	vm.Globals = make([]float32, header.NumGlobals)
	if err := binary.Read(r, binary.LittleEndian, &vm.Globals); err != nil {
		return fmt.Errorf("failed to read globals: %w", err)
	}

	vm.EntityFields = int(header.EntityFields)
	vm.EdictSize = int(header.EntityFields)*4 + 28

	return nil
}

func (vm *VM) FindFunction(name string) int {
	for i, fn := range vm.Functions {
		if vm.GetString(fn.Name) == name {
			return i
		}
	}
	return -1
}

func (vm *VM) FindGlobal(name string) int {
	for _, def := range vm.GlobalDefs {
		if vm.GetString(def.Name) == name {
			return int(def.Ofs)
		}
	}
	return -1
}

func (vm *VM) FindField(name string) int {
	for _, def := range vm.FieldDefs {
		if vm.GetString(def.Name) == name {
			return int(def.Ofs)
		}
	}
	return -1
}

func (vm *VM) EnterFunction(f *DFunction) error {
	if vm.Depth >= MaxStackDepth {
		return fmt.Errorf("stack overflow")
	}

	vm.Stack[vm.Depth].S = vm.XStatement
	vm.Stack[vm.Depth].Func = vm.XFunction
	vm.Depth++

	// Save off any locals that the new function steps on.
	// C: for (i = 0; i < c; i++) localstack[used+i] = ((int*)globals)[parm_start+i]
	c := int(f.Locals)
	if vm.LocalUsed+c > LocalStackSize {
		return fmt.Errorf("local stack overflow")
	}
	for i := 0; i < c; i++ {
		vm.LocalStack[vm.LocalUsed+i] = int32(math.Float32bits(vm.Globals[int(f.ParmStart)+i]))
	}
	vm.LocalUsed += c

	// Copy parameters from OFS_PARM* to the function's local space.
	// C: o = f->parm_start; for each parm, copy parm_size slots from OFS_PARMn
	o := int(f.ParmStart)
	for i := 0; i < int(f.NumParms); i++ {
		for j := 0; j < int(f.ParmSize[i]); j++ {
			vm.Globals[o] = vm.Globals[OFSParm0+i*3+j]
			o++
		}
	}

	vm.XFunction = f

	return nil
}

func (vm *VM) LeaveFunction() error {
	if vm.Depth <= 0 {
		return fmt.Errorf("stack underflow")
	}

	// Restore locals from the stack.
	// C: localstack_used -= c; for (i = 0; i < c; i++) ((int*)globals)[parm_start+i] = localstack[used+i]
	if vm.XFunction != nil {
		c := int(vm.XFunction.Locals)
		vm.LocalUsed -= c
		if vm.LocalUsed < 0 {
			return fmt.Errorf("local stack underflow")
		}
		for i := 0; i < c; i++ {
			vm.Globals[int(vm.XFunction.ParmStart)+i] = math.Float32frombits(uint32(vm.LocalStack[vm.LocalUsed+i]))
		}
	}

	// Up stack
	vm.Depth--
	vm.XFunction = vm.Stack[vm.Depth].Func
	vm.XStatement = vm.Stack[vm.Depth].S

	return nil
}
