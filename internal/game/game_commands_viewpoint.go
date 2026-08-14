package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/console"
)

type viewpointJSON struct {
	ID          string     `json:"id"`
	Game        string     `json:"game"`
	Map         string     `json:"map"`
	Pos         [3]float64 `json:"pos"`
	Angles      [3]float64 `json:"angles"`
	Tag         string     `json:"tag"`
	Description string     `json:"description"`
}

func round2(v float32) float64 {
	return math.Round(float64(v)*100) / 100
}

func (g *Game) formatViewpointJSON(args []string) (string, viewpointJSON) {
	camOrigin, camAngles := g.runtimeViewState()

	mapName := "start"
	if g.Client != nil && g.Client.LevelName != "" {
		mapName = strings.TrimSuffix(filepath.Base(g.Client.LevelName), ".bsp")
	} else if g.Server != nil && g.Server.MapName() != "" {
		mapName = g.Server.MapName()
	}

	gameDir := "id1"
	if g.Host != nil && g.Host.CVar != nil {
		if cvarVal := strings.TrimSpace(g.Host.CVar.StringValue("game")); cvarVal != "" {
			gameDir = cvarVal
		}
	}

	id := fmt.Sprintf("%s-%s-view", gameDir, mapName)
	desc := fmt.Sprintf("%s %s camera viewpoint", gameDir, mapName)
	tag := gameDir

	var remainingArgs []string
	shouldAppend := false

	for _, arg := range args {
		if arg == "-append" {
			shouldAppend = true
		} else if strings.TrimSpace(arg) != "" {
			remainingArgs = append(remainingArgs, arg)
		}
	}

	if len(remainingArgs) > 0 {
		id = remainingArgs[0]
	}
	if len(remainingArgs) > 1 {
		desc = strings.Join(remainingArgs[1:], " ")
	}

	vp := viewpointJSON{
		ID:   id,
		Game: gameDir,
		Map:  mapName,
		Pos: [3]float64{
			round2(camOrigin.X),
			round2(camOrigin.Y),
			round2(camOrigin.Z),
		},
		Angles: [3]float64{
			round2(camAngles.X),
			round2(camAngles.Y),
			round2(camAngles.Z),
		},
		Tag:         tag,
		Description: desc,
	}

	data, err := json.MarshalIndent(vp, "", "  ")
	if err != nil {
		return "", vp
	}

	if shouldAppend {
		g.appendViewpointToJSONFile(vp)
	}

	return string(data), vp
}

func (g *Game) appendViewpointToJSONFile(vp viewpointJSON) {
	projectDir, err := os.Getwd()
	if err != nil {
		console.Printf("viewpos_json: getwd error: %v\n", err)
		return
	}
	viewpointsPath := filepath.Join(projectDir, "testdata", "parity", "viewpoints.json")

	fileData, err := os.ReadFile(viewpointsPath)
	if err != nil {
		console.Printf("viewpos_json: cannot read %s: %v\n", viewpointsPath, err)
		return
	}

	type viewpointsHeader struct {
		BaseDir    string          `json:"basedir"`
		Viewpoints []viewpointJSON `json:"viewpoints"`
	}

	var cfg viewpointsHeader
	if err := json.Unmarshal(fileData, &cfg); err != nil {
		console.Printf("viewpos_json: unmarshal %s: %v\n", viewpointsPath, err)
		return
	}

	// Update existing ID or append new one
	updated := false
	for i, existing := range cfg.Viewpoints {
		if existing.ID == vp.ID {
			cfg.Viewpoints[i] = vp
			updated = true
			break
		}
	}
	if !updated {
		cfg.Viewpoints = append(cfg.Viewpoints, vp)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		console.Printf("viewpos_json: encode error: %v\n", err)
		return
	}

	if err := os.WriteFile(viewpointsPath, buf.Bytes(), 0o644); err != nil {
		console.Printf("viewpos_json: write %s: %v\n", viewpointsPath, err)
		return
	}
	console.Printf("viewpos_json: appended viewpoint %q to %s\n", vp.ID, viewpointsPath)
}

func (g *Game) cmdViewposJSON(args []string) {
	jsonStr, _ := g.formatViewpointJSON(args)
	if jsonStr == "" {
		console.Printf("viewpos_json: failed to format viewpoint JSON\n")
		return
	}
	console.Printf("%s\n", jsonStr)
}
