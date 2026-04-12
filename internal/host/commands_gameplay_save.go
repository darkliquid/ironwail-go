// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/server"
)

type loadSaveOptions struct {
	kexOnly bool
}

type decodedHostSaveFile struct {
	native *hostSaveFile
	text   *server.TextSaveGameState
}

func (d decodedHostSaveFile) mapName() string {
	switch {
	case d.native != nil && d.native.Server != nil:
		return d.native.Server.MapName
	case d.text != nil:
		return d.text.MapName
	default:
		return ""
	}
}

func (d decodedHostSaveFile) skill() int {
	switch {
	case d.native != nil:
		return d.native.Skill
	case d.text != nil:
		return d.text.Skill
	default:
		return 0
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
	if save.native == nil && save.text == nil {
		return fmt.Errorf("savegame is empty")
	}
	if save.native != nil && save.native.Server == nil {
		return fmt.Errorf("savegame is missing server state")
	}
	if save.text != nil {
		activeGameDir := strings.TrimSpace(h.gameDir)
		if activeGameDir == "" {
			activeGameDir = "id1"
		}
		targetGameDir := strings.TrimSpace(save.text.GameDir)
		if targetGameDir != "" && !strings.EqualFold(targetGameDir, activeGameDir) {
			h.invalidateLastSave(name)
			return fmt.Errorf("ERROR: KEX savegame targets game %s, but the active game is %s", targetGameDir, activeGameDir)
		}
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
	if err := subs.Server.SpawnServer(save.mapName(), fsInstance); err != nil {
		return fmt.Errorf("Couldn't load map")
	}
	if err := h.startLocalServerSession(subs, func() error {
		if save.native != nil {
			if err := srv.RestoreSaveGameState(save.native.Server); err != nil {
				return err
			}
		} else if save.text != nil {
			if err := srv.RestoreTextSaveGameState(save.text); err != nil {
				return err
			}
		}
		h.currentSkill = save.skill()
		cvar.SetInt("skill", save.skill())
		return nil
	}); err != nil {
		return err
	}
	h.syncAutosaveLastTimeFromServer(srv)
	h.setLastSave(name)
	return nil
}

func (h *Host) syncAutosaveLastTimeFromServer(srv *server.Server) {
	if h == nil || srv == nil {
		return
	}
	h.autosave.lastTime = float64(srv.Time)
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

// SaveEntryAllowed reports whether entering the Single Player -> Save menu item
// should be permitted, mirroring cmdSave's top-level gating.
func (h *Host) SaveEntryAllowed(subs *Subsystems) bool {
	if subs == nil || subs.Server == nil || !h.serverActive || !subs.Server.IsActive() {
		return false
	}
	if subs.Server.GetMaxClients() != 1 {
		return false
	}
	if clientState := LoopbackClientState(subs); clientState != nil && clientState.Intermission != 0 {
		return false
	}
	return true
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
	h.BeginSavingIndicator(0)

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

func decodeHostSaveFile(data []byte, options loadSaveOptions) (decodedHostSaveFile, error) {
	var (
		save    hostSaveFile
		decoded decodedHostSaveFile
	)

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return decoded, fmt.Errorf("savegame is empty")
	}

	expectedVersion := expectedSaveVersion(options)
	if trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &save); err != nil {
			return decoded, err
		}
		if save.Version != expectedVersion {
			return decoded, fmt.Errorf("ERROR: Savegame is version %d, not %d", save.Version, expectedVersion)
		}
		decoded.native = &save
		return decoded, nil
	}

	if version, ok := sniffTextSaveVersion(trimmed); ok {
		if version != expectedVersion {
			return decoded, fmt.Errorf("ERROR: Savegame is version %d, not %d", version, expectedVersion)
		}
		parsed, err := server.ParseTextSaveGame(trimmed)
		if err != nil {
			return decoded, fmt.Errorf("ERROR: couldn't parse text savegame: %w", err)
		}
		decoded.text = parsed
		return decoded, nil
	}

	if err := json.Unmarshal(trimmed, &save); err != nil {
		return decoded, err
	}
	if save.Version != expectedVersion {
		return decoded, fmt.Errorf("ERROR: Savegame is version %d, not %d", save.Version, expectedVersion)
	}
	decoded.native = &save
	return decoded, nil
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

		foundPath, data, err := h.readSaveFile(slotName, loadSaveOptions{})
		if err != nil {
			slots = append(slots, slot)
			continue
		}

		options := h.effectiveLoadSaveOptions(slotName, foundPath, loadSaveOptions{})
		decoded, err := decodeHostSaveFile(data, options)
		if err != nil {
			slots = append(slots, slot)
			continue
		}

		switch {
		case decoded.native != nil && decoded.native.Server != nil:
			if mapName := strings.TrimSpace(decoded.native.Server.MapName); mapName != "" {
				slot.DisplayName = mapName
			}
		case decoded.text != nil:
			if title := strings.TrimSpace(decoded.text.Title); title != "" {
				slot.DisplayName = title
			} else if mapName := strings.TrimSpace(decoded.text.MapName); mapName != "" {
				slot.DisplayName = mapName
			}
		}

		slots = append(slots, slot)
	}

	return slots
}
