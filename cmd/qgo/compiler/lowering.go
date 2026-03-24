package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/qc"
	"golang.org/x/tools/go/packages"
)

// Lowerer translates a type-checked Go AST into IR.
type Lowerer struct {
	errors ErrorList

	program     IRProgram
	nextVReg    VReg
	vregMap     map[types.Object]VReg // Go object -> virtual register
	constFloats map[float64]VReg      // const float pool
	constStrs   map[string]VReg       // const string pool
	labelCount  int

	entityFields map[types.Type][]IRField // Type -> fields
	fieldOffsets map[types.Object]uint16  // Field object -> offset

	breakLabels    []string
	continueLabels []string

	// Per-package state during lowering
	currentInfo *types.Info
	currentFset *token.FileSet
}

// NewLowerer creates a new lowerer.
func NewLowerer() *Lowerer {
	return &Lowerer{
		entityFields: make(map[types.Type][]IRField),
		fieldOffsets: make(map[types.Object]uint16),
	}
}

// LowerPackages processes a collection of packages and returns the IR program.
func (l *Lowerer) LowerPackages(pkgs []*packages.Package) (*IRProgram, error) {
	// Pass 1: declarations and entity structs for explicitly requested packages only.
	// Imported package bodies are intentionally excluded; lowering should not descend
	// into dependency implementation files.
	for _, p := range pkgs {
		l.currentInfo = p.TypesInfo
		l.currentFset = p.Fset
		for _, file := range sortedSyntaxFiles(p) {
			l.lowerFileDecls(file)
		}
	}

	// Pass 2: function bodies for explicitly requested packages only.
	for _, p := range pkgs {
		l.currentInfo = p.TypesInfo
		l.currentFset = p.Fset
		for _, file := range sortedSyntaxFiles(p) {
			l.lowerFileFuncs(file)
		}
	}

	if err := l.errors.Err(); err != nil {
		return nil, err
	}

	return &l.program, nil
}

func sortedSyntaxFiles(p *packages.Package) []*ast.File {
	files := append([]*ast.File(nil), p.Syntax...)
	sort.Slice(files, func(i, j int) bool {
		pi := p.Fset.Position(files[i].Pos()).Filename
		pj := p.Fset.Position(files[j].Pos()).Filename
		return pi < pj
	})
	return files
}

func (l *Lowerer) lowerFileDecls(file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			l.lowerGenDecl(d)
		case *ast.FuncDecl:
			l.registerFunc(d)
		}
	}
}

func (l *Lowerer) lowerFileFuncs(file *ast.File) {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			l.lowerFuncBody(fd)
		}
	}
}

func (l *Lowerer) lowerGenDecl(decl *ast.GenDecl) {
	switch decl.Tok {
	case token.VAR:
		for _, spec := range decl.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if name.Name == "_" {
					continue
				}

				obj := l.currentInfo.Defs[name]
				if obj == nil {
					continue
				}

				g := IRGlobal{
					Name: name.Name,
					Type: l.goTypeToQC(obj.Type()),
				}

				// Check for qgo tag in comments or if it's a field (ValueSpec doesn't have Tags, but we can check the comment)
				if vs.Doc != nil {
					for _, c := range vs.Doc.List {
						if strings.HasPrefix(c.Text, "//qgo:") {
							g.Name = strings.TrimSpace(c.Text[6:])
						}
					}
				} else if vs.Comment != nil {
					for _, c := range vs.Comment.List {
						if strings.HasPrefix(c.Text, "//qgo:") {
							g.Name = strings.TrimSpace(c.Text[6:])
						}
					}
				}

				// Initial value
				if i < len(vs.Values) {
					g = l.evalGlobalInit(g, vs.Values[i])
				}

				l.program.Globals = append(l.program.Globals, g)
			}
		}

	case token.TYPE:
		for _, spec := range decl.Specs {
			ts := spec.(*ast.TypeSpec)
			l.checkEntityType(decl, ts)
		}
	}
}

