// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package game

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
)

// RenderRecordEntity represents a quantized, deterministic entity record for render hashing.
type RenderRecordEntity struct {
	ModelIndex int
	Frame      int
	Skin       int
	Effects    int
	OriginX    int32 // Quantized origin (e.g. round(x * 8))
	OriginY    int32
	OriginZ    int32
	AnglePitch int32 // Quantized angle (e.g. round(pitch * 8))
	AngleYaw   int32
	AngleRoll  int32
}

// RenderRecord represents a deterministic frame capture for screenshot-less parity verification.
type RenderRecord struct {
	ViewLeaf int
	ViewOrg  [3]int32
	Entities []RenderRecordEntity
}

func quantize(val float32, scale float32) int32 {
	return int32(math.Round(float64(val * scale)))
}

// Hash returns a deterministic SHA-256 hex string representing the frame's render state.
func (r *RenderRecord) Hash() string {
	// Sort entities deterministically to be invariant to draw list traversal order
	sortedEnts := make([]RenderRecordEntity, len(r.Entities))
	copy(sortedEnts, r.Entities)
	sort.Slice(sortedEnts, func(i, j int) bool {
		if sortedEnts[i].ModelIndex != sortedEnts[j].ModelIndex {
			return sortedEnts[i].ModelIndex < sortedEnts[j].ModelIndex
		}
		if sortedEnts[i].Frame != sortedEnts[j].Frame {
			return sortedEnts[i].Frame < sortedEnts[j].Frame
		}
		if sortedEnts[i].Skin != sortedEnts[j].Skin {
			return sortedEnts[i].Skin < sortedEnts[j].Skin
		}
		if sortedEnts[i].OriginX != sortedEnts[j].OriginX {
			return sortedEnts[i].OriginX < sortedEnts[j].OriginX
		}
		if sortedEnts[i].OriginY != sortedEnts[j].OriginY {
			return sortedEnts[i].OriginY < sortedEnts[j].OriginY
		}
		if sortedEnts[i].OriginZ != sortedEnts[j].OriginZ {
			return sortedEnts[i].OriginZ < sortedEnts[j].OriginZ
		}
		if sortedEnts[i].AnglePitch != sortedEnts[j].AnglePitch {
			return sortedEnts[i].AnglePitch < sortedEnts[j].AnglePitch
		}
		if sortedEnts[i].AngleYaw != sortedEnts[j].AngleYaw {
			return sortedEnts[i].AngleYaw < sortedEnts[j].AngleYaw
		}
		return sortedEnts[i].AngleRoll < sortedEnts[j].AngleRoll
	})

	h := sha256.New()
	buf := make([]byte, 4)

	binary.LittleEndian.PutUint32(buf, uint32(r.ViewLeaf))
	h.Write(buf)

	for _, v := range r.ViewOrg {
		binary.LittleEndian.PutUint32(buf, uint32(v))
		h.Write(buf)
	}

	binary.LittleEndian.PutUint32(buf, uint32(len(sortedEnts)))
	h.Write(buf)

	entBuf := make([]byte, 40)
	for _, e := range sortedEnts {
		binary.LittleEndian.PutUint32(entBuf[0:4], uint32(e.ModelIndex))
		binary.LittleEndian.PutUint32(entBuf[4:8], uint32(e.Frame))
		binary.LittleEndian.PutUint32(entBuf[8:12], uint32(e.Skin))
		binary.LittleEndian.PutUint32(entBuf[12:16], uint32(e.Effects))
		binary.LittleEndian.PutUint32(entBuf[16:20], uint32(e.OriginX))
		binary.LittleEndian.PutUint32(entBuf[20:24], uint32(e.OriginY))
		binary.LittleEndian.PutUint32(entBuf[24:28], uint32(e.OriginZ))
		binary.LittleEndian.PutUint32(entBuf[28:32], uint32(e.AnglePitch))
		binary.LittleEndian.PutUint32(entBuf[32:36], uint32(e.AngleYaw))
		binary.LittleEndian.PutUint32(entBuf[36:40], uint32(e.AngleRoll))
		h.Write(entBuf)
	}

	return hex.EncodeToString(h.Sum(nil))
}

// ComputeRenderRecordFromDump constructs a RenderRecord from a DumpFrameState.
func ComputeRenderRecordFromDump(dump DumpFrameState) RenderRecord {
	rec := RenderRecord{
		ViewLeaf: dump.ViewLeaf,
		ViewOrg: [3]int32{
			quantize(dump.ViewOrg[0], 8),
			quantize(dump.ViewOrg[1], 8),
			quantize(dump.ViewOrg[2], 8),
		},
		Entities: make([]RenderRecordEntity, 0, len(dump.Visedicts)),
	}

	for _, e := range dump.Visedicts {
		rec.Entities = append(rec.Entities, RenderRecordEntity{
			ModelIndex: e.ModelIndex,
			Frame:      e.Frame,
			Skin:       e.Skin,
			Effects:    e.Effects,
			OriginX:    quantize(e.Origin[0], 8),
			OriginY:    quantize(e.Origin[1], 8),
			OriginZ:    quantize(e.Origin[2], 8),
			AnglePitch: quantize(e.Angles[0], 8),
			AngleYaw:   quantize(e.Angles[1], 8),
			AngleRoll:  quantize(e.Angles[2], 8),
		})
	}

	return rec
}

// ComputeRenderRecord extracts the current frame's RenderRecord from the Game instance.
func (g *Game) ComputeRenderRecord(frameIndex int) RenderRecord {
	dump := g.CaptureDumpState(frameIndex)
	return ComputeRenderRecordFromDump(dump)
}
