package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunCompile_LegacyCLIStillBuilds(t *testing.T) {
	dir := t.TempDir()
	writeMainTestFile(t, filepath.Join(dir, "go.mod"), "module qgolegacycli\n\ngo 1.26\n")
	writeMainTestFile(t, filepath.Join(dir, "main.qgo"), `package main

func Main() float32 { return 1 }
`)

	outputPath := filepath.Join(dir, "progs.dat")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"-o", outputPath, dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty progs.dat output")
	}
}

func TestRunSourceOrder_DefaultTextFunctions(t *testing.T) {
	dir := t.TempDir()
	writeMainTestFile(t, filepath.Join(dir, "go.mod"), "module qgosourceordercli\n\ngo 1.26\n")
	writeMainTestFile(t, filepath.Join(dir, "z_last.qgo"), `package main

func Zed() float32 { return 2 }
`)
	writeMainTestFile(t, filepath.Join(dir, "a_first.qgo"), `package main

func Able() float32 { return 1 }
`)
	writeMainTestFile(t, filepath.Join(dir, "main.qgo"), `package main

func MainValue() float32 { return Able() + Zed() }
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"source-order", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	want := strings.Join([]string{
		"0\ta_first.qgo\tAble",
		"1\tmain.qgo\tMainValue",
		"2\tz_last.qgo\tZed",
		"",
	}, "\n")
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunSourceOrder_JSONFunctionsContract(t *testing.T) {
	dir := t.TempDir()
	writeMainTestFile(t, filepath.Join(dir, "go.mod"), "module qgosourceorderjsonfunctions\n\ngo 1.26\n")
	writeMainTestFile(t, filepath.Join(dir, "z_last.qgo"), `package main

func Zed() float32 { return 2 }
`)
	writeMainTestFile(t, filepath.Join(dir, "a_first.qgo"), `package main

func Able() float32 { return 1 }

func Alpha() float32 { return Able() }
`)
	writeMainTestFile(t, filepath.Join(dir, "main.qgo"), `package main

func MainValue() float32 { return Able() + Zed() }
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"source-order", "-format", "json", "-scope", "functions", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	want := strings.Join([]string{
		`[{"index":0,"file":"a_first.qgo","function":"Able"},{"index":1,"file":"a_first.qgo","function":"Alpha"},{"index":2,"file":"main.qgo","function":"MainValue"},{"index":3,"file":"z_last.qgo","function":"Zed"}]`,
		"",
	}, "\n")
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(stdout.String(), dir) {
		t.Fatalf("stdout = %q, want normalized relative paths", stdout.String())
	}
}

func TestRunSourceOrder_JSONFilesContract(t *testing.T) {
	dir := t.TempDir()
	writeMainTestFile(t, filepath.Join(dir, "go.mod"), "module qgosourceorderjsonfiles\n\ngo 1.26\n")
	writeMainTestFile(t, filepath.Join(dir, "z_last.qgo"), `package main

func Zed() float32 { return 2 }
`)
	writeMainTestFile(t, filepath.Join(dir, "a_first.qgo"), `package main

func Able() float32 { return 1 }

func Alpha() float32 { return Able() }
`)
	writeMainTestFile(t, filepath.Join(dir, "main.qgo"), `package main

func MainValue() float32 { return Able() + Zed() }
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"source-order", "-format", "json", "-scope", "files", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	want := strings.Join([]string{
		`[{"index":0,"file":"a_first.qgo"},{"index":1,"file":"main.qgo"},{"index":2,"file":"z_last.qgo"}]`,
		"",
	}, "\n")
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(stdout.String(), dir) {
		t.Fatalf("stdout = %q, want normalized relative paths", stdout.String())
	}
}

func TestRunSourceOrder_JSONUsesForwardSlashPathsOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows path normalization contract test")
	}
	dir := t.TempDir()
	writeMainTestFile(t, filepath.Join(dir, "go.mod"), "module qgosourceorderjsonpath\n\ngo 1.26\n")
	nested := filepath.Join(dir, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeMainTestFile(t, filepath.Join(nested, "inner.qgo"), `package main

func Inner() float32 { return 4 }
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"source-order", "-format", "json", "-scope", "functions", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	if strings.Contains(stdout.String(), `nested\\inner.qgo`) {
		t.Fatalf("stdout = %q, want slash-normalized relative path", stdout.String())
	}
	if !strings.Contains(stdout.String(), `nested/inner.qgo`) {
		t.Fatalf("stdout = %q, want slash-normalized relative path", stdout.String())
	}
}

func TestRunSourceOrder_UnsupportedFormatReturnsFailure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"source-order", "-format", "xml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, `qgo: unsupported -format "xml"`) {
		t.Fatalf("stderr = %q, want qgo-prefixed unsupported format error", got)
	}
}

func TestRunSourceOrder_WriteFailureReturnsFailure(t *testing.T) {
	dir := t.TempDir()
	writeMainTestFile(t, filepath.Join(dir, "go.mod"), "module qgosourceorderwrite\n\ngo 1.26\n")
	writeMainTestFile(t, filepath.Join(dir, "main.qgo"), `package main

func MainValue() float32 { return 1 }
`)

	outputPath := filepath.Join(dir, "out")
	if err := os.Mkdir(outputPath, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"source-order", "-o", outputPath, dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := stderr.String(); !strings.Contains(got, "qgo: write "+outputPath) {
		t.Fatalf("stderr = %q, want qgo-prefixed write error", got)
	}
}

func writeMainTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
