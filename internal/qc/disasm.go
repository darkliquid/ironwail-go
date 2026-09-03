package qc

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// opcodeNames maps each defined Opcode to its printable mnemonic. The index in
// the slice equals the opcode value (the enum is a zero-based iota).
var opcodeNames = []string{
	OPDone:      "DONE",
	OPMulF:      "MULF",
	OPMulV:      "MULV",
	OPMulFV:     "MULFV",
	OPMulVF:     "MULVF",
	OPDivF:      "DIVF",
	OPAddF:      "ADDF",
	OPAddV:      "ADDV",
	OPSubF:      "SUBF",
	OPSubV:      "SUBV",
	OPEqF:       "EQF",
	OPEqV:       "EQV",
	OPEqS:       "EQS",
	OPEqE:       "EQE",
	OPEqFNC:     "EQFNC",
	OPNeF:       "NEF",
	OPNeV:       "NEV",
	OPNeS:       "NES",
	OPNeE:       "NEE",
	OPNeFNC:     "NEFNC",
	OPLE:        "LE",
	OPGE:        "GE",
	OPLT:        "LT",
	OPGT:        "GT",
	OPLoadF:     "LOADF",
	OPLoadV:     "LOADV",
	OPLoadS:     "LOADS",
	OPLoadEnt:   "LOADENT",
	OPLoadFld:   "LOADFLD",
	OPLoadFNC:   "LOADFNC",
	OPAddress:   "ADDRESS",
	OPStoreF:    "STOREF",
	OPStoreV:    "STOREV",
	OPStoreS:    "STORES",
	OPStoreEnt:  "STOREENT",
	OPStoreFld:  "STOREFLD",
	OPStoreFNC:  "STOREFNC",
	OPStorePF:   "STOREPF",
	OPStorePV:   "STOREPV",
	OPStorePS:   "STOREPS",
	OPStorePEnt: "STOREPENT",
	OPStorePFld: "STOREPFLD",
	OPStorePFNC: "STOREPFNC",
	OPReturn:    "RETURN",
	OPNotF:      "NOTF",
	OPNotV:      "NOTV",
	OPNotS:      "NOTS",
	OPNotEnt:    "NOTENT",
	OPNotFNC:    "NOTFNC",
	OPIF:        "IF",
	OPIFNot:     "IFNOT",
	OPCall0:     "CALL0",
	OPCall1:     "CALL1",
	OPCall2:     "CALL2",
	OPCall3:     "CALL3",
	OPCall4:     "CALL4",
	OPCall5:     "CALL5",
	OPCall6:     "CALL6",
	OPCall7:     "CALL7",
	OPCall8:     "CALL8",
	OPState:     "STATE",
	OPGoto:      "GOTO",
	OPAnd:       "AND",
	OPOr:        "OR",
	OPBitAnd:    "BITAND",
	OPBitOr:     "BITOR",
}

// OpcodeName returns the printable mnemonic for an opcode, or a placeholder
// like "?17" for values outside the defined enum range.
func OpcodeName(op Opcode) string {
	if int(op) >= 0 && int(op) < len(opcodeNames) && opcodeNames[op] != "" {
		return opcodeNames[op]
	}
	return fmt.Sprintf("?%d", int(op))
}

// DisasmOptions controls disassembly output.
type DisasmOptions struct {
	// Function, when non-empty, restricts output to the named function.
	Function string
}

// Disassemble writes a readable disassembly of the loaded progs to w. With
// opts.Function empty it disassembles every function; otherwise only the named
// one. It returns an error when a named function is not present.
func (vm *VM) Disassemble(w io.Writer, opts DisasmOptions) error {
	if vm == nil {
		return fmt.Errorf("nil VM")
	}

	globals := vm.globalNameMap()
	fields := vm.fieldNameMap()

	_, _ = fmt.Fprintf(w, "; progs.dat: crc=%d functions=%d statements=%d globals=%d strings=%d\n",
		vm.CRC, len(vm.Functions), len(vm.Statements), len(vm.Globals), len(vm.Strings))

	if opts.Function != "" {
		idx := vm.FindFunction(opts.Function)
		if idx < 0 {
			return fmt.Errorf("function %q not found", opts.Function)
		}
		vm.disassembleFunction(w, idx, globals, fields)
		return nil
	}

	for _, idx := range vm.bytecodeFunctionOrder() {
		vm.disassembleFunction(w, idx, globals, fields)
	}
	return nil
}

