// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
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
	h.CmdMap(subs.Server.GetMapName(), subs)
}

func (h *Host) CmdChangelevel(level string, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}

	h.stopSessionSounds(subs)
	subs.Server.SaveSpawnParms()

	// Set LoadGame so ConnectClient preserves spawn parms (skips SetNewParms).
	// Mirrors C Ironwail: SV_SpawnServer sends reconnect to connected clients,
	// and SV_Spawn_f restores saved spawn parms from host_client->spawn_parms.
	subs.Server.SetLoadGame(true)

	if fsInstance, ok := subs.Files.(*fs.FileSystem); ok {
		if err := subs.Server.SpawnServer(level, fsInstance); err != nil {
			subs.Server.SetLoadGame(false)
			h.Error(fmt.Sprintf("failed to change level to %s: %v", level, err), subs)
			return
		}
	} else {
		subs.Server.SetLoadGame(false)
		return
	}

	if err := h.startLocalServerSession(subs, nil); err != nil {
		subs.Server.SetLoadGame(false)
		h.Error(fmt.Sprintf("failed to start session for %s: %v", level, err), subs)
		return
	}
	subs.Server.SetLoadGame(false)
}
