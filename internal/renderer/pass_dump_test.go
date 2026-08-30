package renderer

import (
	"bytes"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodeLinearizedDepthToGrayscalePNG(t *testing.T) {
	width, height := 4, 4
	depths := make([]float32, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			depths[y*width+x] = float32(y*width+x) / float32(width*height-1)
		}
	}

	t.Run("raw normalized depth to gray", func(t *testing.T) {
		img := EncodeLinearizedDepthToGrayImage(depths, width, height, 0, 0)
		if img == nil {
			t.Fatalf("expected non-nil image")
		}
		if img.Rect.Dx() != width || img.Rect.Dy() != height {
			t.Fatalf("expected dimensions %dx%d, got %dx%d", width, height, img.Rect.Dx(), img.Rect.Dy())
		}
		if img.GrayAt(0, 0).Y != 0 {
			t.Errorf("expected pixel (0,0) to be 0, got %d", img.GrayAt(0, 0).Y)
		}
		if img.GrayAt(width-1, height-1).Y != 255 {
			t.Errorf("expected pixel (%d,%d) to be 255, got %d", width-1, height-1, img.GrayAt(width-1, height-1).Y)
		}
	})

	t.Run("perspective depth linearization", func(t *testing.T) {
		near := float32(4.0)
		far := float32(4096.0)
		img := EncodeLinearizedDepthToGrayImage(depths, width, height, near, far)
		if img == nil {
			t.Fatalf("expected non-nil linearized depth image")
		}
		// First pixel is d=0 -> linear=0 -> 0
		if img.GrayAt(0, 0).Y != 0 {
			t.Errorf("expected pixel (0,0) to be 0, got %d", img.GrayAt(0, 0).Y)
		}
		// Last pixel is d=1 -> linear=1 -> 255
		if img.GrayAt(width-1, height-1).Y != 255 {
			t.Errorf("expected pixel (%d,%d) to be 255, got %d", width-1, height-1, img.GrayAt(width-1, height-1).Y)
		}
	})

	t.Run("depth byte buffer with padding", func(t *testing.T) {
		bytesPerRow := 256
		buf := make([]byte, bytesPerRow*height)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				d := float32(x) / float32(width-1)
				bits := math.Float32bits(d)
				idx := y*bytesPerRow + x*4
				buf[idx+0] = byte(bits)
				buf[idx+1] = byte(bits >> 8)
				buf[idx+2] = byte(bits >> 16)
				buf[idx+3] = byte(bits >> 24)
			}
		}
		img := EncodeDepthBytesToGrayImage(buf, width, height, bytesPerRow, 0, 0)
		if img == nil {
			t.Fatalf("expected non-nil depth image from bytes")
		}
		if img.GrayAt(0, 0).Y != 0 {
			t.Errorf("expected (0,0) to be 0, got %d", img.GrayAt(0, 0).Y)
		}
		if img.GrayAt(width-1, 0).Y != 255 {
			t.Errorf("expected (%d,0) to be 255, got %d", width-1, img.GrayAt(width-1, 0).Y)
		}
	})

	t.Run("save and decode PNG", func(t *testing.T) {
		img := EncodeLinearizedDepthToGrayImage(depths, width, height, 4.0, 4096.0)
		tmpFile := filepath.Join(t.TempDir(), "depth_test.png")
		if err := saveImagePNG(img, tmpFile); err != nil {
			t.Fatalf("failed to save depth png: %v", err)
		}
		f, err := os.Open(tmpFile)
		if err != nil {
			t.Fatalf("failed to open saved png: %v", err)
		}
		defer func() { _ = f.Close() }()
		decoded, err := png.Decode(f)
		if err != nil {
			t.Fatalf("failed to decode saved png: %v", err)
		}
		if decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != height {
			t.Fatalf("decoded bounds mismatch: %v", decoded.Bounds())
		}
	})

	t.Run("invalid parameters and NaN/Inf clamping", func(t *testing.T) {
		if EncodeLinearizedDepthToGrayImage(nil, width, height, 0, 0) != nil {
			t.Errorf("expected nil for nil depths")
		}
		if EncodeLinearizedDepthToGrayImage(depths, 0, height, 0, 0) != nil {
			t.Errorf("expected nil for width=0")
		}
		if EncodeLinearizedDepthToGrayImage(depths, width, 0, 0, 0) != nil {
			t.Errorf("expected nil for height=0")
		}
		if EncodeLinearizedDepthToGrayImage([]float32{1.0}, 2, 2, 0, 0) != nil {
			t.Errorf("expected nil for slice length mismatch")
		}

		nanDepths := []float32{float32(math.NaN()), -5.0, 10.0, float32(math.Inf(1))}
		img := EncodeLinearizedDepthToGrayImage(nanDepths, 2, 2, 0, 0)
		if img == nil {
			t.Fatalf("expected non-nil image for NaN/Inf values")
		}
		if img.GrayAt(0, 0).Y != 0 {
			t.Errorf("expected NaN to clamp to 0, got %d", img.GrayAt(0, 0).Y)
		}
		if img.GrayAt(1, 0).Y != 0 {
			t.Errorf("expected negative depth to clamp to 0, got %d", img.GrayAt(1, 0).Y)
		}
		if img.GrayAt(0, 1).Y != 255 {
			t.Errorf("expected >1 depth to clamp to 255, got %d", img.GrayAt(0, 1).Y)
		}
		if img.GrayAt(1, 1).Y != 255 {
			t.Errorf("expected +Inf depth to clamp to 255, got %d", img.GrayAt(1, 1).Y)
		}
	})
}

