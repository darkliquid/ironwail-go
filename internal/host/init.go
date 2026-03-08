// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/server"
)

var (
	hostSubsystemRegistry sync.Map
	hostCVarsOnce         sync.Once
)

const (
	clientNameCVar     = "_cl_name"
	clientColorCVar    = "_cl_color"
	serverHostnameCVar = "hostname"

	defaultClientName     = "player"
	defaultServerHostname = "UNNAMED"
)

func registerHostCVars() {
	cvar.Register("nomonsters", "0", cvar.FlagServerInfo, "Disable monster spawning for new games")
	cvar.Register(clientNameCVar, defaultClientName, cvar.FlagArchive|cvar.FlagUserInfo, "Player name")
	cvar.Register(clientColorCVar, "0", cvar.FlagArchive|cvar.FlagUserInfo, "Player shirt and pants colors")
	cvar.Register(serverHostnameCVar, defaultServerHostname, cvar.FlagServerInfo, "Server hostname")
}

// serverDatagramSource is satisfied by server.Server to expose loopback-ready
// client messages.
type serverDatagramSource interface {
	GetClientLoopbackMessage(clientNum int) []byte
}

type serverCommandSink interface {
	SubmitLoopbackCmd(clientNum int, viewAngles [3]float32, forward, side, up float32, buttons, impulse int, sentTime float64) error
	SubmitLoopbackStringCommand(clientNum int, cmd string) error
}

type localLoopbackClient struct {
	inner    *cl.Client
	parser   *cl.Parser
	srv      serverDatagramSource
	cmd      serverCommandSink
	cmdReady bool

	lastServerMessage []byte
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
	c.cmdReady = false
	c.lastServerMessage = nil
	return nil
}

func (c *localLoopbackClient) Frame(frameTime float64) error {
	if c == nil || c.inner == nil {
		return nil
	}
	if c.inner.State != cl.StateActive {
		c.cmdReady = false
		return nil
	}
	c.inner.AccumulateCmd(float32(frameTime))
	c.cmdReady = true
	return nil
}
func (c *localLoopbackClient) Shutdown() {}

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
		c.lastServerMessage = nil
		return nil
	}
	data := c.srv.GetClientLoopbackMessage(0)
	if len(data) == 0 {
		c.lastServerMessage = nil
		return nil
	}
	c.lastServerMessage = append(c.lastServerMessage[:0], data...)
	if err := c.parser.ParseServerMessage(data); err != nil {
		// Log but don't abort — a parse error on one frame shouldn't crash the loop.
		_ = err
	}
	return nil
}

func (c *localLoopbackClient) LastServerMessage() []byte {
	if len(c.lastServerMessage) == 0 {
		return nil
	}
	return append([]byte(nil), c.lastServerMessage...)
}

func (c *localLoopbackClient) SendCommand() error {
	if c == nil || c.inner == nil || c.cmd == nil || !c.cmdReady || c.inner.State != cl.StateActive {
		return nil
	}
	cmd := c.inner.PendingCmd
	if err := c.cmd.SubmitLoopbackCmd(0, cmd.ViewAngles, cmd.Forward, cmd.Side, cmd.Up, cmd.Buttons, cmd.Impulse, c.inner.Time); err != nil {
		return err
	}
	c.inner.Cmd = cmd
	c.cmdReady = false
	return nil
}

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
		if cmd, ok := srv.(serverCommandSink); ok {
			lc.cmd = cmd
		}
	}
}

func LoopbackClientState(subs *Subsystems) *cl.Client {
	if subs == nil {
		return nil
	}
	lc, ok := subs.Client.(*localLoopbackClient)
	if !ok {
		return nil
	}
	return lc.inner
}

type clientStateProvider interface {
	ClientState() *cl.Client
}

func (c *localLoopbackClient) ClientState() *cl.Client {
	if c == nil {
		return nil
	}
	return c.inner
}

func ActiveClientState(subs *Subsystems) *cl.Client {
	if subs == nil || subs.Client == nil {
		return nil
	}
	provider, ok := subs.Client.(clientStateProvider)
	if !ok {
		return nil
	}
	return provider.ClientState()
}

