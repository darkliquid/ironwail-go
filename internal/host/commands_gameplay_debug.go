// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"math"
	"strconv"

	"github.com/darkliquid/ironwail-go/internal/server"
)

// findViewthing searches the server's edicts for an entity with classname "viewthing".
func (h *Host) findViewthing(subs *Subsystems) *server.Edict {
	srv, ok := subs.Server.(*server.Server)
	if !ok || srv == nil {
		return nil
	}
	for i := 1; i < srv.NumEdicts; i++ {
		ent := srv.EdictNum(i)
		if ent == nil || ent.Free || ent.Vars == nil {
			continue
		}
		if srv.GetString(ent.Vars.ClassName) == "viewthing" {
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
	ent.Vars.Frame = float32(frame)
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
	frame := int(ent.Vars.Frame) + 1
	ent.Vars.Frame = float32(frame)
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
	frame := int(ent.Vars.Frame) - 1
	if frame < 0 {
		frame = 0
	}
	ent.Vars.Frame = float32(frame)
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
	subs.Console.Print(fmt.Sprintf("viewpos: %.2f %.2f %.2f (yaw: %.2f pitch: %.2f)\n", ent.Vars.Origin[0], ent.Vars.Origin[1], ent.Vars.Origin[2], ent.Vars.VAngle[1], ent.Vars.VAngle[0]))
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

	if len(filtered) != 3 && len(filtered) != 6 {
		if subs.Console != nil {
			subs.Console.Print("usage:\n")
			subs.Console.Print("   setpos <x> <y> <z>\n")
			subs.Console.Print("   setpos <x> <y> <z> <pitch> <yaw> <roll>\n")
			subs.Console.Print(fmt.Sprintf("current values:\n   %d %d %d %d %d %d\n",
				int(math.Round(float64(ent.Vars.Origin[0]))),
				int(math.Round(float64(ent.Vars.Origin[1]))),
				int(math.Round(float64(ent.Vars.Origin[2]))),
				int(math.Round(float64(ent.Vars.VAngle[0]))),
				int(math.Round(float64(ent.Vars.VAngle[1]))),
				int(math.Round(float64(ent.Vars.VAngle[2])))))
		}
		return
	}

	// Auto-enable noclip
	if server.MoveType(ent.Vars.MoveType) != server.MoveTypeNoClip {
		ent.Vars.MoveType = float32(server.MoveTypeNoClip)
		if subs.Console != nil {
			subs.Console.Print("noclip ON\n")
		}
	}

	// Clear velocity
	ent.Vars.Velocity = [3]float32{}

	// Set origin
	ent.Vars.Origin[0] = filtered[0]
	ent.Vars.Origin[1] = filtered[1]
	ent.Vars.Origin[2] = filtered[2]

	// Optionally set angles
	if len(filtered) == 6 {
		ent.Vars.Angles[0] = filtered[3]
		ent.Vars.Angles[1] = filtered[4]
		ent.Vars.Angles[2] = filtered[5]
		ent.Vars.FixAngle = 1
	}

	// Relink entity in world
	if srv, ok := subs.Server.(*server.Server); ok {
		srv.LinkEdict(ent, false)
	}
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
		className := srv.GetString(ent.Vars.ClassName)
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
		if ent.Vars != nil {
			if ent.Vars.Solid != 0 {
				solid++
			}
			if ent.Vars.Model != 0 {
				models++
			}
			if server.MoveType(ent.Vars.MoveType) == server.MoveTypeStep {
				step++
			}
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
