package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/host"
)

type startupOptions struct {
	BaseDir    string
	GameDir    string
	Dedicated  bool
	Listen     bool
	MaxClients int
	Port       int
	Args       []string
}

func parseStartupOptions(rawArgs []string) (startupOptions, error) {
	opts := startupOptions{
		BaseDir:    ".",
		GameDir:    "id1",
		MaxClients: 1,
		Port:       26000,
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
		default:
			opts.Args = append(opts.Args, arg)
		}
	}

	return opts, nil
}
