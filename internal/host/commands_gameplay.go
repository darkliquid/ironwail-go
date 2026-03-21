// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/server"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
	"math"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type loadSaveOptions struct {
	kexOnly bool
}

func (h *Host) CmdSkill(skill int) {
	if skill < 0 {
		skill = 0
	}
	if skill > 3 {
		skill = 3
	}
	h.currentSkill = skill
}

func (h *Host) CmdPause(subs *Subsystems) {
	if h.forwardClientCommand("pause", nil, subs) {
		return
	}
	if h.serverActive && h.maxClients == 1 {
		h.serverPaused = !h.serverPaused
	}
}

func (h *Host) CmdGod(subs *Subsystems) {
	if h.forwardClientCommand("god", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) ^ server.FlagGodMode)
	if subs.Console != nil {
		if uint32(ent.Vars.Flags)&server.FlagGodMode != 0 {
			subs.Console.Print("godmode ON\n")
		} else {
			subs.Console.Print("godmode OFF\n")
		}
	}
}
func (h *Host) CmdNoClip(subs *Subsystems) {
	if h.forwardClientCommand("noclip", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	if server.MoveType(ent.Vars.MoveType) == server.MoveTypeNoClip {
		ent.Vars.MoveType = float32(server.MoveTypeWalk)
		if subs.Console != nil {
			subs.Console.Print("noclip OFF\n")
		}
	} else {
		ent.Vars.MoveType = float32(server.MoveTypeNoClip)
		if subs.Console != nil {
			subs.Console.Print("noclip ON\n")
		}
	}
}
func (h *Host) CmdFly(subs *Subsystems) {
	if h.forwardClientCommand("fly", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	if server.MoveType(ent.Vars.MoveType) == server.MoveTypeFly {
		ent.Vars.MoveType = float32(server.MoveTypeWalk)
		if subs.Console != nil {
			subs.Console.Print("fly OFF\n")
		}
	} else {
		ent.Vars.MoveType = float32(server.MoveTypeFly)
		if subs.Console != nil {
			subs.Console.Print("fly ON\n")
		}
	}
}
func (h *Host) CmdNotarget(subs *Subsystems) {
	if h.forwardClientCommand("notarget", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) ^ server.FlagNoTarget)
	if subs.Console != nil {
		if uint32(ent.Vars.Flags)&server.FlagNoTarget != 0 {
			subs.Console.Print("notarget ON\n")
		} else {
			subs.Console.Print("notarget OFF\n")
		}
	}
}
func (h *Host) CmdTracepos(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil || subs.Console == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}

	forward, _, _ := qtypes.AngleVectors(qtypes.Vec3{X: ent.Vars.VAngle[0], Y: ent.Vars.VAngle[1], Z: ent.Vars.VAngle[2]})
	start := ent.Vars.Origin
	start[2] += 22 // eye height
	end := [3]float32{
		start[0] + forward.X*8192,
		start[1] + forward.Y*8192,
		start[2] + forward.Z*8192,
	}

	srv, _ := subs.Server.(*server.Server)
	trace := srv.Move(start, [3]float32{}, [3]float32{}, end, server.MoveType(server.MoveNormal), ent)

	subs.Console.Print(fmt.Sprintf("trace at: %.1f %.1f %.1f\n", trace.EndPos[0], trace.EndPos[1], trace.EndPos[2]))
	subs.Console.Print(fmt.Sprintf("fraction: %.4f\n", trace.Fraction))
	if trace.Entity != nil {
		entNum := srv.NumForEdict(trace.Entity)
		className := srv.GetString(trace.Entity.Vars.ClassName)
		subs.Console.Print(fmt.Sprintf("hit entity %d: %s\n", entNum, className))
	} else {
		subs.Console.Print("hit world\n")
	}
	subs.Console.Print(fmt.Sprintf("plane normal: %.2f %.2f %.2f\n", trace.PlaneNormal[0], trace.PlaneNormal[1], trace.PlaneNormal[2]))
}

func (h *Host) CmdSoundinfo(subs *Subsystems) {
	if subs.Audio == nil || subs.Console == nil {
		return
	}
	subs.Console.Print(subs.Audio.SoundInfo())
}

