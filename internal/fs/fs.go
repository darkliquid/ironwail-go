// Package fs implements the Quake virtual filesystem (VFS).
//
// Quake uses a layered search-path system to locate game assets such as maps,
// textures, models, and sounds. The VFS consists of two kinds of sources:
//
//  1. Loose files on disk — ordinary files inside a game directory (e.g. id1/).
//  2. PAK archives — concatenated binary archives with a central directory.
//
// Search paths are stacked so that later additions override earlier ones. This
// gives "last-added-wins" semantics: a mod directory added after id1/ can
// replace any asset simply by providing a file with the same internal path.
//
// # PAK archive format
//
// A PAK file has a 12-byte header:
//
//	Bytes 0–3:   Magic "PACK" (ASCII)
//	Bytes 4–7:   Offset of the central directory (little-endian int32)
//	Bytes 8–11:  Length of the central directory in bytes (little-endian int32)
//
// The central directory is an array of 64-byte entries:
//
//	Bytes 0–55:  Null-terminated filename (max 56 chars)
//	Bytes 56–59: Offset of the file data within the PAK (little-endian int32)
//	Bytes 60–63: Length of the file data in bytes (little-endian int32)
//
// File data is stored contiguously between the header and the directory.
//
// # Mod loading order
//
// Initialization always adds id1/ first (the base Quake data), then the
// engine's own ironwail.pak, then any user-selected game directory (e.g.
// "hipnotic" or "rogue"). Within each game directory, numbered PAK files
// (pak0.pak, pak1.pak, …) are loaded in ascending numeric order and finally
// the loose-file directory itself is added on top, so loose files always win
// over PAK contents.
//
// The lookupPaths slice is searched front-to-back. Because each
// AddGameDirectory call *prepends* its entries, the most-recently-added
// game directory is consulted first — this is the core of the override
// mechanism that lets mods and patches replace base-game content.
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

// Quake filesystem path limits and well-known filenames.
//
// MaxQPath is the maximum length of a path stored inside a PAK directory entry
// (56 bytes for the name field, but Quake historically enforced a 64-char
// limit on "Quake-path" strings passed through the engine).
//
// MaxOSPath is a generous upper bound for native OS path strings and is used
// for buffer sizing when constructing on-disk paths.
//
// EnginePakName is the filename of the engine-provided resource archive that
// ships alongside the executable. It supplies built-in assets (e.g. default
// configs, charset textures) and is loaded after id1/ but before any mod
// directory so that mods can still override engine defaults.
const (
	MaxQPath      = 64
	MaxOSPath     = 1024
	EnginePakName = "ironwail.pak"
)

// PackFile represents a single file entry inside a PAK archive.
//
// Name is the original path as stored in the PAK directory (e.g.
// "maps/e1m1.bsp"). Lookup is the case-folded, slash-normalised variant used
// for case-insensitive matching — Quake's original filesystem was
// case-insensitive on DOS/Windows, and we preserve that behaviour. FilePos and
// FileLen describe the byte range within the PAK's data section.
type PackFile struct {
	Name    string
	Lookup  string
	FilePos int32
	FileLen int32
}

// Pack represents an open PAK archive.
//
// The Handle is kept open for the lifetime of the filesystem so that
// individual files can be read on demand without re-opening the archive.
// Filename is the on-disk path to the PAK; Files is the parsed central
// directory — a flat list of PackFile entries that is scanned linearly
// during file lookup (acceptable because PAK files rarely exceed a few
// hundred entries).
type Pack struct {
	Filename string
	Handle   *os.File
	Files    []PackFile
}

// SearchResult describes where a requested file was found within the VFS.
//
// If IsPack is false the file lives on disk and SourceFS + Name can be used
// with the standard io/fs package to read it (Path gives the full OS path).
// If IsPack is true the file lives inside a PAK archive: Pack identifies the
// archive, and FilePos/FileLen give the byte window to read from the Pack's
// open Handle.
type SearchResult struct {
	Path     string
	Name     string
	SourceFS iofs.FS
	IsPack   bool
	Pack     *Pack
	FilePos  int32
	FileLen  int32
}

