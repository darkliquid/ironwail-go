package renderer

import surfaceimpl "github.com/darkliquid/ironwail-go/internal/renderer/surface"

const (
	MaxSanityLightmaps = surfaceimpl.MaxSanityLightmaps
)

var (
	ErrBrokenTextureAnimationCycle = surfaceimpl.ErrBrokenTextureAnimationCycle
	ErrInfiniteTextureAnimation    = surfaceimpl.ErrInfiniteTextureAnimation
)

type SurfaceTexture = surfaceimpl.SurfaceTexture
type Chart = surfaceimpl.Chart
type Lightmap = surfaceimpl.Lightmap
type LightmapAllocator = surfaceimpl.LightmapAllocator
type SurfaceLightmapInput = surfaceimpl.SurfaceLightmapInput

func BuildTextureAnimations(names []string) ([]*SurfaceTexture, error) {
	return surfaceimpl.BuildTextureAnimations(names)
}

func TextureAnimation(base *SurfaceTexture, frame int, timeSeconds float64) (*SurfaceTexture, error) {
	return surfaceimpl.TextureAnimation(base, frame, timeSeconds)
}

func NewLightmapAllocator(blockWidth, blockHeight int, reserveFirstTexel bool) (*LightmapAllocator, error) {
	return surfaceimpl.NewLightmapAllocator(blockWidth, blockHeight, reserveFirstTexel)
}

func NumLightmapTaps(styles [4]byte) int {
	return surfaceimpl.NumLightmapTaps(styles)
}

func FillSurfaceLightmap(in SurfaceLightmapInput, lightmap Lightmap, lightmapWidth int, dst []uint32) error {
	return surfaceimpl.FillSurfaceLightmap(in, lightmap, lightmapWidth, dst)
}
