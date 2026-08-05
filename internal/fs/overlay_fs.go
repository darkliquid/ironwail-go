// This file belongs to the VFS mount subsystem: the OverlayFS that resolves
// names across the ordered mount stack (plan 19b).

package fs

import (
	"errors"
	iofs "io/fs"
	"io"
)

// overlayFS resolves a name across an ordered list of io/fs.FS sources.
// Source 0 has the highest priority (Quake override order): the first source
// that can satisfy the request wins. If a source returns fs.ErrNotExist the
// overlay falls through to the next; any other error is returned immediately.
//
// Each source is either a *PakFS (an archive) or a *rootFS (a loose
// directory). The overlay is the single resolution path of the VFS; it is
// built from the FileSystem.mounts stack.
type overlayFS struct {
	sources []iofs.FS // priority order: index 0 = highest
}

// newOverlayFS builds an overlay over the ordered mount stack.
func newOverlayFS(mounts []mount) *overlayFS {
	sources := make([]iofs.FS, 0, len(mounts))
	for i := range mounts {
		sources = append(sources, mounts[i].fs())
	}
	return &overlayFS{sources: sources}
}

// Open implements io/fs.FS.
func (o *overlayFS) Open(name string) (iofs.File, error) {
	for _, src := range o.sources {
		f, err := src.Open(name)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, iofs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, &iofs.PathError{Op: "open", Path: name, Err: iofs.ErrNotExist}
}

// ReadFile implements io/fs.ReadFileFS.
func (o *overlayFS) ReadFile(name string) ([]byte, error) {
	for _, src := range o.sources {
		if r, ok := src.(iofs.ReadFileFS); ok {
			data, err := r.ReadFile(name)
			if err == nil {
				return data, nil
			}
			if !errors.Is(err, iofs.ErrNotExist) {
				return nil, err
			}
			continue
		}
		// Fallback for plain io/fs.FS sources.
		f, err := src.Open(name)
		if err != nil {
			if !errors.Is(err, iofs.ErrNotExist) {
				return nil, err
			}
			continue
		}
		data, readErr := io.ReadAll(f)
		closeErr := f.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return data, nil
	}
	return nil, &iofs.PathError{Op: "read", Path: name, Err: iofs.ErrNotExist}
}

// Stat implements io/fs.StatFS.
func (o *overlayFS) Stat(name string) (iofs.FileInfo, error) {
	for _, src := range o.sources {
		if s, ok := src.(iofs.StatFS); ok {
			fi, err := s.Stat(name)
			if err == nil {
				return fi, nil
			}
			if !errors.Is(err, iofs.ErrNotExist) {
				return nil, err
			}
			continue
		}
		// Fallback: open + stat via the file's own Stat.
		f, err := src.Open(name)
		if err != nil {
			if !errors.Is(err, iofs.ErrNotExist) {
				return nil, err
			}
			continue
		}
		fi, statErr := f.Stat()
		closeErr := f.Close()
		if statErr != nil {
			return nil, statErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return fi, nil
	}
	return nil, &iofs.PathError{Op: "stat", Path: name, Err: iofs.ErrNotExist}
}