func (l *Lowerer) checkEntityType(decl *ast.GenDecl, ts *ast.TypeSpec) {
	// Check for //qgo:entity directive
	isEntity := false
	if decl.Doc != nil {
		for _, c := range decl.Doc.List {
			if strings.TrimSpace(c.Text) == "//qgo:entity" {
				isEntity = true
				break
			}
		}
	}

	if !isEntity {
		return
	}

	if _, ok := ts.Type.(*ast.StructType); !ok {
		l.errors.Addf(l.pos(ts), "//qgo:entity can only be applied to struct types")
		return
	}

	obj := l.currentInfo.Defs[ts.Name]
	if obj == nil {
		return
	}

	structType := obj.Type().Underlying().(*types.Struct)
	var fields []IRField
	l.collectEntityFields(&fields, structType, 0)

	// Register fields if they haven't been registered yet
	// (Quake fields are global and shared across all entities)
	for _, f := range fields {
		found := false
		for i := range l.program.Fields {
			if l.program.Fields[i].Name == f.Name {
				if l.program.Fields[i].Offset != f.Offset {
					l.errors.Addf(l.pos(ts), "conflicting offset for field %s", f.Name)
				}
				found = true
				break
			}
		}
		if !found {
			l.program.Fields = append(l.program.Fields, f)
		}
	}

	l.entityFields[obj.Type()] = fields
}

func (l *Lowerer) collectEntityFields(out *[]IRField, st *types.Struct, baseOffset uint16) uint16 {
	currentOffset := baseOffset
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag := st.Tag(i)

		qcName := ""
		if tag != "" {
			qcName = reflect.StructTag(tag).Get("qgo")
		}
		if qcName == "" {
			qcName = strings.ToLower(f.Name())
		}

		qcType := l.goTypeToQC(f.Type())

		if f.Anonymous() {
			if embedded, ok := f.Type().Underlying().(*types.Struct); ok {
				currentOffset = l.collectEntityFields(out, embedded, currentOffset)
				continue
			}
		}

		*out = append(*out, IRField{
			Name:   qcName,
			Type:   qcType,
			Offset: currentOffset,
		})
		l.fieldOffsets[f] = currentOffset
		currentOffset += slotsForType(qcType)
	}
	return currentOffset
}

func (l *Lowerer) evalGlobalInit(g IRGlobal, expr ast.Expr) IRGlobal {
	// Try constant evaluation
	if tv, ok := l.currentInfo.Types[expr]; ok && tv.Value != nil {
		switch g.Type {
		case EvFloat:
			if f, ok := constantToFloat64(tv.Value); ok {
				g.InitFloat = f
			}
		case EvString:
			g.InitStr = tv.Value.ExactString()
			// Strip quotes from string constant
			if len(g.InitStr) >= 2 && g.InitStr[0] == '"' {
				g.InitStr, _ = strconv.Unquote(g.InitStr)
			}
		}
	}
	return g
}

func (l *Lowerer) registerFunc(fd *ast.FuncDecl) {
	obj := l.currentInfo.Defs[fd.Name]
	if obj == nil {
		return
	}

	fn := IRFunc{
		Name:   fd.Name.Name,
		QCName: fd.Name.Name, // May be transformed later
	}

	sig := obj.Type().(*types.Signature)

	// Parameters
	params := sig.Params()
	for i := range params.Len() {
		p := params.At(i)
		fn.Params = append(fn.Params, IRParam{
			Name: p.Name(),
			Type: l.goTypeToQC(p.Type()),
		})
	}

	// Return type
	results := sig.Results()
	if results.Len() > 0 {
		fn.ReturnType = l.goTypeToQC(results.At(0).Type())
	} else {
		fn.ReturnType = EvVoid
	}

	// Check for builtin directive in preceding comments
	if fd.Doc != nil {
		for _, c := range fd.Doc.List {
			if num, ok := parseBuiltinDirective(c.Text); ok {
				fn.IsBuiltin = true
				fn.BuiltinNum = num
			}
		}
	}

	l.program.Functions = append(l.program.Functions, fn)
}