func (h *Host) CmdParticleTexture(mode string, subs *Subsystems) {
	if subs.Console == nil {
		return
	}
	cvar.Set("r_particles", mode)
	subs.Console.Print(fmt.Sprintf("particle_texture set to %s\n", mode))
}

func (h *Host) CmdMaps(subs *Subsystems) {
	if subs == nil || subs.Files == nil || subs.Console == nil {
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}
	files := fsInstance.ListFiles("maps/*.bsp")
	subs.Console.Print("Maps found:\n")
	for _, f := range files {
		name := filepath.Base(f)
		name = strings.TrimSuffix(name, ".bsp")
		subs.Console.Print(fmt.Sprintf("  %s\n", name))
	}
}

// CmdRandmap picks a random map from the available maps and changes to it.
func (h *Host) CmdRandmap(subs *Subsystems) {
	if subs == nil || subs.Files == nil || subs.Console == nil {
		return
	}
	if !h.serverActive || subs.Server == nil {
		subs.Console.Print("randmap: no server running\n")
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}
	files := fsInstance.ListFiles("maps/*.bsp")
	if len(files) == 0 {
		subs.Console.Print("randmap: no maps found\n")
		return
	}

	// Build list of map names
	maps := make([]string, 0, len(files))
	for _, f := range files {
		name := filepath.Base(f)
		name = strings.TrimSuffix(name, ".bsp")
		maps = append(maps, name)
	}

	choice := maps[rand.Intn(len(maps))]
	subs.Console.Print(fmt.Sprintf("randmap: changing to %s\n", choice))
	h.CmdMap(choice, subs)
}

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

func (h *Host) CmdKill(subs *Subsystems) {
	if h.forwardClientCommand("kill", nil, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil {
		return
	}
	subs.Server.KillClient(0)
}

func (h *Host) CmdSpawn(subs *Subsystems) {
	if err := h.runHandshakeStep("spawn", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("spawn failed: %v\n", err))
	}
}

func (h *Host) CmdBegin(subs *Subsystems) {
	if err := h.runHandshakeStep("begin", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("begin failed: %v\n", err))
	}
}

func (h *Host) CmdPreSpawn(subs *Subsystems) {
	if err := h.runHandshakeStep("prespawn", subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("prespawn failed: %v\n", err))
	}
}

func (h *Host) CmdPing(subs *Subsystems) {
	if h.forwardClientCommand("ping", nil, subs) {
		return
	}
	if subs.Server == nil || subs.Console == nil {
		return
	}
	maxClients := subs.Server.GetMaxClients()
	subs.Console.Print("Client pings:\n")
	for i := 0; i < maxClients; i++ {
		name := subs.Server.GetClientName(i)
		if name == "" {
			continue
		}
		ping := subs.Server.GetClientPing(i)
		subs.Console.Print(fmt.Sprintf("  %d: %-16s %.0f ms\n", i, name, ping))
	}
}

func (h *Host) CmdLoad(name string, subs *Subsystems) {
	if err := h.loadSave(name, loadSaveOptions{}, subs); err != nil && subs != nil && subs.Console != nil {
		subs.Console.Print(err.Error() + "\n")
	}
}

