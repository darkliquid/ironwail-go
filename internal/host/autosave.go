// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

// checkAutosave mirrors Quake's Host_CheckAutosave behavior by periodically
// queuing "save auto" while in an active single-player game.
func (h *Host) checkAutosave(subs *Subsystems) {
	if h == nil || subs == nil || subs.Server == nil || subs.Commands == nil {
		return
	}
	if !h.serverActive || !subs.Server.IsActive() {
		return
	}
	if h.clientState != caConnected && h.clientState != caActive {
		return
	}
	if h.signOns <= 0 {
		return
	}
	if subs.Server.GetMaxClients() != 1 {
		return
	}
	if !cvar.BoolValue("sv_autosave") {
		return
	}
	if clientState := LoopbackClientState(subs); clientState != nil && clientState.Intermission != 0 {
		return
	}
	if player := subs.Server.EdictNum(1); player != nil && player.Vars != nil && player.Vars.Health <= 0 {
		return
	}

	intervalSeconds := cvar.FloatValue("sv_autosave_interval")
	if intervalSeconds <= 0 {
		return
	}
	if h.realtime < h.nextAutosave {
		return
	}

	h.nextAutosave = h.realtime + intervalSeconds
	console.Printf("Autosaving...\n")
	subs.Commands.AddText(fmt.Sprintf("save \"autosave/%s\" 0\n", subs.Server.GetMapName()))
}
