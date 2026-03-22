package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func runBuildPrompt(args []string) int {
	fs := flag.NewFlagSet("build-prompt", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	summaryPath := fs.String("summary", "", "Path to Ralph summary JSON.")
	tasksPath := fs.String("tasks", "", "Path to Ralph task-records JSON.")
	logPath := fs.String("log", "", "Path to the telemetry log.")
	outputPath := fs.String("output", "", "Where to write the generated prompt.")
	maxTasks := fs.Int("max-tasks", 5, "Maximum number of task records to include.")
	verbose := fs.Bool("verbose", ralphVerbose, "Enable verbose Ralph logging.")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	ralphVerbose = *verbose

	required := map[string]string{
		"summary": *summaryPath,
		"tasks":   *tasksPath,
		"log":     *logPath,
		"output":  *outputPath,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			fmt.Fprintf(os.Stderr, "missing required --%s\n", name)
			return 2
		}
	}

	summary, err := loadSummary(*summaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	tasks, err := loadTasks(*tasksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	verbosef("building prompt from summary=%s tasks=%s log=%s", *summaryPath, *tasksPath, *logPath)
	for i, task := range limitTasks(tasks, *maxTasks) {
		verbosef("prompt task[%d] severity=%s title=%q", i, task.Severity, task.Title)
	}

	if err := ensureParentDirs(*outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if err := os.WriteFile(*outputPath, []byte(buildPromptText(summary, tasks, *logPath, *summaryPath, *tasksPath, *maxTasks)), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *outputPath, err)
		return 1
	}
	verbosef("prompt output=%s", *outputPath)
	fmt.Printf("Wrote Ralph Copilot prompt to %s\n", *outputPath)
	return 0
}

func buildPromptText(summary summaryFile, tasks []taskRecord, logPath, summaryPath, tasksPath string, maxTasks int) string {
	tasks = limitTasks(tasks, maxTasks)

	var lines []string
	lines = append(lines, "Work in the current repository and address the current Ralph telemetry findings.")
	lines = append(lines, "")
	lines = append(lines, "Constraints:")
	lines = append(lines, "- Fix the issues from the Ralph task records below.")
	lines = append(lines, "- Run relevant validation before finishing.")
	lines = append(lines, "- If an issue looks blocked or diagnosis is still weak, add or refine telemetry instead of guessing.")
	lines = append(lines, "- Do not start another continuous loop from inside this Copilot run.")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Ralph iteration: %d", summary.Iteration))
	lines = append(lines, fmt.Sprintf("Telemetry log: %s", logPath))
	lines = append(lines, fmt.Sprintf("Ralph summary JSON: %s", summaryPath))
	lines = append(lines, fmt.Sprintf("Ralph task JSON: %s", tasksPath))
	lines = append(lines, "")

	if len(summary.SeverityCounts) > 0 {
		var parts []string
		keys := make([]string, 0, len(summary.SeverityCounts))
		for key := range summary.SeverityCounts {
			keys = append(keys, key)
		}
		sortStrings(keys)
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%d", key, summary.SeverityCounts[key]))
		}
		lines = append(lines, "Severity counts: "+strings.Join(parts, ", "))
		lines = append(lines, "")
	}

	if len(tasks) == 0 {
		lines = append(lines, "No actionable Ralph task records were generated.")
	} else {
		lines = append(lines, "Current Ralph task records:")
		for i, task := range tasks {
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, fallback(task.Title, "Untitled task")))
			lines = append(lines, fmt.Sprintf("   Severity: %s", fallback(task.Severity, "unknown")))
			lines = append(lines, fmt.Sprintf("   Labels: %s", strings.Join(task.Labels, ", ")))
			count := task.Count
			if count == 0 {
				count = task.StallCount
			}
			lines = append(lines, fmt.Sprintf("   Count: %d", count))
			if task.Fingerprint != "" {
				lines = append(lines, fmt.Sprintf("   Fingerprint: %s", task.Fingerprint))
			}
			if task.Example != "" {
				lines = append(lines, fmt.Sprintf("   Example: %s", task.Example))
			}
			if strings.TrimSpace(task.Description) != "" {
				lines = append(lines, "   Description:")
				for _, line := range strings.Split(strings.TrimSpace(task.Description), "\n") {
					lines = append(lines, "     "+line)
				}
			}
			lines = append(lines, "")
		}
	}

	lines = append(lines, "After making changes, summarize what you changed and which validations you ran.")
	return strings.Join(lines, "\n") + "\n"
}

func limitTasks(tasks []taskRecord, maxTasks int) []taskRecord {
	if maxTasks >= 0 && len(tasks) > maxTasks {
		verbosef("truncating task list from %d to %d entries", len(tasks), maxTasks)
		return tasks[:maxTasks]
	}
	return tasks
}

func loadSummary(path string) (summaryFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return summaryFile{}, fmt.Errorf("read %s: %w", path, err)
	}
	var summary summaryFile
	if err := json.Unmarshal(data, &summary); err != nil {
		return summaryFile{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return summary, nil
}

func loadTasks(path string) ([]taskRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var tasks []taskRecord
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return tasks, nil
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
