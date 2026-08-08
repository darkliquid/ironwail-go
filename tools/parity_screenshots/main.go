// Command parity_screenshots drives the Ironwail-Go and reference Ironwail
// binaries across a set of viewpoints, captures screenshots, and compares
// them pixel-by-pixel to report rendering parity metrics.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type viewpointsFile struct {
	BaseDir    string      `json:"basedir"`
	Viewpoints []viewpoint `json:"viewpoints"`
}

type viewpoint struct {
	ID          string     `json:"id"`
	Game        string     `json:"game,omitempty"`
	Map         string     `json:"map"`
	Pos         [3]float64 `json:"pos"`
	Angles      [3]float64 `json:"angles"`
	Tag         string     `json:"tag,omitempty"`
	Description string     `json:"description"`
}

type comparisonMetrics struct {
	Width                       int
	Height                      int
	MismatchPixels              int
	TotalPixels                 int
	MismatchPercent             float64
	MeanChannelDelta            float64
	MaxChannelDelta             uint8
	MeanRedDelta                float64
	MeanGreenDelta              float64
	MeanBlueDelta               float64
	MeanAlphaDelta              float64
	MeanPerceptualDelta         float64
	MeanMismatchPerceptualDelta float64
	MaxPerceptualDelta          float64
	// SSIM (structural similarity) on luminance, scaled to [0,1] where 1 is
	// identical. MeanSSIM is the frame average; MinSSIM the worst 8x8 block.
	MeanSSIM float64
	MinSSIM  float64
	Regions  []diffRegion
}

type diffRegion struct {
	MinX   int
	MinY   int
	MaxX   int
	MaxY   int
	Pixels int
}

type captureSummary struct {
	Count    int
	Failures int
}

type compareSummary struct {
	ReferenceCount int
	GoCount        int
	MatchCount     int
	DiffCount      int
	MissingCount   int
}

// viewpointResult is the per-viewpoint comparison outcome, used by the
// structured JSON report and the markdown table (plan 14.3).
type viewpointResult struct {
	ID              string            `json:"id"`
	Map             string            `json:"map"`
	Description     string            `json:"description"`
	Status          string            `json:"status"` // ok | diff | missing | skip
	MismatchPercent float64           `json:"mismatch_percent,omitempty"`
	MeanSSIM        float64           `json:"mean_ssim,omitempty"`
	MinSSIM         float64           `json:"min_ssim,omitempty"`
	MeanChannel     float64           `json:"mean_channel_delta,omitempty"`
	MaxChannel      uint8             `json:"max_channel_delta,omitempty"`
	Perceptual      float64           `json:"mean_perceptual_delta,omitempty"`
	Regions         int               `json:"diff_regions,omitempty"`
	Metrics         comparisonMetrics `json:"-"`
}

func main() {
	os.Exit(run())
}

