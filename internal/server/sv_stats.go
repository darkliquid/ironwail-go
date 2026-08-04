// This file belongs to the Network/Protocol subsystem: server-to-client message encoding, client management, and protocol types.

package server

import "strings"

const (
	defaultEffectsMask = 0xff
	statNonClient      = 11
)

func (s *Server) effectsMask() int {
	if s == nil || s.EffectsMask == 0 {
		return defaultEffectsMask
	}
	return s.EffectsMask
}

func (s *Server) standardQuakeWeaponEncoding() bool {
	if s == nil || s.FileSystem == nil {
		return true
	}
	fsInfo, ok := s.FileSystem.(interface{ GameDir() string })
	if !ok {
		return true
	}
	switch strings.ToLower(fsInfo.GameDir()) {
	case "rogue", "hipnotic", "quoth":
		return false
	default:
		return true
	}
}

// CalcStats derives HUD/stat slots from player entvars for SVCUpdateStat style networking.
func (s *Server) CalcStats(client *Client, statsi []int, statsf []float32, statss []string) {
	ent := client.Edict
	if ent == nil {
		return
	}

	for i := range statsi {
		statsi[i] = 0
	}
	for i := range statsf {
		statsf[i] = 0
	}
	for i := range statss {
		statss[i] = ""
	}

	const (
		StatHealth       = 0
		StatWeapon       = 2
		StatAmmo         = 3
		StatArmor        = 4
		StatWeaponFrame  = 5
		StatShells       = 6
		StatNails        = 7
		StatRockets      = 8
		StatCells        = 9
		StatActiveWeapon = 10
	)

	statsf[StatHealth] = ent.Health(s)
	statsi[StatWeapon] = int(ent.WeaponModel(s))
	statsf[StatAmmo] = ent.CurrentAmmo(s)
	statsf[StatArmor] = ent.ArmorValue(s)
	statsf[StatWeaponFrame] = ent.WeaponFrame(s)
	statsf[StatShells] = ent.AmmoShells(s)
	statsf[StatNails] = ent.AmmoNails(s)
	statsf[StatRockets] = ent.AmmoRockets(s)
	statsf[StatCells] = ent.AmmoCells(s)
	statsf[StatActiveWeapon] = ent.Weapon(s)
}
