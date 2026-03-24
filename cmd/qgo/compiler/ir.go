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
	foldLiteralConstFloatOps(fn)
	pruneConstConditionBranches(fn)
	foldStoreSelfCopies(fn)
	propagateLocalCopies(fn)
	pruneUnreachableBlocks(fn)
	eliminateDeadVirtualStores(fn)
	pruneUnusedLocals(fn)
}

// pruneConstConditionBranches removes conditional branches whose condition is a
// known literal float within the current straight-line segment.
func pruneConstConditionBranches(fn *IRFunc) {
	if len(fn.Body) == 0 {
		return
	}

	known := make(map[VReg]float64)
	optimized := make([]IRInst, 0, len(fn.Body))

	for _, inst := range fn.Body {
		if inst.IsLabel() {
			clearKnownFloatConsts(known)
			optimized = append(optimized, inst)
			continue
		}

		switch inst.Op {
		case qc.OPIF, qc.OPIFNot:
			cond, ok := known[inst.A]
			clearKnownFloatConsts(known)
			if !ok {
				optimized = append(optimized, inst)
				continue
			}

			truthy := float32(cond) != 0
			taken := (inst.Op == qc.OPIF && truthy) || (inst.Op == qc.OPIFNot && !truthy)
			if taken {
				optimized = append(optimized, IRInst{Op: qc.OPGoto, Label: inst.Label})
			}
			continue
		}

		updateKnownFloatConsts(known, inst, false)
		optimized = append(optimized, inst)
		if isIRBlockTerminator(inst.Op) {
			clearKnownFloatConsts(known)
		}
	}

	fn.Body = optimized
}

