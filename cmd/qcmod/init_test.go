package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runInitForTest invokes runInit into a fresh temp dir and returns it.
func runInitForTest(t *testing.T, args ...string) (dir string, out string, code int) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "mygame")
	fullArgs := append(append([]string{}, args...), dir)
	var stdout, stderr strings.Builder
	code = runInit(fullArgs, &stdout, &stderr)
	t.Logf("qoqm init stderr: %s", stderr.String())
	return dir, stdout.String(), code
}

func readGenerated(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func TestInitScaffoldsGenericTree(t *testing.T) {
	dir, _, code := runInitForTest(t, "-kind", "generic")
	if code != 0 {
		t.Fatalf("runInit exit = %d, want 0", code)
	}
	for _, rel := range []string{
		"go.mod", "main.go", "gameconfig.go", "progs/progs.go",
		"game_test.go", "Makefile", "README.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing generated file %s: %v", rel, err)
		}
	}
}

func TestInitGoModContract(t *testing.T) {
	dir, _, code := runInitForTest(t, "-kind", "tc")
	if code != 0 {
		t.Fatalf("runInit exit = %d, want 0", code)
	}

	// The generated go.mod must parse and declare both module requires and
	// their replace directives — this is what makes the scaffold buildable.
	cmd := exec.Command("go", "mod", "edit", "-json")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod edit -json: %v\n%s", err, out)
	}
	var mod struct {
		Module struct{ Path string }
		Require []struct {
			Path    string
			Version string
		}
		Replace []struct {
			Old struct{ Path string }
			New struct{ Path string }
		}
	}
	if err := json.Unmarshal(out, &mod); err != nil {
		t.Fatalf("parse go.mod json: %v", err)
	}
	if mod.Module.Path != "mygame" {
		t.Errorf("module path = %q, want mygame", mod.Module.Path)
	}
	required := map[string]bool{}
	for _, r := range mod.Require {
		required[r.Path] = true
	}
	for _, want := range []string{"github.com/darkliquid/ironwail-go", "quake"} {
		if !required[want] {
			t.Errorf("go.mod missing require %s", want)
		}
	}
	replaced := map[string]string{}
	for _, r := range mod.Replace {
		replaced[r.Old.Path] = r.New.Path
	}
	for old, wantSuffix := range map[string]string{
		"github.com/darkliquid/ironwail-go": "",                      // engine root
		"quake":                            "/pkg/qgo/quake",         // quake module
	} {
		newPath, ok := replaced[old]
		if !ok {
			t.Errorf("go.mod missing replace for %s", old)
			continue
		}
		if wantSuffix != "" && !strings.HasSuffix(newPath, wantSuffix) {
			t.Errorf("replace %s => %q does not end in %q", old, newPath, wantSuffix)
		}
	}
	// Replace paths must be go.mod-valid directory paths (rooted, or ./ or
	// ../-prefixed) — never bare relative like "ironwail-go".
	for _, r := range mod.Replace {
		p := r.New.Path
		if !filepath.IsAbs(p) && !strings.HasPrefix(p, "./") && !strings.HasPrefix(p, "../") {
			t.Errorf("replace new path %q is not a valid module directory path", p)
		}
	}
}

func TestInitMainImportsPublicSDKOnly(t *testing.T) {
	dir, _, code := runInitForTest(t)
	if code != 0 {
		t.Fatalf("runInit exit = %d, want 0", code)
	}
	main := readGenerated(t, dir, "main.go")
	if !strings.Contains(main, "github.com/darkliquid/ironwail-go/sdk") {
		t.Error("main.go must import the public sdk package")
	}
	if strings.Contains(main, "internal/") {
		t.Error("main.go must not import engine internal packages (Go internal rule)")
	}
	config := readGenerated(t, dir, "gameconfig.go")
	if !strings.Contains(config, "GameName:          \"Mygame\"") {
		t.Errorf("gameconfig.go missing title-cased GameName:\n%s", config)
	}
	if !strings.Contains(config, "BaseGameDir:       \"mygame\"") {
		t.Errorf("gameconfig.go missing BaseGameDir:\n%s", config)
	}
}

