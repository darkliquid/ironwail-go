package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

var (
	ignorePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\[svdbg\b`),
		regexp.MustCompile(`(?i)^(INFO|DEBUG)\b`),
		regexp.MustCompile(`(?i)^\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}\s+INFO\b`),
	}
	matchers = []struct {
		severity string
		pattern  *regexp.Regexp
	}{
		{"error", regexp.MustCompile(`(?i)\bpanic:|fatal error:|segmentation fault|assert|unexpected .*fail|crash|stack trace\b`)},
		{"error", regexp.MustCompile(`(?i)\berror\b|failed\b|failure\b`)},
		{"warning", regexp.MustCompile(`(?i)\bwarn(?:ing)?\b`)},
		{"issue", regexp.MustCompile(`(?i)\bTODO\b|\bFIXME\b|\bparity\b`)},
	}
	hexRE       = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	floatRE     = regexp.MustCompile(`\b\d+\.\d+\b`)
	intRE       = regexp.MustCompile(`\b\d+\b`)
	quotedRE    = regexp.MustCompile(`"[^"]+"`)
	parensRE    = regexp.MustCompile(`\([^)]*\)`)
	wsRE        = regexp.MustCompile(`\s+`)
	timePrefix  = regexp.MustCompile(`^\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}\s+`)
	bracketPref = regexp.MustCompile(`^\[[^\]]+\]\s*`)
)

type issue struct {
	Fingerprint string `json:"fingerprint"`
	Severity    string `json:"severity"`
	Normalized  string `json:"-"`
	Example     string `json:"example,omitempty"`
	Count       int    `json:"count,omitempty"`
}

type taskRecord struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Severity    string   `json:"severity"`
	Labels      []string `json:"labels"`
	Fingerprint string   `json:"fingerprint"`
	Count       int      `json:"count,omitempty"`
	SeenCount   int      `json:"seen_count,omitempty"`
	StallCount  int      `json:"stall_count,omitempty"`
	Description string   `json:"description"`
	Example     string   `json:"example,omitempty"`
}

type issueState struct {
	FirstSeenIteration int  `json:"first_seen_iteration"`
	LastSeenIteration  int  `json:"last_seen_iteration"`
	SeenCount          int  `json:"seen_count"`
	Active             bool `json:"active"`
}

type stateFile struct {
	Issues map[string]*issueState `json:"issues"`
}

type summaryFile struct {
	Iteration       int            `json:"iteration"`
	Log             string         `json:"log"`
	LineCount       int            `json:"line_count"`
	SeverityCounts  map[string]int `json:"severity_counts"`
	IssueCount      int            `json:"issue_count"`
	TaskRecordCount int            `json:"task_record_count"`
	StallThreshold  int            `json:"stall_threshold"`
}

type beadsSyncRecord struct {
	ID     string `json:"id"`
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
}

