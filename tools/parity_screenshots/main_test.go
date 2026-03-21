package main

import (
	"image"
	"image/color"
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
