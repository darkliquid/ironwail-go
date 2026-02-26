// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"strings"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/server"
	"github.com/ironwail/ironwail-go/internal/fs"
)

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
	cmdsys.AddCommand("reconnect", func(args []string) { h.CmdReconnect(subs) }, "Reconnect to current server")
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
	cmdsys.AddCommand("spawn", func(args []string) { h.CmdSpawn(subs) }, "Spawn into game")
	cmdsys.AddCommand("begin", func(args []string) { h.CmdBegin(subs) }, "Begin game")
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
	}, "Save current game")
	cmdsys.AddCommand("give", func(args []string) {
		if len(args) > 1 {
			h.CmdGive(args[0], args[1], subs)
		}
	}, "Give items/ammo")

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

	if fsInstance, ok := subs.Files.(*fs.FileSystem); ok {
		if err := subs.Server.SpawnServer(mapName, fsInstance); err != nil {
			return fmt.Errorf("failed to spawn server for map %s: %w", mapName, err)
		}
	} else {
		return fmt.Errorf("filesystem implementation is missing")
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
	// TODO: Implement connect
}

func (h *Host) CmdReconnect(subs *Subsystems) {
	// TODO: Implement reconnect
}

func (h *Host) CmdName(name string, subs *Subsystems) {
	if subs.Server != nil {
		subs.Server.SetClientName(0, name)
	}
}

func (h *Host) CmdColor(colorStr string, subs *Subsystems) {
	if subs.Server != nil {
		var color int
		fmt.Sscanf(colorStr, "%d", &color)
		subs.Server.SetClientColor(0, color)
	}
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
	// TODO: Implement spawn
}

func (h *Host) CmdBegin(subs *Subsystems) {
	// TODO: Implement begin
}

func (h *Host) CmdPreSpawn(subs *Subsystems) {
	// TODO: Implement prespawn
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
	// TODO: Implement load
}

func (h *Host) CmdSave(name string, subs *Subsystems) {
	// TODO: Implement save
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

