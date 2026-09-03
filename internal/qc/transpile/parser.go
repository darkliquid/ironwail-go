package transpile

import (
	"fmt"
	"strings"
)

// QC operator precedence, loosest to tightest. QC's `&` is logical AND on
// floats (classic) or bitwise AND (rerelease extensions add `&&`/`||` as
// short-circuit forms, which bind looser than single `&` in C-like grammars).
//
//	ternary  ?:            (rerelease)
//	||                     (rerelease)
//	&&                     (rerelease)
//	&                      (logical AND in classic; bitwise on flags)
//	== !=
//	< > <= >=
//	+ -
//	* /
//	unary ! -
//	postfix .field call

// parser walks the token stream and produces the QC AST.
type parser struct {
	toks []Token
	pos  int
	line int
}

func (p *parser) peek() Token { return p.toks[p.pos] }

func (p *parser) peekAt(off int) Token {
	if p.pos+off >= len(p.toks) {
		return p.toks[len(p.toks)-1] // EOF
	}
	return p.toks[p.pos+off]
}

func (p *parser) next() Token {
	t := p.toks[p.pos]
	if t.Kind != TokEOF {
		p.pos++
	}
	p.line = t.Line
	return t
}

func (p *parser) atEOF() bool { return p.peek().Kind == TokEOF }

// isPunct reports whether the current token is the given punctuation.
func (p *parser) isPunct(tok string) bool {
	t := p.peek()
	return t.Kind == TokPunct && t.Text == tok
}

// accept consumes tok if present.
func (p *parser) accept(tok string) bool {
	if p.isPunct(tok) {
		p.pos++
		return true
	}
	return false
}

// expect consumes a punctuation token or errors.
func (p *parser) expect(tok string) error {
	t := p.next()
	if t.Kind != TokPunct || t.Text != tok {
		return fmt.Errorf("line %d: expected %q, got %q", t.Line, tok, t.Text)
	}
	return nil
}

// Parse turns QuakeC source into top-level declarations.
func Parse(src string) ([]Decl, error) {
	toks, err := Lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}

	var decls []Decl
	for !p.atEOF() {
		d, err := p.parseDecl()
		if err != nil {
			return nil, err
		}
		if d != nil {
			decls = append(decls, d)
		}
	}
	return decls, nil
}

// parseDecl handles the top-level forms:
//
//	.type name;                        field declaration
//	type name [= value];               global
//	type([params]) name = {body};      function
//	type([params]) name = expr;        constant function alias
func (p *parser) parseDecl() (Decl, error) {
	if p.accept(";") {
		return nil, nil // stray separator
	}

	// `.type name;` field declaration: '.' then type then name.
	if p.isPunct(".") {
		p.next()
		typ := p.next()
		if typ.Kind != TokTypeName {
			return nil, fmt.Errorf("line %d: expected type after '.', got %q", typ.Line, typ.Text)
		}
		name := p.next()
		if name.Kind != TokIdent && name.Kind != TokTypeName {
			return nil, fmt.Errorf("line %d: expected field name, got %q", name.Line, name.Text)
		}
		if err := p.expect(";"); err != nil {
			return nil, err
		}
		return &FieldDecl{QCType: typ.Text, Name: name.Text}, nil
	}

	typ := p.next()
	if typ.Kind != TokTypeName {
		// Unknown top-level construct: skip to the next ';' to resync, so one
		// unsupported form does not abort the whole file.
		if err := p.skipToSemicolon(typ); err != nil {
			return nil, err
		}
		return nil, nil
	}

	// Function form: type ( [params] ) name = ...
	if p.accept("(") {
		params, err := p.parseParams()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		name := p.next()
		if name.Kind != TokIdent && name.Kind != TokTypeName {
			return nil, fmt.Errorf("line %d: expected function name, got %q", name.Line, name.Text)
		}
		// The `=` is optional in the rerelease QC grammar: both
		// `void() name = { ... };` and `void() name { ... };` are valid.
		p.accept("=")

		fn := &FuncDecl{QCType: typ.Text, Name: name.Text, Params: params}

		if p.isPunct("{") {
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			fn.Body = body.List
		} else {
			// Constant function alias: `void() foo = sub;`
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if ident, ok := e.(*Ident); ok {
				fn.Alias = ident.Name
			} else {
				fn.Alias = "<expr>"
			}
		}
		// The trailing ; is conventional but optional (real sources
		// frequently omit it after a braced body).
		p.accept(";")
		return fn, nil
	}

	// Global form: type name [= value];
	name := p.next()
	if name.Kind != TokIdent && name.Kind != TokTypeName {
		return nil, fmt.Errorf("line %d: expected global name, got %q", name.Line, name.Text)
	}
	g := &GlobalDecl{QCType: typ.Text, Name: name.Text}
	if p.accept("=") {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		g.Value = exprText(e)
	}
	if err := p.expect(";"); err != nil {
		return nil, err
	}
	return g, nil
}