func (l *Lowerer) lowerFuncBody(fd *ast.FuncDecl) {
	if fd.Body == nil {
		return
	}

	// Find the matching IRFunc
	var fn *IRFunc
	for i := range l.program.Functions {
		if l.program.Functions[i].Name == fd.Name.Name {
			fn = &l.program.Functions[i]
			break
		}
	}
	if fn == nil || fn.IsBuiltin {
		return
	}

	// Reset per-function state.
	// Start VRegs at a safe base so they don't collide with OFS_* constants
	// used as direct global offsets in IR instructions (e.g., OFS_RETURN=1).
	l.nextVReg = vregBase
	l.vregMap = make(map[types.Object]VReg)
	l.constFloats = make(map[float64]VReg)
	l.constStrs = make(map[string]VReg)

	// Register parameters as VRegs
	sig := l.currentInfo.Defs[fd.Name].Type().(*types.Signature)
	params := sig.Params()
	for i := range params.Len() {
		p := params.At(i)
		vreg := l.allocVReg()
		l.vregMap[p] = vreg
		fn.Locals = append(fn.Locals, IRLocal{
			Name: p.Name(),
			Type: l.goTypeToQC(p.Type()),
			VReg: vreg,
		})
	}

	// Lower body
	for _, stmt := range fd.Body.List {
		l.lowerStmt(fn, stmt)
	}

	// Ensure function ends with DONE/RETURN
	if len(fn.Body) == 0 || !l.isTerminating(fn.Body[len(fn.Body)-1]) {
		fn.Body = append(fn.Body, IRInst{Op: qc.OPReturn})
	}
}

func (l *Lowerer) lowerStmt(fn *IRFunc, stmt ast.Stmt) {
	switch s := stmt.(type) {
	case *ast.ReturnStmt:
		l.lowerReturn(fn, s)
	case *ast.AssignStmt:
		l.lowerAssign(fn, s)
	case *ast.ExprStmt:
		l.lowerExpr(fn, s.X)
	case *ast.IfStmt:
		l.lowerIf(fn, s)
	case *ast.ForStmt:
		l.lowerFor(fn, s)
	case *ast.SwitchStmt:
		l.lowerSwitchStmt(fn, s)
	case *ast.BranchStmt:
		l.lowerBranch(fn, s)
	case *ast.IncDecStmt:
		l.lowerIncDec(fn, s)
	case *ast.DeclStmt:
		l.lowerDeclStmt(fn, s)
	case *ast.BlockStmt:
		for _, inner := range s.List {
			l.lowerStmt(fn, inner)
		}
	default:
		l.errors.Addf(l.pos(stmt), "unsupported statement type: %T", stmt)
	}
}

func (l *Lowerer) lowerReturn(fn *IRFunc, s *ast.ReturnStmt) {
	if len(s.Results) > 0 {
		result := l.lowerExpr(fn, s.Results[0])
		// Store result to OFS_RETURN, then RETURN with A=OFS_RETURN.
		// RETURN always copies 3 slots from A, so A must point to a safe
		// 3-slot region. OFS_RETURN (slots 1-3) is always safe.
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForStore(fn.ReturnType),
			A:    result,
			B:    VReg(qc.OFSReturn),
			Type: fn.ReturnType,
		})
	}
	fn.Body = append(fn.Body, IRInst{Op: qc.OPReturn, A: VReg(qc.OFSReturn)})
}

