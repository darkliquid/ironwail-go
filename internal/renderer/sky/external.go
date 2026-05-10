package sky

import (
	"bytes"
	"fmt"
	stdimage "image"
	"image/draw"
	_ "image/jpeg"
	"log/slog"
	"math"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

const skyboxLogSubsystem = "renderer.skybox"

var (
	SkyboxFaceSuffixes     = [...]string{"rt", "bk", "lf", "ft", "up", "dn"}
	SkyboxFaceExts         = [...]string{"png", "tga", "jpg"}
	SkyboxCubemapFaceOrder = [...]int{
		3, // ft -> +X
		1, // bk -> -X
		4, // up -> +Y
		5, // dn -> -Y
		0, // rt -> +Z
		2, // lf -> -Z
	}
)

type ExternalSkyboxFace struct {
	Suffix string
	Path   string
	Width  int
	Height int
	RGBA   []byte
}

type ExternalSkyboxWind struct {
	Dist   float32
	Yaw    float32
	Pitch  float32
	Period float32
}

type ExternalSkyboxRenderMode uint8

const (
	ExternalSkyboxRenderEmbedded ExternalSkyboxRenderMode = iota
	ExternalSkyboxRenderCubemap
	ExternalSkyboxRenderFaces
)

// SelectExternalSkyboxRenderMode chooses between classic scrolling sky and external skybox paths based on assets and user settings.
func SelectExternalSkyboxRenderMode(loaded int, cubemapEligible bool) ExternalSkyboxRenderMode {
	if loaded <= 0 {
		return ExternalSkyboxRenderEmbedded
	}
	if cubemapEligible {
		return ExternalSkyboxRenderCubemap
	}
	return ExternalSkyboxRenderFaces
}

// NormalizeSkyboxBaseName canonicalizes skybox names so pack files and loose files resolve identically across platforms.
func NormalizeSkyboxBaseName(name string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	trimmed = strings.TrimLeft(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	trimmedLower := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(trimmedLower, "gfx/env/"):
		trimmed = trimmed[8:]
	case strings.HasPrefix(trimmedLower, "env/"):
		trimmed = trimmed[4:]
	}
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	return path.Base(trimmed)
}

// SkyboxFaceSearchPaths enumerates candidate file paths for six cubemap faces and multiple supported image extensions.
func SkyboxFaceSearchPaths(baseName, suffix string) []string {
	paths := make([]string, 0, len(SkyboxFaceExts))
	for _, ext := range SkyboxFaceExts {
		paths = append(paths, "gfx/env/"+baseName+suffix+"."+ext)
	}
	return paths
}

// DecodeSkyboxImage decodes one skybox face image into GPU-ready pixels while validating dimensions/format.
func DecodeSkyboxImage(path string, data []byte) (rgba []byte, width, height int, ok bool) {
	rgba, width, height, err := decodeSkyboxImage(path, data)
	return rgba, width, height, err == nil
}

func decodeSkyboxImage(path string, data []byte) (rgba []byte, width, height int, err error) {
	ext := strings.ToLower(filepath.Ext(path))
	var (
		img stdimage.Image
	)
	switch ext {
	case ".tga":
		img, err = qimage.DecodeTGA(data)
	default:
		img, _, err = stdimage.Decode(bytes.NewReader(data))
	}
	if err != nil || img == nil {
		if err == nil {
			err = fmt.Errorf("decoder returned nil image")
		}
		return nil, 0, 0, fmt.Errorf("decode %s: %w", path, err)
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, 0, 0, fmt.Errorf("invalid decoded dimensions %dx%d", bounds.Dx(), bounds.Dy())
	}
	rgbaImg := stdimage.NewRGBA(stdimage.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgbaImg, rgbaImg.Bounds(), img, bounds.Min, draw.Src)
	return append([]byte(nil), rgbaImg.Pix...), bounds.Dx(), bounds.Dy(), nil
}

// LoadExternalSkyboxFaces loads and validates all six sky faces before creating cubemap or layered sky resources.
func LoadExternalSkyboxFaces(baseName string, loadFile func(string) ([]byte, error)) (faces [6]ExternalSkyboxFace, loaded int) {
	if baseName == "" || loadFile == nil {
		return faces, 0
	}
	start := time.Now()
	slog.Info("external skybox load begin", "subsystem", skyboxLogSubsystem, "name", baseName)
	for i, suffix := range SkyboxFaceSuffixes {
		paths := SkyboxFaceSearchPaths(baseName, suffix)
		slog.Info("external skybox face search begin", "subsystem", skyboxLogSubsystem, "name", baseName, "face", suffix, "candidates", strings.Join(paths, ","))
		for _, candidate := range paths {
			candidateStart := time.Now()
			result := loadSkyboxFileCandidateDetailed(candidate, loadFile)
			if result.err != nil || len(result.data) == 0 {
				errText := ""
				if result.err != nil {
					errText = result.err.Error()
				}
				slog.Info("external skybox candidate unavailable", "subsystem", skyboxLogSubsystem, "name", baseName, "face", suffix, "candidate", candidate, "resolved_path", result.path, "bytes", len(result.data), "error", errText, "elapsed_ms", elapsedMilliseconds(candidateStart))
				continue
			}
			slog.Info("external skybox candidate loaded", "subsystem", skyboxLogSubsystem, "name", baseName, "face", suffix, "candidate", candidate, "resolved_path", result.path, "lowercase_fallback", result.lowercaseFallback, "bytes", len(result.data), "elapsed_ms", elapsedMilliseconds(candidateStart))

			decodeStart := time.Now()
			slog.Info("external skybox decode begin", "subsystem", skyboxLogSubsystem, "name", baseName, "face", suffix, "path", result.path, "bytes", len(result.data))
			rgba, width, height, err := decodeSkyboxImage(result.path, result.data)
			if err != nil {
				slog.Warn("external skybox decode failed", "subsystem", skyboxLogSubsystem, "name", baseName, "face", suffix, "path", result.path, "bytes", len(result.data), "error", err, "elapsed_ms", elapsedMilliseconds(decodeStart))
				continue
			}
			faces[i] = ExternalSkyboxFace{
				Suffix: suffix,
				Path:   candidate,
				Width:  width,
				Height: height,
				RGBA:   rgba,
			}
			loaded++
			slog.Info("external skybox face decoded", "subsystem", skyboxLogSubsystem, "name", baseName, "face", suffix, "path", result.path, "width", width, "height", height, "rgba_bytes", len(rgba), "elapsed_ms", elapsedMilliseconds(decodeStart))
			break
		}
		if faces[i].Path == "" {
			slog.Warn("external skybox face missing", "subsystem", skyboxLogSubsystem, "name", baseName, "face", suffix, "candidates", strings.Join(paths, ","))
		}
	}
	slog.Info("external skybox load complete", "subsystem", skyboxLogSubsystem, "name", baseName, "loaded_faces", loaded, "elapsed_ms", elapsedMilliseconds(start))
	return faces, loaded
}

// LoadExternalSkyboxWind loads C Ironwail-compatible skywind settings from
// gfx/env/<skyname>wind.cfg. The base sky name commonly ends with an
// underscore, so qbj3's mak_cloudysky4_ resolves to mak_cloudysky4_wind.cfg.
func LoadExternalSkyboxWind(baseName string, loadFile func(string) ([]byte, error)) (ExternalSkyboxWind, bool) {
	if baseName == "" || loadFile == nil {
		return ExternalSkyboxWind{}, false
	}
	candidate := "gfx/env/" + baseName + "wind.cfg"
	start := time.Now()
	result := loadSkyboxFileCandidateDetailed(candidate, loadFile)
	if result.err != nil || len(result.data) == 0 {
		slog.Info("external skybox wind config unavailable", "subsystem", skyboxLogSubsystem, "name", baseName, "candidate", candidate, "resolved_path", result.path, "error", errorString(result.err), "elapsed_ms", elapsedMilliseconds(start))
		return ExternalSkyboxWind{}, false
	}
	wind, ok := ParseExternalSkyboxWind(result.data)
	if !ok {
		slog.Warn("external skybox wind config invalid", "subsystem", skyboxLogSubsystem, "name", baseName, "candidate", candidate, "resolved_path", result.path, "elapsed_ms", elapsedMilliseconds(start))
		return ExternalSkyboxWind{}, false
	}
	slog.Info("external skybox wind config loaded", "subsystem", skyboxLogSubsystem, "name", baseName, "candidate", candidate, "resolved_path", result.path, "dist", wind.Dist, "yaw", wind.Yaw, "period", wind.Period, "pitch", wind.Pitch, "elapsed_ms", elapsedMilliseconds(start))
	return wind, true
}

// ParseExternalSkyboxWind mirrors C Ironwail's Skywind_Load_f token format:
// "skywind <dist> <yaw> <period> <pitch>". Missing numeric tokens keep the C
// defaults established by Skywind_Clear.
func ParseExternalSkyboxWind(data []byte) (ExternalSkyboxWind, bool) {
	fields := strings.Fields(stripSkywindLineComments(string(data)))
	if len(fields) == 0 || fields[0] != "skywind" {
		return ExternalSkyboxWind{}, false
	}
	wind := ExternalSkyboxWind{
		Dist:   0,
		Yaw:    45,
		Pitch:  0,
		Period: 30,
	}
	if len(fields) > 1 {
		wind.Dist = clampFloat32(parseSkywindFloat(fields[1]), -2, 2)
	}
	if len(fields) > 2 {
		wind.Yaw = float32(math.Mod(float64(parseSkywindFloat(fields[2])), 360))
	}
	if len(fields) > 3 {
		wind.Period = parseSkywindFloat(fields[3])
	}
	if len(fields) > 4 {
		wind.Pitch = float32(math.Mod(float64(parseSkywindFloat(fields[4])+90), 180)) - 90
	}
	return wind, true
}

func stripSkywindLineComments(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if comment := strings.Index(line, "//"); comment >= 0 {
			lines[i] = line[:comment]
		}
	}
	return strings.Join(lines, "\n")
}

