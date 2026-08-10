package compiler

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func (l *Lowerer) lowerCallExpr(fn *IRFunc, call *ast.CallExpr) VReg {
	if result, handled := l.lowerFieldOffsetIntrinsic(fn, call); handled {
		return result
	}
	if result, handled := l.lowerTypeConversionExpr(fn, call); handled {
		return result
	}
	if result, handled := l.lowerSprintfIntrinsic(fn, call); handled {
		return result
	}

	// Resolve the function being called
	var funcObj *types.Func
	switch f := call.Fun.(type) {
	case *ast.Ident:
		if obj, ok := l.currentInfo.Uses[f].(*types.Func); ok {
			funcObj = obj
		}
	case *ast.SelectorExpr:
		// Check for method calls (e.g., v.Add(o))
		if sel, ok := l.currentInfo.Selections[f]; ok {
			if fnObj, ok := sel.Obj().(*types.Func); ok {
				sig := fnObj.Type().(*types.Signature)
				if sig.Recv() != nil {
					recvType := l.goTypeToQC(sig.Recv().Type())
					if recvType == EvVector {
						// It's a method on Vec3.
						var op qc.Opcode
						var resType qc.EType
						switch fnObj.Name() {
						case "Add":
							op = qc.OPAddV
							resType = EvVector
						case "Sub":
							op = qc.OPSubV
							resType = EvVector
						case "Mul":
							op = qc.OPMulVF
							resType = EvVector
						case "Scale":
							op = qc.OPMulVF
							resType = EvVector
						case "Dot":
							op = qc.OPMulV
							resType = EvFloat
						default:
							l.errors.Addf(l.pos(call), "unsupported Vec3 method: %s", fnObj.Name())
							return VRegInvalid
						}

						if len(call.Args) != 1 {
							l.errors.Addf(l.pos(call), "unsupported Vec3 method arity for %s: got %d args, want 1", fnObj.Name(), len(call.Args))
							return VRegInvalid
						}

						recvVReg := l.lowerExpr(fn, f.X)
						argVReg := l.lowerExpr(fn, call.Args[0])
						result := l.allocVReg()

						fn.Body = append(fn.Body, IRInst{
							Op:   op,
							A:    recvVReg,
							B:    argVReg,
							C:    result,
							Type: resType,
						})
						fn.Locals = append(fn.Locals, IRLocal{Type: resType, VReg: result})
						return result
					}
				}
				funcObj = fnObj
			}
		}
		// Also check Uses for package-level functions
		if funcObj == nil {
			if obj, ok := l.currentInfo.Uses[f.Sel].(*types.Func); ok {
				funcObj = obj
			}
		}
	}

	// Lower arguments and store to OFS_PARM slots
	for i, arg := range call.Args {
		if i >= qc.MaxParms {
			l.errors.Addf(l.pos(call), "too many arguments (max %d)", qc.MaxParms)
			break
		}
		argVReg := l.lowerExpr(fn, arg)
		argType := l.goTypeToQC(l.currentInfo.Types[arg].Type)
		parmOfs := VReg(qc.OFSParm0 + i*3)

		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForStore(argType),
			A:    argVReg,
			B:    parmOfs,
			Type: argType,
		})
	}

	// Find function VReg (reference to the function global or field)
	var funcVReg VReg
	if funcObj != nil {
		funcVReg = l.resolveObject(fn, funcObj)
	} else {
		// Try lowering as a general expression (e.g., self.think1)
		funcVReg = l.lowerExpr(fn, call.Fun)
	}

	if funcVReg == VRegInvalid {
		l.errors.Addf(l.pos(call), "cannot resolve called function")
		return VRegInvalid
	}

	// Emit CALL
	fn.Body = append(fn.Body, IRInst{
		Op:       qc.OPCall0,
		A:        funcVReg,
		ArgCount: len(call.Args),
	})

	// Return value is in OFS_RETURN
	var sig *types.Signature
	if funcObj != nil {
		sig = funcObj.Type().(*types.Signature)
	} else {
		tv := l.currentInfo.Types[call.Fun]
		if s, ok := tv.Type.Underlying().(*types.Signature); ok {
			sig = s
		}
	}

	if sig != nil && sig.Results().Len() > 0 {
		retType := l.goTypeToQC(sig.Results().At(0).Type())
		result := l.allocVReg()
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForStore(retType),
			A:    VReg(qc.OFSReturn),
			B:    result,
			Type: retType,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: retType, VReg: result})
		return result
	}

	return VRegInvalid
}

