// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
)

func (h *Host) CmdMap(mapName string, subs *Subsystems) error {
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
	if err := handshake.LocalSignonReply(step); err != nil {
		return fmt.Errorf("%s handshake failed: %w", step, err)
	}
	h.signOns = handshake.LocalSignon()
	h.clientState = handshake.State()
	return nil
}

func (h *Host) CmdMapname(subs *Subsystems) {
	if subs.Console == nil {
		return
	}

	if h.serverActive {
		subs.Console.Print("mapname: (server active)\n")
	} else if h.clientState == caConnected {
		subs.Console.Print("mapname: (connected)\n")
	} else {
		subs.Console.Print("no map loaded\n")
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
