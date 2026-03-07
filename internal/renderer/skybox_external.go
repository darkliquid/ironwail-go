package renderer

import (
	"bytes"
	stdimage "image"
	"image/draw"
	_ "image/jpeg"
	"path"
	"path/filepath"
	"strings"

	qimage "github.com/ironwail/ironwail-go/internal/image"
)

var (
	skyboxFaceSuffixes     = [...]string{"rt", "bk", "lf", "ft", "up", "dn"}
	skyboxFaceExts         = [...]string{"png", "tga", "jpg"}
	skyboxCubemapFaceOrder = [...]int{
		3, // ft -> +X
		1, // bk -> -X
		4, // up -> +Y
		5, // dn -> -Y
		0, // rt -> +Z
		2, // lf -> -Z
	}
)

type externalSkyboxFace struct {
	Suffix string
	Path   string
	Width  int
	Height int
	RGBA   []byte
}

func normalizeSkyboxBaseName(name string) string {
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

func skyboxFaceSearchPaths(baseName, suffix string) []string {
	paths := make([]string, 0, len(skyboxFaceExts))
	for _, ext := range skyboxFaceExts {
		paths = append(paths, "gfx/env/"+baseName+suffix+"."+ext)
	}
	return paths
}

func decodeSkyboxImage(path string, data []byte) (rgba []byte, width, height int, ok bool) {
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

func loadExternalSkyboxFaces(baseName string, loadFile func(string) ([]byte, error)) (faces [6]externalSkyboxFace, loaded int) {
	if baseName == "" || loadFile == nil {
		return faces, 0
	}
	for i, suffix := range skyboxFaceSuffixes {
		paths := skyboxFaceSearchPaths(baseName, suffix)
		for _, candidate := range paths {
			data, err := loadFile(candidate)
			if err != nil || len(data) == 0 {
				continue
			}
			rgba, width, height, ok := decodeSkyboxImage(candidate, data)
			if !ok {
				continue
			}
			faces[i] = externalSkyboxFace{
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

func externalSkyboxCubemapEligible(faces [6]externalSkyboxFace, loaded int) bool {
	_, ok := externalSkyboxCubemapFaceSize(faces, loaded)
	return ok
}

func externalSkyboxCubemapFaceSize(faces [6]externalSkyboxFace, loaded int) (int, bool) {
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
