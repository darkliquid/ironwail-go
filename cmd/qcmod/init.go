package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// runInit implements `qcmod init [flags] [dir]`, scaffolding a new
// standalone mod from a built-in template (bead ironwail-go-bvw).
func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	pkg := fs.String("pkg", "progs", "Go package name for the QuakeGo sources")
	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod init: %v\n", err)
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod init: exactly one directory path is required")
		return 2
	}

	dir := fs.Arg(0)
	base := filepath.Base(dir)
	if base == "." || base == "/" || base == "" {
		_, _ = fmt.Fprintf(stderr, "qcmod init: invalid directory %q\n", dir)
		return 2
	}

	if _, err := os.Stat(dir); err == nil {
		_, _ = fmt.Fprintf(stderr, "qcmod init: %s already exists\n", dir)
		return 1
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod init: mkdir %s: %v\n", dir, err)
		return 1
	}

	gameName := titleCase(base)
	files := map[string]string{
		"go.mod":         fmt.Sprintf(goModTmpl, base),
		"main.go":        mainTmpl,
		"gameconfig.go":  fmt.Sprintf(configTmpl, gameName, base),
		"progs/progs.go": fmt.Sprintf(progsTmpl, *pkg),
		"game_test.go":   testTmpl,
	}

	for rel, content := range files {
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod init: mkdir %s: %v\n", filepath.Dir(path), err)
			return 1
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod init: write %s: %v\n", path, err)
			return 1
		}
	}

	fmt.Fprintf(stdout, "Created %s/\n", dir)
	fmt.Fprintf(stdout, "  go.mod          module %s (quake module linked via replace)\n", base)
	fmt.Fprintf(stdout, "  main.go         game entry point using engine.Run\n")
	fmt.Fprintf(stdout, "  gameconfig.go   GameConfig pre-populated for %q\n", gameName)
	fmt.Fprintf(stdout, "  progs/progs.go  QuakeGo gameplay sources\n")
	fmt.Fprintf(stdout, "  game_test.go    example sim test\n")
	fmt.Fprintf(stdout, "\nNext steps:\n")
	fmt.Fprintf(stdout, "  cd %s && go mod tidy\n", dir)
	fmt.Fprintf(stdout, "  go test ./...\n")
	return 0
}

// titleCase converts a directory name to a title-cased game name.
func titleCase(dir string) string {
	parts := strings.FieldsFunc(dir, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

const goModTmpl = `module %s

go 1.26

require (
	quake v0.0.0
)

replace quake => %%s
`

const mainTmpl = `// Command <modname> is a standalone game built on the Ironwail-Go engine.
//
// The gameconfig.Config determines the game's identity, data paths, and
// feature gates. See the gameconfig package for all available fields.
package main

import (
	"github.com/darkliquid/ironwail-go/internal/engine"
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/gameconfig"
)

func main() {
	config := newGameConfig()
	g, err := engine.Run(config, engine.Args(os.Args...))
	if err != nil {
		panic(err)
	}
	_ = g // the engine owns the game loop; add hooks here as needed
}

func newGameConfig() gameconfig.Config {
	return gameconfig.Config{
		GameName:    "<GAME_NAME>",
		BaseGameDir: "<BASE_DIR>",
		RequireRegistered: false,
	}
}
`

const configTmpl = `package main

import "github.com/darkliquid/ironwail-go/internal/gameconfig"

// newGameConfig returns the game's identity and feature configuration.
// All fields are documented in the gameconfig package. Unset fields
// resolve to stock Quake defaults via Resolve().
func newGameConfig() gameconfig.Config {
	return gameconfig.Config{
		GameName:    "%s",
		BaseGameDir: "%s",
		RequireRegistered: false,
		DefaultRegistered: true,
	}
}
`

const progsTmpl = `package %s

// TODO: add QuakeGo gameplay functions here.
// These are compiled to progs.dat by cmd/qgo.
`

const testTmpl = `package main

import (
	"testing"

	"quake"
	"quake/sim"
)

func TestExample(t *testing.T) {
	w := sim.New()
	e := w.Spawn("info_player_start")
	if e == nil {
		t.Fatal("expected entity")
	}
	_ = quake.Entity{} // placeholder
}
`
