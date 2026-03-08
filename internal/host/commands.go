// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/client"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/server"
)

var saveNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

const maxAliasName = 32

type hostSaveFile struct {
	Version int                   `json:"version"`
	Skill   int                   `json:"skill"`
	Server  *server.SaveGameState `json:"server"`
}

type handshakeClient interface {
	Client
	LocalServerInfo() error
	LocalSignonReply(command string) error
	LocalSignon() int
}

func (h *Host) RegisterCommands(subs *Subsystems) {
	cmdsys.AddCommand("quit", func(args []string) { h.CmdQuit() }, "Exit game")
	cmdsys.AddCommand("map", func(args []string) {
		if len(args) > 0 {
			h.CmdMap(args[0], subs)
		}
	}, "Start a new map")
	cmdsys.AddCommand("skill", func(args []string) {
		if len(args) > 0 {
			var skill int
			fmt.Sscanf(args[0], "%d", &skill)
			h.CmdSkill(skill)
		}
	}, "Set game skill level (0-3)")
	cmdsys.AddCommand("pause", func(args []string) { h.CmdPause() }, "Pause game")
	cmdsys.AddCommand("status", func(args []string) { h.CmdStatus(subs) }, "Show server status")
	cmdsys.AddCommand("mapname", func(args []string) { h.CmdMapname(subs) }, "Show current map name")
	cmdsys.AddCommand("god", func(args []string) { h.CmdGod(subs) }, "Toggle god mode")
	cmdsys.AddCommand("noclip", func(args []string) { h.CmdNoClip(subs) }, "Toggle noclip mode")
	cmdsys.AddCommand("fly", func(args []string) { h.CmdFly(subs) }, "Toggle fly mode")
	cmdsys.AddCommand("notarget", func(args []string) { h.CmdNotarget(subs) }, "Toggle notarget mode")
	cmdsys.AddCommand("say", func(args []string) {
		if len(args) > 0 {
			h.CmdSay(strings.Join(args, " "), subs)
		}
	}, "Send a message to all players")
	cmdsys.AddCommand("serverinfo", func(args []string) { h.CmdServerInfo(subs) }, "Show server information")
	cmdsys.AddCommand("restart", func(args []string) { h.CmdRestart(subs) }, "Restart current map")
	cmdsys.AddCommand("changelevel", func(args []string) {
		if len(args) > 0 {
			h.CmdChangelevel(args[0], subs)
		}
	}, "Change to a new level")
	cmdsys.AddCommand("connect", func(args []string) {
		if len(args) > 0 {
			h.CmdConnect(args[0], subs)
		}
	}, "Connect to a server")
	cmdsys.AddCommand("disconnect", func(args []string) { h.CmdDisconnect(subs) }, "Disconnect from current server")
	cmdsys.AddCommand("reconnect", func(args []string) { h.CmdReconnect(subs) }, "Reconnect to current server")
	cmdsys.AddCommand("name", func(args []string) {
		if len(args) > 0 {
			h.CmdName(args[0], subs)
		}
	}, "Set player name")
	cmdsys.AddCommand("color", func(args []string) {
		if len(args) > 0 {
			h.CmdColor(args, subs)
		}
	}, "Set player color")
	cmdsys.AddCommand("kill", func(args []string) { h.CmdKill(subs) }, "Suicide")
	cmdsys.AddCommand("spawn", func(args []string) { h.CmdSpawn(subs) }, "Spawn into game")
	cmdsys.AddCommand("begin", func(args []string) { h.CmdBegin(subs) }, "Begin game")
	cmdsys.AddCommand("prespawn", func(args []string) { h.CmdPreSpawn(subs) }, "Pre-spawn handshake")
	cmdsys.AddCommand("kick", func(args []string) {
		h.CmdKick(args, subs)
	}, "Kick a player from the server")
	cmdsys.AddCommand("ping", func(args []string) { h.CmdPing(subs) }, "Show player pings")
	cmdsys.AddCommand("load", func(args []string) {
		if len(args) > 0 {
			h.CmdLoad(args[0], subs)
		}
	}, "Load a saved game")
	cmdsys.AddCommand("save", func(args []string) {
		if len(args) > 0 {
			h.CmdSave(args[0], subs)
		}
	}, "Save current game")
	cmdsys.AddCommand("give", func(args []string) {
		if len(args) > 1 {
			h.CmdGive(args[0], args[1], subs)
		}
	}, "Give items/ammo")

	// Demo commands
	cmdsys.AddCommand("record", func(args []string) {
		if len(args) > 0 {
			h.CmdRecord(args[0], subs)
		}
	}, "Start recording a demo")
	cmdsys.AddCommand("stop", func(args []string) {
		h.CmdStop(subs)
	}, "Stop recording a demo")
	cmdsys.AddCommand("playdemo", func(args []string) {
		if len(args) > 0 {
			h.CmdPlaydemo(args[0], subs)
		}
	}, "Play a demo")
	cmdsys.AddCommand("stopdemo", func(args []string) {
		h.CmdStopdemo(subs)
	}, "Stop demo playback")

	// Menu commands
	cmdsys.AddCommand("togglemenu", func(args []string) {
		h.CmdToggleMenu()
	}, "Toggle the main menu")
	cmdsys.AddCommand("menu_main", func(args []string) {
		h.CmdMenuMain()
	}, "Show the main menu")
	cmdsys.AddCommand("menu_quit", func(args []string) {
		h.CmdMenuQuit()
	}, "Show the quit confirmation")
	cmdsys.AddCommand("exec", func(args []string) {
		if len(args) > 0 {
			h.CmdExec(args[0], subs)
			return
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: exec <filename>\n")
		}
	}, "Execute a script file")
	cmdsys.AddCommand("alias", func(args []string) {
		h.CmdAlias(args, subs)
	}, "Create, list, and inspect command aliases")
	cmdsys.AddCommand("unalias", func(args []string) {
		h.CmdUnalias(args, subs)
	}, "Delete a command alias")
	cmdsys.AddCommand("unaliasall", func(args []string) {
		h.CmdUnaliasAll()
	}, "Delete all command aliases")
	cmdsys.AddCommand("saveconfig", func(args []string) {
		if err := h.WriteConfig(subs); err != nil && subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("saveconfig failed: %v\n", err))
		}
	}, "Write config.cfg")
}