func (l *Lowerer) lowerFieldOffsetIntrinsic(fn *IRFunc, call *ast.CallExpr) (VReg, bool) {
	if result, handled := l.lowerEntityFieldFloatMethodIntrinsic(fn, call); handled {
		return result, true
	}

	intrinsic, ok := l.fieldOffsetIntrinsicName(call)
	if !ok {
		if deferredName, deferred := l.deferredFieldOffsetIntrinsicName(call); deferred {
			l.errors.Addf(
				l.pos(call),
				"quake.%s is deferred for dynamic field access; only quake.FieldFloat and quake.SetFieldFloat are currently supported",
				deferredName,
			)
			return VRegInvalid, true
		}
		return VRegInvalid, false
	}

	switch intrinsic {
	case "FieldFloat":
		if len(call.Args) != 2 {
			l.errors.Addf(l.pos(call), "quake.FieldFloat expects 2 args: (entity, fieldOffset)")
			return VRegInvalid, true
		}
		entType := l.goTypeToQC(l.currentInfo.Types[call.Args[0]].Type)
		ofsType := l.goTypeToQC(l.currentInfo.Types[call.Args[1]].Type)
		if entType != EvEntity {
			l.errors.Addf(l.pos(call.Args[0]), "quake.FieldFloat arg 1 must be entity, got %T", l.currentInfo.Types[call.Args[0]].Type)
			return VRegInvalid, true
		}
		if ofsType != EvField {
			l.errors.Addf(l.pos(call.Args[1]), "quake.FieldFloat arg 2 must be field offset, got %T", l.currentInfo.Types[call.Args[1]].Type)
			return VRegInvalid, true
		}

		ent := l.lowerExpr(fn, call.Args[0])
		ofs := l.lowerExpr(fn, call.Args[1])
		result := l.allocVReg()
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForLoad(EvFloat),
			A:    ent,
			B:    ofs,
			C:    result,
			Type: EvFloat,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: EvFloat, VReg: result})
		return result, true

	case "SetFieldFloat":
		if len(call.Args) != 3 {
			l.errors.Addf(l.pos(call), "quake.SetFieldFloat expects 3 args: (entity, fieldOffset, value)")
			return VRegInvalid, true
		}
		entType := l.goTypeToQC(l.currentInfo.Types[call.Args[0]].Type)
		ofsType := l.goTypeToQC(l.currentInfo.Types[call.Args[1]].Type)
		valType := l.goTypeToQC(l.currentInfo.Types[call.Args[2]].Type)
		if entType != EvEntity {
			l.errors.Addf(l.pos(call.Args[0]), "quake.SetFieldFloat arg 1 must be entity, got %T", l.currentInfo.Types[call.Args[0]].Type)
			return VRegInvalid, true
		}
		if ofsType != EvField {
			l.errors.Addf(l.pos(call.Args[1]), "quake.SetFieldFloat arg 2 must be field offset, got %T", l.currentInfo.Types[call.Args[1]].Type)
			return VRegInvalid, true
		}
		if valType != EvFloat {
			l.errors.Addf(l.pos(call.Args[2]), "quake.SetFieldFloat arg 3 must be float, got %T", l.currentInfo.Types[call.Args[2]].Type)
			return VRegInvalid, true
		}

		ent := l.lowerExpr(fn, call.Args[0])
		ofs := l.lowerExpr(fn, call.Args[1])
		val := l.lowerExpr(fn, call.Args[2])

		ptr := l.allocVReg()
		fn.Body = append(fn.Body, IRInst{
			Op:   qc.OPAddress,
			A:    ent,
			B:    ofs,
			C:    ptr,
			Type: EvPointer,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: EvPointer, VReg: ptr})
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForStoreP(EvFloat),
			A:    val,
			B:    ptr,
			Type: EvFloat,
		})
		return VRegInvalid, true
	}

	return VRegInvalid, false
}