// searchPath is a single entry in the VFS search stack.
//
// Exactly one of (fs, pack) is non-nil for any given entry:
//   - If pack is non-nil this entry represents a PAK archive.
//   - Otherwise root + fs represent a loose-file directory on disk.
//
// The lookupPaths slice is ordered so that the *most recently added* entries
// come first (prepend semantics), giving "last-added-wins" override behaviour.
type searchPath struct {
	root string
	fs   iofs.FS
	pack *Pack
}

// FileSystem is the central Quake VFS manager.
//
// searchPaths holds loose-file directories only and is used by ListFiles for
// glob-based enumeration. lookupPaths is the unified lookup stack containing
// both PAK entries and loose-file entries in priority order — this is what
// FindFile walks. packs tracks every open PAK so that Close can release their
// file handles. gameDir is the currently active mod directory name (e.g.
// "hipnotic"), and baseDir is the root installation path containing id1/ and
// other game directories.
type FileSystem struct {
	searchPaths []searchPath
	lookupPaths []searchPath
	packs       []*Pack
	gameDir     string
	baseDir     string
	initialized bool
}

// NewFileSystem creates an empty, uninitialised VFS. Call Init to populate the
// search paths with the base game directory and any selected mod.
func NewFileSystem() *FileSystem {
	return &FileSystem{
		searchPaths: make([]searchPath, 0),
		lookupPaths: make([]searchPath, 0),
		packs:       make([]*Pack, 0),
	}
}

