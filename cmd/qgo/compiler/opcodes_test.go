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
					{Op: qc.OPStoreF, B: 12, ImmFloat: 5},                       // keep immediate const
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