func TestEncodeOITRevealToGrayscalePNG(t *testing.T) {
	width, height := 3, 3
	data := []byte{
		0, 64, 128,
		192, 255, 32,
		96, 160, 224,
	}

	t.Run("tight packed R8Unorm", func(t *testing.T) {
		img := EncodeOITRevealToGrayImage(data, width, height, 0)
		if img == nil {
			t.Fatalf("expected non-nil reveal image")
		}
		if img.Rect.Dx() != width || img.Rect.Dy() != height {
			t.Fatalf("expected %dx%d, got %dx%d", width, height, img.Rect.Dx(), img.Rect.Dy())
		}
		if img.GrayAt(0, 0).Y != 0 {
			t.Errorf("expected (0,0)=0, got %d", img.GrayAt(0, 0).Y)
		}
		if img.GrayAt(1, 0).Y != 64 {
			t.Errorf("expected (1,0)=64, got %d", img.GrayAt(1, 0).Y)
		}
		if img.GrayAt(1, 1).Y != 255 {
			t.Errorf("expected (1,1)=255, got %d", img.GrayAt(1, 1).Y)
		}
	})

	t.Run("strided/padded R8Unorm", func(t *testing.T) {
		bytesPerRow := 256
		padded := make([]byte, bytesPerRow*height)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				padded[y*bytesPerRow+x] = data[y*width+x]
			}
		}
		img := EncodeOITRevealToGrayImage(padded, width, height, bytesPerRow)
		if img == nil {
			t.Fatalf("expected non-nil reveal image from padded buffer")
		}
		if img.GrayAt(0, 0).Y != 0 {
			t.Errorf("expected (0,0)=0, got %d", img.GrayAt(0, 0).Y)
		}
		if img.GrayAt(2, 2).Y != 224 {
			t.Errorf("expected (2,2)=224, got %d", img.GrayAt(2, 2).Y)
		}
	})

	t.Run("save and decode reveal PNG", func(t *testing.T) {
		img := EncodeOITRevealToGrayImage(data, width, height, 0)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("png encode error: %v", err)
		}
		decoded, err := png.Decode(&buf)
		if err != nil {
			t.Fatalf("png decode error: %v", err)
		}
		if decoded.Bounds().Dx() != width || decoded.Bounds().Dy() != height {
			t.Fatalf("decoded dimensions mismatch: %v", decoded.Bounds())
		}
	})

	t.Run("invalid reveal parameters", func(t *testing.T) {
		if EncodeOITRevealToGrayImage(nil, width, height, 0) != nil {
			t.Errorf("expected nil for nil data")
		}
		if EncodeOITRevealToGrayImage(data, 0, height, 0) != nil {
			t.Errorf("expected nil for width=0")
		}
		if EncodeOITRevealToGrayImage(data, width, 0, 0) != nil {
			t.Errorf("expected nil for height=0")
		}
	})
}