// Init sets up the VFS search paths following Quake's conventional mod
// loading order:
//
//  1. Add id1/ (the base Quake data, always required).
//  2. Add ironwail.pak (engine-provided overrides, found next to the
//     executable or in basedir).
//  3. If a mod gamedir is specified (and is not "id1"), add that directory.
//
// Because AddGameDirectory prepends its entries into lookupPaths, the mod
// directory ends up being searched first, then ironwail.pak, then id1/ —
// exactly the override order a player would expect.
//
// Init is idempotent; calling it a second time is a no-op.
func (fs *FileSystem) Init(basedir, gamedir string) error {
	if fs.initialized {
		return nil
	}
	fs.baseDir = basedir
	fs.gameDir = gamedir

	// Base directory is the installation directory, we must always add 'id1'
	// as the fundamental Quake directory, then add any custom game directory.

	if err := fs.AddGameDirectory(filepath.Join(basedir, "id1")); err != nil {
		return fmt.Errorf("failed to add id1 directory: %w", err)
	}
	fs.addEnginePak()

	if gamedir != "" && gamedir != "id1" {
		if err := fs.AddGameDirectory(filepath.Join(basedir, gamedir)); err != nil {
			return fmt.Errorf("failed to add game directory: %w", err)
		}
	}

	fs.initialized = true
	return nil
}

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
func (fs *FileSystem) FindFile(filename string) (*SearchResult, error) {
	sanitizedName, err := sanitizePath(filename)
	if err != nil {
		return nil, err
	}

	lookupName := canonicalPackLookup(sanitizedName)
	for _, searchPath := range fs.lookupPaths {
		if searchPath.pack == nil {
			fullPath := filepath.Join(searchPath.root, filepath.FromSlash(sanitizedName))

			if !isWithinRoot(searchPath.root, fullPath) {
				return nil, fmt.Errorf("invalid path traversal attempt: %s", filename)
			}

			if stat, err := iofs.Stat(searchPath.fs, sanitizedName); err == nil && !stat.IsDir() {
				return &SearchResult{
					Path:     fullPath,
					Name:     sanitizedName,
					SourceFS: searchPath.fs,
					IsPack:   false,
				}, nil
			}
			continue
		}

		for _, pf := range searchPath.pack.Files {
			if pf.Lookup == lookupName {
				return &SearchResult{
					Path:    searchPath.pack.Filename,
					Name:    sanitizedName,
					IsPack:  true,
					Pack:    searchPath.pack,
					FilePos: pf.FilePos,
					FileLen: pf.FileLen,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("file not found: %s", sanitizedName)
}

// LoadFile is a convenience method that finds a file in the VFS and reads its
// entire contents into memory. For PAK-resident files this seeks to the
// stored offset and reads exactly FileLen bytes; for loose files it delegates
// to the standard io/fs.ReadFile.
func (fs *FileSystem) LoadFile(filename string) ([]byte, error) {
	result, err := fs.FindFile(filename)
	if err != nil {
		return nil, err
	}

	if result.IsPack {
		return fs.loadFromPack(result)
	}

	return iofs.ReadFile(result.SourceFS, result.Name)
}

// FindFirstAvailable attempts to locate the first file that exists from a
// prioritised list of candidate filenames. This is used when the engine can
// accept multiple asset variants — for example trying "maps/e1m1.bsp" first
// and falling back to "maps/e1m1.lit".
//
// For each search-path entry the method tests *all* candidates before moving
// to the next entry, so a candidate that appears earlier in the list is
// preferred only within the same search-path priority level; a later
// candidate in a higher-priority path still wins over an earlier candidate
// in a lower-priority path.
func (fs *FileSystem) FindFirstAvailable(filenames []string) (*SearchResult, error) {
	if len(filenames) == 0 {
		return nil, fmt.Errorf("no filenames provided")
	}

	type candidate struct {
		name   string
		lookup string
	}
	candidates := make([]candidate, 0, len(filenames))
	for _, filename := range filenames {
		sanitizedName, err := sanitizePath(filename)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate{
			name:   sanitizedName,
			lookup: canonicalPackLookup(sanitizedName),
		})
	}

	for _, searchPath := range fs.lookupPaths {
		if searchPath.pack == nil {
			for _, candidate := range candidates {
				fullPath := filepath.Join(searchPath.root, filepath.FromSlash(candidate.name))
				if !isWithinRoot(searchPath.root, fullPath) {
					return nil, fmt.Errorf("invalid path traversal attempt: %s", candidate.name)
				}
				if stat, err := iofs.Stat(searchPath.fs, candidate.name); err == nil && !stat.IsDir() {
					return &SearchResult{
						Path:     fullPath,
						Name:     candidate.name,
						SourceFS: searchPath.fs,
						IsPack:   false,
					}, nil
				}
			}
			continue
		}

		for _, candidate := range candidates {
			for _, pf := range searchPath.pack.Files {
				if pf.Lookup == candidate.lookup {
					return &SearchResult{
						Path:    searchPath.pack.Filename,
						Name:    candidate.name,
						IsPack:  true,
						Pack:    searchPath.pack,
						FilePos: pf.FilePos,
						FileLen: pf.FileLen,
					}, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("none of the files were found: %s", strings.Join(filenames, ", "))
}

// LoadFirstAvailable combines FindFirstAvailable with a full file read. It
// returns the name of the file that was found, its contents, and any error.
// This is useful when the caller needs to know *which* variant was loaded
// (e.g. to select the appropriate parser).
func (fs *FileSystem) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	result, err := fs.FindFirstAvailable(filenames)
	if err != nil {
		return "", nil, err
	}

	if result.IsPack {
		data, err := fs.loadFromPack(result)
		if err != nil {
			return "", nil, err
		}
		return result.Name, data, nil
	}

	data, err := iofs.ReadFile(result.SourceFS, result.Name)
	if err != nil {
		return "", nil, err
	}
	return result.Name, data, nil
}

// loadFromPack reads a file's raw bytes from an open PAK archive. It seeks to
// the byte offset recorded in the SearchResult and reads exactly FileLen bytes
// using io.ReadFull, which guarantees we get the complete file or an error.
func (fs *FileSystem) loadFromPack(result *SearchResult) ([]byte, error) {
	if _, err := result.Pack.Handle.Seek(int64(result.FilePos), io.SeekStart); err != nil {
		return nil, err
	}

	data := make([]byte, result.FileLen)
	if _, err := io.ReadFull(result.Pack.Handle, data); err != nil {
		return nil, err
	}

	return data, nil
}

// FileExists returns true if the named file can be found anywhere in the VFS
// (either as a loose file or inside a PAK). It is a thin wrapper around
// FindFile that discards the SearchResult.
func (fs *FileSystem) FileExists(filename string) bool {
	_, err := fs.FindFile(filename)
	return err == nil
}

// GetGameDir returns the name of the currently active mod directory (e.g.
// "hipnotic"), or "" if no mod is loaded (i.e. running base id1).
func (fs *FileSystem) GetGameDir() string {
	return fs.gameDir
}

// GetBaseDir returns the root installation path that contains id1/ and any
// other game directories. All relative game paths are resolved against this.
func (fs *FileSystem) GetBaseDir() string {
	return fs.baseDir
}

// ModInfo describes a discovered mod directory under the base Quake directory.
type ModInfo struct {
	// Name is the directory name (e.g. "hipnotic", "rogue", "mymod").
	Name string
}

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
func (fs *FileSystem) Close() {
	for _, pack := range fs.packs {
		if pack.Handle != nil {
			pack.Handle.Close()
		}
	}
}

// SkipPath returns the filename component of a path by stripping everything
// up to and including the last '/'. This is the Quake equivalent of
// filepath.Base but operates only on forward slashes (Quake-paths are always
// slash-separated regardless of the host OS).
func SkipPath(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx >= 0 {
		return path[idx+1:]
	}
	return path
}

// StripExtension removes the file extension (including the dot) from a path.
// It correctly handles paths like "maps/e1m1.bsp" → "maps/e1m1" and avoids
// stripping dots that appear in directory components (e.g. "../file" is
// returned unchanged).
func StripExtension(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx > strings.LastIndex(path, "/") {
		return path[:idx]
	}
	return path
}

// GetExtension returns the file extension without the leading dot (e.g. "bsp"
// for "maps/e1m1.bsp"). Returns "" if there is no extension or if the last
// dot belongs to a directory component.
func GetExtension(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx > strings.LastIndex(path, "/") && idx < len(path)-1 {
		return path[idx+1:]
	}
	return ""
}

// AddExtension appends ext to path only if the path has no extension already.
// ext should include the leading dot (e.g. ".bsp"). If the path already has
// an extension it is returned unchanged. This matches the behaviour of the
// original C COM_AddExtension function.
func AddExtension(path, ext string) string {
	if GetExtension(path) == "" {
		return path + ext
	}
	return path
}

// DefaultExtension is a semantic alias for AddExtension — it appends ext to
// path if no extension is present. In the original Quake source these were
// separate functions (COM_DefaultExtension vs COM_AddExtension) but they have
// identical logic.
func DefaultExtension(path, ext string) string {
	if GetExtension(path) == "" {
		return path + ext
	}
	return path
}

// FileBase extracts the "base name" of a file — the filename without its
// directory prefix or extension. For example "maps/e1m1.bsp" → "e1m1".
// This combines SkipPath and StripExtension.
func FileBase(path string) string {
	path = SkipPath(path)
	return StripExtension(path)
}

// CreatePath ensures that the directory hierarchy leading to path exists,
// creating intermediate directories as needed (mode 0755). This is used when
// writing files to the user's game directory — e.g. saving screenshots or
// config files — where the target subdirectory may not exist yet.
func CreatePath(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// sanitizePath normalises a user-provided filename into a safe, relative,
// forward-slash-separated path suitable for VFS lookup. It rejects absolute
// paths and any ".." traversal attempts to prevent directory-escape attacks
// (a real concern when file paths come from network messages or mod scripts).
func sanitizePath(filename string) (string, error) {
	normalized := strings.ReplaceAll(filename, "\\", "/")
	cleaned := filepath.ToSlash(filepath.Clean(filepath.FromSlash(normalized)))

	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("invalid empty path")
	}

	if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("absolute paths are not allowed: %s", filename)
	}

	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("invalid path traversal attempt: %s", filename)
	}

	return cleaned, nil
}

// isWithinRoot verifies that a resolved target path is a descendant of root.
// This is the second layer of defence against path-traversal attacks — even
// if sanitizePath missed something, this check prevents reading files outside
// the game directory by computing the relative path and ensuring it doesn't
// start with "..".
func isWithinRoot(root, target string) bool {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil {
		return false
	}

	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

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
