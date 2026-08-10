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

// funcGlobalCell returns a REAL global-offset VReg (sub-vregBase) that holds
// the function table index of obj. The cell is registered once per object
// (keyed by the object identity, plan L2) and its index value is patched in
// codegen via FuncInit. Unknown functions that are not registered in the
// program receive a cell initialized to 0 (null function).
//
// pkgQualifiedName is the dedup lookup key (e.g. "quake.MakeVec3" — two
// packages may share a short name); qcName is the name used by the function
// RECORD in the emitted table (e.g. "Target" or "MakeVec3").
func (l *Lowerer) funcGlobalCell(fn *IRFunc, obj types.Object, pkgQualifiedName, qcName string) VReg {
	if v, ok := l.funcCells[obj]; ok {
		return v
	}
	ofs := l.allocGlobalOfs(1)
	v := VReg(ofs)
	l.funcCells[obj] = v

	// Register the cell as an EvFunction global. The codegen pass patches
	// its data slot to the function table index (FuncInit), or leaves 0.
	l.program.Globals = append(l.program.Globals, IRGlobal{
		Name:     pkgQualifiedName,
		Type:     EvFunction,
		Offset:   ofs,
		FuncInit: qcName,
	})
	return v
}

// intConst returns a REAL global-offset VReg whose global slot holds the raw
// int32 value val (host byte order, matching C's raw integer globals). Used
// wherever an IR operand must be an int/function index (OPAddress field
// offsets, OP_CALL targets, OPStorePFNC values).
func (l *Lowerer) intConst(fn *IRFunc, val int64) VReg {
	if v, ok := l.constInts[val]; ok {
		return v
	}
	ofs := l.allocGlobalOfs(1)
	v := VReg(ofs)
	l.constInts[val] = v

	// Register an anonymous (unnamed) EvFunction global at this offset; the
	// per-function IRStoreFNC instruction below writes the raw int bits into
	// it at codegen time (emitInst OPStoreFNC+HasImmFloat).
	l.program.Globals = append(l.program.Globals, IRGlobal{
		Name:   "",
		Type:   EvFunction,
		Offset: ofs,
	})
	fn.Body = append(fn.Body, IRInst{
		Op:          qc.OPStoreFNC,
		A:           v,
		B:           v,
		ImmFloat:    float64(uint32(val)),
		HasImmFloat: true,
		Type:        EvFunction,
	})
	return v
}

// engineVarQCGlobal maps a quake/engine package var name to the QCVM global
// offset it mirrors (e.g. engine.Self → self at OFSSelf). The engine package
// declares these as plain Go vars so QuakeC code can read/write them; the
// compiler must resolve them to the REAL global slots (GlobalAllocator
// pre-registers the same offsets by name) instead of uninitialized locals.
var engineVarQCGlobal = map[string]uint16{
	"Self":             qc.OFSSelf,             // self
	"Other":            qc.OFSOther,            // other
	"World":            qc.OFSWorld,            // world
	"Time":             qc.OFSTime,             // time
	"FrameTime":        qc.OFSFrameTime,        // frametime
	"MapName":          qc.OFSMapName,          // mapname
	"VForward":         qc.OFSGlobalVForward,   // v_forward
	"VUp":              qc.OFSGlobalVUp,        // v_up
	"VRight":           qc.OFSGlobalVRight,     // v_right
	"TraceAllSolid":    qc.OFSTraceAllSolid,    // trace_allsolid
	"TraceStartSolid":  qc.OFSTraceStartSolid,  // trace_startsolid
	"TraceFraction":    qc.OFSTraceFraction,    // trace_fraction
	"TraceEndPos":      qc.OFSTraceEndPos,      // trace_endpos
	"TracePlaneNormal": qc.OFSTracePlaneNormal, // trace_plane_normal
	"TracePlaneDist":   qc.OFSTracePlaneDist,   // trace_plane_dist
	"TraceEnt":         qc.OFSTraceEnt,         // trace_ent
	"TraceInOpen":      qc.OFSTraceInOpen,      // trace_inopen
	"TraceInWater":     qc.OFSTraceInWater,     // trace_inwater
}

func (l *Lowerer) resolveObject(fn *IRFunc, obj types.Object) VReg {
	if v, ok := l.vregMap[obj]; ok {
		return v
	}
	// A package-level variable that is not a function value.
	if _, isFunc := obj.(*types.Func); !isFunc {
		if pv, ok := obj.(*types.Var); ok && pv.Pkg() != nil {
			// quake/engine globals mirror QCVM globals at fixed offsets.
			if pv.Pkg().Name() == "engine" {
				if ofs, ok := engineVarQCGlobal[pv.Name()]; ok {
					return VReg(ofs)
				}
			}
			// A package var whose qgo tag binds a QCVM system global
			// (e.g. quakego defs.go `//qgo:self`): return that fixed slot.
			if ofs, ok := l.globalVarOfs[obj]; ok {
				return VReg(ofs)
			}

			// Fallback for plain package vars (rare in QC code).
			v := l.allocVReg()
			l.vregMap[obj] = v
			return v
		}
	}
	// Function object: return its function-value cell (real global offset).
	if fnObj, isFunc := obj.(*types.Func); isFunc {
		// A method (recv != nil) shares the package-qualified name with every
		// other method of the same name in the package (e.g. the many
		// `entity()` accessor methods). Its DDef name must still be unique so
		// Reserve never collides; qualify with the receiver type. Method
		// values are never registered as QC functions, so their cell stays 0
		// (a null func) — the unique name is only for the DDef slot.
		if fnObj.Type() != nil {
			if sig, ok := fnObj.Type().(*types.Signature); ok && sig.Recv() != nil {
				recv := sig.Recv().Type()
				if p, ok := recv.(*types.Pointer); ok {
					recv = p.Elem()
				}
				recvName := "?"
				if named, ok := recv.(*types.Named); ok && named.Obj() != nil {
					recvName = named.Obj().Name()
				}
				pkgQualified := fnObj.Name() + "@" + recvName
				return l.funcGlobalCell(fn, obj, pkgQualified, fnObj.Name())
			}
		}
		name := fnObj.Name()
		pkgQualified := fnObj.Name()
		if pkg := fnObj.Pkg(); pkg != nil {
			pkgQualified = pkg.Name() + "." + fnObj.Name()
		}
		// The DDef/record name is the Go short (QC) name; the qualified name
		// is only for deduping cells across packages (plan L2).
		return l.funcGlobalCell(fn, obj, pkgQualified, name)
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
