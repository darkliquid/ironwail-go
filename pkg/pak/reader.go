package pak

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// ReadSeekerCloserHandle represents an open byte source for PAK files that supports
// sequential reading, seeking, random-access reading at an offset, and closing.
type ReadSeekerCloserHandle interface {
	io.Reader
	io.Seeker
	io.ReaderAt
	io.Closer
}

// Reader provides random-access reading over a Quake PAK archive.
// It implements io/fs.FS, io/fs.ReadFileFS, io/fs.StatFS, and io/fs.ReadDirFS.
type Reader struct {
	Filename string
	Handle   ReadSeekerCloserHandle
	Files    []Entry
	lookup   map[string]int
	mu       sync.Mutex
}

// Pack is an alias for Reader for backwards compatibility.
type Pack = Reader

// Open opens a PAK archive from the local filesystem.
func Open(filename string) (*Reader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("open pak %q: %w", filename, err)
	}
	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat pak %q: %w", filename, err)
	}
	r, err := NewReader(file, stat.Size())
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	r.Filename = filename
	return r, nil
}

// OpenBytes parses a PAK archive from an in-memory byte slice.
func OpenBytes(filename string, data []byte) (*Reader, error) {
	handle := &byteReaderHandle{Reader: bytes.NewReader(data)}
	r, err := NewReader(handle, int64(len(data)))
	if err != nil {
		return nil, err
	}
	r.Filename = filename
	return r, nil
}

// LoadPackFromBytes is an alias for OpenBytes for backwards compatibility.
func LoadPackFromBytes(filename string, data []byte) (*Reader, error) {
	return OpenBytes(filename, data)
}

// NewReader creates a new Reader reading from r, which must have the given total size (or <= 0 if unknown).
func NewReader(r io.ReaderAt, size int64) (*Reader, error) {
	if size > 0 && size < HeaderSize {
		return nil, fmt.Errorf("pak is too short (%d bytes, header is %d bytes)", size, HeaderSize)
	}

	var header Header
	hdrBuf := make([]byte, HeaderSize)
	if _, err := r.ReadAt(hdrBuf, 0); err != nil {
		return nil, fmt.Errorf("read pak header: %w", err)
	}

	copy(header.ID[:], hdrBuf[0:4])
	header.DirOfs = int32(binary.LittleEndian.Uint32(hdrBuf[4:8]))
	header.DirLen = int32(binary.LittleEndian.Uint32(hdrBuf[8:12]))

	if string(header.ID[:]) != Magic {
		return nil, fmt.Errorf("not a valid pak file (bad magic %q)", string(header.ID[:]))
	}

	if header.DirOfs < HeaderSize || (size > 0 && int64(header.DirOfs) > size) {
		return nil, fmt.Errorf("directory offset %d out of bounds (size %d)", header.DirOfs, size)
	}
	if header.DirLen < 0 || header.DirLen%EntrySize != 0 {
		return nil, fmt.Errorf("invalid directory length %d (must be positive multiple of %d)", header.DirLen, EntrySize)
	}
	if size > 0 && int64(header.DirOfs)+int64(header.DirLen) > size {
		return nil, fmt.Errorf("directory extends past end of file (%d > %d)", int64(header.DirOfs)+int64(header.DirLen), size)
	}

	numFiles := int(header.DirLen / EntrySize)
	files := make([]Entry, numFiles)
	lookup := make(map[string]int, numFiles)

	if header.DirLen > 0 {
		dirBuf := make([]byte, header.DirLen)
		if _, err := r.ReadAt(dirBuf, int64(header.DirOfs)); err != nil {
			return nil, fmt.Errorf("read pak directory: %w", err)
		}

		for i := 0; i < numFiles; i++ {
			entBytes := dirBuf[i*EntrySize : (i+1)*EntrySize]
			nameLen := 0
			for nameLen < MaxFileName && entBytes[nameLen] != 0 {
				nameLen++
			}
			rawName := string(entBytes[:nameLen])
			filePos := int32(binary.LittleEndian.Uint32(entBytes[56:60]))
			fileLen := int32(binary.LittleEndian.Uint32(entBytes[60:64]))
			cName := CanonicalLookup(rawName)

			files[i] = Entry{
				Name:    rawName,
				Lookup:  cName,
				FilePos: filePos,
				FileLen: fileLen,
			}
			if _, exists := lookup[cName]; !exists {
				lookup[cName] = i
			}
		}
	}

	var handle ReadSeekerCloserHandle
	if h, ok := r.(ReadSeekerCloserHandle); ok {
		handle = h
	} else {
		handle = &readerAtAdapter{r: r, size: size}
	}

	return &Reader{
		Handle: handle,
		Files:  files,
		lookup: lookup,
	}, nil
}

// Close releases any resources associated with the reader.
func (r *Reader) Close() error {
	if r == nil || r.Handle == nil {
		return nil
	}
	return r.Handle.Close()
}

// Entries returns a copy or slice of all file entries in the archive.
func (r *Reader) Entries() []Entry {
	if r == nil {
		return nil
	}
	return r.Files
}

// Find looks up an entry by filename using case-insensitive, slash-normalised matching.
func (r *Reader) Find(name string) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	cName := CanonicalLookup(name)
	if idx, ok := r.lookup[cName]; ok {
		return r.Files[idx], true
	}
	return Entry{}, false
}

