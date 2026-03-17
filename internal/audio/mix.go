// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"math"
)

const paintBufferSize = 2048

type filter struct {
	memory     []float32
	kernel     []float32
	kernelSize int
	m          int
	parity     int
	fc         float32
}

type Mixer struct {
	scaleTable  ScaleTable
	volume      float64
	paintBuffer [paintBufferSize]SamplePair
	underwater  UnderwaterState
	loFreqLevel float32
	hiFreqLevel float32

	filterL       filter
	filterR       filter
	filterQuality int
	sndSpeed      int
}

func NewMixer() *Mixer {
	return &Mixer{
		volume:        0.7,
		filterQuality: 5,
		sndSpeed:      11025,
	}
}

func (m *Mixer) SetVolume(vol float64) {
	m.volume = vol
	InitScaleTable(&m.scaleTable, vol)
}

func (m *Mixer) SetFilterQuality(quality int) {
	m.filterQuality = quality
}

func (m *Mixer) SetSndSpeed(speed int) {
	m.sndSpeed = speed
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

			ltime := paintedTime

			pitch := ch.Pitch
			if pitch <= 0 {
				pitch = 1.0
			}

			for ltime < end {
				// How many output samples until we hit the end of the cache?
				remainingSource := float32(cache.Length-1-ch.Pos) - ch.PosFraction
				if remainingSource < 0 {
					remainingSource = 0
				}
				outputNeeded := int(math.Ceil(float64(remainingSource / pitch)))

				paintCount := end - ltime
				if outputNeeded < paintCount {
					paintCount = outputNeeded
				}

				if paintCount > 0 {
					if cache.Width == 1 {
						m.paintChannel8(ch, cache, paintCount, ltime-paintedTime)
					} else {
						m.paintChannel16(ch, cache, paintCount, sndVol, ltime-paintedTime)
					}
					ltime += paintCount
				}

				if ch.Pos >= cache.Length-1 {
					if cache.LoopStart >= 0 {
						ch.Pos = cache.LoopStart
						ch.PosFraction = 0
						// We don't really use ch.End for termination anymore, but
						// let's keep it somewhat sane.
						ch.End = ltime + int(float32(cache.Length-ch.Pos)/pitch)
					} else {
						ch.SFX = nil
						break
					}
				} else {
					// We filled the buffer but haven't reached the end of the sound
					break
				}
			}
		}

		for i := 0; i < count; i++ {
			m.paintBuffer[i].Left = int32(ClampInt(int(m.paintBuffer[i].Left), -32768*256, 32767*256) / 2)
			m.paintBuffer[i].Right = int32(ClampInt(int(m.paintBuffer[i].Right), -32768*256, 32767*256) / 2)
		}

		if m.sndSpeed == 11025 && dma.Speed == 44100 {
			m.lowpassFilter(true, count, &m.filterL, m.filterQuality)
			m.lowpassFilter(false, count, &m.filterR, m.filterQuality)
		}

		if m.underwater.Intensity > 0 {
			m.applyUnderwaterFilter(count)
		}

		m.updateLevels(count)

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

		m.transferPaintBuffer(dma, paintedTime, count)
		paintedTime = end
	}

	return paintedTime
}

func (m *Mixer) paintChannel8(ch *Channel, cache *SoundCache, count int, paintBufferStart int) {
	if ch.LeftVol > 255 {
		ch.LeftVol = 255
	}
	if ch.RightVol > 255 {
		ch.RightVol = 255
	}

	lscale := m.scaleTable[ch.LeftVol>>3]
	rscale := m.scaleTable[ch.RightVol>>3]

	step := ch.Pitch
	if step <= 0 {
		step = 1.0
	}

	for i := 0; i < count; i++ {
		if ch.Pos >= cache.Length-1 {
			break
		}

		// Linear interpolation
		frac := ch.PosFraction
		s1L := float32(lscale[cache.Data[ch.Pos]])
		s1R := float32(rscale[cache.Data[ch.Pos]])
		s2L := float32(lscale[cache.Data[ch.Pos+1]])
		s2R := float32(rscale[cache.Data[ch.Pos+1]])

		m.paintBuffer[paintBufferStart+i].Left += int32(s1L + frac*(s2L-s1L))
		m.paintBuffer[paintBufferStart+i].Right += int32(s1R + frac*(s2R-s1R))

		ch.PosFraction += step
		for ch.PosFraction >= 1.0 {
			ch.PosFraction -= 1.0
			ch.Pos++
		}
	}
}

