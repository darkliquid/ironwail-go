// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

type trackingReadSeekCloser struct {
	reader     *bytes.Reader
	closeCalls int
	closed     bool
	closeErr   error
}

func newTrackingReadSeekCloser(data []byte, closeErr error) *trackingReadSeekCloser {
	return &trackingReadSeekCloser{
		reader:   bytes.NewReader(data),
		closeErr: closeErr,
	}
}

func (h *trackingReadSeekCloser) Read(p []byte) (int, error) {
	return h.reader.Read(p)
}

func (h *trackingReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return h.reader.Seek(offset, whence)
}

func (h *trackingReadSeekCloser) Close() error {
	h.closeCalls++
	h.closed = true
	return h.closeErr
}

type trackingOpenFileFS struct {
	files      map[string][]byte
	closeErr   map[string]error
	openErr    map[string]error
	openRecord map[string][]*trackingReadSeekCloser
}

func (f *trackingOpenFileFS) OpenFile(filename string) (io.ReadSeekCloser, int64, error) {
	if err, ok := f.openErr[filename]; ok {
		return nil, 0, err
	}
	data, ok := f.files[filename]
	if !ok {
		return nil, 0, os.ErrNotExist
	}
	handle := newTrackingReadSeekCloser(data, f.closeErr[filename])
	if f.openRecord == nil {
		f.openRecord = make(map[string][]*trackingReadSeekCloser)
	}
	f.openRecord[filename] = append(f.openRecord[filename], handle)
	return handle, int64(len(data)), nil
}

func firstExistingModel(t *testing.T, vfs *fs.FileSystem, candidates []string) string {
	t.Helper()
	for _, name := range candidates {
		if vfs.FileExists(name) {
			return name
		}
	}
	t.Skipf("none of the model candidates exist: %v", candidates)
	return ""
}

func expectedModelInfoFromLoadFile(t *testing.T, vfs *fs.FileSystem, modelName string) cachedModelInfo {
	t.Helper()

	data, err := vfs.LoadFile(modelName)
	if err != nil {
		t.Fatalf("LoadFile(%q): %v", modelName, err)
	}

	var info cachedModelInfo
	switch filepath.Ext(modelName) {
	case ".mdl":
		m, err := model.LoadAliasModel(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("LoadAliasModel(%q): %v", modelName, err)
		}
		info = cachedModelInfo{mins: m.Mins, maxs: m.Maxs, known: true}
	case ".spr":
		sprite, err := model.LoadSprite(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("LoadSprite(%q): %v", modelName, err)
		}
		info.mins, info.maxs = spriteBounds(sprite)
		info.known = true
	case ".bsp":
		tree, err := bsp.LoadTree(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("LoadTree(%q): %v", modelName, err)
		}
		if len(tree.Models) > 0 {
			info = cachedModelInfo{mins: tree.Models[0].BoundsMin, maxs: tree.Models[0].BoundsMax, known: true}
		} else {
			info = cachedModelInfo{known: true}
		}
	default:
		t.Fatalf("unsupported extension for parity helper: %q", modelName)
	}

	return info
}

func TestCacheModelInfoOpenFileParsingParity(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	models := []string{
		firstExistingModel(t, vfs, []string{"progs/player.mdl", "progs/soldier.mdl", "progs/backpack.mdl"}),
		firstExistingModel(t, vfs, []string{"progs/flame.spr", "progs/s_explod.spr", "progs/s_bubble.spr"}),
		firstExistingModel(t, vfs, []string{"maps/start.bsp", "maps/e1m1.bsp"}),
	}

	for _, modelName := range models {
		t.Run(modelName, func(t *testing.T) {
			s := NewServer()
			s.FileSystem = vfs
			s.modelCache = make(map[string]cachedModelInfo)

			want := expectedModelInfoFromLoadFile(t, vfs, modelName)
			got, err := s.cacheModelInfo(modelName)
			if err != nil {
				t.Fatalf("cacheModelInfo(%q): %v", modelName, err)
			}
			if got != want {
				t.Fatalf("cacheModelInfo(%q) = %+v, want %+v", modelName, got, want)
			}
		})
	}
}

func TestCacheModelInfoOpenFileHandleClosedOnSuccessAndParseError(t *testing.T) {
	tmpDir := t.TempDir()
	sprPath := filepath.Join(tmpDir, "test.spr")
	writeTestSprite(t, sprPath, 8, 6)
	validSprite, err := os.ReadFile(sprPath)
	if err != nil {
		t.Fatalf("read sprite fixture: %v", err)
	}

	stub := &trackingOpenFileFS{
		files: map[string][]byte{
			"progs/ok.spr":  validSprite,
			"progs/bad.mdl": {0x00, 0x01, 0x02},
		},
		closeErr: map[string]error{
			"progs/ok.spr": errors.New("close sentinel"),
		},
		openErr: make(map[string]error),
	}

	s := NewServer()
	s.FileSystem = stub
	s.modelCache = make(map[string]cachedModelInfo)

	if _, err := s.cacheModelInfo("progs/ok.spr"); err != nil {
		t.Fatalf("cacheModelInfo(progs/ok.spr): %v", err)
	}
	okHandles := stub.openRecord["progs/ok.spr"]
	if len(okHandles) != 1 {
		t.Fatalf("OpenFile(progs/ok.spr) calls = %d, want 1", len(okHandles))
	}
	if !okHandles[0].closed || okHandles[0].closeCalls != 1 {
		t.Fatalf("success handle closed=%v closeCalls=%d, want true/1", okHandles[0].closed, okHandles[0].closeCalls)
	}

	if _, err := s.cacheModelInfo("progs/bad.mdl"); err == nil {
		t.Fatal("cacheModelInfo(invalid mdl) err = nil, want parse error")
	}
	badHandles := stub.openRecord["progs/bad.mdl"]
	if len(badHandles) != 1 {
		t.Fatalf("OpenFile(progs/bad.mdl) calls = %d, want 1", len(badHandles))
	}
	if !badHandles[0].closed || badHandles[0].closeCalls != 1 {
		t.Fatalf("error handle closed=%v closeCalls=%d, want true/1", badHandles[0].closed, badHandles[0].closeCalls)
	}
}
