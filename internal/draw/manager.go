package draw

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/image"
)

// Manager handles loading and caching of 2D graphics assets from WAD files.
// It loads gfx.wad and caches QPic objects for rendering.
//
// In Quake-engine architecture, all flat/HUD/menu rendering is handled separately
// from 3D world rendering. After the 3D scene is drawn each frame, 2D elements
// (console text, the status bar, menus, crosshairs, etc.) are composited on top
// as textured quads in screen space. The Manager serves as the asset layer for
// this 2D pipeline: it owns the WAD archive, resolves asset names through
// multiple fallback strategies (WAD lump → pak file → loose file), and caches
// parsed QPic images so repeated draws don't re-parse binary data every frame.
//
// The Manager is intentionally decoupled from the GPU. It deals only with
// CPU-side pixel data (palette-indexed byte slices). The actual texture upload
// and quad rendering happen in higher-level rendering packages that consume
// the QPic/conchars data this Manager provides.
type Manager struct {
	mu sync.RWMutex

	// wad is the loaded WAD file containing all graphics assets.
	wad *image.Wad

	// filesys is the game filesystem, used to load standalone pic files
	// (e.g. gfx/qplaque.lmp stored directly in pak, not inside gfx.wad).
	filesys *fs.FileSystem

	// baseDir is set when initialized from a directory (fallback path).
	baseDir string

	// pics is a cache of parsed QPic objects indexed by lump name.
	pics map[string]*image.QPic

	// palette is the Quake color palette for color translation.
	palette []byte

	// initialized indicates whether the manager has been initialized.
	initialized bool
}

// NewManager creates a new draw manager with an empty picture cache.
//
// The manager starts in an uninitialized state — callers must follow up with
// Init (for normal game startup via the virtual filesystem) or InitFromDir
// (for tests and tools that load assets from a plain directory). Until one of
// those methods succeeds, all asset lookups return nil.
func NewManager() *Manager {
	return &Manager{
		pics: make(map[string]*image.QPic),
	}
}

// Init loads the WAD file and initializes the asset cache.
// It searches for gfx.wad in the provided filesystem.
//
// This is the primary initialization path used during normal engine startup.
// The filesystem abstraction (fs.FileSystem) may search across multiple pak
// files and loose directories, mirroring Quake's cascading search path
// (id1/pak0.pak → id1/pak1.pak → mod directory → etc.).
//
// The Quake color palette (palette.lmp) is loaded here because every
// palette-indexed asset in the engine depends on it. The palette is 768 bytes:
// 256 colors × 3 bytes (R, G, B). It is shared engine-wide for converting
// 8-bit pixel indices into displayable RGB colors.
func (m *Manager) Init(filesys *fs.FileSystem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	slog.Info("Initializing draw manager")

	// Try to load gfx.wad from the filesystem
	wadData, err := filesys.LoadFile("gfx.wad")
	if err != nil {
		return fmt.Errorf("failed to load gfx.wad: %w", err)
	}

	wad, err := image.LoadWad(bytes.NewReader(wadData))
	if err != nil {
		return fmt.Errorf("failed to parse gfx.wad: %w", err)
	}
	m.wad = wad
	m.filesys = filesys

	// Load the palette: first try gfx/palette.lmp from the filesystem (real Quake data),
	// then fall back to palette.lmp lump inside gfx.wad (test data).
	palData, err := filesys.LoadFile("gfx/palette.lmp")
	if err != nil || len(palData) < 768 {
		paletteLump, ok := wad.Lumps["palette.lmp"]
		if !ok || len(paletteLump.Data) < 768 {
			return fmt.Errorf("failed to find palette (tried gfx/palette.lmp and gfx.wad)")
		}
		palData = paletteLump.Data
	}
	m.palette = palData[:768]

	slog.Info("Draw manager initialized", "lumps", len(wad.Lumps), "palette_colors", 256)

	m.initialized = true
	return nil
}

