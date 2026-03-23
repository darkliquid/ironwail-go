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