func (l *Lowerer) lowerEntityFieldFloatMethodIntrinsic(fn *IRFunc, call *ast.CallExpr) (VReg, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return VRegInvalid, false
	}
	if l.currentInfo.Selections[sel] == nil {
		return VRegInvalid, false
	}

	methodName := sel.Sel.Name
	tv, ok := l.currentInfo.Types[sel.X]
	if !ok || tv.Type == nil {
		return VRegInvalid, false
	}
	entType := l.goTypeToQC(tv.Type)
	if entType != EvEntity {
		return VRegInvalid, false
	}

	if methodName != "FieldFloat" && methodName != "SetFieldFloat" {
		if !strings.HasPrefix(methodName, "Field") && !strings.HasPrefix(methodName, "SetField") {
			return VRegInvalid, false
		}
		l.errors.Addf(
			l.pos(call),
			"quake.Entity.%s is deferred for dynamic field access; only quake.Entity.FieldFloat and quake.Entity.SetFieldFloat receiver forms are currently supported",
			methodName,
		)
		return VRegInvalid, true
	}

	switch methodName {
	case "FieldFloat":
		if len(call.Args) != 1 {
			l.errors.Addf(l.pos(call), "quake.Entity.FieldFloat expects 1 arg: (fieldOffset)")
			return VRegInvalid, true
		}
		ofsType := l.goTypeToQC(l.currentInfo.Types[call.Args[0]].Type)
		if ofsType != EvField {
			l.errors.Addf(l.pos(call.Args[0]), "quake.Entity.FieldFloat arg 1 must be field offset, got %T", l.currentInfo.Types[call.Args[0]].Type)
			return VRegInvalid, true
		}

		ent := l.lowerExpr(fn, sel.X)
		ofs := l.lowerExpr(fn, call.Args[0])
		result := l.allocVReg()
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForLoad(EvFloat),
			A:    ent,
			B:    ofs,
			C:    result,
			Type: EvFloat,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: EvFloat, VReg: result})
		return result, true

	case "SetFieldFloat":
		if len(call.Args) != 2 {
			l.errors.Addf(l.pos(call), "quake.Entity.SetFieldFloat expects 2 args: (fieldOffset, value)")
			return VRegInvalid, true
		}
		ofsType := l.goTypeToQC(l.currentInfo.Types[call.Args[0]].Type)
		valType := l.goTypeToQC(l.currentInfo.Types[call.Args[1]].Type)
		if ofsType != EvField {
			l.errors.Addf(l.pos(call.Args[0]), "quake.Entity.SetFieldFloat arg 1 must be field offset, got %T", l.currentInfo.Types[call.Args[0]].Type)
			return VRegInvalid, true
		}
		if valType != EvFloat {
			l.errors.Addf(l.pos(call.Args[1]), "quake.Entity.SetFieldFloat arg 2 must be float, got %T", l.currentInfo.Types[call.Args[1]].Type)
			return VRegInvalid, true
		}

		ent := l.lowerExpr(fn, sel.X)
		ofs := l.lowerExpr(fn, call.Args[0])
		val := l.lowerExpr(fn, call.Args[1])

		ptr := l.allocVReg()
		fn.Body = append(fn.Body, IRInst{
			Op:   qc.OPAddress,
			A:    ent,
			B:    ofs,
			C:    ptr,
			Type: EvPointer,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: EvPointer, VReg: ptr})
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForStoreP(EvFloat),
			A:    val,
			B:    ptr,
			Type: EvFloat,
		})
		return VRegInvalid, true
	}

	return VRegInvalid, false
}

