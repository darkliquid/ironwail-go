// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"math"

	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/server"
)

// checkAutosave mirrors the simpler Host_CheckAutosave gates that decide
// whether single-player autosave is even eligible this frame.
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
	if player := subs.Server.EdictNum(1); player != nil && player.Vars != nil {
		if player.Vars.Health <= 0 {
			return
		}
		if server.MoveType(player.Vars.MoveType) == server.MoveTypeNone {
			return
		}
		speed := math.Sqrt(
			float64(player.Vars.Velocity[0]*player.Vars.Velocity[0] +
				player.Vars.Velocity[1]*player.Vars.Velocity[1] +
				player.Vars.Velocity[2]*player.Vars.Velocity[2]),
		)
		if speed > 100 {
			return
		}
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
