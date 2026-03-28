package sky

import (
	"bytes"
	stdimage "image"
	"image/draw"
	_ "image/jpeg"
	"path"
	"path/filepath"
	"strings"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

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
	ext := strings.ToLower(filepath.Ext(path))
	var (
		img stdimage.Image
		err error
	)
	switch ext {
	case ".tga":
		img, err = qimage.DecodeTGA(data)
	default:
		img, _, err = stdimage.Decode(bytes.NewReader(data))
	}
	if err != nil || img == nil {
		return nil, 0, 0, false
	}
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return nil, 0, 0, false
	}
	rgbaImg := stdimage.NewRGBA(stdimage.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(rgbaImg, rgbaImg.Bounds(), img, bounds.Min, draw.Src)
	return append([]byte(nil), rgbaImg.Pix...), bounds.Dx(), bounds.Dy(), true
}

// LoadExternalSkyboxFaces loads and validates all six sky faces before creating cubemap or layered sky resources.
func LoadExternalSkyboxFaces(baseName string, loadFile func(string) ([]byte, error)) (faces [6]ExternalSkyboxFace, loaded int) {
	if baseName == "" || loadFile == nil {
		return faces, 0
	}
	for i, suffix := range SkyboxFaceSuffixes {
		paths := SkyboxFaceSearchPaths(baseName, suffix)
		for _, candidate := range paths {
			data, err := LoadSkyboxFileCandidate(candidate, loadFile)
			if err != nil || len(data) == 0 {
				continue
			}
			rgba, width, height, ok := DecodeSkyboxImage(candidate, data)
			if !ok {
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
			break
		}
	}
	return faces, loaded
}

// LoadSkyboxFileCandidate tries one specific sky face file candidate and reports whether decoding succeeded.
func LoadSkyboxFileCandidate(candidate string, loadFile func(string) ([]byte, error)) ([]byte, error) {
	data, err := loadFile(candidate)
	if err == nil && len(data) > 0 {
		return data, nil
	}
	lowerCandidate := strings.ToLower(candidate)
	if lowerCandidate == candidate {
		return data, err
	}
	lowerData, lowerErr := loadFile(lowerCandidate)
	if lowerErr == nil && len(lowerData) > 0 {
		return lowerData, nil
	}
	return data, err
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