// skipToSemicolon consumes tokens through the next ';' (tracking braces so a
// missing ';' inside a block cannot swallow the rest of the file), reporting
// the resync line on error.
func (p *parser) skipToSemicolon(start Token) error {
	depth := 0
	for !p.atEOF() {
		t := p.next()
		if t.Kind != TokPunct {
			continue
		}
		switch t.Text {
		case "{":
			depth++
		case "}":
			if depth > 0 {
				depth--
			}
		case ";":
			if depth == 0 {
				return nil
			}
		}
	}
	return fmt.Errorf("line %d: unterminated declaration", start.Line)
}

// parseParams parses an optional `[type name, type name]` parameter list.
func (p *parser) parseParams() ([]Param, error) {
	var params []Param
	for !p.isPunct(")") && !p.atEOF() {
		typ := p.next()
		if typ.Kind != TokTypeName {
			return nil, fmt.Errorf("line %d: expected parameter type, got %q", typ.Line, typ.Text)
		}
		name := p.next()
		if name.Kind != TokIdent && name.Kind != TokTypeName {
			return nil, fmt.Errorf("line %d: expected parameter name, got %q", name.Line, name.Text)
		}
		params = append(params, Param{QCType: typ.Text, Name: name.Text})
		if !p.accept(",") {
			break
		}
	}
	return params, nil
}

// parseBlock parses `{ stmt... }`.
func (p *parser) parseBlock() (*BlockStmt, error) {
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	blk := &BlockStmt{}
	for !p.isPunct("}") && !p.atEOF() {
		st, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		if st != nil {
			blk.List = append(blk.List, st)
		}
	}
	if err := p.expect("}"); err != nil {
		return nil, err
	}
	return blk, nil
}

// isTypeToken reports whether the upcoming token declares a local's type.
// QC locals are written `local float x;`; the rerelease also allows dropping
// `local` inside functions, which the parser detects by `type ident` followed
// by ';' or '='. Expression statements never start with a bare type name.
func (p *parser) atLocalDecl() bool {
	t := p.peek()
	if t.Text == "local" && t.Kind == TokIdent {
		return true
	}
	if t.Kind != TokTypeName {
		return false
	}
	n1 := p.peekAt(1)
	n2 := p.peekAt(2)
	if n1.Kind != TokIdent && n1.Kind != TokTypeName {
		return false
	}
	return (n2.Kind == TokPunct && (n2.Text == ";" || n2.Text == "," || n2.Text == "="))
}

// parseStmt parses one statement.
func (p *parser) parseStmt() (Stmt, error) {
	t := p.peek()

	switch {
	case p.isPunct(";"):
		p.next()
		return nil, nil

	case p.isPunct("{"):
		return p.parseBlock()

	case t.Kind == TokIdent && t.Text == "if":
		return p.parseIf()

	case t.Kind == TokIdent && t.Text == "while":
		p.next()
		if err := p.expect("("); err != nil {
			return nil, err
		}
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		body, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		return &WhileStmt{Cond: cond, Body: body}, nil

	case t.Kind == TokIdent && t.Text == "do":
		p.next()
		body, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		w := p.next()
		if w.Kind != TokIdent || w.Text != "while" {
			return nil, fmt.Errorf("line %d: expected 'while' after do-body", w.Line)
		}
		if err := p.expect("("); err != nil {
			return nil, err
		}
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		if err := p.expect(";"); err != nil {
			return nil, err
		}
		return &DoWhileStmt{Body: body, Cond: cond}, nil

	case t.Kind == TokIdent && t.Text == "for":
		return p.parseFor()

	case t.Kind == TokIdent && t.Text == "return":
		p.next()
		if !p.isPunct(";") {
			val, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expect(";"); err != nil {
				return nil, err
			}
			return &ReturnStmt{Value: val}, nil
		}
		p.next()
		return &ReturnStmt{}, nil

	case t.Kind == TokIdent && (t.Text == "break" || t.Text == "continue"):
		p.next()
		if err := p.expect(";"); err != nil {
			return nil, err
		}
		return &ExprStmt{X: &Ident{Name: t.Text}}, nil // mapped by the emitter

	case p.atLocalDecl():
		return p.parseLocal()

	default:
		// Expression or assignment statement.
		lhs, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if p.isPunct("=") {
			p.next()
			rhs, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expect(";"); err != nil {
				return nil, err
			}
			return &AssignStmt{LHS: lhs, RHS: rhs}, nil
		}
		for _, compound := range []struct{ tok, op string }{
			{"+=", "+"}, {"-=", "-"}, {"*=", "*"}, {"/=", "/"},
		} {
			if p.isPunct(compound.tok) {
				p.next()
				rhs, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				if err := p.expect(";"); err != nil {
					return nil, err
				}
				return &AssignStmt{LHS: lhs, RHS: &Binary{Op: compound.op, LHS: lhs, RHS: rhs}}, nil
			}
		}
		if err := p.expect(";"); err != nil {
			return nil, err
		}
		return &ExprStmt{X: lhs}, nil
	}
}

