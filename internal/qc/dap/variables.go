package dap

import (
	"fmt"
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

// GetVariables returns child variables for a given reference ID.
func (vm *VariableManager) GetVariables(ref int) []Variable {
	if vm.target == nil {
		return nil
	}
	qcvm := vm.target.VM()

	if ref >= scopeLocalsBase && ref < scopeGlobalsBase {
		// Locals
		var vars []Variable
		if qcvm != nil {
			if len(qcvm.Globals) > qc.OFSSelf {
				selfEnt := int(qcvm.GInt(qc.OFSSelf))
				vars = append(vars, Variable{
					Name:  "self",
					Value: fmt.Sprintf("edict %d (%s)", selfEnt, vm.target.GetEdictClassName(selfEnt)),
					Type:  "entity",
				})
			}
			if len(qcvm.Globals) > qc.OFSOther {
				otherEnt := int(qcvm.GInt(qc.OFSOther))
				vars = append(vars, Variable{
					Name:  "other",
					Value: fmt.Sprintf("edict %d (%s)", otherEnt, vm.target.GetEdictClassName(otherEnt)),
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
					size := int(qcvm.XFunction.ParmSize[i])
					if size <= 0 {
						size = 1
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
		count := vm.target.EdictCount()
		for i := 0; i < count; i++ {
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
		var vars []Variable
		fields := vm.target.FieldNames()
		for name, ofs := range fields {
			if strings.Contains(name, "origin") || strings.Contains(name, "velocity") || strings.Contains(name, "angles") {
				vec := vm.target.GetEdictVector(entNum, ofs)
				vars = append(vars, Variable{
					Name:  name,
					Value: fmt.Sprintf("[%v, %v, %v]", vec[0], vec[1], vec[2]),
					Type:  "vector",
				})
			} else if name == "classname" || name == "model" || name == "target" || name == "targetname" {
				str := vm.target.GetEdictString(entNum, ofs)
				vars = append(vars, Variable{
					Name:  name,
					Value: fmt.Sprintf("%q", str),
					Type:  "string",
				})
			} else {
				fl := vm.target.GetEdictFloat(entNum, ofs)
				vars = append(vars, Variable{
					Name:  name,
					Value: fmt.Sprintf("%v", fl),
					Type:  "float",
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
	if vm.target == nil {
		return "", fmt.Errorf("no active target")
	}
	qcvm := vm.target.VM()
	if qcvm == nil {
		return "", fmt.Errorf("no active VM")
	}

	switch expr {
	case "time":
		if len(qcvm.Globals) > qc.OFSTime {
			return fmt.Sprintf("%v", qcvm.Globals[qc.OFSTime]), nil
		}
		return "0", nil
	case "self":
		if len(qcvm.Globals) > qc.OFSSelf {
			selfEnt := int(qcvm.GInt(qc.OFSSelf))
			return fmt.Sprintf("edict %d (%s)", selfEnt, vm.target.GetEdictClassName(selfEnt)), nil
		}
		return "edict 0 ()", nil
	case "other":
		if len(qcvm.Globals) > qc.OFSOther {
			otherEnt := int(qcvm.GInt(qc.OFSOther))
			return fmt.Sprintf("edict %d (%s)", otherEnt, vm.target.GetEdictClassName(otherEnt)), nil
		}
		return "edict 0 ()", nil
	}

	if strings.HasPrefix(expr, "self.") {
		field := strings.TrimPrefix(expr, "self.")
		if len(qcvm.Globals) > qc.OFSSelf {
			entNum := int(qcvm.GInt(qc.OFSSelf))
			if ofs, ok := vm.target.FieldNames()[field]; ok {
				return fmt.Sprintf("%v", vm.target.GetEdictFloat(entNum, ofs)), nil
			}
		}
	}

	return "", fmt.Errorf("unknown expression: %s", expr)
}
