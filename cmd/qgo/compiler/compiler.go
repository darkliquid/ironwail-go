package compiler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

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

	// Parse all Go and qgo files in the directory
	var files []*ast.File
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	pkgName := ""
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".qgo") {
			continue
		}

		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}

		if pkgName == "" {
			pkgName = f.Name.Name
		} else if pkgName != f.Name.Name {
			// Skip files from different packages in the same directory (standard Go behavior)
			continue
		}

		files = append(files, f)
	}

	if len(files) == 0 {
		return nil, &CompileError{Msg: "no Go or qgo files found in " + dir}
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

	_, err = conf.Check(pkgName, fset, files, info)
	if err != nil {
		return nil, err
	}

	// Lower AST → IR
	lowerer := NewLowerer(c.synth, info, fset)
	irProg, err := lowerer.Lower(files)
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
