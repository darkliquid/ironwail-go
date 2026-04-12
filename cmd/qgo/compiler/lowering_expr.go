package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

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
	case *ast.TypeAssertExpr:
		return l.lowerTypeAssertExpr(e)

	default:
		l.errors.Addf(l.pos(expr), "unsupported expression type: %T", expr)
		return VRegInvalid
	}
}

func (l *Lowerer) lowerTypeAssertExpr(expr *ast.TypeAssertExpr) VReg {
	l.errors.Addf(
		l.pos(expr),
		"unsupported type assertion expression: x.(T) is deferred",
	)
	return VRegInvalid
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

func (l *Lowerer) lowerTypeConversionExpr(fn *IRFunc, call *ast.CallExpr) (VReg, bool) {
	tv, ok := l.currentInfo.Types[call.Fun]
	if !ok || !tv.IsType() {
		return VRegInvalid, false
	}
	if len(call.Args) != 1 {
		l.errors.Addf(l.pos(call), "type conversion expects 1 arg, got %d", len(call.Args))
		return VRegInvalid, true
	}

	targetType := tv.Type
	sourceType := l.currentInfo.Types[call.Args[0]].Type
	if targetType == nil || sourceType == nil {
		l.errors.Addf(l.pos(call), "cannot resolve type conversion")
		return VRegInvalid, true
	}

	targetQC := l.goTypeToQC(targetType)
	sourceQC := l.goTypeToQC(sourceType)
	if targetQC != sourceQC {
		if l.isEquivalentEntityPointerConversion(sourceType, targetType) {
			return l.lowerExpr(fn, call.Args[0]), true
		}
		l.errors.Addf(l.pos(call), "unsupported type conversion from %s to %s", sourceType.String(), targetType.String())
		return VRegInvalid, true
	}

	return l.lowerExpr(fn, call.Args[0]), true
}

func (l *Lowerer) isEquivalentEntityPointerConversion(sourceType, targetType types.Type) bool {
	srcPtr, ok := sourceType.(*types.Pointer)
	if !ok {
		return false
	}
	dstPtr, ok := targetType.(*types.Pointer)
	if !ok {
		return false
	}
	if !types.Identical(srcPtr.Elem().Underlying(), dstPtr.Elem().Underlying()) {
		return false
	}

	srcNamed, ok := srcPtr.Elem().(*types.Named)
	if ok && srcNamed.Obj().Name() == "Entity" {
		return true
	}
	dstNamed, ok := dstPtr.Elem().(*types.Named)
	return ok && dstNamed.Obj().Name() == "Entity"
}
