// Package fs implements the Quake virtual filesystem (VFS).
//
// This file implements search-path and PAK discovery/loading logic.
package fs

import (
	"encoding/binary"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// addEnginePak locates and loads the engine-provided PAK archive
// (ironwail.pak). It checks up to two candidate directories — the directory
// containing the executable and the basedir — deduplicating them if they
// overlap. The engine PAK is prepended into lookupPaths so that its assets
// sit *above* id1/ but *below* any subsequently-added mod directory.
func (fs *FileSystem) addEnginePak() {
	candidates := make([]string, 0, 2)
	seen := make(map[string]struct{}, 2)

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Clean(filepath.Dir(exePath))
		if _, exists := seen[exeDir]; !exists {
			candidates = append(candidates, exeDir)
			seen[exeDir] = struct{}{}
		}
	}

	if fs.baseDir != "" {
		baseDir := filepath.Clean(fs.baseDir)
		if _, exists := seen[baseDir]; !exists {
			candidates = append(candidates, baseDir)
		}
	}

	for _, dir := range candidates {
		enginePakPath := filepath.Join(dir, EnginePakName)
		if _, err := os.Stat(enginePakPath); err != nil {
			continue
		}

		pack, err := fs.loadPack(enginePakPath)
		if err != nil {
			fmt.Printf("Warning: failed to load engine pak %s: %v\n", enginePakPath, err)
			continue
		}

		fs.packs = append(fs.packs, pack)
		fs.lookupPaths = append([]searchPath{{pack: pack}}, fs.lookupPaths...)
		fmt.Printf("Added engine pak: %s (%d files)\n", EnginePakName, len(pack.Files))
		return
	}
}

// AddGameDirectory registers a game directory (e.g. "id1/" or "hipnotic/")
// with the VFS. The process mirrors the original Quake engine:
//
//  1. The directory itself is added to searchPaths as a loose-file source.
//  2. Any numbered PAK files (pak0.pak, pak1.pak, …) found in the directory
//     are opened and their entries added to the lookup stack. PAK files are
//     loaded in ascending numeric order so that higher-numbered paks override
//     lower ones (pak1.pak beats pak0.pak).
//  3. The loose-file directory is placed at the *top* of the lookup group so
//     that individual files on disk override anything in a PAK — this is how
//     development overrides and runtime downloads work.
//
// The entire group (loose + PAKs) is prepended to lookupPaths, ensuring that
// a game directory added later overrides all earlier ones.
func (fs *FileSystem) AddGameDirectory(dir string) error {
	cleanDir := filepath.Clean(dir)
	loosePath := searchPath{
		root: cleanDir,
		fs:   os.DirFS(cleanDir),
	}
	fs.searchPaths = append(fs.searchPaths, loosePath)

	pakFiles, err := discoverPakFiles(cleanDir)
	if err != nil {
		return err
	}

	lookupGroup := make([]searchPath, 0, len(pakFiles)+1)
	for _, pakFile := range pakFiles {
		pack, err := fs.loadPack(pakFile)
		if err != nil {
			fmt.Printf("Warning: failed to load pack %s: %v\n", pakFile, err)
			continue
		}
		fs.packs = append(fs.packs, pack)
		lookupGroup = append([]searchPath{{pack: pack}}, lookupGroup...)
	}
	lookupGroup = append(lookupGroup, loosePath)
	fs.lookupPaths = append(lookupGroup, fs.lookupPaths...)

	return nil
}

// loadPack opens a PAK file, validates its 12-byte header, and reads the
// central directory into memory.
//
// The PAK format is:
//
//	Header (12 bytes):  [4]byte "PACK" | int32 DirOfs | int32 DirLen
//	Directory entries:  Each 64 bytes — [56]byte Name | int32 FilePos | int32 FileLen
//	File data:          Raw bytes referenced by FilePos/FileLen pairs.
//
// The number of entries is DirLen / 64. Each entry's Name is a null-terminated
// C string occupying up to 56 bytes; we trim the trailing zeros to produce a
// clean Go string. A canonical lowercase/slash-normalised "Lookup" key is also
// computed for case-insensitive searching.
//
// The underlying os.File is intentionally kept open (stored in Pack.Handle)
// so that file data can be read on demand later without reopening the archive.
func (fs *FileSystem) loadPack(filename string) (*Pack, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	var header struct {
		ID     [4]byte
		DirOfs int32
		DirLen int32
	}

	if err := binary.Read(file, binary.LittleEndian, &header); err != nil {
		file.Close()
		return nil, err
	}

	if string(header.ID[:]) != "PACK" {
		file.Close()
		return nil, fmt.Errorf("not a pack file")
	}

	numFiles := int(header.DirLen / 64)

	if _, err := file.Seek(int64(header.DirOfs), io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}

	files := make([]PackFile, numFiles)
	for i := 0; i < numFiles; i++ {
		var entry struct {
			Name    [56]byte
			FilePos int32
			FileLen int32
		}
		if err := binary.Read(file, binary.LittleEndian, &entry); err != nil {
			file.Close()
			return nil, err
		}
		idx := 0
		for idx < len(entry.Name) && entry.Name[idx] != 0 {
			idx++
		}
		files[i] = PackFile{
			Name:    string(entry.Name[:idx]),
			Lookup:  canonicalPackLookup(string(entry.Name[:idx])),
			FilePos: entry.FilePos,
			FileLen: entry.FileLen,
		}
	}

	return &Pack{
		Filename: filename,
		Handle:   file,
		Files:    files,
	}, nil
}

