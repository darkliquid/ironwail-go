package renderer

import (
	"errors"
	"fmt"
	"strings"
)

const (
	MaxSanityLightmaps = 1 << 20
)

var (
	ErrBrokenTextureAnimationCycle = errors.New("broken texture animation cycle")
	ErrInfiniteTextureAnimation    = errors.New("infinite texture animation cycle")
)

type SurfaceTexture struct {
	TextureIndex   int32
	AnimTotal      int
	AnimMin        int
	AnimMax        int
	AnimNext       *SurfaceTexture
	AlternateAnims *SurfaceTexture
}

const textureAnimationCycle = 2

// BuildTextureAnimations precomputes linked animation chains for anim textures so runtime sampling can jump directly to the proper frame.
func BuildTextureAnimations(names []string) ([]*SurfaceTexture, error) {
	textures := make([]*SurfaceTexture, len(names))
	trimmed := make([]string, len(names))
	for i, name := range names {
		trimmed[i] = strings.TrimRight(name, "\x00")
		if trimmed[i] == "" {
			continue
		}
		textures[i] = &SurfaceTexture{TextureIndex: int32(i)}
	}

	for i, texture := range textures {
		if texture == nil || texture.AnimNext != nil {
			continue
		}
		name := trimmed[i]
		if len(name) < 2 || name[0] != '+' {
			continue
		}

		var (
			anims    [10]*SurfaceTexture
			altAnims [10]*SurfaceTexture
			maxAnim  int
			altMax   int
		)

		frameIndex, alternate, err := textureAnimationFrame(name)
		if err != nil {
			return nil, err
		}
		if alternate {
			altAnims[frameIndex] = texture
			altMax = frameIndex + 1
		} else {
			anims[frameIndex] = texture
			maxAnim = frameIndex + 1
		}

		for j := i + 1; j < len(textures); j++ {
			if textures[j] == nil {
				continue
			}
			otherName := trimmed[j]
			if len(otherName) < 2 || otherName[0] != '+' || otherName[2:] != name[2:] {
				continue
			}

			frameIndex, alternate, err := textureAnimationFrame(otherName)
			if err != nil {
				return nil, err
			}
			if alternate {
				altAnims[frameIndex] = textures[j]
				if frameIndex+1 > altMax {
					altMax = frameIndex + 1
				}
			} else {
				anims[frameIndex] = textures[j]
				if frameIndex+1 > maxAnim {
					maxAnim = frameIndex + 1
				}
			}
		}

		for j := 0; j < maxAnim; j++ {
			if anims[j] == nil {
				return nil, fmt.Errorf("missing frame %d of %s", j, name)
			}
			anims[j].AnimTotal = maxAnim * textureAnimationCycle
			anims[j].AnimMin = j * textureAnimationCycle
			anims[j].AnimMax = (j + 1) * textureAnimationCycle
			anims[j].AnimNext = anims[(j+1)%maxAnim]
			if altMax > 0 {
				anims[j].AlternateAnims = altAnims[0]
			}
		}

		for j := 0; j < altMax; j++ {
			if altAnims[j] == nil {
				return nil, fmt.Errorf("missing frame %d of %s", j, name)
			}
			altAnims[j].AnimTotal = altMax * textureAnimationCycle
			altAnims[j].AnimMin = j * textureAnimationCycle
			altAnims[j].AnimMax = (j + 1) * textureAnimationCycle
			altAnims[j].AnimNext = altAnims[(j+1)%altMax]
			if maxAnim > 0 {
				altAnims[j].AlternateAnims = anims[0]
			}
		}
	}

	return textures, nil
}

// textureAnimationFrame selects the correct animation frame for a texture at the current time, matching Quake's deterministic anim cadence.
func textureAnimationFrame(name string) (frameIndex int, alternate bool, err error) {
	if len(name) < 2 || name[0] != '+' {
		return 0, false, fmt.Errorf("bad animating texture %q", name)
	}

	frame := name[1]
	switch {
	case frame >= '0' && frame <= '9':
		return int(frame - '0'), false, nil
	case frame >= 'a' && frame <= 'j':
		return int(frame - 'a'), true, nil
	case frame >= 'A' && frame <= 'J':
		return int(frame - 'A'), true, nil
	default:
		return 0, false, fmt.Errorf("bad animating texture %q", name)
	}
}

