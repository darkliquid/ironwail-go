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
	fsInfo, ok := s.FileSystem.(interface{ GetGameDir() string })
	if !ok {
		return true
	}
	switch strings.ToLower(fsInfo.GetGameDir()) {
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

	statsf[StatHealth] = ent.Vars.Health
	statsi[StatWeapon] = int(ent.Vars.WeaponModel)
	statsf[StatAmmo] = ent.Vars.CurrentAmmo
	statsf[StatArmor] = ent.Vars.ArmorValue
	statsf[StatWeaponFrame] = ent.Vars.WeaponFrame
	statsf[StatShells] = ent.Vars.AmmoShells
	statsf[StatNails] = ent.Vars.AmmoNails
	statsf[StatRockets] = ent.Vars.AmmoRockets
	statsf[StatCells] = ent.Vars.AmmoCells
	statsf[StatActiveWeapon] = ent.Vars.Weapon
}
