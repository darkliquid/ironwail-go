// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestLoadMapEntitiesPopulatesQCVM(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 16)
	s.ClearWorld()

	vm.FieldDefs = []qc.DDef{
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldClassName), Name: vm.AllocString("classname")},
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldTarget), Name: vm.AllocString("target")},
		{Type: uint16(qc.EvString), Ofs: uint16(qc.EntFieldTargetName), Name: vm.AllocString("targetname")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("worldspawn"), FirstStatement: 0},
		{Name: vm.AllocString("func_button"), FirstStatement: 1},
		{Name: vm.AllocString("func_door"), FirstStatement: 2},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

	raw := `{
"classname" "worldspawn"
}
{
"classname" "func_button"
"target" "door1"
}
{
"classname" "func_door"
"targetname" "door1"
}`

	if err := s.loadMapEntities(raw); err != nil {
		t.Fatalf("loadMapEntities error: %v", err)
	}

	btnTarget := vm.String(vm.EString(1, qc.EntFieldTarget))
	if btnTarget != "door1" {
		t.Errorf("func_button target in QCVM = %q, want %q", btnTarget, "door1")
	}

	doorTargetName := vm.String(vm.EString(2, qc.EntFieldTargetName))
	if doorTargetName != "door1" {
		t.Errorf("func_door targetname in QCVM = %q, want %q", doorTargetName, "door1")
	}
}