// bytecodeFunctionOrder returns function indices sorted by first statement so
// each bytecode function's statement range can be computed. Builtins (negative
// FirstStatement) sort first by index and have no statement body.
func (vm *VM) bytecodeFunctionOrder() []int {
	order := make([]int, 0, len(vm.Functions))
	for i := range vm.Functions {
		order = append(order, i)
	}
	sort.SliceStable(order, func(a, b int) bool {
		fa, fb := &vm.Functions[order[a]], &vm.Functions[order[b]]
		// Builtins (negative) have no statements; keep them in index order first.
		aBuilt, bBuilt := fa.FirstStatement < 0, fb.FirstStatement < 0
		if aBuilt != bBuilt {
			return aBuilt
		}
		if aBuilt && bBuilt {
			return order[a] < order[b]
		}
		return fa.FirstStatement < fb.FirstStatement
	})
	return order
}

// functionStatementRange returns the [start, end) statement index range owned
// by the function, or an empty range for builtins.
func (vm *VM) functionStatementRange(idx int) (int, int) {
	f := &vm.Functions[idx]
	if f.FirstStatement < 0 {
		return 0, 0
	}
	start := int(f.FirstStatement)
	end := len(vm.Statements)
	// End is the smallest bytecode first-statement greater than start.
	for j := range vm.Functions {
		if j == idx {
			continue
		}
		fs := vm.Functions[j].FirstStatement
		if fs >= 0 && int(fs) > start && int(fs) < end {
			end = int(fs)
		}
	}
	return start, end
}

func (vm *VM) disassembleFunction(w io.Writer, idx int, globals, fields map[int32]string) {
	f := &vm.Functions[idx]
	name := vm.String(f.Name)

	if f.FirstStatement < 0 {
		_, _ = fmt.Fprintf(w, "func %d %s = builtin %d\n", idx, name, -f.FirstStatement)
		return
	}

	start, end := vm.functionStatementRange(idx)
	_, _ = fmt.Fprintf(w, "\nfunc %d %s  (statements %d..%d)\n", idx, name, start, end)
	for s := start; s < end && s < len(vm.Statements); s++ {
		_, _ = fmt.Fprintf(w, "  %s\n", vm.decodeStatement(s, globals, fields))
	}
}

// decodeStatement renders one statement as a readable, annotated line.
// decodeStatement renders one statement as a readable, annotated line. Operand
// shapes follow the QCVM interpreter: most ops read globals A and B and write
// global C; stores write B; loads read an entity and a field-offset holder.
func (vm *VM) decodeStatement(idx int, globals, fields map[int32]string) string {
	st := &vm.Statements[idx]
	op := Opcode(st.Op)

	var b strings.Builder
	fmt.Fprintf(&b, "%6d  %-8s", idx, OpcodeName(op))

	a, bb, c := globalRef(int(st.A), globals), globalRef(int(st.B), globals), globalRef(int(st.C), globals)

	switch {
	case op >= OPCall0 && op <= OPCall8:
		fmt.Fprintf(&b, " %s", a)
		if ann := vm.callAnnotation(int(st.A)); ann != "" {
			fmt.Fprintf(&b, "  ; %s", ann)
		}
	case op == OPIF || op == OPIFNot:
		fmt.Fprintf(&b, " %s -> %d", a, idx+int(int16(st.B)))
	case op == OPGoto:
		fmt.Fprintf(&b, " -> %d", idx+int(int16(st.A)))
	case op == OPLoadF || op == OPLoadV || op == OPLoadS || op == OPLoadEnt || op == OPLoadFld || op == OPLoadFNC:
		fmt.Fprintf(&b, " %s  ; %s = %s.%s", c, c, a, vm.fieldRefName(int(st.B), fields))
	case op == OPStorePF || op == OPStorePV || op == OPStorePS || op == OPStorePEnt || op == OPStorePFld || op == OPStorePFNC:
		fmt.Fprintf(&b, " %s -> ptr g%d", a, st.B)
	case op == OPStoreF || op == OPStoreV || op == OPStoreS || op == OPStoreEnt || op == OPStoreFld || op == OPStoreFNC:
		fmt.Fprintf(&b, " %s -> %s", a, bb)
	case op == OPEqS || op == OPNeS:
		fmt.Fprintf(&b, " %s(%q), %s(%q) -> %s", a, vm.stringAt(int(st.A)), bb, vm.stringAt(int(st.B)), c)
	case op == OPState:
		fmt.Fprintf(&b, " frame=%g think=%s", vm.GFloat(int(st.A)), bb)
	case op == OPDone || op == OPReturn || op == OPNotF || op == OPNotV || op == OPNotS || op == OPNotEnt || op == OPNotFNC:
		fmt.Fprintf(&b, " %s", a)
	default: // arithmetic, comparisons, logic: A op B -> C
		fmt.Fprintf(&b, " %s, %s -> %s", a, bb, c)
	}

	return b.String()
}

