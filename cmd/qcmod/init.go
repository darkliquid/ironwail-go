package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

//go:embed template
var templateFS embed.FS

// runInit implements `qcmod init [-kind <kind>] [-engine <path>] [dir]`,
// scaffolding a new standalone mod from an embedded template (bead
// ironwail-go-uxr; SPEC-006 M5).
//
// The generated mod is a separate Go module that imports the public sdk
// package (github.com/darkliquid/ironwail-go/sdk) — never internal packages,
// which Go restricts to the engine module itself.
func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	kind := fs.String("kind", kindGeneric, "template kind: "+strings.Join(templateKinds, ", "))
	pkg := fs.String("pkg", "progs", "Go package name for the QuakeGo sources")
	enginePath := fs.String("engine", "", "path to the ironwail-go checkout (default: auto-detect from build location or $IRONWAIL_GO_ROOT)")
	useAbsolute := fs.Bool("absolute", false, "write absolute replace paths instead of relative")
	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod init: %v\n", err)
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod init: exactly one directory path is required")
		return 2
	}
	if !validKind(*kind) {
		_, _ = fmt.Fprintf(stderr, "qcmod init: unknown kind %q (valid: %s)\n", *kind, strings.Join(templateKinds, ", "))
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

	engineRoot, err := resolveEngineRoot(*enginePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod init: %v\n", err)
		return 1
	}

	data := initData{
		Module:     base,
		Pkg:        *pkg,
		GameName:   titleCase(base),
		BaseDir:    base,
		EngineRoot: replacePath(dir, engineRoot, *useAbsolute),
	}

	if err := scaffoldMod(dir, *kind, data); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod init: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "Created %s/ (%s template)\n", dir, *kind)
	_, _ = fmt.Fprintf(stdout, "  go.mod          module %s; quake + engine module replaces\n", base)
	_, _ = fmt.Fprintf(stdout, "  main.go         game entry point via sdk.Run\n")
	_, _ = fmt.Fprintf(stdout, "  gameconfig.go   sdk.Config pre-populated for %q\n", data.GameName)
	_, _ = fmt.Fprintf(stdout, "  progs/%s.go     QuakeGo gameplay sources\n", *pkg)
	_, _ = fmt.Fprintf(stdout, "  game_test.go    example sim test\n")
	_, _ = fmt.Fprintf(stdout, "  Makefile        build / run / test targets\n")
	_, _ = fmt.Fprintf(stdout, "  README.md       layout + data placement\n")
	_, _ = fmt.Fprintf(stdout, "\nNext steps:\n")
	_, _ = fmt.Fprintf(stdout, "  cd %s && go mod tidy\n", dir)
	_, _ = fmt.Fprintf(stdout, "  go test ./...\n")
	_, _ = fmt.Fprintf(stdout, "  make run   (runs from the parent dir so ./%s resolves)\n", base)
	return 0
}

const (
	kindGeneric = "generic"
	kindSP      = "sp"
	kindDM      = "dm"
	kindTC      = "tc"
)

var templateKinds = []string{kindGeneric, kindSP, kindDM, kindTC}

func validKind(kind string) bool {
	for _, k := range templateKinds {
		if k == kind {
			return true
		}
	}
	return false
}

// initData is the render context for every embedded template.
type initData struct {
	Module     string // go module name (= directory basename)
	Pkg        string // Go package name for QuakeGo sources
	GameName   string // title-cased game name
	BaseDir    string // base game data directory name
	EngineRoot string // replace-directive path to the engine checkout
}

// scaffoldMod renders the template tree for kind into dir. Shared files
// come from template/, kind-specific files from template/<kind>/; the
// trailing .tmpl suffix is stripped from generated file names.
func scaffoldMod(dir, kind string, data initData) error {
	shared := []string{"go.mod.tmpl", "main.go.tmpl", "Makefile.tmpl"}
	kindFiles := []string{
		"gameconfig.go.tmpl",
		"progs/progs.go.tmpl",
		"game_test.go.tmpl",
		"README.md.tmpl",
	}

	render := func(src, dst string) error {
		content, err := templateFS.ReadFile("template/" + src)
		if err != nil {
			return fmt.Errorf("read template %s: %w", src, err)
		}
		tpl, err := template.New(src).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parse template %s: %w", src, err)
		}
		var buf strings.Builder
		if err := tpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("render template %s: %w", src, err)
		}
		path := filepath.Join(dir, dst)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		return nil
	}

	for _, name := range shared {
		if err := render(name, strings.TrimSuffix(name, ".tmpl")); err != nil {
			return err
		}
	}
	for _, name := range kindFiles {
		if err := render(kind+"/"+name, strings.TrimSuffix(name, ".tmpl")); err != nil {
			return err
		}
	}
	return nil
}

// resolveEngineRoot locates the ironwail-go checkout whose sdk package the
// generated mod will import. Resolution order: explicit -engine flag, the
// directory holding this binary's source (compile-time path, valid for
// builds made inside a checkout), then $IRONWAIL_GO_ROOT. The returned path
// is absolute and verified against the engine module's go.mod.
func resolveEngineRoot(explicit string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err == nil {
			if err := validateEngineRoot(root); err == nil {
				return root, nil
			}
		}
		return "", fmt.Errorf("invalid engine path %q: %v", explicit, validateEngineRoot(mustAbs(explicit)))
	}

	// Walk up from this source file (present at compile time for builds made
	// inside a checkout — `go run ./cmd/qcmod` and `go install` both keep it).
	if _, file, _, ok := runtime.Caller(0); ok {
		for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
			if err := validateEngineRoot(dir); err == nil {
				return dir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}

	if env := os.Getenv("IRONWAIL_GO_ROOT"); env != "" {
		root, err := filepath.Abs(env)
		if err == nil {
			if err := validateEngineRoot(root); err == nil {
				return root, nil
			}
			return "", fmt.Errorf("$IRONWAIL_GO_ROOT %q is not an ironwail-go checkout: %v", env, validateEngineRoot(root))
		}
	}

	return "", fmt.Errorf("cannot locate the ironwail-go checkout: pass -engine <path> or set $IRONWAIL_GO_ROOT")
}

// validateEngineRoot checks that root is the engine module: the quake module
// lives beside it and root's go.mod declares the engine module path.
func validateEngineRoot(root string) error {
	if !fileExists(filepath.Join(root, "pkg", "qgo", "quake", "go.mod")) {
		return fmt.Errorf("no pkg/qgo/quake module under %s", root)
	}
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return fmt.Errorf("read %s/go.mod: %w", root, err)
	}
	if !strings.Contains(string(goMod), "module github.com/darkliquid/ironwail-go") {
		return fmt.Errorf("%s/go.mod does not declare module github.com/darkliquid/ironwail-go", root)
	}
	return nil
}

// replacePath renders the go.mod replace-directive path from the mod dir to
// the engine root: relative (with ./ or ../ prefix, as go.mod requires) when
// possible unless absolute is forced, else absolute.
func replacePath(modDir, engineRoot string, absolute bool) string {
	if !absolute {
		if absMod, err := filepath.Abs(modDir); err == nil {
			if rel, err := filepath.Rel(absMod, engineRoot); err == nil {
				clean := filepath.Clean(rel)
				if clean == "." {
					return "."
				}
				if !strings.HasPrefix(clean, "..") {
					return "./" + clean
				}
				return clean
			}
		}
	}
	if abs, err := filepath.Abs(engineRoot); err == nil {
		return abs
	}
	return engineRoot
}

func mustAbs(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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