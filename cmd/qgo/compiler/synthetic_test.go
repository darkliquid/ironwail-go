package compiler

import (
	"go/types"
	"testing"
)

func TestSyntheticPackages_QuakeTypes(t *testing.T) {
	sp := NewSyntheticPackages()

	// Check quake package has expected types
	scope := sp.Quake.Scope()
	for _, name := range []string{"Vec3", "Entity", "Func", "FieldOffset"} {
		obj := scope.Lookup(name)
		if obj == nil {
			t.Errorf("quake.%s not found", name)
		}
	}

	// Check World constant
	world := scope.Lookup("World")
	if world == nil {
		t.Fatal("quake.World not found")
	}
	if _, ok := world.(*types.Const); !ok {
		t.Error("World should be a constant")
	}
}

func TestSyntheticPackages_EngineBuiltins(t *testing.T) {
	sp := NewSyntheticPackages()

	// Check engine package has builtins
	scope := sp.QuakeEngine.Scope()
	builtinNames := []string{"Spawn", "Remove", "Random", "SetOrigin", "MakeVectors"}
	for _, name := range builtinNames {
		obj := scope.Lookup(name)
		if obj == nil {
			t.Errorf("engine.%s not found", name)
			continue
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			t.Errorf("engine.%s should be a function", name)
			continue
		}
		if _, ok := sp.Builtins[fn]; !ok {
			t.Errorf("engine.%s not in builtins map", name)
		}
	}

	// Verify specific builtin numbers
	spawn := scope.Lookup("Spawn").(*types.Func)
	if sp.Builtins[spawn] != 14 {
		t.Errorf("Spawn builtin = %d, want 14", sp.Builtins[spawn])
	}
}

func TestSyntheticPackages_EngineGlobals(t *testing.T) {
	sp := NewSyntheticPackages()
	scope := sp.QuakeEngine.Scope()

	for _, name := range []string{"Self", "Other", "World", "Time", "FrameTime", "MapName"} {
		obj := scope.Lookup(name)
		if obj == nil {
			t.Errorf("engine.%s not found", name)
		}
	}
}

func TestSyntheticImporter(t *testing.T) {
	sp := NewSyntheticPackages()
	imp := NewSyntheticImporter(sp)

	pkg, err := imp.Import("quake")
	if err != nil {
		t.Fatalf("import quake: %v", err)
	}
	if pkg.Path() != "quake" {
		t.Errorf("path = %q, want %q", pkg.Path(), "quake")
	}

	pkg, err = imp.Import("quake/engine")
	if err != nil {
		t.Fatalf("import quake/engine: %v", err)
	}
	if pkg.Path() != "quake/engine" {
		t.Errorf("path = %q, want %q", pkg.Path(), "quake/engine")
	}

	_, err = imp.Import("fmt")
	if err == nil {
		t.Error("import fmt should fail")
	}
}
