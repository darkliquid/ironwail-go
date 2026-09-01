package dap

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

type mockTarget struct {
	vm *qc.VM
}

func (m *mockTarget) VM() *qc.VM { return m.vm }
func (m *mockTarget) EdictCount() int { return 2 }
func (m *mockTarget) GetEdictFloat(entNum, offset int) float32 {
	if offset == qc.EntFieldHealth {
		return 100.0
	}
	return 0
}
func (m *mockTarget) GetEdictString(entNum, offset int) string {
	if entNum == 0 && offset == qc.EntFieldClassName {
		return "worldspawn"
	}
	if entNum == 1 && offset == qc.EntFieldClassName {
		return "player"
	}
	return ""
}
func (m *mockTarget) GetEdictVector(entNum, offset int) [3]float32 {
	if offset == qc.EntFieldOrigin {
		return [3]float32{10, 20, 30}
	}
	return [3]float32{}
}
func (m *mockTarget) GetEdictClassName(entNum int) string {
	return m.GetEdictString(entNum, qc.EntFieldClassName)
}
func (m *mockTarget) FieldNames() map[string]int {
	return map[string]int{
		"classname": qc.EntFieldClassName,
		"origin":    qc.EntFieldOrigin,
		"health":    qc.EntFieldHealth,
	}
}

func TestResolveScopesAndVariables(t *testing.T) {
	vm := qc.NewVM()
	vm.Globals = make([]float32, 100)
	vm.SetGInt(qc.OFSSelf, 1)
	vm.SetGInt(qc.OFSOther, 0)
	vm.Globals[qc.OFSTime] = 12.5

	target := &mockTarget{vm: vm}
	mgr := NewVariableManager(target)

	scopes := mgr.GetScopes(0)
	if len(scopes) != 3 {
		t.Fatalf("Expected 3 scopes, got %d", len(scopes))
	}

	// Locals
	locals := mgr.GetVariables(scopes[0].VariablesReference)
	if len(locals) == 0 {
		t.Fatal("Expected local variables")
	}

	// Globals
	globals := mgr.GetVariables(scopes[1].VariablesReference)
	foundTime := false
	for _, g := range globals {
		if g.Name == "time" && g.Value == "12.5" {
			foundTime = true
		}
	}
	if !foundTime {
		t.Fatalf("Expected global 'time=12.5' in globals: %+v", globals)
	}

	// Edicts scope
	edicts := mgr.GetVariables(scopes[2].VariablesReference)
	if len(edicts) != 2 {
		t.Fatalf("Expected 2 edicts, got %d", len(edicts))
	}

	// Inspect specific edict fields
	fields := mgr.GetVariables(edicts[1].VariablesReference)
	foundOrigin := false
	for _, f := range fields {
		if f.Name == "origin" && strings.Contains(f.Value, "10") {
			foundOrigin = true
		}
	}
	if !foundOrigin {
		t.Fatalf("Expected 'origin' field on player edict: %+v", fields)
	}
}

func TestEvaluate(t *testing.T) {
	vm := qc.NewVM()
	vm.Globals = make([]float32, 100)
	vm.SetGInt(qc.OFSSelf, 1)
	vm.SetGInt(qc.OFSOther, 0)
	vm.Globals[qc.OFSTime] = 12.5

	target := &mockTarget{vm: vm}
	mgr := NewVariableManager(target)

	val, err := mgr.Evaluate("time")
	if err != nil || val != "12.5" {
		t.Fatalf("Evaluate(time) = (%q, %v), want (\"12.5\", nil)", val, err)
	}

	val, err = mgr.Evaluate("self")
	if err != nil || !strings.Contains(val, "player") {
		t.Fatalf("Evaluate(self) = (%q, %v), want containing 'player'", val, err)
	}

	val, err = mgr.Evaluate("other")
	if err != nil || !strings.Contains(val, "worldspawn") {
		t.Fatalf("Evaluate(other) = (%q, %v), want containing 'worldspawn'", val, err)
	}

	val, err = mgr.Evaluate("self.health")
	if err != nil || val != "100" {
		t.Fatalf("Evaluate(self.health) = (%q, %v), want (\"100\", nil)", val, err)
	}

	_, err = mgr.Evaluate("unknown_var")
	if err == nil {
		t.Fatalf("Evaluate(unknown_var) expected error, got nil")
	}
}
