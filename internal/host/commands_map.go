// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"path/filepath"
	"sort"
	"strings"
)

func (h *Host) CmdMap(mapName string, subs *Subsystems) error {
	return h.CmdMapWithSpawnArgs(mapName, nil, subs)
}

func (h *Host) CmdMapWithSpawnArgs(mapName string, spawnArgs []string, subs *Subsystems) error {
	h.spawnArgs = formatSpawnArgs(spawnArgs)
	if subs == nil {
		subs = h.Subs
	}
	if subs == nil {
		fallbackClient := newLocalLoopbackClient()
		if err := fallbackClient.Init(); err != nil {
			return fmt.Errorf("subsystems not initialized")
		}
		if err := fallbackClient.LocalServerInfo(); err != nil {
			return err
		}
		if err := fallbackClient.LocalSignonReply("prespawn"); err != nil {
			return err
		}
		if err := fallbackClient.LocalSignonReply("spawn"); err != nil {
			return err
		}
		if err := fallbackClient.LocalSignonReply("begin"); err != nil {
			return err
		}
		h.signOns = fallbackClient.LocalSignon()
		h.clientState = fallbackClient.State()
		h.serverActive = false
		return nil
	}
	if subs.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	h.stopSessionSounds(subs)

	if h.demoState != nil && h.demoState.Playback {
		if err := h.demoState.StopPlayback(); err != nil && subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("Error stopping demo playback: %v\n", err))
		}
	}
	h.SetDemoNum(-1)

	h.clientState = caDisconnected
	h.serverActive = false

	if err := subs.Server.Init(h.maxClients); err != nil {
		return fmt.Errorf("failed to init server for map %s: %w", mapName, err)
	}

	if fsInstance, ok := subs.Files.(*fs.FileSystem); ok {
		if err := subs.Server.SpawnServer(mapName, fsInstance); err != nil {
			return fmt.Errorf("failed to spawn server for map %s: %w", mapName, err)
		}
	} else {
		return fmt.Errorf("filesystem implementation is missing")
	}

	h.resetAutosaveState()
	h.serverActive = true
	h.clientState = caDisconnected
	h.signOns = 0
	if h.dedicated {
		if subs.Client != nil {
			subs.Client.Shutdown()
			subs.Client = nil
		}
		h.updateServerBrowserNetworking(subs)
		return nil
	}

	return h.startLocalServerSession(subs, nil)
}

func (h *Host) runLocalHandshakeStep(step string, subs *Subsystems) error {
	if subs == nil || subs.Client == nil {
		return fmt.Errorf("client not initialized")
	}
	handshake, ok := subs.Client.(handshakeClient)
	if !ok {
		return fmt.Errorf("client does not support %s handshake", step)
	}
	if err := handshake.LocalSignonReply(h.signonCommand(step)); err != nil {
		return fmt.Errorf("%s handshake failed: %w", step, err)
	}
	h.signOns = handshake.LocalSignon()
	h.clientState = handshake.State()
	return nil
}

func (h *Host) signonCommand(step string) string {
	if strings.EqualFold(strings.TrimSpace(step), "spawn") && h.spawnArgs != "" {
		return "spawn " + h.spawnArgs
	}
	return step
}

func formatSpawnArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(args, " "))
}

func (h *Host) CmdMapname(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	if h.serverActive {
		subs.Console.Print(fmt.Sprintf("\"mapname\" is %q\n", subs.Server.GetMapName()))
		return
	}
	if h.clientState == caConnected {
		mapName := ""
		if clientState := ActiveClientState(subs); clientState != nil {
			mapName = clientState.MapName
		}
		subs.Console.Print(fmt.Sprintf("\"mapname\" is %q\n", mapName))
		return
	}
	subs.Console.Print("no map loaded\n")
}

