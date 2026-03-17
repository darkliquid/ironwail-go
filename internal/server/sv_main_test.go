package server

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func withSkillCVar(t *testing.T, value string) {
	t.Helper()
	if cvar.Get("skill") == nil {
		cvar.Register("skill", "1", cvar.FlagArchive, "")
	}
	original := cvar.StringValue("skill")
	cvar.Set("skill", value)
	t.Cleanup(func() {
		cvar.Set("skill", original)
	})
}

func TestSpawnServerSyncsRoundedClampedSkillToQCVM(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	testCases := []struct {
		name  string
		value string
		want  int
	}{
		{name: "negative clamps to zero", value: "-1", want: 0},
		{name: "fraction rounds to nearest", value: "0.6", want: 1},
		{name: "middle value preserved", value: "2.2", want: 2},
		{name: "high value clamps to three", value: "4", want: 3},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			withSkillCVar(t, tc.value)

			vfs := fs.NewFileSystem()
			if err := vfs.Init(baseDir, "id1"); err != nil {
				t.Fatalf("init filesystem: %v", err)
			}
			defer vfs.Close()

			s := NewServer()
			if err := s.Init(1); err != nil {
				t.Fatalf("init server: %v", err)
			}

			progsData, err := vfs.LoadFile("progs.dat")
			if err != nil {
				t.Fatalf("load progs.dat: %v", err)
			}
			if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
				t.Fatalf("LoadProgs: %v", err)
			}
			qc.RegisterBuiltins(s.QCVM)

			if err := s.SpawnServer("start", vfs); err != nil {
				t.Fatalf("spawn server: %v", err)
			}

			if got := int(s.QCVM.GetGlobalFloat("skill")); got != tc.want {
				t.Fatalf("QC skill global = %d, want %d", got, tc.want)
			}
		})
	}
}
