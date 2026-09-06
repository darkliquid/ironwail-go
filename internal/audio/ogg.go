// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"bytes"
	"fmt"

	"github.com/jfreymuth/oggvorbis"
)

type oggStream struct {
	reader   *oggvorbis.Reader
	channels int
	rate     int
	length   int64
	fltBuf   []float32
}

func (s *oggStream) ReadFrames(dst []byte) (int, error) {
	frameSize := s.channels * 2
	maxFrames := len(dst) / frameSize
	if maxFrames == 0 {
		return 0, nil
	}
	neededSamples := maxFrames * s.channels
	if len(s.fltBuf) < neededSamples {
		s.fltBuf = make([]float32, neededSamples)
	}

	n, err := s.reader.Read(s.fltBuf[:neededSamples])
	frames := n / s.channels
	for i := 0; i < n; i++ {
		scaled := int32(s.fltBuf[i] * 32768.0)
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}
		val := uint16(int16(scaled))
		idx := i * 2
		dst[idx] = byte(val)
		dst[idx+1] = byte(val >> 8)
	}
	return frames, err
}

func (s *oggStream) SeekFrame(frame int64) error {
	if frame < 0 || (s.length > 0 && frame > s.length) {
		return fmt.Errorf("invalid frame offset %d", frame)
	}
	return s.reader.SetPosition(frame)
}

func (s *oggStream) Close() error {
	return nil
}

func decodeMusicOGG(name string, data []byte) (*musicTrack, error) {
	reader, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode OGG track %s: %w", name, err)
	}
	ch := reader.Channels()
	if ch != 1 && ch != 2 {
		return nil, fmt.Errorf("unsupported OGG channel count %d for %s", ch, name)
	}
	rate := reader.SampleRate()
	if rate <= 0 {
		return nil, fmt.Errorf("invalid sample rate %d for %s", rate, name)
	}
	length := int(reader.Length())
	if length <= 0 {
		return nil, fmt.Errorf("decoded OGG track %s has no samples", name)
	}

	return &musicTrack{
		name:     name,
		stream:   &oggStream{reader: reader, channels: ch, rate: rate, length: int64(length)},
		samples:  length,
		rate:     rate,
		width:    2,
		channels: ch,
	}, nil
}
