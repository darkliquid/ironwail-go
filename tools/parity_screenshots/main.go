package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
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
	Regions                     []diffRegion
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

func main() {
	os.Exit(run())
}

func run() int {
	mode := "help"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	projectDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "getwd: %v\n", err)
		return 1
	}

	viewpointsPath := filepath.Join(projectDir, "testdata", "parity", "viewpoints.json")
	cfg := loadViewpoints(viewpointsPath)
	quakeBaseDir := envOr("QUAKE_BASEDIR", cfg.BaseDir)
	if quakeBaseDir == "" {
		quakeBaseDir = "/home/darkliquid/Games/Heroic/Quake"
	}
	ironwailBin := envOr("IRONWAIL_BIN", filepath.Join(quakeBaseDir, "ironwail"))
	goBin := envOr("GO_BIN", filepath.Join(projectDir, "ironwailgo-cgo"))
	parityWidth := parseIntEnv("PARITY_WIDTH", 1280)
	parityHeight := parseIntEnv("PARITY_HEIGHT", 720)
	refDir := filepath.Join(projectDir, "testdata", "parity", "reference")
	goDir := filepath.Join(projectDir, "testdata", "parity", "go")
	diffDir := filepath.Join(projectDir, "testdata", "parity", "diff")

	checkDeps(viewpointsPath, quakeBaseDir)

	switch mode {
	case "reference", "ref":
		if _, err := captureReference(quakeBaseDir, ironwailBin, refDir, cfg.Viewpoints, parityWidth, parityHeight); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
	case "go":
		if _, err := captureGo(projectDir, quakeBaseDir, goBin, goDir, cfg.Viewpoints, parityWidth, parityHeight); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
	case "compare", "cmp":
		summary, err := compare(refDir, goDir, diffDir, cfg.Viewpoints)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		if compareFailed(summary) {
			return 1
		}
	case "both", "all":
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
		summary, err := compare(refDir, goDir, diffDir, cfg.Viewpoints)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			return 1
		}
		if compareFailed(summary) {
			return 1
		}
	default:
		printUsage()
		return 2
	}
	return 0
}

func captureReference(quakeBaseDir, ironwailBin, refDir string, viewpoints []viewpoint, width, height int) (captureSummary, error) {
	if _, err := os.Stat(ironwailBin); err != nil {
		return captureSummary{}, fmt.Errorf("C Ironwail binary not found: %s", ironwailBin)
	}
	mustMkdir(refDir)
	screenshotDir := filepath.Join(quakeBaseDir, "id1", "screenshots")
	mustMkdir(screenshotDir)

	fmt.Println("=== Capturing reference screenshots from C Ironwail ===")
	fmt.Println("Binary:", ironwailBin)
	fmt.Println("Output:", refDir)
	fmt.Println()

	summary := captureSummary{Count: len(viewpoints)}
	for _, vp := range viewpoints {
		fmt.Printf("  [REF] %s: %s\n", vp.ID, vp.Description)
		clearScreenshotMatches(screenshotDir, vp.ID)
		cfgFile := genReferenceCfg(filepath.Join(quakeBaseDir, "id1"), vp, width, height)
		args := []string{
			"-basedir", quakeBaseDir,
			"-window", "-width", fmt.Sprintf("%d", width), "-height", fmt.Sprintf("%d", height),
			"+map", vp.Map,
			"+exec", filepath.Base(cfgFile),
		}
		fmt.Printf("    exec: %s %s", ironwailBin, strings.Join(args, " "))
		if status := runWithTimeout(30*time.Second, captureEnv(true), ironwailBin, args...); status != 0 {
			fmt.Printf("    WARNING: C Ironwail exited with error for %s\n", vp.ID)
			summary.Failures++
		}
		_ = os.Remove(cfgFile)
		if err := moveFirstMatch(screenshotDir, vp.ID, filepath.Join(refDir, vp.ID+".png")); err != nil {
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
		if status := runInDir(projectDir, []string{"CGO_ENABLED=1"}, "go", "build", "-o", goBin, "./cmd/ironwailgo"); status != 0 {
			return captureSummary{}, errors.New("failed to build Go binary")
		}
	} else if _, err := os.Stat(goBin); err != nil {
		return captureSummary{}, fmt.Errorf("Go binary not found: %s", goBin)
	}
	mustMkdir(goDir)

	fmt.Println("=== Capturing Go port screenshots ===")
	fmt.Println("Binary:", goBin)
	fmt.Println("Output:", goDir)
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
		args := []string{
			"-basedir", quakeBaseDir,
			"-width", fmt.Sprintf("%d", captureWidth),
			"-height", fmt.Sprintf("%d", captureHeight),
			"-screenshot", filepath.Join(goDir, vp.ID+".png"),
			"+scr_viewsize", "130",
			"+r_drawviewmodel", "0",
			"+crosshair", "0",
			"+map", vp.Map,
			"+noclip",
			"+setpos",
			fmtFloat(vp.Pos[0]), fmtFloat(vp.Pos[1]), fmtFloat(vp.Pos[2]),
			fmtFloat(vp.Angles[0]), fmtFloat(vp.Angles[1]), fmtFloat(vp.Angles[2]),
		}
		fmt.Printf("    exec: %s %s", goBin, strings.Join(args, " "))
		if status := runWithTimeout(30*time.Second, captureEnv(false), goBin, args...); status != 0 {
			fmt.Printf("    WARNING: Go binary exited with error for %s\n", vp.ID)
			summary.Failures++
		}
		if _, err := os.Stat(filepath.Join(goDir, vp.ID+".png")); err != nil {
			summary.Failures++
			return summary, fmt.Errorf("capture go %s: missing screenshot %s", vp.ID, filepath.Join(goDir, vp.ID+".png"))
		}
		if changed, fromWidth, fromHeight, err := normalizeImageSize(filepath.Join(goDir, vp.ID+".png"), captureWidth, captureHeight); err != nil {
			summary.Failures++
			return summary, fmt.Errorf("normalize go %s: %w", vp.ID, err)
		} else if changed {
			fmt.Printf("    normalized Go image from %dx%d to %dx%d\n", fromWidth, fromHeight, captureWidth, captureHeight)
		}
	}

	fmt.Println()
	fmt.Printf("Go screenshots saved to: %s\n", goDir)
	if summary.Failures > 0 {
		return summary, fmt.Errorf("Go capture completed with %d failure(s)", summary.Failures)
	}
	return summary, nil
}

