// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/client"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/server"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
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
	cmdsys.AddCommand("pause", func(args []string) { h.CmdPause() }, "Pause game")
	cmdsys.AddCommand("status", func(args []string) { h.CmdStatus(subs) }, "Show server status")
	cmdsys.AddCommand("mapname", func(args []string) { h.CmdMapname(subs) }, "Show current map name")
	cmdsys.AddCommand("god", func(args []string) { h.CmdGod(subs) }, "Toggle god mode")
	cmdsys.AddCommand("noclip", func(args []string) { h.CmdNoClip(subs) }, "Toggle noclip mode")
	cmdsys.AddCommand("fly", func(args []string) { h.CmdFly(subs) }, "Toggle fly mode")
	cmdsys.AddCommand("notarget", func(args []string) { h.CmdNotarget(subs) }, "Toggle notarget mode")
	cmdsys.AddCommand("say", func(args []string) {
		if len(args) > 0 {
			h.CmdSay(strings.Join(args, " "), subs)
		}
	}, "Send a message to all players")
	cmdsys.AddCommand("say_team", func(args []string) {
		if len(args) > 0 {
			h.CmdSayTeam(strings.Join(args, " "), subs)
		}
	}, "Send a message to your team")
	cmdsys.AddCommand("tell", func(args []string) {
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
	cmdsys.AddCommand("name", func(args []string) {
		if len(args) > 0 {
			h.CmdName(args[0], subs)
		}
	}, "Set player name")
	cmdsys.AddCommand("color", func(args []string) {
		if len(args) > 0 {
			h.CmdColor(args, subs)
		}
	}, "Set player color")
	cmdsys.AddCommand("kill", func(args []string) { h.CmdKill(subs) }, "Suicide")
	cmdsys.AddCommand("spawn", func(args []string) { h.CmdSpawn(subs) }, "Spawn into game")
	cmdsys.AddCommand("begin", func(args []string) { h.CmdBegin(subs) }, "Begin game")
	cmdsys.AddCommand("prespawn", func(args []string) { h.CmdPreSpawn(subs) }, "Pre-spawn handshake")
	cmdsys.AddCommand("kick", func(args []string) {
		h.CmdKick(args, subs)
	}, "Kick a player from the server")
	cmdsys.AddCommand("ban", func(args []string) {
		h.CmdBan(args, subs)
	}, "Ban a player from the server")
	cmdsys.AddCommand("tracepos", func(args []string) { h.CmdTracepos(subs) }, "Trace from view origin to find surface/edict info")
	cmdsys.AddCommand("soundinfo", func(args []string) { h.CmdSoundinfo(subs) }, "Show audio system statistics")
	cmdsys.AddCommand("particle_texture", func(args []string) {
		if len(args) > 0 {
			h.CmdParticleTexture(args[0], subs)
		}
	}, "Change particle rendering style (1=soft, 2=pixel)")
	cmdsys.AddCommand("ping", func(args []string) { h.CmdPing(subs) }, "Show player pings")
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
	cmdsys.AddCommand("give", func(args []string) {
	        if len(args) > 1 {
	                h.CmdGive(args[0], args[1], subs)
	        }
	}, "Give items/ammo")
	cmdsys.AddCommand("maps", func(args []string) { h.CmdMaps(subs) }, "List all maps")
	cmdsys.AddCommand("viewpos", func(args []string) { h.CmdViewpos(subs) }, "Show current view position")
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
		if len(args) > 0 {
			h.CmdExec(args[0], subs)
			return
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: exec <filename>\n")
		}
	}, "Execute a script file")
	cmdsys.AddCommand("echo", func(args []string) {
		h.CmdEcho(args, subs)
	}, "Print text to the console")
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
	cmdsys.AddCommand("saveconfig", func(args []string) {
		if err := h.WriteConfig(subs); err != nil && subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("saveconfig failed: %v\n", err))
		}
	}, "Write config.cfg")
}

func (h *Host) CmdQuit() {
	h.Abort("quit")
}