func (h *Host) loadSave(name string, options loadSaveOptions, subs *Subsystems) error {
	if subs == nil || subs.Console == nil {
		return nil
	}
	displayName, err := saveDisplayName(name)
	if err != nil {
		return err
	}
	if cvar.BoolValue("nomonsters") {
		subs.Console.Print("Warning: \"nomonsters\" disabled automatically.\n")
		cvar.Set("nomonsters", "0")
	}
	path, data, err := h.readSaveFile(name, options)
	if err != nil {
		h.invalidateLastSave(name)
		return err
	}
	subs.Console.Print(fmt.Sprintf("Loading game from %s...\n", displayName))
	effectiveOptions := h.effectiveLoadSaveOptions(name, path, options)
	save, err := decodeHostSaveFile(data, effectiveOptions)
	if err != nil {
		h.invalidateLastSave(name)
		return err
	}
	if save.Server == nil {
		return fmt.Errorf("savegame is missing server state")
	}
	if subs.Server == nil {
		return fmt.Errorf("server is not initialized")
	}
	if subs.Server.GetMaxClients() != 1 {
		return fmt.Errorf("savegames require single-player mode")
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		return fmt.Errorf("savegames require the built-in server")
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return fmt.Errorf("filesystem implementation is missing")
	}

	h.BeginLoadingTransitionPlaque(0)
	h.stopSessionSounds(subs)

	if h.demoState != nil && h.demoState.Playback {
		if err := h.demoState.StopPlayback(); err != nil && subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("Error stopping demo playback: %v\n", err))
		}
	}
	h.SetDemoNum(-1)

	h.serverActive = false
	h.clientState = caDisconnected
	h.signOns = 0

	if err := subs.Server.Init(h.maxClients); err != nil {
		return err
	}
	srv.LoadGame = true
	defer func() { srv.LoadGame = false }()
	if err := subs.Server.SpawnServer(save.Server.MapName, fsInstance); err != nil {
		return fmt.Errorf("Couldn't load map")
	}
	if err := h.startLocalServerSession(subs, func() error {
		if err := srv.RestoreSaveGameState(save.Server); err != nil {
			return err
		}
		h.currentSkill = save.Skill
		cvar.SetInt("skill", save.Skill)
		return nil
	}); err != nil {
		return err
	}
	h.setLastSave(name)
	return nil
}

func (h *Host) CmdLoadArgs(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if len(args) == 0 {
		subs.Console.Print("load <savename> : load a game\n")
		return
	}
	if err := h.loadSave(args[0], loadSaveOptions{
		kexOnly: len(args) >= 2 && strings.EqualFold(strings.TrimSpace(args[1]), "kex"),
	}, subs); err != nil {
		subs.Console.Print(err.Error() + "\n")
	}
}

func (h *Host) CmdSave(name string, subs *Subsystems) {
	h.cmdSave(name, subs, false)
}

func (h *Host) CmdSaveArgs(args []string, subs *Subsystems) {
	if len(args) == 0 {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("save <savename> : save a game\n")
		}
		return
	}
	skipNotify := len(args) >= 2 && isFalseySaveNotifyArg(args[1])
	h.cmdSave(args[0], subs, skipNotify)
}

func isFalseySaveNotifyArg(arg string) bool {
	value, err := strconv.ParseFloat(strings.TrimSpace(arg), 64)
	return err == nil && value == 0
}

func (h *Host) cmdSave(name string, subs *Subsystems, skipNotify bool) {
	if subs == nil || subs.Console == nil {
		return
	}
	if subs.Server == nil || !h.serverActive || !subs.Server.IsActive() {
		subs.Console.Print("Not playing a local game.\n")
		return
	}
	if subs.Server.GetMaxClients() != 1 {
		subs.Console.Print("Can't save multiplayer games.\n")
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		subs.Console.Print("save failed: savegames require the built-in server\n")
		return
	}
	if cvar.BoolValue("nomonsters") {
		subs.Console.Print("Can't save when using \"nomonsters\".\n")
		return
	}
	if clientState := LoopbackClientState(subs); clientState != nil && clientState.Intermission != 0 {
		subs.Console.Print("Can't save in intermission.\n")
		return
	}
	if srv.Static != nil {
		for _, client := range srv.Static.Clients {
			if client == nil || !client.Active || client.Edict == nil || client.Edict.Vars == nil {
				continue
			}
			if client.Edict.Vars.Health <= 0 {
				subs.Console.Print("Can't savegame with a dead player\n")
				return
			}
		}
	}
	path, err := h.saveFilePath(name)
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	displayName, err := saveDisplayName(name)
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	state, err := srv.CaptureSaveGameState()
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	save := hostSaveFile{
		Version: server.SaveGameVersion,
		Skill:   h.currentSkill,
		Server:  state,
	}
	data, err := json.MarshalIndent(save, "", "  ")
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		subs.Console.Print("ERROR: couldn't open.\n")
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		subs.Console.Print("ERROR: couldn't open.\n")
		return
	}
	h.setLastSave(name)

	if !skipNotify {
		subs.Console.Print(fmt.Sprintf("Saving game to %s...\n", displayName))
	}
}

