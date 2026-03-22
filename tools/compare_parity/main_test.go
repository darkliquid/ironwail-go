package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCollectGoFunctionsFromSource(t *testing.T) {
	src := `package sample

func Plain() {}
func (r *Receiver) MethodName(v int) {}
`
	got := collectGoFunctionsFromSource(src)
	want := []string{"plain", "methodname"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("collectGoFunctionsFromSource() = %v, want %v", got, want)
	}
}

func TestCollectCFunctionsFromSource(t *testing.T) {
	src := `
void SV_Move(void)
{
}

static int CL_ParseUpdate (void)
{
}

if (condition)
`
	got := collectCFunctionsFromSource(src)
	want := []string{"SV_Move", "CL_ParseUpdate"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("collectCFunctionsFromSource() = %v, want %v", got, want)
	}
}

func TestNormalizeSearchName(t *testing.T) {
	tests := map[string]string{
		"SV_MoveToGoal": "movetogoal",
		"CL_Parse_TEnt": "parsetent",
		"Host_Frame":    "frame",
		"Custom_Name":   "customname",
	}
	for input, want := range tests {
		if got := normalizeSearchName(input); got != want {
			t.Fatalf("normalizeSearchName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildReportIncludesMappedAndMissingFunctions(t *testing.T) {
	cDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cDir, "sv_phys.c"), []byte("void SV_MoveToGoal(void)\nvoid SV_MissingThing(void)\n"), 0o644); err != nil {
		t.Fatalf("write test C file: %v", err)
	}
	for _, name := range cFiles[1:] {
		if err := os.WriteFile(filepath.Join(cDir, name), []byte{}, 0o644); err != nil {
			t.Fatalf("write filler C file %s: %v", name, err)
		}
	}

	report, err := buildReport(cDir, map[string]struct{}{
		"movetogoal": {},
	})
	if err != nil {
		t.Fatalf("buildReport() error = %v", err)
	}
	if !strings.Contains(report, "Found 1 mapped functions. Missing: 1.") {
		t.Fatalf("report missing summary: %s", report)
	}
	if !strings.Contains(report, "`SV_MissingThing` is missing or heavily refactored.") {
		t.Fatalf("report missing unmapped function entry: %s", report)
	}
}
