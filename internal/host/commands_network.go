// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"strconv"
	"strings"
)

func (h *Host) CmdStatus(subs *Subsystems) {
	if subs.Console == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString("host:    Ironwail Go\n")
	if h.serverActive && subs.Server != nil {
		sb.WriteString(fmt.Sprintf("map:     %s\n", subs.Server.GetMapName()))
		maxClients := subs.Server.GetMaxClients()
		activeCount := 0
		for i := 0; i < maxClients; i++ {
			if subs.Server.IsClientActive(i) {
				activeCount++
			}
		}
		sb.WriteString(fmt.Sprintf("players: %d active (%d max)\n", activeCount, maxClients))
		sb.WriteString("\nslot  name             ping\n")
		sb.WriteString("----  ---------------- ----\n")
		for i := 0; i < maxClients; i++ {
			if !subs.Server.IsClientActive(i) {
				continue
			}
			name := subs.Server.GetClientName(i)
			ping := subs.Server.GetClientPing(i)
			sb.WriteString(fmt.Sprintf("%4d  %-16s %4.0f\n", i, name, ping))
		}
	} else {
		sb.WriteString("map:     (no server active)\n")
	}

	subs.Console.Print(sb.String())
}

var bannedPlayers = make(map[string]bool)

func (h *Host) CmdBan(args []string, subs *Subsystems) {
	if !h.serverActive || subs.Server == nil || subs.Console == nil {
		return
	}
	if len(args) == 0 {
		subs.Console.Print("Banned names:\n")
		for name := range bannedPlayers {
			subs.Console.Print(fmt.Sprintf("  %s\n", name))
		}
		return
	}

	target := args[0]
	maxClients := subs.Server.GetMaxClients()

	found := false
	for i := 0; i < maxClients; i++ {
		if subs.Server.IsClientActive(i) && subs.Server.GetClientName(i) == target {
			bannedPlayers[target] = true
			subs.Server.KickClient(i, "host", "Banned by admin")
			subs.Console.Print(fmt.Sprintf("Banned and kicked %s\n", target))
			found = true
			break
		}
	}

	if !found {
		bannedPlayers[target] = true
		subs.Console.Print(fmt.Sprintf("Added %s to ban list\n", target))
	}
}

func (h *Host) CmdKick(args []string, subs *Subsystems) {
	if !h.serverActive || subs == nil || subs.Server == nil || len(args) == 0 {
		return
	}

	target := -1
	reasonStart := 1

	if len(args) > 1 && args[0] == "#" {
		slot, err := strconv.Atoi(args[1])
		if err != nil || slot <= 0 {
			return
		}
		target = slot - 1
		reasonStart = 2
		if !subs.Server.IsClientActive(target) {
			return
		}
	} else {
		for i := 0; i < subs.Server.GetMaxClients(); i++ {
			if !subs.Server.IsClientActive(i) {
				continue
			}
			if strings.EqualFold(subs.Server.GetClientName(i), args[0]) {
				target = i
				break
			}
		}
	}

	if target < 0 || target == 0 {
		return
	}

	who := subs.Server.GetClientName(0)
	if who == "" {
		who = "Console"
	}

	var reason string
	if len(args) > reasonStart {
		reason = strings.Join(args[reasonStart:], " ")
	}
	subs.Server.KickClient(target, who, reason)
}

func (h *Host) CmdSay(message string, subs *Subsystems) {
	if subs.Client == nil || message == "" {
		return
	}
	subs.Client.SendStringCmd(fmt.Sprintf("say %s", message))
}

func (h *Host) CmdSayTeam(message string, subs *Subsystems) {
	if subs.Client == nil || message == "" {
		return
	}
	subs.Client.SendStringCmd(fmt.Sprintf("say_team %s", message))
}

func (h *Host) CmdTell(args []string, subs *Subsystems) {
	if subs.Client == nil || len(args) < 2 {
		return
	}
	subs.Client.SendStringCmd(fmt.Sprintf("tell %s", strings.Join(args, " ")))
}

func (h *Host) CmdServerInfo(subs *Subsystems) {
	if subs.Console == nil {
		return
	}

	subs.Console.Print(fmt.Sprintf("Server info:\n"))
	subs.Console.Print(fmt.Sprintf("  host:      %s\n", currentServerHostname()))
	subs.Console.Print(fmt.Sprintf("  active:    %v\n", h.serverActive))
	subs.Console.Print(fmt.Sprintf("  paused:    %v\n", h.serverPaused))
	subs.Console.Print(fmt.Sprintf("  maxclients: %d\n", h.maxClients))
	subs.Console.Print(fmt.Sprintf("  skill:     %d\n", h.currentSkill))
}

// CmdSlist initiates a LAN server search and prints discovered servers
// to the console, matching the C Ironwail "slist" command.
func (h *Host) CmdSlist(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	subs.Console.Print("Searching for LAN servers...\n")

	sb := inet.NewServerBrowser()
	sb.Start()
	sb.Wait()

	results := sb.Results()
	if len(results) == 0 {
		subs.Console.Print("No servers found.\n")
		return
	}
	subs.Console.Print(fmt.Sprintf("Found %d server(s):\n", len(results)))
	for _, entry := range results {
		subs.Console.Print(fmt.Sprintf("  %s\n", entry.String()))
	}
}

func (h *Host) EndGame(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Host_EndGame: %s\n", message))
	}

	if h.serverActive {
		h.ShutdownServer(subs)
	}

	h.clientState = caDisconnected
	h.Abort(message)
}

func (h *Host) ShutdownServer(subs *Subsystems) {
	if !h.serverActive {
		return
	}

	h.serverActive = false
	h.serverPaused = false

	if subs == nil {
		subs = h.Subs
	}
	h.updateServerBrowserNetworking(subs)

	if subs != nil && subs.Server != nil {
		subs.Server.Shutdown()
	}
}

