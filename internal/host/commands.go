// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/menu"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

var saveNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`)

// defaultRemoteClientFactory is the production factory used by NewHost; tests
// override the Host's RemoteClientFactory field to inject fakes.
func defaultRemoteClientFactory(network *inet.Network, address string) (Client, error) {
	if network == nil {
		network = inet.DefaultNetwork()
	}
	socket := network.Connect(address)
	if socket == nil {
		return nil, fmt.Errorf("unable to connect to %s", address)
	}
	client := newRemoteDatagramClient(network, socket)
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

func (h *Host) replaceCommand(name string, fn cmdsys.CommandFunc, desc string) {
	h.Cmd.RemoveCommand(name)
	h.Cmd.AddCommand(name, fn, desc)
}

func (h *Host) replaceClientCommand(name string, fn cmdsys.CommandFunc, desc string) {
	h.Cmd.RemoveCommand(name)
	h.Cmd.AddClientCommand(name, fn, desc)
}

func (h *Host) RegisterCommands(subs *Subsystems) {
	h.replaceCommand("quit", func(args []string) { h.CmdQuit() }, "Exit game")
	h.replaceCommand("map", func(args []string) {
		if len(args) > 0 {
			h.CmdMapWithSpawnArgs(args[0], args[1:], subs)
		}
	}, "Start a new map")
	h.replaceCommand("skill", func(args []string) {
		if len(args) > 0 {
			var skill int
			fmt.Sscanf(args[0], "%d", &skill)
			h.CmdSkill(skill)
		}
	}, "Set game skill level (0-3)")
	h.replaceClientCommand("pause", func(args []string) { h.CmdPause(subs) }, "Pause game")
	h.replaceClientCommand("status", func(args []string) { h.CmdStatus(subs) }, "Show server status")
	h.replaceCommand("mapname", func(args []string) { h.CmdMapname(subs) }, "Show current map name")
	h.replaceCommand("mods", func(args []string) { h.CmdMods(args, subs) }, "List available mod directories")
	h.replaceCommand("games", func(args []string) { h.CmdMods(args, subs) }, "Alias for mods")
	h.replaceCommand("game", func(args []string) { h.CmdGame(args, subs) }, "Switch active game directory")
	h.replaceCommand("skies", func(args []string) { h.CmdSkies(args, subs) }, "List available skyboxes")
	h.replaceClientCommand("god", func(args []string) { h.CmdGod(subs) }, "Toggle god mode")
	h.replaceClientCommand("noclip", func(args []string) { h.CmdNoClip(subs) }, "Toggle noclip mode")
	h.replaceClientCommand("fly", func(args []string) { h.CmdFly(subs) }, "Toggle fly mode")
	h.replaceClientCommand("notarget", func(args []string) { h.CmdNotarget(subs) }, "Toggle notarget mode")
	h.replaceClientCommand("say", func(args []string) {
		if len(args) > 0 {
			h.CmdSay(strings.Join(args, " "), subs)
		}
	}, "Send a message to all players")
	h.replaceClientCommand("say_team", func(args []string) {
		if len(args) > 0 {
			h.CmdSayTeam(strings.Join(args, " "), subs)
		}
	}, "Send a message to your team")
	h.replaceClientCommand("tell", func(args []string) {
		if len(args) > 1 {
			h.CmdTell(args, subs)
		}
	}, "Send a message to a specific player")
	h.replaceCommand("serverinfo", func(args []string) { h.CmdServerInfo(subs) }, "Show server information")
	h.replaceCommand("restart", func(args []string) { h.CmdRestart(subs) }, "Restart current map")
	h.replaceCommand("changelevel", func(args []string) {
		if len(args) > 0 {
			h.CmdChangelevel(args[0], subs)
		}
	}, "Change to a new level")
	h.replaceCommand("connect", func(args []string) {
		if len(args) > 0 {
			h.CmdConnect(args[0], subs)
		}
	}, "Connect to a server")
	h.replaceCommand("disconnect", func(args []string) { h.CmdDisconnect(subs) }, "Disconnect from current server")
	h.replaceCommand("cmd", func(args []string) { h.CmdForwardToServer(args, subs) }, "Forward command line to current server")
	h.replaceCommand("rcon", func(args []string) { h.CmdRcon(args, subs) }, "Forward a remote console command to current server")
	h.replaceCommand("reconnect", func(args []string) { h.CmdReconnect(subs) }, "Reconnect to current server")
	h.replaceCommand("slist", func(args []string) { h.CmdSlist(subs) }, "List LAN Quake servers")
	h.replaceCommand("test2", func(args []string) {
		if len(args) > 0 {
			h.CmdTest2(args[0], subs)
		}
	}, "Query a server's rule list")
	h.replaceCommand("players", func(args []string) {
		if len(args) > 0 {
			h.CmdPlayers(args[0], subs)
		}
	}, "Query a server's player list")
	h.replaceCommand("listen", func(args []string) { h.CmdListen(args, subs) }, "Enable/disable network listening")
	h.replaceCommand("maxplayers", func(args []string) { h.CmdMaxPlayers(args, subs) }, "Show or set maximum player slots")
	h.replaceCommand("port", func(args []string) { h.CmdPort(args, subs) }, "Show or set network host port")
	h.replaceClientCommand("name", func(args []string) {
		if len(args) == 0 {
			// C prints current name on no-arg query.
			subs.Console.Print(fmt.Sprintf("\"name\" is \"%s\"\n", h.CVar.StringValue(clientNameCVar)))
			return
		}
		// C uses full Cmd_Args for multi-word names.
		h.CmdName(strings.Join(args, " "), subs)
	}, "Set player name")
	h.replaceClientCommand("color", func(args []string) {
		if len(args) == 0 {
			// C prints current color on no-arg query.
			color := h.CVar.IntValue(clientColorCVar)
			subs.Console.Print(fmt.Sprintf("\"color\" is \"%d %d\"\n", color>>4, color&15))
			return
		}
		h.CmdColor(args, subs)
	}, "Set player color")
	h.replaceClientCommand("kill", func(args []string) { h.CmdKill(subs) }, "Suicide")
	h.replaceClientCommand("spawn", func(args []string) { h.CmdSpawn(subs) }, "Spawn into game")
	h.replaceClientCommand("begin", func(args []string) { h.CmdBegin(subs) }, "Begin game")
	h.replaceClientCommand("prespawn", func(args []string) { h.CmdPreSpawn(subs) }, "Pre-spawn handshake")
	h.replaceClientCommand("kick", func(args []string) {
		h.CmdKick(args, subs)
	}, "Kick a player from the server")
	h.replaceCommand("ban", func(args []string) {
		h.CmdBan(args, subs)
	}, "Ban a player from the server")
	h.replaceCommand("tracepos", func(args []string) { h.CmdTracepos(subs) }, "Trace from view origin to find surface/edict info")
	h.replaceCommand("play", func(args []string) {
		if len(args) > 0 {
			h.CmdPlay(args, subs)
		}
	}, "Play one or more local sounds")
	h.replaceCommand("playvol", func(args []string) {
		if len(args) > 1 {
			h.CmdPlayVol(args, subs)
		}
	}, "Play one or more local sounds with explicit volumes")
	h.replaceCommand("stopsound", func(args []string) { h.CmdStopsound(subs) }, "Stop all active sounds")
	h.replaceCommand("soundlist", func(args []string) { h.CmdSoundlist(subs) }, "List precached sounds")
	h.replaceCommand("soundinfo", func(args []string) { h.CmdSoundinfo(subs) }, "Show audio system statistics")
	h.replaceCommand("music", func(args []string) { h.CmdMusic(args, subs) }, "Play or inspect background music")
	h.replaceCommand("music_pause", func(args []string) { h.CmdMusicPause(subs) }, "Pause background music")
	h.replaceCommand("music_resume", func(args []string) { h.CmdMusicResume(subs) }, "Resume background music")
	h.replaceCommand("music_loop", func(args []string) { h.CmdMusicLoop(args, subs) }, "Toggle or set background music looping")
	h.replaceCommand("music_stop", func(args []string) { h.CmdMusicStop(subs) }, "Stop background music")
	h.replaceCommand("music_jump", func(args []string) { h.CmdMusicJump(args, subs) }, "Jump to a module order in the active music track")
	h.replaceCommand("net_stats", func(args []string) { h.CmdNetStats(subs) }, "Show datagram network counters")
	h.replaceCommand("particle_texture", func(args []string) {
		if len(args) > 0 {
			h.CmdParticleTexture(args[0], subs)
		}
	}, "Change particle rendering style (1=soft, 2=pixel)")
	h.replaceCommand("fog", func(args []string) { h.CmdFog(args, subs) }, "Inspect or set client fog parameters")
	h.replaceClientCommand("ping", func(args []string) { h.CmdPing(subs) }, "Show player pings")
	h.replaceCommand("load", func(args []string) {
		h.CmdLoadArgs(args, subs)
	}, "Load a saved game")
	h.replaceCommand("save", func(args []string) {
		h.CmdSaveArgs(args, subs)
	}, "Save current game")
	h.replaceClientCommand("give", func(args []string) {
		if len(args) > 1 {
			h.CmdGive(args[0], args[1], subs)
		}
	}, "Give items/ammo")
	h.replaceCommand("maps", func(args []string) { h.CmdMaps(subs) }, "List all maps")
	h.replaceCommand("randmap", func(args []string) { h.CmdRandmap(subs) }, "Change to a random map")
	h.replaceCommand("viewframe", func(args []string) {
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
	h.replaceCommand("viewnext", func(args []string) { h.CmdViewnext(subs) }, "Advance viewthing to next frame")
	h.replaceCommand("viewprev", func(args []string) { h.CmdViewprev(subs) }, "Rewind viewthing to previous frame")
	h.replaceCommand("viewpos", func(args []string) { h.CmdViewpos(subs) }, "Show current view position")
	h.replaceCommand("setpos", func(args []string) { h.CmdSetPos(args, subs) }, "Teleport to position")
	h.replaceCommand("pr_ents", func(args []string) { h.CmdPrEnts(subs) }, "Print all active entities")
	h.replaceCommand("edictcount", func(args []string) { h.CmdEdictCount(subs) }, "Print edict summary counts")
	h.replaceCommand("devstats", func(args []string) { h.CmdDevStats(subs) }, "Print server development statistics (current and peak)")
	h.replaceCommand("profile", func(args []string) { h.CmdProfile(subs) }, "Show top QC function profile counters")

	// Demo commands
	h.replaceCommand("record", func(args []string) {
		if len(args) > 0 {
			h.CmdRecord(args, subs)
		}
	}, "Start recording a demo")
	h.replaceCommand("stop", func(args []string) {
		h.CmdStop(subs)
	}, "Stop recording a demo")
	h.replaceCommand("playdemo", func(args []string) {
		if len(args) > 0 {
			h.CmdPlaydemo(args[0], subs)
		}
	}, "Play a demo")
	h.replaceCommand("timedemo", func(args []string) {
		if len(args) > 0 {
			h.CmdTimedemo(args[0], subs)
			return
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: timedemo <demoname>\n")
		}
	}, "Benchmark demo playback speed")
	h.replaceCommand("demoseek", func(args []string) {
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
	h.replaceCommand("rewind", func(args []string) {
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
	h.replaceCommand("demogoto", func(args []string) {
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
	h.replaceCommand("demopause", func(args []string) {
		h.CmdDemoPause(subs)
	}, "Toggle demo playback pause")
	h.replaceCommand("demospeed", func(args []string) {
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
	h.replaceCommand("stopdemo", func(args []string) {
		h.CmdStopdemo(subs)
	}, "Stop demo playback")
	h.replaceCommand("startdemos", func(args []string) {
		h.CmdStartdemos(args, subs)
	}, "Set a list of demos to cycle through")
	h.replaceCommand("demos", func(args []string) {
		h.CmdDemos(subs)
	}, "Restart the demo loop")

	// Menu commands
	h.replaceCommand("togglemenu", func(args []string) {
		h.CmdToggleMenu()
	}, "Toggle the main menu")
	h.replaceCommand("menu_main", func(args []string) {
		h.CmdMenuMain()
	}, "Show the main menu")
	h.replaceCommand("menu_singleplayer", func(args []string) {
		h.CmdMenuState(menu.MenuSinglePlayer)
	}, "Show the single-player menu")
	h.replaceCommand("menu_maps", func(args []string) {
		h.CmdMenuState(menu.MenuMods)
	}, "Show the mods browser")
	h.replaceCommand("menu_load", func(args []string) {
		h.CmdMenuState(menu.MenuLoad)
	}, "Show the load-game menu")
	h.replaceCommand("menu_save", func(args []string) {
		h.CmdMenuState(menu.MenuSave)
	}, "Show the save-game menu")
	h.replaceCommand("menu_multiplayer", func(args []string) {
		h.CmdMenuState(menu.MenuMultiPlayer)
	}, "Show the multiplayer menu")
	h.replaceCommand("menu_setup", func(args []string) {
		h.CmdMenuState(menu.MenuSetup)
	}, "Show the player setup menu")
	h.replaceCommand("menu_options", func(args []string) {
		h.CmdMenuState(menu.MenuOptions)
	}, "Show the options menu")
	h.replaceCommand("menu_keys", func(args []string) {
		h.CmdMenuState(menu.MenuControls)
	}, "Show the controls menu")
	h.replaceCommand("menu_video", func(args []string) {
		h.CmdMenuState(menu.MenuVideo)
	}, "Show the video menu")
	h.replaceCommand("menu_help", func(args []string) {
		h.CmdMenuState(menu.MenuHelp)
	}, "Show the help menu")
	h.replaceCommand("menu_quit", func(args []string) {
		h.CmdMenuQuit()
	}, "Show the quit confirmation")
	h.replaceCommand("exec", func(args []string) {
		h.CmdExec(args, subs)
	}, "Execute a script file")
	h.replaceCommand("stuffcmds", func(args []string) {
		h.CmdStuffCmds(subs)
	}, "Insert command-line +commands into the buffer")
	h.replaceCommand("path", func(args []string) {
		h.CmdPath(subs)
	}, "Print the current filesystem search path")
	h.replaceCommand("echo", func(args []string) {
		h.CmdEcho(args, subs)
	}, "Print text to the console")
	h.replaceCommand("version", func(args []string) {
		h.CmdVersion(subs)
	}, "Print engine version")
	h.replaceCommand("clear", func(args []string) {
		h.CmdClear(subs)
	}, "Clear the console buffer")
	h.replaceCommand("condump", func(args []string) {
		h.CmdCondump(args, subs)
	}, "Dump the console text to a file")
	h.replaceCommand("alias", func(args []string) {
		h.CmdAlias(args, subs)
	}, "Create, list, and inspect command aliases")
	h.replaceCommand("unalias", func(args []string) {
		h.CmdUnalias(args, subs)
	}, "Delete a command alias")
	h.replaceCommand("unaliasall", func(args []string) {
		h.CmdUnaliasAll()
	}, "Delete all command aliases")
	h.replaceCommand("writeconfig", func(args []string) {
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
