// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func normalizeConsoleSoundName(name string) string {
	name = strings.TrimSpace(strings.TrimPrefix(name, "sound/"))
	if name == "" {
		return ""
	}
	if filepath.Ext(name) == "" {
		name += ".wav"
	}
	return path.Clean(name)
}

func (h *Host) CmdPlay(args []string, subs *Subsystems) {
	if subs == nil || subs.Audio == nil || subs.Files == nil {
		return
	}
	for _, arg := range args {
		name := normalizeConsoleSoundName(arg)
		if name == "" {
			continue
		}
		_ = subs.Audio.PlayLocalSound(name, func() ([]byte, error) {
			return subs.Files.LoadFile(path.Join("sound", name))
		}, 1.0)
	}
}

func (h *Host) CmdPlayVol(args []string, subs *Subsystems) {
	if subs == nil || subs.Audio == nil || subs.Files == nil {
		return
	}
	if len(args)%2 != 0 {
		if subs.Console != nil {
			subs.Console.Print("usage: playvol <sound> <vol> [sound vol] ...\n")
		}
		return
	}
	for i := 0; i < len(args); i += 2 {
		name := normalizeConsoleSoundName(args[i])
		if name == "" {
			continue
		}
		vol, err := strconv.ParseFloat(args[i+1], 32)
		if err != nil {
			if subs.Console != nil {
				subs.Console.Print("usage: playvol <sound> <vol> [sound vol] ...\n")
			}
			return
		}
		_ = subs.Audio.PlayLocalSound(name, func() ([]byte, error) {
			return subs.Files.LoadFile(path.Join("sound", name))
		}, float32(vol))
	}
}

func (h *Host) CmdStopsound(subs *Subsystems) {
	if subs == nil || subs.Audio == nil {
		return
	}
	subs.Audio.StopAllSounds(true)
}

func (h *Host) CmdSoundlist(subs *Subsystems) {
	if subs == nil || subs.Audio == nil || subs.Console == nil {
		return
	}
	subs.Console.Print(subs.Audio.SoundList())
}

func (h *Host) CmdMusic(args []string, subs *Subsystems) {
	if subs == nil || subs.Audio == nil || subs.Console == nil {
		return
	}
	if len(args) == 0 {
		current := subs.Audio.CurrentMusic()
		if current != "" {
			name := strings.TrimSuffix(path.Base(current), path.Ext(current))
			subs.Console.Print(fmt.Sprintf("Playing %s, use 'music <musicfile>' to change\n", name))
			return
		}
		subs.Console.Print("music <musicfile>\n")
		return
	}
	if subs.Files == nil {
		subs.Console.Print(fmt.Sprintf("Couldn't handle music file %s\n", args[0]))
		return
	}
	if err := subs.Audio.PlayMusic(args[0], func(name string) ([]byte, error) {
		return subs.Files.LoadFile(name)
	}, func(candidates []string) (string, []byte, error) {
		return subs.Files.LoadFirstAvailable(candidates)
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unhandled extension") {
			subs.Console.Print(fmt.Sprintf("Unhandled extension for %s\n", args[0]))
			return
		}
		subs.Console.Print(fmt.Sprintf("Couldn't handle music file %s\n", args[0]))
	}
}

func (h *Host) CmdMusicPause(subs *Subsystems) {
	if subs == nil || subs.Audio == nil {
		return
	}
	subs.Audio.PauseMusic()
}

func (h *Host) CmdMusicResume(subs *Subsystems) {
	if subs == nil || subs.Audio == nil {
		return
	}
	subs.Audio.ResumeMusic()
}

func (h *Host) CmdMusicLoop(args []string, subs *Subsystems) {
	if subs == nil || subs.Audio == nil || subs.Console == nil {
		return
	}
	if len(args) == 1 {
		switch strings.ToLower(args[0]) {
		case "0", "off":
			subs.Audio.SetMusicLoop(false)
		case "1", "on":
			subs.Audio.SetMusicLoop(true)
		case "toggle":
			subs.Audio.ToggleMusicLoop()
		}
	}
	if subs.Audio.MusicLooping() {
		subs.Console.Print("Music will be looped\n")
		return
	}
	subs.Console.Print("Music will not be looped\n")
}

func (h *Host) CmdMusicStop(subs *Subsystems) {
	if subs == nil || subs.Audio == nil {
		return
	}
	subs.Audio.StopMusic()
}

func (h *Host) CmdMusicJump(args []string, subs *Subsystems) {
	if subs == nil || subs.Audio == nil || subs.Console == nil {
		return
	}
	if len(args) != 1 {
		subs.Console.Print("music_jump <ordernum>\n")
		return
	}
	order, err := strconv.Atoi(args[0])
	if err != nil {
		subs.Console.Print("music_jump <ordernum>\n")
		return
	}
	subs.Audio.JumpMusic(order)
}
