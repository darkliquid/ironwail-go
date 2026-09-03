package transpile

import (
	"fmt"
	"strings"
)

// QC AST. Deliberately small: it models the constructs that map cleanly to
// QuakeGo and reports (rather than guesses at) everything else, so a human
// refines the output instead of silently losing semantics.

// Decl is a top-level declaration: a field, a global, or a function.
type Decl interface{ isDecl() }

// FieldDecl is `.float message;` — an entity field declaration.
type FieldDecl struct {
	QCType string // float, string, vector, entity
	Name   string
}

// GlobalDecl is `float force_retouch = 2;` — a plain global.
type GlobalDecl struct {
	QCType string
	Name   string
	Value  string // raw initialiser text; empty when absent
}

// FuncDecl is `void() name = { body };` — a function definition.
type FuncDecl struct {
	QCType string // return type: void, float, vector, string, entity
	Name   string
	Params []Param
	Body   []Stmt
	// Alias is set for constant function aliases: `void() foo = sub;`
	Alias string
}

// Param is a function parameter.
type Param struct {
	QCType string
	Name   string
}

func (*FieldDecl) isDecl()  {}
func (*GlobalDecl) isDecl() {}
func (*FuncDecl) isDecl()   {}

// Stmt is a statement.
type Stmt interface{ isStmt() }

// ExprStmt is an expression evaluated for effect (calls, increments).
type ExprStmt struct{ X Expr }

// AssignStmt is `lhs = rhs;`.
type AssignStmt struct{ LHS, RHS Expr }

// IfStmt is `if (cond) Then [else Else]`.
type IfStmt struct {
	Cond Expr
	Then Stmt
	Else Stmt // nil when absent
}

// WhileStmt is `while (cond) Body`.
type WhileStmt struct {
	Cond Expr
	Body Stmt
}

// DoWhileStmt is `do Body while (cond);`.
type DoWhileStmt struct {
	Body Stmt
	Cond Expr
}

// ForStmt is `for (Init; Cond; Post) Body` — rare in QC but legal.
type ForStmt struct {
	Init Stmt // may be nil
	Cond Expr // may be nil (infinite)
	Post Stmt // may be nil
	Body Stmt
}

// ReturnStmt is `return [expr];`.
type ReturnStmt struct{ Value Expr } // nil for bare return

// BlockStmt is a braced statement list.
type BlockStmt struct{ List []Stmt }

// LocalDecl is one `local float name [= init];` declarator.
type LocalDecl struct {
	QCType string
	Name   string
	Init   string // raw initialiser text; empty when absent
}

// LocalList groups the declarators of one `local` statement (QC allows
// `local float a, b;`). The emitter lowers it to one Go var per declarator.
type LocalList struct{ Decls []Stmt }

func (*ExprStmt) isStmt()    {}
func (*AssignStmt) isStmt()  {}
func (*IfStmt) isStmt()      {}
func (*WhileStmt) isStmt()   {}
func (*DoWhileStmt) isStmt() {}
func (*ForStmt) isStmt()     {}
func (*ReturnStmt) isStmt()  {}
func (*BlockStmt) isStmt()   {}
func (*LocalList) isStmt()   {}
func (*LocalDecl) isStmt()   {}

// Expr is an expression node.
type Expr interface{ isExpr() }

// Ident is a bare identifier (global, local, constant).
type Ident struct{ Name string }

// FieldAccess is `recv.field` (recv is Ident-typed in QC: self/other/...).
type FieldAccess struct {
	Recv  string // "self", "other", ... (QC casing preserved)
	Field string // QC field name
}

// StringLit is a string constant.
type StringLit struct{ Value string }

// NumberLit is a numeric constant.
type NumberLit struct{ Text string }

// VectorLit is 'x y z'.
type VectorLit struct{ X, Y, Z string }

// Call is `name(args...)`.
type Call struct {
	Name string
	Args []Expr
}

// Unary is `op operand` (! -).
type Unary struct {
	Op      string // "!" or "-"
	Operand Expr
}

// Binary is `lhs op rhs`. Op is the QC operator text; note classic QC uses
// `&` for logical AND and has no `&&` (rerelease compilers add `&&`/`||`).
type Binary struct {
	Op  string
	LHS Expr
	RHS Expr
}

// MethodCall is `recv.field(args...)` — calling a function stored in an
// entity field (QC function-pointer dispatch).
type MethodCall struct {
	Recv  string
	Field string
	Args  []Expr
}

// Ternary is `cond ? a : b` (supported by rerelease QC compilers).
type Ternary struct {
	Cond, Then, Else Expr
}

func (*Ident) isExpr()       {}
func (*FieldAccess) isExpr() {}
func (*StringLit) isExpr()   {}
func (*NumberLit) isExpr()   {}
func (*VectorLit) isExpr()   {}
func (*Call) isExpr()        {}
func (*Unary) isExpr()       {}
func (*Binary) isExpr()      {}
func (*MethodCall) isExpr()  {}
func (*Ternary) isExpr()     {}

// exprText renders an expression back to canonical QC text (used for raw
// initialiser capture where the emitter re-renders it in Go form anyway).
func exprText(e Expr) string {
	switch x := e.(type) {
	case *Ident:
		return x.Name
	case *NumberLit:
		return x.Text
	case *StringLit:
		return fmt.Sprintf("%q", x.Value)
	case *FieldAccess:
		return x.Recv + "." + x.Field
	case *Binary:
		return fmt.Sprintf("%s %s %s", exprText(x.LHS), x.Op, exprText(x.RHS))
	case *Unary:
		return x.Op + exprText(x.Operand)
	case *Call:
		args := make([]string, len(x.Args))
		for i, a := range x.Args {
			args[i] = exprText(a)
		}
		return fmt.Sprintf("%s(%s)", x.Name, strings.Join(args, ", "))
	case *VectorLit:
		return fmt.Sprintf("'%s %s %s'", x.X, x.Y, x.Z)
	case *Ternary:
		return fmt.Sprintf("%s ? %s : %s", exprText(x.Cond), exprText(x.Then), exprText(x.Else))
	}
	return "<expr>"
}
