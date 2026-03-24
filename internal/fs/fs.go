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
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
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
	Priority int
}

type readSeekNopCloser struct {
	io.Reader
	io.Seeker
}

func (readSeekNopCloser) Close() error { return nil }

// SearchPathEntry is a snapshot of one mounted VFS search path entry in
// lookup order, suitable for debug/introspection commands such as `path`.
type SearchPathEntry struct {
	Path      string
	IsPack    bool
	FileCount int
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

// SearchPathEntries returns the current lookup stack in the same front-to-back
// order used by file resolution. Pack entries include their file counts so
// console/debug callers can mirror Quake's `path` command output.
func (fs *FileSystem) SearchPathEntries() []SearchPathEntry {
	entries := make([]SearchPathEntry, 0, len(fs.lookupPaths))
	for _, searchPath := range fs.lookupPaths {
		if searchPath.pack != nil {
			entries = append(entries, SearchPathEntry{
				Path:      searchPath.pack.Filename,
				IsPack:    true,
				FileCount: len(searchPath.pack.Files),
			})
			continue
		}
		entries = append(entries, SearchPathEntry{
			Path: searchPath.root,
		})
	}
	return entries
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
func (fs *FileSystem) FindFile(filename string) (*SearchResult, error) {
	sanitizedName, err := sanitizePath(filename)
	if err != nil {
		return nil, err
	}

	lookupName := canonicalPackLookup(sanitizedName)
	for priority, searchPath := range fs.lookupPaths {
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
					Priority: priority,
				}, nil
			}
			continue
		}

		for _, pf := range searchPath.pack.Files {
			if pf.Lookup == lookupName {
				return &SearchResult{
					Path:     searchPath.pack.Filename,
					Name:     sanitizedName,
					IsPack:   true,
					Pack:     searchPath.pack,
					FilePos:  pf.FilePos,
					FileLen:  pf.FileLen,
					Priority: priority,
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
	return fs.loadSearchResult(result)
}

// OpenFile opens a file from the VFS as a streaming read/seek handle and
// returns the handle plus its byte length.
//
// For loose files this returns an *os.File opened at offset 0.
// For PAK-resident files this returns a section reader over the file's byte
// range in the open archive handle.
func (fs *FileSystem) OpenFile(filename string) (io.ReadSeekCloser, int64, error) {
	result, err := fs.FindFile(filename)
	if err != nil {
		return nil, 0, err
	}
	return fs.openSearchResult(result)
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

	for priority, searchPath := range fs.lookupPaths {
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
						Priority: priority,
					}, nil
				}
			}
			continue
		}

		for _, candidate := range candidates {
			for _, pf := range searchPath.pack.Files {
				if pf.Lookup == candidate.lookup {
					return &SearchResult{
						Path:     searchPath.pack.Filename,
						Name:     candidate.name,
						IsPack:   true,
						Pack:     searchPath.pack,
						FilePos:  pf.FilePos,
						FileLen:  pf.FileLen,
						Priority: priority,
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
	data, err := fs.loadSearchResult(result)
	if err != nil {
		return "", nil, err
	}
	return result.Name, data, nil
}

// LoadMapBSPAndLit loads a BSP plus an eligible .lit sidecar from the same or
// a higher-priority search path. Lower-priority .lit files are ignored so mods
// cannot accidentally recolor a map loaded from a newer override path.
func (fs *FileSystem) LoadMapBSPAndLit(worldModel string) ([]byte, []byte, error) {
	bspResult, err := fs.FindFile(worldModel)
	if err != nil {
		return nil, nil, err
	}
	bspData, err := fs.loadSearchResult(bspResult)
	if err != nil {
		return nil, nil, err
	}

	litName := strings.TrimSuffix(worldModel, filepath.Ext(worldModel)) + ".lit"
	litResult, err := fs.FindFile(litName)
	if err != nil || litResult == nil || litResult.Priority > bspResult.Priority {
		return bspData, nil, nil
	}
	litData, err := fs.loadSearchResult(litResult)
	if err != nil {
		return nil, nil, err
	}
	return bspData, litData, nil
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

func (fs *FileSystem) loadSearchResult(result *SearchResult) ([]byte, error) {
	if result.IsPack {
		return fs.loadFromPack(result)
	}
	return iofs.ReadFile(result.SourceFS, result.Name)
}

func (fs *FileSystem) openSearchResult(result *SearchResult) (io.ReadSeekCloser, int64, error) {
	if result.IsPack {
		reader := io.NewSectionReader(result.Pack.Handle, int64(result.FilePos), int64(result.FileLen))
		return readSeekNopCloser{
			Reader: reader,
			Seeker: reader,
		}, int64(result.FileLen), nil
	}

	file, err := os.Open(result.Path)
	if err != nil {
		return nil, 0, err
	}
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, stat.Size(), nil
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
