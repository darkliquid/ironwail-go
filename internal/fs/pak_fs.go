package fs

import (
	"io/fs"
)

// PakFS implements the standard library io/fs.FS interface over a mounted
// PAK archive (*Pack).
type PakFS struct {
	pack *Pack
}

// NewPakFS wraps an open PAK archive as an io/fs.FS.
func NewPakFS(pack *Pack) *PakFS {
	return &PakFS{pack: pack}
}

// Pack returns the underlying archive.
func (p *PakFS) Pack() *Pack {
	if p == nil {
		return nil
	}
	return p.pack
}

// Open implements io/fs.FS.
func (p *PakFS) Open(name string) (fs.File, error) {
	if p == nil || p.pack == nil {
		return nil, fs.ErrInvalid
	}
	return p.pack.Open(name)
}

// ReadFile implements io/fs.ReadFileFS.
func (p *PakFS) ReadFile(name string) ([]byte, error) {
	if p == nil || p.pack == nil {
		return nil, fs.ErrInvalid
	}
	return p.pack.ReadFile(name)
}

// Stat implements io/fs.StatFS.
func (p *PakFS) Stat(name string) (fs.FileInfo, error) {
	if p == nil || p.pack == nil {
		return nil, fs.ErrInvalid
	}
	return p.pack.Stat(name)
}

// ReadDir implements io/fs.ReadDirFS.
func (p *PakFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if p == nil || p.pack == nil {
		return nil, fs.ErrInvalid
	}
	return p.pack.ReadDir(name)
}

// Resolve returns a SearchResult for the named archive entry, or nil if the
// pack does not contain it.
func (p *PakFS) Resolve(name string) *SearchResult {
	if p == nil || p.pack == nil {
		return nil
	}
	entry, ok := p.pack.Find(name)
	if !ok {
		return nil
	}
	return &SearchResult{
		Path:    p.pack.Filename,
		Name:    entry.Name,
		IsPack:  true,
		Reader:  p.pack,
		Pack:    p.pack,
		FilePos: entry.FilePos,
		FileLen: entry.FileLen,
	}
}

func (p *PakFS) readAt(fi *PackFile) ([]byte, error) {
	if p == nil || p.pack == nil || fi == nil {
		return nil, fs.ErrInvalid
	}
	return p.pack.ReadAt(*fi)
}
