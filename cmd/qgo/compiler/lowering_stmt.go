package compiler

import (
	"go/ast"
	"go/token"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

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
	case *ast.TypeSwitchStmt:
		l.lowerTypeSwitchStmt(s)
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

func (l *Lowerer) lowerTypeSwitchStmt(s *ast.TypeSwitchStmt) {
	l.errors.Addf(
		l.pos(s),
		"unsupported type switch statement: switch v := x.(type) is deferred",
	)
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
	if len(s.Lhs) != len(s.Rhs) {
		l.errors.Addf(
			l.pos(s),
			"unsupported assignment arity: %d values on left, %d on right (multi-value assignment lowering is not implemented)",
			len(s.Lhs),
			len(s.Rhs),
		)
		return
	}
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

	if s.Tag == nil {
		l.errors.Addf(l.pos(s), "unsupported switch form: tagless switch")
		return
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
	ident, ok := s.X.(*ast.Ident)
	if !ok {
		l.errors.Addf(l.pos(s.X), "increment/decrement requires an assignable identifier")
		return
	}

	obj := l.currentInfo.Uses[ident]
	if obj == nil {
		obj = l.currentInfo.Defs[ident]
	}
	if obj == nil {
		l.errors.Addf(l.pos(s.X), "undefined identifier in increment/decrement: %s", ident.Name)
		return
	}

	operand, ok := l.vregMap[obj]
	if !ok {
		l.errors.Addf(l.pos(s.X), "increment/decrement requires a writable local identifier: %s", ident.Name)
		return
	}
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
