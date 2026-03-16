// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"bytes"
	"fmt"
	"io"

	"github.com/hajimehoshi/go-mp3"
)

func decodeMusicMP3(name string, data []byte) (*musicTrack, error) {
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode MP3 track %s: %w", name, err)
	}

	samples := int(decoder.Length() / int64(4)) // 16-bit stereo is 4 bytes per frame
	if samples == 0 {
		return nil, fmt.Errorf("decoded MP3 track %s has no samples", name)
	}

	pcm, err := io.ReadAll(decoder)
	if err != nil {
		return nil, fmt.Errorf("failed to read MP3 data for %s: %w", name, err)
	}

	return &musicTrack{
		name:     name,
		data:     pcm,
		samples:  samples,
		rate:     decoder.SampleRate(),
		width:    2,
		channels: 2, // go-mp3 always outputs stereo 16-bit
	}, nil
}