func (h *Host) CmdQuit() {
	h.Abort("quit")
}

func (h *Host) CmdExec(filename string, subs *Subsystems) {
	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}

	if filename == "" {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: exec <filename>\n")
		}
		return
	}

	var (
		data []byte
		err  error
	)
	switch {
	case filepath.IsAbs(filename):
		data, err = os.ReadFile(filename)
	case h.userDir != "":
		data, err = os.ReadFile(filepath.Join(h.userDir, filename))
	default:
		err = os.ErrNotExist
	}
	if err == nil {
		executeConfigText(subs, string(data))
		return
	}
	if err != nil && !os.IsNotExist(err) {
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("couldn't exec %s: %v\n", filename, err))
		}
		return
	}
	if subs != nil && subs.Files != nil {
		data, err = subs.Files.LoadFile(filename)
		if err == nil {
			executeConfigText(subs, string(data))
			return
		}
	}
	if subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("couldn't exec %s\n", filename))
	}
}

func (h *Host) CmdAlias(args []string, subs *Subsystems) {
	switch len(args) {
	case 0:
		aliases := cmdsys.Aliases()
		if len(aliases) == 0 {
			if subs != nil && subs.Console != nil {
				subs.Console.Print("no alias commands found\n")
			}
			return
		}
		count := 0
		for name, value := range aliases {
			if subs != nil && subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("   %s: %s", name, value))
			}
			count++
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("%d alias command(s)\n", count))
		}
	case 1:
		if value, ok := cmdsys.Alias(args[0]); ok {
			if subs != nil && subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("   %s: %s", strings.ToLower(args[0]), value))
			}
		}
	default:
		name := args[0]
		if len(name) >= maxAliasName {
			if subs != nil && subs.Console != nil {
				subs.Console.Print("Alias name is too long\n")
			}
			return
		}
		command := strings.Join(args[1:], " ") + "\n"
		cmdsys.AddAlias(name, command)
	}
}