func (h *Host) CmdExec(filename string, subs *Subsystems) {
	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}

	if filename == "" {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: exec <filename>\n")
		}
		return
	}

	var (
		data []byte
		err  error
	)
	switch {
	case filepath.IsAbs(filename):
		data, err = os.ReadFile(filename)
	case h.userDir != "":
		data, err = os.ReadFile(filepath.Join(h.userDir, filename))
	default:
		err = os.ErrNotExist
	}
	if err == nil {
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("execing %s\n", filename))
		}
		executeConfigText(subs, string(data))
		return
	}
	if err != nil && !os.IsNotExist(err) {
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("couldn't exec %s: %v\n", filename, err))
		}
		return
	}
	if subs != nil && subs.Files != nil {
		data, err = subs.Files.LoadFile(filename)
		if err == nil {
			if subs != nil && subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("execing %s\n", filename))
			}
			executeConfigText(subs, string(data))
			return
		}
	}
	if subs != nil && subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("couldn't exec %s\n", filename))
	}
}

func (h *Host) CmdEcho(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	subs.Console.Print(strings.Join(args, " ") + "\n")
}

func (h *Host) CmdClear(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	subs.Console.Clear()
}

func (h *Host) CmdCondump(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	filename := "condump.txt"
	if len(args) > 0 {
		filename = args[0]
	}

	path := filename
	if h.userDir != "" && !filepath.IsAbs(filename) {
		path = filepath.Join(h.userDir, filename)
	}

	if err := subs.Console.Dump(path); err != nil {
		subs.Console.Print(fmt.Sprintf("condump failed: %v\n", err))
	} else {
		subs.Console.Print(fmt.Sprintf("Dumped console text to %s.\n", filename))
	}
}

func (h *Host) CmdAlias(args []string, subs *Subsystems) {
	switch len(args) {
	case 0:
		aliases := cmdsys.Aliases()
		if len(aliases) == 0 {
			if subs != nil && subs.Console != nil {
				subs.Console.Print("no alias commands found\n")
			}
			return
		}
		count := 0
		for name, value := range aliases {
			if subs != nil && subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("   %s: %s", name, value))
			}
			count++
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("%d alias command(s)\n", count))
		}
	case 1:
		if value, ok := cmdsys.Alias(args[0]); ok {
			if subs != nil && subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("   %s: %s", strings.ToLower(args[0]), value))
			}
		}
	default:
		name := args[0]
		if len(name) >= maxAliasName {
			if subs != nil && subs.Console != nil {
				subs.Console.Print("Alias name is too long\n")
			}
			return
		}
		command := strings.Join(args[1:], " ") + "\n"
		cmdsys.AddAlias(name, command)
	}
}

func (h *Host) CmdUnalias(args []string, subs *Subsystems) {
	if len(args) != 1 {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("unalias <name> : delete alias\n")
		}
		return
	}
	if !cmdsys.RemoveAlias(args[0]) {
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("No alias named %s\n", args[0]))
		}
	}
}

func (h *Host) CmdUnaliasAll() {
	cmdsys.UnaliasAll()
}