// parseLocal parses `local type a, b = 3;` (the `local` keyword may already
// be consumed by the caller).
func (p *parser) parseLocal() (Stmt, error) {
	if p.peek().Text == "local" {
		p.next()
	}
	typ := p.next()
	if typ.Kind != TokTypeName {
		return nil, fmt.Errorf("line %d: expected local type, got %q", typ.Line, typ.Text)
	}

	// Emit as an assignment-free declaration list; the emitter lowers each
	// name into a `var name T` declaration. Initialisers become assignments.
	var stmts []Stmt
	for {
		name := p.next()
		if name.Kind != TokIdent && name.Kind != TokTypeName {
			return nil, fmt.Errorf("line %d: expected local name, got %q", name.Line, name.Text)
		}
		l := &LocalDecl{QCType: typ.Text, Name: name.Text}
		if p.accept("=") {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			l.Init = exprText(e)
		}
		stmts = append(stmts, l)
		if !p.accept(",") {
			break
		}
	}
	if err := p.expect(";"); err != nil {
		return nil, err
	}
	return &LocalList{Decls: stmts}, nil
}

// parseFor parses `for (init; cond; post) body`.
func (p *parser) parseFor() (Stmt, error) {
	p.next()
	if err := p.expect("("); err != nil {
		return nil, err
	}
	f := &ForStmt{}

	if !p.isPunct(";") {
		init, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		f.Init = init
	} else {
		p.next() // empty init
	}

	if !p.isPunct(";") {
		cond, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		f.Cond = cond
	}
	if err := p.expect(";"); err != nil {
		return nil, err
	}

	if !p.isPunct(")") {
		post, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		f.Post = post
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}

	body, err := p.parseStmt()
	if err != nil {
		return nil, err
	}
	f.Body = body
	return f, nil
}

// parseIf parses `if (cond) stmt [else stmt]`, with `else if` chaining.
func (p *parser) parseIf() (Stmt, error) {
	p.next() // if
	if err := p.expect("("); err != nil {
		return nil, err
	}
	cond, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	thenStmt, err := p.parseStmt()
	if err != nil {
		return nil, err
	}
	st := &IfStmt{Cond: cond, Then: thenStmt}

	if e := p.peek(); e.Kind == TokIdent && e.Text == "else" {
		p.next()
		elseStmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		st.Else = elseStmt
	}
	return st, nil
}

// parseExpr parses a full expression (ternary level).
func (p *parser) parseExpr() (Expr, error) {
	cond, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.isPunct("?") {
		p.next()
		te, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if err := p.expect(":"); err != nil {
			return nil, err
		}
		ee, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return &Ternary{Cond: cond, Then: te, Else: ee}, nil
	}
	return cond, nil
}

// parseOr handles `||`.
func (p *parser) parseOr() (Expr, error) {
	lhs, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.isPunct("||") {
		p.next()
		rhs, err := p.parseBitOr()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: "||", LHS: lhs, RHS: rhs}
	}
	return lhs, nil
}

// parseAnd handles `&&`.
func (p *parser) parseAnd() (Expr, error) {
	lhs, err := p.parseBitOr()
	if err != nil {
		return nil, err
	}
	for p.isPunct("&&") {
		p.next()
		rhs, err := p.parseBitOr()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: "&&", LHS: lhs, RHS: rhs}
	}
	return lhs, nil
}

// parseBitOr handles single `|` (bitwise OR; classic QC logical OR).
func (p *parser) parseBitOr() (Expr, error) {
	lhs, err := p.parseBitAnd()
	if err != nil {
		return nil, err
	}
	for p.isPunct("|") {
		p.next()
		rhs, err := p.parseBitAnd()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: "|", LHS: lhs, RHS: rhs}
	}
	return lhs, nil
}

// parseBitAnd handles single `&` (bitwise / classic QC logical AND).
func (p *parser) parseBitAnd() (Expr, error) {
	lhs, err := p.parseEq()
	if err != nil {
		return nil, err
	}
	for p.isPunct("&") {
		p.next()
		rhs, err := p.parseEq()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: "&", LHS: lhs, RHS: rhs}
	}
	return lhs, nil
}