func run() int {
	mode := "help"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	if mode != "reference" && mode != "ref" && mode != "go" && mode != "compare" && mode != "cmp" && mode != "both" && mode != "all" && mode != "report" {
		printUsage()
		return 2
	}

	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		return 1
	}

	viewpointsPath := filepath.Join(projectDir, "testdata", "parity", "viewpoints.json")
	cfg := loadViewpoints(viewpointsPath)
	if tag := os.Getenv("PARITY_TAG"); tag != "" {
		var filtered []viewpoint
		for _, vp := range cfg.Viewpoints {
			if vp.Tag == tag {
				filtered = append(filtered, vp)
			}
		}
		cfg.Viewpoints = filtered
		fmt.Printf("PARITY_TAG=%s: %d viewpoints", tag, len(filtered))
	}
	quakeBaseDir := envOr("QUAKE_BASEDIR", "")
	if quakeBaseDir == "" {
		quakeBaseDir = envOr("QUAKE_DIR", cfg.BaseDir)
	}
	if quakeBaseDir == "" {
		if st, err := os.Stat(filepath.Join(projectDir, "quake-data")); err == nil && st.IsDir() {
			quakeBaseDir = filepath.Join(projectDir, "quake-data")
		}
	}
	ironwailBin := envOr("IRONWAIL_BIN", "")
	if ironwailBin == "" {
		if st, err := os.Stat(filepath.Join(projectDir, "ironwail", "Linux", "ironwail")); err == nil && !st.IsDir() {
			ironwailBin = filepath.Join(projectDir, "ironwail", "Linux", "ironwail")
		} else if st, err := os.Stat(filepath.Join(projectDir, "ironwail")); err == nil && !st.IsDir() {
			ironwailBin = filepath.Join(projectDir, "ironwail")
		} else {
			ironwailBin = filepath.Join(quakeBaseDir, "ironwail")
		}
	}
	goBin := envOr("GO_BIN", filepath.Join(projectDir, "ironwailgo"))
	parityWidth := parseIntEnv("PARITY_WIDTH", 1280)
	parityHeight := parseIntEnv("PARITY_HEIGHT", 720)
	refDir := filepath.Join(projectDir, "testdata", "parity", "reference")
	goDir := filepath.Join(projectDir, "testdata", "parity", "go")
	diffDir := filepath.Join(projectDir, "testdata", "parity", "diff")

	switch mode {
	case "reference", "ref":
		if quakeBaseDir == "" {
			return missingQuakeBaseDir()
		}
		checkQuakeData(quakeBaseDir)
		if _, err := captureReference(quakeBaseDir, ironwailBin, refDir, cfg.Viewpoints, parityWidth, parityHeight); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v", err)
			return 1
		}
	case "go":
		if quakeBaseDir == "" {
			return missingQuakeBaseDir()
		}
		checkQuakeData(quakeBaseDir)
		if _, err := captureGo(projectDir, quakeBaseDir, goBin, goDir, cfg.Viewpoints, parityWidth, parityHeight); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
	case "compare", "cmp":
		summary, _, err := compare(refDir, goDir, diffDir, cfg.Viewpoints)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		if compareFailed(summary) {
			return 1
		}
	case "both", "all":
		if quakeBaseDir == "" {
			return missingQuakeBaseDir()
		}
		checkQuakeData(quakeBaseDir)
		if _, err := captureReference(quakeBaseDir, ironwailBin, refDir, cfg.Viewpoints, parityWidth, parityHeight); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		fmt.Println()
		if _, err := captureGo(projectDir, quakeBaseDir, goBin, goDir, cfg.Viewpoints, parityWidth, parityHeight); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		fmt.Println()
		summary, _, err := compare(refDir, goDir, diffDir, cfg.Viewpoints)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		if compareFailed(summary) {
			return 1
		}
	case "report":
		summary, results, err := compare(refDir, goDir, diffDir, cfg.Viewpoints)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v", err)
			return 1
		}
		writeParityReport(projectDir, results, summary)
	}
	return 0
}

func missingQuakeBaseDir() int {
	fmt.Fprintln(os.Stderr, "ERROR: QUAKE_BASEDIR is not set and testdata/parity/viewpoints.json has no base_dir")
	fmt.Fprintln(os.Stderr, "Set QUAKE_BASEDIR=/path/to/quake before running reference, go, or both captures.")
	return 1
}

