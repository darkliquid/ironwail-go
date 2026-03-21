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

	metrics, diff, err := compareImages(ref, got, 0)
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
	if diff.NRGBAAt(0, 0).A != 0 || diff.NRGBAAt(1, 0).A != 0 {
		t.Fatalf("diff image should remain transparent for exact matches")
	}
}

func TestCompareImagesCountsMismatchedPixels(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	got := image.NewNRGBA(ref.Bounds())

	ref.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	ref.SetNRGBA(1, 0, color.NRGBA{R: 40, G: 50, B: 60, A: 255})
	got.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	got.SetNRGBA(1, 0, color.NRGBA{R: 60, G: 70, B: 80, A: 255})

	metrics, diff, err := compareImages(ref, got, 0)
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
	if got := diff.NRGBAAt(1, 0); got.A != 255 {
		t.Fatalf("diff alpha = %d, want 255 for mismatched pixel", got.A)
	}
}

func TestCompareImagesHonorsTolerance(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	got := image.NewNRGBA(ref.Bounds())

	ref.SetNRGBA(0, 0, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	got.SetNRGBA(0, 0, color.NRGBA{R: 101, G: 100, B: 100, A: 255})

	metrics, _, err := compareImages(ref, got, 1)
	if err != nil {
		t.Fatalf("compareImages returned error: %v", err)
	}
	if metrics.MismatchPixels != 0 {
		t.Fatalf("MismatchPixels = %d, want 0 with tolerance", metrics.MismatchPixels)
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

func TestGenReferenceCfgClosesConsoleBeforeScreenshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := genReferenceCfg(dir, viewpoint{
		ID:     "start_spawn",
		Map:    "start",
		Pos:    [3]float64{480, 64, 88},
		Angles: [3]float64{0, 90, 0},
	})
	t.Cleanup(func() { _ = os.Remove(cfgPath) })

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "scr_conspeed 999999") {
		t.Fatalf("cfg missing fast console close setting:\n%s", text)
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
