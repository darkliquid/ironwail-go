package compiler

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestOpcodeForStore(t *testing.T) {
	tests := []struct {
		typ    qc.EType
		expect qc.Opcode
	}{
		{EvFloat, qc.OPStoreF},
		{EvVector, qc.OPStoreV},
		{EvString, qc.OPStoreS},
		{EvEntity, qc.OPStoreEnt},
		{EvField, qc.OPStoreFld},
		{EvFunction, qc.OPStoreFNC},
	}
	for _, tt := range tests {
		got := opcodeForStore(tt.typ)
		if got != tt.expect {
			t.Errorf("opcodeForStore(%d) = %d, want %d", tt.typ, got, tt.expect)
		}
	}
}

func TestOpcodeForLoad(t *testing.T) {
	tests := []struct {
		typ    qc.EType
		expect qc.Opcode
	}{
		{EvFloat, qc.OPLoadF},
		{EvVector, qc.OPLoadV},
		{EvString, qc.OPLoadS},
		{EvEntity, qc.OPLoadEnt},
		{EvField, qc.OPLoadFld},
		{EvFunction, qc.OPLoadFNC},
	}
	for _, tt := range tests {
		got := opcodeForLoad(tt.typ)
		if got != tt.expect {
			t.Errorf("opcodeForLoad(%d) = %d, want %d", tt.typ, got, tt.expect)
		}
	}
}

func TestSlotsForType(t *testing.T) {
	if slotsForType(EvVector) != 3 {
		t.Error("vector should use 3 slots")
	}
	if slotsForType(EvFloat) != 1 {
		t.Error("float should use 1 slot")
	}
	if slotsForType(EvString) != 1 {
		t.Error("string should use 1 slot")
	}
}

func TestOptimizeIRProgram_RemovesNoOpSelfStores(t *testing.T) {
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, A: 10, B: 10},                             // removable
					{Op: qc.OPStoreS, A: 11, B: 11},                             // removable
					{Op: qc.OPStoreF, B: 12, ImmFloat: 5, HasImmFloat: true},    // keep immediate const
					{Op: qc.OPStoreS, B: 13, ImmStr: "hello"},                   // keep immediate const
					{Op: qc.OPStoreV, A: 20, B: 21},                             // keep real copy
					{Op: qc.OPStoreF, A: qc.OFSReturn, B: qc.OFSReturn},         // removable
					{Op: qc.OPStoreEnt, A: qc.OFSSelf, B: qc.OFSSelf},           // removable
					{Op: qc.OPStoreFNC, A: qc.OFSMsgEntity, B: qc.OFSMsgEntity}, // removable
				},
			},
			{
				Name:      "builtin",
				IsBuiltin: true,
				Body: []IRInst{
					{Op: qc.OPStoreF, A: 1, B: 1}, // optimizer should skip builtin bodies
				},
			},
		},
	}

	optimizeIRProgram(prog)

	mainBody := prog.Functions[0].Body
	if len(mainBody) != 3 {
		t.Fatalf("main body len = %d, want 3", len(mainBody))
	}
	if mainBody[0].Op != qc.OPStoreF || mainBody[0].ImmFloat != 5 {
		t.Fatalf("first kept inst = %+v, want immediate float store", mainBody[0])
	}
	if mainBody[1].Op != qc.OPStoreS || mainBody[1].ImmStr != "hello" {
		t.Fatalf("second kept inst = %+v, want immediate string store", mainBody[1])
	}
	if mainBody[2].Op != qc.OPStoreV || mainBody[2].A != 20 || mainBody[2].B != 21 {
		t.Fatalf("third kept inst = %+v, want vector copy store 20->21", mainBody[2])
	}

	builtinBody := prog.Functions[1].Body
	if len(builtinBody) != 1 {
		t.Fatalf("builtin body len = %d, want 1", len(builtinBody))
	}
	if builtinBody[0].Op != qc.OPStoreF || builtinBody[0].A != 1 || builtinBody[0].B != 1 {
		t.Fatalf("builtin body modified unexpectedly: %+v", builtinBody[0])
	}
}

