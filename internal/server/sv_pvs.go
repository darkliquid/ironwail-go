// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.

package server

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// SV_AddToFatPVS builds an expanded visibility set around a point to reduce pop-in during movement.
func (s *Server) SV_AddToFatPVS(org [3]float32, client *Client) {
	if s.WorldTree == nil || len(s.WorldTree.Nodes) == 0 {
		return
	}
	s.sv_AddToFatPVSRecursive(org, bsp.TreeChild{Index: 0, IsLeaf: false}, client)
}

// sv_AddToFatPVSRecursive walks BSP recursively and ORs visible leaves into the client's FatPVS mask.
func (s *Server) sv_AddToFatPVSRecursive(org [3]float32, child bsp.TreeChild, client *Client) {
	for {
		if child.IsLeaf {
			leaf := &s.WorldTree.Leafs[child.Index]
			if leaf.Contents != bsp.ContentsSolid {
				pvs := s.WorldTree.LeafPVS(leaf)
				if client.FatPVS == nil || len(client.FatPVS) != len(pvs) {
					client.FatPVS = make([]byte, len(pvs))
					copy(client.FatPVS, pvs)
				} else {
					for i := range pvs {
						client.FatPVS[i] |= pvs[i]
					}
				}
			}
			return
		}

		node := &s.WorldTree.Nodes[child.Index]
		plane := &s.WorldTree.Planes[node.PlaneNum]
		var d float32
		if plane.Type < 3 {
			d = org[plane.Type] - plane.Dist
		} else {
			d = VecDot(org, plane.Normal) - plane.Dist
		}

		if d > 8 {
			child = node.Children[0]
		} else if d < -8 {
			child = node.Children[1]
		} else {
			// go down both
			s.sv_AddToFatPVSRecursive(org, node.Children[0], client)
			child = node.Children[1]
		}
	}
}

// SV_VisibleToClient checks whether any entity leaf intersects the client's precomputed FatPVS.
func (s *Server) SV_VisibleToClient(ent *Edict, client *Client) bool {
	if ent == nil || client.FatPVS == nil {
		return false
	}
	if ent.NumLeafs >= MaxEntityLeafs {
		return true
	}
	if ent.NumLeafs == 0 {
		return false
	}

	for i := 0; i < ent.NumLeafs; i++ {
		leafIdx := ent.LeafNums[i]
		if leafIdx < 0 {
			continue
		}
		byteIdx := leafIdx >> 3
		if byteIdx >= len(client.FatPVS) {
			continue
		}
		if (client.FatPVS[byteIdx] & (1 << (uint(leafIdx) & 7))) != 0 {
			return true
		}
	}

	return false
}

// SV_EdictInPVS checks whether any of the edict's leaf numbers are visible
// in the given PVS byte array. Returns true if any leaf is set.
func (s *Server) SV_EdictInPVS(test *Edict, pvs []byte) bool {
	if test == nil || len(pvs) == 0 {
		return false
	}
	if test.NumLeafs >= MaxEntityLeafs {
		return true
	}
	if test.NumLeafs == 0 {
		return false
	}
	for i := 0; i < test.NumLeafs; i++ {
		leafIdx := test.LeafNums[i]
		if leafIdx < 0 {
			continue
		}
		byteIdx := leafIdx >> 3
		if byteIdx < 0 || byteIdx >= len(pvs) {
			continue
		}
		if (pvs[byteIdx] & (1 << (uint(leafIdx) & 7))) != 0 {
			return true
		}
	}
	return false
}
