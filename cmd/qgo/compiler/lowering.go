package compiler

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

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

	// Plan 28: function-value and int-literal global cells. These cells are
	// allocated at lowering time with REAL global offsets (>= FreeGlobalBase)
	// so that sub-vregBase VRegs passed to OPAddress/OPCall/OPStorePFNC
	// resolve to int-holding global slots (the VM reads GInt there).
	funcCells    map[types.Object]VReg // func-valued object -> cell VReg (offset)
	constInts    map[int64]VReg        // int literal pool (cell VRegs)
	nextGlobalOfs uint16               // free-global cursor, mirrors GlobalAllocator
	globalVarOfs map[types.Object]uint16 // package-level var -> real global offset
}

// NewLowerer creates a new lowerer.
func NewLowerer() *Lowerer {
	return &Lowerer{
		entityFields:  make(map[types.Type][]IRField),
		fieldOffsets:  make(map[types.Object]uint16),
		funcCells:     make(map[types.Object]VReg),
		constInts:     make(map[int64]VReg),
		globalVarOfs:  make(map[types.Object]uint16),
		nextGlobalOfs: FreeGlobalBase,
	}
}

// allocGlobalOfs advances the lowering-time global cursor and returns the
// next free offset, honoring the system/param window guard (plan D2).
func (l *Lowerer) allocGlobalOfs(slots uint16) uint16 {
	ofs := l.nextGlobalOfs
	if systemParamWindow(ofs, slots) {
		// Backstop: never hand out a slot inside the param/system window.
		l.errors.Addf(token.Position{}, "internal: global offset %d falls in system/param window", ofs)
	}
	l.nextGlobalOfs += slots
	return ofs
}

// LowerPackages processes a collection of packages and returns the IR program.
func (l *Lowerer) LowerPackages(pkgs []*packages.Package) (*IRProgram, error) {
	// Pass 1: declarations and entity structs across the FULL package graph,
	// including imported dependencies. The target package imports `quake`,
	// whose Entity struct carries the //qgo:entity directive that registers
	// every QuakeC entity field; compiled progs without these have
	// num_entityfields=0 and every OPAddress/field access reads outside
	// edict storage. Dependency function *bodies* are never lowered (Pass 2
	// stays target-only).
	allPkgs := flattenPackageGraph(pkgs)
	for _, p := range allPkgs {
		l.currentInfo = p.TypesInfo
		l.currentFset = p.Fset
		target := isTargetPackage(p, pkgs)
		for _, file := range sortedSyntaxFiles(p) {
			l.lowerFileDecls(file, target)
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

// flattenPackageGraph returns the target packages plus every transitively
// imported package, de-duplicated by path, in a deterministic order (targets
// first, then dependencies sorted by PkgPath).
func flattenPackageGraph(pkgs []*packages.Package) []*packages.Package {
	seen := make(map[string]bool)
	var order []*packages.Package
	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if p == nil || p.PkgPath == "" || seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		order = append(order, p)
		paths := make([]string, 0, len(p.Imports))
		for path := range p.Imports {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		for _, path := range paths {
			visit(p.Imports[path])
		}
	}
	for _, p := range pkgs {
		visit(p)
	}
	return order
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

func (l *Lowerer) lowerFileDecls(file *ast.File, target bool) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			l.lowerGenDecl(d)
		case *ast.FuncDecl:
			if !target {
				// Dependency packages: only register builtin-annotated
				// functions (so calls to engine stubs resolve); plain
				// dependency funcs (e.g. quake.Sprintf, a compile-time
				// intrinsic) are not part of the target program and have no
				// emitted body.
				if !hasBuiltinDirective(d) {
					continue
				}
			}
			l.registerFunc(d)
		}
	}
}

func isTargetPackage(p *packages.Package, targets []*packages.Package) bool {
	for _, t := range targets {
		if t == p || (t != nil && p != nil && t.PkgPath == p.PkgPath) {
			return true
		}
	}
	return false
}

// hasBuiltinDirective reports whether a function declaration carries a
// //qgo:builtin directive.
func hasBuiltinDirective(fd *ast.FuncDecl) bool {
	if fd == nil || fd.Doc == nil {
		return false
	}
	for _, c := range fd.Doc.List {
		if strings.Contains(c.Text, "qgo:builtin") {
			return true
		}
	}
	return false
}

func (l *Lowerer) lowerFileFuncs(file *ast.File) {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			l.lowerFuncBody(fd)
		}
	}
}
