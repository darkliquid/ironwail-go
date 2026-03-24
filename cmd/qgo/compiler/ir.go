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
	Op          qc.Opcode // QCVM opcode
	A, B, C     VReg      // Operands (virtual registers)
	Type        qc.EType  // Result type
	ImmFloat    float64   // Immediate float value (for const loads)
	HasImmFloat bool      // True when ImmFloat is a meaningful immediate value
	ImmStr      string    // Immediate string value (for const strings)
	Label       string    // Branch target label (for GOTO/IF/IFNOT)
	ArgCount    int       // For CALL instructions: number of arguments
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
	foldConstFloatOps(fn)
	foldStoreSelfCopies(fn)
	eliminateDeadVirtualStores(fn)
	pruneUnusedLocals(fn)
}

// foldConstFloatOps performs a local constant fold for scalar float operations.
// It only folds instructions whose operands are known float constants within
// the current function body traversal order.
func foldConstFloatOps(fn *IRFunc) {
	if len(fn.Body) == 0 {
		return
	}
	known := make(map[VReg]float64)
	for i := range fn.Body {
		inst := fn.Body[i]
		if folded, ok := foldConstFloatInst(inst, known); ok {
			fn.Body[i] = folded
			inst = folded
		}
		updateKnownFloatConsts(known, inst)
	}
}

func foldConstFloatInst(inst IRInst, known map[VReg]float64) (IRInst, bool) {
	a, aok := known[inst.A]
	b, bok := known[inst.B]
	fa := float32(a)
	fb := float32(b)
	var out float32
	switch inst.Op {
	case qc.OPAddF:
		if !aok || !bok {
			return IRInst{}, false
		}
		out = fa + fb
	case qc.OPSubF:
		if !aok || !bok {
			return IRInst{}, false
		}
		out = fa - fb
	case qc.OPMulF:
		if !aok || !bok {
			return IRInst{}, false
		}
		out = fa * fb
	case qc.OPDivF:
		if !aok || !bok {
			return IRInst{}, false
		}
		out = fa / fb
	case qc.OPEqF:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa == fb {
			out = 1
		}
	case qc.OPNeF:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa != fb {
			out = 1
		}
	case qc.OPLE:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa <= fb {
			out = 1
		}
	case qc.OPGE:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa >= fb {
			out = 1
		}
	case qc.OPLT:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa < fb {
			out = 1
		}
	case qc.OPGT:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa > fb {
			out = 1
		}
	case qc.OPAnd:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa != 0 && fb != 0 {
			out = 1
		}
	case qc.OPOr:
		if !aok || !bok {
			return IRInst{}, false
		}
		if fa != 0 || fb != 0 {
			out = 1
		}
	case qc.OPBitAnd:
		if !aok || !bok {
			return IRInst{}, false
		}
		out = float32(int(fa) & int(fb))
	case qc.OPBitOr:
		if !aok || !bok {
			return IRInst{}, false
		}
		out = float32(int(fa) | int(fb))
	case qc.OPNotF:
		if !aok {
			return IRInst{}, false
		}
		if fa == 0 {
			out = 1
		}
	default:
		return IRInst{}, false
	}

	return IRInst{
		Op:          qc.OPStoreF,
		A:           inst.C,
		B:           inst.C,
		Type:        EvFloat,
		ImmFloat:    float64(out),
		HasImmFloat: true,
	}, true
}

func updateKnownFloatConsts(known map[VReg]float64, inst IRInst) {
	switch inst.Op {
	case qc.OPStoreF:
		if inst.HasImmFloat {
			known[inst.B] = inst.ImmFloat
			return
		}
		if v, ok := known[inst.A]; ok {
			known[inst.B] = v
			return
		}
		delete(known, inst.B)
	case qc.OPAddF, qc.OPSubF, qc.OPMulF, qc.OPDivF,
		qc.OPEqF, qc.OPNeF, qc.OPLE, qc.OPGE, qc.OPLT, qc.OPGT,
		qc.OPAnd, qc.OPOr, qc.OPBitAnd, qc.OPBitOr, qc.OPNotF:
		delete(known, inst.C)
	}
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
	if inst.Op == qc.OPStoreF && inst.HasImmFloat {
		return false
	}
	switch inst.Op {
	case qc.OPStoreF, qc.OPStoreV, qc.OPStoreS, qc.OPStoreEnt, qc.OPStoreFld, qc.OPStoreFNC:
		return inst.A == inst.B
	default:
		return false
	}
}

// eliminateDeadVirtualStores removes pure instructions that define only virtual
// registers whose values are never consumed later in the same straight-line IR
// body. To keep this first DCE slice safe and deterministic, the pass skips
// functions that contain labels or branch opcodes.
func eliminateDeadVirtualStores(fn *IRFunc) {
	if len(fn.Body) == 0 || hasIRControlFlow(fn.Body) {
		return
	}

	live := make(map[VReg]struct{})
	keptRev := make([]IRInst, 0, len(fn.Body))
	for i := len(fn.Body) - 1; i >= 0; i-- {
		inst := fn.Body[i]
		info := irLivenessInfo(inst)

		keep := true
		if info.pure && isVirtualVReg(info.def) {
			if _, ok := live[info.def]; !ok {
				keep = false
			} else {
				delete(live, info.def)
			}
		} else if isVirtualVReg(info.def) {
			delete(live, info.def)
		}

		if keep {
			for _, u := range info.uses {
				if isVirtualVReg(u) {
					live[u] = struct{}{}
				}
			}
			keptRev = append(keptRev, inst)
		}
	}

	for i, j := 0, len(keptRev)-1; i < j; i, j = i+1, j-1 {
		keptRev[i], keptRev[j] = keptRev[j], keptRev[i]
	}
	fn.Body = keptRev
}