// lowerSprintfIntrinsic expands quake.Sprintf(fmt, args...) into a runtime
// chain of engine builtin calls. Literal format segments become string
// constants; %s args are passed through; %f/%d args are ftos()-ed. The
// result is a left-nested strcat chain like C's sprint.
//
// Only literal formats are supported; a dynamic (non-constant) format string
// is a compile error until a real mod needs it (plan R3).
func (l *Lowerer) lowerSprintfIntrinsic(fn *IRFunc, call *ast.CallExpr) (VReg, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return VRegInvalid, false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return VRegInvalid, false
	}
	pkgObj, ok := l.currentInfo.Uses[pkgIdent].(*types.PkgName)
	if !ok || pkgObj == nil || pkgObj.Imported() == nil || pkgObj.Imported().Name() != "quake" {
		return VRegInvalid, false
	}
	if sel.Sel.Name != "Sprintf" {
		return VRegInvalid, false
	}
	if len(call.Args) < 1 {
		l.errors.Addf(l.pos(call), "quake.Sprintf expects at least a format argument")
		return VRegInvalid, true
	}

	// The format must be a constant string.
	fmttv, ok := l.currentInfo.Types[call.Args[0]]
	if !ok || fmttv.Value == nil {
		l.errors.Addf(l.pos(call.Args[0]), "quake.Sprintf requires a constant format string (runtime formats unsupported)")
		return VRegInvalid, true
	}
	format := fmttv.Value.ExactString()
	if len(format) >= 2 && format[0] == '"' {
		if unquoted, err := strconv.Unquote(format); err == nil {
			format = unquoted
		}
	}

	argIdx := 1
	last := VRegInvalid
	i := 0
	for i < len(format) {
		if format[i] != '%' {
			// Scan the literal run.
			j := i
			for j < len(format) && format[j] != '%' {
				j++
			}
			seg := l.constString(fn, format[i:j])
			last = l.sprintfCat(fn, last, seg)
			i = j
			continue
		}
		if i+1 < len(format) && format[i+1] == '%' {
			// Escaped '%%' → literal '%'.
			seg := l.constString(fn, "%")
			last = l.sprintfCat(fn, last, seg)
			i += 2
			continue
		}
		// Format directive: consume "%<verb>".
		verb := byte('s')
		if i+1 < len(format) {
			verb = format[i+1]
			i += 2
		} else {
			i++
		}
		if argIdx >= len(call.Args) {
			l.errors.Addf(l.pos(call), "quake.Sprintf: not enough arguments for format %q", format)
			return VRegInvalid, true
		}
		argVReg := l.lowerExpr(fn, call.Args[argIdx])
		argIdx++
		switch verb {
		case 's':
			last = l.sprintfCat(fn, last, argVReg)
		case 'f', 'd', 'i', 'v', 'g', 'e':
			// Numeric: ftos(arg).
			ftos := l.sprintfFtos(fn, argVReg)
			last = l.sprintfCat(fn, last, ftos)
		default:
			l.errors.Addf(l.pos(call), "quake.Sprintf: unsupported format verb %%%c", verb)
			return VRegInvalid, true
		}
	}

	if last == VRegInvalid {
		// Format with no directives → the literal itself.
		return l.constString(fn, format), true
	}
	if argIdx < len(call.Args) {
		l.errors.Addf(l.pos(call), "quake.Sprintf: too many arguments for format %q", format)
		return VRegInvalid, true
	}
	return last, true
}

// sprintfCat emits strcat(l, r) if l is valid; otherwise returns r (first
// segment needs no concatenation).
func (l *Lowerer) sprintfCat(fn *IRFunc, lv, rv VReg) VReg {
	if lv == VRegInvalid {
		return rv
	}
	return l.emitStringBuiltin(fn, qc.OPCall1, "engine.Strcat", "Strcat", []VReg{lv, rv})
}

// sprintfFtos emits the ftos builtin call for a numeric argument.
func (l *Lowerer) sprintfFtos(fn *IRFunc, v VReg) VReg {
	return l.sprintfCat(fn, VRegInvalid, l.emitStringBuiltin(fn, qc.OPCall1, "engine.Ftos", "Ftos", []VReg{v}))
}

// emitStringBuiltin emits a call to an engine builtin (Strcat/Ftos above)
// returning a string, using the builtin's func cell (FuncInit resolves to
// the correct table index). Parameters are copied to OFS_PARM0+i*3 (the VM
// strides params by 3 slots, matching EnterFunction's copy loop).
func (l *Lowerer) emitStringBuiltin(fn *IRFunc, op qc.Opcode, qualified, qcName string, args []VReg) VReg {
	// The builtin's engine package object is resolved by name at lowering
	// time through the loaded type info. We create a dedicated cell for it.
	cell := l.globalCellForName(qualified, qcName)

	// Copy args to OFS_PARM slots (3-slot stride; scalar types use slot 0).
	for i, a := range args {
		if i >= qc.MaxParms {
			break
		}
		parmOfs := VReg(qc.OFSParm0 + i*3)
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForStore(EvString),
			A:    a,
			B:    parmOfs,
			Type: EvString,
		})
	}
	result := l.allocVReg()
	fn.Body = append(fn.Body, IRInst{
		Op:       op,
		A:        cell,
		ArgCount: len(args),
	})
	fn.Locals = append(fn.Locals, IRLocal{Type: EvString, VReg: result})
	fn.Body = append(fn.Body, IRInst{
		Op:   opcodeForStore(EvString),
		A:    VReg(qc.OFSReturn),
		B:    result,
		Type: EvString,
	})
	return result
}

