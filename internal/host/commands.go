// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"strings"
)

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
}

func (h *Host) CmdNoClip() {
	if !h.serverActive {
		return
	}
}

func (h *Host) CmdFly() {
	if !h.serverActive {
		return
	}
}

func (h *Host) CmdNotarget() {
	if !h.serverActive {
		return
	}
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

func (h *Host) CmdKick(playerNum int) {
	if !h.serverActive {
		return
	}
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

func (h *Host) Error(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Host_Error: %s\n", message))
	}

	h.EndGame(message, subs)
}
