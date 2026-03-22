package server

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/common"
)

// TextSaveGameState captures the line-oriented KEX/native text save format
// after header parsing. The entity/global body stays in raw text form so a
// live server can reuse the existing ED_ParseGlobals / ED_ParseEdict paths.
type TextSaveGameState struct {
	Version     int
	GameDir     string
	Title       string
	Skill       int
	MapName     string
	Time        float32
	SpawnParms  [NumSpawnParms]float32
	LightStyles [64]string
	EntityText  string
}

// ParseTextSaveGame parses the Quake-style text save header and preserves the
// remaining globals/edict block for later restoration into a live server.
func ParseTextSaveGame(data []byte) (*TextSaveGameState, error) {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return nil, fmt.Errorf("savegame is empty")
	}

	state := &TextSaveGameState{}
	var skill float32

	text, state.Version = common.COM_ParseIntNewline(text)
	if state.Version <= 0 {
		return nil, fmt.Errorf("missing savegame version")
	}
	if state.Version == SaveGameVersionKEX {
		var line string
		text, line = parseTextSaveLine(text)
		state.GameDir = line
	}

	text, state.Title = parseTextSaveLine(text)
	for i := 0; i < NumSpawnParms; i++ {
		text, state.SpawnParms[i] = parseTextSaveFloatLine(text)
	}
	text, skill = parseTextSaveFloatLine(text)
	state.Skill = int(skill + 0.1)

	text, state.MapName = parseTextSaveLine(text)
	if state.MapName == "" {
		return nil, fmt.Errorf("savegame map is empty")
	}

	text, state.Time = parseTextSaveFloatLine(text)
	for i := range state.LightStyles {
		text, state.LightStyles[i] = parseTextSaveLine(text)
	}

	state.EntityText = strings.TrimLeft(text, " \t\r\n")
	if state.EntityText == "" {
		return nil, fmt.Errorf("savegame entity data is empty")
	}

	return state, nil
}

// RestoreTextSaveGameState applies a parsed text save to a live spawned server.
// The caller is expected to spawn the target map first, then invoke this before
// the local signon handshake continues.
func (s *Server) RestoreTextSaveGameState(state *TextSaveGameState) error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	if state == nil {
		return fmt.Errorf("text savegame state is nil")
	}
	if state.MapName == "" {
		return fmt.Errorf("savegame map is empty")
	}
	if s.Name != "" && state.MapName != s.Name {
		return fmt.Errorf("savegame map %q does not match active map %q", state.MapName, s.Name)
	}
	if s.QCVM == nil {
		return fmt.Errorf("qcvm is not initialized")
	}

	s.Time = state.Time
	s.Paused = true
	s.LoadGame = true
	copy(s.LightStyles[:], state.LightStyles[:])
	for i := range s.LightStyles {
		if s.LightStyles[i] == "" {
			s.LightStyles[i] = "m"
		}
	}
	if s.Static != nil && len(s.Static.Clients) > 0 && s.Static.Clients[0] != nil {
		s.Static.Clients[0].SpawnParms = state.SpawnParms
	}

	em := &EntityManager{
		edicts:     s.Edicts,
		vm:         s.QCVM,
		maxEdicts:  s.MaxEdicts,
		numEdicts:  s.NumEdicts,
		freeTime:   make([]float32, maxInt(s.MaxEdicts, len(s.Edicts))),
		maxClients: s.GetMaxClients(),
	}
	em.SetCurrentTime(s.Time)

	s.ClearWorld()

	data := state.EntityText
	entnum := -1
	for {
		data = common.COM_Parse(data)
		if common.ComToken == "" {
			break
		}
		if common.ComToken != "{" {
			return fmt.Errorf("First token isn't a brace")
		}
		blockData := "{" + data

		var err error
		if entnum == -1 {
			data, err = em.ED_ParseGlobals(blockData, s.QCVM)
			if err != nil {
				return err
			}
		} else {
			if err := s.ensureTextSaveEdictCapacity(entnum + 1); err != nil {
				return err
			}
			em.edicts = s.Edicts
			if entnum < em.numEdicts {
				em.ED_ClearEdict(entnum)
			} else {
				s.Edicts[entnum] = &Edict{Vars: &EntVars{}, Scale: 16}
				clearQCVMEdictData(s.QCVM, entnum)
			}
			data, err = em.ED_ParseEdict(blockData, entnum)
			if err != nil {
				return err
			}
			if ent := s.Edicts[entnum]; ent != nil && !ent.Free && entnum != 0 {
				if ent.Scale == 0 {
					ent.Scale = 16
				}
				s.LinkEdict(ent, false)
			}
		}

		entnum++
	}

	if entnum <= 0 {
		return fmt.Errorf("savegame contains no edicts")
	}

	for i := entnum; i < s.NumEdicts; i++ {
		em.ED_ClearEdict(i)
	}
	s.NumEdicts = entnum
	s.QCVM.NumEdicts = entnum
	s.setQCTimeGlobal(state.Time)

	if s.Static != nil {
		if serverFlags := s.QCVM.GetGlobalInt("serverflags"); serverFlags != 0 || s.Static.ServerFlags != 0 {
			s.Static.ServerFlags = serverFlags
		}
		for i, client := range s.Static.Clients {
			if client == nil || i+1 >= len(s.Edicts) {
				continue
			}
			client.Edict = s.Edicts[i+1]
		}
	}

	return nil
}

func (s *Server) ensureTextSaveEdictCapacity(required int) error {
	if required <= len(s.Edicts) {
		if s.NumEdicts < required {
			s.NumEdicts = required
		}
		s.ensureQCVMEdictStorage()
		return nil
	}
	if required > s.MaxEdicts {
		return fmt.Errorf("savegame entity count %d exceeds max edicts %d", required, s.MaxEdicts)
	}

	extra := required - len(s.Edicts)
	s.Edicts = append(s.Edicts, make([]*Edict, extra)...)
	if s.NumEdicts < required {
		s.NumEdicts = required
	}
	s.ensureQCVMEdictStorage()
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseTextSaveLine(text string) (string, string) {
	text = strings.TrimLeft(text, " \t")
	if text == "" {
		return "", ""
	}

	line := text
	rest := ""
	if newline := strings.IndexByte(text, '\n'); newline >= 0 {
		line = text[:newline]
		rest = text[newline+1:]
	}

	line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	return rest, line
}

func parseTextSaveFloatLine(text string) (string, float32) {
	rest, line := parseTextSaveLine(text)
	if line == "" {
		return rest, 0
	}
	value, _ := strconv.ParseFloat(line, 32)
	return rest, float32(value)
}
