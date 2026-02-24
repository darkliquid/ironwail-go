// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import "math"

const paintBufferSize = 2048

type Mixer struct {
	scaleTable  ScaleTable
	volume      float64
	paintBuffer [paintBufferSize]SamplePair
	underwater  UnderwaterState
	loFreqLevel float32
	hiFreqLevel float32
}

func NewMixer() *Mixer {
	return &Mixer{
		volume: 0.7,
	}
}

func (m *Mixer) SetVolume(vol float64) {
	m.volume = vol
	InitScaleTable(&m.scaleTable, vol)
}

func (m *Mixer) PaintChannels(channels []Channel, rawSamples *RawSamplesBuffer, dma *DMAInfo, paintedTime, endTime int) int {
	sndVol := int(m.volume * 256)

	for paintedTime < endTime {
		end := endTime
		if endTime-paintedTime > paintBufferSize {
			end = paintedTime + paintBufferSize
		}

		count := end - paintedTime
		for i := 0; i < count; i++ {
			m.paintBuffer[i] = SamplePair{}
		}

		for chIdx := range channels {
			ch := &channels[chIdx]
			if ch.SFX == nil || ch.SFX.Cache == nil {
				continue
			}
			if ch.LeftVol == 0 && ch.RightVol == 0 {
				continue
			}

			cache := ch.SFX.Cache
			if cache.Width == 1 {
				m.paintChannel8(ch, cache, count)
			} else {
				m.paintChannel16(ch, cache, count, sndVol)
			}
		}

		for i := 0; i < count; i++ {
			left := clampInt(int(m.paintBuffer[i].Left), -32768*256, 32767*256) / 2
			right := clampInt(int(m.paintBuffer[i].Right), -32768*256, 32767*256) / 2
			m.paintBuffer[i].Left = int32(left)
			m.paintBuffer[i].Right = int32(right)
		}

		if m.underwater.Intensity > 0 {
			m.applyUnderwaterFilter(count)
		}

		if rawSamples.End >= paintedTime {
			stop := end
			if stop > rawSamples.End {
				stop = rawSamples.End
			}
			for i := paintedTime; i < stop; i++ {
				s := i & (MaxRawSamples - 1)
				m.paintBuffer[i-paintedTime].Left += rawSamples.Samples[s].Left / 2
				m.paintBuffer[i-paintedTime].Right += rawSamples.Samples[s].Right / 2
			}
		}

		m.transferPaintBuffer(dma, count)
		paintedTime = end
	}

	return paintedTime
}

func (m *Mixer) paintChannel8(ch *Channel, cache *SoundCache, count int) {
	if ch.LeftVol > 255 {
		ch.LeftVol = 255
	}
	if ch.RightVol > 255 {
		ch.RightVol = 255
	}

	lscale := m.scaleTable[ch.LeftVol>>3]
	rscale := m.scaleTable[ch.RightVol>>3]

	for i := 0; i < count; i++ {
		if ch.Pos >= len(cache.Data) {
			break
		}
		data := cache.Data[ch.Pos]
		m.paintBuffer[i].Left += lscale[data]
		m.paintBuffer[i].Right += rscale[data]
		ch.Pos++
	}
}

func (m *Mixer) paintChannel16(ch *Channel, cache *SoundCache, count int, sndVol int) {
	leftVol := ch.LeftVol * sndVol / 256
	rightVol := ch.RightVol * sndVol / 256

	for i := 0; i < count; i++ {
		if ch.Pos*2+1 >= len(cache.Data) {
			break
		}
		sample := int(int16(uint16(cache.Data[ch.Pos*2]) | uint16(cache.Data[ch.Pos*2+1])<<8))
		m.paintBuffer[i].Left += int32(sample * leftVol)
		m.paintBuffer[i].Right += int32(sample * rightVol)
		ch.Pos++
	}
}

func (m *Mixer) applyUnderwaterFilter(count int) {
	for i := 0; i < count; i++ {
		m.underwater.Accum[0] += m.underwater.Alpha * (float32(m.paintBuffer[i].Left) - m.underwater.Accum[0])
		m.underwater.Accum[1] += m.underwater.Alpha * (float32(m.paintBuffer[i].Right) - m.underwater.Accum[1])
		m.paintBuffer[i].Left = int32(m.underwater.Accum[0])
		m.paintBuffer[i].Right = int32(m.underwater.Accum[1])
	}
}

func (m *Mixer) transferPaintBuffer(dma *DMAInfo, count int) {
	if dma.SampleBits == 16 && dma.Channels == 2 {
		for i := 0; i < count; i++ {
			pos := (dma.SamplePos + i*2) % len(dma.Buffer)
			left := clampInt(int(m.paintBuffer[i].Left/256), -32768, 32767)
			right := clampInt(int(m.paintBuffer[i].Right/256), -32768, 32767)
			dma.Buffer[pos] = byte(left)
			dma.Buffer[pos+1] = byte(left >> 8)
			dma.Buffer[pos+2] = byte(right)
			dma.Buffer[pos+3] = byte(right >> 8)
		}
		dma.SamplePos = (dma.SamplePos + count*2) % len(dma.Buffer)
	}
}

func (m *Mixer) SetUnderwaterIntensity(target float32) {
	target = float32(math.Min(float64(target), 2.0))
	if m.underwater.Intensity < target {
		m.underwater.Intensity += 0.016
		if m.underwater.Intensity > target {
			m.underwater.Intensity = target
		}
	} else if m.underwater.Intensity > target {
		m.underwater.Intensity -= 0.016
		if m.underwater.Intensity < target {
			m.underwater.Intensity = target
		}
	}
	m.underwater.Alpha = float32(math.Exp(-float64(m.underwater.Intensity) * math.Log(12)))
}

func (m *Mixer) ClearLevels() {
	m.loFreqLevel = 0
	m.hiFreqLevel = 0
}

func (m *Mixer) GetLoFreqLevel() float32 {
	return m.loFreqLevel
}

func (m *Mixer) GetHiFreqLevel() float32 {
	return m.hiFreqLevel
}
func clampInt(val, min, max int) int {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
