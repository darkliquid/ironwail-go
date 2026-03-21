package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunAnalyzeLogFailsWhenLogMissing(t *testing.T) {
	dir := t.TempDir()
	status := runAnalyzeLog([]string{
		"--log", filepath.Join(dir, "missing.log"),
		"--summary", filepath.Join(dir, "summary.json"),
		"--tasks", filepath.Join(dir, "tasks.json"),
		"--state", filepath.Join(dir, "state.json"),
	})
	if status != 1 {
		t.Fatalf("runAnalyzeLog() status = %d, want 1", status)
	}
}

func TestRunBuildPromptFailsOnInvalidSummaryJSON(t *testing.T) {
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.json")
	tasksPath := filepath.Join(dir, "tasks.json")
	if err := os.WriteFile(summaryPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.WriteFile(tasksPath, []byte("[]\n"), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}

	status := runBuildPrompt([]string{
		"--summary", summaryPath,
		"--tasks", tasksPath,
		"--log", filepath.Join(dir, "telemetry.log"),
		"--output", filepath.Join(dir, "prompt.txt"),
	})
	if status != 1 {
		t.Fatalf("runBuildPrompt() status = %d, want 1", status)
	}
}

func TestRunBuildPromptFailsOnInvalidTasksJSON(t *testing.T) {
	dir := t.TempDir()
	summaryPath := filepath.Join(dir, "summary.json")
	tasksPath := filepath.Join(dir, "tasks.json")
	if err := writeJSON(summaryPath, summaryFile{Iteration: 7}); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.WriteFile(tasksPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write tasks: %v", err)
	}

	status := runBuildPrompt([]string{
		"--summary", summaryPath,
		"--tasks", tasksPath,
		"--log", filepath.Join(dir, "telemetry.log"),
		"--output", filepath.Join(dir, "prompt.txt"),
	})
	if status != 1 {
		t.Fatalf("runBuildPrompt() status = %d, want 1", status)
	}
}

func TestSubcommandHelpReturnsZero(t *testing.T) {
	tests := []struct {
		name string
		run  func([]string) int
	}{
		{name: "analyze-log", run: runAnalyzeLog},
		{name: "build-prompt", run: runBuildPrompt},
		{name: "loop", run: runLoop},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if status := tt.run([]string{"--help"}); status != 0 {
				t.Fatalf("%s help status = %d, want 0", tt.name, status)
			}
		})
	}
}

func TestCopyFileMissingSourceReturnsError(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "missing.log"), filepath.Join(dir, "latest.log"))
	if err == nil {
		t.Fatal("copyFile() error = nil, want non-nil")
	}
}

func TestClassifyLineIgnoresCountSummary(t *testing.T) {
	if severity, ok := classifyLine("error_count=2 failure_count=1 warning_count=3"); ok {
		t.Fatalf("classifyLine() = (%q, true), want no match", severity)
	}
}

func TestClassifyLineStillMatchesActionableErrors(t *testing.T) {
	severity, ok := classifyLine("failed to load savegame")
	if !ok || severity != "error" {
		t.Fatalf("classifyLine() = (%q, %t), want (error, true)", severity, ok)
	}
}