func DispatchLoopbackStuffText(subs *Subsystems) {
	if subs == nil || subs.Commands == nil {
		return
	}
	if c := ActiveClientState(subs); c != nil {
		if text := c.ConsumeStuffCommands(); text != "" {
			subs.Commands.AddText(text)
		}
	}
	subs.Commands.Execute()
}

func (c *localLoopbackClient) LocalServerInfo() error {
	if c == nil || c.inner == nil || c.srv == nil || c.parser == nil {
		return fmt.Errorf("loopback client not initialized")
	}
	c.inner.ClearState()
	c.inner.State = cl.StateDisconnected
	data := c.srv.GetClientLoopbackMessage(0)
	if len(data) == 0 {
		return fmt.Errorf("no loopback serverinfo available")
	}
	return c.parser.ParseServerMessage(data)
}

func (c *localLoopbackClient) LocalSignonReply(command string) error {
	if c == nil || c.inner == nil || c.cmd == nil || c.srv == nil || c.parser == nil {
		return fmt.Errorf("loopback client not initialized")
	}
	if err := c.cmd.SubmitLoopbackStringCommand(0, command); err != nil {
		return err
	}
	data := c.srv.GetClientLoopbackMessage(0)
	if len(data) == 0 {
		return fmt.Errorf("no loopback reply for %q", command)
	}
	return c.parser.ParseServerMessage(data)
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
	Input    *input.System
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
	KickClient(clientNum int, who, reason string) bool
	Frame(frameTime float64) error
	Shutdown()
	SaveSpawnParms()
	GetMaxClients() int
	IsClientActive(clientNum int) bool
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
	StopAllSounds(clear bool)
	Shutdown()
}

type Renderer interface {
	Init() error
	UpdateScreen()
	Shutdown()
}

func (h *Host) Init(params *InitParams, subs *Subsystems) error {
	hostCVarsOnce.Do(registerHostCVars)

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

	if err := h.execUserConfig(subs); err != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Warning: couldn't exec config.cfg: %v\n", err))
	}

	h.initialized = true
	hostSubsystemRegistry.Store(h, subs)
	h.realtime = currentTime()
	h.oldrealtime = h.realtime
	h.frameCount = 0

	return nil
}

func executeConfigText(subs *Subsystems, text string) {
	if text == "" {
		return
	}
	if subs != nil && subs.Commands != nil {
		subs.Commands.InsertText(text)
		subs.Commands.Execute()
		return
	}
	cmdsys.InsertText(text)
	cmdsys.Execute()
}

func (h *Host) execUserConfig(subs *Subsystems) error {
	configPath := filepath.Join(h.userDir, "config.cfg")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	executeConfigText(subs, string(data))
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
	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}

	configPath := filepath.Join(h.userDir, "config.cfg")
	f, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	wroteBindings := false
	if subs != nil && subs.Input != nil {
		for key := 0; key < input.NumKeycode; key++ {
			binding := subs.Input.GetBinding(key)
			if binding == "" {
				continue
			}
			wroteBindings = true
			keyName := input.KeyToString(key)
			if keyName == "" {
				keyName = strconv.Itoa(key)
			}
			escapedBinding := strings.ReplaceAll(binding, "\\", "\\\\")
			escapedBinding = strings.ReplaceAll(escapedBinding, "\n", "\\n")
			escapedBinding = strings.ReplaceAll(escapedBinding, "\r", "\\r")
			escapedBinding = strings.ReplaceAll(escapedBinding, "\t", "\\t")
			escapedBinding = strings.ReplaceAll(escapedBinding, "\"", "\\\"")
			fmt.Fprintf(f, "bind %s \"%s\"\n", keyName, escapedBinding)
		}
	}

	archivedVars := cvar.ArchiveVars()
	if wroteBindings && len(archivedVars) > 0 {
		fmt.Fprintln(f)
	}
	for _, line := range archivedVars {
		fmt.Fprintf(f, "%s\n", line)
	}

	if subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Wrote %s\n", configPath))
	}

	return nil
}
