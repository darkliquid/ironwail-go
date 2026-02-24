// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"strings"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
)


func (h *Host) RegisterCommands(subs *Subsystems) {
cmdsys.AddCommand("quit", func(args []string) { h.CmdQuit() }, "Exit the game")
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
cmdsys.AddCommand("pause", func(args []string) { h.CmdPause() }, "Pause the game")
cmdsys.AddCommand("status", func(args []string) { h.CmdStatus(subs) }, "Show server status")
cmdsys.AddCommand("mapname", func(args []string) { h.CmdMapname(subs) }, "Show current map name")
cmdsys.AddCommand("god", func(args []string) { h.CmdGod() }, "Toggle god mode")
cmdsys.AddCommand("noclip", func(args []string) { h.CmdNoClip() }, "Toggle noclip mode")
cmdsys.AddCommand("fly", func(args []string) { h.CmdFly() }, "Toggle fly mode")
cmdsys.AddCommand("notarget", func(args []string) { h.CmdNotarget() }, "Toggle notarget mode")
cmdsys.AddCommand("say", func(args []string) {
if len(args) > 0 {
h.CmdSay(strings.Join(args, " "), subs)
}
}, "Send a message to all players")
cmdsys.AddCommand("serverinfo", func(args []string) { h.CmdServerInfo(subs) }, "Show server information")
cmdsys.AddCommand("restart", func(args []string) { h.CmdRestart(subs) }, "Restart the current map")
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
cmdsys.AddCommand("reconnect", func(args []string) { h.CmdReconnect(subs) }, "Reconnect to the current server")
cmdsys.AddCommand("name", func(args []string) {
if len(args) > 0 {
h.CmdName(args[0], subs)
}
}, "Set player name")
cmdsys.AddCommand("color", func(args []string) {
if len(args) > 0 {
h.CmdColor(args[0], subs)
}
}, "Set player color")
cmdsys.AddCommand("kill", func(args []string) { h.CmdKill(subs) }, "Suicide")
cmdsys.AddCommand("spawn", func(args []string) { h.CmdSpawn(subs) }, "Spawn into the game")
cmdsys.AddCommand("begin", func(args []string) { h.CmdBegin(subs) }, "Begin the game")
cmdsys.AddCommand("prespawn", func(args []string) { h.CmdPreSpawn(subs) }, "Pre-spawn handshake")
cmdsys.AddCommand("kick", func(args []string) {
if len(args) > 0 {
var playerNum int
fmt.Sscanf(args[0], "%d", &playerNum)
h.CmdKick(playerNum, subs)
}
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
}, "Save the current game")
cmdsys.AddCommand("give", func(args []string) {
if len(args) > 1 {
h.CmdGive(args[0], args[1], subs)
}
}, "Give items/ammo")
}


func (h *Host) CmdQuit() {
	h.Abort("quit")
}

func (h *Host) CmdMap(mapName string, subs *Subsystems) error {
	if subs.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	h.clientState = caDisconnected
	h.serverActive = false

	if err := subs.Server.Init(h.maxClients); err != nil {
		return fmt.Errorf("failed to init server for map %s: %w", mapName, err)
	}

	h.serverActive = true
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

func (h *Host) CmdGod() {
if !h.serverActive {
return
}
// TODO: Implement god mode by setting FlagGodMode on player edict
}

func (h *Host) CmdNoClip() {
if !h.serverActive {
return
}
// TODO: Implement noclip by setting MoveTypeNoClip on player edict
}

func (h *Host) CmdFly() {
if !h.serverActive {
return
}
// TODO: Implement fly mode by setting MoveTypeFly on player edict
}

func (h *Host) CmdNotarget() {
if !h.serverActive {
return
}
// TODO: Implement notarget by setting FlagNoTarget on player edict
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

func (h *Host) CmdKick(playerNum int, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	// TODO: Implement kick
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

	if subs.Server != nil {
		subs.Server.Shutdown()
	}

	if subs.Client != nil {
	}
}

func (h *Host) CmdRestart(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	// TODO: Implement restart
}

func (h *Host) CmdChangelevel(level string, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	// TODO: Implement changelevel
}

func (h *Host) CmdConnect(address string, subs *Subsystems) {
	// TODO: Implement connect
}

func (h *Host) CmdReconnect(subs *Subsystems) {
	// TODO: Implement reconnect
}

func (h *Host) CmdName(name string, subs *Subsystems) {
	// TODO: Implement name change
}

func (h *Host) CmdColor(color string, subs *Subsystems) {
	// TODO: Implement color change
}

func (h *Host) CmdKill(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	// TODO: Implement kill
}

func (h *Host) CmdSpawn(subs *Subsystems) {
	// TODO: Implement spawn
}

func (h *Host) CmdBegin(subs *Subsystems) {
	// TODO: Implement begin
}

func (h *Host) CmdPreSpawn(subs *Subsystems) {
	// TODO: Implement prespawn
}

func (h *Host) CmdPing(subs *Subsystems) {
	// TODO: Implement ping
}

func (h *Host) CmdLoad(name string, subs *Subsystems) {
	// TODO: Implement load
}

func (h *Host) CmdSave(name string, subs *Subsystems) {
	// TODO: Implement save
}

func (h *Host) CmdGive(item, value string, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	// TODO: Implement give
}

func (h *Host) Error(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Host_Error: %s\n", message))
	}

	h.EndGame(message, subs)
}
