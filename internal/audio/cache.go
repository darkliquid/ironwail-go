// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"math"
)

// SFXCache manages loaded sound effects.
type SFXCache struct {
	sounds    [MaxSFX]*SFX
	numSounds int
	dmaSpeed  int
	load8Bit  bool
}

// NewSFXCache creates a new sound effect cache.
func NewSFXCache(dmaSpeed int, load8Bit bool) *SFXCache {
	return &SFXCache{
		dmaSpeed: dmaSpeed,
		load8Bit: load8Bit,
	}
}

// FindName returns an existing SFX by name or creates a new one.
func (c *SFXCache) FindName(name string) *SFX {
	for i := 0; i < c.numSounds; i++ {
		if c.sounds[i].Name == name {
			return c.sounds[i]
		}
	}

	if c.numSounds >= MaxSFX {
		return nil
	}

	sfx := &SFX{Name: name}
	c.sounds[c.numSounds] = sfx
	c.numSounds++

	return sfx
}

// Load loads sound data for the given SFX.
func (c *SFXCache) Load(sfx *SFX, fileData []byte) *SoundCache {
	if sfx == nil || len(fileData) == 0 {
		return nil
	}

	if sfx.Cache != nil {
		return sfx.Cache
	}

	sampleData, info, err := LoadWAV(sfx.Name, fileData)
	if err != nil {
		return nil
	}

	outWidth := info.Width
	if c.load8Bit {
		outWidth = 1
	}

	stepScale := float64(info.Rate) / float64(c.dmaSpeed)
	outLength := int(float64(info.Samples) / stepScale)

	cache := &SoundCache{
		Length:    outLength,
		LoopStart: -1,
		Speed:     c.dmaSpeed,
		Width:     outWidth,
		Stereo:    0,
	}

	if info.LoopStart >= 0 {
		cache.LoopStart = int(float64(info.LoopStart) / stepScale)
	}

	cache.Data = make([]byte, outLength*outWidth)

	c.resample(cache, sampleData, info.Samples, info.Width, stepScale)

	sfx.Cache = cache
	sfx.Cached = true

	return cache
}

func (c *SFXCache) resample(cache *SoundCache, data []byte, inSamples, inWidth int, stepScale float64) {
	if stepScale == 1.0 && inWidth == 1 && cache.Width == 1 {
		for i := 0; i < cache.Length; i++ {
			cache.Data[i] = byte(int(data[i]) - 128)
		}
		return
	}

	srcSample := 0
	sampleFrac := 0
	fracStep := int(stepScale * 256)

	for i := 0; i < cache.Length; i++ {
		var sample int

		if inWidth == 2 {
			sample = int(int16(uint16(data[srcSample*2]) | uint16(data[srcSample*2+1])<<8))
		} else {
			sample = (int(data[srcSample]) - 128) << 8
		}

		if cache.Width == 2 {
			cache.Data[i*2] = byte(sample)
			cache.Data[i*2+1] = byte(sample >> 8)
		} else {
			cache.Data[i] = byte(sample >> 8)
		}

		sampleFrac += fracStep
		srcSample += sampleFrac >> 8
		sampleFrac &= 255
	}
}

// ResampleSfx resamples sound data from one sample rate to another.
func ResampleSfx(cache *SoundCache, inRate, inWidth int, data []byte, outRate int) {
	stepScale := float64(inRate) / float64(outRate)
	outCount := int(float64(cache.Length) / stepScale)

	cache.Length = outCount
	if cache.LoopStart >= 0 {
		cache.LoopStart = int(float64(cache.LoopStart) / stepScale)
	}
	cache.Speed = outRate

	srcSample := 0
	sampleFrac := 0
	fracStep := int(stepScale * 256)

	for i := 0; i < outCount; i++ {
		var sample int

		if inWidth == 2 {
			sample = int(int16(uint16(data[srcSample*2]) | uint16(data[srcSample*2+1])<<8))
		} else {
			sample = (int(data[srcSample]) - 128) << 8
		}

		if cache.Width == 2 {
			cache.Data[i*2] = byte(sample)
			cache.Data[i*2+1] = byte(sample >> 8)
		} else {
			cache.Data[i] = byte(sample >> 8)
		}

		sampleFrac += fracStep
		srcSample += sampleFrac >> 8
		sampleFrac &= 255
	}
}

// InitScaleTable precomputes the volume scaling table for 8-bit mixing.
func InitScaleTable(table *ScaleTable, volume float64) {
	for i := 0; i < 32; i++ {
		scale := i * 8 * int(volume*256)
		for j := 0; j < 256; j++ {
			signed := j
			if j >= 128 {
				signed = j - 256
			}
			table[i][j] = int32(signed * scale)
		}
	}
}

// ClampInt clamps an int value between min and max.
func ClampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// Lerp performs linear interpolation.
func Lerp(a, b, t float32) float32 {
	return a + t*(b-a)
}

// VectorNormalize normalizes a vector and returns its length.
func VectorNormalize(v *[3]float32) float32 {
	length := float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
	if length > 0 {
		inv := 1.0 / length
		v[0] *= inv
		v[1] *= inv
		v[2] *= inv
	}
	return length
}

// VectorSubtract subtracts two vectors.
func VectorSubtract(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

// DotProduct computes the dot product of two vectors.
func DotProduct(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}
