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

// NewManager creates a new draw manager.
func NewManager() *Manager {
	return &Manager{
		pics: make(map[string]*image.QPic),
	}
}

// Init loads the WAD file and initializes the asset cache.
// It searches for gfx.wad in the provided filesystem.
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

// loadPic tries all resolution strategies and returns the first match.
// Must be called with m.mu held for writing.
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
func (m *Manager) Palette() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.palette
}

// Shutdown releases all cached resources.
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
