package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/input"
)

type globalTestCommandBuffer struct{}

func (globalTestCommandBuffer) Init()               {}
func (globalTestCommandBuffer) Execute()            { cmdsys.Execute() }
func (globalTestCommandBuffer) AddText(text string) { cmdsys.AddText(text) }
func (globalTestCommandBuffer) InsertText(text string) {
	cmdsys.InsertText(text)
}
func (globalTestCommandBuffer) Shutdown() {}

func TestHostCmdExecRunsUserConfig(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	h.SetUserDir(userDir)

	configPath := filepath.Join(userDir, "autoexec.cfg")
	if err := os.WriteFile(configPath, []byte("test_exec_cmd loaded\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", configPath, err)
	}

	var executed string
	cmdsys.AddCommand("test_exec_cmd", func(args []string) {
		executed = strings.Join(args, " ")
	}, "")
	defer cmdsys.RemoveCommand("test_exec_cmd")

	h.CmdExec("autoexec.cfg", &Subsystems{Commands: globalTestCommandBuffer{}})
	if executed != "loaded" {
		t.Fatalf("exec command payload = %q, want %q", executed, "loaded")
	}
}

func TestHostWriteConfigIncludesBindings(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	inputSystem := input.NewSystem(nil)
	inputSystem.SetBinding(input.KF10, "+attack")
	inputSystem.SetBinding(int('w'), "+forward")
	subs := &Subsystems{
		Console: &mockConsole{},
		Input:   inputSystem,
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: userDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := h.WriteConfig(subs); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(userDir, "config.cfg"))
	if err != nil {
		t.Fatalf("ReadFile(config.cfg): %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`bind w "+forward"`,
		`bind F10 "+attack"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config.cfg missing %q in:\n%s", want, text)
		}
	}
}