func (h *Host) CmdMods(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil || subs.Files == nil {
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}

	filter := ""
	filterDisplay := ""
	if len(args) > 0 {
		filterDisplay = args[0]
		filter = strings.ToLower(strings.TrimSpace(args[0]))
	}

	count := 0
	for _, mod := range fsInstance.ListMods() {
		if filter != "" && !strings.Contains(strings.ToLower(mod.Name), filter) {
			continue
		}
		subs.Console.Print(fmt.Sprintf("   %s\n", mod.Name))
		count++
	}

	switch {
	case filter != "" && count == 0:
		subs.Console.Print(fmt.Sprintf("no mods found containing %q\n", filterDisplay))
	case filter != "":
		label := "mods"
		if count == 1 {
			label = "mod"
		}
		subs.Console.Print(fmt.Sprintf("%d %s containing %q\n", count, label, filterDisplay))
	case count == 0:
		subs.Console.Print("no mods found\n")
	default:
		label := "mods"
		if count == 1 {
			label = "mod"
		}
		subs.Console.Print(fmt.Sprintf("%d %s\n", count, label))
	}
}

func (h *Host) CmdGame(args []string, subs *Subsystems) {
	if subs == nil {
		subs = h.Subs
	}
	if subs == nil || subs.Files == nil || subs.Console == nil {
		return
	}

	fileSys, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}

	current := strings.TrimSpace(fileSys.GetGameDir())
	if current == "" {
		current = "id1"
	}
	if len(args) == 0 {
		subs.Console.Print(fmt.Sprintf("\"game\" is \"%s\"\n", current))
		return
	}
	if len(args) != 1 {
		subs.Console.Print("usage: game <gamedir>\n")
		return
	}

	targetRaw := strings.TrimSpace(args[0])
	if targetRaw == "" {
		subs.Console.Print("usage: game <gamedir>\n")
		return
	}
	target := strings.ToLower(targetRaw)
	if target == "." || target == ".." || strings.Contains(target, "/") || strings.Contains(target, `\`) || filepath.Clean(target) != target {
		subs.Console.Print("game: invalid gamedir\n")
		return
	}
	if target != "id1" {
		allowed := false
		for _, mod := range fileSys.ListMods() {
			if strings.EqualFold(mod.Name, target) {
				allowed = true
				break
			}
		}
		if !allowed {
			subs.Console.Print(fmt.Sprintf("game: unknown gamedir %q\n", targetRaw))
			return
		}
	}

	if strings.EqualFold(target, current) {
		subs.Console.Print(fmt.Sprintf("\"game\" is \"%s\"\n", current))
		return
	}

	baseDir := strings.TrimSpace(h.baseDir)
	if baseDir == "" {
		baseDir = fileSys.GetBaseDir()
	}
	if baseDir == "" {
		subs.Console.Print("game: base directory is not set\n")
		return
	}

	nextFS := fs.NewFileSystem()
	if err := nextFS.Init(baseDir, target); err != nil {
		subs.Console.Print(fmt.Sprintf("game: failed to switch to %q: %v\n", targetRaw, err))
		return
	}

	subs.Files.Close()
	subs.Files = nextFS
	h.gameDir = target
	if h.menu != nil {
		h.menu.SetCurrentMod(target)
	}
	subs.Console.Print(fmt.Sprintf("\"game\" changed to \"%s\"\n", target))
}

func (h *Host) CmdSkies(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil || subs.Files == nil {
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}

	filter := ""
	filterDisplay := ""
	if len(args) > 0 {
		filterDisplay = args[0]
		filter = strings.ToLower(strings.TrimSpace(args[0]))
	}

	skySet := map[string]struct{}{}
	for _, file := range fsInstance.ListFiles("gfx/env/*up.tga") {
		trimmed := strings.TrimPrefix(strings.ToLower(file), "gfx/env/")
		if !strings.HasSuffix(trimmed, "up.tga") {
			continue
		}
		base := strings.TrimSuffix(trimmed, "up.tga")
		if base == "" {
			continue
		}
		skySet[base] = struct{}{}
	}

	skies := make([]string, 0, len(skySet))
	for sky := range skySet {
		skies = append(skies, sky)
	}
	sort.Strings(skies)

	count := 0
	for _, sky := range skies {
		if filter != "" && !strings.Contains(sky, filter) {
			continue
		}
		subs.Console.Print(fmt.Sprintf("   %s\n", sky))
		count++
	}

	switch {
	case filter != "" && count == 0:
		subs.Console.Print(fmt.Sprintf("no skies found containing %q\n", filterDisplay))
	case filter != "":
		label := "skies"
		if count == 1 {
			label = "sky"
		}
		subs.Console.Print(fmt.Sprintf("%d %s containing %q\n", count, label, filterDisplay))
	case count == 0:
		subs.Console.Print("no skies found\n")
	default:
		label := "skies"
		if count == 1 {
			label = "sky"
		}
		subs.Console.Print(fmt.Sprintf("%d %s\n", count, label))
	}
}

func (h *Host) CmdRestart(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	if h.autoLoadLastSave(subs, false, func() {
		h.CmdRestart(subs)
	}) {
		return
	}
	h.CmdMap(subs.Server.GetMapName(), subs)
}

func (h *Host) CmdChangelevel(level string, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	if level == subs.Server.GetMapName() && h.autoLoadLastSave(subs, false, func() {
		h.CmdChangelevel(level, subs)
	}) {
		return
	}

	h.stopSessionSounds(subs)
	subs.Server.SaveSpawnParms()

	// Preserve spawn parms across the reconnect, but still let the destination
	// map run normal player spawn placement instead of treating this like a
	// savegame restore.
	subs.Server.SetPreserveSpawnParms(true)

	if fsInstance, ok := subs.Files.(*fs.FileSystem); ok {
		if err := subs.Server.SpawnServer(level, fsInstance); err != nil {
			subs.Server.SetPreserveSpawnParms(false)
			h.Error(fmt.Sprintf("failed to change level to %s: %v", level, err), subs)
			return
		}
	} else {
		subs.Server.SetPreserveSpawnParms(false)
		return
	}

	if err := h.startLocalServerSession(subs, nil); err != nil {
		subs.Server.SetPreserveSpawnParms(false)
		h.Error(fmt.Sprintf("failed to start session for %s: %v", level, err), subs)
		return
	}
	subs.Server.SetPreserveSpawnParms(false)
}

func (h *Host) autoLoadLastSave(subs *Subsystems, force bool, onDecline func()) bool {
	if subs == nil || subs.Server == nil || subs.Console == nil {
		return false
	}
	if subs.Server.GetMaxClients() != 1 || h.lastSave == "" {
		return false
	}
	if clientState := LoopbackClientState(subs); clientState != nil && clientState.Intermission != 0 {
		return false
	}
	mode := cvar.FloatValue("sv_autoload")
	if mode <= 0 {
		return false
	}
	if !force {
		if mode < 2 {
			if h.menu == nil {
				return false
			}
			saveName := h.lastSave
			h.menu.ShowConfirmationPrompt([]string{
				"LOAD LAST SAVE? (Y/N)",
				"PRESS Y OR ENTER TO LOAD",
				"PRESS N OR ESC TO CONTINUE",
			}, func() {
				h.CmdLoad(saveName, subs)
			}, func() {
				if h.lastSave == saveName {
					h.lastSave = ""
				}
				if onDecline != nil {
					onDecline()
				}
			}, 0)
			return true
		}
		player := subs.Server.EdictNum(1)
		if mode < 3 && player != nil && player.Vars != nil && player.Vars.Health > 0 {
			return false
		}
	}
	subs.Console.Print("Autoloading...\n")
	if err := h.loadSave(h.lastSave, loadSaveOptions{}, subs); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		subs.Console.Print("Autoload failed!\n")
		return false
	}
	return true
}