func (m *Mixer) paintChannel16(ch *Channel, cache *SoundCache, count int, sndVol int, paintBufferStart int) {
	leftVol := ch.LeftVol * sndVol / 256
	rightVol := ch.RightVol * sndVol / 256

	step := ch.Pitch
	if step <= 0 {
		step = 1.0
	}

	for i := 0; i < count; i++ {
		if ch.Pos >= cache.Length-1 {
			break
		}

		// Linear interpolation
		frac := ch.PosFraction
		getSample := func(p int) int32 {
			return int32(int16(uint16(cache.Data[p*2]) | uint16(cache.Data[p*2+1])<<8))
		}

		s1 := getSample(ch.Pos)
		s2 := getSample(ch.Pos + 1)

		sample := int32(float32(s1) + frac*(float32(s2-s1)))

		m.paintBuffer[paintBufferStart+i].Left += sample * int32(leftVol)
		m.paintBuffer[paintBufferStart+i].Right += sample * int32(rightVol)

		ch.PosFraction += step
		for ch.PosFraction >= 1.0 {
			ch.PosFraction -= 1.0
			ch.Pos++
		}
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

func (m *Mixer) updateLevels(count int) {
	if m.volume <= 0 {
		m.loFreqLevel = 0
		m.hiFreqLevel = 0
		return
	}

	scale := 0.5 / (m.volume * 32768.0)
	for i := 0; i < count; i++ {
		sample := float32(math.Abs(float64(m.paintBuffer[i].Left))+math.Abs(float64(m.paintBuffer[i].Right))) * float32(scale)
		m.loFreqLevel = Lerp(m.loFreqLevel, sample, 1e-3)
		m.hiFreqLevel = Lerp(m.hiFreqLevel, sample, 1e-2)
	}
}

func (m *Mixer) lowpassFilter(left bool, count int, f *filter, filterQuality int) {
	mVal := 0
	var bw float32
	switch filterQuality {
	case 1:
		mVal = 126
		bw = 0.900
	case 2:
		mVal = 150
		bw = 0.915
	case 3:
		mVal = 174
		bw = 0.930
	case 4:
		mVal = 198
		bw = 0.945
	case 5:
	default:
		mVal = 222
		bw = 0.960
	}

	fc := (bw * 11025 / 2.0) / 44100.0
	m.updateFilter(f, mVal, float32(fc))

	input := make([]float32, f.kernelSize+count)
	copy(input, f.memory)

	for i := 0; i < count; i++ {
		var val int32
		if left {
			val = m.paintBuffer[i].Left
		} else {
			val = m.paintBuffer[i].Right
		}
		input[f.kernelSize+i] = float32(val) / (32768.0 * 256.0)
	}

	copy(f.memory, input[count:])

	parity := f.parity
	for i := 0; i < count; i++ {
		inputPlusI := input[i:]
		var res float32

		for j := (4 - parity) % 4; j < f.kernelSize; j += 16 {
			res += f.kernel[j] * inputPlusI[j]
			res += f.kernel[j+4] * inputPlusI[j+4]
			res += f.kernel[j+8] * inputPlusI[j+8]
			res += f.kernel[j+12] * inputPlusI[j+12]
		}

		finalVal := int32(res * (32768.0 * 256.0 * 4.0))
		if left {
			m.paintBuffer[i].Left = finalVal
		} else {
			m.paintBuffer[i].Right = finalVal
		}
		parity = (parity + 1) % 4
	}
	f.parity = parity
}

func (m *Mixer) updateFilter(f *filter, mVal int, fc float32) {
	if f.fc != fc || f.m != mVal {
		f.m = mVal
		f.fc = fc
		f.parity = 0
		f.kernelSize = (mVal + 1) + 16 - ((mVal + 1) % 16)
		f.memory = make([]float32, f.kernelSize)
		f.kernel = make([]float32, f.kernelSize)
		m.makeBlackmanWindowKernel(f.kernel, mVal, fc)
	}
}

func (m *Mixer) makeBlackmanWindowKernel(kernel []float32, mVal int, fc float32) {
	for i := 0; i <= mVal; i++ {
		if i == mVal/2 {
			kernel[i] = 2 * math.Pi * fc
		} else {
			kernel[i] = float32((math.Sin(2*math.Pi*float64(fc)*float64(i-mVal/2)) / float64(i-mVal/2)) *
				(0.42 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(mVal)) + 0.08*math.Cos(4*math.Pi*float64(i)/float64(mVal))))
		}
	}

	var sum float32
	for i := 0; i <= mVal; i++ {
		sum += kernel[i]
	}
	for i := 0; i <= mVal; i++ {
		kernel[i] /= sum
	}
}

func (m *Mixer) transferPaintBuffer(dma *DMAInfo, paintedTime int, count int) {
	if dma.SampleBits == 16 && dma.Channels == 2 {
		for i := 0; i < count; i++ {
			lpos := (paintedTime + i) % dma.Samples
			pos := lpos * 4
			if pos+3 >= len(dma.Buffer) {
				continue
			}
			left := ClampInt(int(m.paintBuffer[i].Left/256), -32768, 32767)
			right := ClampInt(int(m.paintBuffer[i].Right/256), -32768, 32767)
			dma.Buffer[pos] = byte(left)
			dma.Buffer[pos+1] = byte(left >> 8)
			dma.Buffer[pos+2] = byte(right)
			dma.Buffer[pos+3] = byte(right >> 8)
		}
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