func (l *Lowerer) lowerAssign(fn *IRFunc, s *ast.AssignStmt) {
	for i, lhs := range s.Lhs {
		rhs := l.lowerExpr(fn, s.Rhs[i])

		switch lv := lhs.(type) {
		case *ast.Ident:
			if s.Tok == token.DEFINE {
				// Short variable declaration
				obj := l.currentInfo.Defs[lv]
				if obj != nil {
					vreg := l.allocVReg()
					l.vregMap[obj] = vreg
					fn.Locals = append(fn.Locals, IRLocal{
						Name: lv.Name,
						Type: l.goTypeToQC(obj.Type()),
						VReg: vreg,
					})
					fn.Body = append(fn.Body, IRInst{
						Op:   opcodeForStore(l.goTypeToQC(obj.Type())),
						A:    rhs,
						B:    vreg,
						Type: l.goTypeToQC(obj.Type()),
					})
					continue
				}
			}

			obj := l.currentInfo.Uses[lv]
			if obj == nil {
				obj = l.currentInfo.Defs[lv]
			}
			if obj == nil {
				l.errors.Addf(l.pos(lv), "unresolved identifier: %s", lv.Name)
				continue
			}

			dst := l.resolveObject(fn, obj)
			fn.Body = append(fn.Body, IRInst{
				Op:   opcodeForStore(l.goTypeToQC(obj.Type())),
				A:    rhs,
				B:    dst,
				Type: l.goTypeToQC(obj.Type()),
			})

		case *ast.SelectorExpr:
			// Entity field store: ent.field = val → ADDRESS + STOREP
			l.lowerFieldStore(fn, lv, rhs)

		case *ast.IndexExpr:
			// Index store: v[i] = val
			l.lowerIndexStore(fn, lv, rhs)

		default:
			l.errors.Addf(l.pos(lhs), "unsupported assignment target: %T", lhs)
		}
	}
}

func (l *Lowerer) lowerExpr(fn *IRFunc, expr ast.Expr) VReg {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return l.lowerBasicLit(fn, e)

	case *ast.Ident:
		return l.lowerIdent(fn, e)

	case *ast.BinaryExpr:
		return l.lowerBinaryExpr(fn, e)

	case *ast.UnaryExpr:
		return l.lowerUnaryExpr(fn, e)

	case *ast.CallExpr:
		// Check for MakeVec3
		if id, ok := e.Fun.(*ast.Ident); ok {
			if id.Name == "MakeVec3" {
				result := l.allocVReg()
				fn.Locals = append(fn.Locals, IRLocal{Type: EvVector, VReg: result})
				for i, arg := range e.Args {
					if i >= 3 {
						break
					}
					val := l.lowerExpr(fn, arg)
					fn.Body = append(fn.Body, IRInst{
						Op:   qc.OPStoreF,
						A:    val,
						B:    VReg(uint16(result) + uint16(i)),
						Type: EvFloat,
					})
				}
				return result
			}
		}
		return l.lowerCallExpr(fn, e)

	case *ast.CompositeLit:
		return l.lowerCompositeLit(fn, e)

	case *ast.SelectorExpr:
		return l.lowerSelectorExpr(fn, e)

	case *ast.IndexExpr:
		return l.lowerIndexExpr(fn, e)

	case *ast.ParenExpr:
		return l.lowerExpr(fn, e.X)

	default:
		l.errors.Addf(l.pos(expr), "unsupported expression type: %T", expr)
		return VRegInvalid
	}
}

func (l *Lowerer) lowerBasicLit(fn *IRFunc, lit *ast.BasicLit) VReg {
	switch lit.Kind {
	case token.FLOAT, token.INT:
		val, _ := strconv.ParseFloat(lit.Value, 64)
		return l.constFloat(fn, val)
	case token.STRING:
		s, _ := strconv.Unquote(lit.Value)
		return l.constString(fn, s)
	default:
		l.errors.Addf(l.pos(lit), "unsupported literal kind: %s", lit.Kind)
		return VRegInvalid
	}
}

func (l *Lowerer) lowerCompositeLit(fn *IRFunc, lit *ast.CompositeLit) VReg {
	qcType := l.goTypeToQC(l.currentInfo.Types[lit].Type)
	if qcType == EvVector {
		// Vector literal
		result := l.allocVReg()
		fn.Locals = append(fn.Locals, IRLocal{Type: EvVector, VReg: result})

		for i, elt := range lit.Elts {
			if i >= 3 {
				break
			}
			val := l.lowerExpr(fn, elt)
			// Store each component to result[i]
			// Since result is a vector VReg, it takes 3 slots.
			// result+i points to the component.
			fn.Body = append(fn.Body, IRInst{
				Op:   qc.OPStoreF,
				A:    val,
				B:    VReg(uint16(result) + uint16(i)),
				Type: EvFloat,
			})
		}
		return result
	}

	if typ := l.currentInfo.Types[lit].Type; typ != nil {
		if _, ok := typ.Underlying().(*types.Struct); ok {
			l.errors.Addf(l.pos(lit), "general struct literals are deferred (only Vec3 vector literals are currently supported): %s", typ.String())
			return VRegInvalid
		}
	}

	l.errors.Addf(l.pos(lit), "unsupported composite literal type: %T", l.currentInfo.Types[lit].Type)
	return VRegInvalid
}

