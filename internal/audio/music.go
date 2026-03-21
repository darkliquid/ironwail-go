// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

var supportedMusicExtensions = []string{".ogg", ".opus", ".mp3", ".flac", ".wav", ".mod", ".s3m", ".xm", ".it"}

type musicResolveFunc func([]string) (string, []byte, error)

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
	loop         bool
	paused       bool
	loader       func(string) ([]byte, error)
	resolver     musicResolveFunc
	track        *musicTrack
}

func (s *System) PlayCDTrack(track, loopTrack int, loader func(string) ([]byte, error), resolvers ...musicResolveFunc) error {
	if track <= 0 {
		s.StopMusic()
		return nil
	}
	var resolver musicResolveFunc
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	if loader == nil && resolver == nil {
		s.StopMusic()
		return fmt.Errorf("music loader is not available")
	}
	if loopTrack == 0 {
		loopTrack = track
	}
	if s.music != nil && s.music.requestTrack == track && s.music.loopTrack == loopTrack {
		return nil
	}

	resolved, err := loadMusicTrack(track, loader, resolver)
	if err != nil {
		s.StopMusic()
		return err
	}

	s.music = &musicState{
		requestTrack: track,
		loopTrack:    loopTrack,
		activeTrack:  track,
		loop:         loopTrack > 0,
		loader:       loader,
		resolver:     resolver,
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

func (s *System) CurrentMusic() string {
	if s == nil || s.music == nil || s.music.track == nil {
		return ""
	}
	return s.music.track.name
}

func (s *System) PauseMusic() {
	if s == nil || s.music == nil {
		return
	}
	s.music.paused = true
}

func (s *System) ResumeMusic() {
	if s == nil || s.music == nil {
		return
	}
	s.music.paused = false
}

func (s *System) SetMusicLoop(loop bool) {
	s.musicLoop = loop
	if s.music != nil && s.music.requestTrack == 0 {
		s.music.loop = loop
	}
}

func (s *System) ToggleMusicLoop() bool {
	s.SetMusicLoop(!s.musicLoop)
	return s.musicLoop
}

func (s *System) MusicLooping() bool {
	return s.musicLoop
}

func (s *System) JumpMusic(order int) bool {
	_ = order
	return false
}

func (s *System) PlayMusic(filename string, loader func(string) ([]byte, error), resolvers ...musicResolveFunc) error {
	name := normalizeMusicFilename(filename)
	if name == "" {
		s.StopMusic()
		return fmt.Errorf("music file name is empty")
	}
	var resolver musicResolveFunc
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	resolved, err := loadMusicFile(name, loader, resolver)
	if err != nil {
		s.StopMusic()
		return err
	}
	s.music = &musicState{
		loop:     s.musicLoop,
		loader:   loader,
		resolver: resolver,
		track:    resolved,
	}
	if s.rawSamples.End < s.paintedTime {
		s.rawSamples.End = s.paintedTime
	}
	return nil
}

func (s *System) updateMusic(endTime int) {
	if !s.started || s.music == nil || s.music.track == nil || s.dma == nil || s.music.paused {
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
	if s.music.requestTrack == 0 {
		if s.music.loop {
			s.music.position = 0
			return nil
		}
		s.StopMusic()
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

	resolved, err := loadMusicTrack(s.music.loopTrack, s.music.loader, s.music.resolver)
	if err != nil {
		return err
	}
	s.music.track = resolved
	s.music.activeTrack = s.music.loopTrack
	s.music.position = 0
	return nil
}

func normalizeMusicFilename(filename string) string {
	filename = strings.TrimSpace(strings.TrimPrefix(strings.ReplaceAll(filename, "\\", "/"), "./"))
	return strings.TrimPrefix(filename, "music/")
}

func musicFileCandidates(filename string) []string {
	filename = normalizeMusicFilename(filename)
	if filename == "" {
		return nil
	}
	if ext := strings.ToLower(filepath.Ext(filename)); ext != "" {
		return []string{path.Join("music", filename)}
	}
	candidates := make([]string, 0, len(supportedMusicExtensions))
	for _, ext := range supportedMusicExtensions {
		candidates = append(candidates, path.Join("music", filename+ext))
	}
	return candidates
}

func loadMusicFile(filename string, loader func(string) ([]byte, error), resolver musicResolveFunc) (*musicTrack, error) {
	candidates := musicFileCandidates(filename)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("music file name is empty")
	}
	if filepath.Ext(filename) != "" {
		if loader == nil {
			return nil, fmt.Errorf("music loader is not available")
		}
		data, err := loader(candidates[0])
		if err != nil {
			return nil, err
		}
		return decodeMusicTrack(candidates[0], data)
	}
	if resolver != nil {
		name, data, err := resolver(candidates)
		if err != nil {
			return nil, err
		}
		return decodeMusicTrack(name, data)
	}
	if loader == nil {
		return nil, fmt.Errorf("music loader is not available")
	}
	var lastErr error
	for _, name := range candidates {
		data, err := loader(name)
		if err != nil {
			lastErr = err
			continue
		}
		return decodeMusicTrack(name, data)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%s not found", candidates[0])
	}
	return nil, lastErr
}

func musicTrackCandidates(track int) []string {
	candidates := make([]string, 0, len(supportedMusicExtensions))
	for _, ext := range supportedMusicExtensions {
		candidates = append(candidates, fmt.Sprintf("music/track%02d%s", track, ext))
	}
	return candidates
}

func loadMusicTrack(track int, loader func(string) ([]byte, error), resolver musicResolveFunc) (*musicTrack, error) {
	candidates := musicTrackCandidates(track)
	if resolver != nil {
		name, data, err := resolver(candidates)
		if err != nil {
			return nil, fmt.Errorf("failed to load CD track %d: %w", track, err)
		}
		loaded, err := decodeMusicTrack(name, data)
		if err != nil {
			return nil, fmt.Errorf("failed to load CD track %d: %w", track, err)
		}
		return loaded, nil
	}

	if loader == nil {
		return nil, fmt.Errorf("failed to load CD track %d: music loader is not available", track)
	}

	var lastErr error
	for _, name := range candidates {
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
		lastErr = fmt.Errorf("%s not found", candidates[0])
	}
	return nil, fmt.Errorf("failed to load CD track %d: %w", track, lastErr)
}

func decodeMusicTrack(name string, data []byte) (*musicTrack, error) {
	lowerName := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lowerName, ".wav"):
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
	case strings.HasSuffix(lowerName, ".ogg"):
		return decodeMusicOGG(name, data)
	case strings.HasSuffix(lowerName, ".mp3"):
		return decodeMusicMP3(name, data)
	case strings.HasSuffix(lowerName, ".opus"):
		return decodeMusicOpus(name, data)
	case strings.HasSuffix(lowerName, ".flac"):
		return decodeMusicFLAC(name, data)
	case strings.HasSuffix(lowerName, ".mod"),
		strings.HasSuffix(lowerName, ".s3m"),
		strings.HasSuffix(lowerName, ".xm"),
		strings.HasSuffix(lowerName, ".it"):
		return decodeMusicTracker(name, data)
	}
	return nil, fmt.Errorf("unsupported music file type for %s", name)
}

func resampleInputFrames(outputFrames, inputRate, outputRate int) int {
	if outputFrames <= 0 || inputRate <= 0 || outputRate <= 0 {
		return 0
	}
	return (outputFrames*inputRate + outputRate - 1) / outputRate
}