func runAnalyzeLog(args []string) int {
	fs := flag.NewFlagSet("analyze-log", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	logPath := fs.String("log", "", "Path to the telemetry log to analyze.")
	summaryPath := fs.String("summary", "", "Path to write the Ralph summary JSON.")
	tasksPath := fs.String("tasks", "", "Path to write actionable task records JSON.")
	statePath := fs.String("state", "", "Path to Ralph state JSON.")
	beadsSyncPath := fs.String("beads-sync", "", "Path to write Ralph beads sync results JSON.")
	beadsBinary := fs.String("beads-binary", "bd", "Beads CLI binary to use for direct sync.")
	applyBeads := fs.Bool("apply-beads", false, "Create/update Ralph tasks directly in beads.")
	verbose := fs.Bool("verbose", ralphVerbose, "Enable verbose Ralph logging.")
	iteration := fs.Int("iteration", 1, "Current Ralph iteration number.")
	stallThreshold := fs.Int("stall-threshold", 3, "Iterations before emitting telemetry-design tasks.")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	ralphVerbose = *verbose

	required := map[string]string{
		"log":     *logPath,
		"summary": *summaryPath,
		"tasks":   *tasksPath,
		"state":   *statePath,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			fmt.Fprintf(os.Stderr, "missing required --%s\n", name)
			return 2
		}
	}

	paths := []string{*summaryPath, *tasksPath, *statePath}
	if strings.TrimSpace(*beadsSyncPath) != "" {
		paths = append(paths, *beadsSyncPath)
	}
	if err := ensureParentDirs(paths...); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	lines := readLines(*logPath)
	issues, severityCounts := summarizeLog(lines)
	verbosef("analyzing log=%s lines=%d iteration=%d", *logPath, len(lines), *iteration)
	verbosef("detected issue_groups=%d severity_counts=%v", len(issues), severityCounts)

	state := loadState(*statePath)
	if state.Issues == nil {
		state.Issues = map[string]*issueState{}
	}

	var tasks []taskRecord
	for _, issue := range issues {
		is := state.Issues[issue.Fingerprint]
		if is == nil {
			is = &issueState{
				FirstSeenIteration: *iteration,
				LastSeenIteration:  *iteration,
			}
			state.Issues[issue.Fingerprint] = is
		}
		is.LastSeenIteration = *iteration
		is.SeenCount++
		is.Active = true

		title := fmt.Sprintf("Ralph: %s %s", issue.Severity, truncate(issue.Normalized, 72))
		labels := []string{"ralph", "telemetry", issue.Severity}
		description := fmt.Sprintf(
			"Generated from Ralph telemetry iteration %d.\n\nFingerprint: %s\nCount in log: %d\nExample line: %s\nLog: %s\n",
			*iteration, issue.Fingerprint, issue.Count, issue.Example, *logPath,
		)
		tasks = append(tasks, taskRecord{
			ID:          safeID(issue.Fingerprint, "ralph"),
			Title:       title,
			Severity:    issue.Severity,
			Labels:      labels,
			Fingerprint: issue.Fingerprint,
			Count:       issue.Count,
			SeenCount:   is.SeenCount,
			Description: description,
			Example:     issue.Example,
		})
		verbosef("task %s severity=%s count=%d title=%q", safeID(issue.Fingerprint, "ralph"), issue.Severity, issue.Count, title)

		if is.SeenCount >= *stallThreshold {
			telemetryTitle := fmt.Sprintf("Ralph: add telemetry for %s", truncate(issue.Normalized, 60))
			telemetryDescription := fmt.Sprintf(
				"Ralph has seen this issue for %d iterations.\n\nPersistent fingerprint: %s\nRepresentative line: %s\nDesign narrower telemetry around the failing path before retrying.",
				is.SeenCount, issue.Fingerprint, issue.Example,
			)
			telemetryLabels := []string{"ralph", "telemetry", "diagnostics"}
			tasks = append(tasks, taskRecord{
				ID:          safeID(issue.Fingerprint, "ralph-telemetry"),
				Title:       telemetryTitle,
				Severity:    "issue",
				Labels:      telemetryLabels,
				Fingerprint: issue.Fingerprint,
				StallCount:  is.SeenCount,
				Description: telemetryDescription,
			})
			verbosef("task %s severity=%s stall_count=%d title=%q", safeID(issue.Fingerprint, "ralph-telemetry"), "issue", is.SeenCount, telemetryTitle)
		}
	}

	live := map[string]struct{}{}
	for _, issue := range issues {
		live[issue.Fingerprint] = struct{}{}
	}
	for fp, issueState := range state.Issues {
		_, ok := live[fp]
		issueState.Active = ok
	}

	summary := summaryFile{
		Iteration:       *iteration,
		Log:             *logPath,
		LineCount:       len(lines),
		SeverityCounts:  severityCounts,
		IssueCount:      len(issues),
		TaskRecordCount: len(tasks),
		StallThreshold:  *stallThreshold,
	}

	if err := writeJSON(*summaryPath, summary); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if err := writeJSON(*tasksPath, tasks); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	if err := writeJSON(*statePath, state); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	var (
		syncRecords []beadsSyncRecord
		err         error
	)
	if *applyBeads {
		syncRecords, err = syncBeads(tasks, *beadsBinary)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	}
	if strings.TrimSpace(*beadsSyncPath) != "" {
		if err := writeJSON(*beadsSyncPath, syncRecords); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	}
	if ralphVerbose && len(syncRecords) > 0 {
		for _, record := range syncRecords {
			if record.Error != "" {
				verbosef("beads %s action=%s error=%s", record.ID, record.Action, record.Error)
				continue
			}
			verbosef("beads %s action=%s", record.ID, record.Action)
		}
	}

	fmt.Printf("Ralph summary: %d actionable issue groups, %d task records, severities=%v\n", len(issues), len(tasks), severityCounts)
	fmt.Printf("Summary: %s\n", *summaryPath)
	fmt.Printf("Task records: %s\n", *tasksPath)
	if strings.TrimSpace(*beadsSyncPath) != "" {
		fmt.Printf("Beads sync report: %s\n", *beadsSyncPath)
	}
	if *applyBeads {
		fmt.Printf("Beads sync: applied %d task records via %s\n", len(syncRecords), *beadsBinary)
	}
	if len(issues) == 0 {
		fmt.Println("No actionable warnings/errors/issues found.")
		return 0
	}
	return 10
}

