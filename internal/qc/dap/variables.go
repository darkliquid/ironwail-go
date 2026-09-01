package dap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

const (
	scopeLocalsBase  = 1000
	scopeGlobalsBase = 2000
	scopeEdictsBase  = 3000
	edictFieldsBase  = 10000
)

// VariableManager maps DAP variablesReference IDs to hierarchical variables.
type VariableManager struct {
	target Target
}

// NewVariableManager creates a new VariableManager.
func NewVariableManager(target Target) *VariableManager {
	return &VariableManager{target: target}
}

// GetScopes returns the standard 3 scopes for a frame.
func (vm *VariableManager) GetScopes(frameID int) []Scope {
	return []Scope{
		{Name: "Locals", VariablesReference: scopeLocalsBase + frameID, Expensive: false},
		{Name: "Globals", VariablesReference: scopeGlobalsBase + frameID, Expensive: false},
		{Name: "Edicts", VariablesReference: scopeEdictsBase + frameID, Expensive: true},
	}
}

func formatEdictField(target Target, entNum int, name string, ofs int) (val string, varType string) {
	if strings.Contains(name, "origin") || strings.Contains(name, "velocity") || strings.Contains(name, "angles") {
		vec := target.GetEdictVector(entNum, ofs)
		return fmt.Sprintf("[%v, %v, %v]", vec[0], vec[1], vec[2]), "vector"
	}
	if name == "classname" || name == "model" || name == "target" || name == "targetname" {
		str := target.GetEdictString(entNum, ofs)
		return fmt.Sprintf("%q", str), "string"
	}
	fl := target.GetEdictFloat(entNum, ofs)
	return fmt.Sprintf("%v", fl), "float"
}

// GetVariables returns child variables for a given reference ID.
func (vm *VariableManager) GetVariables(ref int) []Variable {
	if vm == nil || vm.target == nil {
		return nil
	}
	qcvm := vm.target.VM()
	edictCount := vm.target.EdictCount()

	if ref >= scopeLocalsBase && ref < scopeGlobalsBase {
		// Locals
		var vars []Variable
		if qcvm != nil {
			if len(qcvm.Globals) > qc.OFSSelf {
				selfEnt := int(qcvm.GInt(qc.OFSSelf))
				var className string
				if selfEnt >= 0 && selfEnt < edictCount {
					className = vm.target.GetEdictClassName(selfEnt)
				}
				vars = append(vars, Variable{
					Name:  "self",
					Value: fmt.Sprintf("edict %d (%s)", selfEnt, className),
					Type:  "entity",
				})
			}
			if len(qcvm.Globals) > qc.OFSOther {
				otherEnt := int(qcvm.GInt(qc.OFSOther))
				var className string
				if otherEnt >= 0 && otherEnt < edictCount {
					className = vm.target.GetEdictClassName(otherEnt)
				}
				vars = append(vars, Variable{
					Name:  "other",
					Value: fmt.Sprintf("edict %d (%s)", otherEnt, className),
					Type:  "entity",
				})
			}
			if qcvm.XFunction != nil {
				parmOfs := int(qcvm.XFunction.ParmStart)
				for i := 0; i < int(qcvm.XFunction.NumParms); i++ {
					if parmOfs >= 0 && parmOfs < len(qcvm.Globals) {
						vars = append(vars, Variable{
							Name:  fmt.Sprintf("parm%d", i),
							Value: fmt.Sprintf("%v", qcvm.Globals[parmOfs]),
							Type:  "float",
						})
					}
					size := 1
					if i < len(qcvm.XFunction.ParmSize) && qcvm.XFunction.ParmSize[i] > 0 {
						size = int(qcvm.XFunction.ParmSize[i])
					}
					parmOfs += size
				}
			}
		}
		return vars
	}

	if ref >= scopeGlobalsBase && ref < scopeEdictsBase {
		// Globals
		var vars []Variable
		if qcvm != nil {
			if len(qcvm.Globals) > qc.OFSTime {
				vars = append(vars, Variable{Name: "time", Value: fmt.Sprintf("%v", qcvm.Globals[qc.OFSTime]), Type: "float"})
			}
			if len(qcvm.Globals) > qc.OFSSelf {
				vars = append(vars, Variable{Name: "self", Value: fmt.Sprintf("%d", qcvm.GInt(qc.OFSSelf)), Type: "entity"})
			}
			if len(qcvm.Globals) > qc.OFSOther {
				vars = append(vars, Variable{Name: "other", Value: fmt.Sprintf("%d", qcvm.GInt(qc.OFSOther)), Type: "entity"})
			}
			if len(qcvm.Globals) > qc.OFSWorld {
				vars = append(vars, Variable{Name: "world", Value: fmt.Sprintf("%d", qcvm.GInt(qc.OFSWorld)), Type: "entity"})
			}
		}
		return vars
	}

	if ref >= scopeEdictsBase && ref < edictFieldsBase {
		// Edicts summary list
		var vars []Variable
		for i := 0; i < edictCount; i++ {
			cname := vm.target.GetEdictClassName(i)
			if cname == "" && i > 0 {
				continue // Skip unused / free edicts
			}
			vars = append(vars, Variable{
				Name:               fmt.Sprintf("[%d]", i),
				Value:              cname,
				Type:               "entity",
				VariablesReference: edictFieldsBase + i,
			})
		}
		return vars
	}

	if ref >= edictFieldsBase {
		// Edict fields
		entNum := ref - edictFieldsBase
		if entNum < 0 || entNum >= edictCount {
			return nil
		}
		var vars []Variable
		fields := vm.target.FieldNames()
		if len(fields) > 0 {
			names := make([]string, 0, len(fields))
			for name := range fields {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				ofs := fields[name]
				val, varType := formatEdictField(vm.target, entNum, name, ofs)
				vars = append(vars, Variable{
					Name:  name,
					Value: val,
					Type:  varType,
				})
			}
		}
		return vars
	}

	return nil
}

