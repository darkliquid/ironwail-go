package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareImagesExactMatch(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	ref.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	ref.SetNRGBA(1, 0, color.NRGBA{R: 40, G: 50, B: 60, A: 255})

	got := image.NewNRGBA(ref.Bounds())
	copy(got.Pix, ref.Pix)

	metrics, diff, overlay, err := compareImages(ref, got, 0, 0.5)
	if err != nil {
		t.Fatalf("compareImages returned error: %v", err)
	}
	if metrics.MismatchPixels != 0 {
		t.Fatalf("MismatchPixels = %d, want 0", metrics.MismatchPixels)
	}
	if metrics.MismatchPercent != 0 {
		t.Fatalf("MismatchPercent = %f, want 0", metrics.MismatchPercent)
	}
	if metrics.MaxChannelDelta != 0 {
		t.Fatalf("MaxChannelDelta = %d, want 0", metrics.MaxChannelDelta)
	}
	if metrics.MeanPerceptualDelta != 0 || metrics.MaxPerceptualDelta != 0 {
		t.Fatalf("perceptual deltas = %.2f / %.2f, want 0 / 0", metrics.MeanPerceptualDelta, metrics.MaxPerceptualDelta)
	}
	if len(metrics.Regions) != 0 {
		t.Fatalf("Regions = %d, want 0", len(metrics.Regions))
	}
	if diff.NRGBAAt(0, 0).A != 0 || diff.NRGBAAt(1, 0).A != 0 {
		t.Fatalf("diff image should remain transparent for exact matches")
	}
	if overlay.NRGBAAt(0, 0) != ref.NRGBAAt(0, 0) || overlay.NRGBAAt(1, 0) != ref.NRGBAAt(1, 0) {
		t.Fatalf("overlay should equal source image when images match")
	}
}

func TestCompareImagesCountsMismatchedPixels(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	got := image.NewNRGBA(ref.Bounds())

	ref.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	ref.SetNRGBA(1, 0, color.NRGBA{R: 40, G: 50, B: 60, A: 255})
	got.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	got.SetNRGBA(1, 0, color.NRGBA{R: 60, G: 70, B: 80, A: 255})

	metrics, diff, overlay, err := compareImages(ref, got, 0, 0.5)
	if err != nil {
		t.Fatalf("compareImages returned error: %v", err)
	}
	if metrics.MismatchPixels != 1 {
		t.Fatalf("MismatchPixels = %d, want 1", metrics.MismatchPixels)
	}
	if metrics.TotalPixels != 2 {
		t.Fatalf("TotalPixels = %d, want 2", metrics.TotalPixels)
	}
	if metrics.MaxChannelDelta == 0 {
		t.Fatalf("MaxChannelDelta = 0, want > 0")
	}
	if metrics.MeanRedDelta == 0 || metrics.MeanGreenDelta == 0 || metrics.MeanBlueDelta == 0 {
		t.Fatalf("mean RGB deltas = %.2f/%.2f/%.2f, want nonzero", metrics.MeanRedDelta, metrics.MeanGreenDelta, metrics.MeanBlueDelta)
	}
	if metrics.MeanPerceptualDelta == 0 || metrics.MeanMismatchPerceptualDelta == 0 || metrics.MaxPerceptualDelta == 0 {
		t.Fatalf("perceptual deltas = %.2f/%.2f/%.2f, want nonzero", metrics.MeanPerceptualDelta, metrics.MeanMismatchPerceptualDelta, metrics.MaxPerceptualDelta)
	}
	if len(metrics.Regions) != 1 {
		t.Fatalf("Regions = %d, want 1", len(metrics.Regions))
	}
	if got := metrics.Regions[0]; got.MinX != 1 || got.MaxX != 1 || got.MinY != 0 || got.MaxY != 0 || got.Pixels != 1 {
		t.Fatalf("unexpected region = %+v", got)
	}
	if got := diff.NRGBAAt(1, 0); got.A != 255 {
		t.Fatalf("diff alpha = %d, want 255 for mismatched pixel", got.A)
	}
	if got := overlay.NRGBAAt(1, 0); got.R != 50 || got.G != 60 || got.B != 70 || got.A != 255 {
		t.Fatalf("unexpected overlay pixel = %+v", got)
	}
}

