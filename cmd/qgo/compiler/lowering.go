package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"

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
