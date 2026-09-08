package pak

import (
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SearchResult describes where a requested file was found within the VFS.
type SearchResult struct {
	Path     string   // On-disk path (file path on disk, or pak path)
	Name     string   // Internal relative name
	SourceFS iofs.FS  // Source filesystem for loose files
	IsPack   bool     // True if resident in a PAK archive
	Reader   *Reader  // Underlying PAK reader (if IsPack is true)
	Pack     *Reader  // Alias for Reader for backwards compatibility
	FilePos  int32    // Byte offset within PAK
	FileLen  int32    // Byte length within PAK
	Priority int      // Search priority index (0 = highest)
}

// SearchPathEntry is a snapshot of one mounted VFS search path entry.
type SearchPathEntry struct {
	Path      string
	IsPack    bool
	FileCount int
}

type mountKind uint8

const (
	mountLoose mountKind = iota
	mountPack
)

type mount struct {
	kind   mountKind
	root   string
	fs     iofs.FS
	reader *Reader
}

func (m *mount) Resolve(name string) *SearchResult {
	if m == nil {
		return nil
	}
	switch m.kind {
	case mountLoose:
		if m.fs == nil {
			return nil
		}
		fullPath := filepath.Join(m.root, filepath.FromSlash(name))
		if !isWithinRoot(m.root, fullPath) {
			return nil
		}
		fi, err := iofs.Stat(m.fs, name)
		if err != nil || fi.IsDir() {
			return nil
		}
		return &SearchResult{
			Path:     fullPath,
			Name:     name,
			SourceFS: m.fs,
			IsPack:   false,
		}
	case mountPack:
		if m.reader == nil {
			return nil
		}
		entry, ok := m.reader.Find(name)
		if !ok {
			return nil
		}
		return &SearchResult{
			Path:    m.reader.Filename,
			Name:    entry.Name,
			IsPack:  true,
			Reader:  m.reader,
			Pack:    m.reader,
			FilePos: entry.FilePos,
			FileLen: entry.FileLen,
		}
	}
	return nil
}

// FileSystem provides a layered virtual filesystem (VFS) with last-added-wins priority.
type FileSystem struct {
	mounts  []mount
	readers []*Reader
	mu      sync.RWMutex
}

// NewFileSystem creates an empty, uninitialized FileSystem.
func NewFileSystem() *FileSystem {
	return &FileSystem{
		mounts:  make([]mount, 0),
		readers: make([]*Reader, 0),
	}
}

// MountDirectory mounts a local filesystem directory onto the VFS override stack.
// Newer mounts take precedence over older mounts.
func (vfs *FileSystem) MountDirectory(dir string) error {
	cleanDir := filepath.Clean(dir)
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	m := mount{
		kind: mountLoose,
		root: cleanDir,
		fs:   os.DirFS(cleanDir),
	}
	vfs.mounts = append([]mount{m}, vfs.mounts...)
	return nil
}

// MountPak opens and mounts a PAK file onto the VFS override stack.
func (vfs *FileSystem) MountPak(pakPath string) error {
	r, err := Open(pakPath)
	if err != nil {
		return err
	}
	vfs.MountReader(r)
	return nil
}

// MountReader mounts an already-opened Reader onto the VFS override stack.
func (vfs *FileSystem) MountReader(r *Reader) {
	if r == nil {
		return
	}
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	vfs.readers = append(vfs.readers, r)
	m := mount{
		kind:   mountPack,
		root:   r.Filename,
		reader: r,
		fs:     r,
	}
	vfs.mounts = append([]mount{m}, vfs.mounts...)
}

// MountFS mounts an arbitrary io/fs.FS onto the VFS override stack.
func (vfs *FileSystem) MountFS(root string, fsys iofs.FS) {
	if fsys == nil {
		return
	}
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	m := mount{
		kind: mountLoose,
		root: root,
		fs:   fsys,
	}
	vfs.mounts = append([]mount{m}, vfs.mounts...)
}

// FindFile searches the VFS for the given filename and returns a SearchResult.
func (vfs *FileSystem) FindFile(filename string) (*SearchResult, error) {
	sanitized, err := sanitizePath(filename)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	for priority := range vfs.mounts {
		if result := vfs.mounts[priority].Resolve(sanitized); result != nil {
			result.Priority = priority
			return result, nil
		}
	}
	return nil, fmt.Errorf("file not found: %s", sanitized)
}

// ReadFile loads and returns the entire contents of a file from the VFS.
func (vfs *FileSystem) ReadFile(filename string) ([]byte, error) {
	result, err := vfs.FindFile(filename)
	if err != nil {
		return nil, err
	}
	if result.IsPack {
		entry := Entry{Name: result.Name, FilePos: result.FilePos, FileLen: result.FileLen}
		return result.Reader.ReadAt(entry)
	}
	return iofs.ReadFile(result.SourceFS, result.Name)
}

// LoadFile is an alias for ReadFile.
func (vfs *FileSystem) LoadFile(filename string) ([]byte, error) {
	return vfs.ReadFile(filename)
}

// OpenFile opens a file as a streaming read/seek handle and returns its length.
func (vfs *FileSystem) OpenFile(filename string) (io.ReadSeekCloser, int64, error) {
	result, err := vfs.FindFile(filename)
	if err != nil {
		return nil, 0, err
	}

	if result.IsPack {
		reader := io.NewSectionReader(result.Reader.Handle, int64(result.FilePos), int64(result.FileLen))
		return readSeekNopCloser{
			Reader: reader,
			Seeker: reader,
		}, int64(result.FileLen), nil
	}

	file, err := os.Open(result.Path)
	if err != nil {
		return nil, 0, fmt.Errorf("open %q: %w", result.Path, err)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("stat %q: %w", result.Path, err)
	}
	return file, stat.Size(), nil
}

// Open implements io/fs.FS.
func (vfs *FileSystem) Open(name string) (iofs.File, error) {
	sanitized, err := sanitizePath(name)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	for _, m := range vfs.mounts {
		if m.fs != nil {
			if f, err := m.fs.Open(sanitized); err == nil {
				return f, nil
			}
		}
	}
	return nil, iofs.ErrNotExist
}

// Stat implements io/fs.StatFS.
func (vfs *FileSystem) Stat(name string) (iofs.FileInfo, error) {
	sanitized, err := sanitizePath(name)
	if err != nil {
		return nil, err
	}

	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	for _, m := range vfs.mounts {
		if m.fs != nil {
			if fi, err := iofs.Stat(m.fs, sanitized); err == nil {
				return fi, nil
			}
		}
	}
	return nil, iofs.ErrNotExist
}

// FileExists returns true if the named file can be found anywhere in the VFS.
func (vfs *FileSystem) FileExists(filename string) bool {
	_, err := vfs.FindFile(filename)
	return err == nil
}

// ListFiles returns paths of all files matching pattern across all mounts.
func (vfs *FileSystem) ListFiles(pattern string) []string {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	var results []string
	for _, m := range vfs.mounts {
		if m.kind == mountLoose && m.fs != nil {
			matches, err := iofs.Glob(m.fs, pattern)
			if err == nil {
				for _, match := range matches {
					results = append(results, filepath.ToSlash(match))
				}
			}
		} else if m.kind == mountPack && m.reader != nil {
			for _, pf := range m.reader.Files {
				if matched, err := filepath.Match(pattern, pf.Name); err == nil && matched {
					results = append(results, pf.Name)
				}
			}
		}
	}
	return results
}

// SearchPathEntries returns a snapshot of the mounted search paths in priority order.
func (vfs *FileSystem) SearchPathEntries() []SearchPathEntry {
	vfs.mu.RLock()
	defer vfs.mu.RUnlock()

	entries := make([]SearchPathEntry, 0, len(vfs.mounts))
	for _, m := range vfs.mounts {
		if m.kind == mountPack && m.reader != nil {
			entries = append(entries, SearchPathEntry{
				Path:      m.reader.Filename,
				IsPack:    true,
				FileCount: len(m.reader.Files),
			})
		} else if m.kind == mountLoose {
			entries = append(entries, SearchPathEntry{
				Path: m.root,
			})
		}
	}
	return entries
}

// Close closes all open readers held by the VFS.
func (vfs *FileSystem) Close() error {
	vfs.mu.Lock()
	defer vfs.mu.Unlock()

	var firstErr error
	for _, r := range vfs.readers {
		if err := r.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	vfs.readers = nil
	vfs.mounts = nil
	return firstErr
}

type readSeekNopCloser struct {
	io.Reader
	io.Seeker
}

func (readSeekNopCloser) Close() error { return nil }

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

func isWithinRoot(root, target string) bool {
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)

	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
