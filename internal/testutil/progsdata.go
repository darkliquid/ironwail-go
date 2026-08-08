package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// EnsureProgsData returns the progs.dat game-program blob usable by tests
// that need server-side QuakeC gameplay code.
//
// The modern Quake data distributions (e.g. the 2021 rerelease that ships
// along the "Quake Enhanced" install directories used by QUAKE_DIR) expect
// pre-compiled game code to live in binary formats such as KEX .kpf or inside
// the engine executable, so the id1 directory contains no progs.dat. The
// project keeps the original QuakeC gameplay sources as a Go port under
// pkg/qgo/quakego, and cmd/qgo compiles them back into a byte-identical
// progs.dat, so a missing progs.dat is not a fatal condition: it can be
// rebuilt from source.
//
// Precedence used here:
//
//  1. QUAKE_PROGS_DAT_PATH env var (explicit override).
//  2. A bundled progs.dat next to the test binary (set at build time).
//  3. A loose progs.dat in <quake dir>/id1.
//  4. A progs.dat compiled on the fly from pkg/qgo/quakego with qgo. The
//     result is deterministic (byte-identical across compiles), so it is
//     memoized per process.
func EnsureProgsData(t *testing.T, quakeDir string) []byte {
	t.Helper()
	if envPath := os.Getenv("QUAKE_PROGS_DAT_PATH"); envPath != "" {
		data, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("read QUAKE_PROGS_DAT_PATH %s: %v", envPath, err)
		}
		if len(data) == 0 {
			t.Fatalf("QUAKE_PROGS_DAT_PATH %s is empty", envPath)
		}
		return data
	}

	// A progs.dat placed next to the test binary at build time (e.g. the
	// regular `mise run build-progs` output copied beside the binary).
	if exe, err := os.Executable(); err == nil {
		if data, rerr := os.ReadFile(filepath.Join(filepath.Dir(exe), "progs.dat")); rerr == nil && len(data) > 0 {
			return data
		}
	}

	if quakeDir != "" {
		if data, err := os.ReadFile(filepath.Join(quakeDir, "id1", "progs.dat")); err == nil && len(data) > 0 {
			return data
		}
	}

	data, err := CompileProgsDataFromSource()
	if err != nil {
		t.Fatalf("no progs.dat available (looked in QUAKE_PROGS_DAT_PATH, next to test binary, and %s/id1; all absent) and compiling from pkg/qgo/quakego failed: %v", quakeDir, err)
	}
	return data
}

// CompileProgsDataFromSource builds pkg/qgo/quakego with cmd/qgo and returns
// the compiled progs.dat bytes. The result is deterministic and memoized, so
// sequential tests that fall back to compilation only pay for it once.
func CompileProgsDataFromSource() ([]byte, error) {
	compileProgsOnce.Do(func() {
		compileProgsData, compileProgsErr = compileProgsDataFromSource()
	})
	return compileProgsData, compileProgsErr
}

var (
	compileProgsOnce sync.Once
	compileProgsData []byte
	compileProgsErr  error
)

// compileProgsDataFromSource is the non-memoized implementation of
// CompileProgsDataFromSource.
func compileProgsDataFromSource() ([]byte, error) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}
	// pkg/qgo/quakego is a separate Go module ("module progs" with a
	// `replace quake => ../quake`). Loading it through its own module root
	// is what makes the internal `quake` imports resolve, so the qgo CLI
	// is invoked with that directory as its working directory.
	progsSrc := filepath.Join(root, "pkg", "qgo", "quakego")
	if _, err := os.Stat(progsSrc); err != nil {
		return nil, fmt.Errorf("quakego source not found at %s: %w", progsSrc, err)
	}

	out := filepath.Join(os.TempDir(), "ironwail-go-progs.dat")

	cmd := exec.Command("go", "run", filepath.Join(root, "cmd", "qgo"), "-o", out, ".")
	cmd.Dir = progsSrc
	// CGO_ENABLED=0 matches the project-wide build configuration.
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("qgo compile %s: %w: %s", progsSrc, err, stderr.String())
	}

	data, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("read compiled progs.dat: %w", err)
	}
	if len(data) == 0 {
		return nil, errors.New("compiled progs.dat is empty")
	}
	return data, nil
}