func TestCompareImagesHonorsTolerance(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	got := image.NewNRGBA(ref.Bounds())

	ref.SetNRGBA(0, 0, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	got.SetNRGBA(0, 0, color.NRGBA{R: 101, G: 100, B: 100, A: 255})

	metrics, _, _, err := compareImages(ref, got, 1, 0.5)
	if err != nil {
		t.Fatalf("compareImages returned error: %v", err)
	}
	if metrics.MismatchPixels != 0 {
		t.Fatalf("MismatchPixels = %d, want 0 with tolerance", metrics.MismatchPixels)
	}
}

func TestCompareImagesFindsSeparateDiffRegions(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	got := image.NewNRGBA(ref.Bounds())
	copy(got.Pix, ref.Pix)

	got.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	got.SetNRGBA(2, 2, color.NRGBA{G: 255, A: 255})

	metrics, diff, _, err := compareImages(ref, got, 0, 0.5)
	if err != nil {
		t.Fatalf("compareImages returned error: %v", err)
	}
	if len(metrics.Regions) != 2 {
		t.Fatalf("Regions = %d, want 2", len(metrics.Regions))
	}
	if first := metrics.Regions[0]; first.MinX != 0 || first.MaxX != 0 || first.MinY != 0 || first.MaxY != 0 || first.Pixels != 1 {
		t.Fatalf("unexpected first region = %+v", first)
	}
	if second := metrics.Regions[1]; second.MinX != 2 || second.MaxX != 2 || second.MinY != 2 || second.MaxY != 2 || second.Pixels != 1 {
		t.Fatalf("unexpected second region = %+v", second)
	}
	if got := diff.NRGBAAt(0, 0); got.R != 255 || got.G != 255 || got.B != 0 || got.A != 255 {
		t.Fatalf("expected highlighted region corner at (0,0), got %+v", got)
	}
	if got := diff.NRGBAAt(2, 2); got.R != 255 || got.G != 255 || got.B != 0 || got.A != 255 {
		t.Fatalf("expected highlighted region corner at (2,2), got %+v", got)
	}
}

func TestBlendNRGBAUsesRequestedReferenceAlpha(t *testing.T) {
	got := blendNRGBA(
		color.NRGBA{R: 200, G: 100, B: 0, A: 255},
		color.NRGBA{R: 100, G: 0, B: 200, A: 255},
		0.75,
	)
	if got.R != 175 || got.G != 75 || got.B != 50 || got.A != 255 {
		t.Fatalf("blendNRGBA = %+v, want {R:175 G:75 B:50 A:255}", got)
	}
}

func TestMoveFirstMatchFindsScreenshotInDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "start_spawn.png")
	dst := filepath.Join(dir, "normalized", "start_spawn.png")

	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255})
	f, err := os.Create(src)
	if err != nil {
		t.Fatalf("create source image: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("encode source image: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close source image: %v", err)
	}

	if err := moveFirstMatch(dir, "start_spawn", dst); err != nil {
		t.Fatalf("moveFirstMatch returned error: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("destination image missing: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source image still exists or unexpected error: %v", err)
	}
}

func TestMoveFirstMatchFailsWhenScreenshotMissing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	err := moveFirstMatch(dir, "missing_view", filepath.Join(dir, "out.png"))
	if err == nil {
		t.Fatal("moveFirstMatch returned nil error, want failure")
	}
}

func TestNormalizeImageSizeResizesMismatchedImage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "capture.png")
	src := image.NewNRGBA(image.Rect(0, 0, 4, 3))
	src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	src.SetNRGBA(3, 2, color.NRGBA{B: 255, A: 255})

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if err := png.Encode(f, src); err != nil {
		f.Close()
		t.Fatalf("encode image: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close image: %v", err)
	}

	changed, fromWidth, fromHeight, err := normalizeImageSize(path, 2, 2)
	if err != nil {
		t.Fatalf("normalizeImageSize returned error: %v", err)
	}
	if !changed {
		t.Fatal("normalizeImageSize did not report a size change")
	}
	if fromWidth != 4 || fromHeight != 3 {
		t.Fatalf("normalizeImageSize reported source size %dx%d, want 4x3", fromWidth, fromHeight)
	}

	in, err := os.Open(path)
	if err != nil {
		t.Fatalf("open normalized image: %v", err)
	}
	defer in.Close()
	got, _, err := image.Decode(in)
	if err != nil {
		t.Fatalf("decode normalized image: %v", err)
	}
	if got.Bounds().Dx() != 2 || got.Bounds().Dy() != 2 {
		t.Fatalf("normalized image size = %dx%d, want 2x2", got.Bounds().Dx(), got.Bounds().Dy())
	}
}

func TestResolveGoCaptureSizeUsesReferenceImageDimensions(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	refPath := filepath.Join(dir, "reference.png")
	ref := image.NewNRGBA(image.Rect(0, 0, 7, 5))

	f, err := os.Create(refPath)
	if err != nil {
		t.Fatalf("create reference image: %v", err)
	}
	if err := png.Encode(f, ref); err != nil {
		f.Close()
		t.Fatalf("encode reference image: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close reference image: %v", err)
	}

	width, height, usedReference, err := resolveGoCaptureSize(refPath, 1280, 720)
	if err != nil {
		t.Fatalf("resolveGoCaptureSize returned error: %v", err)
	}
	if !usedReference {
		t.Fatal("resolveGoCaptureSize did not use reference dimensions")
	}
	if width != 7 || height != 5 {
		t.Fatalf("resolveGoCaptureSize = %dx%d, want 7x5", width, height)
	}
}