func (h *Host) CmdUnalias(args []string, subs *Subsystems) {
	if len(args) != 1 {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("unalias <name> : delete alias\n")
		}
		return
	}
	if !cmdsys.RemoveAlias(args[0]) {
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("No alias named %s\n", args[0]))
		}
	}
}

func (h *Host) CmdUnaliasAll() {
	cmdsys.UnaliasAll()
}

func (h *Host) CmdMap(mapName string, subs *Subsystems) error {
	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}
	if subs == nil {
		fallbackClient := newLocalLoopbackClient()
		if err := fallbackClient.Init(); err != nil {
			return fmt.Errorf("subsystems not initialized")
		}
		if err := fallbackClient.LocalServerInfo(); err != nil {
			return err
		}
		if err := fallbackClient.LocalSignonReply("prespawn"); err != nil {
			return err
		}
		if err := fallbackClient.LocalSignonReply("spawn"); err != nil {
			return err
		}
		if err := fallbackClient.LocalSignonReply("begin"); err != nil {
			return err
		}
		h.signOns = fallbackClient.LocalSignon()
		h.clientState = fallbackClient.State()
		h.serverActive = false
		return nil
	}
	if subs.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	h.stopSessionSounds(subs)
	h.clientState = caDisconnected
	h.serverActive = false

	if err := subs.Server.Init(h.maxClients); err != nil {
		return fmt.Errorf("failed to init server for map %s: %w", mapName, err)
	}

	if fsInstance, ok := subs.Files.(*fs.FileSystem); ok {
		if err := subs.Server.SpawnServer(mapName, fsInstance); err != nil {
			return fmt.Errorf("failed to spawn server for map %s: %w", mapName, err)
		}
	} else {
		return fmt.Errorf("filesystem implementation is missing")
	}

	h.serverActive = true

	return h.startLocalServerSession(subs, nil)
}

func (h *Host) runLocalHandshakeStep(step string, subs *Subsystems) error {
	if subs == nil || subs.Client == nil {
		return fmt.Errorf("client not initialized")
	}
	handshake, ok := subs.Client.(handshakeClient)
	if !ok {
		return fmt.Errorf("client does not support %s handshake", step)
	}
	if err := handshake.LocalSignonReply(step); err != nil {
		return fmt.Errorf("%s handshake failed: %w", step, err)
	}
	h.signOns = handshake.LocalSignon()
	h.clientState = handshake.State()
	return nil
}

func (h *Host) CmdSkill(skill int) {
	if skill < 0 {
		skill = 0
	}
	if skill > 3 {
		skill = 3
	}
	h.currentSkill = skill
}

func (h *Host) CmdPause() {
	if h.serverActive && h.maxClients == 1 {
		h.serverPaused = !h.serverPaused
	}
}

func (h *Host) CmdGod(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) ^ server.FlagGodMode)
}

func (h *Host) CmdNoClip(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	if server.MoveType(ent.Vars.MoveType) == server.MoveTypeNoClip {
		ent.Vars.MoveType = float32(server.MoveTypeWalk)
	} else {
		ent.Vars.MoveType = float32(server.MoveTypeNoClip)
	}
}

func (h *Host) CmdFly(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	if server.MoveType(ent.Vars.MoveType) == server.MoveTypeFly {
		ent.Vars.MoveType = float32(server.MoveTypeWalk)
	} else {
		ent.Vars.MoveType = float32(server.MoveTypeFly)
	}
}

func (h *Host) CmdNotarget(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) ^ server.FlagNoTarget)
}

func (h *Host) CmdStatus(subs *Subsystems) {
	if subs.Console == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("host:    Ironwail Go %v\n", Version))
	sb.WriteString(fmt.Sprintf("map:     active=%v\n", h.serverActive))
	sb.WriteString(fmt.Sprintf("players: %d active (%d max)\n", 0, h.maxClients))

	subs.Console.Print(sb.String())
}

