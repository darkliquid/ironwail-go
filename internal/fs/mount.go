// This file belongs to the VFS mount subsystem: the unified mount stack that
// replaced the old searchPaths/lookupPaths dual-slice design (plan 19b).
//
// Every mount is either a loose directory (rootFS) or a PAK archive (PakFS);
// there is no third kind. The stack is ordered high→low priority
// (index 0 = highest), matching Quake override semantics.

package fs

import (
	iofs "io/fs"
	"os"
	"path/filepath"
)

// mountKind distinguishes the two mount flavours.
type mountKind uint8

const (
	mountLoose mountKind = iota
	mountPack
)

// mount is one entry in the VFS override stack.
type mount struct {
	kind mountKind
	// loose is non-nil for mountLoose entries.
	loose *rootFS
	// pak is non-nil for mountPack entries.
	pak *PakFS
}

// rootFS exposes a loose game directory as an io/fs.FS while retaining the
// OS root path needed to reconstruct SearchResult.Path for loose files.
type rootFS struct {
	root string
	fs   iofs.FS
}

// newRootFS wraps an on-disk directory.
func newRootFS(dir string) *rootFS {
	dir = filepath.Clean(dir)
	return &rootFS{root: dir, fs: os.DirFS(dir)}
}

// Open implements io/fs.FS.
func (r *rootFS) Open(name string) (iofs.File, error) {
	if r == nil || r.fs == nil {
		return nil, iofs.ErrInvalid
	}
	return r.fs.Open(name)
}

// ReadFile implements io/fs.ReadFileFS.
func (r *rootFS) ReadFile(name string) ([]byte, error) {
	if r == nil || r.fs == nil {
		return nil, iofs.ErrInvalid
	}
	return iofs.ReadFile(r.fs, name)
}

// Stat implements io/fs.StatFS.
func (r *rootFS) Stat(name string) (iofs.FileInfo, error) {
	if r == nil || r.fs == nil {
		return nil, iofs.ErrInvalid
	}
	return iofs.Stat(r.fs, name)
}

// Resolve returns a SearchResult for the named loose file if it exists and is
// a regular file, or nil otherwise.
func (r *rootFS) Resolve(name string) *SearchResult {
	if r == nil || r.fs == nil {
		return nil
	}
	fullPath := filepath.Join(r.root, filepath.FromSlash(name))
	if !isWithinRoot(r.root, fullPath) {
		return nil
	}
	fi, err := iofs.Stat(r.fs, name)
	if err != nil || fi.IsDir() {
		return nil
	}
	return &SearchResult{
		Path:     fullPath,
		Name:     name,
		SourceFS: r.fs,
		IsPack:   false,
	}
}

// Resolve resolves the name against this mount. Only Resolve on a pack mount
// returns nonzero FilePos/FileLen.
func (m *mount) Resolve(name string) *SearchResult {
	if m == nil {
		return nil
	}
	switch m.kind {
	case mountLoose:
		return m.loose.Resolve(name)
	case mountPack:
		return m.pak.Resolve(name)
	}
	return nil
}
