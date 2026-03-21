package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	ralphVerbose = parseBoolEnv("RALPH_VERBOSE", false)

	args := os.Args[1:]
	for len(args) > 0 {
		switch args[0] {
		case "--verbose", "-v":
			ralphVerbose = true
			args = args[1:]
		default:
			goto dispatch
		}
	}

dispatch:
	if len(args) < 1 {
		usage()
		os.Exit(2)
	}

	var exitCode int
	switch args[0] {
	case "analyze-log":
		exitCode = runAnalyzeLog(args[1:])
	case "build-prompt":
		exitCode = runBuildPrompt(args[1:])
	case "loop":
		exitCode = runLoop(args[1:])
	case "help", "--help", "-h":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown Ralph subcommand %q\n\n", args[0])
		usage()
		exitCode = 2
	}

	os.Exit(exitCode)
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go run ./tools/ralph [--verbose] <%s> [args...]\n", strings.Join([]string{
		"analyze-log",
		"build-prompt",
		"loop",
	}, "|"))
}