func (h *Host) CmdMapname(subs *Subsystems) {
	if subs.Console == nil {
		return
	}

	if h.serverActive {
		subs.Console.Print("mapname: (server active)\n")
	} else if h.clientState == caConnected {
		subs.Console.Print("mapname: (connected)\n")
	} else {
		subs.Console.Print("no map loaded\n")
	}
}

func (h *Host) CmdKick(args []string, subs *Subsystems) {
	if !h.serverActive || subs == nil || subs.Server == nil || len(args) == 0 {
		return
	}

	target := -1
	reasonStart := 1

	if len(args) > 1 && args[0] == "#" {
		slot, err := strconv.Atoi(args[1])
		if err != nil || slot <= 0 {
			return
		}
		target = slot - 1
		reasonStart = 2
		if !subs.Server.IsClientActive(target) {
			return
		}
	} else {
		for i := 0; i < subs.Server.GetMaxClients(); i++ {
			if !subs.Server.IsClientActive(i) {
				continue
			}
			if strings.EqualFold(subs.Server.GetClientName(i), args[0]) {
				target = i
				break
			}
		}
	}

	if target < 0 || target == 0 {
		return
	}

	who := subs.Server.GetClientName(0)
	if who == "" {
		who = "Console"
	}

	var reason string
	if len(args) > reasonStart {
		reason = strings.Join(args[reasonStart:], " ")
	}
	subs.Server.KickClient(target, who, reason)
}

func (h *Host) CmdSay(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("say: %s\n", message))
	}
}

func (h *Host) CmdServerInfo(subs *Subsystems) {
	if subs.Console == nil {
		return
	}

	subs.Console.Print(fmt.Sprintf("Server info:\n"))
	subs.Console.Print(fmt.Sprintf("  active:    %v\n", h.serverActive))
	subs.Console.Print(fmt.Sprintf("  paused:    %v\n", h.serverPaused))
	subs.Console.Print(fmt.Sprintf("  maxclients: %d\n", h.maxClients))
	subs.Console.Print(fmt.Sprintf("  skill:     %d\n", h.currentSkill))
}

func (h *Host) EndGame(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Host_EndGame: %s\n", message))
	}

	if h.serverActive {
		h.ShutdownServer(subs)
	}

	h.clientState = caDisconnected
	h.Abort(message)
}

func (h *Host) ShutdownServer(subs *Subsystems) {
	if !h.serverActive {
		return
	}

	h.serverActive = false
	h.serverPaused = false

	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}

	if subs != nil && subs.Server != nil {
		subs.Server.Shutdown()
	}

	if subs != nil && subs.Client != nil {
	}
}

func (h *Host) CmdRestart(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	h.CmdMap(subs.Server.GetMapName(), subs)
}

func (h *Host) CmdChangelevel(level string, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	subs.Server.SaveSpawnParms()
	if fsInstance, ok := subs.Files.(*fs.FileSystem); ok {
		if err := subs.Server.SpawnServer(level, fsInstance); err != nil {
			h.Error(fmt.Sprintf("failed to change level to %s: %v", level, err), subs)
		}
	}
}

func (h *Host) CmdConnect(address string, subs *Subsystems) {
	h.SetDemoNum(-1)
	address = strings.TrimSpace(address)
	if address == "" {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: connect <server>\n")
		}
		return
	}

	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}

	isLocal := strings.EqualFold(address, "local")
	if isLocal && h.serverActive && subs != nil && subs.Server != nil {
		h.disconnectCurrentSession(subs, false)
		h.CmdReconnect(subs)
		return
	}

	h.disconnectCurrentSession(subs, true)

	if isLocal {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("No local server is active.\n")
		}
		return
	}

	if subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("connect %q: remote multiplayer connect is not implemented yet.\n", address))
	}
}

func (h *Host) CmdDisconnect(subs *Subsystems) {
	h.disconnectCurrentSession(subs, true)
	if subs != nil && subs.Console != nil {
		subs.Console.Print("Disconnected.\n")
	}
}