// File is an alias for Find.
func (r *Reader) File(name string) (Entry, bool) {
	return r.Find(name)
}

// ReadFile reads the complete contents of the named file from the archive.
func (r *Reader) ReadFile(name string) ([]byte, error) {
	entry, ok := r.Find(name)
	if !ok {
		return nil, iofs.ErrNotExist
	}
	return r.ReadAt(entry)
}

// ReadAt reads the byte window for the given Entry from the archive.
func (r *Reader) ReadAt(entry Entry) ([]byte, error) {
	if r == nil || r.Handle == nil {
		return nil, iofs.ErrInvalid
	}
	if entry.FileLen < 0 || entry.FilePos < 0 {
		return nil, fmt.Errorf("invalid entry bounds: pos=%d len=%d", entry.FilePos, entry.FileLen)
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	data := make([]byte, entry.FileLen)
	if _, err := r.Handle.ReadAt(data, int64(entry.FilePos)); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return data, nil
}

// OpenSection returns an io.SectionReader positioned over entry's data.
func (r *Reader) OpenSection(entry Entry) *io.SectionReader {
	return io.NewSectionReader(r.Handle, int64(entry.FilePos), int64(entry.FileLen))
}

// Open implements io/fs.FS.
func (r *Reader) Open(name string) (iofs.File, error) {
	entry, ok := r.Find(name)
	if !ok {
		return nil, iofs.ErrNotExist
	}
	return &pakFile{reader: r, entry: entry}, nil
}

// Stat implements io/fs.StatFS.
func (r *Reader) Stat(name string) (iofs.FileInfo, error) {
	entry, ok := r.Find(name)
	if !ok {
		return nil, iofs.ErrNotExist
	}
	return &pakFileInfo{entry: entry}, nil
}

// ReadDir implements io/fs.ReadDirFS by unfolding the archive's directory
// entries one level deep for the given virtual directory.
func (r *Reader) ReadDir(name string) ([]iofs.DirEntry, error) {
	dir := strings.Trim(path.Clean(name), "/")
	if dir == "." {
		dir = ""
	}
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}

	seen := make(map[string]struct{})
	var entries []iofs.DirEntry

	for _, f := range r.Files {
		trimmed := strings.Trim(f.Name, "/")
		if prefix != "" && !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		rel := strings.TrimPrefix(trimmed, prefix)
		if rel == "" {
			continue
		}
		isDir := false
		if slash := strings.Index(rel, "/"); slash >= 0 {
			rel = rel[:slash]
			isDir = true
		}
		if _, exists := seen[rel]; exists {
			continue
		}
		seen[rel] = struct{}{}
		entries = append(entries, dirEntryFromName{name: rel, isDir: isDir})
	}

	if len(entries) == 0 {
		return nil, iofs.ErrNotExist
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

// pakFile implements io/fs.File over an Entry in Reader.
type pakFile struct {
	reader *Reader
	entry  Entry
	ofs    int64
	data   []byte
}

func (f *pakFile) Stat() (iofs.FileInfo, error) {
	return &pakFileInfo{entry: f.entry}, nil
}

func (f *pakFile) Read(b []byte) (int, error) {
	if f.data == nil {
		data, err := f.reader.ReadAt(f.entry)
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

type pakFileInfo struct {
	entry Entry
}

func (i *pakFileInfo) Name() string       { return path.Base(i.entry.Name) }
func (i *pakFileInfo) Size() int64        { return int64(i.entry.FileLen) }
func (i *pakFileInfo) Mode() iofs.FileMode { return 0 }
func (i *pakFileInfo) ModTime() time.Time { return time.Time{} }
func (i *pakFileInfo) IsDir() bool        { return false }
func (i *pakFileInfo) Sys() any           { return nil }

type dirEntryFromName struct {
	name  string
	isDir bool
}

func (d dirEntryFromName) Name() string { return d.name }
func (d dirEntryFromName) IsDir() bool  { return d.isDir }
func (d dirEntryFromName) Type() iofs.FileMode {
	if d.isDir {
		return iofs.ModeDir
	}
	return 0
}
func (d dirEntryFromName) Info() (iofs.FileInfo, error) { return nil, iofs.ErrNotExist }

type byteReaderHandle struct {
	*bytes.Reader
}

func (b byteReaderHandle) Close() error { return nil }

type readerAtAdapter struct {
	r      io.ReaderAt
	size   int64
	offset int64
}

func (a *readerAtAdapter) Read(p []byte) (int, error) {
	if a.offset >= a.size {
		return 0, io.EOF
	}
	n, err := a.r.ReadAt(p, a.offset)
	a.offset += int64(n)
	return n, err
}

func (a *readerAtAdapter) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = a.offset + offset
	case io.SeekEnd:
		next = a.size + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if next < 0 {
		return 0, fmt.Errorf("negative seek offset: %d", next)
	}
	a.offset = next
	return a.offset, nil
}

func (a *readerAtAdapter) ReadAt(p []byte, off int64) (int, error) {
	return a.r.ReadAt(p, off)
}

func (a *readerAtAdapter) Close() error {
	if c, ok := a.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