func (l *Lowerer) lowerIdent(fn *IRFunc, id *ast.Ident) VReg {
	// Check for constant value
	if tv, ok := l.currentInfo.Types[id]; ok && tv.Value != nil {
		if f, ok := constantToFloat64(tv.Value); ok {
			return l.constFloat(fn, f)
		}
	}

	obj := l.currentInfo.Uses[id]
	if obj == nil {
		obj = l.currentInfo.Defs[id]
	}
	if obj == nil {
		l.errors.Addf(l.pos(id), "unresolved identifier: %s", id.Name)
		return VRegInvalid
	}
	return l.resolveObject(fn, obj)
}

func (l *Lowerer) lowerBinaryExpr(fn *IRFunc, expr *ast.BinaryExpr) VReg {
	left := l.lowerExpr(fn, expr.X)
	right := l.lowerExpr(fn, expr.Y)
	result := l.allocVReg()

	tv := l.currentInfo.Types[expr]
	qcType := l.goTypeToQC(tv.Type)

	var op qc.Opcode
	switch expr.Op {
	case token.ADD:
		if qcType == EvVector {
			op = qc.OPAddV
		} else {
			op = qc.OPAddF
		}
	case token.SUB:
		if qcType == EvVector {
			op = qc.OPSubV
		} else {
			op = qc.OPSubF
		}
	case token.MUL:
		op = qc.OPMulF
		// Check for vec*float, float*vec
		leftType := l.goTypeToQC(l.currentInfo.Types[expr.X].Type)
		rightType := l.goTypeToQC(l.currentInfo.Types[expr.Y].Type)
		if leftType == EvVector && rightType == EvFloat {
			op = qc.OPMulVF
		} else if leftType == EvFloat && rightType == EvVector {
			op = qc.OPMulFV
		} else if leftType == EvVector && rightType == EvVector {
			op = qc.OPMulV // dot product
		}
	case token.QUO:
		op = qc.OPDivF

	case token.EQL:
		op = opcodeForEq(qcType)
	case token.NEQ:
		op = opcodeForNe(qcType)
	case token.LSS:
		op = qc.OPLT
	case token.GTR:
		op = qc.OPGT
	case token.LEQ:
		op = qc.OPLE
	case token.GEQ:
		op = qc.OPGE

	case token.LAND:
		op = qc.OPAnd
	case token.LOR:
		op = qc.OPOr

	case token.AND:
		op = qc.OPBitAnd
	case token.OR:
		op = qc.OPBitOr

	default:
		l.errors.Addf(l.pos(expr), "unsupported binary operator: %s", expr.Op)
		return VRegInvalid
	}

	fn.Body = append(fn.Body, IRInst{
		Op:   op,
		A:    left,
		B:    right,
		C:    result,
		Type: qcType,
	})

	// Register result as a local
	fn.Locals = append(fn.Locals, IRLocal{
		Name: "",
		Type: qcType,
		VReg: result,
	})

	return result
}

func (l *Lowerer) lowerUnaryExpr(fn *IRFunc, expr *ast.UnaryExpr) VReg {
	operand := l.lowerExpr(fn, expr.X)
	qcType := l.goTypeToQC(l.currentInfo.Types[expr].Type)

	switch expr.Op {
	case token.NOT:
		result := l.allocVReg()
		fn.Body = append(fn.Body, IRInst{
			Op:   opcodeForNot(qcType),
			A:    operand,
			C:    result,
			Type: qcType,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: qcType, VReg: result})
		return result

	case token.SUB:
		// -x → 0 - x
		zero := l.constFloat(fn, 0)
		result := l.allocVReg()
		fn.Body = append(fn.Body, IRInst{
			Op:   qc.OPSubF,
			A:    zero,
			B:    operand,
			C:    result,
			Type: EvFloat,
		})
		fn.Locals = append(fn.Locals, IRLocal{Type: EvFloat, VReg: result})
		return result
	}

	l.errors.Addf(l.pos(expr), "unsupported unary operator: %s", expr.Op)
	return VRegInvalid
}