func (h *Host) setLastSave(name string) {
	if relName, err := normalizeSaveName(name); err == nil {
		h.lastSave = relName
	}
}

func (h *Host) invalidateLastSave(name string) {
	relName, err := normalizeSaveName(name)
	if err != nil {
		return
	}
	if relName == h.lastSave {
		h.lastSave = ""
	}
}

func (h *Host) saveFilePath(name string) (string, error) {
	relName, err := normalizeSaveName(name)
	if err != nil {
		return "", err
	}
	if h.userDir == "" {
		return "", fmt.Errorf("user directory is not initialized")
	}
	return filepath.Join(h.userDir, "saves", filepath.FromSlash(relName)+".sav"), nil
}

func saveDisplayName(name string) (string, error) {
	relName, err := normalizeSaveName(name)
	if err != nil {
		return "", err
	}
	return relName + ".sav", nil
}

func normalizeSaveName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" {
		return "", fmt.Errorf("save name is required")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("Relative pathnames are not allowed.")
	}
	clean := strings.TrimPrefix(path.Clean(name), "./")
	if clean == "." || clean == "" || strings.HasPrefix(clean, "/") {
		return "", fmt.Errorf("invalid save name %q", name)
	}
	for _, segment := range strings.Split(clean, "/") {
		if !saveNamePattern.MatchString(segment) {
			return "", fmt.Errorf("invalid save name %q", name)
		}
	}
	return clean, nil
}

func expectedSaveVersion(options loadSaveOptions) int {
	if options.kexOnly {
		return server.SaveGameVersionKEX
	}
	return server.SaveGameVersion
}

func (h *Host) effectiveLoadSaveOptions(name, foundPath string, options loadSaveOptions) loadSaveOptions {
	if options.kexOnly || foundPath == "" || h.baseDir == "" {
		return options
	}
	relName, err := normalizeSaveName(name)
	if err != nil {
		return options
	}
	installRootPath := filepath.Join(h.baseDir, filepath.FromSlash(relName)+".sav")
	if filepath.Clean(foundPath) == filepath.Clean(installRootPath) {
		options.kexOnly = true
	}
	return options
}

func decodeHostSaveFile(data []byte, options loadSaveOptions) (hostSaveFile, error) {
	var save hostSaveFile

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return save, fmt.Errorf("savegame is empty")
	}

	expectedVersion := expectedSaveVersion(options)
	if trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &save); err != nil {
			return save, err
		}
		if save.Version != expectedVersion {
			return save, fmt.Errorf("ERROR: Savegame is version %d, not %d", save.Version, expectedVersion)
		}
		return save, nil
	}

	if version, ok := sniffTextSaveVersion(trimmed); ok {
		if version != expectedVersion {
			return save, fmt.Errorf("ERROR: Savegame is version %d, not %d", version, expectedVersion)
		}
		if version == server.SaveGameVersionKEX {
			return save, fmt.Errorf("ERROR: KEX savegame format is not supported yet.")
		}
	}

	if err := json.Unmarshal(trimmed, &save); err != nil {
		return save, err
	}
	if save.Version != expectedVersion {
		return save, fmt.Errorf("ERROR: Savegame is version %d, not %d", save.Version, expectedVersion)
	}
	return save, nil
}

func sniffTextSaveVersion(data []byte) (int, bool) {
	line := data
	if newline := bytes.IndexByte(line, '\n'); newline >= 0 {
		line = line[:newline]
	}
	line = bytes.TrimSpace(bytes.TrimSuffix(line, []byte{'\r'}))
	if len(line) == 0 {
		return 0, false
	}
	version, err := strconv.Atoi(string(line))
	if err != nil {
		return 0, false
	}
	return version, true
}

func (h *Host) readSaveFile(name string, options loadSaveOptions) (string, []byte, error) {
	searchPaths, err := h.saveFileSearchPaths(name, options)
	if err != nil {
		return "", nil, err
	}
	displayName, err := saveDisplayName(name)
	if err != nil {
		return "", nil, err
	}

	for _, path := range searchPaths {
		data, err := os.ReadFile(path)
		if err == nil {
			return path, data, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, fmt.Errorf("ERROR: couldn't open.")
		}
	}

	return "", nil, fmt.Errorf("ERROR: %s not found.", displayName)
}

