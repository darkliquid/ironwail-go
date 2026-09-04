package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/fs"
)

// runPak implements `qcmod pak <verb> [flags]` — creating, extracting,
// listing, and validating Quake .pak archives (bead ironwail-go-uxr AC2).
func runPak(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "qcmod pak: missing verb (pack|unpack|list|check)")
		return 2
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "pack":
		return runPakPack(rest, stdout, stderr)
	case "unpack", "extract":
		return runPakUnpack(rest, stdout, stderr)
	case "list", "ls":
		return runPakList(rest, stdout, stderr)
	case "check", "validate":
		return runPakCheck(rest, stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "qcmod pak: unknown verb %q (pack|unpack|list|check)\n", verb)
		return 2
	}
}

// pakOutFlag extracts the -o/--out/--output destination from args (either
// "-o value" or "-o=value"), removing it from the returned positional list.
// Supported alongside positionals in any order, like the engine's own CLI
// flag interleaving (fix(cli) 2026-09).
func pakOutFlag(args []string, def string, stderr io.Writer) (out string, pos []string, ok bool) {
	out = def
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-o" || a == "--out" || a == "--output":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintf(stderr, "qcmod pak: flag %s requires a value\n", a)
				return "", nil, false
			}
			out = args[i+1]
			i++
		case strings.HasPrefix(a, "-o="):
			out = strings.TrimPrefix(a, "-o=")
		case strings.HasPrefix(a, "-"):
			_, _ = fmt.Fprintf(stderr, "qcmod pak: unknown flag %q\n", a)
			return "", nil, false
		default:
			pos = append(pos, a)
		}
	}
	return out, pos, true
}

// runPakPack implements `qcmod pak pack <dir> -o <out.pak>`: walks dir and
// stores every file under its slash-normalised relative path.
func runPakPack(args []string, stdout, stderr io.Writer) int {
	out, pos, ok := pakOutFlag(args, "pak0.pak", stderr)
	if !ok {
		return 2
	}
	if len(pos) != 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod pak pack: exactly one directory is required")
		return 2
	}
	srcDir := pos[0]

	var entries []fs.PakEntry
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if err := fs.ValidPakName(name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, fs.PakEntry{Name: name, Data: data})
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak pack: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak pack: mkdir: %v\n", err)
		return 1
	}
	f, err := os.Create(out)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak pack: create %s: %v\n", out, err)
		return 1
	}
	if err := fs.WritePack(f, entries); err != nil {
		_ = f.Close()
		_, _ = fmt.Fprintf(stderr, "qcmod pak pack: %v\n", err)
		return 1
	}
	if err := f.Close(); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak pack: close %s: %v\n", out, err)
		return 1
	}

	var total int
	for _, e := range entries {
		total += len(e.Data)
	}
	_, _ = fmt.Fprintf(stdout, "Packed %d files (%d bytes) -> %s\n", len(entries), total, out)
	return 0
}

// runPakUnpack implements `qcmod pak unpack <pak> -o <dir>`: extracts every
// entry, re-validating names so malicious archives cannot escape the
// destination directory.
func runPakUnpack(args []string, stdout, stderr io.Writer) int {
	out, pos, ok := pakOutFlag(args, ".", stderr)
	if !ok {
		return 2
	}
	if len(pos) != 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod pak unpack: exactly one archive path is required")
		return 2
	}
	pakPath := pos[0]

	data, err := os.ReadFile(pakPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak unpack: read %s: %v\n", pakPath, err)
		return 1
	}
	pack, err := fs.LoadPackFromBytes(pakPath, data)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak unpack: %v\n", err)
		return 1
	}
	pakFS := fs.NewPakFS(pack)

	for _, f := range pack.Files {
		// Defense in depth: archives from third parties may violate the
		// name rules even though our writer refuses to create them.
		if err := fs.ValidPakName(f.Name); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod pak unpack: refusing %q: %v\n", f.Name, err)
			return 1
		}
		content, err := pakFS.ReadFile(f.Name)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod pak unpack: read %q: %v\n", f.Name, err)
			return 1
		}
		dest := filepath.Join(out, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod pak unpack: mkdir: %v\n", err)
			return 1
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod pak unpack: write %s: %v\n", dest, err)
			return 1
		}
	}
	_, _ = fmt.Fprintf(stdout, "Extracted %d files -> %s\n", len(pack.Files), out)
	return 0
}

