package fs

import (
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// PakFS implements the standard library io/fs.FS interface over a mounted
// PAK archive (*Pack). It lets callers treat a Quake .pak archive like any
// other filesystem (io/fs.ReadFile, io/fs.Stat, io/fs.Glob, fs.WalkDir,
// etc.) while preserving the archive's on-disk byte layout through the
// Pack's open handle.
//
// Lookup is case-insensitive and slash-normalised (canonicalPackLookup),
// matching Quake's original DOS/Windows case-folding semantics.
type PakFS struct {
	pack *Pack
}

// NewPakFS wraps an open PAK archive as an io/fs.FS.
func NewPakFS(pack *Pack) *PakFS {
	return &PakFS{pack: pack}
}

// Pack returns the underlying archive.
func (p *PakFS) Pack() *Pack {
	return p.pack
}

// Open implements io/fs.FS. Names are slash-separated virtual paths.
func (p *PakFS) Open(name string) (fs.File, error) {
	if p == nil || p.pack == nil {
		return nil, fs.ErrInvalid
	}
	fi, err := p.find(name)
	if err != nil {
		return nil, err
	}
	return &pakFile{fs: p, fi: fi}, nil
}

// ReadFile implements io/fs.ReadFileFS.
func (p *PakFS) ReadFile(name string) ([]byte, error) {
	fi, err := p.find(name)
	if err != nil {
		return nil, err
	}
	return p.readAt(fi)
}

// Stat implements io/fs.StatFS.
func (p *PakFS) Stat(name string) (fs.FileInfo, error) {
	fi, err := p.find(name)
	if err != nil {
		return nil, err
	}
	return &pakFileInfo{fi: fi}, nil
}

// ReadDir implements io/fs.ReadDirFS by unfolding the archive's directory
// entries one level deep for the given virtual directory.
func (p *PakFS) ReadDir(name string) ([]fs.DirEntry, error) {
	dir := strings.Trim(path.Clean(name), "/")
	seen := make(map[string]struct{})
	var entries []fs.DirEntry
	for _, pf := range p.pack.Files {
		pfDir, base := path.Split(strings.Trim(pf.Name, "/"))
		pfDir = strings.Trim(pfDir, "/")
		if dir == "" {
			// Root: only top-level entries.
			if strings.Contains(pfDir, "/") || base == "" {
				continue
			}
			if _, ok := seen[base]; ok {
				continue
			}
			seen[base] = struct{}{}
			isDir := base != pf.Name || strings.HasSuffix(pf.Name, "/")
			entries = append(entries, dirEntryFromName{name: base, isDir: isDir})
			continue
		}
		if pfDir != dir {
			continue
		}
		if base == "" {
			continue
		}
		if _, ok := seen[base]; ok {
			continue
		}
		seen[base] = struct{}{}
		entries = append(entries, dirEntryFromName{name: base, isDir: false})
	}
	if len(entries) == 0 {
		return nil, fs.ErrNotExist
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

// find locates a PackFile by canonical (case-insensitive, slash-normalised)
// name.
func (p *PakFS) find(name string) (*PackFile, error) {
	lookup := canonicalPackLookup(name)
	for i := range p.pack.Files {
		if p.pack.Files[i].Lookup == lookup {
			return &p.pack.Files[i], nil
		}
	}
	return nil, fs.ErrNotExist
}

// readAt slurps a PackFile's byte window from the archive handle.
func (p *PakFS) readAt(fi *PackFile) ([]byte, error) {
	p.pack.mu.Lock()
	defer p.pack.mu.Unlock()
	data := make([]byte, fi.FileLen)
	if _, err := p.pack.Handle.ReadAt(data, int64(fi.FilePos)); err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
}

// pakFile is an fs.File view over one archive entry.
type pakFile struct {
	fs *PakFS
	fi *PackFile
	// ofs is the read cursor for successive Read calls.
	ofs int64
	data []byte
}

func (f *pakFile) Stat() (fs.FileInfo, error) {
	return &pakFileInfo{fi: f.fi}, nil
}

func (f *pakFile) Read(b []byte) (int, error) {
	if f.data == nil {
		data, err := f.fs.readAt(f.fi)
		if err != nil {
			return 0, err
		}
		f.data = data
	}
	if f.ofs >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(b, f.data[f.ofs:])
	f.ofs += int64(n)
	return n, nil
}

func (f *pakFile) Close() error { return nil }

// pakFileInfo reports the archive metadata for a single entry.
type pakFileInfo struct {
	fi *PackFile
}

func (i *pakFileInfo) Name() string       { return path.Base(i.fi.Name) }
func (i *pakFileInfo) Size() int64        { return int64(i.fi.FileLen) }
func (i *pakFileInfo) Mode() fs.FileMode  { return 0 }
func (i *pakFileInfo) ModTime() time.Time { return time.Time{} }
func (i *pakFileInfo) IsDir() bool        { return false }
func (i *pakFileInfo) Sys() any           { return nil }

// dirEntryFromName adapts a bare name into an fs.DirEntry.
type dirEntryFromName struct {
	name  string
	isDir bool
}

func (d dirEntryFromName) Name() string               { return d.name }
func (d dirEntryFromName) IsDir() bool                { return d.isDir }
func (d dirEntryFromName) Type() fs.FileMode          { return 0 }
func (d dirEntryFromName) Info() (fs.FileInfo, error) { return nil, fs.ErrNotExist }

// Resolve returns a SearchResult for the named archive entry, or nil if the
// pack does not contain it. It is the single home of pack-entry lookup,
// used by both the OverlayFS and (during migration) the legacy loops.
func (p *PakFS) Resolve(name string) *SearchResult {
	if p == nil || p.pack == nil {
		return nil
	}
	fi, err := p.find(name)
	if err != nil {
		return nil
	}
	return &SearchResult{
		Path:    p.pack.Filename,
		Name:    name,
		IsPack:  true,
		Pack:    p.pack,
		FilePos: fi.FilePos,
		FileLen: fi.FileLen,
	}
}
