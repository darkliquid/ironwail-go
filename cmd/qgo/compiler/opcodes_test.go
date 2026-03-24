package compiler

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/qc"
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

func TestOptimizeIRProgram_FoldsConstFloatOps(t *testing.T) {
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					{Op: qc.OPStoreF, A: 100, B: 100, ImmFloat: 7, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPStoreF, A: 101, B: 101, ImmFloat: 3, HasImmFloat: true, Type: EvFloat},
					{Op: qc.OPAddF, A: 100, B: 101, C: 102, Type: EvFloat}, // fold -> 10
					{Op: qc.OPNotF, A: 101, C: 103, Type: EvFloat},         // fold -> 0
					{Op: qc.OPStoreF, A: 102, B: 104, Type: EvFloat},       // propagate 10
					{Op: qc.OPMulF, A: 104, B: 101, C: 105, Type: EvFloat}, // fold -> 30
					{Op: qc.OPStoreF, A: 106, B: 106, Type: EvFloat},       // removable self-store
				},
			},
		},
	}

	optimizeIRProgram(prog)

	body := prog.Functions[0].Body
	if len(body) != 6 {
		t.Fatalf("body len = %d, want 6", len(body))
	}

	if body[2].Op != qc.OPStoreF || body[2].B != 102 || body[2].ImmFloat != 10 || !body[2].HasImmFloat {
		t.Fatalf("inst[2] = %+v, want folded OPStoreF to vreg 102 with ImmFloat=10", body[2])
	}

	if body[3].Op != qc.OPStoreF || body[3].B != 103 || body[3].ImmFloat != 0 || !body[3].HasImmFloat {
		t.Fatalf("inst[3] = %+v, want folded OPStoreF to vreg 103 with ImmFloat=0", body[3])
	}

	if body[5].Op != qc.OPStoreF || body[5].B != 105 || body[5].ImmFloat != 30 || !body[5].HasImmFloat {
		t.Fatalf("inst[5] = %+v, want folded OPStoreF to vreg 105 with ImmFloat=30", body[5])
	}
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

func TestOptimizeIRProgram_DCESkipsControlFlowFunctions(t *testing.T) {
	vTmp := vregBase + 10
	prog := &IRProgram{
		Functions: []IRFunc{
			{
				Name: "main",
				Body: []IRInst{
					LabelInst("start"),
					{Op: qc.OPStoreF, B: vTmp, ImmFloat: 11, HasImmFloat: true, Type: EvFloat}, // dead if linear, should be kept when control-flow exists
					{Op: qc.OPGoto, Label: "start"},
				},
			},
		},
	}

	optimizeIRProgram(prog)
	body := prog.Functions[0].Body
	if len(body) != 3 {
		t.Fatalf("body len = %d, want 3 (DCE should skip control-flow functions)", len(body))
	}
	if !body[0].IsLabel() || body[1].Op != qc.OPStoreF || body[2].Op != qc.OPGoto {
		t.Fatalf("unexpected body after optimize: %+v", body)
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