// globalCellForName returns a function-value cell VReg for a named engine
// builtin (Strcat/Ftos), creating it if absent. The cell's FuncInit is the
// QC record name so codegen patches the correct builtin table index.
func (l *Lowerer) globalCellForName(qualified, qcName string) VReg {
	for i := range l.program.Globals {
		if l.program.Globals[i].Name == qualified {
			return VReg(l.program.Globals[i].Offset)
		}
	}
	ofs := l.allocGlobalOfs(1)
	l.program.Globals = append(l.program.Globals, IRGlobal{
		Name:     qualified,
		Type:     EvFunction,
		Offset:   ofs,
		FuncInit: qcName,
	})
	return VReg(ofs)
}

func (l *Lowerer) fieldOffsetIntrinsicName(call *ast.CallExpr) (string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	pkgObj, ok := l.currentInfo.Uses[pkgIdent].(*types.PkgName)
	if !ok || pkgObj == nil {
		return "", false
	}
	if pkgObj.Imported() == nil || pkgObj.Imported().Name() != "quake" {
		return "", false
	}
	switch sel.Sel.Name {
	case "FieldFloat", "SetFieldFloat":
		return sel.Sel.Name, true
	default:
		return "", false
	}
}

func (l *Lowerer) deferredFieldOffsetIntrinsicName(call *ast.CallExpr) (string, bool) {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if methodName, method := l.quakeEntityMethodName(sel); method {
			switch methodName {
			case "FieldFloat", "SetFieldFloat":
				return "", false
			}
			if strings.HasPrefix(methodName, "Field") || strings.HasPrefix(methodName, "SetField") {
				return "Entity." + methodName, true
			}
		}
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	pkgObj, ok := l.currentInfo.Uses[pkgIdent].(*types.PkgName)
	if !ok || pkgObj == nil {
		return "", false
	}
	if pkgObj.Imported() == nil || pkgObj.Imported().Name() != "quake" {
		return "", false
	}
	switch sel.Sel.Name {
	case "FieldFloat", "SetFieldFloat":
		return "", false
	}
	if strings.HasPrefix(sel.Sel.Name, "Field") || strings.HasPrefix(sel.Sel.Name, "SetField") {
		return sel.Sel.Name, true
	}
	return "", false
}

func (l *Lowerer) quakeEntityMethodName(sel *ast.SelectorExpr) (string, bool) {
	methodSel, ok := l.currentInfo.Selections[sel]
	if !ok || methodSel == nil {
		return "", false
	}
	obj, ok := methodSel.Obj().(*types.Func)
	if !ok || obj == nil {
		return "", false
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return "", false
	}
	recv := sig.Recv().Type()
	if ptr, ok := recv.(*types.Pointer); ok {
		recv = ptr.Elem()
	}
	named, ok := recv.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return "", false
	}
	if named.Obj().Name() != "Entity" {
		return "", false
	}
	pkg := named.Obj().Pkg()
	if pkg.Name() != "quake" {
		return "", false
	}
	return obj.Name(), true
}

