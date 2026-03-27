//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	"strings"

	"github.com/go-gl/gl/v4.6-core/gl"
)

type TextureUploadOptions struct {
	MinFilter  int32
	MagFilter  int32
	LodBias    float32
	Anisotropy float32
}

func ParseTextureMode(mode string) (minFilter, magFilter int32) {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "GL_NEAREST":
		return gl.NEAREST, gl.NEAREST
	case "GL_LINEAR":
		return gl.LINEAR, gl.LINEAR
	case "GL_NEAREST_MIPMAP_NEAREST":
		return gl.NEAREST_MIPMAP_NEAREST, gl.NEAREST
	case "GL_NEAREST_MIPMAP_LINEAR":
		return gl.NEAREST_MIPMAP_LINEAR, gl.NEAREST
	case "GL_LINEAR_MIPMAP_NEAREST":
		return gl.LINEAR_MIPMAP_NEAREST, gl.LINEAR
	case "GL_LINEAR_MIPMAP_LINEAR":
		return gl.LINEAR_MIPMAP_LINEAR, gl.LINEAR
	default:
		return gl.NEAREST, gl.NEAREST
	}
}

func UploadTextureRGBA(width, height int, rgba []byte, options TextureUploadOptions) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, options.MinFilter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, options.MagFilter)
	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_LOD_BIAS, options.LodBias)
	anisotropy := options.Anisotropy
	if anisotropy < 1 {
		anisotropy = 1
	}
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAX_ANISOTROPY, anisotropy)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	return tex
}
