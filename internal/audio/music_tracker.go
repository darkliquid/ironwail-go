// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/gotracker/playback/format"
	"github.com/gotracker/playback/mixing"
	"github.com/gotracker/playback/mixing/sampling"
	"github.com/gotracker/playback/output"
	"github.com/gotracker/playback/player/feature"
	"github.com/gotracker/playback/player/machine"
	"github.com/gotracker/playback/player/machine/settings"
	"github.com/gotracker/playback/player/sampler"
	"github.com/gotracker/playback/song"
)

const (
	trackerSampleRate   = 44100
	trackerChannels     = 2
	trackerSampleFormat = sampling.Format16BitLESigned
	trackerSampleWidth  = 2 // 16-bit = 2 bytes
)

// decodeMusicTracker decodes tracker music formats (MOD, S3M, XM, IT) into PCM samples
func decodeMusicTracker(name string, data []byte) (*musicTrack, error) {
	formatName := detectTrackerFormat(name)
	if formatName == "" {
		return nil, fmt.Errorf("unsupported tracker format for %s", name)
	}

	// Configure features for playback
	features := []feature.Feature{
		feature.UseNativeSampleFormat(true),
		feature.IgnoreUnknownEffect{Enabled: true},
		feature.SongLoop{Count: 0}, // Disable internal looping (Quake handles it)
	}

	// Load the module
	songData, songFormat, err := format.LoadFromReader(formatName, bytes.NewReader(data), features)
	if err != nil {
		return nil, fmt.Errorf("failed to load tracker module %s: %w", name, err)
	}

	// Convert features to settings
	var userSettings settings.UserSettings
	if err := songFormat.ConvertFeaturesToSettings(&userSettings, features); err != nil {
		return nil, fmt.Errorf("failed to configure tracker settings for %s: %w", name, err)
	}

	// Create player machine
	player, err := machine.NewMachine(songData, userSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracker player for %s: %w", name, err)
	}

	// Pre-render the entire track to PCM buffer
	pcmData, err := renderTrackerToPCM(player)
	if err != nil {
		return nil, fmt.Errorf("failed to render tracker %s: %w", name, err)
	}

	if len(pcmData) == 0 {
		return nil, fmt.Errorf("rendered tracker %s has no samples", name)
	}

	// Calculate frame count
	frameSize := trackerChannels * trackerSampleWidth
	if len(pcmData)%frameSize != 0 {
		return nil, fmt.Errorf("rendered tracker %s has partial frame data", name)
	}
	frameCount := len(pcmData) / frameSize

	return &musicTrack{
		name:     name,
		data:     pcmData,
		samples:  frameCount,
		rate:     trackerSampleRate,
		width:    trackerSampleWidth,
		channels: trackerChannels,
	}, nil
}

// renderTrackerToPCM pre-renders the entire tracker module to a PCM buffer
func renderTrackerToPCM(player machine.MachineTicker) ([]byte, error) {
	// Buffer to accumulate PCM data
	var pcmBuffer bytes.Buffer

	// Channel for premix data
	premixChan := make(chan *output.PremixData, 8)
	defer close(premixChan)

	// Create sampler
	out := sampler.NewSampler(trackerSampleRate, trackerChannels, 1.0, func(premix *output.PremixData) {
		premixChan <- premix
	})
	if out == nil {
		return nil, errors.New("could not create sampler")
	}

	// Create mixer for output format conversion
	mixer := mixing.Mixer{
		Channels: trackerChannels,
	}

	// Goroutine to consume premix data and convert to PCM
	renderDone := make(chan error, 1)
	go func() {
		for premix := range premixChan {
			data := mixer.Flatten(premix.SamplesLen, premix.Data, premix.MixerVolume, trackerSampleFormat)
			if _, err := pcmBuffer.Write(data); err != nil {
				renderDone <- fmt.Errorf("write error: %w", err)
				return
			}
		}
		renderDone <- nil
	}()

	// Render loop
	for {
		if err := player.Advance(); err != nil {
			if errors.Is(err, song.ErrStopSong) {
				break
			}
			return nil, fmt.Errorf("advance error: %w", err)
		}

		if err := player.Render(out); err != nil {
			if errors.Is(err, song.ErrStopSong) {
				break
			}
			return nil, fmt.Errorf("render error: %w", err)
		}
	}

	// Wait for consumer goroutine to finish
	if err := <-renderDone; err != nil {
		return nil, err
	}

	return pcmBuffer.Bytes(), nil
}

// detectTrackerFormat returns the format name for gotracker based on file extension
func detectTrackerFormat(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".mod"):
		return "mod"
	case strings.HasSuffix(lower, ".s3m"):
		return "s3m"
	case strings.HasSuffix(lower, ".xm"):
		return "xm"
	case strings.HasSuffix(lower, ".it"):
		return "it"
	default:
		return ""
	}
}