func TestOptimizeIRProgram_FoldsLiteralArithmeticAndComparisons(t *testing.T) {
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, A: 100, B: 100, ImmFloat: 7, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: 101, B: 101, ImmFloat: 3, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: 102, B: 102, ImmFloat: 7, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPAddF, A: 100, B: 101, C: 103, Type: EvFloat}, // fold -> 10
					{Op: qc.OPEqF, A: 100, B: 102, C: 104, Type: EvFloat},  // fold -> 1
					{Op: qc.OPEqF, A: 100, B: 101, C: 105, Type: EvFloat},  // fold -> 0
					{Op: qc.OPNeF, A: 100, B: 101, C: 106, Type: EvFloat},  // fold -> 1
					{Op: qc.OPNeF, A: 100, B: 102, C: 107, Type: EvFloat},  // fold -> 0
					{Op: qc.OPLT, A: 101, B: 100, C: 108, Type: EvFloat},   // fold -> 1
					{Op: qc.OPLT, A: 100, B: 101, C: 109, Type: EvFloat},   // fold -> 0
					{Op: qc.OPLE, A: 101, B: 100, C: 110, Type: EvFloat},   // fold -> 1
					{Op: qc.OPLE, A: 100, B: 101, C: 111, Type: EvFloat},   // fold -> 0
					{Op: qc.OPGT, A: 100, B: 101, C: 112, Type: EvFloat},   // fold -> 1
					{Op: qc.OPGT, A: 101, B: 100, C: 113, Type: EvFloat},   // fold -> 0
					{Op: qc.OPGE, A: 100, B: 101, C: 114, Type: EvFloat},   // fold -> 1
					{Op: qc.OPGE, A: 101, B: 100, C: 115, Type: EvFloat},   // fold -> 0
					{Op: qc.OPStoreF, A: 116, B: 116, Type: EvFloat},       // removable self-store
				},
			},
		},
	}

	optimizeIRProgram(prog)

	body := prog.Functions[0].Body
	if len(body) != 16 {
		t.Fatalf("body len = %d, want 16", len(body))
	}

	if body[3].Op != qc.OPStoreF || body[3].B != 103 || body[3].ImmFloat != 10 || !body[3].HasImmFloat {
		t.Fatalf("inst[3] = %+v, want folded OPStoreF to vreg 103 with ImmFloat=10", body[3])
	}

	want := []struct {
		idx int
		reg VReg
		val float64
	}{
		{4, 104, 1},
		{5, 105, 0},
		{6, 106, 1},
		{7, 107, 0},
		{8, 108, 1},
		{9, 109, 0},
		{10, 110, 1},
		{11, 111, 0},
		{12, 112, 1},
		{13, 113, 0},
		{14, 114, 1},
		{15, 115, 0},
	}

	for _, tc := range want {
		inst := body[tc.idx]
		if inst.Op != qc.OPStoreF || inst.B != tc.reg || inst.ImmFloat != tc.val || !inst.HasImmFloat {
			t.Fatalf("inst[%d] = %+v, want folded OPStoreF to vreg %d with ImmFloat=%v", tc.idx, inst, tc.reg, tc.val)
		}
	}
}

func TestOptimizeIRProgram_FoldsArithmeticChains(t *testing.T) {
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, B: 100, ImmFloat: 2, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, B: 101, ImmFloat: 3, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPAddF, A: 100, B: 101, C: 103, Type: EvFloat}, // fold -> 5
					{Op: qc.OPMulF, A: 103, B: 101, C: 104, Type: EvFloat}, // fold -> 15
					{Op: qc.OPSubF, A: 104, B: 100, C: 105, Type: EvFloat}, // fold -> 13
					{Op: qc.OPStoreF, A: 105, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	var sawAdd, sawMul, sawSub bool
	var sawReturnStore bool
	for _, inst := range body {
		if inst.Op == qc.OPAddF {
			sawAdd = true
		}
		if inst.Op == qc.OPMulF {
			sawMul = true
		}
		if inst.Op == qc.OPSubF {
			sawSub = true
		}
		if inst.Op == qc.OPStoreF && inst.B == VReg(qc.OFSReturn) && inst.A == 105 {
			sawReturnStore = true
		}
	}
	if sawAdd || sawMul || sawSub {
		t.Fatalf("expected arithmetic chain to fold to immediates, got body: %+v", body)
	}
	if !sawReturnStore {
		t.Fatalf("expected return store to use folded chain result vreg 105: %+v", body)
	}
}

func TestOptimizeIRProgram_DoesNotFoldUnaryNot(t *testing.T) {
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, B: 100, ImmFloat: 7, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPNotF, A: 100, C: 101, Type: EvFloat}, // unary not intentionally out of scope
					{Op: qc.OPStoreF, A: 101, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body
	for _, inst := range body {
		if inst.Op == qc.OPNotF && inst.C == 101 {
			return
		}
	}
	t.Fatalf("expected OPNotF to remain unfurled: %+v", body)
}

