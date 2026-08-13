// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package game

import (
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// DumpFrameState captures the full frame state and active edicts into the
// standardized H1 dumpstate schema.
type DumpFrameState struct {
	Frame        int         `json:"frame"`
	ViewOrg      [3]float32  `json:"vieworg"`
	ViewAngles   [3]float32  `json:"viewangles"`
	ViewLeaf     int         `json:"viewleaf"`
	MatView      [16]float32 `json:"r_matview"`
	MatProj      [16]float32 `json:"r_matproj"`
	SvTime       float32     `json:"sv_time,omitempty"`
	Gravity      float32     `json:"gravity,omitempty"`
	MaxClients   int         `json:"maxclients,omitempty"`
	ForceRetouch int         `json:"force_retouch,omitempty"`
	Visedicts    []DumpEdict `json:"visedicts"`
	Lights       []DumpLight `json:"lights"`
}

// DumpEdict represents the extended H1 schema for an edict.
type DumpEdict struct {
	Number       int        `json:"number"`
	Origin       [3]float32 `json:"origin"`
	Angles       [3]float32 `json:"angles"`
	Velocity     [3]float32 `json:"velocity,omitempty"`
	Model        string     `json:"model,omitempty"`
	ModelIndex   int        `json:"modelindex,omitempty"`
	Frame        int        `json:"frame,omitempty"`
	Skin         int        `json:"skin,omitempty"`
	Colormap     int        `json:"colormap,omitempty"`
	Effects      int        `json:"effects,omitempty"`
	Solid        int        `json:"solid,omitempty"`
	Movetype     int        `json:"movetype,omitempty"`
	Flags        int        `json:"flags,omitempty"`
	GroundEntity int        `json:"groundentity,omitempty"`
	LTime        float32    `json:"ltime,omitempty"`
	Think        string     `json:"think,omitempty"`
	NextThink    float32    `json:"nextthink,omitempty"`
	Enemy        int        `json:"enemy,omitempty"`
	GoalEntity   int        `json:"goalentity,omitempty"`
}

// DumpLight represents dynamic light state in the frame.
type DumpLight struct {
	Pos      [3]float32 `json:"pos"`
	Radius   float32    `json:"radius"`
	Color    [3]float32 `json:"color"`
	MinLight float32    `json:"minlight"`
}

// CaptureDumpState produces a DumpFrameState from current Game subsystems.
func (g *Game) CaptureDumpState(frameIndex int) DumpFrameState {
	viewOrg, viewAngles := g.runtimeViewState()
	var camera renderer.CameraState
	var matView, matProj [16]float32
	var viewLeaf int = -1

	if g.Renderer != nil {
		camera = g.runtimeCameraState(viewOrg, viewAngles)
		if r, ok := g.Renderer.(*renderer.Renderer); ok {
			matView = r.ViewMatrix()
			if r.WorldData() != nil && r.WorldData().Geometry != nil {
				tree := r.WorldData().Geometry.Tree
				if leaf := tree.PointInLeaf(viewOrg); leaf != nil {
					for idx := range tree.Leafs {
						if &tree.Leafs[idx] == leaf {
							viewLeaf = idx
							break
						}
					}
				}
			}
		}
	}

	dump := DumpFrameState{
		Frame:      frameIndex,
		ViewOrg:    viewOrg,
		ViewAngles: [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z},
		ViewLeaf:   viewLeaf,
		MatView:    matView,
		MatProj:    matProj,
		Visedicts:  make([]DumpEdict, 0),
		Lights:     make([]DumpLight, 0),
	}

	if g.Server != nil && g.Server.Active {
		dump.SvTime = float32(g.Server.Time)
		dump.Gravity = float32(g.Server.Gravity)
		dump.MaxClients = g.Server.MaxClients()

		for i := 0; i < g.Server.NumEdicts; i++ {
			ent := g.Server.EdictNum(i)
			if ent == nil || ent.Free {
				continue
			}
			de := DumpEdict{
				Number:       i,
				Origin:       ent.Origin(g.Server),
				Angles:       ent.Angles(g.Server),
				Velocity:     ent.Velocity(g.Server),
				Model:        ent.ModelString(g.Server),
				ModelIndex:   int(ent.ModelIndex(g.Server)),
				Frame:        int(ent.Frame(g.Server)),
				Skin:         int(ent.Skin(g.Server)),
				Colormap:     int(ent.Colormap(g.Server)),
				Effects:      int(ent.Effects(g.Server)),
				Solid:        int(ent.Solid(g.Server)),
				Movetype:     int(ent.MoveType(g.Server)),
				Flags:        int(ent.Flags(g.Server)),
				GroundEntity: int(ent.GroundEntity(g.Server)),
				LTime:        ent.LTime(g.Server),
				NextThink:    ent.NextThink(g.Server),
				Enemy:        int(ent.Enemy(g.Server)),
				GoalEntity:   int(ent.GoalEntity(g.Server)),
			}
			dump.Visedicts = append(dump.Visedicts, de)
		}
	} else if g.Client != nil && g.Client.State == cl.StateActive {
		for num, state := range g.Client.Entities {
			modelName := ""
			if state.ModelIndex > 0 && int(state.ModelIndex)-1 < len(g.Client.ModelPrecache) {
				modelName = g.Client.ModelPrecache[int(state.ModelIndex)-1]
			}
			de := DumpEdict{
				Number:     num,
				Origin:     state.Origin,
				Angles:     state.Angles,
				Model:      modelName,
				ModelIndex: int(state.ModelIndex),
				Frame:      int(state.Frame),
				Skin:       int(state.Skin),
				Colormap:   int(state.Colormap),
				Effects:    int(state.Effects),
			}
			dump.Visedicts = append(dump.Visedicts, de)
		}
	}

	if g.Renderer != nil {
		if r, ok := g.Renderer.(*renderer.Renderer); ok {
			for _, l := range r.ActiveLights() {
				dump.Lights = append(dump.Lights, DumpLight{
					Pos:      [3]float32{l.Position[0], l.Position[1], l.Position[2]},
					Radius:   l.Radius,
					Color:    [3]float32{l.Color[0], l.Color[1], l.Color[2]},
					MinLight: l.MinLight,
				})
			}
		}
	}

	return dump
}
