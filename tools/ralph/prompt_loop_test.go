package main

import (
	"strings"
	"testing"
)

func TestBuildPromptTextLimitsTasks(t *testing.T) {
	summary := summaryFile{
		Iteration:      4,
		SeverityCounts: map[string]int{"error": 2, "warning": 1},
	}
	tasks := []taskRecord{
		{Title: "first task", Severity: "error", Labels: []string{"ralph"}, Count: 2},
		{Title: "second task", Severity: "warning", Labels: []string{"ralph"}, Count: 1},
	}

	prompt := buildPromptText(summary, tasks, "test.log", "summary.json", "tasks.json", 1)
	if !strings.Contains(prompt, "1. first task") {
		t.Fatalf("prompt missing first task:\n%s", prompt)
	}
	if strings.Contains(prompt, "2. second task") {
		t.Fatalf("prompt unexpectedly included truncated task:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Severity counts: error=2, warning=1") {
		t.Fatalf("prompt missing severity counts:\n%s", prompt)
	}
}

func TestBuildPromptTextHandlesEmptyTasks(t *testing.T) {
	prompt := buildPromptText(summaryFile{Iteration: 1}, nil, "test.log", "summary.json", "tasks.json", 5)
	if !strings.Contains(prompt, "No actionable Ralph task records were generated.") {
		t.Fatalf("prompt missing empty-task message:\n%s", prompt)
	}
}

func TestBuildCopilotArgsIncludesDefaultsAndExtras(t *testing.T) {
	got := buildCopilotArgs("fix this", "gpt-5.4", []string{"--json", "--debug"})
	want := []string{
		"-p", "fix this",
		"--model", "gpt-5.4",
		"--allow-all",
		"--no-ask-user",
		"--no-auto-update",
		"--json",
		"--debug",
	}
	if len(got) != len(want) {
		t.Fatalf("buildCopilotArgs() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("buildCopilotArgs()[%d] = %q, want %q (%v)", i, got[i], want[i], got)
		}
	}
}
