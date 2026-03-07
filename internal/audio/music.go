// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"fmt"
	"strings"
)

var supportedMusicExtensions = []string{".wav"}

type musicTrack struct {
	name     string
	data     []byte
	samples  int
	rate     int
	width    int
	channels int
}

type musicState struct {
	requestTrack int
	loopTrack    int
	activeTrack  int
	position     int
	loader       func(string) ([]byte, error)
	track        *musicTrack
}

func (s *System) PlayCDTrack(track, loopTrack int, loader func(string) ([]byte, error)) error {
	if track <= 0 {
		s.StopMusic()
		return nil
	}
	if loader == nil {
		s.StopMusic()
		return fmt.Errorf("music loader is not available")
	}
	if loopTrack == 0 {
		loopTrack = track
	}
	if s.music != nil && s.music.requestTrack == track && s.music.loopTrack == loopTrack {
		return nil
	}

	resolved, err := loadMusicTrack(track, loader)
	if err != nil {
		s.StopMusic()
		return err
	}

	s.music = &musicState{
		requestTrack: track,
		loopTrack:    loopTrack,
		activeTrack:  track,
		loader:       loader,
		track:        resolved,
	}
	if s.rawSamples.End < s.paintedTime {
		s.rawSamples.End = s.paintedTime
	}
	return nil
}

func (s *System) StopMusic() {
	s.music = nil
	s.rawSamples.End = s.paintedTime
}

func (s *System) CurrentMusicTrack() int {
	if s == nil || s.music == nil || s.music.track == nil {
		return 0
	}
	return s.music.activeTrack
}

func (s *System) updateMusic(endTime int) {
	if !s.started || s.music == nil || s.music.track == nil || s.dma == nil {
		return
	}
	if s.rawSamples.End < s.paintedTime {
		s.rawSamples.End = s.paintedTime
	}

	for s.music != nil && s.music.track != nil && s.rawSamples.End < endTime {
		if s.music.position >= s.music.track.samples {
			if err := s.advanceMusicTrack(); err != nil {
				s.StopMusic()
				return
			}
			if s.music == nil || s.music.track == nil {
				return
			}
			continue
		}

		neededOut := endTime - s.rawSamples.End
		inputFrames := resampleInputFrames(neededOut, s.music.track.rate, s.dma.Speed)
		if inputFrames < 1 {
			inputFrames = 1
		}

		remaining := s.music.track.samples - s.music.position
		if inputFrames > remaining {
			inputFrames = remaining
		}

		frameSize := s.music.track.channels * s.music.track.width
		start := s.music.position * frameSize
		stop := start + inputFrames*frameSize
		s.AddRawSamples(inputFrames, s.music.track.rate, s.music.track.width, s.music.track.channels, s.music.track.data[start:stop], 1)
		s.music.position += inputFrames
	}
}

func (s *System) advanceMusicTrack() error {
	if s.music == nil {
		return nil
	}
	if s.music.loopTrack <= 0 {
		s.StopMusic()
		return nil
	}
	if s.music.loopTrack == s.music.activeTrack {
		s.music.position = 0
		return nil
	}

	resolved, err := loadMusicTrack(s.music.loopTrack, s.music.loader)
	if err != nil {
		return err
	}
	s.music.track = resolved
	s.music.activeTrack = s.music.loopTrack
	s.music.position = 0
	return nil
}

func loadMusicTrack(track int, loader func(string) ([]byte, error)) (*musicTrack, error) {
	var lastErr error
	for _, ext := range supportedMusicExtensions {
		name := fmt.Sprintf("music/track%02d%s", track, ext)
		data, err := loader(name)
		if err != nil {
			lastErr = err
			continue
		}

		loaded, err := decodeMusicTrack(name, data)
		if err != nil {
			lastErr = err
			continue
		}
		return loaded, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("music/track%02d%s not found", track, supportedMusicExtensions[0])
	}
	return nil, fmt.Errorf("failed to load CD track %d: %w", track, lastErr)
}

func decodeMusicTrack(name string, data []byte) (*musicTrack, error) {
	switch strings.ToLower(name) {
	default:
		if strings.HasSuffix(strings.ToLower(name), ".wav") {
			sampleData, info, err := LoadMusicWAV(name, data)
			if err != nil {
				return nil, err
			}
			return &musicTrack{
				name:     name,
				data:     sampleData,
				samples:  info.Samples,
				rate:     info.Rate,
				width:    info.Width,
				channels: info.Channels,
			}, nil
		}
	}
	return nil, fmt.Errorf("unsupported music file type for %s", name)
}

func resampleInputFrames(outputFrames, inputRate, outputRate int) int {
	if outputFrames <= 0 || inputRate <= 0 || outputRate <= 0 {
		return 0
	}
	return (outputFrames*inputRate + outputRate - 1) / outputRate
}
