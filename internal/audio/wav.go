// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"encoding/binary"
	"fmt"
	"io"
)

type wavParser struct {
	data     []byte
	pos      int
	end      int
	chunkLen int
}

func newWavParser(data []byte) *wavParser {
	return &wavParser{
		data: data,
		pos:  0,
		end:  len(data),
	}
}

func (p *wavParser) getLittleShort() int16 {
	if p.pos+2 > len(p.data) {
		return 0
	}
	val := int16(binary.LittleEndian.Uint16(p.data[p.pos:]))
	p.pos += 2
	return val
}

func (p *wavParser) getLittleLong() int32 {
	if p.pos+4 > len(p.data) {
		return 0
	}
	val := int32(binary.LittleEndian.Uint32(p.data[p.pos:]))
	p.pos += 4
	return val
}

func (p *wavParser) findNextChunk(name string) bool {
	for {
		if p.pos+8 > p.end {
			p.pos = len(p.data)
			return false
		}

		p.pos += 4
		p.chunkLen = int(p.getLittleLong())

		if p.chunkLen < 0 || p.chunkLen > p.end-p.pos {
			p.pos = len(p.data)
			return false
		}

		// WAV chunks are word-aligned (2-byte boundary)
		lastChunk := p.pos + ((p.chunkLen + 1) & ^1)
		p.pos -= 8

		if p.pos+4 <= len(p.data) && string(p.data[p.pos:p.pos+4]) == name {
			return true
		}

		p.pos = lastChunk
	}
}

func (p *wavParser) findChunk(name string) bool {
	p.pos = 0
	return p.findNextChunk(name)
}

// GetWavInfo parses a WAV file and extracts audio metadata.
func GetWavInfo(name string, wav []byte, wavLength int) WAVInfo {
	var info WAVInfo
	info.LoopStart = -1

	if len(wav) == 0 {
		return info
	}

	p := newWavParser(wav)
	p.end = wavLength

	if !p.findChunk("RIFF") {
		return info
	}

	if p.pos+12 > len(wav) || string(wav[p.pos+8:p.pos+12]) != "WAVE" {
		return info
	}

	p.pos = p.pos + 12
	if !p.findChunk("fmt ") {
		return info
	}

	p.pos += 8
	format := p.getLittleShort()
	if format != WAVFormatPCM {
		return info
	}

	info.Channels = int(p.getLittleShort())
	info.Rate = int(p.getLittleLong())
	p.pos += 6

	bitsPerSample := p.getLittleShort()
	if bitsPerSample != 8 && bitsPerSample != 16 {
		return info
	}
	info.Width = int(bitsPerSample / 8)

	if p.findChunk("cue ") {
		p.pos += 32
		info.LoopStart = int(p.getLittleLong())

		if p.findNextChunk("LIST") {
			if p.pos+32 <= len(wav) && string(wav[p.pos+28:p.pos+32]) == "mark" {
				// CoolEdit loop marker workaround
				p.pos += 24
				loopSamples := p.getLittleLong()
				info.Samples = info.LoopStart + int(loopSamples)
			}
		}
	}

	if !p.findChunk("data") {
		return info
	}

	p.pos += 4
	dataSize := int(p.getLittleLong())
	samples := dataSize / info.Width

	if info.Samples == 0 {
		info.Samples = samples
	} else if samples < info.Samples {
		return WAVInfo{}
	}

	if info.LoopStart >= info.Samples {
		info.LoopStart = -1
		info.Samples = samples
	}

	info.DataOfs = p.pos

	return info
}

// LoadWAV loads a WAV file from raw bytes and returns parsed audio data.
func LoadWAV(name string, data []byte) ([]byte, WAVInfo, error) {
	if len(data) == 0 {
		return nil, WAVInfo{}, fmt.Errorf("empty WAV data for %s", name)
	}

	info := GetWavInfo(name, data, len(data))
	if info.Samples == 0 {
		return nil, info, fmt.Errorf("failed to parse WAV header for %s", name)
	}

	if info.Channels != 1 {
		return nil, info, fmt.Errorf("%s is a stereo sample (only mono supported)", name)
	}

	if info.Width != 1 && info.Width != 2 {
		return nil, info, fmt.Errorf("%s is not 8 or 16 bit", name)
	}

	if info.DataOfs < 0 || info.DataOfs+info.Samples*info.Width > len(data) {
		return nil, info, fmt.Errorf("%s has invalid data offset", name)
	}

	sampleData := make([]byte, info.Samples*info.Width)
	copy(sampleData, data[info.DataOfs:info.DataOfs+info.Samples*info.Width])

	return sampleData, info, nil
}

// ConvertUnsigned8ToSigned converts unsigned 8-bit samples to signed 8-bit.
func ConvertUnsigned8ToSigned(data []byte) {
	for i := range data {
		data[i] = data[i] - 128
	}
}

// ConvertSignedToUnsigned8 converts signed 8-bit samples back to unsigned.
func ConvertSignedToUnsigned8(data []byte) {
	for i := range data {
		data[i] = data[i] + 128
	}
}

// ReadWAVFromReader reads a complete WAV file from an io.Reader.
func ReadWAVFromReader(name string, r io.Reader) ([]byte, WAVInfo, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, WAVInfo{}, fmt.Errorf("failed to read %s: %w", name, err)
	}
	return LoadWAV(name, data)
}