func (h *Host) disconnectCurrentSession(subs *Subsystems, stopServer bool) {
	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}

	h.stopSessionSounds(subs)

	if h.demoState != nil && h.demoState.Playback {
		if err := h.demoState.StopPlayback(); err != nil && subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("Error stopping demo playback: %v\n", err))
		}
	}

	if stopServer && h.serverActive {
		h.ShutdownServer(subs)
	}

	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		loopbackClient.ClearState()
		loopbackClient.State = cl.StateDisconnected
	}

	h.signOns = 0
	h.clientState = caDisconnected
}

func (h *Host) CmdReconnect(subs *Subsystems) {
	if h.demoState != nil && h.demoState.Playback {
		return
	}

	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}
	if subs == nil || subs.Client == nil {
		return
	}

	h.BeginLoadingPlaque(0)
	h.stopSessionSounds(subs)

	if h.serverActive && subs.Server != nil {
		if err := h.startLocalServerSession(subs, nil); err != nil {
			if subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("reconnect failed: %v\n", err))
			}
		}
		return
	}

	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		loopbackClient.ClearSignons()
		if loopbackClient.State != cl.StateDisconnected {
			loopbackClient.State = cl.StateConnected
		}
	}

	h.signOns = 0
	if h.clientState != caDisconnected {
		h.clientState = caConnected
	}
}

func (h *Host) CmdName(name string, subs *Subsystems) {
	if subs.Server != nil {
		subs.Server.SetClientName(0, name)
	}
}

func (h *Host) CmdColor(args []string, subs *Subsystems) {
	if subs.Server == nil || len(args) == 0 {
		return
	}

	if len(args) == 1 {
		var color int
		fmt.Sscanf(args[0], "%d", &color)
		subs.Server.SetClientColor(0, color)
		return
	}

	var top, bottom int
	fmt.Sscanf(args[0], "%d", &top)
	fmt.Sscanf(args[1], "%d", &bottom)
	top &= 15
	bottom &= 15
	if top > 13 {
		top = 13
	}
	if bottom > 13 {
		bottom = 13
	}
	subs.Server.SetClientColor(0, top*16+bottom)
}

func (h *Host) CmdKill(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Health = 0
}

func (h *Host) CmdSpawn(subs *Subsystems) {
	if err := h.runLocalHandshakeStep("spawn", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("spawn failed: %v\n", err))
	}
}

func (h *Host) CmdBegin(subs *Subsystems) {
	if err := h.runLocalHandshakeStep("begin", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("begin failed: %v\n", err))
	}
}

func (h *Host) CmdPreSpawn(subs *Subsystems) {
	if err := h.runLocalHandshakeStep("prespawn", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("prespawn failed: %v\n", err))
	}
}

func (h *Host) CmdPing(subs *Subsystems) {
	if subs.Server == nil || subs.Console == nil {
		return
	}
	maxClients := subs.Server.GetMaxClients()
	subs.Console.Print("Client pings:\n")
	for i := 0; i < maxClients; i++ {
		name := subs.Server.GetClientName(i)
		if name == "" {
			continue
		}
		ping := subs.Server.GetClientPing(i)
		subs.Console.Print(fmt.Sprintf("  %d: %-16s %.0f ms\n", i, name, ping))
	}
}

func (h *Host) CmdLoad(name string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	path, err := h.saveFilePath(name)
	if err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	var save hostSaveFile
	if err := json.Unmarshal(data, &save); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	if save.Server == nil {
		subs.Console.Print("load failed: savegame is missing server state\n")
		return
	}
	if subs.Server == nil {
		subs.Console.Print("load failed: server is not initialized\n")
		return
	}
	if subs.Server.GetMaxClients() != 1 {
		subs.Console.Print("load failed: savegames require single-player mode\n")
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		subs.Console.Print("load failed: savegames require the built-in server\n")
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		subs.Console.Print("load failed: filesystem implementation is missing\n")
		return
	}

	h.BeginLoadingPlaque(0)
	h.stopSessionSounds(subs)
	h.serverActive = false
	h.clientState = caDisconnected
	h.signOns = 0

	if err := subs.Server.Init(h.maxClients); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	srv.LoadGame = true
	defer func() { srv.LoadGame = false }()
	if err := subs.Server.SpawnServer(save.Server.MapName, fsInstance); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	if err := h.startLocalServerSession(subs, func() error {
		if err := srv.RestoreSaveGameState(save.Server); err != nil {
			return err
		}
		h.currentSkill = save.Skill
		return nil
	}); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}

	subs.Console.Print(fmt.Sprintf("Loaded %s\n", filepath.Base(path)))
}