// InitFromDir loads WAD files from a specific directory.
// This is useful for testing or loading from a non-standard location.
//
// Unlike Init, this bypasses the virtual filesystem entirely and reads
// directly from the OS filesystem. It expects gfx.wad (and palette.lmp
// within it) to exist in baseDir. This path is primarily used by unit tests
// and command-line tools that need to inspect Quake assets without
// bootstrapping the full engine search path.
func (m *Manager) InitFromDir(baseDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	slog.Info("Initializing draw manager from directory", "dir", baseDir)

	// Load gfx.wad from the directory
	wadPath := filepath.Join(baseDir, "gfx.wad")
	wadData, err := os.ReadFile(wadPath)
	if err != nil {
		return fmt.Errorf("failed to load gfx.wad from %s: %w", wadPath, err)
	}

	wad, err := image.LoadWad(bytes.NewReader(wadData))
	if err != nil {
		return fmt.Errorf("failed to parse gfx.wad: %w", err)
	}
	m.wad = wad
	m.baseDir = baseDir

	// Load the palette from gfx.wad
	paletteLump, ok := wad.Lumps["palette.lmp"]
	if !ok || len(paletteLump.Data) < 768 {
		return fmt.Errorf("failed to find or parse palette.lmp in gfx.wad")
	}

	m.palette = paletteLump.Data[:768]

	slog.Info("Draw manager initialized", "lumps", len(wad.Lumps), "palette_colors", 256)

	m.initialized = true
	return nil
}

// GetPic retrieves a QPic by name, loading and caching it if necessary.
// Returns nil if the pic cannot be found.
//
// This is the main entry point that higher-level rendering code calls every
// frame when it needs a 2D image (e.g., Draw_Pic("gfx/pause.lmp") to show
// the pause screen overlay). The multi-step resolution order mirrors Quake's
// original asset lookup behavior, where assets could live inside WAD archives,
// inside pak files, or as loose files in the game directory.
//
// The cache ensures that repeated lookups (which happen every frame for HUD
// elements) are O(1) map lookups rather than repeated binary parsing.
// Thread safety is provided via a read-write mutex: the fast path (cache hit)
// only acquires a read lock, while the slow path (cache miss → load) upgrades
// to a write lock with double-checked locking to avoid duplicate loads.
//
// Resolution order:
//  1. Cache
//  2. gfx.wad lump by full name (e.g. "gfx/qplaque.lmp")
//  3. gfx.wad lump by bare name (e.g. "qplaque") — real Quake gfx.wad omits paths/extensions
//  4. Standalone file from the pak filesystem (e.g. gfx/qplaque.lmp lives directly in pak0.pak)
//  5. Standalone file from the base directory (InitFromDir fallback)
func (m *Manager) GetPic(name string) *image.QPic {
	m.mu.RLock()
	if !m.initialized {
		m.mu.RUnlock()
		return nil
	}

	// Check cache first
	if pic, ok := m.pics[name]; ok {
		m.mu.RUnlock()
		return pic
	}
	m.mu.RUnlock()

	// Need to load — upgrade to write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check cache
	if pic, ok := m.pics[name]; ok {
		return pic
	}

	pic := m.loadPic(name)
	if pic != nil {
		m.pics[name] = pic
	}
	return pic
}

// IsPicCached reports whether a named pic is already present in the manager's
// in-memory QPic cache without triggering any load path.
func (m *Manager) IsPicCached(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.initialized {
		return false
	}
	_, ok := m.pics[name]
	return ok
}

// loadPic tries all resolution strategies and returns the first match.
// Must be called with m.mu held for writing.
//
// The cascading lookup order (WAD → pak → directory) reproduces Quake's
// original behavior where assets could be embedded in different containers.
// Each strategy is tried in turn; the first successful parse wins.
func (m *Manager) loadPic(name string) *image.QPic {
	// 1 & 2. Try gfx.wad: full name then bare name
	if pic := m.loadFromWad(name); pic != nil {
		return pic
	}

	// 3. Try standalone file from pak filesystem
	if m.filesys != nil {
		if pic := m.loadFromFS(name); pic != nil {
			return pic
		}
	}

	// 4. Try standalone file from base directory
	if m.baseDir != "" {
		if pic := m.loadFromDir(name); pic != nil {
			return pic
		}
	}

	slog.Debug("Pic not found", "name", name)
	return nil
}