func (l *Lowerer) lowerSelectorExpr(fn *IRFunc, sel *ast.SelectorExpr) VReg {
	// Check if this is a package-qualified name (e.g., engine.Self)
	if id, ok := sel.X.(*ast.Ident); ok {
		if _, ok := l.currentInfo.Uses[id].(*types.PkgName); ok {
			// Package-level variable or function
			obj := l.currentInfo.Uses[sel.Sel]
			return l.resolveObject(fn, obj)
		}
	}

	// Entity field access: ent.field → LOAD
	entVReg := l.lowerExpr(fn, sel.X)
	selObj := l.currentInfo.Selections[sel]
	if selObj == nil {
		l.errors.Addf(l.pos(sel), "unresolved selector: %s", sel.Sel.Name)
		return VRegInvalid
	}

	fieldType := l.goTypeToQC(selObj.Type())
	result := l.allocVReg()

	// For entity field access: ADDRESS(ent, fieldOfs) then LOAD. The field
	// offset must be an INT cell (OPAddress reads GInt(st.B)); using a float
	// const would bit-cast the float to a garbage field index.
	fieldOfs := l.resolveFieldOffset(selObj)
	fieldOfsVReg := l.intConst(fn, int64(fieldOfs))

	fn.Body = append(fn.Body, IRInst{
		Op:   opcodeForLoad(fieldType),
		A:    entVReg,
		B:    fieldOfsVReg,
		C:    result,
		Type: fieldType,
	})
	fn.Locals = append(fn.Locals, IRLocal{Type: fieldType, VReg: result})

	return result
}

func (l *Lowerer) lowerFieldStore(fn *IRFunc, sel *ast.SelectorExpr, val VReg) {
	entVReg := l.lowerExpr(fn, sel.X)
	selObj := l.currentInfo.Selections[sel]
	if selObj == nil {
		l.errors.Addf(l.pos(sel), "unresolved selector for store: %s", sel.Sel.Name)
		return
	}

	fieldType := l.goTypeToQC(selObj.Type())
	fieldOfs := l.resolveFieldOffset(selObj)
	fieldOfsVReg := l.intConst(fn, int64(fieldOfs))

	// ADDRESS(ent, fieldOfs) -> pointer
	ptr := l.allocVReg()
	fn.Body = append(fn.Body, IRInst{
		Op:   qc.OPAddress,
		A:    entVReg,
		B:    fieldOfsVReg,
		C:    ptr,
		Type: EvPointer,
	})
	fn.Locals = append(fn.Locals, IRLocal{Type: EvPointer, VReg: ptr})

	// STOREP(val, ptr)
	fn.Body = append(fn.Body, IRInst{
		Op:   opcodeForStoreP(fieldType),
		A:    val,
		B:    ptr,
		Type: fieldType,
	})
}

func (l *Lowerer) lowerIndexExpr(fn *IRFunc, expr *ast.IndexExpr) VReg {
	left := l.lowerExpr(fn, expr.X)
	leftType := l.goTypeToQC(l.currentInfo.Types[expr.X].Type)

	if leftType == EvVector {
		// Index into vector
		index := l.evalConstInt(expr.Index)
		if index < 0 || index >= 3 {
			l.errors.Addf(l.pos(expr), "vector index out of bounds: %d", index)
			return VRegInvalid
		}

		result := l.allocVReg()
		// For vectors, component access is just offset addition
		fn.Body = append(fn.Body, IRInst{
			Op:   qc.OPStoreF,
			A:    VReg(uint16(left) + uint16(index)),
			B:    result,
			Type: EvFloat,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: EvFloat, VReg: result})
		return result
	}

	l.errors.Addf(l.pos(expr), "unsupported index expression type: %T", l.currentInfo.Types[expr.X].Type)
	return VRegInvalid
}

func (l *Lowerer) lowerIndexStore(fn *IRFunc, expr *ast.IndexExpr, val VReg) {
	left := l.lowerExpr(fn, expr.X)
	leftType := l.goTypeToQC(l.currentInfo.Types[expr.X].Type)

	if leftType == EvVector {
		index := l.evalConstInt(expr.Index)
		if index < 0 || index >= 3 {
			l.errors.Addf(l.pos(expr), "vector index out of bounds: %d", index)
			return
		}

		fn.Body = append(fn.Body, IRInst{
			Op:   qc.OPStoreF,
			A:    val,
			B:    VReg(uint16(left) + uint16(index)),
			Type: EvFloat,
		})
	}
}

func (l *Lowerer) evalConstInt(expr ast.Expr) int {
	if tv, ok := l.currentInfo.Types[expr]; ok && tv.Value != nil {
		if val, ok := constantToFloat64(tv.Value); ok {
			return int(val)
		}
	}
	return 0
}

func (l *Lowerer) resolveFieldOffset(sel *types.Selection) uint16 {
	if ofs, ok := l.fieldOffsets[sel.Obj()]; ok {
		return ofs
	}
	return uint16(sel.Index()[0])
}
