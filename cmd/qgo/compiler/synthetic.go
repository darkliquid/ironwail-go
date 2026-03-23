package compiler

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
)

// BuiltinDef describes a single engine builtin function.
type BuiltinDef struct {
	Name       string
	BuiltinNum int
	Params     []*types.Var    // parameter vars
	Results    []*types.Var    // result vars
}

// SyntheticPackages holds the synthetic quake and quake/engine packages
// used for type-checking QGo programs.
type SyntheticPackages struct {
	Quake       *types.Package
	QuakeEngine *types.Package

	// Types from the quake package
	Vec3Type        *types.Named
	EntityType      *types.Named
	FuncType        *types.Named
	FieldOffsetType *types.Named

	// Builtin mapping: *types.Func -> builtin number
	Builtins map[*types.Func]int
}

// NewSyntheticPackages creates the synthetic quake and quake/engine packages.
func NewSyntheticPackages() *SyntheticPackages {
	sp := &SyntheticPackages{
		Builtins: make(map[*types.Func]int),
	}
	sp.buildQuakePackage()
	sp.buildEnginePackage()
	return sp
}

func (sp *SyntheticPackages) buildQuakePackage() {
	pkg := types.NewPackage("quake", "quake")
	sp.Quake = pkg

	// type Vec3 [3]float32
	vec3Under := types.NewArray(types.Typ[types.Float32], 3)
	sp.Vec3Type = types.NewNamed(
		types.NewTypeName(token.NoPos, pkg, "Vec3", nil),
		vec3Under, nil,
	)
	pkg.Scope().Insert(sp.Vec3Type.Obj())

	// type Entity uintptr
	sp.EntityType = types.NewNamed(
		types.NewTypeName(token.NoPos, pkg, "Entity", nil),
		types.Typ[types.Uintptr], nil,
	)
	pkg.Scope().Insert(sp.EntityType.Obj())

	// type Func func()
	funcSig := types.NewSignatureType(nil, nil, nil, nil, nil, false)
	sp.FuncType = types.NewNamed(
		types.NewTypeName(token.NoPos, pkg, "Func", nil),
		funcSig, nil,
	)
	pkg.Scope().Insert(sp.FuncType.Obj())

	// type FieldOffset int
	sp.FieldOffsetType = types.NewNamed(
		types.NewTypeName(token.NoPos, pkg, "FieldOffset", nil),
		types.Typ[types.Int], nil,
	)
	pkg.Scope().Insert(sp.FieldOffsetType.Obj())

	// const World Entity = 0
	worldConst := types.NewConst(
		token.NoPos, pkg, "World",
		sp.EntityType,
		makeIntConst(0),
	)
	pkg.Scope().Insert(worldConst)

	// func Error(msg string)
	errorFunc := types.NewFunc(token.NoPos, pkg, "Error",
		types.NewSignatureType(nil, nil, nil,
			types.NewTuple(types.NewVar(token.NoPos, pkg, "msg", types.Typ[types.String])),
			nil, false,
		),
	)
	pkg.Scope().Insert(errorFunc)

	// func MakeVec3(x, y, z float32) Vec3
	makeVec3 := types.NewFunc(token.NoPos, pkg, "MakeVec3",
		types.NewSignatureType(nil, nil, nil,
			types.NewTuple(
				types.NewVar(token.NoPos, pkg, "x", types.Typ[types.Float32]),
				types.NewVar(token.NoPos, pkg, "y", types.Typ[types.Float32]),
				types.NewVar(token.NoPos, pkg, "z", types.Typ[types.Float32]),
			),
			types.NewTuple(types.NewVar(token.NoPos, pkg, "", sp.Vec3Type)),
			false,
		),
	)
	pkg.Scope().Insert(makeVec3)

	pkg.MarkComplete()
}