func captureReference(quakeBaseDir, ironwailBin, refDir string, viewpoints []viewpoint, width, height int) (captureSummary, error) {
	if _, err := os.Stat(ironwailBin); err != nil {
		return captureSummary{}, fmt.Errorf("c ironwail binary not found: %s", ironwailBin)
	}
	mustMkdir(refDir)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return captureSummary{}, fmt.Errorf("user home dir: %w", err)
	}
	refUserDir := filepath.Join(homeDir, ".ironwail")

	fmt.Println("=== Capturing reference screenshots from C Ironwail ===")
	fmt.Println("Binary:", ironwailBin)
	fmt.Println("Output:", refDir)
	fmt.Println()

	summary := captureSummary{Count: len(viewpoints)}
	for _, vp := range viewpoints {
		fmt.Printf("  [REF] %s: %s\n", vp.ID, vp.Description)
		activeGameDir := "id1"
		if vp.Game != "" {
			activeGameDir = vp.Game
		}
		vpScreenshotDir := filepath.Join(refUserDir, activeGameDir, "screenshots")
		mustMkdir(vpScreenshotDir)

		clearScreenshotMatches(vpScreenshotDir, vp.ID)
		cfgFile := genReferenceCfg(filepath.Join(refUserDir, activeGameDir), vp, width, height)
		args := []string{
			"-basedir", quakeBaseDir,
			"-window", "-width", fmt.Sprintf("%d", width), "-height", fmt.Sprintf("%d", height),
		}
		if vp.Game != "" {
			args = append(args, "-game", vp.Game)
		}
		args = append(args,
			"+map", vp.Map,
			"+exec", filepath.Base(cfgFile),
		)
		fmt.Printf("    exec: %s %s\n", ironwailBin, strings.Join(args, " "))
		if status := runWithTimeout(30*time.Second, captureEnv(true), ironwailBin, args...); status != 0 {
			fmt.Printf("    WARNING: C Ironwail exited with error for %s\n", vp.ID)
			summary.Failures++
		}
		_ = os.Remove(cfgFile)
		if err := moveFirstMatch(vpScreenshotDir, vp.ID, filepath.Join(refDir, vp.ID+".png")); err != nil {
			summary.Failures++
			return summary, fmt.Errorf("capture reference %s: %w", vp.ID, err)
		}
		if actualWidth, actualHeight, err := imageSize(filepath.Join(refDir, vp.ID+".png")); err != nil {
			summary.Failures++
			return summary, fmt.Errorf("inspect reference %s: %w", vp.ID, err)
		} else if actualWidth != width || actualHeight != height {
			fmt.Printf("    reference image landed at %dx%d instead of requested %dx%d\n", actualWidth, actualHeight, width, height)
		}
	}

	fmt.Println()
	fmt.Printf("Reference screenshots saved to: %s\n", refDir)
	if summary.Failures > 0 {
		return summary, fmt.Errorf("reference capture completed with %d failure(s)", summary.Failures)
	}
	return summary, nil
}