func (h *Host) CmdMap(mapName string, subs *Subsystems) error {
	if subs == nil {
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
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

func (h *Host) CmdSkill(skill int) {
	if skill < 0 {
		skill = 0
	}
	if skill > 3 {
		skill = 3
	}
	h.currentSkill = skill
}

func (h *Host) CmdPause() {
	if h.serverActive && h.maxClients == 1 {
		h.serverPaused = !h.serverPaused
	}
}

func (h *Host) CmdGod(subs *Subsystems) {
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
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
	}

	if subs != nil && subs.Server != nil {
		subs.Server.Shutdown()
	}

	if subs != nil && subs.Client != nil {
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
	subs.Server.SaveSpawnParms()
	if fsInstance, ok := subs.Files.(*fs.FileSystem); ok {
		if err := subs.Server.SpawnServer(level, fsInstance); err != nil {
			h.Error(fmt.Sprintf("failed to change level to %s: %v", level, err), subs)
		}
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
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
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
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
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
		if cached, ok := hostSubsystemRegistry.Load(h); ok {
			subs, _ = cached.(*Subsystems)
		}
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

func (h *Host) CmdKill(subs *Subsystems) {
	if !h.serverActive || subs.Server == nil {
		return
	}
	ent := h.getLocalPlayerEdict(subs)
	if ent == nil {
		return
	}
	ent.Vars.Health = 0
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
	if subs == nil || subs.Console == nil {
		return
	}
	path, data, err := h.readSaveFile(name)
	if err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	var save hostSaveFile
	if err := json.Unmarshal(data, &save); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	if save.Server == nil {
		subs.Console.Print("load failed: savegame is missing server state\n")
		return
	}
	if subs.Server == nil {
		subs.Console.Print("load failed: server is not initialized\n")
		return
	}
	if subs.Server.GetMaxClients() != 1 {
		subs.Console.Print("load failed: savegames require single-player mode\n")
		return
	}
	srv, ok := subs.Server.(*server.Server)
	if !ok {
		subs.Console.Print("load failed: savegames require the built-in server\n")
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		subs.Console.Print("load failed: filesystem implementation is missing\n")
		return
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
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	srv.LoadGame = true
	defer func() { srv.LoadGame = false }()
	if err := subs.Server.SpawnServer(save.Server.MapName, fsInstance); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}
	if err := h.startLocalServerSession(subs, func() error {
		if err := srv.RestoreSaveGameState(save.Server); err != nil {
			return err
		}
		h.currentSkill = save.Skill
		return nil
	}); err != nil {
		subs.Console.Print(fmt.Sprintf("load failed: %v\n", err))
		return
	}

	subs.Console.Print(fmt.Sprintf("Loaded %s\n", filepath.Base(path)))
}

func (h *Host) CmdSave(name string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	path, err := h.saveFilePath(name)
	if err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	if subs.Server == nil || !h.serverActive || !subs.Server.IsActive() {
		subs.Console.Print("save failed: no active game\n")
		return
	}
	if subs.Server.GetMaxClients() != 1 {
		subs.Console.Print("save failed: savegames require single-player mode\n")
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
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		subs.Console.Print(fmt.Sprintf("save failed: %v\n", err))
		return
	}

	subs.Console.Print(fmt.Sprintf("Saved %s\n", filepath.Base(path)))
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

func (h *Host) saveFilePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("save name is required")
	}
	if !saveNamePattern.MatchString(name) || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid save name %q", name)
	}
	if h.userDir == "" {
		return "", fmt.Errorf("user directory is not initialized")
	}
	return filepath.Join(h.userDir, "saves", name+".sav"), nil
}

func (h *Host) readSaveFile(name string) (string, []byte, error) {
	searchPaths, err := h.saveFileSearchPaths(name)
	if err != nil {
		return "", nil, err
	}

	for _, path := range searchPaths {
		data, err := os.ReadFile(path)
		if err == nil {
			return path, data, nil
		}
		if !os.IsNotExist(err) {
			return "", nil, err
		}
	}

	return "", nil, fmt.Errorf("%s not found", filepath.Base(searchPaths[0]))
}

func (h *Host) saveFileSearchPaths(name string) ([]string, error) {
	userPath, err := h.saveFilePath(name)
	if err != nil {
		return nil, err
	}

	searchPaths := []string{userPath}
	if h.baseDir == "" {
		return searchPaths, nil
	}

	legacyName := name + ".sav"
	// 2. Active game directory
	if gameDir := strings.TrimSpace(h.gameDir); gameDir != "" && gameDir != "id1" {
		searchPaths = append(searchPaths, filepath.Join(h.baseDir, gameDir, legacyName))
	}

	// 3. Base directory
	searchPaths = append(searchPaths, filepath.Join(h.baseDir, legacyName))

	// 4. Vanilla Quake directory
	if strings.TrimSpace(h.gameDir) != "id1" {
		searchPaths = append(searchPaths, filepath.Join(h.baseDir, "id1", legacyName))
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

		_, data, err := h.readSaveFile(slotName)
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

func (h *Host) Error(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Host_Error: %s\n", message))
	}

	h.EndGame(message, subs)
}

// Menu commands

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
	// Switch to quit state
	h.menu.ShowMenu()
	// Note: The menu system handles quit confirmation internally
}

// Demo commands

func (h *Host) CmdRecord(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	// Check if already recording
	if h.demoState != nil && h.demoState.Recording {
		subs.Console.Print("Already recording a demo. Use 'stop' to end recording.\n")
		return
	}

	// Check if playing back
	if h.demoState != nil && h.demoState.Playback {
		subs.Console.Print("Cannot record during demo playback.\n")
		return
	}

	// Create demo state if needed
	if h.demoState == nil {
		h.demoState = &client.DemoState{
			Speed:     1.0,
			BaseSpeed: 1.0,
		}
	}

	// Get CD track (default to 0)
	cdtrack := 0
	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		cdtrack = loopbackClient.CDTrack
	}

	// Start recording
	if err := h.demoState.StartDemoRecording(filename, cdtrack); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to start recording: %v\n", err))
		return
	}

	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil && loopbackClient.State != cl.StateDisconnected && loopbackClient.Signon > 0 {
		if err := h.demoState.WriteInitialStateSnapshot(loopbackClient); err != nil {
			stopErr := h.demoState.StopRecording()
			if stopErr != nil {
				subs.Console.Print(fmt.Sprintf("Failed to capture initial demo state: %v (also failed to close demo: %v)\n", err, stopErr))
				return
			}
			subs.Console.Print(fmt.Sprintf("Failed to capture initial demo state: %v\n", err))
			return
		}
	}

	subs.Console.Print(fmt.Sprintf("Recording demo to %s\n", h.demoState.Filename))
}

