package compiler

import "github.com/ironwail/ironwail-go/internal/qc"

// VReg is a virtual register identifier used during IR construction.
// Virtual registers are mapped to global offsets during code generation.
type VReg uint32

const VRegInvalid VReg = 0xFFFFFFFF

// vregBase is the starting VReg number for auto-allocated virtual registers.
// This must be higher than any OFS_* constant used as a direct global offset
// in IR instructions (e.g., OFS_RETURN=1, OFS_PARM7=25, OFS_PARMSTART=43).
// VReg values below vregBase are treated as direct global offsets by codegen.
const vregBase VReg = 0x1000

// IRInst represents a single IR instruction.
type IRInst struct {
	Op       qc.Opcode // QCVM opcode
	A, B, C  VReg      // Operands (virtual registers)
	Type     qc.EType  // Result type
	ImmFloat float64   // Immediate float value (for const loads)
	ImmStr   string    // Immediate string value (for const strings)
	Label    string    // Branch target label (for GOTO/IF/IFNOT)
	ArgCount int       // For CALL instructions: number of arguments
}

// IRParam describes a function parameter.
type IRParam struct {
	Name string
	Type qc.EType
}

// IRFunc represents a function in the IR.
type IRFunc struct {
	Name       string    // Go function name
	QCName     string    // Name as it appears in progs.dat
	Params     []IRParam // Parameters
	ReturnType qc.EType  // Return type (EvVoid if none)
	Body       []IRInst  // IR instructions
	Locals     []IRLocal // Local variables
	IsBuiltin  bool      // True if this is a builtin function
	BuiltinNum int       // Builtin number (negative first_statement)
}

// IRLocal describes a local variable within a function.
type IRLocal struct {
	Name string
	Type qc.EType
	VReg VReg
}

// IRGlobal represents a global variable.
type IRGlobal struct {
	Name      string
	Type      qc.EType
	Offset    uint16  // Assigned global offset
	InitFloat float64 // Initial float value
	InitStr   string  // Initial string value (interned later)
	InitVec   [3]float32
}

// IRField represents an entity field definition.
type IRField struct {
	Name   string
	Type   qc.EType
	Offset uint16 // Field offset in entity data
}

// IRProgram is the complete IR representation of a compiled program.
type IRProgram struct {
	Functions []IRFunc
	Globals   []IRGlobal
	Fields    []IRField
}

// LabelInst creates a pseudo-instruction that marks a label target.
func LabelInst(name string) IRInst {
	return IRInst{Label: ":" + name} // ":" prefix distinguishes label defs from refs
}

// IsLabel returns true if this instruction is a label definition.
func (inst *IRInst) IsLabel() bool {
	return len(inst.Label) > 0 && inst.Label[0] == ':'
}

// LabelName returns the label name (without the ":" prefix).
func (inst *IRInst) LabelName() string {
	if inst.IsLabel() {
		return inst.Label[1:]
	}
	return inst.Label
}

// optimizeIRProgram runs lightweight IR optimization passes that preserve
// current semantics while trimming no-op work from lowering output.
func optimizeIRProgram(prog *IRProgram) {
	for i := range prog.Functions {
		fn := &prog.Functions[i]
		if fn.IsBuiltin {
			continue
		}
		optimizeIRFunc(fn)
	}
}

func optimizeIRFunc(fn *IRFunc) {
	foldStoreSelfCopies(fn)
}

// foldStoreSelfCopies removes no-op stores like OPStoreF x -> x that can be
// emitted for constants and return-value materialization.
func foldStoreSelfCopies(fn *IRFunc) {
	if len(fn.Body) == 0 {
		return
	}
	optimized := fn.Body[:0]
	for _, inst := range fn.Body {
		if isNoOpStore(inst) {
			continue
		}
		optimized = append(optimized, inst)
	}
	fn.Body = optimized
}

func isNoOpStore(inst IRInst) bool {
	if inst.ImmStr != "" {
		return false
	}
	if inst.Op == qc.OPStoreF && inst.ImmFloat != 0 {
		return false
	}
	switch inst.Op {
	case qc.OPStoreF, qc.OPStoreV, qc.OPStoreS, qc.OPStoreEnt, qc.OPStoreFld, qc.OPStoreFNC:
		return inst.A == inst.B
	default:
		return false
	}
}
