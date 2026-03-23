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

func replaceCommand(name string, fn cmdsys.CommandFunc, desc string) {
	cmdsys.RemoveCommand(name)
	cmdsys.AddCommand(name, fn, desc)
}

func replaceClientCommand(name string, fn cmdsys.CommandFunc, desc string) {
	cmdsys.RemoveCommand(name)
	cmdsys.AddClientCommand(name, fn, desc)
}

func (h *Host) RegisterCommands(subs *Subsystems) {
	replaceCommand("quit", func(args []string) { h.CmdQuit() }, "Exit game")
	replaceCommand("map", func(args []string) {
		if len(args) > 0 {
			h.CmdMapWithSpawnArgs(args[0], args[1:], subs)
		}
	}, "Start a new map")
	replaceCommand("skill", func(args []string) {
		if len(args) > 0 {
			var skill int
			fmt.Sscanf(args[0], "%d", &skill)
			h.CmdSkill(skill)
		}
	}, "Set game skill level (0-3)")
	replaceClientCommand("pause", func(args []string) { h.CmdPause(subs) }, "Pause game")
	replaceClientCommand("status", func(args []string) { h.CmdStatus(subs) }, "Show server status")
	replaceCommand("mapname", func(args []string) { h.CmdMapname(subs) }, "Show current map name")
	replaceCommand("mods", func(args []string) { h.CmdMods(args, subs) }, "List available mod directories")
	replaceCommand("games", func(args []string) { h.CmdMods(args, subs) }, "Alias for mods")
	replaceCommand("skies", func(args []string) { h.CmdSkies(args, subs) }, "List available skyboxes")
	replaceClientCommand("god", func(args []string) { h.CmdGod(subs) }, "Toggle god mode")
	replaceClientCommand("noclip", func(args []string) { h.CmdNoClip(subs) }, "Toggle noclip mode")
	replaceClientCommand("fly", func(args []string) { h.CmdFly(subs) }, "Toggle fly mode")
	replaceClientCommand("notarget", func(args []string) { h.CmdNotarget(subs) }, "Toggle notarget mode")
	replaceClientCommand("say", func(args []string) {
		if len(args) > 0 {
			h.CmdSay(strings.Join(args, " "), subs)
		}
	}, "Send a message to all players")
	replaceClientCommand("say_team", func(args []string) {
		if len(args) > 0 {
			h.CmdSayTeam(strings.Join(args, " "), subs)
		}
	}, "Send a message to your team")
	replaceClientCommand("tell", func(args []string) {
		if len(args) > 1 {
			h.CmdTell(args, subs)
		}
	}, "Send a message to a specific player")
	replaceCommand("serverinfo", func(args []string) { h.CmdServerInfo(subs) }, "Show server information")
	replaceCommand("restart", func(args []string) { h.CmdRestart(subs) }, "Restart current map")
	replaceCommand("changelevel", func(args []string) {
		if len(args) > 0 {
			h.CmdChangelevel(args[0], subs)
		}
	}, "Change to a new level")
	replaceCommand("connect", func(args []string) {
		if len(args) > 0 {
			h.CmdConnect(args[0], subs)
		}
	}, "Connect to a server")
	replaceCommand("disconnect", func(args []string) { h.CmdDisconnect(subs) }, "Disconnect from current server")
	replaceCommand("cmd", func(args []string) { h.CmdForwardToServer(args, subs) }, "Forward command line to current server")
	replaceCommand("reconnect", func(args []string) { h.CmdReconnect(subs) }, "Reconnect to current server")
	replaceCommand("slist", func(args []string) { h.CmdSlist(subs) }, "Search for LAN servers")
	replaceCommand("test2", func(args []string) {
		if len(args) > 0 {
			h.CmdTest2(args[0], subs)
		}
	}, "Query a server's rule list")
	replaceCommand("listen", func(args []string) { h.CmdListen(args, subs) }, "Enable/disable network listening")
	replaceCommand("maxplayers", func(args []string) { h.CmdMaxPlayers(args, subs) }, "Show or set maximum player slots")
	replaceCommand("port", func(args []string) { h.CmdPort(args, subs) }, "Show or set network host port")
	replaceClientCommand("name", func(args []string) {
		if len(args) > 0 {
			h.CmdName(args[0], subs)
		}
	}, "Set player name")
	replaceClientCommand("color", func(args []string) {
		if len(args) > 0 {
			h.CmdColor(args, subs)
		}
	}, "Set player color")
	replaceClientCommand("kill", func(args []string) { h.CmdKill(subs) }, "Suicide")
	replaceClientCommand("spawn", func(args []string) { h.CmdSpawn(subs) }, "Spawn into game")
	replaceClientCommand("begin", func(args []string) { h.CmdBegin(subs) }, "Begin game")
	replaceClientCommand("prespawn", func(args []string) { h.CmdPreSpawn(subs) }, "Pre-spawn handshake")
	replaceClientCommand("kick", func(args []string) {
		h.CmdKick(args, subs)
	}, "Kick a player from the server")
	replaceCommand("ban", func(args []string) {
		h.CmdBan(args, subs)
	}, "Ban a player from the server")
	replaceCommand("tracepos", func(args []string) { h.CmdTracepos(subs) }, "Trace from view origin to find surface/edict info")
	replaceCommand("play", func(args []string) {
		if len(args) > 0 {
			h.CmdPlay(args, subs)
		}
	}, "Play one or more local sounds")
	replaceCommand("playvol", func(args []string) {
		if len(args) > 1 {
			h.CmdPlayVol(args, subs)
		}
	}, "Play one or more local sounds with explicit volumes")
	replaceCommand("stopsound", func(args []string) { h.CmdStopsound(subs) }, "Stop all active sounds")
	replaceCommand("soundlist", func(args []string) { h.CmdSoundlist(subs) }, "List precached sounds")
	replaceCommand("soundinfo", func(args []string) { h.CmdSoundinfo(subs) }, "Show audio system statistics")
	replaceCommand("music", func(args []string) { h.CmdMusic(args, subs) }, "Play or inspect background music")
	replaceCommand("music_pause", func(args []string) { h.CmdMusicPause(subs) }, "Pause background music")
	replaceCommand("music_resume", func(args []string) { h.CmdMusicResume(subs) }, "Resume background music")
	replaceCommand("music_loop", func(args []string) { h.CmdMusicLoop(args, subs) }, "Toggle or set background music looping")
	replaceCommand("music_stop", func(args []string) { h.CmdMusicStop(subs) }, "Stop background music")
	replaceCommand("music_jump", func(args []string) { h.CmdMusicJump(args, subs) }, "Jump to a module order in the active music track")
	replaceCommand("net_stats", func(args []string) { h.CmdNetStats(subs) }, "Show datagram network counters")
	replaceCommand("particle_texture", func(args []string) {
		if len(args) > 0 {
			h.CmdParticleTexture(args[0], subs)
		}
	}, "Change particle rendering style (1=soft, 2=pixel)")
	replaceCommand("fog", func(args []string) { h.CmdFog(args, subs) }, "Inspect or set client fog parameters")
	replaceClientCommand("ping", func(args []string) { h.CmdPing(subs) }, "Show player pings")
	replaceCommand("load", func(args []string) {
		h.CmdLoadArgs(args, subs)
	}, "Load a saved game")
	replaceCommand("save", func(args []string) {
		h.CmdSaveArgs(args, subs)
	}, "Save current game")
	replaceClientCommand("give", func(args []string) {
		if len(args) > 1 {
			h.CmdGive(args[0], args[1], subs)
		}
	}, "Give items/ammo")
	replaceCommand("maps", func(args []string) { h.CmdMaps(subs) }, "List all maps")
	replaceCommand("randmap", func(args []string) { h.CmdRandmap(subs) }, "Change to a random map")
	replaceCommand("viewframe", func(args []string) {
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
	replaceCommand("viewnext", func(args []string) { h.CmdViewnext(subs) }, "Advance viewthing to next frame")
	replaceCommand("viewprev", func(args []string) { h.CmdViewprev(subs) }, "Rewind viewthing to previous frame")
	replaceCommand("viewpos", func(args []string) { h.CmdViewpos(subs) }, "Show current view position")
	replaceCommand("setpos", func(args []string) { h.CmdSetPos(args, subs) }, "Teleport to position")
	replaceCommand("pr_ents", func(args []string) { h.CmdPrEnts(subs) }, "Print all active entities")
	replaceCommand("edictcount", func(args []string) { h.CmdEdictCount(subs) }, "Print edict summary counts")
	replaceCommand("profile", func(args []string) { h.CmdProfile(subs) }, "Show top QC function profile counters")

	// Demo commands
	replaceCommand("record", func(args []string) {
		if len(args) > 0 {
			h.CmdRecord(args[0], subs)
		}
	}, "Start recording a demo")
	replaceCommand("stop", func(args []string) {
		h.CmdStop(subs)
	}, "Stop recording a demo")
	replaceCommand("playdemo", func(args []string) {
		if len(args) > 0 {
			h.CmdPlaydemo(args[0], subs)
		}
	}, "Play a demo")
	replaceCommand("timedemo", func(args []string) {
		if len(args) > 0 {
			h.CmdTimedemo(args[0], subs)
			return
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: timedemo <demoname>\n")
		}
	}, "Benchmark demo playback speed")
	replaceCommand("demoseek", func(args []string) {
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
	replaceCommand("rewind", func(args []string) {
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
	replaceCommand("demogoto", func(args []string) {
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
	replaceCommand("demopause", func(args []string) {
		h.CmdDemoPause(subs)
	}, "Toggle demo playback pause")
	replaceCommand("demospeed", func(args []string) {
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
	replaceCommand("stopdemo", func(args []string) {
		h.CmdStopdemo(subs)
	}, "Stop demo playback")
	replaceCommand("startdemos", func(args []string) {
		h.CmdStartdemos(args, subs)
	}, "Set a list of demos to cycle through")
	replaceCommand("demos", func(args []string) {
		h.CmdDemos(subs)
	}, "Restart the demo loop")

	// Menu commands
	replaceCommand("togglemenu", func(args []string) {
		h.CmdToggleMenu()
	}, "Toggle the main menu")
	replaceCommand("menu_main", func(args []string) {
		h.CmdMenuMain()
	}, "Show the main menu")
	replaceCommand("menu_quit", func(args []string) {
		h.CmdMenuQuit()
	}, "Show the quit confirmation")
	replaceCommand("exec", func(args []string) {
		h.CmdExec(args, subs)
	}, "Execute a script file")
	replaceCommand("stuffcmds", func(args []string) {
		h.CmdStuffCmds(subs)
	}, "Insert command-line +commands into the buffer")
	replaceCommand("path", func(args []string) {
		h.CmdPath(subs)
	}, "Print the current filesystem search path")
	replaceCommand("echo", func(args []string) {
		h.CmdEcho(args, subs)
	}, "Print text to the console")
	replaceCommand("version", func(args []string) {
		h.CmdVersion(subs)
	}, "Print engine version")
	replaceCommand("clear", func(args []string) {
		h.CmdClear(subs)
	}, "Clear the console buffer")
	replaceCommand("condump", func(args []string) {
		h.CmdCondump(args, subs)
	}, "Dump the console text to a file")
	replaceCommand("alias", func(args []string) {
		h.CmdAlias(args, subs)
	}, "Create, list, and inspect command aliases")
	replaceCommand("unalias", func(args []string) {
		h.CmdUnalias(args, subs)
	}, "Delete a command alias")
	replaceCommand("unaliasall", func(args []string) {
		h.CmdUnaliasAll()
	}, "Delete all command aliases")
	replaceCommand("writeconfig", func(args []string) {
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

	h.resetAutosaveState()
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
