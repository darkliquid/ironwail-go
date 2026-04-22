package compiler

// Module, struct-literal, and builtin directive compile tests split from compiler_test.go.

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeQGoModule(t *testing.T, dir, moduleDecl string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), moduleDecl+`

go 1.26
`)
}

func makeCompilerTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "fieldoffset-intrinsic-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

func writeFieldIntrinsicStubPackage(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir quake stub package: %v", err)
	}
	writeFile(t, filepath.Join(dir, "quake.go"), `package quake

type Entity struct{}
type FieldOffset any

func FieldFloat(entity *Entity, args ...any) float32 { return 0 }
func SetFieldFloat(entity *Entity, args ...any) {}
func (e *Entity) FieldFloat(args ...any) float32 { return 0 }
func (e *Entity) SetFieldFloat(args ...any) {}
`)
}

func writeFieldIntrinsicRuntimeStubPackage(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir quake stub package: %v", err)
	}
	writeFile(t, filepath.Join(dir, "quake.go"), `package quake

type FieldOffset any

func FieldFloat(entity any, args ...any) float32 { return 0 }
func SetFieldFloat(entity any, args ...any) {}

type Entity struct{}

func (e *Entity) FieldFloat(args ...any) float32 { return 0 }
func (e *Entity) SetFieldFloat(args ...any) {}
`)
}