func (h *Host) CmdSave(name string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	path, err := h.saveFilePath(name)
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	if subs.Server == nil || !h.serverActive || !subs.Server.IsActive() {
		subs.Console.Print("save failed: no active game\n")
		return
	}
	if subs.Server.GetMaxClients() != 1 {
		subs.Console.Print("save failed: savegames require single-player mode\n")
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		subs.Console.Print("save failed: savegames require the built-in server\n")
		return
	}
	if cvar.BoolValue("nomonsters") {
		subs.Console.Print("Can't save when using \"nomonsters\".\n")
		return
	}
	if clientState := LoopbackClientState(subs); clientState != nil && clientState.Intermission != 0 {
		subs.Console.Print("Can't save in intermission.\n")
		return
	}
	if srv.Static != nil {
		for _, client := range srv.Static.Clients {
			if client == nil || !client.Active || client.Edict == nil || client.Edict.Vars == nil {
				continue
			}
			if client.Edict.Vars.Health <= 0 {
				subs.Console.Print("Can't savegame with a dead player\n")
				return
			}
		}
	}
	state, err := srv.CaptureSaveGameState()
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	save := hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   h.currentSkill,
		Server:  state,
	}
	data, err := json.MarshalIndent(save, "", "  ")
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}

	subs.Console.Print(fmt.Sprintf("Saved %s\n", filepath.Base(path)))
}

func (h *Host) startLocalServerSession(subs *Subsystems, afterConnect func() error) error {
	if subs == nil || subs.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	h.serverActive = true
	subs.Server.ConnectClient(0)
	if afterConnect != nil {
		if err := afterConnect(); err != nil {
			return err
		}
	}

	h.clientState = caConnected
	h.signOns = 0

	handshake, ok := subs.Client.(handshakeClient)
	if !ok {
		return fmt.Errorf("client handshake implementation missing")
	}
	if err := handshake.LocalServerInfo(); err != nil {
		return fmt.Errorf("local serverinfo handshake failed: %w", err)
	}
	h.clientState = handshake.State()

	if err := h.runLocalHandshakeStep("prespawn", subs); err != nil {
		return err
	}
	if err := h.runLocalHandshakeStep("spawn", subs); err != nil {
		return err
	}
	if err := h.runLocalHandshakeStep("begin", subs); err != nil {
		return err
	}

	return nil
}

func (h *Host) stopSessionSounds(subs *Subsystems) {
	if subs == nil || subs.Audio == nil {
		return
	}
	subs.Audio.StopAllSounds(true)
}

func (h *Host) saveFilePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("save name is required")
	}
	if !saveNamePattern.MatchString(name) || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid save name %q", name)
	}
	if h.userDir == "" {
		return "", fmt.Errorf("user directory is not initialized")
	}
	return filepath.Join(h.userDir, "saves", name+".sav"), nil
}

func (h *Host) CmdGive(item, value string, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil || subs.Console == nil {
		return
	}
	subs.Console.Print(fmt.Sprintf("give %s %s (not fully implemented)\n", item, value))
}

func (h *Host) getLocalPlayerEdict(subs *Subsystems) *server.Edict {
	if subs.Server == nil {
		return nil
	}
	// In single player, local player is always client 0, which is edict 1
	return subs.Server.EdictNum(1)
}

func (h *Host) Error(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Host_Error: %s\n", message))
	}

	h.EndGame(message, subs)
}

// Menu commands

func (h *Host) CmdToggleMenu() {
	if h.menu == nil {
		return
	}
	h.menu.ToggleMenu()
}

func (h *Host) CmdMenuMain() {
	if h.menu == nil {
		return
	}
	h.menu.ShowMenu()
}