func (sp *SyntheticPackages) buildEnginePackage() {
	pkg := types.NewPackage("quake/engine", "engine")
	sp.QuakeEngine = pkg

	// Define all builtins
	builtins := []BuiltinDef{
		{Name: "MakeVectors", BuiltinNum: 1, Params: sp.params(pkg, "ang", sp.Vec3Type)},
		{Name: "SetOrigin", BuiltinNum: 2, Params: sp.params(pkg, "ent", sp.EntityType, "org", sp.Vec3Type)},
		{Name: "SetModel", BuiltinNum: 3, Params: sp.params(pkg, "ent", sp.EntityType, "model", types.Typ[types.String])},
		{Name: "SetSize", BuiltinNum: 4, Params: sp.params(pkg, "ent", sp.EntityType, "min", sp.Vec3Type, "max", sp.Vec3Type)},
		{Name: "BreakStatement", BuiltinNum: 7},
		{Name: "Random", BuiltinNum: 8, Results: sp.results(pkg, types.Typ[types.Float32])},
		{Name: "Sound", BuiltinNum: 9, Params: sp.params(pkg,
			"ent", sp.EntityType, "channel", types.Typ[types.Float32],
			"sample", types.Typ[types.String], "volume", types.Typ[types.Float32],
			"attenuation", types.Typ[types.Float32])},
		{Name: "Normalize", BuiltinNum: 12, Params: sp.params(pkg, "v", sp.Vec3Type), Results: sp.results(pkg, sp.Vec3Type)},
		{Name: "Spawn", BuiltinNum: 14, Results: sp.results(pkg, sp.EntityType)},
		{Name: "Remove", BuiltinNum: 15, Params: sp.params(pkg, "ent", sp.EntityType)},
		{Name: "TraceLine", BuiltinNum: 16, Params: sp.params(pkg,
			"v1", sp.Vec3Type, "v2", sp.Vec3Type,
			"nomonsters", types.Typ[types.Float32], "forent", sp.EntityType)},
		{Name: "BPrint", BuiltinNum: 23, Params: sp.params(pkg, "msg", types.Typ[types.String])},
		{Name: "SPrint", BuiltinNum: 24, Params: sp.params(pkg, "ent", sp.EntityType, "msg", types.Typ[types.String])},
		{Name: "Ftos", BuiltinNum: 26, Params: sp.params(pkg, "f", types.Typ[types.Float32]), Results: sp.results(pkg, types.Typ[types.String])},
		{Name: "Vtos", BuiltinNum: 27, Params: sp.params(pkg, "v", sp.Vec3Type), Results: sp.results(pkg, types.Typ[types.String])},
		{Name: "CvarGet", BuiltinNum: 45, Params: sp.params(pkg, "name", types.Typ[types.String]), Results: sp.results(pkg, types.Typ[types.String])},
		{Name: "CvarSet", BuiltinNum: 72, Params: sp.params(pkg, "name", types.Typ[types.String], "value", types.Typ[types.String])},
		// Additional common builtins
		{Name: "VLen", BuiltinNum: 13, Params: sp.params(pkg, "v", sp.Vec3Type), Results: sp.results(pkg, types.Typ[types.Float32])},
		{Name: "Find", BuiltinNum: 18, Params: sp.params(pkg,
			"start", sp.EntityType, "fld", sp.FieldOffsetType, "match", types.Typ[types.String]),
			Results: sp.results(pkg, sp.EntityType)},
		{Name: "DPrint", BuiltinNum: 25, Params: sp.params(pkg, "msg", types.Typ[types.String])},
		{Name: "Ceil", BuiltinNum: 38, Params: sp.params(pkg, "f", types.Typ[types.Float32]), Results: sp.results(pkg, types.Typ[types.Float32])},
		{Name: "Floor", BuiltinNum: 39, Params: sp.params(pkg, "f", types.Typ[types.Float32]), Results: sp.results(pkg, types.Typ[types.Float32])},
		{Name: "FAbs", BuiltinNum: 43, Params: sp.params(pkg, "f", types.Typ[types.Float32]), Results: sp.results(pkg, types.Typ[types.Float32])},
		{Name: "RInt", BuiltinNum: 36, Params: sp.params(pkg, "f", types.Typ[types.Float32]), Results: sp.results(pkg, types.Typ[types.Float32])},
		{Name: "Strcat", BuiltinNum: 115, Params: sp.params(pkg, "a", types.Typ[types.String], "b", types.Typ[types.String]), Results: sp.results(pkg, types.Typ[types.String])},
		{Name: "Strlen", BuiltinNum: 114, Params: sp.params(pkg, "s", types.Typ[types.String]), Results: sp.results(pkg, types.Typ[types.Float32])},
		{Name: "Stof", BuiltinNum: 81, Params: sp.params(pkg, "s", types.Typ[types.String]), Results: sp.results(pkg, types.Typ[types.Float32])},
	}

	for _, b := range builtins {
		fn := sp.makeBuiltinFunc(pkg, b)
		pkg.Scope().Insert(fn)
		sp.Builtins[fn] = b.BuiltinNum
	}

	// Engine global variables (accessed as engine.Self, engine.Time, etc.)
	pkg.Scope().Insert(types.NewVar(token.NoPos, pkg, "Self", sp.EntityType))
	pkg.Scope().Insert(types.NewVar(token.NoPos, pkg, "Other", sp.EntityType))
	pkg.Scope().Insert(types.NewVar(token.NoPos, pkg, "World", sp.EntityType))
	pkg.Scope().Insert(types.NewVar(token.NoPos, pkg, "Time", types.Typ[types.Float32]))
	pkg.Scope().Insert(types.NewVar(token.NoPos, pkg, "FrameTime", types.Typ[types.Float32]))
	pkg.Scope().Insert(types.NewVar(token.NoPos, pkg, "MapName", types.Typ[types.String]))

	pkg.MarkComplete()
}