// Evaluate evaluates an expression string against target state.
func (vm *VariableManager) Evaluate(expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	if vm == nil || vm.target == nil {
		return "", fmt.Errorf("no active target")
	}
	qcvm := vm.target.VM()
	if qcvm == nil {
		return "", fmt.Errorf("no active VM")
	}
	edictCount := vm.target.EdictCount()

	switch expr {
	case "time":
		if len(qcvm.Globals) > qc.OFSTime {
			return fmt.Sprintf("%v", qcvm.Globals[qc.OFSTime]), nil
		}
		return "0", nil
	case "self":
		if len(qcvm.Globals) > qc.OFSSelf {
			selfEnt := int(qcvm.GInt(qc.OFSSelf))
			var className string
			if selfEnt >= 0 && selfEnt < edictCount {
				className = vm.target.GetEdictClassName(selfEnt)
			}
			return fmt.Sprintf("edict %d (%s)", selfEnt, className), nil
		}
		return "edict 0 ()", nil
	case "other":
		if len(qcvm.Globals) > qc.OFSOther {
			otherEnt := int(qcvm.GInt(qc.OFSOther))
			var className string
			if otherEnt >= 0 && otherEnt < edictCount {
				className = vm.target.GetEdictClassName(otherEnt)
			}
			return fmt.Sprintf("edict %d (%s)", otherEnt, className), nil
		}
		return "edict 0 ()", nil
	}

	if strings.HasPrefix(expr, "self.") {
		field := strings.TrimPrefix(expr, "self.")
		if len(qcvm.Globals) > qc.OFSSelf {
			entNum := int(qcvm.GInt(qc.OFSSelf))
			if entNum < 0 || entNum >= edictCount {
				return "", fmt.Errorf("invalid self entity %d", entNum)
			}
			fields := vm.target.FieldNames()
			if ofs, ok := fields[field]; ok {
				val, _ := formatEdictField(vm.target, entNum, field, ofs)
				return val, nil
			}
		}
	}

	return "", fmt.Errorf("unknown expression: %s", expr)
}
