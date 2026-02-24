// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/ironwail/ironwail-go/internal/cvar"
)


type InitParams struct {
	BaseDir    string
	UserDir    string
	Args       []string
	MaxClients int
}

type Subsystems struct {
	Files    Filesystem
	Commands CommandBuffer
	Console  Console
	Server   Server
	Client   Client
	Audio    Audio
	Renderer Renderer
}

type Filesystem interface {
	Init(baseDir string) error
	Shutdown()
}

type CommandBuffer interface {
	Init()
	Execute()
	AddText(text string)
	InsertText(text string)
	Shutdown()
}

type Console interface {
	Init() error
	Print(msg string)
	Shutdown()
}

type Server interface {
	Init(maxClients int) error
	Frame(frameTime float64) error
	Shutdown()
	IsActive() bool
	IsPaused() bool
}

type Client interface {
	Init() error
	Frame(frameTime float64) error
	Shutdown()
	State() ClientState
	ReadFromServer() error
	SendCommand() error
}

type Audio interface {
	Init() error
	Update(origin, forward, right, up [3]float32)
	Shutdown()
}

type Renderer interface {
	Init() error
	UpdateScreen()
	Shutdown()
}

func (h *Host) Init(params *InitParams, subs *Subsystems) error {
	h.baseDir = params.BaseDir
	h.userDir = params.UserDir
	h.args = params.Args
	h.maxClients = params.MaxClients
	if h.maxClients < 1 {
		h.maxClients = 1
	}

	if h.baseDir == "" {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		h.baseDir = dir
	}

	if h.userDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			h.userDir = h.baseDir
		} else {
			h.userDir = filepath.Join(homeDir, ".ironwail")
		}
	}

	if err := os.MkdirAll(h.userDir, 0755); err != nil {
		return fmt.Errorf("failed to create user directory: %w", err)
	}

	if subs.Files != nil {
		if err := subs.Files.Init(h.baseDir); err != nil {
			return fmt.Errorf("failed to init filesystem: %w", err)
		}
	}

if subs.Commands != nil {
		subs.Commands.Init()
		h.RegisterCommands(subs)
}

	if subs.Console != nil {
		if err := subs.Console.Init(); err != nil {
			return fmt.Errorf("failed to init console: %w", err)
		}
	}

	if subs.Server != nil {
		if err := subs.Server.Init(h.maxClients); err != nil {
			return fmt.Errorf("failed to init server: %w", err)
		}
	}

	if subs.Client != nil {
		if err := subs.Client.Init(); err != nil {
			return fmt.Errorf("failed to init client: %w", err)
		}
	}

	if subs.Audio != nil {
		if err := subs.Audio.Init(); err != nil {
			subs.Console.Print(fmt.Sprintf("Warning: failed to init audio: %v\n", err))
		}
	}

	if subs.Renderer != nil {
		if err := subs.Renderer.Init(); err != nil {
			return fmt.Errorf("failed to init renderer: %w", err)
		}
	}

	h.initialized = true
	h.realtime = currentTime()
	h.oldrealtime = h.realtime
	h.frameCount = 0

	return nil
}

func (h *Host) Shutdown(subs *Subsystems) {
	if !h.initialized {
		return
	}

	h.initialized = false

	if subs.Renderer != nil {
		subs.Renderer.Shutdown()
	}
	if subs.Audio != nil {
		subs.Audio.Shutdown()
	}
	if subs.Client != nil {
		subs.Client.Shutdown()
	}
	if subs.Server != nil {
		subs.Server.Shutdown()
	}
	if subs.Console != nil {
		subs.Console.Shutdown()
	}
	if subs.Commands != nil {
		subs.Commands.Shutdown()
	}
	if subs.Files != nil {
		subs.Files.Shutdown()
	}
}

func (h *Host) WriteConfig(subs *Subsystems) error {
	if !h.initialized {
	return nil
	}

	configPath := filepath.Join(h.userDir, "config.cfg")
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	// Write archived cvars
	for _, line := range cvar.ArchiveVars() {
		fmt.Fprintf(f, "%s\n", line)
	}

	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Wrote %s\n", configPath))
	}

	return nil
}

