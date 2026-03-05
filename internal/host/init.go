// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/server"
)

var hostSubsystemRegistry sync.Map

// serverDatagramSource is satisfied by server.Server to expose per-frame datagrams.
type serverDatagramSource interface {
	GetClientDatagram(clientNum int) []byte
}

type localLoopbackClient struct {
	inner  *cl.Client
	parser *cl.Parser
	srv    serverDatagramSource
}

func newLocalLoopbackClient() *localLoopbackClient {
	c := cl.NewClient()
	return &localLoopbackClient{inner: c, parser: cl.NewParser(c)}
}

func (c *localLoopbackClient) Init() error {
	if c.inner == nil {
		c.inner = cl.NewClient()
		c.parser = cl.NewParser(c.inner)
	}
	c.inner.ClearState()
	return nil
}

func (c *localLoopbackClient) Frame(frameTime float64) error { return nil }
func (c *localLoopbackClient) Shutdown()                     {}

func (c *localLoopbackClient) State() ClientState {
	if c == nil || c.inner == nil {
		return caDisconnected
	}
	switch c.inner.State {
	case cl.StateConnected:
		return caConnected
	case cl.StateActive:
		return caActive
	default:
		return caDisconnected
	}
}

// ReadFromServer polls the loopback server for a client datagram and feeds it
// to the client parser. This is the M3 integration point: server→client messages.
func (c *localLoopbackClient) ReadFromServer() error {
	if c.srv == nil || c.inner.State != cl.StateActive {
		return nil
	}
	data := c.srv.GetClientDatagram(0)
	if len(data) == 0 {
		return nil
	}
	if err := c.parser.ParseServerMessage(data); err != nil {
		// Log but don't abort — a parse error on one frame shouldn't crash the loop.
		_ = err
	}
	return nil
}

func (c *localLoopbackClient) SendCommand() error { return nil }

// SetupLoopbackClientServer wires the loopback client inside subs to the given
// server so that ReadFromServer actually parses per-frame datagrams.
// Call this after constructing subs but before Host.Init.
func SetupLoopbackClientServer(subs *Subsystems, srv serverDatagramSource) {
	if subs == nil || srv == nil {
		return
	}
	// Create the client if not already set.
	if subs.Client == nil {
		subs.Client = newLocalLoopbackClient()
	}
	if lc, ok := subs.Client.(*localLoopbackClient); ok {
		lc.srv = srv
	}
}

func (c *localLoopbackClient) LocalServerInfo() error {
	c.inner.ClearState()
	c.inner.State = cl.StateDisconnected
	return c.inner.HandleServerInfo()
}

func (c *localLoopbackClient) LocalSignonReply(command string) error {
	return c.inner.HandleSignonReply(command)
}

func (c *localLoopbackClient) LocalSignon() int {
	if c == nil || c.inner == nil {
		return 0
	}
	return c.inner.Signon
}

type InitParams struct {
	BaseDir    string
	GameDir    string
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
	Init(baseDir, gameDir string) error
	Close()
	LoadFile(filename string) ([]byte, error)
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
	SpawnServer(mapName string, vfs *fs.FileSystem) error
	ConnectClient(clientNum int)
	Frame(frameTime float64) error
	Shutdown()
	SaveSpawnParms()
	GetMaxClients() int
	GetClientName(clientNum int) string
	SetClientName(clientNum int, name string)
	GetClientColor(clientNum int) int
	SetClientColor(clientNum int, color int)
	GetClientPing(clientNum int) float32
	EdictNum(n int) *server.Edict
	GetMapName() string
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
	h.gameDir = params.GameDir
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
		if err := subs.Files.Init(h.baseDir, h.gameDir); err != nil {
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
		if subs.Client == nil {
			subs.Client = newLocalLoopbackClient()
		}
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
	hostSubsystemRegistry.Store(h, subs)
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
		subs.Files.Close()
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
