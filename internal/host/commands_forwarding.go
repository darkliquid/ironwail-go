// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cmdsys"
)

func (h *Host) forwardClientCommand(command string, args []string, subs *Subsystems) bool {
	if cmdsys.Source() != cmdsys.SrcCommand {
		return false
	}
	if h.serverActive || (subs != nil && subs.Server != nil && subs.Server.IsActive()) {
		return false
	}
	if h.demoState != nil && h.demoState.Playback {
		return true
	}
	if subs == nil || subs.Client == nil || subs.Client.State() == caDisconnected {
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("Can't \"%s\", not connected\n", command))
		}
		return true
	}
	line := command
	if len(args) > 0 {
		line += " " + strings.Join(args, " ")
	}
	_ = subs.Client.SendStringCmd(line)
	return true
}

func (h *Host) CmdForwardToServer(args []string, subs *Subsystems) {
	if cmdsys.Source() != cmdsys.SrcCommand {
		return
	}
	if h.demoState != nil && h.demoState.Playback {
		return
	}
	if subs == nil || subs.Client == nil || subs.Client.State() == caDisconnected {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("Can't \"cmd\", not connected\n")
		}
		return
	}
	line := "\n"
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}
	_ = subs.Client.SendStringCmd(line)
}

func (h *Host) CmdRcon(args []string, subs *Subsystems) {
	if len(args) == 0 {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: rcon <command>\n")
		}
		return
	}
	if h.forwardClientCommand("rcon", args, subs) {
		return
	}
}
