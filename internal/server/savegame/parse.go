// This file contains the standalone text savegame parser, which does not
// depend on Server internals and can live in the sub-package.
package savegame

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/common"
)

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
