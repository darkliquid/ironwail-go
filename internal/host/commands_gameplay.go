// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"

	cl "github.com/darkliquid/ironwail-go/internal/client"

	"math"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/server"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func (h *Host) CmdSkill(skill int) {
	if skill < 0 {
		skill = 0
	}
	if skill > 3 {
		skill = 3
	}
	h.currentSkill = skill
}

func (h *Host) CmdPause(subs *Subsystems) {
	if h.forwardClientCommand("pause", nil, subs) {
		return
	}
	if h.serverActive && h.maxClients == 1 {
		h.serverPaused = !h.serverPaused
	}
}

func (h *Host) CmdGod(subs *Subsystems) {
	if h.forwardClientCommand("god", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) ^ server.FlagGodMode)
	if subs.Console != nil {
		if uint32(ent.Vars.Flags)&server.FlagGodMode != 0 {
			subs.Console.Print("godmode ON\n")
		} else {
			subs.Console.Print("godmode OFF\n")
		}
	}
}
func (h *Host) CmdNoClip(subs *Subsystems) {
	if h.forwardClientCommand("noclip", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	if server.MoveType(ent.Vars.MoveType) == server.MoveTypeNoClip {
		ent.Vars.MoveType = float32(server.MoveTypeWalk)
		if subs.Console != nil {
			subs.Console.Print("noclip OFF\n")
		}
	} else {
		ent.Vars.MoveType = float32(server.MoveTypeNoClip)
		if subs.Console != nil {
			subs.Console.Print("noclip ON\n")
		}
	}
}
func (h *Host) CmdFly(subs *Subsystems) {
	if h.forwardClientCommand("fly", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	if server.MoveType(ent.Vars.MoveType) == server.MoveTypeFly {
		ent.Vars.MoveType = float32(server.MoveTypeWalk)
		if subs.Console != nil {
			subs.Console.Print("fly OFF\n")
		}
	} else {
		ent.Vars.MoveType = float32(server.MoveTypeFly)
		if subs.Console != nil {
			subs.Console.Print("fly ON\n")
		}
	}
}
func (h *Host) CmdNotarget(subs *Subsystems) {
	if h.forwardClientCommand("notarget", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) ^ server.FlagNoTarget)
	if subs.Console != nil {
		if uint32(ent.Vars.Flags)&server.FlagNoTarget != 0 {
			subs.Console.Print("notarget ON\n")
		} else {
			subs.Console.Print("notarget OFF\n")
		}
	}
}
func (h *Host) CmdTracepos(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil || subs.Console == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}

	forward, _, _ := qtypes.AngleVectors(qtypes.Vec3{X: ent.Vars.VAngle[0], Y: ent.Vars.VAngle[1], Z: ent.Vars.VAngle[2]})
	start := ent.Vars.Origin
	start[2] += 22 // eye height
	end := [3]float32{
		start[0] + forward.X*8192,
		start[1] + forward.Y*8192,
		start[2] + forward.Z*8192,
	}

	srv, _ := subs.Server.(*server.Server)
	trace := srv.Move(start, [3]float32{}, [3]float32{}, end, server.MoveType(server.MoveNormal), ent)

	subs.Console.Print(fmt.Sprintf("trace at: %.1f %.1f %.1f\n", trace.EndPos[0], trace.EndPos[1], trace.EndPos[2]))
	subs.Console.Print(fmt.Sprintf("fraction: %.4f\n", trace.Fraction))
	if trace.Entity != nil {
		entNum := srv.NumForEdict(trace.Entity)
		className := srv.GetString(trace.Entity.Vars.ClassName)
		subs.Console.Print(fmt.Sprintf("hit entity %d: %s\n", entNum, className))
	} else {
		subs.Console.Print("hit world\n")
	}
	subs.Console.Print(fmt.Sprintf("plane normal: %.2f %.2f %.2f\n", trace.PlaneNormal[0], trace.PlaneNormal[1], trace.PlaneNormal[2]))
}

func (h *Host) CmdSoundinfo(subs *Subsystems) {
	if subs.Audio == nil || subs.Console == nil {
		return
	}
	subs.Console.Print(subs.Audio.SoundInfo())
}

func (h *Host) CmdParticleTexture(mode string, subs *Subsystems) {
	if subs.Console == nil {
		return
	}
	cvar.Set("r_particles", mode)
	subs.Console.Print(fmt.Sprintf("particle_texture set to %s\n", mode))
}