// FindFile searches the VFS for the given filename and returns a SearchResult
// describing where the file was found.
//
// The search walks lookupPaths front-to-back (most-recently-added first).
// For each entry:
//   - Loose-file entries: stat the file on disk, guarding against directory
//     traversal (the path must stay within the search root).
//   - PAK entries: linearly scan the Pack's file list for a case-insensitive
//     match against the canonical lookup key.
//
// The first match wins — this is the core of the "last-added-wins" override
// system, because AddGameDirectory prepends its entries.

// ListMods returns all valid mod directories found in the basedir.
// A directory is considered a valid mod if it contains at least one pak file
// or a progs.dat file, following the same convention as C Ironwail's mod list.
// The well-known base directory "id1" is excluded because it is not a
// user-selectable mod; other directories are returned in sorted order.
func (fs *FileSystem) ListMods() []ModInfo {
	if fs.baseDir == "" {
		return nil
	}
	entries, err := os.ReadDir(fs.baseDir)
	if err != nil {
		return nil
	}

	var mods []ModInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, "id1") {
			continue
		}
		dirPath := filepath.Join(fs.baseDir, name)
		if isValidModDir(dirPath) {
			mods = append(mods, ModInfo{Name: name})
		}
	}
	return mods
}

// isValidModDir returns true if dir contains a pak file or a progs.dat.
func isValidModDir(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, "progs.dat") {
			return true
		}
		if pakFilePattern.MatchString(name) {
			return true
		}
	}
	return false
}

// ListFiles returns the paths of all files matching a glob pattern across
// every registered search path. It searches both loose-file directories
// (using io/fs.Glob) and PAK archives (using filepath.Match against each
// entry name). The results are not deduplicated — the same logical file may
// appear more than once if it exists in multiple search paths.
func (fs *FileSystem) ListFiles(pattern string) []string {
	var results []string

	for _, searchPath := range fs.searchPaths {
		matches, err := iofs.Glob(searchPath.fs, pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			results = append(results, filepath.ToSlash(match))
		}
	}

	for _, pack := range fs.packs {
		for _, pf := range pack.Files {
			matched, err := filepath.Match(pattern, pf.Name)
			if err == nil && matched {
				results = append(results, pf.Name)
			}
		}
	}

	return results
}

// Close releases all resources held by the VFS. Every open PAK file handle is
// closed. After Close the FileSystem must not be used.

// pakFilePattern matches Quake's conventional numbered PAK filenames:
// pak0.pak, pak1.pak, … The capture group extracts the numeric index used
// for sorting so that pak0 is loaded before pak1, etc. The match is
// case-insensitive to handle Windows-style naming.
var pakFilePattern = regexp.MustCompile(`(?i)^pak([0-9]+)\.pak$`)

// discoverPakFiles scans a directory for numbered PAK files and returns their
// full paths sorted in ascending numeric order (pak0, pak1, pak2, …).
// Non-existent directories are silently accepted (return nil, nil) to allow
// game directories that contain only loose files. Files whose names don't
// match pakN.pak are ignored, as are non-numeric names.
func discoverPakFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type pakInfo struct {
		path  string
		index int
	}
	var pakFiles []pakInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := pakFilePattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}
		idx, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}
		pakFiles = append(pakFiles, pakInfo{
			path:  filepath.Join(dir, entry.Name()),
			index: idx,
		})
	}

	sort.Slice(pakFiles, func(i, j int) bool {
		if pakFiles[i].index != pakFiles[j].index {
			return pakFiles[i].index < pakFiles[j].index
		}
		return strings.ToLower(filepath.Base(pakFiles[i].path)) < strings.ToLower(filepath.Base(pakFiles[j].path))
	})

	paths := make([]string, len(pakFiles))
	for i := range pakFiles {
		paths[i] = pakFiles[i].path
	}
	return paths, nil
}

// canonicalPackLookup normalises a filename for case-insensitive, separator-
// insensitive comparison. All backslashes are replaced with forward slashes
// and the entire string is lowercased. This ensures that "Maps\E1M1.BSP" and
// "maps/e1m1.bsp" produce the same lookup key, matching the behaviour of
// Quake's original DOS filesystem which was case-insensitive.
func canonicalPackLookup(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
}
