package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/gameconfig"
	"github.com/darkliquid/ironwail-go/internal/host"
)

type startupOptions struct {
	BaseDir     string
	GameDir     string
	Dedicated   bool
	Listen      bool
	MaxClients  int
	Port        int
	QCDebug     bool
	QCDebugPort int
	QCDebugWait bool
	Args        []string
}

func parseStartupOptions(rawArgs []string) (startupOptions, error) {
	opts := startupOptions{
		BaseDir:     ".",
		GameDir:     gameconfig.Default().BaseGameDir,
		MaxClients:  1,
		Port:        26000,
		QCDebugPort: 2345,
	}

	parseOptionalCount := func(args []string, idx int, defaultValue int) (value int, consumed bool) {
		value = defaultValue
		if idx+1 >= len(args) {
			return value, false
		}
		next := args[idx+1]
		if strings.HasPrefix(next, "-") || strings.HasPrefix(next, "+") {
			return value, false
		}
		n, err := strconv.Atoi(next)
		if err != nil {
			return value, false
		}
		if n < 1 {
			n = 1
		}
		if n > host.MaxScoreboard {
			n = host.MaxScoreboard
		}
		return n, true
	}

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		switch {
		case strings.EqualFold(arg, "-basedir"):
			if i+1 >= len(rawArgs) {
				return opts, fmt.Errorf("-basedir requires a path")
			}
			opts.BaseDir = rawArgs[i+1]
			i++
		case strings.EqualFold(arg, "-game"):
			if i+1 >= len(rawArgs) {
				return opts, fmt.Errorf("-game requires a directory")
			}
			opts.GameDir = rawArgs[i+1]
			i++
		case strings.EqualFold(arg, "-port"):
			if i+1 >= len(rawArgs) {
				return opts, fmt.Errorf("-port requires a value")
			}
			port, err := strconv.Atoi(rawArgs[i+1])
			if err != nil || port <= 0 {
				return opts, fmt.Errorf("invalid -port value %q", rawArgs[i+1])
			}
			opts.Port = port
			i++
		case strings.EqualFold(arg, "-dedicated"):
			if opts.Listen {
				return opts, fmt.Errorf("-dedicated and -listen are mutually exclusive")
			}
			opts.Dedicated = true
			opts.MaxClients = 8
			if count, consumed := parseOptionalCount(rawArgs, i, opts.MaxClients); consumed {
				opts.MaxClients = count
				i++
			}
		case strings.EqualFold(arg, "-listen"):
			if opts.Dedicated {
				return opts, fmt.Errorf("-dedicated and -listen are mutually exclusive")
			}
			opts.Listen = true
			opts.MaxClients = 8
			if count, consumed := parseOptionalCount(rawArgs, i, opts.MaxClients); consumed {
				opts.MaxClients = count
				i++
			}
		case strings.EqualFold(arg, "-qcdbg"):
			opts.QCDebug = true
			if i+1 < len(rawArgs) {
				next := rawArgs[i+1]
				if !strings.HasPrefix(next, "-") && !strings.HasPrefix(next, "+") {
					if port, err := strconv.Atoi(next); err == nil && port > 0 {
						opts.QCDebugPort = port
						i++
					}
				}
			}
		case strings.EqualFold(arg, "-qcdbg-wait"):
			opts.QCDebugWait = true
		default:
			opts.Args = append(opts.Args, arg)
		}
	}

	return opts, nil
}

// reorderFlagsFirst rebuilds an argument list with all registered -flags
// moved ahead of positional/+command arguments, so the stdlib flag package
// (which stops parsing at the first non-flag) sees every flag regardless of
// how the user interleaved +cvar commands or bare arguments. The relative
// order within each group is preserved.
//
// lookup reports whether a flag name is known and whether it is boolean
// (boolean flags never consume the following argument as a value, matching
// stdlib flag semantics). Unknown -args are kept with the flags so the
// parser still reports them instead of silently dropping them.
//
// "--" terminates flag parsing as usual: everything after it is positional.
func reorderFlagsFirst(args []string, lookup func(name string) (known, isBool bool)) []string {
	flagArgs := make([]string, 0, len(args))
	other := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			flagArgs = append(flagArgs, arg)
			other = append(other, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			other = append(other, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		known, isBool := lookup(name)
		flagArgs = append(flagArgs, arg)
		if known && !isBool && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			flagArgs = append(flagArgs, args[i])
		}
	}
	return append(flagArgs, other...)
}
