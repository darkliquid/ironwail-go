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