// TextureAnimation resolves the final texture pointer after following animation and alternate-chain rules used by switches and water variants.
func TextureAnimation(base *SurfaceTexture, frame int, timeSeconds float64) (*SurfaceTexture, error) {
	if base == nil {
		return nil, nil
	}

	if frame != 0 && base.AlternateAnims != nil {
		base = base.AlternateAnims
	}

	if base.AnimTotal == 0 {
		return base, nil
	}

	relative := int(timeSeconds*10.0) % base.AnimTotal
	if relative < 0 {
		relative += base.AnimTotal
	}

	count := 0
	for base.AnimMin > relative || base.AnimMax <= relative {
		base = base.AnimNext
		if base == nil {
			return nil, ErrBrokenTextureAnimationCycle
		}
		count++
		if count > 100 {
			return nil, ErrInfiniteTextureAnimation
		}
	}

	return base, nil
}

type Chart struct {
	reverse   bool
	x         int
	width     int
	height    int
	allocated []int
}

// Init prepares backend resources needed before the first frame, including API-specific state, cached GPU objects, and per-frame scratch structures used by the renderer.
func (c *Chart) Init(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid chart size %dx%d", width, height)
	}
	c.allocated = make([]int, width)
	c.width = width
	c.height = height
	c.x = 0
	c.reverse = false
	return nil
}

// Add appends a surface/lightmap block into an allocator or batch structure, centralizing bounds/capacity checks before write.
func (c *Chart) Add(w, h int) (outX, outY int, ok bool, err error) {
	if w <= 0 || h <= 0 {
		return 0, 0, false, fmt.Errorf("invalid block size %dx%d", w, h)
	}
	if c.width < w || c.height < h {
		return 0, 0, false, fmt.Errorf("block too large %dx%d, max is %dx%d", w, h, c.width, c.height)
	}

	var x int
	if c.reverse {
		if c.x < w {
			c.x = 0
			c.reverse = false
		}
	}

	if c.reverse {
		x = c.x - w
		c.x = x
	} else {
		if c.x+w > c.width {
			c.x = c.width
			c.reverse = true
			x = c.x - w
			c.x = x
		} else {
			x = c.x
			c.x += w
		}
	}

	y := 0
	for i := 0; i < w; i++ {
		if c.allocated[x+i] > y {
			y = c.allocated[x+i]
		}
	}
	if y+h > c.height {
		return 0, 0, false, nil
	}

	for i := 0; i < w; i++ {
		c.allocated[x+i] = y + h
	}

	return x, y, true, nil
}

type Lightmap struct {
	XOfs int
	YOfs int
}

type LightmapAllocator struct {
	blockWidth  int
	blockHeight int

	lightmaps     []Lightmap
	lastAllocated int
	chart         Chart

	reserveFirstTexel bool
}

// NewLightmapAllocator creates atlas packing state for static lightmaps so many faces can share a small set of GPU textures.
func NewLightmapAllocator(blockWidth, blockHeight int, reserveFirstTexel bool) (*LightmapAllocator, error) {
	if blockWidth <= 0 || blockHeight <= 0 {
		return nil, fmt.Errorf("invalid block size %dx%d", blockWidth, blockHeight)
	}

	lm := &LightmapAllocator{
		blockWidth:        blockWidth,
		blockHeight:       blockHeight,
		reserveFirstTexel: reserveFirstTexel,
	}

	return lm, nil
}

// Count reports how many batched entries are currently queued, useful for deciding when to flush before state or texture changes.
func (l *LightmapAllocator) Count() int {
	return len(l.lightmaps)
}

// Lightmaps exposes currently allocated lightmap pages for upload/debugging and runtime pass binding.
func (l *LightmapAllocator) Lightmaps() []Lightmap {
	out := make([]Lightmap, len(l.lightmaps))
	copy(out, l.lightmaps)
	return out
}