func (h *Host) CmdMenuQuit() {
	if h.menu == nil {
		return
	}
	// Switch to quit state
	h.menu.ShowMenu()
	// Note: The menu system handles quit confirmation internally
}

// Demo commands

func (h *Host) CmdRecord(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	// Check if already recording
	if h.demoState != nil && h.demoState.Recording {
		subs.Console.Print("Already recording a demo. Use 'stop' to end recording.\n")
		return
	}

	// Check if playing back
	if h.demoState != nil && h.demoState.Playback {
		subs.Console.Print("Cannot record during demo playback.\n")
		return
	}

	// Create demo state if needed
	if h.demoState == nil {
		h.demoState = &client.DemoState{
			Speed:     1.0,
			BaseSpeed: 1.0,
		}
	}

	// Get CD track (default to 0)
	cdtrack := 0
	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		cdtrack = loopbackClient.CDTrack
	}

	// Start recording
	if err := h.demoState.StartDemoRecording(filename, cdtrack); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to start recording: %v\n", err))
		return
	}

	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil && loopbackClient.State != cl.StateDisconnected && loopbackClient.Signon > 0 {
		if err := h.demoState.WriteInitialStateSnapshot(loopbackClient); err != nil {
			stopErr := h.demoState.StopRecording()
			if stopErr != nil {
				subs.Console.Print(fmt.Sprintf("Failed to capture initial demo state: %v (also failed to close demo: %v)\n", err, stopErr))
				return
			}
			subs.Console.Print(fmt.Sprintf("Failed to capture initial demo state: %v\n", err))
			return
		}
	}

	subs.Console.Print(fmt.Sprintf("Recording demo to %s\n", h.demoState.Filename))
}

func (h *Host) CmdStop(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	if h.demoState == nil || !h.demoState.Recording {
		subs.Console.Print("Not recording a demo.\n")
		return
	}

	var viewAngles [3]float32
	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		viewAngles = loopbackClient.ViewAngles
	}

	trailerErr := h.demoState.WriteDisconnectTrailer(viewAngles)
	stopErr := h.demoState.StopRecording()
	if trailerErr != nil {
		if stopErr != nil {
			subs.Console.Print(fmt.Sprintf("Error writing disconnect trailer: %v (also failed to close demo: %v)\n", trailerErr, stopErr))
			return
		}
		subs.Console.Print(fmt.Sprintf("Error writing disconnect trailer: %v\n", trailerErr))
		return
	}
	if stopErr != nil {
		subs.Console.Print(fmt.Sprintf("Error stopping demo: %v\n", stopErr))
		return
	}

	subs.Console.Print("Completed demo\n")
}

func (h *Host) CmdPlaydemo(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	// Check if already playing back
	if h.demoState != nil && h.demoState.Playback {
		subs.Console.Print("Already playing back a demo.\n")
		return
	}

	// Check if recording
	if h.demoState != nil && h.demoState.Recording {
		subs.Console.Print("Cannot playback while recording.\n")
		return
	}

	// Disconnect from any current server
	if h.serverActive {
		h.ShutdownServer(subs)
	}
	h.clientState = caDisconnected

	// Create demo state if needed
	if h.demoState == nil {
		h.demoState = &client.DemoState{
			Speed:     1.0,
			BaseSpeed: 1.0,
		}
	}

	// Start playback
	if err := h.demoState.StartDemoPlayback(filename); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to start playback: %v\n", err))
		return
	}

	subs.Console.Print(fmt.Sprintf("Playing demo from %s\n", h.demoState.Filename))

	// Set client state to connected for demo playback
	h.clientState = caConnected

	// Reset the actual client so recorded serverinfo/signon frames can bootstrap playback.
	if clientState := LoopbackClientState(subs); clientState != nil {
		clientState.ClearState()
		clientState.State = cl.StateDisconnected
	}
}

func (h *Host) CmdStopdemo(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}

	if err := h.demoState.StopPlayback(); err != nil {
		subs.Console.Print(fmt.Sprintf("Error stopping demo playback: %v\n", err))
		return
	}

	subs.Console.Print("Demo playback stopped.\n")
	h.clientState = caDisconnected
}