// parseEq handles `==` and `!=`.
func (p *parser) parseEq() (Expr, error) {
	lhs, err := p.parseRel()
	if err != nil {
		return nil, err
	}
	for {
		var op string
		switch {
		case p.isPunct("=="):
			op = "=="
		case p.isPunct("!="):
			op = "!="
		default:
			return lhs, nil
		}
		p.next()
		rhs, err := p.parseRel()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: op, LHS: lhs, RHS: rhs}
	}
}

// parseRel handles `< > <= >=`.
func (p *parser) parseRel() (Expr, error) {
	lhs, err := p.parseAdd()
	if err != nil {
		return nil, err
	}
	for {
		var op string
		switch {
		case p.isPunct("<"):
			op = "<"
		case p.isPunct(">"):
			op = ">"
		case p.isPunct("<="):
			op = "<="
		case p.isPunct(">="):
			op = ">="
		default:
			return lhs, nil
		}
		p.next()
		rhs, err := p.parseAdd()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: op, LHS: lhs, RHS: rhs}
	}
}

// parseAdd handles `+ -`.
func (p *parser) parseAdd() (Expr, error) {
	lhs, err := p.parseMul()
	if err != nil {
		return nil, err
	}
	for p.isPunct("+") || p.isPunct("-") {
		op := p.next().Text
		rhs, err := p.parseMul()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: op, LHS: lhs, RHS: rhs}
	}
	return lhs, nil
}

// parseMul handles `* /`.
func (p *parser) parseMul() (Expr, error) {
	lhs, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.isPunct("*") || p.isPunct("/") {
		op := p.next().Text
		rhs, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		lhs = &Binary{Op: op, LHS: lhs, RHS: rhs}
	}
	return lhs, nil
}

// parseUnary handles `!` and unary `-`.
func (p *parser) parseUnary() (Expr, error) {
	if p.isPunct("!") || p.isPunct("-") {
		op := p.next().Text
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Unary{Op: op, Operand: operand}, nil
	}
	return p.parsePostfix()
}

// parsePostfix handles `.field` access and calls.
func (p *parser) parsePostfix() (Expr, error) {
	x, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch {
		case p.isPunct("."):
			p.next()
			field := p.next()
			if field.Kind != TokIdent && field.Kind != TokTypeName {
				return nil, fmt.Errorf("line %d: expected field name after '.', got %q", field.Line, field.Text)
			}
			switch recv := x.(type) {
			case *Ident:
				x = &FieldAccess{Recv: recv.Name, Field: field.Text}
			case *FieldAccess:
				// Chained field access: other.enemy.absmin_z.
				// The inner field's Go name becomes the receiver text.
				x = &FieldAccess{Recv: recv.Recv + "." + fieldName(recv.Field), Field: field.Text}
			default:
				return nil, fmt.Errorf("line %d: field access on non-identifier receiver", field.Line)
			}
		case p.isPunct("("):
			p.next()
			var args []Expr
			for !p.isPunct(")") && !p.atEOF() {
				a, err := p.parseExpr()
				if err != nil {
					return nil, err
				}
				args = append(args, a)
				if !p.accept(",") {
					break
				}
			}
			if err := p.expect(")"); err != nil {
				return nil, err
			}
			switch callee := x.(type) {
			case *Ident:
				x = &Call{Name: callee.Name, Args: args}
			case *FieldAccess:
				x = &MethodCall{Recv: callee.Recv, Field: callee.Field, Args: args}
			default:
				return nil, fmt.Errorf("line %d: call on unsupported receiver", p.peek().Line)
			}
		default:
			return x, nil
		}
	}
}

// parsePrimary parses identifiers and literals.
func (p *parser) parsePrimary() (Expr, error) {
	t := p.next()
	switch t.Kind {
	case TokIdent, TokTypeName:
		return &Ident{Name: t.Text}, nil
	case TokNumber:
		return &NumberLit{Text: t.Text}, nil
	case TokString:
		return &StringLit{Value: t.Text}, nil
	case TokVector:
		parts := strings.Fields(t.Text)
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
		return &VectorLit{X: parts[0], Y: parts[1], Z: parts[2]}, nil
	case TokPunct:
		if t.Text == "(" {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if err := p.expect(")"); err != nil {
				return nil, err
			}
			return e, nil
		}
	}
	return nil, fmt.Errorf("line %d: unexpected token %q", t.Line, t.Text)
}

// isPunctTok is a Token method form of isPunct for lookahead.
func (t Token) isPunctTok(tok string) bool {
	return t.Kind == TokPunct && t.Text == tok
}