func compare(refDir, goDir, diffDir string, viewpoints []viewpoint) (compareSummary, error) {
	mustMkdir(diffDir)
	overlayDir := filepath.Join(filepath.Dir(diffDir), "overlay")
	mustMkdir(overlayDir)
	fmt.Println("=== Comparing screenshots ===")
	fmt.Println()

	tolerance := clampUint8(parseIntEnv("PARITY_COMPARE_TOLERANCE", 0))
	maxMismatchPercent := parseFloatEnv("PARITY_MAX_MISMATCH_PERCENT", 0)
	onionAlpha := clampFloat(parseFloatEnv("PARITY_ONION_ALPHA", 0.5), 0, 1)

	var summary compareSummary
	for _, vp := range viewpoints {
		refImg := filepath.Join(refDir, vp.ID+".png")
		goImg := filepath.Join(goDir, vp.ID+".png")

		if _, err := os.Stat(refImg); err != nil {
			fmt.Printf("  SKIP %s: no reference image\n", vp.ID)
			summary.MissingCount++
			continue
		}
		summary.ReferenceCount++

		if _, err := os.Stat(goImg); err != nil {
			fmt.Printf("  MISS %s: no Go screenshot\n", vp.ID)
			summary.MissingCount++
			continue
		}
		summary.GoCount++

		metrics, diffImage, overlayImage, err := compareImageFiles(refImg, goImg, tolerance, onionAlpha)
		if err != nil {
			fmt.Printf("  DIFF %s: compare failed: %v (%s)\n", vp.ID, err, vp.Description)
			summary.DiffCount++
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

		if metrics.MismatchPercent <= maxMismatchPercent {
			fmt.Printf("  OK   %s: %.4f%% pixels differ, mean Δ %.2f, perceptual Δ %.2f, max Δ %d (%s)\n",
				vp.ID, metrics.MismatchPercent, metrics.MeanChannelDelta, metrics.MeanPerceptualDelta, metrics.MaxChannelDelta, vp.Description)
			for _, line := range formatMetricDetails(metrics) {
				fmt.Printf("       %s\n", line)
			}
			fmt.Printf("       onion=%s (alpha %.2f ref / %.2f go)\n", overlayPath, onionAlpha, 1-onionAlpha)
			summary.MatchCount++
		} else {
			fmt.Printf("  DIFF %s: %.4f%% pixels differ, mean Δ %.2f, perceptual Δ %.2f, max Δ %d, diff=%s (%s)\n",
				vp.ID, metrics.MismatchPercent, metrics.MeanChannelDelta, metrics.MeanPerceptualDelta, metrics.MaxChannelDelta, diffPath, vp.Description)
			for _, line := range formatMetricDetails(metrics) {
				fmt.Printf("       %s\n", line)
			}
			fmt.Printf("       onion=%s (alpha %.2f ref / %.2f go)\n", overlayPath, onionAlpha, 1-onionAlpha)
			summary.DiffCount++
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
	fmt.Printf("Tolerance:        channel Δ <= %d, mismatch threshold <= %.4f%%, onion alpha %.2f ref / %.2f go\n", tolerance, maxMismatchPercent, onionAlpha, 1-onionAlpha)
	if summary.ReferenceCount == 0 {
		return summary, errors.New("no reference images found")
	}
	return summary, nil
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

func genReferenceCfg(dir string, vp viewpoint, width, height int) string {
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
vid_fullscreen 0
vid_width %d
vid_height %d
scr_viewsize 130
r_drawviewmodel 0
crosshair 0
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
`, width, height, vp.ID, preToggleWaits, postToggleWaits, fmtFloat(vp.Pos[0]), fmtFloat(vp.Pos[1]), fmtFloat(vp.Pos[2]), fmtFloat(vp.Angles[0]), fmtFloat(vp.Angles[1]), fmtFloat(vp.Angles[2]), preShotWaits)
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

func captureEnv(reference bool) []string {
	env := []string{
		"WAYLAND_DISPLAY=",
		"XDG_SESSION_TYPE=x11",
	}
	if reference {
		env = append(env, "SDL_VIDEODRIVER=x11")
	}
	return env
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

func compareImageFiles(refPath, gotPath string, tolerance uint8, onionAlpha float64) (comparisonMetrics, *image.NRGBA, *image.NRGBA, error) {
	refImg, err := loadComparisonImage(refPath)
	if err != nil {
		return comparisonMetrics{}, nil, nil, fmt.Errorf("load reference image: %w", err)
	}
	gotImg, err := loadComparisonImage(gotPath)
	if err != nil {
		return comparisonMetrics{}, nil, nil, fmt.Errorf("load go image: %w", err)
	}
	return compareImages(refImg, gotImg, tolerance, onionAlpha)
}

func refDirForGo(goDir string) string {
	return filepath.Join(filepath.Dir(goDir), "reference")
}

func shouldBuildGoBinary(projectDir, goBin string) bool {
	if strings.TrimSpace(os.Getenv("PARITY_SKIP_GO_BUILD")) != "" {
		return false
	}
	projectAbs, err := filepath.Abs(projectDir)
	if err != nil {
		return false
	}
	goBinAbs, err := filepath.Abs(goBin)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(projectAbs, goBinAbs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func resolveGoCaptureSize(refPath string, fallbackWidth, fallbackHeight int) (int, int, bool, error) {
	if _, err := os.Stat(refPath); err != nil {
		if os.IsNotExist(err) {
			return fallbackWidth, fallbackHeight, false, nil
		}
		return 0, 0, false, err
	}
	width, height, err := imageSize(refPath)
	if err != nil {
		return 0, 0, false, err
	}
	return width, height, true, nil
}

func imageSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func normalizeImageSize(path string, targetWidth, targetHeight int) (bool, int, int, error) {
	width, height, err := imageSize(path)
	if err != nil {
		return false, 0, 0, err
	}
	if width == targetWidth && height == targetHeight {
		return false, width, height, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return false, width, height, err
	}
	defer f.Close()
	src, _, err := image.Decode(f)
	if err != nil {
		return false, width, height, err
	}
	resized := image.NewNRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	scaleNearest(resized, src)

	out, err := os.Create(path)
	if err != nil {
		return false, width, height, err
	}
	defer out.Close()
	if err := png.Encode(out, resized); err != nil {
		return false, width, height, err
	}
	return true, width, height, nil
}

func scaleNearest(dst *image.NRGBA, src image.Image) {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	dstWidth := dst.Bounds().Dx()
	dstHeight := dst.Bounds().Dy()
	for y := 0; y < dstHeight; y++ {
		srcY := srcBounds.Min.Y + y*srcHeight/dstHeight
		for x := 0; x < dstWidth; x++ {
			srcX := srcBounds.Min.X + x*srcWidth/dstWidth
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
}

func compareImages(refImg, gotImg image.Image, tolerance uint8, onionAlpha float64) (comparisonMetrics, *image.NRGBA, *image.NRGBA, error) {
	refBounds := refImg.Bounds()
	gotBounds := gotImg.Bounds()
	if refBounds.Dx() != gotBounds.Dx() || refBounds.Dy() != gotBounds.Dy() {
		return comparisonMetrics{}, nil, nil, fmt.Errorf("dimension mismatch: reference=%dx%d go=%dx%d",
			refBounds.Dx(), refBounds.Dy(), gotBounds.Dx(), gotBounds.Dy())
	}

	width, height := refBounds.Dx(), refBounds.Dy()
	diffImage := image.NewNRGBA(image.Rect(0, 0, width, height))
	overlayImage := image.NewNRGBA(image.Rect(0, 0, width, height))
	totalPixels := width * height
	mismatchMask := make([]bool, totalPixels)
	var mismatchPixels, totalChannelDelta int
	var totalRedDelta, totalGreenDelta, totalBlueDelta, totalAlphaDelta int
	var maxChannelDelta uint8
	var totalPerceptualDelta, mismatchPerceptualDelta, maxPerceptualDelta float64

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
			overlayImage.SetNRGBA(x, y, blendNRGBA(refColor, gotColor, onionAlpha))
			perceptualDelta := perceptualColorDelta(dr, dg, db)
			if perceptualDelta > maxPerceptualDelta {
				maxPerceptualDelta = perceptualDelta
			}
			totalPerceptualDelta += perceptualDelta
			totalChannelDelta += int(dr) + int(dg) + int(db) + int(da)
			totalRedDelta += int(dr)
			totalGreenDelta += int(dg)
			totalBlueDelta += int(db)
			totalAlphaDelta += int(da)
			if channelMax > tolerance {
				mismatchPixels++
				mismatchMask[y*width+x] = true
				mismatchPerceptualDelta += perceptualDelta
				diffImage.SetNRGBA(x, y, color.NRGBA{R: dr, G: dg, B: db, A: 255})
			}
		}
	}

	regions := findDiffRegions(mismatchMask, width, height)
	annotateDiffRegions(diffImage, regions)
	meanMismatchPerceptualDelta := 0.0
	if mismatchPixels > 0 {
		meanMismatchPerceptualDelta = mismatchPerceptualDelta / float64(mismatchPixels)
	}

	return comparisonMetrics{
		Width:                       width,
		Height:                      height,
		MismatchPixels:              mismatchPixels,
		TotalPixels:                 totalPixels,
		MismatchPercent:             (float64(mismatchPixels) * 100) / float64(totalPixels),
		MeanChannelDelta:            float64(totalChannelDelta) / float64(totalPixels*4),
		MaxChannelDelta:             maxChannelDelta,
		MeanRedDelta:                float64(totalRedDelta) / float64(totalPixels),
		MeanGreenDelta:              float64(totalGreenDelta) / float64(totalPixels),
		MeanBlueDelta:               float64(totalBlueDelta) / float64(totalPixels),
		MeanAlphaDelta:              float64(totalAlphaDelta) / float64(totalPixels),
		MeanPerceptualDelta:         totalPerceptualDelta / float64(totalPixels),
		MeanMismatchPerceptualDelta: meanMismatchPerceptualDelta,
		MaxPerceptualDelta:          maxPerceptualDelta,
		Regions:                     regions,
	}, diffImage, overlayImage, nil
}

func blendNRGBA(refColor, gotColor color.NRGBA, refAlpha float64) color.NRGBA {
	goAlpha := 1 - refAlpha
	return color.NRGBA{
		R: blendChannel(refColor.R, gotColor.R, refAlpha, goAlpha),
		G: blendChannel(refColor.G, gotColor.G, refAlpha, goAlpha),
		B: blendChannel(refColor.B, gotColor.B, refAlpha, goAlpha),
		A: blendChannel(refColor.A, gotColor.A, refAlpha, goAlpha),
	}
}

func blendChannel(refValue, goValue uint8, refAlpha, goAlpha float64) uint8 {
	value := float64(refValue)*refAlpha + float64(goValue)*goAlpha
	if value <= 0 {
		return 0
	}
	if value >= 255 {
		return 255
	}
	return uint8(math.Round(value))
}

func formatMetricDetails(metrics comparisonMetrics) []string {
	lines := []string{
		fmt.Sprintf("color |ΔRGBA| mean=(%.2f, %.2f, %.2f, %.2f); perceptual mean/max=%.2f/%.2f; mismatched-pixel perceptual mean=%.2f",
			metrics.MeanRedDelta, metrics.MeanGreenDelta, metrics.MeanBlueDelta, metrics.MeanAlphaDelta,
			metrics.MeanPerceptualDelta, metrics.MaxPerceptualDelta, metrics.MeanMismatchPerceptualDelta),
	}
	if len(metrics.Regions) == 0 {
		return lines
	}
	var regionParts []string
	for i, region := range metrics.Regions {
		if i >= 3 {
			break
		}
		regionParts = append(regionParts, formatDiffRegion(region, metrics.TotalPixels))
	}
	lines = append(lines, fmt.Sprintf("regions=%d; largest: %s", len(metrics.Regions), strings.Join(regionParts, "; ")))
	return lines
}

func formatDiffRegion(region diffRegion, totalPixels int) string {
	return fmt.Sprintf("x=%d..%d y=%d..%d size=%dx%d pixels=%d (%.4f%%)",
		region.MinX, region.MaxX, region.MinY, region.MaxY, region.MaxX-region.MinX+1, region.MaxY-region.MinY+1,
		region.Pixels, (float64(region.Pixels)*100)/float64(totalPixels))
}

func perceptualColorDelta(dr, dg, db uint8) float64 {
	return math.Sqrt(0.2126*float64(dr)*float64(dr) + 0.7152*float64(dg)*float64(dg) + 0.0722*float64(db)*float64(db))
}

func findDiffRegions(mask []bool, width, height int) []diffRegion {
	if len(mask) == 0 {
		return nil
	}
	visited := make([]bool, len(mask))
	regions := make([]diffRegion, 0, 4)
	queue := make([]int, 0, 256)
	for idx, mismatched := range mask {
		if !mismatched || visited[idx] {
			continue
		}
		visited[idx] = true
		queue = queue[:0]
		queue = append(queue, idx)
		x0, y0 := idx%width, idx/width
		region := diffRegion{MinX: x0, MaxX: x0, MinY: y0, MaxY: y0}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			x, y := current%width, current/width
			region.Pixels++
			if x < region.MinX {
				region.MinX = x
			}
			if x > region.MaxX {
				region.MaxX = x
			}
			if y < region.MinY {
				region.MinY = y
			}
			if y > region.MaxY {
				region.MaxY = y
			}
			for _, next := range neighborIndices(x, y, width, height) {
				if !mask[next] || visited[next] {
					continue
				}
				visited[next] = true
				queue = append(queue, next)
			}
		}
		regions = append(regions, region)
	}
	sort.Slice(regions, func(i, j int) bool {
		if regions[i].Pixels != regions[j].Pixels {
			return regions[i].Pixels > regions[j].Pixels
		}
		if regions[i].MinY != regions[j].MinY {
			return regions[i].MinY < regions[j].MinY
		}
		return regions[i].MinX < regions[j].MinX
	})
	return regions
}

func neighborIndices(x, y, width, height int) []int {
	neighbors := make([]int, 0, 4)
	if x > 0 {
		neighbors = append(neighbors, y*width+x-1)
	}
	if x+1 < width {
		neighbors = append(neighbors, y*width+x+1)
	}
	if y > 0 {
		neighbors = append(neighbors, (y-1)*width+x)
	}
	if y+1 < height {
		neighbors = append(neighbors, (y+1)*width+x)
	}
	return neighbors
}

func annotateDiffRegions(diffImage *image.NRGBA, regions []diffRegion) {
	highlight := color.NRGBA{R: 255, G: 255, B: 0, A: 255}
	for i, region := range regions {
		if i >= 3 {
			break
		}
		for x := region.MinX; x <= region.MaxX; x++ {
			diffImage.SetNRGBA(x, region.MinY, highlight)
			diffImage.SetNRGBA(x, region.MaxY, highlight)
		}
		for y := region.MinY; y <= region.MaxY; y++ {
			diffImage.SetNRGBA(region.MinX, y, highlight)
			diffImage.SetNRGBA(region.MaxX, y, highlight)
		}
	}
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

func clampFloat(v, minValue, maxValue float64) float64 {
	if v < minValue {
		return minValue
	}
	if v > maxValue {
		return maxValue
	}
	return v
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

func compareFailed(summary compareSummary) bool {
	return summary.ReferenceCount == 0 || summary.DiffCount > 0 || summary.MissingCount > 0 || summary.GoCount != summary.ReferenceCount
}

func printUsage() {
	fmt.Println("Usage: go run ./tools/parity_screenshots {reference|go|compare|both}")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  reference  Capture reference screenshots from C Ironwail")
	fmt.Println("  go         Capture screenshots from the Go CGO/OpenGL parity build")
	fmt.Println("  compare    Compare reference vs Go screenshots (nonzero on diffs/missing captures)")
	fmt.Println("  both       Do all three in sequence (nonzero on diffs/missing captures)")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  QUAKE_BASEDIR  Path to Quake data")
	fmt.Println("  IRONWAIL_BIN   Path to C Ironwail binary")
	fmt.Println("  GO_BIN         Path to Go binary (default: ./ironwailgo-cgo)")
	fmt.Println("  PARITY_ONION_ALPHA  Blend weight for reference image in overlay output (default: 0.5)")
}