func captureGo(projectDir, quakeBaseDir, goBin, goDir string, viewpoints []viewpoint, width, height int) (captureSummary, error) {
	if shouldBuildGoBinary(projectDir, goBin) {
		fmt.Println("Building Go binary...")
		mustMkdir(filepath.Dir(goBin))
		if status := runInDir(projectDir, []string{"CGO_ENABLED=0"}, "go", "build", "-tags", "audio_oto", "-o", goBin, "./cmd/ironwailgo"); status != 0 {
			return captureSummary{}, errors.New("failed to build Go binary")
		}
	} else if _, err := os.Stat(goBin); err != nil {
		return captureSummary{}, fmt.Errorf("go binary not found: %s", goBin)
	}
	mustMkdir(goDir)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return captureSummary{}, fmt.Errorf("user home dir: %w", err)
	}
	refUserDir := filepath.Join(homeDir, ".ironwail")

	fmt.Println("=== Capturing Go port screenshots ===")
	fmt.Println("Binary:", goBin)
	fmt.Println("Output:", goDir)
	fmt.Println("Capture method:", goCaptureMethod())
	fmt.Println()

	summary := captureSummary{Count: len(viewpoints)}
	for _, vp := range viewpoints {
		fmt.Printf("  [GO] %s: %s\n", vp.ID, vp.Description)
		_ = os.Remove(filepath.Join(goDir, vp.ID+".png"))
		captureWidth, captureHeight, usedReferenceSize, err := resolveGoCaptureSize(filepath.Join(refDirForGo(goDir), vp.ID+".png"), width, height)
		if err != nil {
			summary.Failures++
			return summary, fmt.Errorf("resolve go size %s: %w", vp.ID, err)
		}
		if usedReferenceSize {
			fmt.Printf("    matching reference image size %dx%d\n", captureWidth, captureHeight)
		}
		outputPath := filepath.Join(goDir, vp.ID+".png")
		activeGameDir := "id1"
		if vp.Game != "" {
			activeGameDir = vp.Game
		}
		cfgFile := genGoCfg(filepath.Join(refUserDir, activeGameDir), vp)
		args := goCaptureArgs(quakeBaseDir, captureWidth, captureHeight, vp, outputPath, filepath.Base(cfgFile))
		fmt.Printf("    exec: %s %s\n", goBin, strings.Join(args, " "))
		status := 0
		goEnv := goCaptureEnv(vp)
		if goCaptureMethod() == "window" {
			status = runGoWindowCapture(30*time.Second, goBin, args, outputPath, goEnv)
		} else {
			status = runWithTimeout(30*time.Second, goEnv, goBin, args...)
		}
		_ = os.Remove(cfgFile)
		if status != 0 {
			fmt.Printf("    WARNING: Go binary exited with error for %s\n", vp.ID)
			summary.Failures++
		}
		if _, err := os.Stat(outputPath); err != nil {
			summary.Failures++
			return summary, fmt.Errorf("capture go %s: missing screenshot %s", vp.ID, outputPath)
		}
		if changed, fromWidth, fromHeight, err := normalizeImageSize(outputPath, captureWidth, captureHeight); err != nil {
			summary.Failures++
			return summary, fmt.Errorf("normalize go %s: %w", vp.ID, err)
		} else if changed {
			fmt.Printf("    normalized Go image from %dx%d to %dx%d\n", fromWidth, fromHeight, captureWidth, captureHeight)
		}
	}

	fmt.Println()
	fmt.Printf("Go screenshots saved to: %s\n", goDir)
	if summary.Failures > 0 {
		return summary, fmt.Errorf("go capture completed with %d failure(s)", summary.Failures)
	}
	return summary, nil
}