func summarizeLog(lines []string) ([]issue, map[string]int) {
	issues := map[string]*issue{}
	severityCounts := map[string]int{}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r\n")
		severity, ok := classifyLine(line)
		if !ok {
			continue
		}
		normalized := normalizeLine(line)
		fingerprint := strings.ToLower(normalized)
		found := issues[fingerprint]
		if found == nil {
			found = &issue{
				Fingerprint: fingerprint,
				Severity:    severity,
				Normalized:  normalized,
				Example:     strings.TrimSpace(line),
			}
			issues[fingerprint] = found
		}
		if severityRank(severity) > severityRank(found.Severity) {
			found.Severity = severity
		}
		found.Count++
		severityCounts[severity]++
	}

	ordered := make([]issue, 0, len(issues))
	for _, issue := range issues {
		ordered = append(ordered, *issue)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if severityRank(ordered[i].Severity) != severityRank(ordered[j].Severity) {
			return severityRank(ordered[i].Severity) > severityRank(ordered[j].Severity)
		}
		if ordered[i].Count != ordered[j].Count {
			return ordered[i].Count > ordered[j].Count
		}
		return ordered[i].Normalized < ordered[j].Normalized
	})
	return ordered, severityCounts
}

func classifyLine(line string) (string, bool) {
	for _, pattern := range ignorePatterns {
		if pattern.MatchString(line) {
			return "", false
		}
	}
	for _, matcher := range matchers {
		if matcher.pattern.MatchString(line) {
			return matcher.severity, true
		}
	}
	return "", false
}

func normalizeLine(line string) string {
	line = stripTimestampPrefix(line)
	line = hexRE.ReplaceAllString(line, "<hex>")
	line = floatRE.ReplaceAllString(line, "<float>")
	line = intRE.ReplaceAllString(line, "<n>")
	line = quotedRE.ReplaceAllString(line, `"<quoted>"`)
	line = parensRE.ReplaceAllString(line, "(...)")
	line = wsRE.ReplaceAllString(strings.TrimSpace(line), " ")
	return line
}

func stripTimestampPrefix(line string) string {
	line = timePrefix.ReplaceAllString(line, "")
	line = bracketPref.ReplaceAllString(line, "")
	return strings.TrimSpace(line)
}

func severityRank(severity string) int {
	switch severity {
	case "error":
		return 3
	case "warning":
		return 2
	case "issue":
		return 1
	default:
		return 0
	}
}

func safeID(text, prefix string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(text) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	body := strings.Trim(b.String(), "-")
	if body == "" {
		body = "issue"
	}
	if len(body) > 48 {
		body = strings.TrimRight(body[:48], "-")
	}
	return prefix + "-" + body
}

func loadState(path string) stateFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return stateFile{Issues: map[string]*issueState{}}
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return stateFile{Issues: map[string]*issueState{}}
	}
	return state
}

func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func syncBeads(tasks []taskRecord, beadsBinary string) ([]beadsSyncRecord, error) {
	if _, err := exec.LookPath(beadsBinary); err != nil {
		return nil, fmt.Errorf("beads CLI %q not found: %w", beadsBinary, err)
	}

	records := make([]beadsSyncRecord, 0, len(tasks))
	for _, task := range tasks {
		action, err := syncBeadsTask(beadsBinary, task)
		record := beadsSyncRecord{
			ID:     task.ID,
			Action: action,
		}
		if err != nil {
			record.Error = err.Error()
			records = append(records, record)
			return records, fmt.Errorf("beads sync failed for %s: %w", task.ID, err)
		}
		records = append(records, record)
	}
	return records, nil
}

func syncBeadsTask(beadsBinary string, task taskRecord) (string, error) {
	priority := "2"
	externalRef := "ralph:" + task.Fingerprint
	labelCSV := strings.Join(task.Labels, ",")
	if beadsIssueExists(beadsBinary, task.ID) {
		verbosef("updating beads task id=%s title=%q labels=%s", task.ID, task.Title, labelCSV)
		args := []string{
			"update",
			task.ID,
			"--title", task.Title,
			"--description", task.Description,
			"--priority", priority,
			"--type", "task",
			"--set-labels", labelCSV,
			"--external-ref", externalRef,
		}
		if err := runQuiet(beadsBinary, args...); err != nil {
			return "update_failed", err
		}
		return "updated", nil
	}

	verbosef("creating beads task id=%s title=%q labels=%s", task.ID, task.Title, labelCSV)
	args := []string{
		"create",
		task.Title,
		"--id", task.ID,
		"--description", task.Description,
		"--priority", priority,
		"--issue-type", "task",
		"--labels", labelCSV,
		"--external-ref", externalRef,
	}
	if err := runQuiet(beadsBinary, args...); err != nil {
		return "create_failed", err
	}
	return "created", nil
}

func beadsIssueExists(beadsBinary, issueID string) bool {
	cmd := exec.Command(beadsBinary, "show", "--json", "--id", issueID)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func runQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), message)
}