func (h *Host) CmdStop(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	if h.demoState == nil || !h.demoState.Recording {
		subs.Console.Print("Not recording a demo.\n")
		return
	}

	var viewAngles [3]float32
	if loopbackClient := LoopbackClientState(subs); loopbackClient != nil {
		viewAngles = loopbackClient.ViewAngles
	}

	trailerErr := h.demoState.WriteDisconnectTrailer(viewAngles)
	stopErr := h.demoState.StopRecording()
	if trailerErr != nil {
		if stopErr != nil {
			subs.Console.Print(fmt.Sprintf("Error writing disconnect trailer: %v (also failed to close demo: %v)\n", trailerErr, stopErr))
			return
		}
		subs.Console.Print(fmt.Sprintf("Error writing disconnect trailer: %v\n", trailerErr))
		return
	}
	if stopErr != nil {
		subs.Console.Print(fmt.Sprintf("Error stopping demo: %v\n", stopErr))
		return
	}

	subs.Console.Print("Completed demo\n")
}

func (h *Host) CmdPlaydemo(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	// Check if already playing back
	if h.demoState != nil && h.demoState.Playback {
		subs.Console.Print("Already playing back a demo.\n")
		return
	}

	// Check if recording
	if h.demoState != nil && h.demoState.Recording {
		subs.Console.Print("Cannot playback while recording.\n")
		return
	}

	// Disconnect from any current server
	if h.serverActive {
		h.ShutdownServer(subs)
	}
	h.clientState = caDisconnected

	// Create demo state if needed
	if h.demoState == nil {
		h.demoState = &client.DemoState{
			Speed:     1.0,
			BaseSpeed: 1.0,
		}
	}

	// Start playback
	if err := h.demoState.StartDemoPlayback(filename); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to start playback: %v\n", err))
		return
	}

	subs.Console.Print(fmt.Sprintf("Playing demo from %s\n", h.demoState.Filename))

	// Set client state to connected for demo playback
	h.clientState = caConnected

	// Reset the actual client so recorded serverinfo/signon frames can bootstrap playback.
	if clientState := LoopbackClientState(subs); clientState != nil {
		clientState.ClearState()
		clientState.State = cl.StateDisconnected
	}
}