// AllocBlock performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func (l *LightmapAllocator) AllocBlock(w, h int) (texnum, x, y int, err error) {
	for texnum = l.lastAllocated; texnum < MaxSanityLightmaps; texnum++ {
		if texnum == len(l.lightmaps) {
			l.lightmaps = append(l.lightmaps, Lightmap{})
			if err := l.chart.Init(l.blockWidth, l.blockHeight); err != nil {
				return 0, 0, 0, err
			}
			if l.reserveFirstTexel && len(l.lightmaps) == 1 {
				l.chart.x = 1
				l.chart.allocated[0] = 1
			}
		}

		x, y, ok, addErr := l.chart.Add(w, h)
		if addErr != nil {
			return 0, 0, 0, addErr
		}
		if !ok {
			continue
		}

		l.lastAllocated = texnum
		return texnum, x, y, nil
	}

	return 0, 0, 0, errors.New("allocblock: full")
}

type SurfaceLightmapInput struct {
	Styles [4]byte

	Extents [2]int16
	LightS  int16
	LightT  int16

	Samples []byte
}

// NumLightmapTaps performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func NumLightmapTaps(styles [4]byte) int {
	if styles[1] == 255 {
		return 1
	}
	if styles[2] == 255 {
		return 2
	}
	return 3
}

// lightstyleCount performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func lightstyleCount(styles [4]byte) int {
	count := 0
	for _, s := range styles {
		if s == 255 {
			break
		}
		count++
	}
	if count == 0 {
		return 1
	}
	return count
}

// FillSurfaceLightmap performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func FillSurfaceLightmap(in SurfaceLightmapInput, lightmap Lightmap, lightmapWidth int, dst []uint32) error {
	if len(in.Samples) == 0 {
		return nil
	}

	smax := int(in.Extents[0]>>4) + 1
	tmax := int(in.Extents[1]>>4) + 1
	if smax <= 0 || tmax <= 0 {
		return fmt.Errorf("invalid extents %v", in.Extents)
	}

	mapCount := lightstyleCount(in.Styles)
	facesize := smax * tmax * 3
	if len(in.Samples) < facesize*mapCount {
		return fmt.Errorf("insufficient lightmap samples: have=%d need=%d", len(in.Samples), facesize*mapCount)
	}

	xofs := lightmap.XOfs + int(in.LightS)
	yofs := lightmap.YOfs + int(in.LightT)

	strideNeed := smax * NumLightmapTaps(in.Styles)
	lastRow := yofs + tmax - 1
	lastIndex := lastRow*lightmapWidth + xofs + strideNeed - 1
	if lightmapWidth <= 0 || xofs < 0 || yofs < 0 || lastIndex < 0 || lastIndex >= len(dst) {
		return fmt.Errorf("lightmap output out of bounds: width=%d xofs=%d yofs=%d need=%d dst=%d", lightmapWidth, xofs, yofs, strideNeed, len(dst))
	}

	src := in.Samples
	base := yofs*lightmapWidth + xofs

	if in.Styles[1] == 255 {
		for t := 0; t < tmax; t++ {
			row := base + t*lightmapWidth
			for s := 0; s < smax; s++ {
				dst[row+s] = uint32(src[0]) | (uint32(src[1]) << 8) | (uint32(src[2]) << 16) | 0xff000000
				src = src[3:]
			}
		}
		return nil
	}

	if in.Styles[2] == 255 {
		for t := 0; t < tmax; t++ {
			row := base + t*lightmapWidth
			for s := 0; s < smax; s++ {
				dst[row+s] = uint32(src[0]) | (uint32(src[1]) << 8) | (uint32(src[2]) << 16) | 0xff000000
				dst[row+s+smax] = uint32(src[facesize]) | (uint32(src[facesize+1]) << 8) | (uint32(src[facesize+2]) << 16) | 0xff000000
				src = src[3:]
			}
		}
		return nil
	}

	for t := 0; t < tmax; t++ {
		row := base + t*lightmapWidth
		for s := 0; s < smax; s++ {
			mapsrc := src
			var r, g, b uint32
			for mapIdx := 0; mapIdx < 4 && in.Styles[mapIdx] != 255; mapIdx++ {
				mapOfs := mapIdx * facesize
				r |= uint32(mapsrc[mapOfs]) << (mapIdx << 3)
				g |= uint32(mapsrc[mapOfs+1]) << (mapIdx << 3)
				b |= uint32(mapsrc[mapOfs+2]) << (mapIdx << 3)
			}
			dst[row+s] = r
			dst[row+s+smax] = g
			dst[row+s+smax*2] = b
			src = src[3:]
		}
	}

	return nil
}
