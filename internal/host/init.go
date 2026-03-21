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

	"github.com/ironwail/ironwail-go/internal/audio"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/compatrand"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/server"
)

var (
	hostCVarsOnce sync.Once
)

const (
	clientNameCVar     = "_cl_name"
	clientColorCVar    = "_cl_color"
	serverHostnameCVar = "hostname"
	configFileName     = "ironwail.cfg"
	legacyConfigName   = "config.cfg"

	defaultClientName     = "player"
	defaultServerHostname = "UNNAMED"
)

func registerHostCVars() {
	cvar.Register("skill", "1", cvar.FlagArchive, "Single-player skill level")
	cvar.Register("dedicated", "0", cvar.FlagServerInfo, "Run as dedicated server")
	cvar.Register("nomonsters", "0", cvar.FlagServerInfo, "Disable monster spawning for new games")
	cvar.Register("coop", "0", cvar.FlagServerInfo, "Cooperative game mode")
	cvar.Register("deathmatch", "0", cvar.FlagServerInfo, "Deathmatch game mode")
	cvar.Register("sv_altnoclip", "1", cvar.FlagServerInfo, "Use fly-style noclip movement when enabled")
	cvar.Register("sv_freezenonclients", "0", cvar.FlagServerInfo, "Freeze non-client entities when enabled")
	cvar.Register("sv_nostep", "0", cvar.FlagServerInfo, "Disable stair-step movement retries when enabled")
	cvar.Register("fraglimit", "0", cvar.FlagNotify|cvar.FlagServerInfo, "Match frag limit")
	cvar.Register("timelimit", "0", cvar.FlagNotify|cvar.FlagServerInfo, "Match time limit in minutes")
	cvar.Register("teamplay", "0", cvar.FlagNotify|cvar.FlagServerInfo, "Teamplay rules")
	cvar.Register(clientNameCVar, defaultClientName, cvar.FlagArchive|cvar.FlagUserInfo, "Player name")
	cvar.Register(clientColorCVar, "0", cvar.FlagArchive|cvar.FlagUserInfo, "Player shirt and pants colors")
	cvar.Register(serverHostnameCVar, defaultServerHostname, cvar.FlagServerInfo, "Server hostname")
	cvar.Register("host_speeds", "0", cvar.FlagNone, "Show frame timing information")
	cvar.Register("host_autosave", "5", cvar.FlagArchive, "Autosave interval in minutes (<=0 disables)")
	cvar.Register("sv_gameplayfix_elevators", "2", cvar.FlagArchive, "Nudge entities on elevators to prevent crushing (0=off, 1=clients, 2=all)")
	audio.RegisterCVars()
	server.RegisterDebugTelemetryCVars()
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

type compatRNGSetter interface {
	SetCompatRNG(rng *compatrand.RNG)
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
	c.inner.RecordSentCmd(cmd)
	c.cmdReady = false
	return nil
}

func (c *localLoopbackClient) SendStringCmd(cmd string) error {
	if c == nil || c.cmd == nil {
		return fmt.Errorf("loopback client not initialized")
	}
	return c.cmd.SubmitLoopbackStringCommand(0, cmd)
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
	subs.Commands.ExecuteWithSource(cmdsys.SrcServer)
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
	BaseDir      string
	GameDir      string
	UserDir      string
	Args         []string
	MaxClients   int
	VersionMajor int
	VersionMinor int
	VersionPatch int
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
	LoadFirstAvailable(filenames []string) (string, []byte, error)
	FileExists(filename string) bool
}

type CommandBuffer interface {
	Init()
	Execute()
	ExecuteWithSource(source cmdsys.CommandSource)
	AddText(text string)
	InsertText(text string)
	Shutdown()
}

type Console interface {
	Init() error
	Print(msg string)
	Clear()
	Dump(filename string) error
	Shutdown()
}

type Server interface {
	Init(maxClients int) error
	SpawnServer(mapName string, vfs *fs.FileSystem) error
	ConnectClient(clientNum int)
	KillClient(clientNum int) bool
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
	SetLoadGame(v bool)
	SetPreserveSpawnParms(v bool)
}

type Client interface {
	Init() error
	Frame(frameTime float64) error
	Shutdown()
	State() ClientState
	ReadFromServer() error
	SendCommand() error
	SendStringCmd(cmd string) error
}

type Audio interface {
	Init() error
	Update(origin, velocity, forward, right, up [3]float32)
	StopAllSounds(clear bool)
	SoundInfo() string
	SoundList() string
	PlayLocalSound(name string, loader func() ([]byte, error), vol float32) error
	PlayMusic(filename string, loader func(string) ([]byte, error), resolver func([]string) (string, []byte, error)) error
	PauseMusic()
	ResumeMusic()
	SetMusicLoop(loop bool)
	ToggleMusicLoop() bool
	MusicLooping() bool
	CurrentMusic() string
	JumpMusic(order int) bool
	StopMusic()
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
	h.versionMajor = params.VersionMajor
	h.versionMinor = params.VersionMinor
	h.versionPatch = params.VersionPatch
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
		subs.Console.Print("Console initialized.\n")
	}

	if subs.Server != nil {
		if setter, ok := subs.Server.(compatRNGSetter); ok {
			setter.SetCompatRNG(h.compatRNG)
		}
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
			if subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("Warning: failed to init audio: %v\n", err))
			}
		}
	}

	if subs.Renderer != nil {
		if err := subs.Renderer.Init(); err != nil {
			return fmt.Errorf("failed to init renderer: %w", err)
		}
	}

	if subs.Console != nil {
		subs.Console.Print("\nLanguage initialization\n\n")
		subs.Console.Print("========= Quake Initialized =========\n\n")
	}

	// Execute quake.rc from the game filesystem (pak0.pak).
	// This mirrors C Ironwail's Cbuf_InsertText("exec quake.rc\n") in Host_Init.
	// quake.rc chains: exec default.cfg → exec config.cfg → exec autoexec.cfg
	//                  → stuffcmds → startdemos demo1 demo2 demo3
	//
	// If quake.rc isn't available (e.g. no PAK files in test environments),
	// fall back to directly loading the user config from userDir.
	if subs.Files != nil && subs.Files.FileExists("quake.rc") {
		executeConfigText(subs, "exec quake.rc\n")
	} else {
		if err := h.execUserConfig(subs); err != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("Warning: couldn't exec %s: %v\n", configFileName, err))
		}
	}

	h.initialized = true
	h.Subs = subs
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
	for _, name := range []string{configFileName, legacyConfigName} {
		configPath := filepath.Join(h.userDir, name)
		data, err := os.ReadFile(configPath)
		if err == nil {
			executeConfigText(subs, string(data))
			return nil
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
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
	return h.WriteConfigNamed("", subs)
}

func (h *Host) WriteConfigNamed(name string, subs *Subsystems) error {
	if !h.initialized {
		return nil
	}
	if subs == nil {
		subs = h.Subs
	}

	configName := name
	if configName == "" {
		configName = configFileName
	}
	if filepath.Ext(configName) == "" {
		configName += ".cfg"
	}

	configPath := filepath.Join(h.userDir, configName)
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
	if _, err := fmt.Fprintln(f, "vid_restart"); err != nil {
		return fmt.Errorf("failed to write vid_restart state: %w", err)
	}

	if clientState := ActiveClientState(subs); clientState != nil && (clientState.InputMLook.State&1) != 0 {
		if _, err := fmt.Fprintln(f, "+mlook"); err != nil {
			return fmt.Errorf("failed to write +mlook state: %w", err)
		}
	}

	if subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Wrote %s\n", configPath))
	}

	return nil
}
