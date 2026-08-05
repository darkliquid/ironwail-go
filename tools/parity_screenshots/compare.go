package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

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
	defer func() { _ = f.Close() }()

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
	defer func() { _ = f.Close() }()
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
	defer func() { _ = out.Close() }()
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
				diffImage.SetNRGBA(x, y, color.NRGBA{R: 255, G: clampUint8(int(dg) / 2), B: clampUint8(int(db) / 2), A: 255})
			}
		}
	}

	regions := findDiffRegions(mismatchMask, width, height)
	annotateDiffRegions(diffImage, regions)
	meanMismatchPerceptualDelta := 0.0
	if mismatchPixels > 0 {
		meanMismatchPerceptualDelta = mismatchPerceptualDelta / float64(mismatchPixels)
	}

	meanSSIM, minSSIM := computeSSIM(refImg, gotImg, refBounds.Min, gotBounds.Min, width, height)

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
		MeanSSIM:                    meanSSIM,
		MinSSIM:                     minSSIM,
		Regions:                     regions,
	}, diffImage, overlayImage, nil
}

func computeSSIM(refImg, gotImg image.Image, refMin, gotMin image.Point, width, height int) (mean, min float64) {
	const (
		k1    = 0.01
		k2    = 0.03
		l     = 255.0
		c1    = (k1 * l) * (k1 * l)
		c2    = (k2 * l) * (k2 * l)
		block = 8
	)
	min = 1.0
	blocks := 0
	var sum float64
	for by := 0; by+block <= height; by += block {
		for bx := 0; bx+block <= width; bx += block {
			var sumRef, sumGot float64
			for y := by; y < by+block; y++ {
				for x := bx; x < bx+block; x++ {
					ref := color.NRGBAModel.Convert(refImg.At(refMin.X+x, refMin.Y+y)).(color.NRGBA)
					got := color.NRGBAModel.Convert(gotImg.At(gotMin.X+x, gotMin.Y+y)).(color.NRGBA)
					sumRef += luminance(ref)
					sumGot += luminance(got)
				}
			}
			meanRef := sumRef / (block * block)
			meanGot := sumGot / (block * block)
			var varRef, varGot, cov float64
			for y := by; y < by+block; y++ {
				for x := bx; x < bx+block; x++ {
					ref := color.NRGBAModel.Convert(refImg.At(refMin.X+x, refMin.Y+y)).(color.NRGBA)
					got := color.NRGBAModel.Convert(gotImg.At(gotMin.X+x, gotMin.Y+y)).(color.NRGBA)
					r := luminance(ref) - meanRef
					g := luminance(got) - meanGot
					varRef += r * r
					varGot += g * g
					cov += r * g
				}
			}
			n := float64(block * block)
			varRef /= n
			varGot /= n
			cov /= n
			ssim := ((2*meanRef*meanGot + c1) * (2*cov + c2)) / ((meanRef*meanRef + meanGot*meanGot + c1) * (varRef + varGot + c2))
			if ssim < 0 {
				ssim = 0
			}
			if ssim > 1 {
				ssim = 1
			}
			if ssim < min {
				min = ssim
			}
			sum += ssim
			blocks++
		}
	}
	if blocks == 0 {
		return 0, 0
	}
	return sum / float64(blocks), min
}

func luminance(c color.NRGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
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
	defer func() { _ = f.Close() }()
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

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func compareFailed(summary compareSummary) bool {
	return summary.ReferenceCount == 0 || summary.DiffCount > 0 || summary.MissingCount > 0 || summary.GoCount != summary.ReferenceCount
}

func resultForViewpoint(vp viewpoint, status string, m comparisonMetrics) viewpointResult {
	return viewpointResult{
		ID:              vp.ID,
		Map:             vp.Map,
		Description:     vp.Description,
		Status:          status,
		MismatchPercent: m.MismatchPercent,
		MeanSSIM:        m.MeanSSIM,
		MinSSIM:         m.MinSSIM,
		MeanChannel:     m.MeanChannelDelta,
		MaxChannel:      m.MaxChannelDelta,
		Perceptual:      m.MeanPerceptualDelta,
		Regions:         len(m.Regions),
		Metrics:         m,
	}
}

func writeParityReport(projectDir string, results []viewpointResult, summary compareSummary) {
	resultsPath := filepath.Join(projectDir, "testdata", "parity", "results.json")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"summary": summary,
		"results": results,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: encode report: %v\n", err)
		return
	}
	if err := os.WriteFile(resultsPath, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write report %s: %v\n", resultsPath, err)
		return
	}
	fmt.Printf("JSON report: %s\n", resultsPath)

	fmt.Println()
	fmt.Println("## Parity Report")
	fmt.Println()
	fmt.Println("| Viewpoint | Map | Status | Mismatch % | SSIM | Max Δ | Regions |")
	fmt.Println("|---|---|---|---|---|---|---|")
	for _, r := range results {
		ssim := "-"
		mismatch := "-"
		maxDelta := "-"
		regions := "-"
		if r.Status == "ok" || r.Status == "diff" {
			mismatch = fmt.Sprintf("%.4f", r.MismatchPercent)
			ssim = fmt.Sprintf("%.4f", r.MeanSSIM)
			maxDelta = fmt.Sprintf("%d", r.MaxChannel)
			regions = fmt.Sprintf("%d", r.Regions)
		}
		fmt.Printf("| %s | %s | %s | %s | %s | %s | %s |\n", r.ID, r.Map, r.Status, mismatch, ssim, maxDelta, regions)
	}
	fmt.Println()
	fmt.Printf("Totals: %d reference, %d go, %d match, %d diff, %d missing\n",
		summary.ReferenceCount, summary.GoCount, summary.MatchCount, summary.DiffCount, summary.MissingCount)
}

func mustMkdir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		die("mkdir %s: %v", path, err)
	}
}
