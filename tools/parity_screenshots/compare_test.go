package main

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestSSIMThresholdAndDiffMask(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	got := image.NewNRGBA(image.Rect(0, 0, 16, 16))

	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			ref.SetNRGBA(x, y, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
			got.SetNRGBA(x, y, color.NRGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	// Introduce difference in a quadrant
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			got.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	metrics, diff, _, err := compareImages(ref, got, 0, 0.5)
	if err != nil {
		t.Fatalf("compareImages error: %v", err)
	}

	if metrics.MismatchPixels == 0 {
		t.Fatalf("expected mismatch pixels > 0")
	}
	if metrics.MeanSSIM >= 1.0 {
		t.Fatalf("expected MeanSSIM < 1.0 for differing images, got %f", metrics.MeanSSIM)
	}

	// Verify diff image has red highlight at (0,0)
	c := diff.NRGBAAt(0, 0)
	if c.R != 255 || c.A != 255 {
		t.Fatalf("expected bright red diff pixel at (0,0), got R=%d A=%d", c.R, c.A)
	}
}

func TestWriteParityReport(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdir(filepath.Join(tmpDir, "testdata", "parity"))

	results := []viewpointResult{
		{
			ID:          "test-vp",
			Map:         "start",
			Description: "test description",
			Status:      "ok",
			MeanSSIM:    0.995,
		},
	}
	summary := compareSummary{
		ReferenceCount: 1,
		GoCount:        1,
		MatchCount:     1,
	}

	writeParityReport(tmpDir, results, summary)

	reportPath := filepath.Join(tmpDir, "testdata", "parity", "results.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("results.json was not created at %s", reportPath)
	}
}
