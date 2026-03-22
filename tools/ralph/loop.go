package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func runLoop(args []string) int {
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	verbose := fs.Bool("verbose", ralphVerbose, "Enable verbose Ralph logging.")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	ralphVerbose = *verbose

	mode := "continuous"
	if fs.NArg() > 0 {
		mode = fs.Arg(0)
	}
	if fs.NArg() > 1 || (mode != "once" && mode != "continuous") {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/ralph loop [once|continuous]")
		return 2
	}

	quakeDir := os.Getenv("QUAKE_DIR")
	if quakeDir == "" {
		fmt.Fprintln(os.Stderr, "QUAKE_DIR is required")
		return 1
	}

	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		return 1
	}

	outDir := envOr("RALPH_OUT_DIR", filepath.Join(projectDir, ".ralph"))
	runDir := filepath.Join(outDir, "runs")
	statePath := envOr("RALPH_STATE", filepath.Join(outDir, "state.json"))
	summaryPath := envOr("RALPH_SUMMARY", filepath.Join(outDir, "latest-summary.json"))
	tasksPath := envOr("RALPH_TASKS", filepath.Join(outDir, "latest-task-records.json"))
	beadsSyncPath := envOr("RALPH_BEADS_SYNC", filepath.Join(outDir, "latest-beads-sync.json"))
	promptPath := envOr("RALPH_PROMPT_PATH", filepath.Join(outDir, "latest-copilot-prompt.txt"))
	latestLog := envOr("RALPH_LOG", filepath.Join(outDir, "latest.log"))
	engineBin := envOr("RALPH_ENGINE_BIN", filepath.Join(projectDir, "ironwailgo-cgo"))

	if _, err := os.Stat(engineBin); err != nil {
		fmt.Fprintf(os.Stderr, "missing executable engine binary: %s\n", engineBin)
		return 1
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", runDir, err)
		return 1
	}

	timeoutSeconds := parseIntEnv("RALPH_TIMEOUT", 30)
	stallThreshold := parseIntEnv("RALPH_STALL_THRESHOLD", 3)
	maxIterations := parseIntEnv("RALPH_MAX_ITERATIONS", 0)
	sleepSeconds := parseIntEnv("RALPH_SLEEP", 5)
	invokeCopilot := parseBoolEnv("RALPH_INVOKE_COPILOT", true)
	applyBeads := parseBoolEnv("RALPH_APPLY_BEADS", false)
	beadsBinary := envOr("RALPH_BEADS_BIN", "bd")
	copilotBin := envOr("RALPH_COPILOT_BIN", "copilot")
	copilotModel := envOr("RALPH_COPILOT_MODEL", "gpt-5.4")
	copilotMaxTasks := parseIntEnv("RALPH_COPILOT_MAX_TASKS", 5)
	copilotExtraArgs := strings.Fields(os.Getenv("RALPH_COPILOT_ARGS"))
	engineExtraArgs := strings.Fields(os.Getenv("RALPH_ENGINE_ARGS"))
	verbosef("loop mode=%s timeout=%d stall_threshold=%d max_iterations=%d apply_beads=%t invoke_copilot=%t",
		mode, timeoutSeconds, stallThreshold, maxIterations, applyBeads, invokeCopilot)

	defaultArgs := []string{
		"+sv_debug_telemetry", "1",
		"+sv_debug_telemetry_summary", "1",
		"+sv_debug_qc_trace", "1",
		"+sv_debug_qc_trace_verbosity", "1",
		"+cl_debug_view", "2",
	}

	for iteration := 1; ; iteration++ {
		timestamp := time.Now().UTC().Format("20060102T150405Z")
		runLog := filepath.Join(runDir, fmt.Sprintf("ralph-%s.log", timestamp))

		fmt.Printf("== Ralph iteration %d ==\n", iteration)
		fmt.Printf("log: %s\n", runLog)
		verbosef("engine command=%s -basedir %s %s", engineBin, quakeDir, strings.Join(append(defaultArgs, engineExtraArgs...), " "))

		engineStatus := runEngine(engineBin, quakeDir, runLog, timeoutSeconds, append(defaultArgs, engineExtraArgs...)...)
		if err := copyFile(runLog, latestLog); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		fmt.Printf("engine exit: %d\n", engineStatus)
		engineFailed := engineStatus != 0 && engineStatus != 124

		analysisArgs := []string{
			"--log", runLog,
			"--summary", summaryPath,
			"--tasks", tasksPath,
			"--state", statePath,
			"--beads-sync", beadsSyncPath,
			"--iteration", fmt.Sprintf("%d", iteration),
			"--stall-threshold", fmt.Sprintf("%d", stallThreshold),
		}
		if applyBeads {
			analysisArgs = append(analysisArgs, "--apply-beads", "--beads-binary", beadsBinary)
		}
		if ralphVerbose {
			analysisArgs = append(analysisArgs, "--verbose")
		}
		verbosef("running analyze-log summary=%s tasks=%s state=%s", summaryPath, tasksPath, statePath)
		analysisStatus := runAnalyzeLog(analysisArgs)
		if analysisStatus != 0 && analysisStatus != 10 {
			return analysisStatus
		}
		if analysisStatus == 0 {
			if engineFailed {
				fmt.Fprintf(os.Stderr, "Ralph loop: engine exited unexpectedly with status %d and produced no actionable findings\n", engineStatus)
				return engineStatus
			}
			fmt.Println("Ralph loop: no actionable issues detected.")
			return 0
		}

		promptFile := filepath.Join(runDir, fmt.Sprintf("ralph-%s.prompt.txt", timestamp))
		copilotLog := filepath.Join(runDir, fmt.Sprintf("ralph-%s.copilot.log", timestamp))
		promptArgs := []string{
			"--summary", summaryPath,
			"--tasks", tasksPath,
			"--log", runLog,
			"--output", promptFile,
			"--max-tasks", fmt.Sprintf("%d", copilotMaxTasks),
		}
		if ralphVerbose {
			promptArgs = append(promptArgs, "--verbose")
			verbosef("running build-prompt output=%s max_tasks=%d", promptFile, copilotMaxTasks)
		}
		promptStatus := runBuildPrompt(promptArgs)
		if promptStatus != 0 {
			fmt.Fprintln(os.Stderr, "failed to build Ralph Copilot prompt")
			return promptStatus
		}
		if err := copyFile(promptFile, promptPath); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}

		if invokeCopilot {
			if _, err := exec.LookPath(copilotBin); err != nil {
				fmt.Fprintf(os.Stderr, "Ralph loop: Copilot CLI %q not found\n", copilotBin)
				return 1
			}
			promptBytes, err := os.ReadFile(promptFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "read %s: %v\n", promptFile, err)
				return 1
			}
			copilotArgs := buildCopilotArgs(string(promptBytes), copilotModel, copilotExtraArgs)
			fmt.Printf("Ralph loop: invoking Copilot with prompt %s\n", promptFile)
			verbosef("copilot command=%s %s", copilotBin, strings.Join(copilotArgs[2:], " "))
			status := runToFile(projectDir, copilotLog, copilotBin, copilotArgs...)
			fmt.Printf("copilot exit: %d\n", status)
			fmt.Printf("copilot log: %s\n", copilotLog)
		}

		if mode == "once" {
			return 0
		}
		if maxIterations != 0 && iteration >= maxIterations {
			fmt.Printf("Ralph loop: reached RALPH_MAX_ITERATIONS=%d\n", maxIterations)
			return 0
		}

		fmt.Printf("Ralph loop: actionable issues remain; artifacts in %s\n", outDir)
		verbosef("sleeping for %d seconds before next iteration", sleepSeconds)
		time.Sleep(time.Duration(sleepSeconds) * time.Second)
	}
}

