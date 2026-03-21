package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	qimage "github.com/ironwail/ironwail-go/internal/image"
)

type viewpointsFile struct {
	BaseDir    string      `json:"basedir"`
	Viewpoints []viewpoint `json:"viewpoints"`
}

type viewpoint struct {
	ID          string     `json:"id"`
	Map         string     `json:"map"`
	Pos         [3]float64 `json:"pos"`
	Angles      [3]float64 `json:"angles"`
	Description string     `json:"description"`
}

type comparisonMetrics struct {
	Width            int
	Height           int
	MismatchPixels   int
	TotalPixels      int
	MismatchPercent  float64
	MeanChannelDelta float64
	MaxChannelDelta  uint8
}

func main() {
	mode := "help"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		os.Exit(1)
	}

	viewpointsPath := filepath.Join(projectDir, "testdata", "parity", "viewpoints.json")
	cfg := loadViewpoints(viewpointsPath)
	quakeBaseDir := envOr("QUAKE_BASEDIR", cfg.BaseDir)
	if quakeBaseDir == "" {
		quakeBaseDir = "/home/darkliquid/Games/Heroic/Quake"
	}
	ironwailBin := envOr("IRONWAIL_BIN", filepath.Join(quakeBaseDir, "ironwail"))
	goBin := envOr("GO_BIN", filepath.Join(projectDir, "ironwailgo-wgpu"))
	refDir := filepath.Join(projectDir, "testdata", "parity", "reference")
	goDir := filepath.Join(projectDir, "testdata", "parity", "go")
	diffDir := filepath.Join(projectDir, "testdata", "parity", "diff")

	checkDeps(viewpointsPath, quakeBaseDir)

	switch mode {
	case "reference", "ref":
		captureReference(quakeBaseDir, ironwailBin, refDir, cfg.Viewpoints)
	case "go":
		captureGo(projectDir, quakeBaseDir, goBin, goDir, cfg.Viewpoints)
	case "compare", "cmp":
		compare(refDir, goDir, diffDir, cfg.Viewpoints)
	case "both", "all":
		captureReference(quakeBaseDir, ironwailBin, refDir, cfg.Viewpoints)
		fmt.Println()
		captureGo(projectDir, quakeBaseDir, goBin, goDir, cfg.Viewpoints)
		fmt.Println()
		compare(refDir, goDir, diffDir, cfg.Viewpoints)
	default:
		printUsage()
	}
}

func captureReference(quakeBaseDir, ironwailBin, refDir string, viewpoints []viewpoint) {
	if _, err := os.Stat(ironwailBin); err != nil {
		die("C Ironwail binary not found: %s", ironwailBin)
	}
	mustMkdir(refDir)
	screenshotDir := filepath.Join(quakeBaseDir, "id1", "screenshots")
	mustMkdir(screenshotDir)

	fmt.Println("=== Capturing reference screenshots from C Ironwail ===")
	fmt.Println("Binary:", ironwailBin)
	fmt.Println("Output:", refDir)
	fmt.Println()

	for _, vp := range viewpoints {
		fmt.Printf("  [REF] %s: %s\n", vp.ID, vp.Description)
		clearScreenshotMatches(screenshotDir, vp.ID)
		cfgFile := genReferenceCfg(filepath.Join(quakeBaseDir, "id1"), vp)
		args := []string{
			"-basedir", quakeBaseDir,
			"-window", "-width", "1280", "-height", "720",
			"+map", vp.Map,
			"+exec", filepath.Base(cfgFile),
		}
		if status := runWithTimeout(30*time.Second, ironwailBin, args...); status != 0 {
			fmt.Printf("    WARNING: C Ironwail exited with error for %s\n", vp.ID)
		}
		_ = os.Remove(cfgFile)
		if err := moveFirstMatch(screenshotDir, vp.ID, filepath.Join(refDir, vp.ID+".png")); err != nil {
			die("capture reference %s: %v", vp.ID, err)
		}
	}

	fmt.Println()
	fmt.Printf("Reference screenshots saved to: %s\n", refDir)
}

func captureGo(projectDir, quakeBaseDir, goBin, goDir string, viewpoints []viewpoint) {
	if _, err := os.Stat(goBin); err != nil {
		fmt.Println("Building Go binary...")
		if status := runInDir(projectDir, "go", "build", "-tags=gogpu", "-o", "ironwailgo-wgpu", "./cmd/ironwailgo"); status != 0 {
			die("failed to build Go binary")
		}
	}
	mustMkdir(goDir)

	fmt.Println("=== Capturing Go port screenshots ===")
	fmt.Println("Binary:", goBin)
	fmt.Println("Output:", goDir)
	fmt.Println()

	for _, vp := range viewpoints {
		fmt.Printf("  [GO] %s: %s\n", vp.ID, vp.Description)
		args := []string{
			"-basedir", quakeBaseDir,
			"-screenshot", filepath.Join(goDir, vp.ID+".png"),
			"+map", vp.Map,
			"+noclip",
			"+setpos",
			fmtFloat(vp.Pos[0]), fmtFloat(vp.Pos[1]), fmtFloat(vp.Pos[2]),
			fmtFloat(vp.Angles[0]), fmtFloat(vp.Angles[1]), fmtFloat(vp.Angles[2]),
		}
		if status := runWithTimeout(30*time.Second, goBin, args...); status != 0 {
			fmt.Printf("    WARNING: Go binary exited with error for %s\n", vp.ID)
		}
	}

	fmt.Println()
	fmt.Printf("Go screenshots saved to: %s\n", goDir)
}

