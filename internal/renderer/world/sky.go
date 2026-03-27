package world

import (
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

func ReadBoolCvar(name string) bool {
	return cvar.BoolValue(name)
}

func ReadSkyLayerSpeedCvar(name string, fallback float32) float32 {
	cv := cvar.Get(name)
	if cv == nil {
		return fallback
	}
	speed := cv.Float32()
	if speed < 0 {
		return 0
	}
	return speed
}

func ProceduralSkyGradientColors() (horizon, zenith [3]float32) {
	return [3]float32{0.40, 0.53, 0.78}, [3]float32{0.07, 0.10, 0.23}
}

func ShouldUseProceduralSky(fastSky, proceduralSkyEnabled, externalSkyEmbedded bool) bool {
	return fastSky && proceduralSkyEnabled && externalSkyEmbedded
}

func ResolveSkyFogMix(cvarValue float32, overrideHasValue bool, overrideValue float32, fogDensity float32) float32 {
	if fogDensity <= 0 {
		return 0
	}
	skyFog := clamp01(cvarValue)
	if overrideHasValue {
		skyFog = clamp01(overrideValue)
	}
	return skyFog
}

func BuildSkyFlatRGBA(alphaLayer []byte) [4]byte {
	var out [4]byte
	if len(alphaLayer) < 4 {
		out[3] = 255
		return out
	}
	var (
		sumR uint64
		sumG uint64
		sumB uint64
		n    uint64
	)
	for i := 0; i+3 < len(alphaLayer); i += 4 {
		if alphaLayer[i+3] == 0 {
			continue
		}
		sumR += uint64(alphaLayer[i+0])
		sumG += uint64(alphaLayer[i+1])
		sumB += uint64(alphaLayer[i+2])
		n++
	}
	if n == 0 {
		out[3] = 255
		return out
	}
	out[0] = byte(sumR / n)
	out[1] = byte(sumG / n)
	out[2] = byte(sumB / n)
	out[3] = 255
	return out
}

func ShouldSplitAsQuake64Sky(treeVersion int32, width, height int) bool {
	return bsp.IsQuake64(treeVersion) || (width == 32 && height == 64)
}

func ExtractEmbeddedSkyLayers(pixels []byte, width, height int, palette []byte, quake64 bool) (solidRGBA, alphaRGBA []byte, layerWidth, layerHeight int, ok bool) {
	if width <= 0 || height <= 0 || len(pixels) < width*height {
		return nil, nil, 0, 0, false
	}
	if quake64 {
		if height < 2 {
			return nil, nil, 0, 0, false
		}
		layerWidth = width
		layerHeight = height / 2
		if layerHeight <= 0 {
			return nil, nil, 0, 0, false
		}
		layerSize := layerWidth * layerHeight
		front := pixels[:layerSize]
		back := pixels[layerSize : layerSize*2]
		solidRGBA = indexedOpaqueToRGBA(back, palette)
		alphaRGBA = make([]byte, layerSize*4)
		for i, p := range front {
			r, g, b := paletteColor(p, palette)
			alphaRGBA[i*4] = r
			alphaRGBA[i*4+1] = g
			alphaRGBA[i*4+2] = b
			alphaRGBA[i*4+3] = 128
		}
		return solidRGBA, alphaRGBA, layerWidth, layerHeight, true
	}
	if width < 2 {
		return nil, nil, 0, 0, false
	}
	layerWidth = width / 2
	layerHeight = height
	if layerWidth <= 0 {
		return nil, nil, 0, 0, false
	}
	layerSize := layerWidth * layerHeight
	backIndexed := make([]byte, layerSize)
	frontIndexed := make([]byte, layerSize)
	for y := 0; y < height; y++ {
		row := y * width
		copy(backIndexed[y*layerWidth:(y+1)*layerWidth], pixels[row+layerWidth:row+width])
		copy(frontIndexed[y*layerWidth:(y+1)*layerWidth], pixels[row:row+layerWidth])
	}
	solidRGBA = indexedOpaqueToRGBA(backIndexed, palette)
	alphaRGBA = make([]byte, layerSize*4)
	for i, p := range frontIndexed {
		if p == 0 || p == 255 {
			r, g, b := paletteColor(255, palette)
			alphaRGBA[i*4] = r
			alphaRGBA[i*4+1] = g
			alphaRGBA[i*4+2] = b
			alphaRGBA[i*4+3] = 0
			continue
		}
		r, g, b := paletteColor(p, palette)
		alphaRGBA[i*4] = r
		alphaRGBA[i*4+1] = g
		alphaRGBA[i*4+2] = b
		alphaRGBA[i*4+3] = 255
	}
	return solidRGBA, alphaRGBA, layerWidth, layerHeight, true
}

func indexedOpaqueToRGBA(pixels []byte, palette []byte) []byte {
	rgba := make([]byte, len(pixels)*4)
	for i, p := range pixels {
		r, g, b := paletteColor(p, palette)
		rgba[i*4] = r
		rgba[i*4+1] = g
		rgba[i*4+2] = b
		rgba[i*4+3] = 255
	}
	return rgba
}

func paletteColor(index byte, palette []byte) (r, g, b byte) {
	if len(palette) < 768 {
		return index, index, index
	}
	offset := int(index) * 3
	if offset >= len(palette)-2 {
		return index, index, index
	}
	return palette[offset], palette[offset+1], palette[offset+2]
}
