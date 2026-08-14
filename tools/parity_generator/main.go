package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go run ./tools/parity_generator <demo_name>\n\n")
		fmt.Fprintf(os.Stderr, "Environment Variables:\n")
		fmt.Fprintf(os.Stderr, "  QUAKE_DIR or QUAKE_BASEDIR  Path to Quake base directory containing id1/ (default: ./quake-data symlink)\n")
		fmt.Fprintf(os.Stderr, "  IRONWAIL_BIN               Path to C Ironwail binary (default: ./ironwail/Linux/ironwail via the ./ironwail symlink)\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	demoName := flag.Arg(0)

	// Resolve Quake directory: QUAKE_BASEDIR or QUAKE_DIR env, then
	// ./quake-data symlink (AGENTS.md convention), then error.
	quakeDir := os.Getenv("QUAKE_BASEDIR")
	if quakeDir == "" {
		quakeDir = os.Getenv("QUAKE_DIR")
	}
	if quakeDir == "" {
		if st, err := os.Stat("quake-data"); err == nil && st.IsDir() {
			quakeDir = "quake-data"
		}
	}
	if quakeDir == "" {
		fmt.Fprintf(os.Stderr, "Error: set QUAKE_DIR (or create the ./quake-data symlink) to locate the Quake data directory.\n")
		os.Exit(1)
	}
	quakeDir = filepath.Clean(quakeDir)

	// Validate Quake directory
	if _, err := os.Stat(filepath.Join(quakeDir, "id1")); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Quake data directory 'id1' not found at %s. Please set QUAKE_DIR environment variable.\n", quakeDir)
		os.Exit(1)
	}

	// Resolve C Ironwail binary: IRONWAIL_BIN env, then the ./ironwail
	// symlink's Linux build directory, then error.
	ironwailBin := os.Getenv("IRONWAIL_BIN")
	if ironwailBin == "" {
		if st, err := os.Stat("ironwail/Linux/ironwail"); err == nil && !st.IsDir() {
			ironwailBin = "ironwail/Linux/ironwail"
		} else if st, err := os.Stat("ironwail"); err == nil && !st.IsDir() {
			ironwailBin = "ironwail"
		}
	}
	ironwailBin = filepath.Clean(ironwailBin)

	// Validate C Ironwail binary
	if _, err := os.Stat(ironwailBin); err != nil {
		fmt.Fprintf(os.Stderr, "Error: C Ironwail binary not found at %s. Please build the C binary or set IRONWAIL_BIN environment variable.\n", ironwailBin)
		os.Exit(1)
	}

	// Resolve Go project root directory
	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
		os.Exit(1)
	}

	outputDir := filepath.Join(projectDir, "testdata", "parity")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory %s: %v\n", outputDir, err)
		os.Exit(1)
	}

	outputPath := filepath.Join(outputDir, fmt.Sprintf("reference_%s_state.json", demoName))

	// Remove any pre-existing dump file
	_ = os.Remove(outputPath)

	fmt.Printf("Generating frame-state reference for demo '%s'...\n", demoName)
	fmt.Printf("  Quake Dir: %s\n", quakeDir)
	fmt.Printf("  C Binary:  %s\n", ironwailBin)
	fmt.Printf("  Output:    %s\n\n", outputPath)

	// Build the command arguments to run timedemo and output frame state
	args := []string{
		"-basedir", quakeDir,
		"-window",
		"-width", "1237",
		"-height", "1428",
		"-dumpstate", outputPath,
		"+timedemo", demoName,
	}

	cmd := exec.Command(ironwailBin, args...)

	// Combine stdout and stderr into a single pipe
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	// Set environment to run under X11 in headless/windowed environments safely
	cmd.Env = append(os.Environ(),
		"WAYLAND_DISPLAY=",
		"XDG_SESSION_TYPE=x11",
		"SDL_VIDEODRIVER=x11",
	)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start C Ironwail process: %v\n", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	demoFinished := false
	var finishedMu sync.Mutex

	// Goroutine to scan stdout for completion markers
	go func() {
		defer wg.Done()
		defer func() { _ = pw.Close() }()

		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println(line) // Stream output to console

			// Look for the timedemo finish report, e.g. "969 frames 1.8 seconds 530.1 fps"
			if strings.Contains(line, "frames") && strings.Contains(line, "seconds") && strings.Contains(line, "fps") {
				finishedMu.Lock()
				demoFinished = true
				finishedMu.Unlock()
			}
		}
	}()

	// Monitor loop to watch file size and timedemo finish flag
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastSize int64 = 0
	lastSizeChange := time.Now()

	for range ticker.C {
		finishedMu.Lock()
		done := demoFinished
		finishedMu.Unlock()

		if done {
			fmt.Println("\nTimedemo report detected. Waiting briefly before terminating process...")
			time.Sleep(500 * time.Millisecond)
			_ = cmd.Process.Kill()
			goto ProcessTerminated
		}

		// Watchdog: check if state file has stopped growing
		if info, err := os.Stat(outputPath); err == nil {
			size := info.Size()
			if size > 0 {
				if size != lastSize {
					lastSize = size
					lastSizeChange = time.Now()
				} else if time.Since(lastSizeChange) > 1500*time.Millisecond {
					fmt.Println("\nWatchdog: State dump file size has not changed for 1.5 seconds. Terminating process...")
					_ = cmd.Process.Kill()
					goto ProcessTerminated
				}
			}
		}
	}

ProcessTerminated:
	// Close the pipe writer to unblock the scanner reader
	_ = pw.Close()
	_ = cmd.Wait()
	wg.Wait()

	// Verify output file exists and is populated
	info, err := os.Stat(outputPath)
	if err != nil || info.Size() == 0 {
		fmt.Fprintf(os.Stderr, "\nError: Reference state dump file was not generated or is empty.\n")
		os.Exit(1)
	}

	// Count number of frames dumped (lines in JSON)
	frameCount := 0
	if f, err := os.Open(outputPath); err == nil {
		defer func() { _ = f.Close() }()
		s := bufio.NewScanner(f)
		for s.Scan() {
			frameCount++
		}
	}

	fmt.Printf("\nSuccess! Generated reference state file with %d frames (%d bytes).\n", frameCount, info.Size())
}