func TestOptimizeIRProgram_EliminatesDeadVirtualStores(t *testing.T) {
	vA := vregBase + 1
	vB := vregBase + 2
	vC := vregBase + 3
	vD := vregBase + 4
	vE := vregBase + 5
	vF := vregBase + 6
	vAddr := vregBase + 7

	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, B: vA, ImmFloat: 5, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, B: vB, ImmFloat: 7, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPAddF, A: vA, B: vB, C: vC, Type: EvFloat}, // dead, no side effects
					{Op: qc.OPStoreF, A: vC, B: vD, Type: EvFloat},      // dead, result unused
					{Op: qc.OPStoreF, B: vE, ImmFloat: 3, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPAddress, A: qc.OFSSelf, B: 0, C: vAddr, Type: EvPointer},      // live (feeds storep)
					{Op: qc.OPStorePF, A: vE, B: vAddr, Type: EvFloat},                      // side effect: must keep
					{Op: qc.OPStoreF, B: vF, ImmFloat: 9, HasImmFloat: true, Type: EvFloat}, // dead tail def
					{Op: qc.OPDone},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	if len(body) != 4 {
		t.Fatalf("body len = %d, want 4", len(body))
	}
	if body[0].Op != qc.OPStoreF || body[0].B != vE || body[0].ImmFloat != 3 || !body[0].HasImmFloat {
		t.Fatalf("inst[0] = %+v, want immediate store for vE", body[0])
	}
	if body[1].Op != qc.OPAddress || body[1].C != vAddr {
		t.Fatalf("inst[1] = %+v, want OPAddress defining vAddr", body[1])
	}
	if body[2].Op != qc.OPStorePF || body[2].A != vE || body[2].B != vAddr {
		t.Fatalf("inst[2] = %+v, want OPStorePF vE -> vAddr", body[2])
	}
	if body[3].Op != qc.OPDone {
		t.Fatalf("inst[3] = %+v, want OPDone", body[3])
	}

	for _, inst := range body {
		if (inst.Op == qc.OPAddF && inst.C == vC) ||
			(inst.Op == qc.OPStoreF && inst.B == vD) ||
			(inst.Op == qc.OPStoreF && inst.B == vF) ||
			(inst.Op == qc.OPStoreF && inst.B == vA) ||
			(inst.Op == qc.OPStoreF && inst.B == vB) {
			t.Fatalf("dead instruction still present: %+v", inst)
		}
	}
}

func TestOptimizeIRProgram_DCEHandlesSimpleControlFlow(t *testing.T) {
	vDead := vregBase + 10
	vCond := vregBase + 11
	vKeep := vregBase + 12
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, B: vDead, ImmFloat: 11, HasImmFloat: true, Type: EvFloat},          // dead across both paths
					{Op: qc.OPAddF, A: VReg(qc.OFSParm0), B: VReg(qc.OFSParm1), C: vCond, Type: EvFloat}, // dynamic branch condition
					{Op: qc.OPIF, A: vCond, Label: "then"},
					{Op: qc.OPStoreF, B: vKeep, ImmFloat: 2, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPGoto, Label: "done"},
					LabelInst("start"),
					LabelInst("then"),
					{Op: qc.OPStoreF, B: vKeep, ImmFloat: 3, HasImmFloat: true, Type: EvFloat},
					LabelInst("done"),
					{Op: qc.OPStoreF, A: vKeep, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	for _, inst := range body {
		if inst.Op == qc.OPStoreF && inst.B == vDead {
			t.Fatalf("dead control-flow store still present: %+v", inst)
		}
	}

	var sawIf, sawThenStore, sawElseStore, sawReturn bool
	for _, inst := range body {
		if inst.Op == qc.OPIF && inst.A == vCond && inst.Label == "then" {
			sawIf = true
		}
		if inst.Op == qc.OPStoreF && inst.B == vKeep && inst.ImmFloat == 2 && inst.HasImmFloat {
			sawElseStore = true
		}
		if inst.Op == qc.OPStoreF && inst.B == vKeep && inst.ImmFloat == 3 && inst.HasImmFloat {
			sawThenStore = true
		}
		if inst.Op == qc.OPReturn && inst.A == VReg(qc.OFSReturn) {
			sawReturn = true
		}
	}
	if !sawIf || !sawThenStore || !sawElseStore || !sawReturn {
		t.Fatalf("missing expected control-flow ops after DCE: %+v", body)
	}
}