func (h *Host) CmdConnect(address string, subs *Subsystems) {
	h.SetDemoNum(-1)
	address = strings.TrimSpace(address)
	if address == "" {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: connect <server>\n")
		}
		return
	}

	if subs == nil {
		subs = h.Subs
	}

	isLocal := strings.EqualFold(address, "local")
	if isLocal && h.serverActive && subs != nil && subs.Server != nil {
		h.disconnectCurrentSession(subs, false)
		h.CmdReconnect(subs)
		return
	}

	h.disconnectCurrentSession(subs, true)

	if isLocal {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("No local server is active.\n")
		}
		return
	}
	if err := h.startRemoteSession(address, subs); err != nil {
		if subs != nil && subs.Console != nil {
			msg := fmt.Sprintf("connect %q failed: %v", address, err)
			// Check if the client knows the reason
			if remote, ok := subs.Client.(interface{ Error() string }); ok {
				if reason := remote.Error(); reason != "" {
					msg = fmt.Sprintf("connect %q rejected: %s", address, reason)
				}
			}
			subs.Console.Print(msg + "\n")
		}
		return
	}
	h.CmdReconnect(subs)
	if subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Connecting to %s...\n", address))
	}
}

func (h *Host) CmdDisconnect(subs *Subsystems) {
	h.disconnectCurrentSession(subs, true)
	if subs != nil && subs.Console != nil {
		subs.Console.Print("Disconnected.\n")
	}
}

func (h *Host) disconnectCurrentSession(subs *Subsystems, stopServer bool) {
	if subs == nil {
		subs = h.Subs
	}

	h.stopSessionSounds(subs)

	if h.demoState != nil && h.demoState.Playback {
		if err := h.demoState.StopPlayback(); err != nil && subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("Error stopping demo playback: %v\n", err))
		}
	}

	if stopServer && h.serverActive {
		h.ShutdownServer(subs)
	}
	if subs != nil && subs.Client != nil {
		subs.Client.Shutdown()
	}

	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		loopbackClient.ClearState()
		loopbackClient.State = cl.StateDisconnected
	}

	h.signOns = 0
	h.clientState = caDisconnected
}

func (h *Host) CmdReconnect(subs *Subsystems) {
	if h.demoState != nil && h.demoState.Playback {
		return
	}

	if subs == nil {
		subs = h.Subs
	}
	if subs == nil || subs.Client == nil {
		return
	}

	h.BeginLoadingTransitionPlaque(0)
	h.stopSessionSounds(subs)

	if h.serverActive && subs.Server != nil {
		if err := h.startLocalServerSession(subs, nil); err != nil {
			if subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("reconnect failed: %v\n", err))
			}
		}
		return
	}

	remoteReset := false
	if remoteClient, ok := subs.Client.(reconnectResetClient); ok {
		if err := remoteClient.ResetConnectionState(); err != nil {
			if subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("reconnect reset failed: %v\n", err))
			}
		} else {
			remoteReset = true
		}
	}

	if !remoteReset {
		if clientState := ActiveClientState(subs); clientState != nil {
			clientState.ClearSignons()
			if clientState.State != cl.StateDisconnected {
				clientState.State = cl.StateConnected
			}
		}
	}

	h.signOns = 0
	if h.clientState != caDisconnected {
		h.clientState = caConnected
	}
}

func (h *Host) CmdName(name string, subs *Subsystems) {
	cvar.Set(clientNameCVar, name)
	if subs.Server != nil {
		subs.Server.SetClientName(0, name)
	}
}

func (h *Host) CmdColor(args []string, subs *Subsystems) {
	if len(args) == 0 {
		return
	}

	var top, bottom int
	fmt.Sscanf(args[0], "%d", &top)
	if len(args) == 1 {
		bottom = top
	} else {
		fmt.Sscanf(args[1], "%d", &bottom)
	}
	top = clampClientColor(top)
	bottom = clampClientColor(bottom)
	color := top*16 + bottom
	cvar.SetInt(clientColorCVar, color)
	if subs.Server != nil {
		subs.Server.SetClientColor(0, color)
	}
}

func clampClientColor(value int) int {
	value &= 15
	if value > 13 {
		return 13
	}
	return value
}

func currentServerHostname() string {
	if value := cvar.StringValue(serverHostnameCVar); value != "" {
		return value
	}
	return defaultServerHostname
}

func (h *Host) runHandshakeStep(step string, subs *Subsystems) error {
	if h.serverActive {
		return h.runLocalHandshakeStep(step, subs)
	}
	if subs == nil || subs.Client == nil {
		return fmt.Errorf("client not initialized")
	}
	remoteClient, ok := subs.Client.(signonCommandClient)
	if !ok {
		return fmt.Errorf("client does not support %s handshake", step)
	}
	if err := remoteClient.SendSignonCommand(step); err != nil {
		return fmt.Errorf("%s handshake failed: %w", step, err)
	}
	if state := ActiveClientState(subs); state != nil {
		h.signOns = state.Signon
	}
	h.clientState = subs.Client.State()
	return nil
}

func (h *Host) startRemoteSession(address string, subs *Subsystems) error {
	if subs == nil {
		return fmt.Errorf("subsystems not initialized")
	}
	remoteClient, err := remoteClientFactory(address)
	if err != nil {
		return err
	}
	subs.Client = remoteClient
	h.serverActive = false
	h.clientState = caConnected
	h.signOns = 0
	return nil
}

func (h *Host) stopSessionSounds(subs *Subsystems) {
	if subs == nil || subs.Audio == nil {
		return
	}
	subs.Audio.StopAllSounds(true)
}
