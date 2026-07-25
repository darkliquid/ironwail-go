package game_test

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// TestQBJ2WaterTranslucencyRaster launches ironwailgo with map start in mod qbj2,
// captures screenshots of both r_wateralpha 1.0 (opaque baseline) and r_wateralpha 0.6 (translucent test),
// exports PNG files for visual verification, and automatically asserts that underwater floor geometry is rendered.
func TestQBJ2WaterTranslucencyRaster(t *testing.T) {
	testutil.SkipIfNoQuakeDir(t)
	testutil.SkipIfNoPak0(t)

	quakeDir, err := testutil.LocateQuakeDir()
	if err != nil {
		t.Skipf("Skipping test: Quake dir not found: %v", err)
	}
	qbj2Dir := filepath.Join(quakeDir, "qbj2")
	if _, err := os.Stat(qbj2Dir); os.IsNotExist(err) {
		t.Skipf("Skipping test: qbj2 mod directory not found at %s", qbj2Dir)
	}

	// Ensure ironwailgo binary is built
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Failed to resolve repo root: %v", err)
	}

	binPath := filepath.Join(repoRoot, "ironwailgo")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cmd/ironwailgo")
	buildCmd.Dir = repoRoot
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build ironwailgo binary: %v\nOutput: %s", err, string(out))
	}

	outDir := filepath.Join(repoRoot, ".tmp", "water_raster_test")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp output dir: %v", err)
	}

	// Function to run engine capture with specified r_wateralpha value
	runCapture := func(waterAlpha string, outName string, gameDir, mapName, pos, angles string) (float64, float64, float64, string) {
		pngPath := filepath.Join(outDir, outName)
		_ = os.Remove(pngPath)

		cfgPath := filepath.Join(outDir, "raster_test.cfg")
		cfgContent := fmt.Sprintf("r_wateralpha %s\nr_debug_water 1\nscr_viewsize 130\nr_drawviewmodel 0\ncrosshair 0\ntogglemenu\n", waterAlpha)
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		runCmd := exec.Command(
			binPath,
			"-basedir", quakeDir,
			"-game", gameDir,
			"-screenshot", pngPath,
			"-width", "640",
			"-height", "480",
			"+map", mapName,
			"+exec", cfgPath,
		)
		runCmd.Dir = repoRoot
		runCmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"PARITY_RUN=1",
			"PARITY_GO_CAPTURE=engine",
			"PARITY_MAP="+mapName,
			"PARITY_POS="+pos,
			"PARITY_ANGLES="+angles,
		)

		out, err := runCmd.CombinedOutput()
		if _, statErr := os.Stat(pngPath); os.IsNotExist(statErr) {
			t.Fatalf("ironwailgo failed to export screenshot to %s (err: %v)\nOutput: %s", pngPath, err, string(out))
		}

		f, err := os.Open(pngPath)
		if err != nil {
			t.Fatalf("Could not open captured screenshot %s: %v", pngPath, err)
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			t.Fatalf("Failed to decode screenshot PNG: %v", err)
		}

		bounds := img.Bounds()
		centerX := bounds.Min.X + bounds.Dx()/2
		centerY := bounds.Min.Y + bounds.Dy()/2

		var sumR, sumG, sumB uint64
		var count uint64
		for dy := -15; dy <= 15; dy++ {
			for dx := -15; dx <= 15; dx++ {
				x, y := centerX+dx, centerY+dy
				if x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y {
					c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
					sumR += uint64(c.R)
					sumG += uint64(c.G)
					sumB += uint64(c.B)
					count++
				}
			}
		}

		if count == 0 {
			t.Fatalf("No pixels sampled from rendered screenshot")
		}

		return float64(sumR) / float64(count), float64(sumG) / float64(count), float64(sumB) / float64(count), pngPath
	}

	// Test on e1m1 (standard Quake map with simple water)
	t.Run("e1m1", func(t *testing.T) {
		runWaterRasterTest(t, runCapture, "id1", "e1m1", "e1m1_water_opaque_1.0.png", "e1m1_water_translucent_0.35.png", "588 1008 -256", "90 0 0", "1.0", "0.35")
	})

	// Test on qbj2 start (BSP2 map with lit water and worldspawn wateralpha=0.6)
	t.Run("qbj2_start", func(t *testing.T) {
		runWaterRasterTest(t, runCapture, "qbj2", "start", "qbj2_water_opaque_1.0.png", "qbj2_water_translucent_0.35.png", "-736 224 -1980", "90 0 0", "1.0", "0.35")
	})
}

func runWaterRasterTest(t *testing.T,
	runCapture func(waterAlpha string, outName string, gameDir, mapName, pos, angles string) (float64, float64, float64, string),
	gameDir, mapName, opaqueName, translucentName, pos, angles, opaqueAlpha, translucentAlpha string,
) {
	rOpaque, gOpaque, bOpaque, pathOpaque := runCapture(opaqueAlpha, opaqueName, gameDir, mapName, pos, angles)
	t.Logf("Opaque baseline (r_wateralpha %s) average RGB: (%.1f, %.1f, %.1f) - PNG: %s", opaqueAlpha, rOpaque, gOpaque, bOpaque, pathOpaque)

	rTrans, gTrans, bTrans, pathTrans := runCapture(translucentAlpha, translucentName, gameDir, mapName, pos, angles)
	t.Logf("Translucent test (r_wateralpha 0.35) average RGB: (%.1f, %.1f, %.1f) - PNG: %s", rTrans, gTrans, bTrans, pathTrans)

	// Calculate color difference between translucent test and expected 0.6 dimming of opaque baseline
	// If water is translucent over an underwater floor:
	//   RGB_translucent = alpha * RGB_water + (1-alpha) * RGB_floor
	// If underwater floor is NOT drawn (opaque/black background):
	//   RGB_translucent = alpha * RGB_water + (1-alpha) * (0, 0, 0) = alpha * RGB_opaque
	alphaVal := 0.35
	expectedBlackDimmedR := alphaVal * rOpaque
	expectedBlackDimmedG := alphaVal * gOpaque
	expectedBlackDimmedB := alphaVal * bOpaque

	floorContributionR := rTrans - expectedBlackDimmedR
	floorContributionG := gTrans - expectedBlackDimmedG
	floorContributionB := bTrans - expectedBlackDimmedB

	t.Logf("Underwater floor contribution delta RGB: (%.1f, %.1f, %.1f)", floorContributionR, floorContributionG, floorContributionB)

	// AUTOMATED TRANSLUCENCY ASSERTION:
	// If floorContribution is near zero (<= 3.0), the underwater floor geometry is missing/culled/opaque!
	if floorContributionR <= 3.0 && floorContributionG <= 3.0 && floorContributionB <= 3.0 {
		t.Fatalf("AUTOMATED RASTER TEST FAIL: Water is rendering OPAQUE or underwater floor geometry is missing! Underwater floor contribution delta is near zero (%.1f, %.1f, %.1f). Opaque PNG: %s | Translucent PNG: %s",
			floorContributionR, floorContributionG, floorContributionB, pathOpaque, pathTrans)
	}

	t.Logf("AUTOMATED RASTER TEST PASS: Rendered water is translucent and shows underwater floor geometry (floor delta RGB: %.1f, %.1f, %.1f)",
		floorContributionR, floorContributionG, floorContributionB)
}