func compare(refDir, goDir, diffDir string, viewpoints []viewpoint) (compareSummary, []viewpointResult, error) {
	mustMkdir(diffDir)
	overlayDir := filepath.Join(filepath.Dir(diffDir), "overlay")
	mustMkdir(overlayDir)
	fmt.Println("=== Comparing screenshots ===")
	fmt.Println()

	tolerance := clampUint8(parseIntEnv("PARITY_COMPARE_TOLERANCE", 0))
	maxMismatchPercent := parseFloatEnv("PARITY_MAX_MISMATCH_PERCENT", 0)
	maxSSIMDiff := clampFloat(parseFloatEnv("PARITY_MAX_SSIM_DIFF", 0.02), 0, 1)
	onionAlpha := clampFloat(parseFloatEnv("PARITY_ONION_ALPHA", 0.5), 0, 1)

	var summary compareSummary
	var results []viewpointResult
	for _, vp := range viewpoints {
		refImg := filepath.Join(refDir, vp.ID+".png")
		goImg := filepath.Join(goDir, vp.ID+".png")

		if _, err := os.Stat(refImg); err != nil {
			fmt.Printf("  SKIP %s: no reference image\n", vp.ID)
			summary.MissingCount++
			results = append(results, viewpointResult{ID: vp.ID, Map: vp.Map, Description: vp.Description, Status: "skip"})
			continue
		}
		summary.ReferenceCount++

		if _, err := os.Stat(goImg); err != nil {
			fmt.Printf("  MISS %s: no Go screenshot\n", vp.ID)
			summary.MissingCount++
			results = append(results, viewpointResult{ID: vp.ID, Map: vp.Map, Description: vp.Description, Status: "missing"})
			continue
		}
		summary.GoCount++

		metrics, diffImage, overlayImage, err := compareImageFiles(refImg, goImg, tolerance, onionAlpha)
		if err != nil {
			fmt.Printf("  DIFF %s: compare failed: %v (%s)\n", vp.ID, err, vp.Description)
			summary.DiffCount++
			results = append(results, viewpointResult{ID: vp.ID, Map: vp.Map, Description: vp.Description, Status: "diff"})
			continue
		}
		diffPath := filepath.Join(diffDir, vp.ID+".png")
		overlayPath := filepath.Join(overlayDir, vp.ID+".png")
		if err := writePNG(overlayPath, overlayImage); err != nil {
			fmt.Printf("  DIFF %s: failed to write onion overlay: %v (%s)\n", vp.ID, err, vp.Description)
			summary.DiffCount++
			continue
		}
		if metrics.MismatchPixels == 0 {
			_ = os.Remove(diffPath)
		} else if err := writePNG(diffPath, diffImage); err != nil {
			fmt.Printf("  DIFF %s: failed to write diff image: %v (%s)\n", vp.ID, err, vp.Description)
			summary.DiffCount++
			continue
		}

		ssimOK := 1.0-metrics.MeanSSIM <= maxSSIMDiff
		if metrics.MismatchPercent <= maxMismatchPercent && ssimOK {
			fmt.Printf("  OK   %s: %.4f%% pixels differ, SSIM %.4f, mean Δ %.2f, perceptual Δ %.2f, max Δ %d (%s)\n",
				vp.ID, metrics.MismatchPercent, metrics.MeanSSIM, metrics.MeanChannelDelta, metrics.MeanPerceptualDelta, metrics.MaxChannelDelta, vp.Description)
			for _, line := range formatMetricDetails(metrics) {
				fmt.Printf("       %s\n", line)
			}
			fmt.Printf("       onion=%s (alpha %.2f ref / %.2f go)\n", overlayPath, onionAlpha, 1-onionAlpha)
			summary.MatchCount++
			results = append(results, resultForViewpoint(vp, "ok", metrics))
		} else {
			fmt.Printf("  DIFF %s: %.4f%% pixels differ, SSIM %.4f, mean Δ %.2f, perceptual Δ %.2f, max Δ %d, diff=%s (%s)\n",
				vp.ID, metrics.MismatchPercent, metrics.MeanSSIM, metrics.MeanChannelDelta, metrics.MeanPerceptualDelta, metrics.MaxChannelDelta, diffPath, vp.Description)
			for _, line := range formatMetricDetails(metrics) {
				fmt.Printf("       %s\n", line)
			}
			fmt.Printf("       onion=%s (alpha %.2f ref / %.2f go)\n", overlayPath, onionAlpha, 1-onionAlpha)
			summary.DiffCount++
			results = append(results, resultForViewpoint(vp, "diff", metrics))
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Reference images: %d\n", summary.ReferenceCount)
	fmt.Printf("Go images:        %d\n", summary.GoCount)
	fmt.Printf("Matches:          %d\n", summary.MatchCount)
	fmt.Printf("Diffs:            %d\n", summary.DiffCount)
	fmt.Printf("Missing:          %d\n", summary.MissingCount)
	fmt.Printf("Diff images:      %s\n", diffDir)
	fmt.Printf("Onion overlays:   %s\n", overlayDir)
	fmt.Println()
	fmt.Printf("Tolerance:        channel Δ <= %d, mismatch threshold <= %.4f%%, SSIM diff <= %.4f, onion alpha %.2f ref / %.2f go\n", tolerance, maxMismatchPercent, maxSSIMDiff, onionAlpha, 1-onionAlpha)
	if summary.ReferenceCount == 0 {
		return summary, results, errors.New("no reference images found")
	}
	return summary, results, nil
}

func loadViewpoints(path string) viewpointsFile {
	data, err := os.ReadFile(path)
	if err != nil {
		die("viewpoints file not found: %s", path)
	}
	var cfg viewpointsFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		die("parse viewpoints file: %v", err)
	}
	return cfg
}

func checkQuakeData(quakeBaseDir string) {
	if _, err := os.Stat(filepath.Join(quakeBaseDir, "id1")); err != nil {
		die("Quake data not found at %s", filepath.Join(quakeBaseDir, "id1"))
	}
}

func genReferenceCfg(dir string, vp viewpoint, width, height int) string {
	mustMkdir(dir)
	f, err := os.CreateTemp(dir, "parity_*.cfg")
	if err != nil {
		die("create temp cfg: %v", err)
	}
	defer func() { _ = f.Close() }()
	preToggleWaits := waitLines(45)
	postToggleWaits := waitLines(150)
	preShotWaits := waitLines(4)
	content := fmt.Sprintf(`// Auto-generated parity screenshot config
vid_fullscreen 0
vid_width %d
vid_height %d
scr_viewsize 130
r_drawviewmodel 0
crosshair 0
fov 90
gamma 1
con_speed 999999
scr_conspeed 999999
cl_screenshotname screenshots/%s
%s
hideconsole
%s
host_framerate 0.0001
setpos %s %s %s %s %s %s
%s
screenshot png
wait
quit
`, width, height, vp.ID, preToggleWaits, postToggleWaits, fmtFloat(vp.Pos[0]), fmtFloat(vp.Pos[1]), fmtFloat(vp.Pos[2]), fmtFloat(vp.Angles[0]), fmtFloat(vp.Angles[1]), fmtFloat(vp.Angles[2]), preShotWaits)
	if _, err := io.WriteString(f, content); err != nil {
		die("write cfg: %v", err)
	}
	return f.Name()
}

func genGoCfg(dir string, vp viewpoint) string {
	mustMkdir(dir)
	f, err := os.CreateTemp(dir, "parity_go_*.cfg")
	if err != nil {
		die("create Go temp cfg: %v", err)
	}
	defer func() { _ = f.Close() }()
	// Keep Go capture setup close to the reference flow so map spawn settles,
	// menu state is dismissed by startup logic, and setpos runs deterministically
	// before window capture.
	content := fmt.Sprintf(`// Auto-generated parity Go screenshot config
scr_viewsize 130
r_drawviewmodel 0
crosshair 0
host_framerate 0.0001
noclip
setpos %s %s %s %s %s %s
`, fmtFloat(vp.Pos[0]), fmtFloat(vp.Pos[1]), fmtFloat(vp.Pos[2]), fmtFloat(vp.Angles[0]), fmtFloat(vp.Angles[1]), fmtFloat(vp.Angles[2]))
	if _, err := io.WriteString(f, content); err != nil {
		die("write Go cfg: %v", err)
	}
	return f.Name()
}

func moveFirstMatch(dir, id, dst string) error {
	patterns := []string{
		filepath.Join(dir, id+"*.png"),
		filepath.Join(dir, id+"*.tga"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			if err := normalizeCaptureToPNG(match, dst); err != nil {
				return fmt.Errorf("normalize %s -> %s: %w", match, dst, err)
			}
			_ = os.Remove(match)
			return nil
		}
	}
	return fmt.Errorf("no screenshot matching %q found in %s", id, dir)
}

func clearScreenshotMatches(dir, id string) {
	patterns := []string{
		filepath.Join(dir, id+"*.png"),
		filepath.Join(dir, id+"*.tga"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			_ = os.Remove(match)
		}
	}
}

func runWithTimeout(timeout time.Duration, env []string, name string, args ...string) int {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitCode := exitErr.ExitCode(); exitCode >= 0 {
				return exitCode
			}
			return 124
		}
		return 1
	}
	return 0
}

func runInDir(dir string, env []string, name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}
