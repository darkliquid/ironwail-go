package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
)

// writeTestTree creates a small mod-like directory tree in base.
func writeTestTree(t *testing.T, base string) {
	t.Helper()
	files := map[string]string{
		"progs.dat":          "progs-bytes",
		"maps/e1m1.bsp":      "bsp-data",
		"gfx/palette.lmp":    "palette",
		"sounds/door/dr1.wav": "sound-data",
	}
	for rel, content := range files {
		path := filepath.Join(base, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

func runPakForTest(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var stdout, stderr strings.Builder
	code := runPak(args, &stdout, &stderr)
	t.Logf("qcmod pak %v stderr: %s", args, stderr.String())
	return stdout.String(), code
}

func TestPakPackListUnpackRoundTrip(t *testing.T) {
	src := t.TempDir()
	writeTestTree(t, src)
	pakPath := filepath.Join(t.TempDir(), "pak0.pak")

	if out, code := runPakForTest(t, "pack", src, "-o", pakPath); code != 0 {
		t.Fatalf("pack exit = %d (%s)", code, out)
	}

	dest := t.TempDir()
	if out, code := runPakForTest(t, "unpack", pakPath, "-o", dest); code != 0 {
		t.Fatalf("unpack exit = %d (%s)", code, out)
	}

	// Tree equality: walk src and dest in parallel.
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		got, err := os.ReadFile(filepath.Join(dest, rel))
		if err != nil {
			return err
		}
		want, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s content differs after round trip", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("round-trip compare: %v", err)
	}

	// list output: one line per file, sorted by name.
	out, code := runPakForTest(t, "list", pakPath)
	if code != 0 {
		t.Fatalf("list exit = %d", code)
	}
	for _, want := range []string{"gfx/palette.lmp", "maps/e1m1.bsp", "progs.dat", "sounds/door/dr1.wav"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Errorf("list output has %d lines, want 4:\n%s", len(lines), out)
	}
	// First line (sorted): "<size>  gfx/palette.lmp".
	if !strings.HasSuffix(lines[0], "gfx/palette.lmp") || !strings.HasPrefix(strings.TrimSpace(lines[0]), "7") {
		t.Errorf("list first line %q does not show size + name", lines[0])
	}
}

func TestPakPackRejectsInvalidNames(t *testing.T) {
	t.Run("backslash name", func(t *testing.T) {
		src := t.TempDir()
		if err := os.WriteFile(filepath.Join(src, "a\\b.txt"), []byte("x"), 0o644); err != nil {
			t.Skipf("cannot create backslash filename: %v", err)
		}
		if _, code := runPakForTest(t, "pack", src, "-o", filepath.Join(t.TempDir(), "x.pak")); code != 1 {
			t.Errorf("pack exit = %d, want 1 (invalid name rejected)", code)
		}
	})
	t.Run("overlong name", func(t *testing.T) {
		src := t.TempDir()
		long := strings.Repeat("a", 57)
		if err := os.WriteFile(filepath.Join(src, long), []byte("x"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, code := runPakForTest(t, "pack", src, "-o", filepath.Join(t.TempDir(), "x.pak")); code != 1 {
			t.Errorf("pack exit = %d, want 1 (overlong name rejected)", code)
		}
	})
}

func TestPakUnpackRejectsHostileNames(t *testing.T) {
	// Craft an archive whose entry name escapes with "../" — things the
	// writer never produces but a third-party archive might.
	var buf bytes.Buffer
	buf.WriteString("PACK")
	_ = binary.Write(&buf, binary.LittleEndian, int32(12+4)) // DirOfs
	_ = binary.Write(&buf, binary.LittleEndian, int32(64))   // DirLen
	buf.Write([]byte("evil"))
	entry := make([]byte, 64)
	copy(entry, "../escape.txt")
	binary.LittleEndian.PutUint32(entry[56:60], 12)
	binary.LittleEndian.PutUint32(entry[60:64], 4)
	buf.Write(entry)

	pakPath := filepath.Join(t.TempDir(), "evil.pak")
	if err := os.WriteFile(pakPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, code := runPakForTest(t, "unpack", pakPath, "-o", t.TempDir()); code != 1 {
		t.Errorf("unpack exit = %d, want 1 (hostile name must be refused)", code)
	}
}

func TestPakCheck(t *testing.T) {
	t.Run("valid archive", func(t *testing.T) {
		src := t.TempDir()
		writeTestTree(t, src)
		pakPath := filepath.Join(t.TempDir(), "pak0.pak")
		if out, code := runPakForTest(t, "pack", src, "-o", pakPath); code != 0 {
			t.Fatalf("pack exit = %d (%s)", code, out)
		}
		out, code := runPakForTest(t, "check", pakPath)
		if code != 0 {
			t.Fatalf("check exit = %d\n%s", code, out)
		}
		if !strings.Contains(out, "OK") || !strings.Contains(out, "4 files") {
			t.Errorf("check output %q lacks OK/4 files", out)
		}
	})
	t.Run("corrupt header", func(t *testing.T) {
		pakPath := filepath.Join(t.TempDir(), "bad.pak")
		_ = os.WriteFile(pakPath, []byte("NOPE............"), 0o644)
		if _, code := runPakForTest(t, "check", pakPath); code != 1 {
			t.Errorf("bad magic: exit = %d, want 1", code)
		}
	})
	t.Run("out of range entry", func(t *testing.T) {
		var buf bytes.Buffer
		buf.WriteString("PACK")
		_ = binary.Write(&buf, binary.LittleEndian, int32(12+4)) // DirOfs
		_ = binary.Write(&buf, binary.LittleEndian, int32(64))   // DirLen
		buf.Write([]byte("evil"))
		entry := make([]byte, 64)
		copy(entry, "a.txt")
		binary.LittleEndian.PutUint32(entry[56:60], 12)
		binary.LittleEndian.PutUint32(entry[60:64], 999999) // FileLen beyond dir
		buf.Write(entry)
		pakPath := filepath.Join(t.TempDir(), "oob.pak")
		if err := os.WriteFile(pakPath, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, code := runPakForTest(t, "check", pakPath)
		if code != 1 {
			t.Errorf("OOB entry: exit = %d, want 1", code)
		}
	})
	t.Run("missing verb", func(t *testing.T) {
		if _, code := runPakForTest(t); code != 2 {
			t.Errorf("no verb: exit = %d, want 2", code)
		}
	})
}

// TestPakBacksTheScaffoldRoundTrips is the packaging half of the mod cycle:
// a scaffolded mod directory packs and unpacks cleanly.
func TestPakBacksTheScaffoldRoundTrips(t *testing.T) {
	dir, _, code := runInitForTest(t, "-kind", "tc")
	if code != 0 {
		t.Fatalf("init exit = %d", code)
	}
	pakPath := filepath.Join(t.TempDir(), "pak0.pak")
	if out, code := runPakForTest(t, "pack", dir, "-o", pakPath); code != 0 {
		t.Fatalf("pack exit = %d (%s)", code, out)
	}
	dest := t.TempDir()
	if out, code := runPakForTest(t, "unpack", pakPath, "-o", dest); code != 0 {
		t.Fatalf("unpack exit = %d (%s)", code, out)
	}
	for _, rel := range []string{"gameconfig.go", "progs/progs.go", "Makefile", "README.md"} {
		if _, err := os.Stat(filepath.Join(dest, rel)); err != nil {
			t.Errorf("round trip lost %s: %v", rel, err)
		}
	}
	// The repacked mod still reads back as a valid fs.Pack via the engine
	// reader (integration with the VFS mount layer).
	data, err := os.ReadFile(pakPath)
	if err != nil {
		t.Fatalf("read pak: %v", err)
	}
	pack, err := fs.LoadPackFromBytes(pakPath, data)
	if err != nil {
		t.Fatalf("LoadPackFromBytes: %v", err)
	}
	if len(pack.Files) != 7 { // go.mod, main.go, gameconfig.go, progs/progs.go, game_test.go, Makefile, README.md
		t.Errorf("pack holds %d files, want 7", len(pack.Files))
	}
}