func TestResolveGoCaptureSizeFallsBackWithoutReference(t *testing.T) {
	t.Parallel()

	width, height, usedReference, err := resolveGoCaptureSize(filepath.Join(t.TempDir(), "missing.png"), 1280, 720)
	if err != nil {
		t.Fatalf("resolveGoCaptureSize returned error: %v", err)
	}
	if usedReference {
		t.Fatal("resolveGoCaptureSize unexpectedly used reference dimensions")
	}
	if width != 1280 || height != 720 {
		t.Fatalf("resolveGoCaptureSize = %dx%d, want fallback 1280x720", width, height)
	}
}

func TestShouldBuildGoBinaryForProjectLocalBinary(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	goBin := filepath.Join(projectDir, "ironwailgo-wgpu")
	if !shouldBuildGoBinary(projectDir, goBin) {
		t.Fatal("shouldBuildGoBinary returned false for project-local binary")
	}
}

func TestShouldBuildGoBinarySkipsExternalBinary(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	goBin := filepath.Join(t.TempDir(), "ironwailgo-wgpu")
	if shouldBuildGoBinary(projectDir, goBin) {
		t.Fatal("shouldBuildGoBinary returned true for external binary")
	}
}

func TestShouldBuildGoBinaryHonorsSkipEnv(t *testing.T) {
	projectDir := t.TempDir()
	goBin := filepath.Join(projectDir, "ironwailgo-wgpu")
	t.Setenv("PARITY_SKIP_GO_BUILD", "1")
	if shouldBuildGoBinary(projectDir, goBin) {
		t.Fatal("shouldBuildGoBinary ignored PARITY_SKIP_GO_BUILD")
	}
}

func TestGenReferenceCfgClosesConsoleBeforeScreenshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := genReferenceCfg(dir, viewpoint{
		ID:     "start_spawn",
		Map:    "start",
		Pos:    [3]float64{480, 64, 88},
		Angles: [3]float64{0, 90, 0},
	}, 1280, 720)
	t.Cleanup(func() { _ = os.Remove(cfgPath) })

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "scr_conspeed 999999") {
		t.Fatalf("cfg missing fast console close setting:\n%s", text)
	}
	if !strings.Contains(text, "\nvid_width 1280\n") || !strings.Contains(text, "\nvid_height 720\n") {
		t.Fatalf("cfg missing explicit video resolution:\n%s", text)
	}
	if !strings.Contains(text, "\nscr_viewsize 130\n") {
		t.Fatalf("cfg missing cleanliness view size:\n%s", text)
	}
	if !strings.Contains(text, "\nr_drawviewmodel 0\n") {
		t.Fatalf("cfg missing hidden viewmodel setting:\n%s", text)
	}
	if !strings.Contains(text, "\ncrosshair 0\n") {
		t.Fatalf("cfg missing hidden crosshair setting:\n%s", text)
	}
	if !strings.Contains(text, "\ntoggleconsole\n") {
		t.Fatalf("cfg missing toggleconsole command:\n%s", text)
	}
	if !strings.Contains(text, "\nhost_framerate 0.0001\n") {
		t.Fatalf("cfg missing freeze-time command:\n%s", text)
	}
	if !strings.Contains(text, "cl_screenshotname screenshots/start_spawn") {
		t.Fatalf("cfg missing deterministic screenshot name:\n%s", text)
	}

	toggleIdx := strings.Index(text, "\ntoggleconsole\n")
	setposIdx := strings.Index(text, "\nsetpos ")
	freezeIdx := strings.Index(text, "\nhost_framerate 0.0001\n")
	shotIdx := strings.Index(text, "\nscreenshot png\n")
	if toggleIdx == -1 || setposIdx == -1 || freezeIdx == -1 || shotIdx == -1 {
		t.Fatalf("cfg missing expected command ordering markers:\n%s", text)
	}
	if !(toggleIdx < freezeIdx && freezeIdx < setposIdx && setposIdx < shotIdx) {
		t.Fatalf("expected toggleconsole -> host_framerate -> setpos -> screenshot ordering:\n%s", text)
	}
	if waits := countWaitLines(text[:toggleIdx]); waits < 40 {
		t.Fatalf("expected substantial startup waits before toggleconsole, found %d:\n%s", waits, text)
	}
	if waits := countWaitLines(text[toggleIdx:freezeIdx]); waits < 10 {
		t.Fatalf("expected console-close waits before freeze/setpos, found %d:\n%s", waits, text)
	}
}

func countWaitLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "wait" {
			count++
		}
	}
	return count
}

func TestCompareFailed(t *testing.T) {
	t.Parallel()

	if compareFailed(compareSummary{ReferenceCount: 2, GoCount: 2, MatchCount: 2}) {
		t.Fatal("compareFailed returned true for a clean comparison")
	}
	if !compareFailed(compareSummary{ReferenceCount: 0}) {
		t.Fatal("compareFailed returned false for missing references")
	}
	if !compareFailed(compareSummary{ReferenceCount: 2, GoCount: 1, MatchCount: 1, MissingCount: 1}) {
		t.Fatal("compareFailed returned false for missing go screenshots")
	}
	if !compareFailed(compareSummary{ReferenceCount: 2, GoCount: 2, DiffCount: 1}) {
		t.Fatal("compareFailed returned false for diffs")
	}
}
