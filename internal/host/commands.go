// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/server"
)

var saveNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

var remoteClientFactory = func(address string) (Client, error) {
	socket := inet.Connect(address)
	if socket == nil {
		return nil, fmt.Errorf("unable to connect to %s", address)
	}
	client := newRemoteDatagramClient(socket)
	if err := client.Init(); err != nil {
		client.Shutdown()
		return nil, err
	}
	return client, nil
}

const maxAliasName = 32

type hostSaveFile struct {
	Version int                   `json:"version"`
	Skill   int                   `json:"skill"`
	Server  *server.SaveGameState `json:"server"`
}

type SaveSlotInfo struct {
	Name        string
	DisplayName string
}

const unusedSaveSlotDisplay = "--- UNUSED SLOT ---"

type handshakeClient interface {
	Client
	LocalServerInfo() error
	LocalSignonReply(command string) error
	LocalSignon() int
}

func (h *Host) RegisterCommands(subs *Subsystems) {
	cmdsys.AddCommand("quit", func(args []string) { h.CmdQuit() }, "Exit game")
	cmdsys.AddCommand("map", func(args []string) {
		if len(args) > 0 {
			h.CmdMap(args[0], subs)
		}
	}, "Start a new map")
	cmdsys.AddCommand("skill", func(args []string) {
		if len(args) > 0 {
			var skill int
			fmt.Sscanf(args[0], "%d", &skill)
			h.CmdSkill(skill)
		}
	}, "Set game skill level (0-3)")
	cmdsys.AddClientCommand("pause", func(args []string) { h.CmdPause(subs) }, "Pause game")
	cmdsys.AddClientCommand("status", func(args []string) { h.CmdStatus(subs) }, "Show server status")
	cmdsys.AddCommand("mapname", func(args []string) { h.CmdMapname(subs) }, "Show current map name")
	cmdsys.AddClientCommand("god", func(args []string) { h.CmdGod(subs) }, "Toggle god mode")
	cmdsys.AddClientCommand("noclip", func(args []string) { h.CmdNoClip(subs) }, "Toggle noclip mode")
	cmdsys.AddClientCommand("fly", func(args []string) { h.CmdFly(subs) }, "Toggle fly mode")
	cmdsys.AddClientCommand("notarget", func(args []string) { h.CmdNotarget(subs) }, "Toggle notarget mode")
	cmdsys.AddClientCommand("say", func(args []string) {
		if len(args) > 0 {
			h.CmdSay(strings.Join(args, " "), subs)
		}
	}, "Send a message to all players")
	cmdsys.AddClientCommand("say_team", func(args []string) {
		if len(args) > 0 {
			h.CmdSayTeam(strings.Join(args, " "), subs)
		}
	}, "Send a message to your team")
	cmdsys.AddClientCommand("tell", func(args []string) {
		if len(args) > 1 {
			h.CmdTell(args, subs)
		}
	}, "Send a message to a specific player")
	cmdsys.AddCommand("serverinfo", func(args []string) { h.CmdServerInfo(subs) }, "Show server information")
	cmdsys.AddCommand("restart", func(args []string) { h.CmdRestart(subs) }, "Restart current map")
	cmdsys.AddCommand("changelevel", func(args []string) {
		if len(args) > 0 {
			h.CmdChangelevel(args[0], subs)
		}
	}, "Change to a new level")
	cmdsys.AddCommand("connect", func(args []string) {
		if len(args) > 0 {
			h.CmdConnect(args[0], subs)
		}
	}, "Connect to a server")
	cmdsys.AddCommand("disconnect", func(args []string) { h.CmdDisconnect(subs) }, "Disconnect from current server")
	cmdsys.AddCommand("reconnect", func(args []string) { h.CmdReconnect(subs) }, "Reconnect to current server")
	cmdsys.AddCommand("slist", func(args []string) { h.CmdSlist(subs) }, "Search for LAN servers")
	cmdsys.AddClientCommand("name", func(args []string) {
		if len(args) > 0 {
			h.CmdName(args[0], subs)
		}
	}, "Set player name")
	cmdsys.AddClientCommand("color", func(args []string) {
		if len(args) > 0 {
			h.CmdColor(args, subs)
		}
	}, "Set player color")
	cmdsys.AddClientCommand("kill", func(args []string) { h.CmdKill(subs) }, "Suicide")
	cmdsys.AddClientCommand("spawn", func(args []string) { h.CmdSpawn(subs) }, "Spawn into game")
	cmdsys.AddClientCommand("begin", func(args []string) { h.CmdBegin(subs) }, "Begin game")
	cmdsys.AddClientCommand("prespawn", func(args []string) { h.CmdPreSpawn(subs) }, "Pre-spawn handshake")
	cmdsys.AddClientCommand("kick", func(args []string) {
		h.CmdKick(args, subs)
	}, "Kick a player from the server")
	cmdsys.AddCommand("ban", func(args []string) {
		h.CmdBan(args, subs)
	}, "Ban a player from the server")
	cmdsys.AddCommand("tracepos", func(args []string) { h.CmdTracepos(subs) }, "Trace from view origin to find surface/edict info")
	cmdsys.AddCommand("play", func(args []string) {
		if len(args) > 0 {
			h.CmdPlay(args, subs)
		}
	}, "Play one or more local sounds")
	cmdsys.AddCommand("playvol", func(args []string) {
		if len(args) > 1 {
			h.CmdPlayVol(args, subs)
		}
	}, "Play one or more local sounds with explicit volumes")
	cmdsys.AddCommand("stopsound", func(args []string) { h.CmdStopsound(subs) }, "Stop all active sounds")
	cmdsys.AddCommand("soundlist", func(args []string) { h.CmdSoundlist(subs) }, "List precached sounds")
	cmdsys.AddCommand("soundinfo", func(args []string) { h.CmdSoundinfo(subs) }, "Show audio system statistics")
	cmdsys.AddCommand("music", func(args []string) { h.CmdMusic(args, subs) }, "Play or inspect background music")
	cmdsys.AddCommand("music_pause", func(args []string) { h.CmdMusicPause(subs) }, "Pause background music")
	cmdsys.AddCommand("music_resume", func(args []string) { h.CmdMusicResume(subs) }, "Resume background music")
	cmdsys.AddCommand("music_loop", func(args []string) { h.CmdMusicLoop(args, subs) }, "Toggle or set background music looping")
	cmdsys.AddCommand("music_stop", func(args []string) { h.CmdMusicStop(subs) }, "Stop background music")
	cmdsys.AddCommand("music_jump", func(args []string) { h.CmdMusicJump(args, subs) }, "Jump to a module order in the active music track")
	cmdsys.AddCommand("particle_texture", func(args []string) {
		if len(args) > 0 {
			h.CmdParticleTexture(args[0], subs)
		}
	}, "Change particle rendering style (1=soft, 2=pixel)")
	cmdsys.AddClientCommand("ping", func(args []string) { h.CmdPing(subs) }, "Show player pings")
	cmdsys.AddCommand("load", func(args []string) {
		if len(args) > 0 {
			h.CmdLoad(args[0], subs)
		}
	}, "Load a saved game")
	cmdsys.AddCommand("save", func(args []string) {
		if len(args) > 0 {
			h.CmdSave(args[0], subs)
		}
	}, "Save current game")
	cmdsys.AddClientCommand("give", func(args []string) {
		if len(args) > 1 {
			h.CmdGive(args[0], args[1], subs)
		}
	}, "Give items/ammo")
	cmdsys.AddCommand("maps", func(args []string) { h.CmdMaps(subs) }, "List all maps")
	cmdsys.AddCommand("randmap", func(args []string) { h.CmdRandmap(subs) }, "Change to a random map")
	cmdsys.AddCommand("viewframe", func(args []string) {
		if len(args) > 0 {
			frame, err := strconv.Atoi(args[0])
			if err != nil {
				if subs != nil && subs.Console != nil {
					subs.Console.Print("usage: viewframe <frame>\n")
				}
				return
			}
			h.CmdViewframe(frame, subs)
		}
	}, "Set viewthing animation frame")
	cmdsys.AddCommand("viewnext", func(args []string) { h.CmdViewnext(subs) }, "Advance viewthing to next frame")
	cmdsys.AddCommand("viewprev", func(args []string) { h.CmdViewprev(subs) }, "Rewind viewthing to previous frame")
	cmdsys.AddCommand("viewpos", func(args []string) { h.CmdViewpos(subs) }, "Show current view position")
	cmdsys.AddCommand("setpos", func(args []string) { h.CmdSetPos(args, subs) }, "Teleport to position")
	cmdsys.AddCommand("pr_ents", func(args []string) { h.CmdPrEnts(subs) }, "Print all active entities")

	// Demo commands
	cmdsys.AddCommand("record", func(args []string) {
		if len(args) > 0 {
			h.CmdRecord(args[0], subs)
		}
	}, "Start recording a demo")
	cmdsys.AddCommand("stop", func(args []string) {
		h.CmdStop(subs)
	}, "Stop recording a demo")
	cmdsys.AddCommand("playdemo", func(args []string) {
		if len(args) > 0 {
			h.CmdPlaydemo(args[0], subs)
		}
	}, "Play a demo")
	cmdsys.AddCommand("timedemo", func(args []string) {
		if len(args) > 0 {
			h.CmdTimedemo(args[0], subs)
			return
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: timedemo <demoname>\n")
		}
	}, "Benchmark demo playback speed")
	cmdsys.AddCommand("demoseek", func(args []string) {
		if len(args) > 0 {
			target, err := strconv.Atoi(args[0])
			if err != nil {
				if subs != nil && subs.Console != nil {
					subs.Console.Print("usage: demoseek <frame>\n")
				}
				return
			}
			h.CmdDemoSeek(target, subs)
			return
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: demoseek <frame>\n")
		}
	}, "Seek to an absolute demo frame")
	cmdsys.AddCommand("rewind", func(args []string) {
		frames := 1
		if len(args) > 0 {
			value, err := strconv.Atoi(args[0])
			if err != nil || value <= 0 {
				if subs != nil && subs.Console != nil {
					subs.Console.Print("usage: rewind [frames]\n")
				}
				return
			}
			frames = value
		}
		h.CmdRewind(frames, subs)
	}, "Rewind demo playback by frame count")
	cmdsys.AddCommand("demogoto", func(args []string) {
		if len(args) > 0 {
			seconds, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				if subs != nil && subs.Console != nil {
					subs.Console.Print("usage: demogoto <seconds>\n")
				}
				return
			}
			h.CmdDemoGoto(seconds, subs)
			return
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: demogoto <seconds>\n")
		}
	}, "Seek demo playback to a time in seconds")
	cmdsys.AddCommand("demopause", func(args []string) {
		h.CmdDemoPause(subs)
	}, "Toggle demo playback pause")
	cmdsys.AddCommand("demospeed", func(args []string) {
		if len(args) > 0 {
			speed, err := strconv.ParseFloat(args[0], 32)
			if err != nil || speed <= 0 {
				if subs != nil && subs.Console != nil {
					subs.Console.Print("usage: demospeed <multiplier> (positive number)\n")
				}
				return
			}
			h.CmdDemoSpeed(float32(speed), subs)
			return
		}
		if subs != nil && subs.Console != nil {
			if h.demoState != nil && h.demoState.Playback {
				subs.Console.Print(fmt.Sprintf("Demo speed: %.2f\n", h.demoState.Speed))
			} else {
				subs.Console.Print("Not playing back a demo.\n")
			}
		}
	}, "Set demo playback speed multiplier")
	cmdsys.AddCommand("stopdemo", func(args []string) {
		h.CmdStopdemo(subs)
	}, "Stop demo playback")
	cmdsys.AddCommand("startdemos", func(args []string) {
		h.CmdStartdemos(args, subs)
	}, "Set a list of demos to cycle through")
	cmdsys.AddCommand("demos", func(args []string) {
		h.CmdDemos(subs)
	}, "Restart the demo loop")

	// Menu commands
	cmdsys.AddCommand("togglemenu", func(args []string) {
		h.CmdToggleMenu()
	}, "Toggle the main menu")
	cmdsys.AddCommand("menu_main", func(args []string) {
		h.CmdMenuMain()
	}, "Show the main menu")
	cmdsys.AddCommand("menu_quit", func(args []string) {
		h.CmdMenuQuit()
	}, "Show the quit confirmation")
	cmdsys.AddCommand("exec", func(args []string) {
		h.CmdExec(args, subs)
	}, "Execute a script file")
	cmdsys.AddCommand("stuffcmds", func(args []string) {
		h.CmdStuffCmds(subs)
	}, "Insert command-line +commands into the buffer")
	cmdsys.AddCommand("echo", func(args []string) {
		h.CmdEcho(args, subs)
	}, "Print text to the console")
	cmdsys.AddCommand("version", func(args []string) {
		h.CmdVersion(subs)
	}, "Print engine version")
	cmdsys.AddCommand("clear", func(args []string) {
		h.CmdClear(subs)
	}, "Clear the console buffer")
	cmdsys.AddCommand("condump", func(args []string) {
		h.CmdCondump(args, subs)
	}, "Dump the console text to a file")
	cmdsys.AddCommand("alias", func(args []string) {
		h.CmdAlias(args, subs)
	}, "Create, list, and inspect command aliases")
	cmdsys.AddCommand("unalias", func(args []string) {
		h.CmdUnalias(args, subs)
	}, "Delete a command alias")
	cmdsys.AddCommand("unaliasall", func(args []string) {
		h.CmdUnaliasAll()
	}, "Delete all command aliases")
	cmdsys.AddCommand("writeconfig", func(args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if err := h.WriteConfigNamed(name, subs); err != nil && subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("writeconfig failed: %v\n", err))
		}
	}, "Write ironwail.cfg or a named config file")
}

