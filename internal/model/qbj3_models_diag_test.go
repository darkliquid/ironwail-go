package model

import (
	"bytes"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// TestQbj3AliasModelsLoad probes the qbj3 mod model files involved in the
// visible-weapon / visible-keycard regression: v_wrench.mdl (first-person
// weapon), g_wrench.mdl, and b_s_key.mdl (keycard world model). If any fails
// to parse, the renderer will silently skip it and the item will appear
// invisible even though the network path is healthy.
func TestQbj3AliasModelsLoad(t *testing.T) {
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	vfs := fs.NewFileSystem()
	if err := vfs.Init(quakeDir, "qbj3"); err != nil {
		t.Fatalf("Init(qbj3): %v", err)
	}
	defer vfs.Close()

	names := []string{
		"progs/v_wrench.mdl",
		"progs/g_wrench.mdl",
		"progs/b_s_key.mdl",
		"progs/v_flakshotgun.mdl",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			data, err := vfs.LoadFile(name)
			if err != nil {
				t.Fatalf("LoadFile: %v", err)
			}
			m, err := LoadAliasModel(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("LoadAliasModel: %v", err)
			}
			if m.AliasHeader == nil || m.AliasHeader.NumFrames < 1 {
				t.Fatalf("bad alias header/frames: %+v", m.AliasHeader)
			}
			t.Logf("%s: frames=%d skins=%d verts=%d tris=%d",
				name, m.AliasHeader.NumFrames, m.AliasHeader.NumSkins,
				m.AliasHeader.NumVerts, m.AliasHeader.NumTris)
		})
	}
}
