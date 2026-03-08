// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/jfreymuth/oggvorbis"
)

func decodeMusicOGG(name string, data []byte) (*musicTrack, error) {
	samples, format, err := oggvorbis.ReadAll(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode OGG track %s: %w", name, err)
	}
	if format == nil {
		return nil, fmt.Errorf("missing OGG format metadata for %s", name)
	}
	if format.SampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate %d for %s", format.SampleRate, name)
	}
	if format.Channels != 1 && format.Channels != 2 {
		return nil, fmt.Errorf("unsupported OGG channel count %d for %s", format.Channels, name)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("decoded OGG track %s has no samples", name)
	}
	if len(samples)%format.Channels != 0 {
		return nil, fmt.Errorf("decoded OGG track %s has partial frame data", name)
	}

	pcm := make([]byte, len(samples)*2)
	for i, sample := range samples {
		scaled := int32(sample * 32768.0)
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}
		binary.LittleEndian.PutUint16(pcm[i*2:], uint16(int16(scaled)))
	}

	return &musicTrack{
		name:     name,
		data:     pcm,
		samples:  len(samples) / format.Channels,
		rate:     format.SampleRate,
		width:    2,
		channels: format.Channels,
	}, nil
}
