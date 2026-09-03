package dap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

type mockTarget struct {
	vm *qc.VM
}

func (m *mockTarget) VM() *qc.VM      { return m.vm }
func (m *mockTarget) EdictCount() int { return 2 }
func (m *mockTarget) GetEdictFloat(entNum, offset int) float32 {
	if entNum < 0 || entNum >= m.EdictCount() {
		return 0
	}
	if offset == qc.EntFieldHealth {
		return 100.0
	}
	return 0
}
func (m *mockTarget) GetEdictString(entNum, offset int) string {
	if entNum < 0 || entNum >= m.EdictCount() {
		return ""
	}
	if entNum == 0 && offset == qc.EntFieldClassName {
		return "worldspawn"
	}
	if entNum == 1 && offset == qc.EntFieldClassName {
		return "player"
	}
	if entNum == 1 && offset == qc.EntFieldModel {
		return "progs/player.mdl"
	}
	return ""
}
func (m *mockTarget) GetEdictVector(entNum, offset int) [3]float32 {
	if entNum < 0 || entNum >= m.EdictCount() {
		return [3]float32{}
	}
	if offset == qc.EntFieldOrigin {
		return [3]float32{10, 20, 30}
	}
	if offset == qc.EntFieldAngles {
		return [3]float32{0, 90, 0}
	}
	if offset == qc.EntFieldVelocity {
		return [3]float32{100, 0, 0}
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
		"angles":    qc.EntFieldAngles,
		"velocity":  qc.EntFieldVelocity,
		"model":     qc.EntFieldModel,
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

func TestVariablesFunctionParameters(t *testing.T) {
	vm := qc.NewVM()
	vm.Globals = make([]float32, 100)
	fn := &qc.DFunction{
		ParmStart: 10,
		NumParms:  3,
		ParmSize:  [qc.MaxParms]byte{1, 3, 1},
	}
	vm.XFunction = fn
	vm.Globals[10] = 42.0 // parm0 (size 1)
	vm.Globals[11] = 99.0 // parm1 (size 3, starts at 11)
	vm.Globals[14] = 7.5  // parm2 (size 1, starts at 14)

	target := &mockTarget{vm: vm}
	mgr := NewVariableManager(target)

	vars := mgr.GetVariables(scopeLocalsBase)
	var parm0, parm1, parm2 *Variable
	for i := range vars {
		switch vars[i].Name {
		case "parm0":
			parm0 = &vars[i]
		case "parm1":
			parm1 = &vars[i]
		case "parm2":
			parm2 = &vars[i]
		}
	}

	if parm0 == nil || parm0.Value != "42" {
		t.Fatalf("Expected parm0 = 42, got %+v", parm0)
	}
	if parm1 == nil || parm1.Value != "99" {
		t.Fatalf("Expected parm1 = 99, got %+v", parm1)
	}
	if parm2 == nil || parm2.Value != "7.5" {
		t.Fatalf("Expected parm2 = 7.5, got %+v", parm2)
	}

	// Test NumParms exceeding len(ParmSize) (guard test)
	fn.NumParms = 10
	varsExceed := mgr.GetVariables(scopeLocalsBase)
	if len(varsExceed) == 0 {
		t.Fatal("Expected variables without panic when NumParms exceeds ParmSize capacity")
	}
}

func TestVariablesSortedFieldOrder(t *testing.T) {
	vm := qc.NewVM()
	target := &mockTarget{vm: vm}
	mgr := NewVariableManager(target)

	fields := mgr.GetVariables(edictFieldsBase + 1)
	var names []string
	for _, f := range fields {
		names = append(names, f.Name)
	}

	expected := []string{"angles", "classname", "health", "model", "origin", "velocity"}
	if !reflect.DeepEqual(names, expected) {
		t.Fatalf("Fields not sorted deterministically: got %v, want %v", names, expected)
	}
}

func TestVariablesBoundsChecks(t *testing.T) {
	vm := qc.NewVM()
	vm.Globals = make([]float32, 100)
	target := &mockTarget{vm: vm}
	mgr := NewVariableManager(target)

	// Ref out of bounds for edicts
	if vars := mgr.GetVariables(edictFieldsBase + 999); vars != nil {
		t.Fatalf("Expected nil for out-of-bounds edict ref, got %+v", vars)
	}
	if vars := mgr.GetVariables(-1); vars != nil {
		t.Fatalf("Expected nil for negative ref, got %+v", vars)
	}
	if vars := mgr.GetVariables(500); vars != nil {
		t.Fatalf("Expected nil for unrecognized ref, got %+v", vars)
	}

	// Out of bounds self / other entNum in globals
	vm.SetGInt(qc.OFSSelf, 999)
	vm.SetGInt(qc.OFSOther, -5)

	locals := mgr.GetVariables(scopeLocalsBase)
	var foundSelf, foundOther bool
	for _, v := range locals {
		if v.Name == "self" && v.Value == "edict 999 ()" {
			foundSelf = true
		}
		if v.Name == "other" && v.Value == "edict -5 ()" {
			foundOther = true
		}
	}
	if !foundSelf || !foundOther {
		t.Fatalf("Expected safe out-of-bounds class name resolution for self/other, got locals: %+v", locals)
	}

	// Evaluate with out of bounds self
	val, err := mgr.Evaluate("self")
	if err != nil || val != "edict 999 ()" {
		t.Fatalf("Evaluate(self) with invalid entNum = (%q, %v), want (\"edict 999 ()\", nil)", val, err)
	}

	val, err = mgr.Evaluate("other")
	if err != nil || val != "edict -5 ()" {
		t.Fatalf("Evaluate(other) with invalid entNum = (%q, %v), want (\"edict -5 ()\", nil)", val, err)
	}

	_, err = mgr.Evaluate("self.origin")
	if err == nil {
		t.Fatal("Evaluate(self.origin) expected error for invalid self entity, got nil")
	}
}

func TestVariablesNilHandling(t *testing.T) {
	// Nil manager
	var nilMgr *VariableManager
	if scopes := nilMgr.GetVariables(scopeLocalsBase); scopes != nil {
		t.Fatalf("Expected nil for nil VariableManager, got %+v", scopes)
	}
	if _, err := nilMgr.Evaluate("time"); err == nil {
		t.Fatal("Expected error evaluating with nil VariableManager")
	}

	// Nil target
	mgrNilTarget := NewVariableManager(nil)
	if scopes := mgrNilTarget.GetScopes(0); len(scopes) != 3 {
		t.Fatalf("Expected 3 scopes even with nil target, got %d", len(scopes))
	}
	if vars := mgrNilTarget.GetVariables(scopeLocalsBase); vars != nil {
		t.Fatalf("Expected nil for nil target, got %+v", vars)
	}
	if _, err := mgrNilTarget.Evaluate("time"); err == nil {
		t.Fatal("Expected error evaluating with nil target")
	}

	// Nil VM on target
	mgrNilVM := NewVariableManager(&mockTarget{vm: nil})
	if vars := mgrNilVM.GetVariables(scopeLocalsBase); len(vars) != 0 {
		t.Fatalf("Expected empty locals for nil VM, got %+v", vars)
	}
	if vars := mgrNilVM.GetVariables(scopeGlobalsBase); len(vars) != 0 {
		t.Fatalf("Expected empty globals for nil VM, got %+v", vars)
	}
	if vars := mgrNilVM.GetVariables(scopeEdictsBase); len(vars) != 2 {
		t.Fatalf("Expected 2 edicts from target with nil VM, got %d", len(vars))
	}
	if _, err := mgrNilVM.Evaluate("time"); err == nil {
		t.Fatal("Expected error evaluating with nil VM")
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

	val, err = mgr.Evaluate("self.origin")
	if err != nil || val != "[10, 20, 30]" {
		t.Fatalf("Evaluate(self.origin) = (%q, %v), want (\"[10, 20, 30]\", nil)", val, err)
	}

	val, err = mgr.Evaluate("self.classname")
	if err != nil || val != "\"player\"" {
		t.Fatalf("Evaluate(self.classname) = (%q, %v), want (\"\\\"player\\\"\", nil)", val, err)
	}

	val, err = mgr.Evaluate("self.model")
	if err != nil || val != "\"progs/player.mdl\"" {
		t.Fatalf("Evaluate(self.model) = (%q, %v), want (\"\\\"progs/player.mdl\\\"\", nil)", val, err)
	}

	_, err = mgr.Evaluate("unknown_var")
	if err == nil {
		t.Fatalf("Evaluate(unknown_var) expected error, got nil")
	}
}