// LoadSkyboxFileCandidate tries one specific sky face file candidate and reports whether decoding succeeded.
func LoadSkyboxFileCandidate(candidate string, loadFile func(string) ([]byte, error)) ([]byte, error) {
	result := loadSkyboxFileCandidateDetailed(candidate, loadFile)
	return result.data, result.err
}

type skyboxFileCandidateResult struct {
	data              []byte
	path              string
	lowercaseFallback bool
	err               error
}

func loadSkyboxFileCandidateDetailed(candidate string, loadFile func(string) ([]byte, error)) skyboxFileCandidateResult {
	data, err := loadFile(candidate)
	if err == nil && len(data) > 0 {
		return skyboxFileCandidateResult{data: data, path: candidate}
	}
	lowerCandidate := strings.ToLower(candidate)
	if lowerCandidate == candidate {
		return skyboxFileCandidateResult{data: data, path: candidate, err: err}
	}
	lowerData, lowerErr := loadFile(lowerCandidate)
	if lowerErr == nil && len(lowerData) > 0 {
		return skyboxFileCandidateResult{data: lowerData, path: lowerCandidate, lowercaseFallback: true}
	}
	if err != nil {
		return skyboxFileCandidateResult{data: data, path: candidate, err: err}
	}
	return skyboxFileCandidateResult{data: lowerData, path: lowerCandidate, lowercaseFallback: true, err: lowerErr}
}

