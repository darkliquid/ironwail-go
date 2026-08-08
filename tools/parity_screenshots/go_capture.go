package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

func goCaptureMethod() string {
	method := strings.ToLower(strings.TrimSpace(os.Getenv("PARITY_GO_CAPTURE")))
	switch method {
	case "", "engine":
		// Engine readback (renderer CaptureScreenshot) is the parity-correct
		// default: it captures the scene texture directly, matching C
		// Ironwail's internal framebuffer screenshot. X11 window capture
		// (xdotool + ImageMagick import) goes through the compositor, which
		// applies its own gamma/color management and dims the output — that
		// only belongs in an explicit opt-in for diagnosing window-specific
		// issues.
		return "engine"
	case "window":
		return "window"
	default:
		return "engine"
	}
}

func goCaptureArgs(quakeBaseDir string, width, height int, vp viewpoint, outputPath string, cfgFile string) []string {
	args := []string{
		"-basedir", quakeBaseDir,
		"-window",
		"-width", fmt.Sprintf("%d", width),
		"-height", fmt.Sprintf("%d", height),
	}
	if vp.Game != "" {
		args = append(args, "-game", vp.Game)
	}
	if goCaptureMethod() == "engine" {
		args = append(args, "-screenshot", outputPath)
	}
	args = append(args,
		"+map", vp.Map,
		"+exec", cfgFile,
	)
	return args
}

func runGoWindowCapture(timeout time.Duration, goBin string, args []string, outputPath string, env []string) int {
	if _, err := exec.LookPath("xdotool"); err != nil {
		fmt.Printf("    ERROR: PARITY_GO_CAPTURE=window requires xdotool: %v\n", err)
		return 1
	}
	if _, err := exec.LookPath("import"); err != nil {
		fmt.Printf("    ERROR: PARITY_GO_CAPTURE=window requires ImageMagick import: %v\n", err)
		return 1
	}
	if strings.TrimSpace(os.Getenv("DISPLAY")) == "" {
		fmt.Printf("    ERROR: PARITY_GO_CAPTURE=window requires DISPLAY\n")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, goBin, args...)
	cmd.Env = append(os.Environ(), env...)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("    ERROR: attach Go stdout pipe: %v\n", err)
		return 1
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("    ERROR: attach Go stderr pipe: %v\n", err)
		return 1
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("    ERROR: start Go capture: %v\n", err)
		return 1
	}
	const readyMarker = "PARITY_READY"
	markerSeen := make(chan struct{}, 1)
	go watchForReadyMarker(stdoutPipe, readyMarker, markerSeen)
	go watchForReadyMarker(stderrPipe, readyMarker, markerSeen)

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			<-done
		}
	}()

	windowID, err := waitForWindowID(ctx, cmd.Process.Pid, parseCaptureArgInt(args, "-width"), parseCaptureArgInt(args, "-height"))
	if err != nil {
		fmt.Printf("    ERROR: find Go window: %v\n", err)
		return 1
	}
	targetWidth := parseCaptureArgInt(args, "-width")
	targetHeight := parseCaptureArgInt(args, "-height")
	settle := time.Duration(parseIntEnv("PARITY_GO_WINDOW_SETTLE_MS", 9000)) * time.Millisecond
	if settle > 0 {
		ready := false
		select {
		case <-markerSeen:
			ready = true
		case <-time.After(settle):
			fmt.Printf("    NOTE: Go capture marker %q not observed before timeout; capturing anyway\n", readyMarker)
		case <-ctx.Done():
			return 124
		case err := <-done:
			if err != nil {
				return processExitCode(err)
			}
			return 0
		}
		if ready {
			postReadyDelay := time.Duration(parseIntEnv("PARITY_GO_POST_READY_MS", 2000)) * time.Millisecond
			if postReadyDelay > 0 {
				select {
				case <-time.After(postReadyDelay):
				case <-ctx.Done():
					return 124
				case err := <-done:
					if err != nil {
						return processExitCode(err)
					}
					return 0
				}
			}
		}
	}
	if targetWidth > 0 && targetHeight > 0 {
		if err := waitForWindowGeometry(ctx, windowID, targetWidth, targetHeight, 5*time.Second); err != nil {
			fmt.Printf("    NOTE: Go window did not settle to requested geometry before capture: %v\n", err)
		}
	}

	importCmd := exec.CommandContext(ctx, "import", "-window", windowID, outputPath)
	importCmd.Stdout = io.Discard
	importCmd.Stderr = io.Discard
	if err := importCmd.Run(); err != nil {
		fmt.Printf("    ERROR: import Go window %s: %v\n", windowID, err)
		return 1
	}

	_ = cmd.Process.Kill()
	<-done
	if ctx.Err() == context.DeadlineExceeded {
		return 124
	}
	return 0
}

func waitForWindowID(parent context.Context, pid int, targetWidth, targetHeight int) (string, error) {
	searchTimeout := 10 * time.Second
	if deadline, ok := parent.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < searchTimeout {
			searchTimeout = remaining
		}
	}
	if searchTimeout <= 0 {
		return "", context.DeadlineExceeded
	}
	ctx, cancel := context.WithTimeout(parent, searchTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "xdotool", "search", "--sync", "--onlyvisible", "--pid", fmt.Sprint(pid), "--name", "Ironwail-Go").Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", errors.New("xdotool returned no windows")
	}
	return bestWindowID(ctx, fields, targetWidth, targetHeight), nil
}

func bestWindowID(ctx context.Context, ids []string, targetWidth, targetHeight int) string {
	if len(ids) == 0 {
		return ""
	}
	if targetWidth <= 0 || targetHeight <= 0 {
		return ids[0]
	}
	bestID := ids[0]
	bestScore := int(^uint(0) >> 1)
	for _, id := range ids {
		width, height, ok := windowGeometry(ctx, id)
		if !ok {
			continue
		}
		score := absInt(width-targetWidth) + absInt(height-targetHeight)
		if score < bestScore {
			bestID = id
			bestScore = score
		}
	}
	return bestID
}

func windowGeometry(ctx context.Context, id string) (int, int, bool) {
	out, err := exec.CommandContext(ctx, "xdotool", "getwindowgeometry", "--shell", id).Output()
	if err != nil {
		return 0, 0, false
	}
	var width, height int
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "WIDTH":
			_, _ = fmt.Sscanf(value, "%d", &width)
		case "HEIGHT":
			_, _ = fmt.Sscanf(value, "%d", &height)
		}
	}
	return width, height, width > 0 && height > 0
}

func parseCaptureArgInt(args []string, key string) int {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == key {
			var value int
			_, _ = fmt.Sscanf(args[i+1], "%d", &value)
			return value
		}
	}
	return 0
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func processExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitCode := exitErr.ExitCode(); exitCode >= 0 {
			return exitCode
		}
	}
	return 1
}

func waitForWindowGeometry(ctx context.Context, windowID string, targetWidth, targetHeight int, timeout time.Duration) error {
	waitCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		waitCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		width, height, ok := windowGeometry(waitCtx, windowID)
		if ok && width == targetWidth && height == targetHeight {
			return nil
		}
		select {
		case <-waitCtx.Done():
			if width == 0 && height == 0 {
				return waitCtx.Err()
			}
			return fmt.Errorf("final geometry %dx%d, expected %dx%d", width, height, targetWidth, targetHeight)
		case <-ticker.C:
		}
	}
}

func watchForReadyMarker(r io.Reader, marker string, seen chan<- struct{}) {
	scanner := bufio.NewScanner(r)
	const maxLine = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLine)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), marker) {
			select {
			case seen <- struct{}{}:
			default:
			}
			return
		}
	}
}