// runPakList implements `qcmod pak list <pak>`: prints each entry as
// "<size> <name>", sorted for stable output.
func runPakList(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod pak list: exactly one archive path is required")
		return 2
	}
	pakPath := args[0]
	pack, err := loadPackFile(pakPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak list: %v\n", err)
		return 1
	}
	entries := append([]fs.PackFile(nil), pack.Files...)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Lookup < entries[j].Lookup })
	for _, f := range entries {
		_, _ = fmt.Fprintf(stdout, "%9d  %s\n", f.FileLen, f.Name)
	}
	return 0
}

// runPakCheck implements `qcmod pak check <pak>`: verifies the directory
// table (magic, alignment, uniqueness, byte ranges) so broken or hostile
// archives are caught before mounting.
func runPakCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod pak check: exactly one archive path is required")
		return 2
	}
	pakPath := args[0]
	data, err := os.ReadFile(pakPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak check: read %s: %v\n", pakPath, err)
		return 1
	}
	if len(data) < 12 {
		_, _ = fmt.Fprintf(stderr, "qcmod pak check: %s is too short to be a pak archive\n", pakPath)
		return 1
	}
	var header struct {
		ID     [4]byte
		DirOfs int32
		DirLen int32
	}
	if err := binary.Read(bytes.NewReader(data[:12]), binary.LittleEndian, &header); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod pak check: read header: %v\n", err)
		return 1
	}
	if string(header.ID[:]) != "PACK" {
		_, _ = fmt.Fprintf(stderr, "qcmod pak check: %s is not a pak archive (bad magic)\n", pakPath)
		return 1
	}

	problems := 0
	report := func(format string, a ...any) {
		problems++
		_, _ = fmt.Fprintf(stderr, "qcmod pak check: %s\n", fmt.Sprintf(format, a...))
	}

	if header.DirOfs < 12 || int(header.DirOfs) > len(data) {
		report("directory offset %d out of range (file %d bytes)", header.DirOfs, len(data))
	}
	if header.DirLen < 0 {
		report("negative directory length %d", header.DirLen)
	} else if header.DirLen%64 != 0 {
		report("directory length %d is not a multiple of 64", header.DirLen)
	}
	dataEnd := int(header.DirOfs)
	if dataEnd > len(data) {
		dataEnd = len(data)
	}

	var pack *fs.Pack
	pack, err = fs.LoadPackFromBytes(pakPath, data)
	if err != nil {
		report("directory table unreadable: %v", err)
	} else {
		seen := make(map[string]struct{}, len(pack.Files))
		for _, f := range pack.Files {
			if err := fs.ValidPakName(f.Name); err != nil {
				report("invalid entry name %q: %v", f.Name, err)
			}
			if _, dup := seen[f.Lookup]; dup {
				report("duplicate entry %q (case-insensitive)", f.Name)
			}
			seen[f.Lookup] = struct{}{}
			if f.FileLen < 0 {
				report("entry %q has negative length %d", f.Name, f.FileLen)
				continue
			}
			if f.FilePos < 12 || int64(f.FilePos)+int64(f.FileLen) > int64(dataEnd) {
				report("entry %q data [%d, %d) out of range (data ends at %d)", f.Name, f.FilePos, f.FilePos+f.FileLen, dataEnd)
			}
		}
	}

	if header.DirLen >= 0 && int(header.DirOfs)+int(header.DirLen) > len(data) {
		report("directory table extends past end of file")
	}

	if problems > 0 {
		_, _ = fmt.Fprintf(stderr, "qcmod pak check: %s: %d problem(s)\n", pakPath, problems)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "OK: %s: %d files, %d bytes, directory at %d\n", pakPath, len(pack.Files), len(data), header.DirOfs)
	return 0
}

func loadPackFile(pakPath string) (*fs.Pack, error) {
	data, err := os.ReadFile(pakPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", pakPath, err)
	}
	return fs.LoadPackFromBytes(pakPath, data)
}