func (h *Host) CmdFog(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	state := fogRuntimeState(subs)
	if state == nil {
		return
	}

	density, color := state.FogValues()
	targetDensity := density
	targetColor := color
	fadeTime := float32(0)

	switch len(args) {
	default:
		subs.Console.Print("usage:\n")
		subs.Console.Print("   fog <density>\n")
		subs.Console.Print("   fog <red> <green> <blue>\n")
		subs.Console.Print("   fog <density> <red> <green> <blue>\n")
		subs.Console.Print("current values:\n")
		subs.Console.Print(fmt.Sprintf("   \"density\" is \"%g\"\n", density))
		subs.Console.Print(fmt.Sprintf("   \"red\"     is \"%g\"\n", color[0]))
		subs.Console.Print(fmt.Sprintf("   \"green\"   is \"%g\"\n", color[1]))
		subs.Console.Print(fmt.Sprintf("   \"blue\"    is \"%g\"\n", color[2]))
		return
	case 1:
		targetDensity = parseFogFloat(args[0])
	case 2:
		targetDensity = parseFogFloat(args[0])
		fadeTime = parseFogFloat(args[1])
	case 3:
		targetColor = [3]float32{
			parseFogFloat(args[0]),
			parseFogFloat(args[1]),
			parseFogFloat(args[2]),
		}
	case 4:
		targetDensity = parseFogFloat(args[0])
		targetColor = [3]float32{
			parseFogFloat(args[1]),
			parseFogFloat(args[2]),
			parseFogFloat(args[3]),
		}
	case 5:
		targetDensity = parseFogFloat(args[0])
		targetColor = [3]float32{
			parseFogFloat(args[1]),
			parseFogFloat(args[2]),
			parseFogFloat(args[3]),
		}
		fadeTime = parseFogFloat(args[4])
	}

	if targetDensity < 0 {
		targetDensity = 0
	}
	for i := range targetColor {
		if targetColor[i] < 0 {
			targetColor[i] = 0
		}
		if targetColor[i] > 1 {
			targetColor[i] = 1
		}
	}

	state.SetFogState(
		fogByte(targetDensity),
		[3]byte{fogByte(targetColor[0]), fogByte(targetColor[1]), fogByte(targetColor[2])},
		fadeTime,
	)
}

func fogRuntimeState(subs *Subsystems) *cl.Client {
	if subs == nil || subs.Client == nil {
		return nil
	}
	stateClient, ok := subs.Client.(runtimeStateClient)
	if !ok {
		return nil
	}
	return stateClient.RuntimeState()
}

func parseFogFloat(s string) float32 {
	value, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return 0
	}
	return float32(value)
}

func fogByte(value float32) byte {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 255
	}
	return byte(math.Round(float64(value * 255)))
}

func (h *Host) CmdMaps(subs *Subsystems) {
	if subs == nil || subs.Files == nil || subs.Console == nil {
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}
	files := fsInstance.ListFiles("maps/*.bsp")
	subs.Console.Print("Maps found:\n")
	for _, f := range files {
		name := filepath.Base(f)
		name = strings.TrimSuffix(name, ".bsp")
		subs.Console.Print(fmt.Sprintf("  %s\n", name))
	}
}

// CmdRandmap picks a random map from the available maps and changes to it.
func (h *Host) CmdRandmap(subs *Subsystems) {
	if subs == nil || subs.Files == nil || subs.Console == nil || subs.Commands == nil {
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}
	files := fsInstance.ListFiles("maps/*.bsp")
	if len(files) == 0 {
		subs.Console.Print("randmap: no maps found\n")
		return
	}

	// Build list of map names
	maps := make([]string, 0, len(files))
	for _, f := range files {
		name := filepath.Base(f)
		name = strings.TrimSuffix(name, ".bsp")
		maps = append(maps, name)
	}

	// C Host_Randmap_f queues the map command via Cbuf_AddText, so it works
	// even without an active server.
	choice := maps[rand.Intn(len(maps))]
	subs.Console.Print(fmt.Sprintf("randmap: changing to %s\n", choice))
	subs.Commands.AddText(fmt.Sprintf("map %s\n", choice))
}

// findViewthing searches the server's edicts for an entity with classname "viewthing".
func (h *Host) CmdKill(subs *Subsystems) {
	if h.forwardClientCommand("kill", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	subs.Server.KillClient(0)
}

func (h *Host) CmdSpawn(subs *Subsystems) {
	if err := h.runHandshakeStep("spawn", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("spawn failed: %v\n", err))
	}
}

func (h *Host) CmdBegin(subs *Subsystems) {
	if err := h.runHandshakeStep("begin", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("begin failed: %v\n", err))
	}
}

func (h *Host) CmdPreSpawn(subs *Subsystems) {
	if err := h.runHandshakeStep("prespawn", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("prespawn failed: %v\n", err))
	}
}

func (h *Host) CmdPing(subs *Subsystems) {
	if h.forwardClientCommand("ping", nil, subs) {
		return
	}
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

func (h *Host) CmdGive(item, value string, subs *Subsystems) {
	if h.forwardClientCommand("give", []string{item, value}, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil || subs.Console == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}

	val := float32(0)
	fmt.Sscanf(value, "%f", &val)
	if val <= 0 {
		val = 100
	}

	switch item {
	case "h":
		ent.Vars.Health += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f health\n", val))
	case "s":
		ent.Vars.AmmoShells += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f shells\n", val))
	case "n":
		ent.Vars.AmmoNails += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f nails\n", val))
	case "r":
		ent.Vars.AmmoRockets += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f rockets\n", val))
	case "c":
		ent.Vars.AmmoCells += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f cells\n", val))
	default:
		subs.Console.Print(fmt.Sprintf("give %s %s (not supported)\n", item, value))
	}
}
func (h *Host) getLocalPlayerEdict(subs *Subsystems) *server.Edict {
	if subs.Server == nil {
		return nil
	}
	// In single player, local player is always client 0, which is edict 1
	return subs.Server.EdictNum(1)
}

// Demo commands