func (h *Host) startLocalServerSession(subs *Subsystems, afterConnect func() error) (err error) {
	if subs == nil || subs.Server == nil {
		return fmt.Errorf("server not initialized")
	}

	previousServerActive := h.serverActive
	previousClientState := h.clientState
	previousSignOns := h.signOns
	teardownOnFailure := !previousServerActive

	defer func() {
		h.updateServerBrowserNetworking(subs)
		if err == nil {
			return
		}
		if teardownOnFailure {
			subs.Server.Shutdown()
			if subs.Client != nil {
				subs.Client.Shutdown()
			}
			if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
				loopbackClient.ClearState()
				loopbackClient.State = cl.StateDisconnected
			}
			h.serverActive = false
			h.clientState = caDisconnected
			h.signOns = 0
			return
		}
		h.serverActive = previousServerActive
		h.clientState = previousClientState
		h.signOns = previousSignOns
	}()

	handshake, ok := subs.Client.(handshakeClient)
	if !ok {
		if subs.Client != nil {
			subs.Client.Shutdown()
		}
		localClient := newLocalLoopbackClient()
		if serverSource, sourceOK := subs.Server.(serverDatagramSource); sourceOK {
			localClient.srv = serverSource
			if cmdSource, cmdOK := subs.Server.(serverCommandSink); cmdOK {
				localClient.cmd = cmdSource
			}
		}
		if err := localClient.Init(); err != nil {
			return fmt.Errorf("failed to initialize local client: %w", err)
		}
		subs.Client = localClient
		handshake = localClient
		ok = true
	}
	if !ok {
		return fmt.Errorf("client handshake implementation missing")
	}

	h.serverActive = true
	h.updateServerBrowserNetworking(subs)
	subs.Server.ConnectClient(0)
	if afterConnect != nil {
		if err := afterConnect(); err != nil {
			return err
		}
	}

	h.clientState = caConnected
	h.signOns = 0

	if err := handshake.LocalServerInfo(); err != nil {
		return fmt.Errorf("local serverinfo handshake failed: %w", err)
	}
	h.clientState = handshake.State()

	if err := h.runLocalHandshakeStep("prespawn", subs); err != nil {
		return err
	}
	if err := h.runLocalHandshakeStep("spawn", subs); err != nil {
		return err
	}
	if err := h.runLocalHandshakeStep("begin", subs); err != nil {
		return err
	}

	return nil
}
