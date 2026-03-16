// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"bytes"
	"fmt"
	"io"

	opusgo "github.com/kazzmir/opus-go"
)

func decodeMusicOpus(name string, data []byte) (*musicTrack, error) {
	player, err := opusgo.NewPlayerFromReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode Opus track %s: %w", name, err)
	}

	var pcm []byte
	buf := make([]byte, 16384)
	for {
		n, err := player.ReadPacket(buf)
		if n > 0 {
			pcm = append(pcm, buf[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read Opus data for %s: %w", name, err)
		}
		if n == 0 && player.IsFinished() {
			break
		}
	}

	// Opus is always 48kHz stereo from this player
	channels := 2
	width := 2
	samples := len(pcm) / (channels * width)

	return &musicTrack{
		name:     name,
		data:     pcm,
		samples:  samples,
		rate:     player.SampleRate(),
		width:    width,
		channels: channels,
	}, nil
}