// stringAt resolves a global holding a string index to its content, for
// annotation previews.
func (vm *VM) stringAt(gofs int) string {
	return vm.String(vm.GInt(gofs))
}

// callAnnotation resolves a CALL operand (a global holding a function index)
// to "name()" for bytecode functions or "name (builtin N)" for builtins.
// callAnnotation resolves a CALL operand (a global holding a function index)
// to "name()" for bytecode functions or "name (builtin N)" for builtins.
// Function references are stored as raw int32 bit patterns in the globals
// array, so they are read via GFunction (not GFloat).
func (vm *VM) callAnnotation(gofs int) string {
	fn := int(vm.GFunction(gofs))
	if fn < 0 || fn >= len(vm.Functions) {
		return ""
	}
	f := &vm.Functions[fn]
	name := vm.String(f.Name)
	if f.FirstStatement < 0 {
		return fmt.Sprintf("%s (builtin %d)", name, -f.FirstStatement)
	}
	if name == "" {
		return fmt.Sprintf("fn%d()", fn)
	}
	return name + "()"
}

// fieldRefName resolves the field-offset-holder global used by OPLoad/STOREP to
// the field's name. The referenced global's value is the entity field index.
func (vm *VM) fieldRefName(gofs int, fields map[int32]string) string {
	fieldOfs := vm.GInt(gofs)
	if name, ok := fields[fieldOfs]; ok {
		return name
	}
	return fmt.Sprintf("field_%d", fieldOfs)
}

// globalNameMap builds offset -> global name from the progs GlobalDefs.
func (vm *VM) globalNameMap() map[int32]string {
	m := make(map[int32]string, len(vm.GlobalDefs))
	for _, d := range vm.GlobalDefs {
		name := vm.String(d.Name)
		if name == "" {
			continue
		}
		if _, exists := m[int32(d.Ofs)]; !exists {
			m[int32(d.Ofs)] = name
		}
	}
	return m
}

// fieldNameMap builds offset -> field name from the progs FieldDefs.
func (vm *VM) fieldNameMap() map[int32]string {
	m := make(map[int32]string, len(vm.FieldDefs))
	for _, d := range vm.FieldDefs {
		name := vm.String(d.Name)
		if name == "" {
			continue
		}
		if _, exists := m[int32(d.Ofs)]; !exists {
			m[int32(d.Ofs)] = name
		}
	}
	return m
}

// globalRef renders a global operand as name if known, else g<ofs>.
func globalRef(ofs int, globals map[int32]string) string {
	if name, ok := globals[int32(ofs)]; ok {
		return name
	}
	return fmt.Sprintf("g%d", ofs)
}

// globalName is like globalRef but always returns the bare name/placeholder
// used as an assignment destination annotation.
