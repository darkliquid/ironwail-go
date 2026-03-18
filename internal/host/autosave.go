// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import "github.com/ironwail/ironwail-go/internal/cvar"

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

	intervalMinutes := cvar.FloatValue("host_autosave")
	if intervalMinutes <= 0 {
		return
	}
	if h.realtime < h.nextAutosave {
		return
	}

	h.nextAutosave = h.realtime + intervalMinutes*60
	subs.Commands.AddText("save auto\n")
}