func (h *Host) saveFileSearchPaths(name string, options loadSaveOptions) ([]string, error) {
	userPath, err := h.saveFilePath(name)
	if err != nil {
		return nil, err
	}

	relName, err := normalizeSaveName(name)
	if err != nil {
		return nil, err
	}
	legacyName := filepath.FromSlash(relName) + ".sav"
	if options.kexOnly {
		if h.baseDir == "" {
			return nil, nil
		}
		return []string{filepath.Join(h.baseDir, legacyName)}, nil
	}

	searchPaths := []string{userPath}
	if h.baseDir == "" {
		return searchPaths, nil
	}

	// 2. Active game directory
	if gameDir := strings.TrimSpace(h.gameDir); gameDir != "" {
		searchPaths = append(searchPaths, filepath.Join(h.baseDir, gameDir, legacyName))
		searchPaths = append(searchPaths, filepath.Join(h.baseDir, gameDir, "saves", legacyName))
	}

	// 3. Base directory
	searchPaths = append(searchPaths, filepath.Join(h.baseDir, legacyName))

	// 4. Vanilla Quake directory
	if strings.TrimSpace(h.gameDir) != "id1" {
		searchPaths = append(searchPaths, filepath.Join(h.baseDir, "id1", legacyName))
		searchPaths = append(searchPaths, filepath.Join(h.baseDir, "id1", "saves", legacyName))
	}

	return searchPaths, nil
}

func (h *Host) ListSaveSlots(count int) []SaveSlotInfo {
	if count <= 0 {
		count = 12
	}

	slots := make([]SaveSlotInfo, 0, count)
	for i := 0; i < count; i++ {
		slotName := fmt.Sprintf("s%d", i)
		slot := SaveSlotInfo{
			Name:        slotName,
			DisplayName: unusedSaveSlotDisplay,
		}

		_, data, err := h.readSaveFile(slotName, loadSaveOptions{})
		if err != nil {
			slots = append(slots, slot)
			continue
		}

		var save hostSaveFile
		if err := json.Unmarshal(data, &save); err != nil {
			slots = append(slots, slot)
			continue
		}

		if save.Server != nil {
			if mapName := strings.TrimSpace(save.Server.MapName); mapName != "" {
				slot.DisplayName = mapName
			}
		}

		slots = append(slots, slot)
	}

	return slots
}

func (h *Host) CmdGive(item, value string, subs *Subsystems) {
	if h.forwardClientCommand("give", []string{item, value}, subs) {
		return
	}
	if !h.serverActive || subs.Server == nil || subs.Console == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}

	val := float32(0)
	fmt.Sscanf(value, "%f", &val)
	if val <= 0 {
		val = 100
	}

	switch item {
	case "h":
		ent.Vars.Health += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f health\n", val))
	case "s":
		ent.Vars.AmmoShells += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f shells\n", val))
	case "n":
		ent.Vars.AmmoNails += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f nails\n", val))
	case "r":
		ent.Vars.AmmoRockets += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f rockets\n", val))
	case "c":
		ent.Vars.AmmoCells += val
		subs.Console.Print(fmt.Sprintf("Gave %.0f cells\n", val))
	default:
		subs.Console.Print(fmt.Sprintf("give %s %s (not supported)\n", item, value))
	}
}
func (h *Host) getLocalPlayerEdict(subs *Subsystems) *server.Edict {
	if subs.Server == nil {
		return nil
	}
	// In single player, local player is always client 0, which is edict 1
	return subs.Server.EdictNum(1)
}

func (h *Host) CmdToggleMenu() {
	if h.menu == nil {
		return
	}
	h.menu.ToggleMenu()
}

func (h *Host) CmdMenuMain() {
	if h.menu == nil {
		return
	}
	h.menu.ShowMenu()
}

func (h *Host) CmdMenuQuit() {
	if h.menu == nil {
		return
	}
	h.menu.ShowQuitPrompt()
}

// Demo commands
