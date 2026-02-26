package draw

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

	// Load the palette from gfx.wad
	paletteLump, ok := wad.Lumps["palette.lmp"]
	if !ok || len(paletteLump.Data) < 768 {
		return fmt.Errorf("failed to find or parse palette.lmp in gfx.wad")
	}

	// Quake palette is 768 bytes (256 colors * 3 RGB components)
	m.palette = paletteLump.Data[:768]

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
// Returns nil if the lump doesn't exist or isn't a QPic.
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

	// Need to load the lump - upgrade to write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check cache in case another goroutine loaded it
	if pic, ok := m.pics[name]; ok {
		return pic
	}

	// Load the lump from WAD
	lump, ok := m.wad.Lumps[name]
	if !ok {
		slog.Debug("Lump not found", "name", name)
		return nil
	}

	// Parse the QPic
	if lump.Type != image.TypQPic {
		slog.Debug("Lump is not a QPic", "name", name, "type", lump.Type)
		return nil
	}

	pic, err := image.ParseQPic(lump.Data)
	if err != nil {
		slog.Warn("Failed to parse QPic", "name", name, "error", err)
		return nil
	}

	// Cache the parsed QPic
	m.pics[name] = pic

	return pic
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
	m.palette = nil
	m.initialized = false

	slog.Debug("Draw manager shut down")
}