func (l *Lowerer) lowerCallExpr(fn *IRFunc, call *ast.CallExpr) VReg {
	if result, handled := l.lowerFieldOffsetIntrinsic(fn, call); handled {
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
						// It's a method on Vec3
						recvVReg := l.lowerExpr(fn, f.X)
						argVReg := l.lowerExpr(fn, call.Args[0])
						result := l.allocVReg()

						var op qc.Opcode
						var resType qc.EType = EvVector
						switch fnObj.Name() {
						case "Add":
							op = qc.OPAddV
						case "Sub":
							op = qc.OPSubV
						case "Mul":
							op = qc.OPMulVF
						case "Dot":
							op = qc.OPMulV
							resType = EvFloat
						default:
							l.errors.Addf(l.pos(call), "unsupported Vec3 method: %s", fnObj.Name())
							return VRegInvalid
						}

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
	intrinsic, ok := l.fieldOffsetIntrinsicName(call)
	if !ok {
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

	// For entity field access: ADDRESS(ent, fieldOfs) then LOAD
	fieldOfs := l.resolveFieldOffset(selObj)
	fieldOfsVReg := l.constFloat(fn, float64(fieldOfs))

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
	fieldOfsVReg := l.constFloat(fn, float64(fieldOfs))

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

func (l *Lowerer) lowerIf(fn *IRFunc, s *ast.IfStmt) {
	if s.Init != nil {
		l.lowerStmt(fn, s.Init)
	}

	cond := l.lowerExpr(fn, s.Cond)
	elseLabel := l.newLabel("else")
	endLabel := l.newLabel("endif")

	if s.Else != nil {
		fn.Body = append(fn.Body, IRInst{Op: qc.OPIFNot, A: cond, Label: elseLabel})
		for _, stmt := range s.Body.List {
			l.lowerStmt(fn, stmt)
		}
		fn.Body = append(fn.Body, IRInst{Op: qc.OPGoto, Label: endLabel})
		fn.Body = append(fn.Body, LabelInst(elseLabel))
		l.lowerStmt(fn, s.Else)
		fn.Body = append(fn.Body, LabelInst(endLabel))
	} else {
		fn.Body = append(fn.Body, IRInst{Op: qc.OPIFNot, A: cond, Label: endLabel})
		for _, stmt := range s.Body.List {
			l.lowerStmt(fn, stmt)
		}
		fn.Body = append(fn.Body, LabelInst(endLabel))
	}
}

func (l *Lowerer) lowerFor(fn *IRFunc, s *ast.ForStmt) {
	if s.Init != nil {
		l.lowerStmt(fn, s.Init)
	}

	topLabel := l.newLabel("for_top")
	exitLabel := l.newLabel("for_exit")
	postLabel := l.newLabel("for_post")

	l.breakLabels = append(l.breakLabels, exitLabel)
	l.continueLabels = append(l.continueLabels, postLabel)

	fn.Body = append(fn.Body, LabelInst(topLabel))

	if s.Cond != nil {
		cond := l.lowerExpr(fn, s.Cond)
		fn.Body = append(fn.Body, IRInst{Op: qc.OPIFNot, A: cond, Label: exitLabel})
	}

	for _, stmt := range s.Body.List {
		l.lowerStmt(fn, stmt)
	}

	fn.Body = append(fn.Body, LabelInst(postLabel))
	if s.Post != nil {
		l.lowerStmt(fn, s.Post)
	}

	fn.Body = append(fn.Body, IRInst{Op: qc.OPGoto, Label: topLabel})
	fn.Body = append(fn.Body, LabelInst(exitLabel))

	l.breakLabels = l.breakLabels[:len(l.breakLabels)-1]
	l.continueLabels = l.continueLabels[:len(l.continueLabels)-1]
}

func (l *Lowerer) lowerSwitchStmt(fn *IRFunc, s *ast.SwitchStmt) {
	if s.Init != nil {
		l.lowerStmt(fn, s.Init)
	}

	tag := l.lowerExpr(fn, s.Tag)
	tagType := l.goTypeToQC(l.currentInfo.Types[s.Tag].Type)
	endLabel := l.newLabel("sw_end")

	l.breakLabels = append(l.breakLabels, endLabel)

	var defaultClause *ast.CaseClause
	var nextCaseLabel string

	for _, stmt := range s.Body.List {
		cc := stmt.(*ast.CaseClause)
		if cc.List == nil {
			defaultClause = cc
			continue
		}

		caseBodyLabel := l.newLabel("sw_case_body")
		nextCaseLabel = l.newLabel("sw_next_case")

		// Compare tag against each expression in the case list (OR logic)
		for _, expr := range cc.List {
			val := l.lowerExpr(fn, expr)
			cond := l.allocVReg()
			fn.Body = append(fn.Body, IRInst{
				Op:   opcodeForEq(tagType),
				A:    tag,
				B:    val,
				C:    cond,
				Type: EvFloat,
			})
			fn.Locals = append(fn.Locals, IRLocal{Type: EvFloat, VReg: cond})
			fn.Body = append(fn.Body, IRInst{Op: qc.OPIF, A: cond, Label: caseBodyLabel})
		}

		// If no expression matched, jump to next case
		fn.Body = append(fn.Body, IRInst{Op: qc.OPGoto, Label: nextCaseLabel})

		// Case body
		fn.Body = append(fn.Body, LabelInst(caseBodyLabel))
		for _, s := range cc.Body {
			l.lowerStmt(fn, s)
		}
		fn.Body = append(fn.Body, IRInst{Op: qc.OPGoto, Label: endLabel})

		// Label for next case comparison
		fn.Body = append(fn.Body, LabelInst(nextCaseLabel))
	}

	// Default case if no other cases matched
	if defaultClause != nil {
		for _, s := range defaultClause.Body {
			l.lowerStmt(fn, s)
		}
	}

	fn.Body = append(fn.Body, LabelInst(endLabel))
	l.breakLabels = l.breakLabels[:len(l.breakLabels)-1]
}

func (l *Lowerer) lowerBranch(fn *IRFunc, s *ast.BranchStmt) {
	switch s.Tok {
	case token.BREAK:
		if len(l.breakLabels) == 0 {
			l.errors.Addf(l.pos(s), "break outside of loop or switch")
			return
		}
		fn.Body = append(fn.Body, IRInst{Op: qc.OPGoto, Label: l.breakLabels[len(l.breakLabels)-1]})
	case token.CONTINUE:
		if len(l.continueLabels) == 0 {
			l.errors.Addf(l.pos(s), "continue outside of loop")
			return
		}
		fn.Body = append(fn.Body, IRInst{Op: qc.OPGoto, Label: l.continueLabels[len(l.continueLabels)-1]})
	default:
		l.errors.Addf(l.pos(s), "unsupported branch statement: %s", s.Tok)
	}
}

func (l *Lowerer) lowerIncDec(fn *IRFunc, s *ast.IncDecStmt) {
	operand := l.lowerExpr(fn, s.X)
	one := l.constFloat(fn, 1.0)

	var op qc.Opcode
	if s.Tok == token.INC {
		op = qc.OPAddF
	} else {
		op = qc.OPSubF
	}

	// result = operand +/- 1
	result := l.allocVReg()
	fn.Body = append(fn.Body, IRInst{
		Op: op, A: operand, B: one, C: result, Type: EvFloat,
	})
	fn.Locals = append(fn.Locals, IRLocal{Type: EvFloat, VReg: result})

	// Store back to the operand
	fn.Body = append(fn.Body, IRInst{
		Op:   qc.OPStoreF,
		A:    result,
		B:    operand,
		Type: EvFloat,
	})
}

func (l *Lowerer) lowerDeclStmt(fn *IRFunc, s *ast.DeclStmt) {
	gd, ok := s.Decl.(*ast.GenDecl)
	if !ok || gd.Tok != token.VAR {
		return
	}

	for _, spec := range gd.Specs {
		vs := spec.(*ast.ValueSpec)
		for i, name := range vs.Names {
			if name.Name == "_" {
				continue
			}
			obj := l.currentInfo.Defs[name]
			if obj == nil {
				continue
			}
			vreg := l.allocVReg()
			l.vregMap[obj] = vreg
			qcType := l.goTypeToQC(obj.Type())
			fn.Locals = append(fn.Locals, IRLocal{
				Name: name.Name,
				Type: qcType,
				VReg: vreg,
			})

			if i < len(vs.Values) {
				val := l.lowerExpr(fn, vs.Values[i])
				fn.Body = append(fn.Body, IRInst{
					Op:   opcodeForStore(qcType),
					A:    val,
					B:    vreg,
					Type: qcType,
				})
			}
		}
	}
}

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

func builtinAliasToNumber(alias string) (int, bool) {
	switch alias {
	case "setorigin":
		return 2, true
	case "setmodel":
		return 3, true
	case "setsize":
		return 4, true
	case "sound":
		return 8, true
	case "spawn":
		return 14, true
	case "remove":
		return 15, true
	case "traceline":
		return 16, true
	case "checkclient":
		return 17, true
	case "find":
		return 18, true
	case "precache_sound":
		return 19, true
	case "precache_model":
		return 20, true
	case "stuffcmd":
		return 21, true
	case "findradius":
		return 22, true
	case "bprint":
		return 23, true
	case "sprint":
		return 24, true
	case "dprint":
		return 25, true
	case "ftos":
		return 26, true
	case "vtos":
		return 27, true
	case "eprint":
		return 31, true
	case "walkmove":
		return 32, true
	case "droptofloor":
		return 34, true
	case "lightstyle":
		return 35, true
	case "rint":
		return 36, true
	case "floor":
		return 37, true
	case "ceil":
		return 38, true
	case "checkbottom":
		return 40, true
	case "pointcontents":
		return 41, true
	case "fabs":
		return 43, true
	case "aim":
		return 44, true
	case "cvar":
		return 45, true
	case "localcmd":
		return 46, true
	case "nextent":
		return 47, true
	case "particle":
		return 48, true
	case "changeyaw":
		return 49, true
	case "vectoangles":
		return 51, true
	case "writebyte":
		return 52, true
	case "writechar":
		return 53, true
	case "writeshort":
		return 54, true
	case "writelong":
		return 55, true
	case "writecoord":
		return 56, true
	case "writeangle":
		return 57, true
	case "writestring":
		return 58, true
	case "writeentity":
		return 59, true
	case "sin":
		return 60, true
	case "cos":
		return 61, true
	case "sqrt":
		return 62, true
	case "etos":
		return 65, true
	case "movetogoal":
		return 67, true
	case "precache_file":
		return 68, true
	case "makestatic":
		return 69, true
	case "changelevel":
		return 70, true
	case "cvar_set":
		return 72, true
	case "centerprint":
		return 73, true
	case "ambientsound":
		return 74, true
	case "precache_model2":
		return 75, true
	case "precache_sound2":
		return 76, true
	case "precache_file2":
		return 77, true
	case "setspawnparms":
		return 78, true
	case "local_sound":
		return 80, true
	}
	return 0, false
}

func parseBuiltinDirective(comment string) (int, bool) {
	const prefix = "//qgo:builtin "
	if len(comment) <= len(prefix) || comment[:len(prefix)] != prefix {
		return 0, false
	}
	token := strings.TrimSpace(comment[len(prefix):])
	n, err := strconv.Atoi(token)
	if err != nil {
		if n, ok := builtinAliasToNumber(strings.ToLower(token)); ok {
			return n, true
		}
		return 0, false
	}
	return n, true
}