func (h *Host) CmdStopdemo(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}

	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}

	if err := h.demoState.StopPlayback(); err != nil {
		subs.Console.Print(fmt.Sprintf("Error stopping demo playback: %v\n", err))
		return
	}

	subs.Console.Print("Demo playback stopped.\n")
	h.clientState = caDisconnected
	h.SetDemoNum(-1)
}

// MaxDemos is the maximum number of demos in a startdemos playlist.
const MaxDemos = 8

// CmdStartdemos stores a list of demo names for attract-mode cycling.
// If no game is active it begins playback immediately.
func (h *Host) CmdStartdemos(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if len(args) == 0 {
		subs.Console.Print("usage: startdemos <demo1> [demo2] ...\n")
		return
	}

	count := len(args)
	if count > MaxDemos {
		count = MaxDemos
	}
	h.SetDemoList(args[:count])
	h.SetDemoNum(0)

	// If no game is in progress, start playing the first demo now.
	if h.clientState == caDisconnected && !h.serverActive {
		h.CmdDemos(subs)
	}
}

// CmdDemos restarts the demo loop from the current position.
func (h *Host) CmdDemos(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.DemoNum() < 0 {
		subs.Console.Print("No demo loop active.\n")
		return
	}

	demos := h.DemoList()
	if len(demos) == 0 {
		h.SetDemoNum(-1)
		return
	}

	num := h.DemoNum()
	// Wrap around when we reach the end.
	if num >= len(demos) || demos[num] == "" {
		num = 0
		h.SetDemoNum(num)
		if len(demos) == 0 || demos[0] == "" {
			h.SetDemoNum(-1)
			return
		}
	}

	h.CmdPlaydemo(demos[num], subs)
	h.SetDemoNum(num + 1)
}

func (h *Host) CmdTimedemo(filename string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	h.CmdPlaydemo(filename, subs)
	if h.demoState == nil || !h.demoState.Playback {
		return
	}
	h.demoState.EnableTimeDemo()
	subs.Console.Print(fmt.Sprintf("Timing demo %s\n", h.demoState.Filename))
}

func (h *Host) CmdDemoSeek(frame int, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}
	if frame < 0 || frame >= len(h.demoState.Frames) {
		subs.Console.Print(fmt.Sprintf("Frame %d out of range (0-%d).\n", frame, len(h.demoState.Frames)))
		return
	}
	if err := h.seekDemoFrame(frame, subs); err != nil {
		subs.Console.Print(fmt.Sprintf("Failed to seek demo: %v\n", err))
		return
	}
	subs.Console.Print(fmt.Sprintf("Demo seeked to frame %d.\n", frame))
}

func (h *Host) CmdRewind(frames int, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	if h.demoState == nil || !h.demoState.Playback {
		subs.Console.Print("Not playing back a demo.\n")
		return
	}
	if frames <= 0 {
		frames = 1
	}
	target := h.demoState.FrameIndex - frames
	if target < 0 {
		target = 0
	}
	h.CmdDemoSeek(target, subs)
}

func (h *Host) seekDemoFrame(frame int, subs *Subsystems) error {
	if h.demoState == nil {
		return fmt.Errorf("demo state unavailable")
	}
	clientState := LoopbackClientState(subs)
	if clientState == nil {
		return fmt.Errorf("loopback client state unavailable")
	}
	if err := h.demoState.SeekFrame(0); err != nil {
		return err
	}
	clientState.ClearState()
	clientState.State = cl.StateDisconnected
	h.clientState = caConnected
	h.signOns = 0

	parser := cl.NewParser(clientState)
	for i := 0; i < frame; i++ {
		msgData, viewAngles, err := h.demoState.ReadDemoFrame()
		if err != nil {
			return fmt.Errorf("read frame %d: %w", i, err)
		}
		clientState.MViewAngles[1] = clientState.MViewAngles[0]
		clientState.MViewAngles[0] = viewAngles
		clientState.ViewAngles = viewAngles
		if err := parser.ParseServerMessage(msgData); err != nil {
			return fmt.Errorf("parse frame %d: %w", i, err)
		}
		DispatchLoopbackStuffText(subs)
	}
	return nil
}
