// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"math"
	"strconv"

	"github.com/darkliquid/ironwail-go/internal/server"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// findViewthing searches the server's edicts for an entity with classname "viewthing".
func (h *Host) findViewthing(subs *Subsystems) *server.Edict {
	srv, ok := subs.Server.(*server.Server)
	if !ok || srv == nil {
		return nil
	}
	for i := 1; i < srv.NumEdicts; i++ {
		ent := srv.EdictNum(i)
		if ent == nil || ent.Free {
			continue
		}
		if srv.String(ent.ClassName(srv)) == "viewthing" {
			return ent
		}
	}
	return nil
}

// CmdViewframe sets the viewthing entity's animation frame to the given value.
func (h *Host) CmdViewframe(frame int, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if !h.serverActive || subs.Server == nil {
		subs.Console.Print("viewframe: no server running\n")
		return
	}
	ent := h.findViewthing(subs)
	if ent == nil {
		subs.Console.Print("viewframe: no viewthing on map\n")
		return
	}
	if frame < 0 {
		frame = 0
	}
	srv, _ := subs.Server.(*server.Server)
	ent.SetFrame(srv, float32(frame))
	subs.Console.Print(fmt.Sprintf("frame %d\n", frame))
}

// CmdViewnext advances the viewthing entity's animation frame by one.
func (h *Host) CmdViewnext(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if !h.serverActive || subs.Server == nil {
		subs.Console.Print("viewnext: no server running\n")
		return
	}
	ent := h.findViewthing(subs)
	if ent == nil {
		subs.Console.Print("viewnext: no viewthing on map\n")
		return
	}
	srv, _ := subs.Server.(*server.Server)
	frame := int(ent.Frame(srv)) + 1
	ent.SetFrame(srv, float32(frame))
	subs.Console.Print(fmt.Sprintf("frame %d\n", frame))
}

// CmdViewprev decrements the viewthing entity's animation frame by one (clamped to 0).
func (h *Host) CmdViewprev(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if !h.serverActive || subs.Server == nil {
		subs.Console.Print("viewprev: no server running\n")
		return
	}
	ent := h.findViewthing(subs)
	if ent == nil {
		subs.Console.Print("viewprev: no viewthing on map\n")
		return
	}
	srv, _ := subs.Server.(*server.Server)
	frame := int(ent.Frame(srv)) - 1
	if frame < 0 {
		frame = 0
	}
	ent.SetFrame(srv, float32(frame))
	subs.Console.Print(fmt.Sprintf("frame %d\n", frame))
}

func (h *Host) CmdViewpos(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	srv, _ := subs.Server.(*server.Server)
	entOrigin := ent.Origin(srv)
	entVAngle := ent.VAngle(srv)
	subs.Console.Print(fmt.Sprintf("viewpos: %.2f %.2f %.2f (yaw: %.2f pitch: %.2f)\n", entOrigin.X, entOrigin.Y, entOrigin.Z, entVAngle.Y, entVAngle.X))
}

func (h *Host) CmdSetPos(args []string, subs *Subsystems) {
	if h.forwardClientCommand("setpos", args, subs) {
		return
	}
	if !h.serverActive || subs == nil || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}

	// Filter out parentheses (for copy-pasting from viewpos output)
	var filtered []float32
	for _, arg := range args {
		if arg == "(" || arg == ")" {
			continue
		}
		v, err := strconv.ParseFloat(arg, 32)
		if err != nil {
			continue
		}
		filtered = append(filtered, float32(v))
	}

	srv, _ := subs.Server.(*server.Server)
	entOrigin := ent.Origin(srv)
	entVAngle := ent.VAngle(srv)
	if len(filtered) != 3 && len(filtered) != 6 {
		if subs.Console != nil {
			subs.Console.Print("usage:\n")
			subs.Console.Print("   setpos <x> <y> <z>\n")
			subs.Console.Print("   setpos <x> <y> <z> <pitch> <yaw> <roll>\n")
			subs.Console.Print(fmt.Sprintf("current values:\n   %d %d %d %d %d %d\n",
				int(math.Round(float64(entOrigin.X))),
				int(math.Round(float64(entOrigin.Y))),
				int(math.Round(float64(entOrigin.Z))),
				int(math.Round(float64(entVAngle.X))),
				int(math.Round(float64(entVAngle.Y))),
				int(math.Round(float64(entVAngle.Z)))))
		}
		return
	}

	// Auto-enable noclip so the player doesn't fall through the world
	// when teleporting to arbitrary positions.
	if server.MoveType(ent.MoveType(srv)) != server.MoveTypeNoClip {
		ent.SetMoveType(srv, float32(server.MoveTypeNoClip))
		if subs.Console != nil {
			subs.Console.Print("noclip ON\n")
		}
	}

	// Clear velocity
	ent.SetVelocity(srv, qtypes.Vec3{})

	// Set origin
	entOrigin.X = filtered[0]
	entOrigin.Y = filtered[1]
	entOrigin.Z = filtered[2]
	ent.SetOrigin(srv, entOrigin)

	// Optionally set angles
	if len(filtered) == 6 {
		entAngles := ent.Angles(srv)
		entAngles.X = filtered[3]
		entAngles.Y = filtered[4]
		entAngles.Z = filtered[5]
		ent.SetAngles(srv, entAngles)
		// Keep server and immediate local view queries aligned. C updates client
		// view through fixangle; mirroring VAngle here avoids transient stale
		// orientation when scripts issue setpos and immediately capture output.
		ent.SetVAngle(srv, qtypes.Vec3{X: filtered[3], Y: filtered[4], Z: filtered[5]})
		ent.SetFixAngle(srv, 1)
	}

	// Relink entity in world
	srv.LinkEdict(ent, false)
}

