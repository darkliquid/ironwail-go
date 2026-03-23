// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/fs"
)

func (h *Host) CmdQuit() {
	h.Abort("quit")
}

func (h *Host) CmdStuffCmds(subs *Subsystems) {
	if subs == nil || subs.Commands == nil {
		return
	}

	args := h.args
	if len(args) == 0 {
		args = os.Args[1:]
	}

	var (
		builder strings.Builder
		current []string
	)

	flush := func() {
		if len(current) == 0 {
			return
		}
		builder.WriteString(strings.Join(current, " "))
		builder.WriteByte('\n')
		current = nil
	}

	for _, arg := range args {
		if arg == "" {
			continue
		}
		switch {
		case strings.HasPrefix(arg, "+"):
			flush()
			command := strings.TrimPrefix(arg, "+")
			if command == "" {
				continue
			}
			current = []string{command}
		case strings.HasPrefix(arg, "-"):
			flush()
		default:
			if len(current) > 0 {
				current = append(current, arg)
			}
		}
	}
	flush()

	if builder.Len() > 0 {
		subs.Commands.InsertText(builder.String())
	}
}

func (h *Host) CmdExec(args []string, subs *Subsystems) {
	if subs == nil {
		subs = h.Subs
	}

	if len(args) == 0 {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: exec <filename>\n")
		}
		return
	}

	filename := args[0]
	if filename == "" {
		if subs != nil && subs.Console != nil {
			subs.Console.Print("usage: exec <filename>\n")
		}
		return
	}
	if len(args) == 1 && strings.EqualFold(filename, legacyConfigName) && h.userConfigFileExists(configFileName) {
		filename = configFileName
	}
	if builtin, ok := builtinExecConfigText(filename); ok {
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("execing %s\n", filename))
		}
		executeConfigText(subs, builtin)
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

func (h *Host) configFileExists(filename string, subs *Subsystems) bool {
	switch {
	case filepath.IsAbs(filename):
		_, err := os.Stat(filename)
		return err == nil
	case h.userDir != "":
		if _, err := os.Stat(filepath.Join(h.userDir, filename)); err == nil {
			return true
		}
	}
	return subs != nil && subs.Files != nil && subs.Files.FileExists(filename)
}

func (h *Host) userConfigFileExists(filename string) bool {
	switch {
	case filepath.IsAbs(filename):
		_, err := os.Stat(filename)
		return err == nil
	case h.userDir != "":
		_, err := os.Stat(filepath.Join(h.userDir, filename))
		return err == nil
	default:
		return false
	}
}

func (h *Host) CmdEcho(args []string, subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	subs.Console.Print(strings.Join(args, " ") + "\n")
}

func (h *Host) CmdPath(subs *Subsystems) {
	if subs == nil || subs.Console == nil || subs.Files == nil {
		return
	}
	fsInstance, ok := subs.Files.(*fs.FileSystem)
	if !ok {
		return
	}

	subs.Console.Print("Current search path:\n")
	for _, entry := range fsInstance.SearchPathEntries() {
		if entry.IsPack {
			subs.Console.Print(fmt.Sprintf("%s (%d files)\n", entry.Path, entry.FileCount))
			continue
		}
		subs.Console.Print(entry.Path + "\n")
	}
}

func (h *Host) CmdVersion(subs *Subsystems) {
	if subs == nil || subs.Console == nil {
		return
	}
	subs.Console.Print(fmt.Sprintf("Version %d.%d.%d (Ironwail Go)\n", h.versionMajor, h.versionMinor, h.versionPatch))
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
				subs.Console.Print(fmt.Sprintf("   %s: %s\n", name, value))
			}
			count++
		}
		if subs != nil && subs.Console != nil {
			subs.Console.Print(fmt.Sprintf("%d alias command(s)\n", count))
		}
	case 1:
		if value, ok := cmdsys.Alias(args[0]); ok {
			if subs != nil && subs.Console != nil {
				subs.Console.Print(fmt.Sprintf("   %s: %s\n", strings.ToLower(args[0]), value))
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

func (h *Host) Error(message string, subs *Subsystems) {
	if subs.Console != nil {
		subs.Console.Print(fmt.Sprintf("Host_Error: %s\n", message))
	}

	h.EndGame(message, subs)
}

// Menu commands