func hasIRControlFlow(body []IRInst) bool {
	for _, inst := range body {
		if inst.IsLabel() {
			return true
		}
		switch inst.Op {
		case qc.OPGoto, qc.OPIF, qc.OPIFNot:
			return true
		}
	}
	return false
}

func isVirtualVReg(v VReg) bool {
	return v != VRegInvalid && v >= vregBase
}

type irInstInfo struct {
	def  VReg
	uses []VReg
	pure bool
}

func irLivenessInfo(inst IRInst) irInstInfo {
	info := irInstInfo{
		def:  VRegInvalid,
		uses: nil,
		pure: false,
	}

	switch inst.Op {
	case qc.OPStoreF, qc.OPStoreV, qc.OPStoreS, qc.OPStoreEnt, qc.OPStoreFld, qc.OPStoreFNC:
		info.def = inst.B
		info.pure = true
		if !(inst.Op == qc.OPStoreS && inst.ImmStr != "") && !(inst.Op == qc.OPStoreF && inst.HasImmFloat) {
			info.uses = []VReg{inst.A}
		}
	case qc.OPLoadF, qc.OPLoadV, qc.OPLoadS, qc.OPLoadEnt, qc.OPLoadFld, qc.OPLoadFNC:
		info.def = inst.C
		info.uses = []VReg{inst.A, inst.B}
		info.pure = true
	case qc.OPAddress:
		info.def = inst.C
		info.uses = []VReg{inst.A, inst.B}
		info.pure = true
	case qc.OPAddF, qc.OPSubF, qc.OPMulF, qc.OPDivF,
		qc.OPAddV, qc.OPSubV,
		qc.OPMulFV, qc.OPMulVF,
		qc.OPEqF, qc.OPEqV, qc.OPEqS, qc.OPEqE, qc.OPEqFNC,
		qc.OPNeF, qc.OPNeV, qc.OPNeS, qc.OPNeE, qc.OPNeFNC,
		qc.OPLE, qc.OPGE, qc.OPLT, qc.OPGT,
		qc.OPAnd, qc.OPOr, qc.OPBitAnd, qc.OPBitOr:
		info.def = inst.C
		info.uses = []VReg{inst.A, inst.B}
		info.pure = true
	case qc.OPNotF, qc.OPNotV, qc.OPNotS, qc.OPNotEnt, qc.OPNotFNC:
		info.def = inst.C
		info.uses = []VReg{inst.A}
		info.pure = true
	case qc.OPIF, qc.OPIFNot:
		info.uses = []VReg{inst.A}
	case qc.OPStorePF, qc.OPStorePV, qc.OPStorePS, qc.OPStorePEnt, qc.OPStorePFld, qc.OPStorePFNC:
		info.uses = []VReg{inst.A, inst.B}
	case qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3, qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
		info.uses = []VReg{inst.A}
	case qc.OPReturn, qc.OPDone:
		info.uses = []VReg{inst.A, inst.B, inst.C}
	default:
		info.uses = []VReg{inst.A, inst.B, inst.C}
	}

	return info
}

func pruneUnusedLocals(fn *IRFunc) {
	if len(fn.Locals) == 0 {
		return
	}

	used := collectUsedVRegs(fn.Body)
	if len(used) == 0 {
		if len(fn.Params) < len(fn.Locals) {
			fn.Locals = fn.Locals[:len(fn.Params)]
		}
		return
	}

	kept := fn.Locals[:0]
	for i, local := range fn.Locals {
		if i < len(fn.Params) || localUsesAnySlot(local, used) {
			kept = append(kept, local)
		}
	}
	fn.Locals = kept
}

func collectUsedVRegs(body []IRInst) map[VReg]struct{} {
	used := make(map[VReg]struct{})
	for _, inst := range body {
		if inst.IsLabel() {
			continue
		}
		if inst.ImmStr != "" && inst.Op == qc.OPStoreS {
			markUsedVReg(used, inst.B)
			continue
		}
		if inst.Op == qc.OPStoreF && inst.HasImmFloat {
			markUsedVReg(used, inst.B)
			continue
		}

		switch inst.Op {
		case qc.OPGoto:
			continue
		case qc.OPIF, qc.OPIFNot:
			markUsedVReg(used, inst.A)
			continue
		case qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3, qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
			markUsedVReg(used, inst.A)
			continue
		}

		markUsedVReg(used, inst.A)
		markUsedVReg(used, inst.B)
		markUsedVReg(used, inst.C)
	}
	return used
}

func markUsedVReg(used map[VReg]struct{}, v VReg) {
	if v == VRegInvalid {
		return
	}
	used[v] = struct{}{}
}

func localUsesAnySlot(local IRLocal, used map[VReg]struct{}) bool {
	slots := slotsForType(local.Type)
	for i := uint16(0); i < slots; i++ {
		if _, ok := used[local.VReg+VReg(i)]; ok {
			return true
		}
	}
	return false
}