func TestEncodeHDRColorToRGBAPNG(t *testing.T) {
	t.Run("RGBA8 standard buffer", func(t *testing.T) {
		width, height := 2, 2
		rgba := []byte{
			255, 0, 0, 255,
			0, 255, 0, 255,
			0, 0, 255, 255,
			255, 255, 0, 128,
		}
		img := EncodeRGBA8ToNRGBA(rgba, width, height, 0, false)
		if img == nil {
			t.Fatalf("expected non-nil NRGBA image")
		}
		if img.NRGBAAt(0, 0) != (color.NRGBA{R: 255, G: 0, B: 0, A: 255}) {
			t.Errorf("pixel (0,0) mismatch: got %+v", img.NRGBAAt(0, 0))
		}
		if img.NRGBAAt(1, 0) != (color.NRGBA{R: 0, G: 255, B: 0, A: 255}) {
			t.Errorf("pixel (1,0) mismatch: got %+v", img.NRGBAAt(1, 0))
		}
		if img.NRGBAAt(0, 1) != (color.NRGBA{R: 0, G: 0, B: 255, A: 255}) {
			t.Errorf("pixel (0,1) mismatch: got %+v", img.NRGBAAt(0, 1))
		}
		if img.NRGBAAt(1, 1) != (color.NRGBA{R: 255, G: 255, B: 0, A: 128}) {
			t.Errorf("pixel (1,1) mismatch: got %+v", img.NRGBAAt(1, 1))
		}
	})

	t.Run("BGRA8 standard buffer", func(t *testing.T) {
		width, height := 1, 1
		bgra := []byte{0, 128, 255, 255} // B=0, G=128, R=255
		img := EncodeRGBA8ToNRGBA(bgra, width, height, 0, true)
		if img == nil {
			t.Fatalf("expected non-nil NRGBA image")
		}
		expected := color.NRGBA{R: 255, G: 128, B: 0, A: 255}
		if img.NRGBAAt(0, 0) != expected {
			t.Errorf("expected %+v, got %+v", expected, img.NRGBAAt(0, 0))
		}
	})

	t.Run("RGBA16Float half float buffer", func(t *testing.T) {
		// IEEE 754 half float constants:
		// 1.0 = 0x3C00 (little endian: 0x00, 0x3C)
		// 0.5 = 0x3800 (little endian: 0x00, 0x38)
		// 0.0 = 0x0000 (little endian: 0x00, 0x00)
		width, height := 2, 1
		halfData := []byte{
			// Pixel 0: R=1.0, G=0.0, B=0.0, A=1.0
			0x00, 0x3C, 0x00, 0x00, 0x00, 0x00, 0x00, 0x3C,
			// Pixel 1: R=0.5, G=1.0, B=0.5, A=1.0
			0x00, 0x38, 0x00, 0x3C, 0x00, 0x38, 0x00, 0x3C,
		}
		img := EncodeRGBA16FloatToNRGBA(halfData, width, height, 0)
		if img == nil {
			t.Fatalf("expected non-nil NRGBA image from RGBA16F")
		}
		p0 := img.NRGBAAt(0, 0)
		if p0.R != 255 || p0.G != 0 || p0.B != 0 || p0.A != 255 {
			t.Errorf("pixel 0 mismatch: got %+v", p0)
		}
		p1 := img.NRGBAAt(1, 0)
		if p1.R < 126 || p1.R > 129 || p1.G != 255 || p1.B < 126 || p1.B > 129 || p1.A != 255 {
			t.Errorf("pixel 1 mismatch: got %+v", p1)
		}
	})

	t.Run("save and decode HDR NRGBA PNG", func(t *testing.T) {
		rgba := []byte{255, 128, 64, 255}
		img := EncodeRGBA8ToNRGBA(rgba, 1, 1, 0, false)
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("png encode error: %v", err)
		}
		decoded, err := png.Decode(&buf)
		if err != nil {
			t.Fatalf("png decode error: %v", err)
		}
		if decoded.Bounds().Dx() != 1 || decoded.Bounds().Dy() != 1 {
			t.Fatalf("decoded dimensions mismatch: %v", decoded.Bounds())
		}
	})
}

func TestPassIsolateCVar(t *testing.T) {
	modes := []struct {
		mode PassIsolateMode
		name string
		val  int
	}{
		{PassIsolateNormal, "normal", 0},
		{PassIsolateAccum, "accum", 1},
		{PassIsolateReveal, "reveal", 2},
		{PassIsolateDepth, "depth", 3},
		{PassIsolateOpaque, "opaque", 4},
		{PassIsolateTranslucent, "translucent", 5},
	}

	for _, tc := range modes {
		t.Run("mode_"+tc.name, func(t *testing.T) {
			if int(tc.mode) != tc.val {
				t.Errorf("mode %s value = %d, want %d", tc.name, int(tc.mode), tc.val)
			}
			if tc.mode.String() != tc.name {
				t.Errorf("mode %d string = %q, want %q", tc.val, tc.mode.String(), tc.name)
			}
			parsed, err := ParsePassIsolateMode(tc.name)
			if err != nil {
				t.Fatalf("parse by name %q failed: %v", tc.name, err)
			}
			if parsed != tc.mode {
				t.Errorf("parsed %q = %v, want %v", tc.name, parsed, tc.mode)
			}
		})
	}

	t.Run("unknown mode string", func(t *testing.T) {
		unknown := PassIsolateMode(99)
		if unknown.String() != "unknown" {
			t.Errorf("expected 'unknown', got %q", unknown.String())
		}
		_, err := ParsePassIsolateMode("nonexistent")
		if err == nil {
			t.Errorf("expected error for nonexistent mode string")
		}
	})

	t.Run("set and get pass isolate mode", func(t *testing.T) {
		SetPassIsolateMode(PassIsolateDepth)
		if GetPassIsolateMode() != PassIsolateDepth {
			t.Errorf("expected PassIsolateDepth, got %v", GetPassIsolateMode())
		}
		SetPassIsolateMode(PassIsolateNormal)
		if GetPassIsolateMode() != PassIsolateNormal {
			t.Errorf("expected PassIsolateNormal, got %v", GetPassIsolateMode())
		}
	})
}