func parseSkywindFloat(s string) float32 {
	value, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0
	}
	return float32(value)
}

func clampFloat32(value, minValue, maxValue float32) float32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func elapsedMilliseconds(start time.Time) float64 {
	return float64(time.Since(start)) / float64(time.Millisecond)
}

// ExternalSkyboxCubemapEligible checks whether loaded faces satisfy cubemap constraints (format/size/orientation compatibility).
func ExternalSkyboxCubemapEligible(faces [6]ExternalSkyboxFace, loaded int) bool {
	_, ok := ExternalSkyboxCubemapFaceSize(faces, loaded)
	return ok
}

// ExternalSkyboxCubemapFaceSize returns the agreed face dimension used for cubemap allocation.
func ExternalSkyboxCubemapFaceSize(faces [6]ExternalSkyboxFace, loaded int) (int, bool) {
	if loaded <= 0 {
		return 0, false
	}
	faceSize := 0
	for _, face := range faces {
		if face.Width == 0 || face.Height == 0 || len(face.RGBA) == 0 {
			continue
		}
		if face.Width <= 0 || face.Height <= 0 || face.Width != face.Height {
			return 0, false
		}
		if len(face.RGBA) != face.Width*face.Height*4 {
			return 0, false
		}
		if faceSize == 0 {
			faceSize = face.Width
			continue
		}
		if face.Width != faceSize {
			return 0, false
		}
	}
	if faceSize <= 0 {
		return 0, false
	}
	return faceSize, true
}
