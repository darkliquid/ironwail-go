// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"math"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/server"
)

func (h *Host) resetAutosaveState() {
	h.autosave = autosaveState{}
}

// checkAutosave mirrors Host_CheckAutosave closely enough to preserve the
// gameplay-facing heuristics around when autosaves are considered safe.
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
	player := subs.Server.EdictNum(1)
	if player == nil || player.Vars == nil {
		return
	}
	if player.Vars.Health <= 0 {
		return
	}

	intervalSeconds := cvar.FloatValue("sv_autosave_interval")
	if intervalSeconds <= 0 {
		return
	}
	now := h.realtime
	health := float64(player.Vars.Health)
	if h.clientState == caActive {
		h.updateAutosaveSecretState(subs)
	}

	if h.autosave.prevHealth == 0 {
		h.autosave.prevHealth = health
	}
	healthChange := health - h.autosave.prevHealth
	if healthChange < 0 && (healthChange < -3 || health < 100 || isHazardWater(player.Vars.WaterType)) {
		h.autosave.hurtTime = now
	}
	h.autosave.prevHealth = health

	if player.Vars.Button0 != 0 {
		h.autosave.shootTime = now
	}

	flags := uint32(player.Vars.Flags)
	if server.MoveType(player.Vars.MoveType) == server.MoveTypeNoClip || flags&(server.FlagGodMode|server.FlagNoTarget) != 0 {
		h.autosave.cheatTime += h.frameTime
		return
	}
	if now-h.autosave.hurtTime < 3 {
		return
	}
	if now-h.autosave.shootTime < 3 {
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

	elapsed := now - h.autosave.lastTime - h.autosave.cheatTime
	if elapsed < 3 {
		return
	}

	score := elapsed / intervalSeconds
	effectiveHealth := math.Min(100, health+float64(player.Vars.ArmorType*player.Vars.ArmorValue)) / 100
	score *= effectiveHealth
	score += math.Max(0, healthChange) / 100
	score -= (speed / 100) * 0.25
	score += h.autosave.secretBoost * 0.25
	score += clamp(1-(now-float64(player.Vars.TeleportTime))/1.5, 0, 1) * 0.5
	if score < 1 {
		return
	}

	h.autosave.lastTime = now
	h.autosave.cheatTime = 0
	console.Printf("Autosaving...\n")
	subs.Commands.AddText(fmt.Sprintf("save \"autosave/%s\" 0\n", subs.Server.GetMapName()))
}

func (h *Host) updateAutosaveSecretState(subs *Subsystems) {
	srv, ok := subs.Server.(*server.Server)
	if !ok || srv.QCVM == nil || srv.QCVM.GlobalVars == nil {
		return
	}
	foundSecrets := float64(srv.QCVM.GlobalVars.FoundSecrets)
	if foundSecrets != h.autosave.prevSecrets {
		h.autosave.prevSecrets = foundSecrets
		h.autosave.secretBoost = 1
		return
	}
	h.autosave.secretBoost = math.Max(0, h.autosave.secretBoost-h.frameTime/1.5)
}

func isHazardWater(waterType float32) bool {
	return int(waterType) == bsp.ContentsSlime || int(waterType) == bsp.ContentsLava
}