func TestInitKinds(t *testing.T) {
	for _, tc := range []struct {
		kind     string
		file     string
		contains []string
	}{
		{kindGeneric, "gameconfig.go", []string{"RequireRegistered: false", "DefaultRegistered: true"}},
		{kindSP, "progs/progs.go", []string{"SpawnPlayerStart", "HealthPickupThink"}},
		{kindSP, "game_test.go", []string{"TestSinglePlayerDefaults"}},
		{kindDM, "gameconfig.go", []string{"DefaultDeathmatch: 1"}},
		{kindDM, "game_test.go", []string{"TestDeathmatchDefaults"}},
		{kindDM, "progs/progs.go", []string{"RespawnItemThink"}},
		{kindTC, "gameconfig.go", []string{"ModDirMenuLabel", "CSQCInitName"}},
		{kindTC, "game_test.go", []string{"TestTotalConversionIdentity"}},
	} {
		dir, _, code := runInitForTest(t, "-kind", tc.kind)
		if code != 0 {
			t.Fatalf("kind %s: runInit exit = %d, want 0", tc.kind, code)
		}
		content := readGenerated(t, dir, tc.file)
		for _, want := range tc.contains {
			if !strings.Contains(content, want) {
				t.Errorf("kind %s %s missing %q", tc.kind, tc.file, want)
			}
		}
	}
}

func TestInitRejectsInvalidArgs(t *testing.T) {
	_, _, code := runInitForTest(t, "-kind", "bogus")
	if code != 2 {
		t.Errorf("invalid kind: exit = %d, want 2", code)
	}
	// Missing dir argument.
	var stdout, stderr strings.Builder
	if code := runInit(nil, &stdout, &stderr); code != 2 {
		t.Errorf("missing dir: exit = %d, want 2", code)
	}
	// Existing dir.
	dir := t.TempDir()
	if code := runInit([]string{dir}, &stdout, &stderr); code != 1 {
		t.Errorf("existing dir: exit = %d, want 1", code)
	}
}

func TestReplacePath(t *testing.T) {
	// Sibling mod dir: relative ".." path (valid go.mod directory path).
	got := replacePath("/home/user/games/mygame", "/home/user/games/ironwail-go", false)
	if got != "../ironwail-go" {
		t.Errorf("sibling replace path = %q, want ../ironwail-go", got)
	}
	// Mod dir inside the checkout: relative "./" prefixed path.
	if got := replacePath("/home/user/ironwail-go/mods/mygame", "/home/user/ironwail-go", false); got != "../.." {
		t.Errorf("nested replace path = %q, want ../..", got)
	}
	// Absolute mode.
	if got := replacePath("/home/user/games/mygame", "/home/user/games/ironwail-go", true); !filepath.IsAbs(got) {
		t.Errorf("absolute replace path = %q, want absolute", got)
	}
}

func TestResolveEngineRootExplicit(t *testing.T) {
	root, err := resolveEngineRoot("definitely-not-a-checkout")
	if err == nil {
		t.Fatalf("resolveEngineRoot(bogus) = %q, want error", root)
	}
}

func TestTitleCase(t *testing.T) {
	for in, want := range map[string]string{
		"mygame":    "Mygame",
		"my-mod":    "MyMod",
		"my_mod":    "MyMod",
		"a b c":     "ABC",
		"ironwail":  "Ironwail",
	} {
		if got := titleCase(in); got != want {
			t.Errorf("titleCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestInitScaffoldBuilds is the opt-in end-to-end check: a scaffold from
// every kind must resolve its modules (go mod tidy), pass its tests, and
// build a binary. It is gated behind QC_E2E_BUILD because it compiles the
// full engine dependency graph via the sdk package.
func TestInitScaffoldBuilds(t *testing.T) {
	if os.Getenv("QC_E2E_BUILD") == "" {
		t.Skip("set QC_E2E_BUILD=1 to run the full scaffold build (slow)")
	}
	for _, kind := range templateKinds {
		dir, _, code := runInitForTest(t, "-kind", kind)
		if code != 0 {
			t.Fatalf("kind %s: runInit exit = %d", kind, code)
		}
		run := func(name string, args ...string) {
			t.Helper()
			cmd := exec.Command(name, args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("kind %s: %s %v: %s", kind, name, err, out)
			}
		}
		run("go", "mod", "tidy")
		run("go", "test", "./...")
		run("go", "build", "-o", "bin/mygame", ".")
	}
}