func TestOptimizeIRProgram_PrunesUnusedLocalsAfterDCE(t *testing.T) {
	vKeep := vregBase + 20
	vDead := vregBase + 21
	vParam := vregBase + 22
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name:   "main",
				Params: []IRParam{{Name: "p", Type: EvFloat}},
				Locals: []IRLocal{
					{Name: "p", Type: EvFloat, VReg: vParam},
					{Name: "keep", Type: EvFloat, VReg: vKeep},
					{Name: "dead", Type: EvFloat, VReg: vDead},
				},
				Body: []IRInst{
					{Op: qc.OPStoreF, B: vKeep, ImmFloat: 3, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, B: vDead, ImmFloat: 7, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: vKeep, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)

	fn := prog.Functions[0]
	if len(fn.Locals) != 2 {
		t.Fatalf("locals len = %d, want 2", len(fn.Locals))
	}
	if fn.Locals[0].VReg != vParam || fn.Locals[1].VReg != vKeep {
		t.Fatalf("locals = %+v, want param+keep locals", fn.Locals)
	}
	for _, inst := range fn.Body {
		if inst.Op == qc.OPStoreF && inst.B == vDead {
			t.Fatalf("dead temp store still present: %+v", inst)
		}
	}
}

func TestOptimizeIRProgram_PrunesUnreachableBlocksAfterTerminator(t *testing.T) {
	vLive := vregBase + 30
	vUnreachable := vregBase + 31

	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, B: vLive, ImmFloat: 4, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: vLive, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
					LabelInst("dead"),
					{Op: qc.OPStoreF, B: vUnreachable, ImmFloat: 9, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: vUnreachable, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	if len(body) != 3 {
		t.Fatalf("body len = %d, want 3", len(body))
	}
	if body[0].Op != qc.OPStoreF || body[0].B != vLive || body[0].ImmFloat != 4 || !body[0].HasImmFloat {
		t.Fatalf("inst[0] = %+v, want reachable immediate store", body[0])
	}
	if body[1].Op != qc.OPStoreF || body[1].A != vLive || body[1].B != VReg(qc.OFSReturn) {
		t.Fatalf("inst[1] = %+v, want reachable return store", body[1])
	}
	if body[2].Op != qc.OPReturn {
		t.Fatalf("inst[2] = %+v, want reachable return", body[2])
	}

	for _, inst := range body {
		if inst.IsLabel() || (inst.Op == qc.OPStoreF && inst.B == vUnreachable) {
			t.Fatalf("unreachable block instruction still present: %+v", inst)
		}
	}
}

func TestOptimizeIRProgram_PrunesConstConditionBranches(t *testing.T) {
	vFalse := vregBase + 40
	vTrue := vregBase + 41
	vCondDynamic := vregBase + 42

	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, B: vFalse, ImmFloat: 0, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPIF, A: vFalse, Label: "never"}, // prune: never taken
					{Op: qc.OPStoreF, B: vFalse, ImmFloat: 0, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPIFNot, A: vFalse, Label: "take1"}, // prune -> goto
					LabelInst("take1"),
					{Op: qc.OPStoreF, B: vTrue, ImmFloat: 1, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPIF, A: vTrue, Label: "take2"}, // prune -> goto
					{Op: qc.OPStoreF, B: vTrue, ImmFloat: 1, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPIFNot, A: vTrue, Label: "never2"}, // prune: never taken
					LabelInst("take2"),
					{Op: qc.OPAddF, A: VReg(qc.OFSParm0), B: VReg(qc.OFSParm1), C: vCondDynamic, Type: EvFloat},
					{Op: qc.OPIFNot, A: vCondDynamic, Label: "dynamic"}, // must stay conditional
					LabelInst("dynamic"),
					{Op: qc.OPDone},
					LabelInst("never"),
					LabelInst("never2"),
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	var hasIfFalse, hasIfNotTrue, hasGotoTake1, hasGotoTake2, hasDynamicIfNot bool
	for _, inst := range body {
		if inst.Op == qc.OPIF && inst.A == vFalse && inst.Label == "never" {
			hasIfFalse = true
		}
		if inst.Op == qc.OPIFNot && inst.A == vTrue && inst.Label == "never2" {
			hasIfNotTrue = true
		}
		if inst.Op == qc.OPGoto && inst.Label == "take1" {
			hasGotoTake1 = true
		}
		if inst.Op == qc.OPGoto && inst.Label == "take2" {
			hasGotoTake2 = true
		}
		if inst.Op == qc.OPIFNot && inst.A == vCondDynamic && inst.Label == "dynamic" {
			hasDynamicIfNot = true
		}
	}

	if hasIfFalse {
		t.Fatalf("constant-false OPIF should be pruned: %+v", body)
	}
	if hasIfNotTrue {
		t.Fatalf("constant-true OPIFNot should be pruned: %+v", body)
	}
	if !hasGotoTake1 || !hasGotoTake2 {
		t.Fatalf("expected taken constant branches rewritten to GOTO: %+v", body)
	}
	if !hasDynamicIfNot {
		t.Fatalf("expected dynamic OPIFNot to remain conditional: %+v", body)
	}
}

func TestOptimizeIRProgram_PropagatesLocalCopiesInStraightLine(t *testing.T) {
	vSrc := vregBase + 50
	vTmp := vregBase + 51
	vRes := vregBase + 52

	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Locals: []IRLocal{
					{Name: "src", Type: EvFloat, VReg: vSrc},
					{Name: "tmp", Type: EvFloat, VReg: vTmp},
					{Name: "res", Type: EvFloat, VReg: vRes},
				},
				Body: []IRInst{
					{Op: qc.OPStoreF, B: vSrc, ImmFloat: 4, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: vSrc, B: vTmp, Type: EvFloat}, // tmp = src
					{Op: qc.OPAddF, A: vTmp, B: VReg(qc.OFSParm0), C: vRes, Type: EvFloat},
					{Op: qc.OPStoreF, A: vRes, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	var sawAdd bool
	for _, inst := range body {
		if inst.Op == qc.OPAddF && inst.C == vRes {
			sawAdd = true
			if inst.A != vSrc {
				t.Fatalf("expected copy-propagated add source %d, got %d (%+v)", vSrc, inst.A, inst)
			}
		}
	}
	if !sawAdd {
		t.Fatalf("expected OPAddF in optimized body: %+v", body)
	}
}

func TestOptimizeIRProgram_CopyPropagationStopsOnInterveningWrite(t *testing.T) {
	vSrc := vregBase + 60
	vTmp := vregBase + 61
	vRes := vregBase + 62

	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Locals: []IRLocal{
					{Name: "src", Type: EvFloat, VReg: vSrc},
					{Name: "tmp", Type: EvFloat, VReg: vTmp},
					{Name: "res", Type: EvFloat, VReg: vRes},
				},
				Body: []IRInst{
					{Op: qc.OPStoreF, B: vSrc, ImmFloat: 5, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: vSrc, B: vTmp, Type: EvFloat},                        // tmp = src
					{Op: qc.OPStoreF, B: vSrc, ImmFloat: 9, HasImmFloat: true, Type: EvFloat}, // write src invalidates tmp alias
					{Op: qc.OPAddF, A: vTmp, B: VReg(qc.OFSParm0), C: vRes, Type: EvFloat},
					{Op: qc.OPStoreF, A: vRes, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	for _, inst := range body {
		if inst.Op == qc.OPAddF && inst.C == vRes {
			if inst.A != vTmp {
				t.Fatalf("expected add to keep tmp after source write, got %+v", inst)
			}
			return
		}
	}
	t.Fatalf("expected OPAddF in optimized body: %+v", body)
}

func TestOptimizeIRProgram_CopyPropagationStopsAtLabels(t *testing.T) {
	vSrc := vregBase + 70
	vTmp := vregBase + 71
	vRes := vregBase + 72

	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Locals: []IRLocal{
					{Name: "src", Type: EvFloat, VReg: vSrc},
					{Name: "tmp", Type: EvFloat, VReg: vTmp},
					{Name: "res", Type: EvFloat, VReg: vRes},
				},
				Body: []IRInst{
					{Op: qc.OPStoreF, A: vSrc, B: vTmp, Type: EvFloat}, // tmp = src
					LabelInst("join"), // new block boundary clears alias state
					{Op: qc.OPAddF, A: vTmp, B: VReg(qc.OFSParm0), C: vRes, Type: EvFloat},
					{Op: qc.OPStoreF, A: vRes, B: VReg(qc.OFSReturn), Type: EvFloat},
					{Op: qc.OPReturn, A: VReg(qc.OFSReturn)},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body

	for _, inst := range body {
		if inst.Op == qc.OPAddF && inst.C == vRes {
			if inst.A != vTmp {
				t.Fatalf("expected add to keep tmp across label boundary, got %+v", inst)
			}
			return
		}
	}
	t.Fatalf("expected OPAddF in optimized body: %+v", body)
}