// loadFromWad attempts to find and parse a QPic from the loaded gfx.wad archive.
//
// WAD lumps in Quake's original gfx.wad are stored with short names (e.g. "pause")
// without path prefixes or file extensions, but callers often use full paths like
// "gfx/pause.lmp". This method tries both the full name and a stripped "bare" name
// to handle either convention. Only lumps of type TypQPic or TypConsolePic are
// valid 2D picture assets; other lump types (e.g. TypMipTex for conchars) are
// rejected here and handled by dedicated accessors like GetConcharsData.
func (m *Manager) loadFromWad(name string) *image.QPic {
	lump, ok := m.wad.Lumps[name]
	if !ok {
		// Try bare name without path prefix or extension
		base := filepath.Base(name)
		bare := image.CleanupName(strings.TrimSuffix(base, filepath.Ext(base)))
		lump, ok = m.wad.Lumps[bare]
	}
	if !ok {
		return nil
	}
	if lump.Type != image.TypQPic && lump.Type != image.TypConsolePic {
		return nil
	}
	pic, err := image.ParseQPic(lump.Data)
	if err != nil {
		slog.Error("Failed to parse QPic from WAD", "name", name, "error", err)
		return nil
	}
	return pic
}

// loadFromFS attempts to load a QPic from the virtual filesystem (pak files).
//
// Some Quake assets live directly in pak archives as standalone .lmp files
// rather than inside gfx.wad. For example, gfx/qplaque.lmp may be stored
// as a top-level entry in pak0.pak. This method handles that case by reading
// the raw bytes via the filesystem abstraction and parsing them as a QPic.
func (m *Manager) loadFromFS(name string) *image.QPic {
	data, err := m.filesys.LoadFile(name)
	if err != nil {
		return nil
	}
	pic, err := image.ParseQPic(data)
	if err != nil {
		slog.Error("Failed to parse QPic from filesystem", "name", name, "error", err)
		return nil
	}
	return pic
}

// loadFromDir attempts to load a QPic from the base directory on disk.
//
// This is the last-resort fallback, used only when the Manager was initialized
// via InitFromDir (typically in tests). It constructs an OS filesystem path
// from the base directory and the asset name, reads the file, and parses it
// as a QPic. Forward slashes in the name are converted to OS-native separators.
func (m *Manager) loadFromDir(name string) *image.QPic {
	path := filepath.Join(m.baseDir, filepath.FromSlash(name))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	pic, err := image.ParseQPic(data)
	if err != nil {
		slog.Error("Failed to parse QPic from dir", "name", name, "error", err)
		return nil
	}
	return pic
}

// GetConcharsData returns the raw 128×128 byte pixel data for the conchars font,
// or nil if not loaded. Conchars is stored in gfx.wad as a TypMipTex lump
// (raw indexed pixels, no header — 16384 bytes for a 128×128 bitmap).
//
// The conchars bitmap is the engine's built-in bitmap font, arranged as a 16×16
// grid of 8×8 pixel character glyphs. It covers ASCII and some extended characters
// used by Quake's console, menus, and HUD. Each byte is a palette index; the
// rendering code expands these to RGBA using the loaded palette, treating
// palette index 0 as transparent so the font can overlay arbitrary backgrounds.
//
// This is kept separate from the normal GetPic path because conchars is not a
// QPic — it has no width/height header — and is handled specially by the
// original engine (W_GetLumpName("conchars") in the C source).
func (m *Manager) GetConcharsData() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.wad == nil {
		return nil
	}
	lump, ok := m.wad.Lumps["conchars"]
	if !ok || len(lump.Data) < 128*128 {
		return nil
	}
	return lump.Data[:128*128]
}

// Palette returns the Quake color palette.
// The palette is 768 bytes (256 colors * 3 RGB components).
//
// Every palette-indexed texture and picture in Quake stores pixels as single
// bytes (0–255) that are indices into this shared palette. To display them on
// a modern GPU, the rendering code looks up each index in this table to get
// the actual RGB color. The palette was defined by id Software's artists and
// is stored in palette.lmp inside the game's pak files.
func (m *Manager) Palette() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.palette
}

// Shutdown releases all cached resources and resets the manager to an
// uninitialized state.
//
// This is called during engine shutdown or when changing game directories
// (mod switching). Clearing the cache ensures that stale assets from a
// previous mod are not accidentally served when a new mod is loaded.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pics = make(map[string]*image.QPic)
	m.wad = nil
	m.filesys = nil
	m.baseDir = ""
	m.palette = nil
	m.initialized = false

	slog.Debug("Draw manager shut down")
}