func (h *Host) CmdPrEnts(subs *Subsystems) {
	if subs == nil || subs.Server == nil || subs.Console == nil {
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		return
	}
	subs.Console.Print(fmt.Sprintf("%d edicts\n", srv.NumEdicts))
	for i := 0; i < srv.NumEdicts; i++ {
		ent := srv.Edicts[i]
		if ent == nil || ent.Free {
			continue
		}
		className := srv.String(ent.ClassName(srv))
		subs.Console.Print(fmt.Sprintf("%d: %s\n", i, className))
	}
}

func (h *Host) CmdEdictCount(subs *Subsystems) {
	if subs == nil || subs.Server == nil || subs.Console == nil || !h.serverActive {
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		return
	}

	active, models, solid, step := 0, 0, 0, 0
	for i := 0; i < srv.NumEdicts; i++ {
		ent := srv.Edicts[i]
		if ent == nil || ent.Free {
			continue
		}
		active++
		if ent.Solid(srv) != 0 {
			solid++
		}
		if ent.Model(srv) != 0 {
			models++
		}
		if server.MoveType(ent.MoveType(srv)) == server.MoveTypeStep {
			step++
		}
	}

	subs.Console.Print(fmt.Sprintf("num_edicts:%3d\n", srv.NumEdicts))
	subs.Console.Print(fmt.Sprintf("active    :%3d\n", active))
	subs.Console.Print(fmt.Sprintf("peak      :%3d\n", srv.PeakEdicts()))
	subs.Console.Print(fmt.Sprintf("view      :%3d\n", models))
	subs.Console.Print(fmt.Sprintf("touch     :%3d\n", solid))
	subs.Console.Print(fmt.Sprintf("step      :%3d\n", step))
	yes, no := server.CheckBottomStats()
	subs.Console.Print(fmt.Sprintf("c_yes     :%3d\n", yes))
	subs.Console.Print(fmt.Sprintf("c_no      :%3d\n", no))
}

func (h *Host) CmdProfile(subs *Subsystems) {
	if subs == nil || subs.Server == nil || subs.Console == nil || !h.serverActive {
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		return
	}
	for _, result := range srv.QCProfileResults(10) {
		subs.Console.Print(fmt.Sprintf("%7d %s\n", result.Profile, result.Name))
	}
}

func (h *Host) CmdDevStats(subs *Subsystems) {
	if subs == nil || subs.Server == nil || subs.Console == nil || !h.serverActive {
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		return
	}
	curr, peak := srv.DevStatsSnapshot()
	subs.Console.Print("devstats | Curr  Peak\n")
	subs.Console.Print("---------+-----------\n")
	subs.Console.Print(fmt.Sprintf("Edicts   |%5d %5d\n", curr.Edicts, peak.Edicts))
	subs.Console.Print(fmt.Sprintf("Packet   |%5d %5d\n", curr.PacketSize, peak.PacketSize))
	subs.Console.Print(fmt.Sprintf("Visedicts|%5d %5d\n", curr.Visedicts, peak.Visedicts))
	subs.Console.Print(fmt.Sprintf("Efrags   |%5d %5d\n", curr.Efrags, peak.Efrags))
	subs.Console.Print(fmt.Sprintf("Dlights  |%5d %5d\n", curr.DLights, peak.DLights))
	subs.Console.Print(fmt.Sprintf("Beams    |%5d %5d\n", curr.Beams, peak.Beams))
	subs.Console.Print(fmt.Sprintf("Tempents |%5d %5d\n", curr.Tempents, peak.Tempents))
	subs.Console.Print(fmt.Sprintf("GL upload|%4dK %4dK\n", curr.GPUUpload/1024, peak.GPUUpload/1024))
}
