package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// Helper methods

func (l *Lowerer) allocVReg() VReg {
	v := l.nextVReg
	l.nextVReg++
	return v
}

func (l *Lowerer) constFloat(fn *IRFunc, val float64) VReg {
	if v, ok := l.constFloats[val]; ok {
		return v
	}
	v := l.allocVReg()
	l.constFloats[val] = v
	fn.Locals = append(fn.Locals, IRLocal{
		Name: "",
		Type: EvFloat,
		VReg: v,
	})
	// Emit a const-init pseudo-instruction (handled during codegen as an immediate global)
	fn.Body = append(fn.Body, IRInst{
		Op:          qc.OPStoreF,
		A:           v, // self-referential: codegen sets this slot's initial value
		B:           v,
		ImmFloat:    val,
		HasImmFloat: true,
		Type:        EvFloat,
	})
	return v
}

func (l *Lowerer) constString(fn *IRFunc, val string) VReg {
	if v, ok := l.constStrs[val]; ok {
		return v
	}
	v := l.allocVReg()
	l.constStrs[val] = v
	fn.Locals = append(fn.Locals, IRLocal{
		Name: "",
		Type: EvString,
		VReg: v,
	})
	fn.Body = append(fn.Body, IRInst{
		Op:     qc.OPStoreS,
		A:      v,
		B:      v,
		ImmStr: val,
		Type:   EvString,
	})
	return v
}

func (l *Lowerer) resolveObject(fn *IRFunc, obj types.Object) VReg {
	if v, ok := l.vregMap[obj]; ok {
		return v
	}
	// Must be a global or builtin — use a placeholder VReg that codegen resolves
	v := l.allocVReg()
	l.vregMap[obj] = v
	return v
}

func (l *Lowerer) newLabel(prefix string) string {
	l.labelCount++
	return prefix + "_" + strconv.Itoa(l.labelCount)
}

func (l *Lowerer) pos(node ast.Node) token.Position {
	return l.currentFset.Position(node.Pos())
}

func (l *Lowerer) isTerminating(inst IRInst) bool {
	return inst.Op == qc.OPReturn || inst.Op == qc.OPDone
}

// goTypeToQC maps a Go type to a QCVM EType.
func (l *Lowerer) goTypeToQC(t types.Type) qc.EType {
	// Handle pointers (especially *quake.Entity)
	if ptr, ok := t.(*types.Pointer); ok {
		return l.goTypeToQC(ptr.Elem())
	}

	// Check named types first
	if named, ok := t.(*types.Named); ok {
		switch named.Obj().Name() {
		case "Vec3":
			return EvVector
		case "Entity":
			return EvEntity
		case "Func":
			return EvFunction
		case "FieldOffset":
			return EvField
		}
		// Unwrap
		return l.goTypeToQC(named.Underlying())
	}

	switch bt := t.Underlying().(type) {
	case *types.Basic:
		switch bt.Kind() {
		case types.Float32, types.Float64, types.UntypedFloat:
			return EvFloat
		case types.Int, types.Int32, types.Int64, types.Uint, types.Uint32,
			types.UntypedInt:
			return EvFloat // QCVM uses float for integers
		case types.Bool, types.UntypedBool:
			return EvFloat
		case types.String, types.UntypedString:
			return EvString
		case types.Uintptr:
			return EvEntity
		}
	case *types.Array:
		if bt.Len() == 3 {
			return EvVector
		}
	case *types.Signature:
		return EvFunction
	}

	return EvFloat // default fallback
}

// constantToFloat64 extracts a float64 from a go/constant value.
func constantToFloat64(v interface{ ExactString() string }) (float64, bool) {
	s := v.ExactString()
	if s == "true" {
		return 1, true
	}
	if s == "false" {
		return 0, true
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Try parsing as integer
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, false
		}
		return float64(i), true
	}
	return f, true
}

func parseBuiltinDirective(comment string, registry builtinNameRegistry) (int, bool, string) {
	const base = "//qgo:builtin"
	if !strings.HasPrefix(comment, base) {
		return 0, false, ""
	}
	rest := strings.TrimSpace(comment[len(base):])
	if rest == "" {
		return 0, true, "malformed //qgo:builtin directive: expected one builtin number or alias"
	}
	tokens := strings.Fields(rest)
	if len(tokens) != 1 {
		return 0, true, "malformed //qgo:builtin directive: expected one builtin number or alias"
	}
	token := tokens[0]
	n, err := strconv.Atoi(token)
	if err != nil {
		if n, ok := registry.numberForName(token); ok {
			return n, true, ""
		}
		return 0, true, "unknown //qgo:builtin alias \"" + token + "\""
	}
	return n, true, ""
}
