package compiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"

	"github.com/ironwail/ironwail-go/internal/qc"
)

// Compiler orchestrates the full Go → progs.dat compilation pipeline.
type Compiler struct {
	Verbose bool
	synth   *SyntheticPackages
}

// New creates a new Compiler instance.
func New() *Compiler {
	return &Compiler{
		synth: NewSyntheticPackages(),
	}
}

// Compile compiles a Go package directory into a progs.dat binary.
func (c *Compiler) Compile(dir string) ([]byte, error) {
	fset := token.NewFileSet()

	// Parse all Go files in the directory
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Find the target package (should be exactly one)
	var pkg *ast.Package
	for _, p := range pkgs {
		pkg = p
		break
	}
	if pkg == nil {
		return nil, &CompileError{Msg: "no Go packages found in " + dir}
	}

	// Collect files for type-checking
	var files []*ast.File
	for _, f := range pkg.Files {
		files = append(files, f)
	}

	// Type-check with synthetic importer
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	conf := types.Config{
		Importer: NewSyntheticImporter(c.synth),
	}

	_, err = conf.Check(pkg.Name, fset, files, info)
	if err != nil {
		return nil, err
	}

	// Lower AST → IR
	lowerer := NewLowerer(c.synth, info, fset)
	irProg, err := lowerer.Lower(pkg)
	if err != nil {
		return nil, err
	}

	// Register builtins from synthetic packages
	c.registerBuiltins(irProg)

	// Code generation: IR → QCVM
	globals := NewGlobalAllocator()
	strings := NewStringTable()
	codegen := NewCodeGen(globals, strings)

	emitInput, err := codegen.Generate(irProg)
	if err != nil {
		return nil, err
	}

	// Emit binary
	return Emit(emitInput)
}

func (c *Compiler) registerBuiltins(prog *IRProgram) {
	for fn, num := range c.synth.Builtins {
		// Check if any IR function references this builtin
		prog.Functions = append(prog.Functions, IRFunc{
			Name:       fn.Name(),
			QCName:     fn.Name(),
			IsBuiltin:  true,
			BuiltinNum: num,
			Params:     sigToIRParams(fn.Type().(*types.Signature)),
			ReturnType: c.sigReturnType(fn.Type().(*types.Signature)),
		})
	}
}

func sigToIRParams(sig *types.Signature) []IRParam {
	var params []IRParam
	for i := range sig.Params().Len() {
		p := sig.Params().At(i)
		params = append(params, IRParam{
			Name: p.Name(),
			Type: goBasicTypeToQC(p.Type()),
		})
	}
	return params
}

func (c *Compiler) sigReturnType(sig *types.Signature) qc.EType {
	if sig.Results().Len() == 0 {
		return EvVoid
	}
	return goBasicTypeToQC(sig.Results().At(0).Type())
}

func goBasicTypeToQC(t types.Type) qc.EType {
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
		return goBasicTypeToQC(named.Underlying())
	}
	switch bt := t.Underlying().(type) {
	case *types.Basic:
		switch bt.Kind() {
		case types.Float32, types.Float64, types.UntypedFloat,
			types.Int, types.Int32, types.Int64, types.UntypedInt,
			types.Bool, types.UntypedBool:
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
	return EvFloat
}