func (sp *SyntheticPackages) params(pkg *types.Package, nameTypePairs ...any) []*types.Var {
	var vars []*types.Var
	for i := 0; i < len(nameTypePairs); i += 2 {
		name := nameTypePairs[i].(string)
		typ := nameTypePairs[i+1].(types.Type)
		vars = append(vars, types.NewVar(token.NoPos, pkg, name, typ))
	}
	return vars
}

func (sp *SyntheticPackages) results(pkg *types.Package, typs ...types.Type) []*types.Var {
	var vars []*types.Var
	for _, t := range typs {
		vars = append(vars, types.NewVar(token.NoPos, pkg, "", t))
	}
	return vars
}

func (sp *SyntheticPackages) makeBuiltinFunc(pkg *types.Package, b BuiltinDef) *types.Func {
	var paramTuple, resultTuple *types.Tuple
	if len(b.Params) > 0 {
		paramTuple = types.NewTuple(b.Params...)
	}
	if len(b.Results) > 0 {
		resultTuple = types.NewTuple(b.Results...)
	}
	sig := types.NewSignatureType(nil, nil, nil, paramTuple, resultTuple, false)
	return types.NewFunc(token.NoPos, pkg, b.Name, sig)
}

func makeIntConst(val int64) constant.Value {
	return constant.MakeInt64(val)
}

// SyntheticImporter implements types.Importer for QGo programs.
type SyntheticImporter struct {
	pkgs *SyntheticPackages
}

// NewSyntheticImporter creates an importer that provides the quake packages.
func NewSyntheticImporter(pkgs *SyntheticPackages) *SyntheticImporter {
	return &SyntheticImporter{pkgs: pkgs}
}

// Import implements types.Importer.
func (si *SyntheticImporter) Import(path string) (*types.Package, error) {
	switch path {
	case "quake":
		return si.pkgs.Quake, nil
	case "quake/engine":
		return si.pkgs.QuakeEngine, nil
	default:
		return nil, fmt.Errorf("unsupported import path: %q (only \"quake\" and \"quake/engine\" are available)", path)
	}
}