// foldLiteralConstFloatOps performs a local constant fold for scalar float
// arithmetic/comparison operations when both operands are known literal float
// immediates in the current traversal.
func foldLiteralConstFloatOps(fn *IRFunc) {
	if len(fn.Body) == 0 {
		return
	}
	known := make(map[VReg]float64)
	for i := range fn.Body {
		inst := fn.Body[i]
		foldedNow := false
		if folded, ok := foldConstFloatInst(inst, known); ok {
			fn.Body[i] = folded
			inst = folded
			foldedNow = true
		}
		updateKnownFloatConsts(known, inst, foldedNow)
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

func updateKnownFloatConsts(known map[VReg]float64, inst IRInst, foldedNow bool) {
	switch inst.Op {
	case qc.OPStoreF:
		if inst.HasImmFloat {
			known[inst.B] = inst.ImmFloat
			return
		}
		delete(known, inst.B)
	case qc.OPAddF, qc.OPSubF, qc.OPMulF, qc.OPDivF,
		qc.OPEqF, qc.OPNeF, qc.OPLE, qc.OPGE, qc.OPLT, qc.OPGT:
		delete(known, inst.C)
	}
}

func clearKnownFloatConsts(known map[VReg]float64) {
	for reg := range known {
		delete(known, reg)
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

// propagateLocalCopies rewrites straight-line uses of local temporaries copied
// from other single-slot locals (tmp = x; use(tmp) -> use(x)) while staying
// within one basic-block segment and invalidating aliases on writes/calls.
func propagateLocalCopies(fn *IRFunc) {
	if len(fn.Body) == 0 || len(fn.Locals) == 0 {
		return
	}

	localSlots := make(map[VReg]uint16, len(fn.Locals))
	for _, local := range fn.Locals {
		localSlots[local.VReg] = slotsForType(local.Type)
	}
	isTrackableLocal := func(v VReg) bool {
		return localSlots[v] == 1
	}

	aliases := make(map[VReg]VReg)
	clearAliases := func() {
		for dst := range aliases {
			delete(aliases, dst)
		}
	}
	resolveAlias := func(v VReg) VReg {
		if !isTrackableLocal(v) {
			return v
		}
		cur := v
		seen := map[VReg]struct{}{}
		for {
			next, ok := aliases[cur]
			if !ok || next == cur {
				return cur
			}
			if _, loop := seen[cur]; loop {
				return cur
			}
			seen[cur] = struct{}{}
			cur = next
		}
	}
	invalidateAliasForDef := func(def VReg) {
		if !isTrackableLocal(def) {
			return
		}
		for dst, src := range aliases {
			if dst == def || src == def || resolveAlias(src) == def {
				delete(aliases, dst)
			}
		}
	}

	rewriteOperand := func(v VReg) VReg {
		return resolveAlias(v)
	}

	for i := range fn.Body {
		inst := fn.Body[i]
		if inst.IsLabel() {
			clearAliases()
			continue
		}

		inst = rewriteCopyPropUses(inst, rewriteOperand)
		info := irLivenessInfo(inst)
		invalidateAliasForDef(info.def)

		if isTrackableLocalCopy(inst, isTrackableLocal) {
			inst.A = resolveAlias(inst.A)
			aliases[inst.B] = inst.A
		}

		fn.Body[i] = inst

		if isIRBlockTerminator(inst.Op) || irMayClobberLocals(inst.Op) {
			clearAliases()
		}
	}
}

func isTrackableLocalCopy(inst IRInst, isTrackableLocal func(VReg) bool) bool {
	if inst.ImmStr != "" || (inst.Op == qc.OPStoreF && inst.HasImmFloat) {
		return false
	}
	switch inst.Op {
	case qc.OPStoreF, qc.OPStoreS, qc.OPStoreEnt, qc.OPStoreFld, qc.OPStoreFNC:
		return inst.A != inst.B && isTrackableLocal(inst.A) && isTrackableLocal(inst.B)
	default:
		return false
	}
}

func rewriteCopyPropUses(inst IRInst, rewrite func(VReg) VReg) IRInst {
	switch inst.Op {
	case qc.OPStoreF, qc.OPStoreV, qc.OPStoreS, qc.OPStoreEnt, qc.OPStoreFld, qc.OPStoreFNC:
		if !(inst.Op == qc.OPStoreS && inst.ImmStr != "") && !(inst.Op == qc.OPStoreF && inst.HasImmFloat) {
			inst.A = rewrite(inst.A)
		}
	case qc.OPLoadF, qc.OPLoadV, qc.OPLoadS, qc.OPLoadEnt, qc.OPLoadFld, qc.OPLoadFNC,
		qc.OPAddress,
		qc.OPAddF, qc.OPSubF, qc.OPMulF, qc.OPDivF,
		qc.OPAddV, qc.OPSubV,
		qc.OPMulFV, qc.OPMulVF,
		qc.OPEqF, qc.OPEqV, qc.OPEqS, qc.OPEqE, qc.OPEqFNC,
		qc.OPNeF, qc.OPNeV, qc.OPNeS, qc.OPNeE, qc.OPNeFNC,
		qc.OPLE, qc.OPGE, qc.OPLT, qc.OPGT,
		qc.OPAnd, qc.OPOr, qc.OPBitAnd, qc.OPBitOr:
		inst.A = rewrite(inst.A)
		inst.B = rewrite(inst.B)
	case qc.OPNotF, qc.OPNotV, qc.OPNotS, qc.OPNotEnt, qc.OPNotFNC:
		inst.A = rewrite(inst.A)
	case qc.OPIF, qc.OPIFNot, qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3, qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
		inst.A = rewrite(inst.A)
	case qc.OPStorePF, qc.OPStorePV, qc.OPStorePS, qc.OPStorePEnt, qc.OPStorePFld, qc.OPStorePFNC:
		inst.A = rewrite(inst.A)
		inst.B = rewrite(inst.B)
	case qc.OPReturn, qc.OPDone:
		inst.A = rewrite(inst.A)
		inst.B = rewrite(inst.B)
		inst.C = rewrite(inst.C)
	}
	return inst
}

func irMayClobberLocals(op qc.Opcode) bool {
	switch op {
	case qc.OPStorePF, qc.OPStorePV, qc.OPStorePS, qc.OPStorePEnt, qc.OPStorePFld, qc.OPStorePFNC,
		qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3, qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
		return true
	default:
		return false
	}
}

// eliminateDeadVirtualStores removes pure instructions that define only virtual
// registers whose values are never consumed along any simple label/branch
// control-flow path in the function body.
func eliminateDeadVirtualStores(fn *IRFunc) {
	if len(fn.Body) == 0 {
		return
	}

	blocks, labelToBlock, ok := buildIRBasicBlocks(fn.Body)
	if !ok || len(blocks) == 0 {
		return
	}

	for i := range blocks {
		succ, succOK := irBlockSuccessors(fn.Body, blocks, labelToBlock, i)
		if !succOK {
			return
		}
		blocks[i].succ = succ
	}

	// Solve backward liveness at block boundaries.
	liveIn := make([]map[VReg]struct{}, len(blocks))
	liveOut := make([]map[VReg]struct{}, len(blocks))
	for i := range blocks {
		liveIn[i] = make(map[VReg]struct{})
		liveOut[i] = make(map[VReg]struct{})
	}

	changed := true
	for changed {
		changed = false
		for bi := len(blocks) - 1; bi >= 0; bi-- {
			newOut := make(map[VReg]struct{})
			for _, succ := range blocks[bi].succ {
				mergeVRegSet(newOut, liveIn[succ])
			}
			newIn := irBlockLiveIn(fn.Body, blocks[bi], newOut)

			if !equalVRegSet(newOut, liveOut[bi]) {
				liveOut[bi] = newOut
				changed = true
			}
			if !equalVRegSet(newIn, liveIn[bi]) {
				liveIn[bi] = newIn
				changed = true
			}
		}
	}

	keptByBlock := make([][]IRInst, len(blocks))
	for bi := len(blocks) - 1; bi >= 0; bi-- {
		block := blocks[bi]
		live := cloneVRegSet(liveOut[bi])
		keptRev := make([]IRInst, 0, block.end-block.start)
		for i := block.end - 1; i >= block.start; i-- {
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
		keptByBlock[bi] = keptRev
	}

	optimized := make([]IRInst, 0, len(fn.Body))
	for _, kept := range keptByBlock {
		optimized = append(optimized, kept...)
	}
	fn.Body = optimized
}

func isVirtualVReg(v VReg) bool {
	return v != VRegInvalid && v >= vregBase
}

type irBlock struct {
	start int
	end   int
	succ  []int
}

func buildIRBasicBlocks(body []IRInst) ([]irBlock, map[string]int, bool) {
	if len(body) == 0 {
		return nil, nil, true
	}

	leaders := map[int]struct{}{0: {}}
	for i, inst := range body {
		if inst.IsLabel() {
			leaders[i] = struct{}{}
		}
		if i+1 < len(body) && isIRBlockTerminator(inst.Op) {
			leaders[i+1] = struct{}{}
		}
	}

	leaderOrder := make([]int, 0, len(leaders))
	for idx := range leaders {
		leaderOrder = append(leaderOrder, idx)
	}
	for i := 0; i < len(leaderOrder)-1; i++ {
		for j := i + 1; j < len(leaderOrder); j++ {
			if leaderOrder[j] < leaderOrder[i] {
				leaderOrder[i], leaderOrder[j] = leaderOrder[j], leaderOrder[i]
			}
		}
	}

	blocks := make([]irBlock, 0, len(leaderOrder))
	for i, start := range leaderOrder {
		end := len(body)
		if i+1 < len(leaderOrder) {
			end = leaderOrder[i+1]
		}
		blocks = append(blocks, irBlock{start: start, end: end})
	}

	labelToBlock := make(map[string]int)
	for bi, block := range blocks {
		for i := block.start; i < block.end; i++ {
			if !body[i].IsLabel() {
				break
			}
			labelToBlock[body[i].LabelName()] = bi
		}
	}
	return blocks, labelToBlock, true
}

func isIRBlockTerminator(op qc.Opcode) bool {
	switch op {
	case qc.OPGoto, qc.OPIF, qc.OPIFNot, qc.OPReturn, qc.OPDone:
		return true
	default:
		return false
	}
}

// pruneUnreachableBlocks removes whole basic blocks that cannot be reached from
// function entry once explicit control-flow terminators are respected.
func pruneUnreachableBlocks(fn *IRFunc) {
	if len(fn.Body) == 0 {
		return
	}

	blocks, labelToBlock, ok := buildIRBasicBlocks(fn.Body)
	if !ok || len(blocks) == 0 {
		return
	}

	succ := make([][]int, len(blocks))
	for i := range blocks {
		next, succOK := irBlockSuccessors(fn.Body, blocks, labelToBlock, i)
		if !succOK {
			// Unknown branch target: keep function body unchanged.
			return
		}
		succ[i] = next
	}

	reachable := make([]bool, len(blocks))
	work := []int{0}
	reachable[0] = true
	for len(work) > 0 {
		bi := work[len(work)-1]
		work = work[:len(work)-1]
		for _, next := range succ[bi] {
			if next < 0 || next >= len(blocks) || reachable[next] {
				continue
			}
			reachable[next] = true
			work = append(work, next)
		}
	}

	pruned := make([]IRInst, 0, len(fn.Body))
	for bi, block := range blocks {
		if !reachable[bi] {
			continue
		}
		pruned = append(pruned, fn.Body[block.start:block.end]...)
	}
	fn.Body = pruned
}

func irBlockSuccessors(body []IRInst, blocks []irBlock, labelToBlock map[string]int, bi int) ([]int, bool) {
	termInst, ok := irBlockTerminatorInst(body, blocks[bi])
	if !ok {
		if bi+1 < len(blocks) {
			return []int{bi + 1}, true
		}
		return nil, true
	}

	switch termInst.Op {
	case qc.OPGoto:
		target, ok := labelToBlock[termInst.Label]
		if !ok {
			return nil, false
		}
		return []int{target}, true
	case qc.OPIF, qc.OPIFNot:
		target, ok := labelToBlock[termInst.Label]
		if !ok {
			return nil, false
		}
		succ := []int{target}
		if bi+1 < len(blocks) && target != bi+1 {
			succ = append(succ, bi+1)
		}
		return succ, true
	case qc.OPReturn, qc.OPDone:
		return nil, true
	default:
		if bi+1 < len(blocks) {
			return []int{bi + 1}, true
		}
		return nil, true
	}
}

func irBlockTerminatorInst(body []IRInst, block irBlock) (IRInst, bool) {
	for i := block.end - 1; i >= block.start; i-- {
		inst := body[i]
		if inst.IsLabel() {
			continue
		}
		if isIRBlockTerminator(inst.Op) {
			return inst, true
		}
		return IRInst{}, false
	}
	return IRInst{}, false
}

func irBlockLiveIn(body []IRInst, block irBlock, liveOut map[VReg]struct{}) map[VReg]struct{} {
	live := cloneVRegSet(liveOut)
	for i := block.end - 1; i >= block.start; i-- {
		inst := body[i]
		info := irLivenessInfo(inst)
		if info.pure && isVirtualVReg(info.def) {
			if _, ok := live[info.def]; ok {
				delete(live, info.def)
				for _, u := range info.uses {
					if isVirtualVReg(u) {
						live[u] = struct{}{}
					}
				}
			}
			continue
		}
		if isVirtualVReg(info.def) {
			delete(live, info.def)
		}
		for _, u := range info.uses {
			if isVirtualVReg(u) {
				live[u] = struct{}{}
			}
		}
	}
	return live
}

func cloneVRegSet(in map[VReg]struct{}) map[VReg]struct{} {
	out := make(map[VReg]struct{}, len(in))
	for v := range in {
		out[v] = struct{}{}
	}
	return out
}

func mergeVRegSet(dst, src map[VReg]struct{}) {
	for v := range src {
		dst[v] = struct{}{}
	}
}

func equalVRegSet(a, b map[VReg]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for v := range a {
		if _, ok := b[v]; !ok {
			return false
		}
	}
	return true
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