func compare(refDir, goDir, diffDir string, viewpoints []viewpoint) {
	mustMkdir(diffDir)
	fmt.Println("=== Comparing screenshots ===")
	fmt.Println()

	tolerance := clampUint8(parseIntEnv("PARITY_COMPARE_TOLERANCE", 0))
	maxMismatchPercent := parseFloatEnv("PARITY_MAX_MISMATCH_PERCENT", 0)

	var refCount, goCount, matchCount, diffCount, missingCount int
	for _, vp := range viewpoints {
		refImg := filepath.Join(refDir, vp.ID+".png")
		goImg := filepath.Join(goDir, vp.ID+".png")

		if _, err := os.Stat(refImg); err != nil {
			fmt.Printf("  SKIP %s: no reference image\n", vp.ID)
			continue
		}
		refCount++

		if _, err := os.Stat(goImg); err != nil {
			fmt.Printf("  MISS %s: no Go screenshot\n", vp.ID)
			missingCount++
			continue
		}
		goCount++

		metrics, diffImage, err := compareImageFiles(refImg, goImg, tolerance)
		if err != nil {
			fmt.Printf("  DIFF %s: compare failed: %v (%s)\n", vp.ID, err, vp.Description)
			diffCount++
			continue
		}
		diffPath := filepath.Join(diffDir, vp.ID+".png")
		if metrics.MismatchPixels == 0 {
			_ = os.Remove(diffPath)
		} else if err := writePNG(diffPath, diffImage); err != nil {
			fmt.Printf("  DIFF %s: failed to write diff image: %v (%s)\n", vp.ID, err, vp.Description)
			diffCount++
			continue
		}

		if metrics.MismatchPercent <= maxMismatchPercent {
			fmt.Printf("  OK   %s: %.4f%% pixels differ, mean Δ %.2f, max Δ %d (%s)\n",
				vp.ID, metrics.MismatchPercent, metrics.MeanChannelDelta, metrics.MaxChannelDelta, vp.Description)
			matchCount++
		} else {
			fmt.Printf("  DIFF %s: %.4f%% pixels differ, mean Δ %.2f, max Δ %d, diff=%s (%s)\n",
				vp.ID, metrics.MismatchPercent, metrics.MeanChannelDelta, metrics.MaxChannelDelta, diffPath, vp.Description)
			diffCount++
		}
	}

	fmt.Println()
	fmt.Println("=== Summary ===")
	fmt.Printf("Reference images: %d\n", refCount)
	fmt.Printf("Go images:        %d\n", goCount)
	fmt.Printf("Matches:          %d\n", matchCount)
	fmt.Printf("Diffs:            %d\n", diffCount)
	fmt.Printf("Missing:          %d\n", missingCount)
	fmt.Printf("Diff images:      %s\n", diffDir)
	fmt.Println()
	fmt.Printf("Tolerance:        channel Δ <= %d, mismatch threshold <= %.4f%%\n", tolerance, maxMismatchPercent)
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

func checkDeps(viewpointsPath, quakeBaseDir string) {
	if _, err := os.Stat(viewpointsPath); err != nil {
		die("viewpoints file not found: %s", viewpointsPath)
	}
	if _, err := os.Stat(filepath.Join(quakeBaseDir, "id1")); err != nil {
		die("Quake data not found at %s", filepath.Join(quakeBaseDir, "id1"))
	}
}

func genReferenceCfg(dir string, vp viewpoint) string {
	mustMkdir(dir)
	f, err := os.CreateTemp(dir, "parity_*.cfg")
	if err != nil {
		die("create temp cfg: %v", err)
	}
	defer f.Close()
	preToggleWaits := waitLines(45)
	postToggleWaits := waitLines(12)
	preShotWaits := waitLines(4)
	content := fmt.Sprintf(`// Auto-generated parity screenshot config
scr_viewsize 100
r_drawviewmodel 1
fov 90
gamma 1
scr_conspeed 999999
cl_screenshotname screenshots/%s
%s
toggleconsole
%s
host_framerate 0.0001
setpos %s %s %s %s %s %s
%s
screenshot png
wait
quit
`, vp.ID, preToggleWaits, postToggleWaits, fmtFloat(vp.Pos[0]), fmtFloat(vp.Pos[1]), fmtFloat(vp.Pos[2]), fmtFloat(vp.Angles[0]), fmtFloat(vp.Angles[1]), fmtFloat(vp.Angles[2]), preShotWaits)
	if _, err := io.WriteString(f, content); err != nil {
		die("write cfg: %v", err)
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

func waitLines(count int) string {
	if count <= 0 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < count; i++ {
		b.WriteString("wait\n")
	}
	return b.String()
}

func runWithTimeout(timeout time.Duration, name string, args ...string) int {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
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

func runInDir(dir, name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
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

func fmtFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func mustMkdir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		die("mkdir %s: %v", path, err)
	}
}

func compareImageFiles(refPath, gotPath string, tolerance uint8) (comparisonMetrics, *image.NRGBA, error) {
	refImg, err := loadComparisonImage(refPath)
	if err != nil {
		return comparisonMetrics{}, nil, fmt.Errorf("load reference image: %w", err)
	}
	gotImg, err := loadComparisonImage(gotPath)
	if err != nil {
		return comparisonMetrics{}, nil, fmt.Errorf("load go image: %w", err)
	}
	return compareImages(refImg, gotImg, tolerance)
}

func compareImages(refImg, gotImg image.Image, tolerance uint8) (comparisonMetrics, *image.NRGBA, error) {
	refBounds := refImg.Bounds()
	gotBounds := gotImg.Bounds()
	if refBounds.Dx() != gotBounds.Dx() || refBounds.Dy() != gotBounds.Dy() {
		return comparisonMetrics{}, nil, fmt.Errorf("dimension mismatch: reference=%dx%d go=%dx%d",
			refBounds.Dx(), refBounds.Dy(), gotBounds.Dx(), gotBounds.Dy())
	}

	width, height := refBounds.Dx(), refBounds.Dy()
	diffImage := image.NewNRGBA(image.Rect(0, 0, width, height))
	totalPixels := width * height
	var mismatchPixels, totalChannelDelta int
	var maxChannelDelta uint8

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			refColor := color.NRGBAModel.Convert(refImg.At(refBounds.Min.X+x, refBounds.Min.Y+y)).(color.NRGBA)
			gotColor := color.NRGBAModel.Convert(gotImg.At(gotBounds.Min.X+x, gotBounds.Min.Y+y)).(color.NRGBA)

			dr := absDiff(refColor.R, gotColor.R)
			dg := absDiff(refColor.G, gotColor.G)
			db := absDiff(refColor.B, gotColor.B)
			da := absDiff(refColor.A, gotColor.A)

			channelMax := maxUint8(maxUint8(dr, dg), maxUint8(db, da))
			if channelMax > maxChannelDelta {
				maxChannelDelta = channelMax
			}
			totalChannelDelta += int(dr) + int(dg) + int(db) + int(da)
			if channelMax > tolerance {
				mismatchPixels++
				diffImage.SetNRGBA(x, y, color.NRGBA{R: dr, G: dg, B: db, A: 255})
			}
		}
	}

	return comparisonMetrics{
		Width:            width,
		Height:           height,
		MismatchPixels:   mismatchPixels,
		TotalPixels:      totalPixels,
		MismatchPercent:  (float64(mismatchPixels) * 100) / float64(totalPixels),
		MeanChannelDelta: float64(totalChannelDelta) / float64(totalPixels*4),
		MaxChannelDelta:  maxChannelDelta,
	}, diffImage, nil
}

func loadComparisonImage(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".tga":
		return qimage.LoadTGA(bytes.NewReader(data))
	case ".png":
		img, err := qimage.LoadPNG(bytes.NewReader(data))
		if err == nil {
			return img, nil
		}
		return qimage.LoadTGA(bytes.NewReader(data))
	default:
		if img, err := qimage.LoadPNG(bytes.NewReader(data)); err == nil {
			return img, nil
		}
		return qimage.LoadTGA(bytes.NewReader(data))
	}
}

func normalizeCaptureToPNG(src, dst string) error {
	ext := strings.ToLower(filepath.Ext(src))
	if ext == ".png" {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		mustMkdir(filepath.Dir(dst))
		return os.WriteFile(dst, data, 0o644)
	}

	img, err := loadComparisonImage(src)
	if err != nil {
		return err
	}
	return writePNG(dst, img)
}

func writePNG(path string, img image.Image) error {
	mustMkdir(filepath.Dir(path))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func maxUint8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}

func clampUint8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func parseFloatEnv(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed float64
	if _, err := fmt.Sscanf(value, "%f", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func parseIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil {
		return fallback
	}
	return parsed
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func printUsage() {
	fmt.Println("Usage: go run ./tools/parity_screenshots {reference|go|compare|both}")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  reference  Capture reference screenshots from C Ironwail")
	fmt.Println("  go         Capture screenshots from Go port")
	fmt.Println("  compare    Compare reference vs Go screenshots")
	fmt.Println("  both       Do all three in sequence")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  QUAKE_BASEDIR  Path to Quake data")
	fmt.Println("  IRONWAIL_BIN   Path to C Ironwail binary")
	fmt.Println("  GO_BIN         Path to Go binary (default: ./ironwailgo-wgpu)")
}
