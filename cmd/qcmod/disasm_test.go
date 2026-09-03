package main

import (
	"strings"
	"testing"
)

func TestRunDisasmFiltersToNamedFunction(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runDisasm([]string{"-func", "multi_trigger"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDisasm exit=%d stderr=%q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "multi_trigger") {
		t.Errorf("output missing function name, got:\n%s", out)
	}
	if !strings.Contains(out, "statements") {
		t.Errorf("output missing statement range, got:\n%s", out)
	}
	// Filtered output must not include unrelated functions.
	if strings.Contains(out, "func 60") {
		t.Errorf("filtered output leaked other functions, got:\n%s", out)
	}
}

func TestRunDisasmUnknownFunctionFails(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runDisasm([]string{"-func", "definitely_not_a_qc_function_xyz"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown function, got stdout=%q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "not found") {
		t.Errorf("stderr missing not-found error, got %q", stderr.String())
	}
}

func TestRunDisasmMissingProgsPathFails(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runDisasm([]string{"/nonexistent/progs.dat"}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit for missing progs file")
	}
}