func buildCopilotArgs(prompt, model string, extraArgs []string) []string {
	args := []string{
		"-p", prompt,
		"--model", model,
		"--allow-all",
		"--no-ask-user",
		"--no-auto-update",
	}
	return append(args, extraArgs...)
}

func runEngine(engineBin, quakeDir, logPath string, timeoutSeconds int, args ...string) int {
	engineArgs := append([]string{"-basedir", quakeDir}, args...)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, engineBin, engineArgs...)
	cmd.Env = append(os.Environ(), "WAYLAND_DISPLAY=")
	return runCmdToFile(cmd, logPath)
}

func runToFile(dir, logPath, name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return runCmdToFile(cmd, logPath)
}

func runCmdToFile(cmd *exec.Cmd, logPath string) int {
	if err := ensureParentDirs(logPath); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	f, err := os.Create(logPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", logPath, err)
		return 1
	}
	defer f.Close()

	cmd.Stdout = f
	cmd.Stderr = f
	if ralphVerbose {
		cmd.Stdout = io.MultiWriter(os.Stdout, f)
		cmd.Stderr = io.MultiWriter(os.Stderr, f)
	}
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitCode := exitErr.ExitCode(); exitCode >= 0 {
				return exitCode
			}
			return 124
		}
		fmt.Fprintf(os.Stderr, "%s failed: %v\n", cmd.Path, err)
		return 1
	}
	return 0
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	if err := ensureParentDirs(dst); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}
	return nil
}
