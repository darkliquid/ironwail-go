// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/mewkiz/flac"
)

func decodeMusicFLAC(name string, data []byte) (*musicTrack, error) {
	stream, err := flac.New(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode FLAC track %s: %w", name, err)
	}
	defer stream.Close()

	info := stream.Info
	if info.SampleRate == 0 {
		return nil, fmt.Errorf("invalid sample rate for %s", name)
	}
	if info.NChannels != 1 && info.NChannels != 2 {
		return nil, fmt.Errorf("unsupported FLAC channel count %d for %s", info.NChannels, name)
	}
	if info.BitsPerSample != 16 && info.BitsPerSample != 24 {
		return nil, fmt.Errorf("unsupported FLAC bit depth %d for %s (need 16 or 24)", info.BitsPerSample, name)
	}

	// Decode all frames into interleaved 16-bit PCM.
	var pcm []byte
	for {
		frame, err := stream.ParseNext()
		if err != nil {
			break // EOF or error — use what we have
		}
		nSamples := int(frame.Subframes[0].NSamples)
		buf := make([]byte, nSamples*int(info.NChannels)*2)
		for i := 0; i < nSamples; i++ {
			for ch := 0; ch < int(info.NChannels); ch++ {
				sample := frame.Subframes[ch].Samples[i]
				// Normalize to 16-bit range
				if info.BitsPerSample == 24 {
					sample >>= 8
				}
				if sample > 32767 {
					sample = 32767
				} else if sample < -32768 {
					sample = -32768
				}
				off := (i*int(info.NChannels) + ch) * 2
				binary.LittleEndian.PutUint16(buf[off:], uint16(int16(sample)))
			}
		}
		pcm = append(pcm, buf...)
	}

	if len(pcm) == 0 {
		return nil, fmt.Errorf("decoded FLAC track %s has no samples", name)
	}

	totalSamples := len(pcm) / (int(info.NChannels) * 2)
	return &musicTrack{
		name:     name,
		data:     pcm,
		samples:  totalSamples,
		rate:     int(info.SampleRate),
		width:    2,
		channels: int(info.NChannels),
	}, nil
}
