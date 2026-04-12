package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

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

	// Check for builtin directives in preceding comments.
	// A function may carry at most one valid builtin mapping.
	if fd.Doc != nil {
		seenBuiltin := false
		seenBuiltinNum := 0
		for _, c := range fd.Doc.List {
			num, matched, errMsg := parseBuiltinDirective(c.Text, builtinDirectiveRegistry)
			if !matched {
				continue
			}
			if errMsg != "" {
				l.errors.Add(l.pos(c), errMsg)
				continue
			}
			if !seenBuiltin {
				seenBuiltin = true
				seenBuiltinNum = num
				fn.IsBuiltin = true
				fn.BuiltinNum = num
				continue
			}
			if num == seenBuiltinNum {
				l.errors.Addf(l.pos(c), "duplicate //qgo:builtin directive for %s (builtin %d)", fd.Name.Name, num)
				continue
			}
			l.errors.Addf(l.pos(c), "ambiguous //qgo:builtin directives for %s: %d and %d", fd.Name.Name, seenBuiltinNum, num)
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
