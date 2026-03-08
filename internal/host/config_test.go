package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
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
	cvarOne := cvar.Register("test_host_config_write_a", "default", cvar.FlagArchive, "")
	cvarTwo := cvar.Register("test_host_config_write_b", "default", cvar.FlagArchive, "")
	cvar.Set(cvarOne.Name, "alpha")
	cvar.Set(cvarTwo.Name, "beta")
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
		`test_host_config_write_a "alpha"`,
		`test_host_config_write_b "beta"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config.cfg missing %q in:\n%s", want, text)
		}
	}
	if strings.Index(text, `bind w "+forward"`) > strings.Index(text, `test_host_config_write_a "alpha"`) {
		t.Fatalf("expected bindings before archived cvars in:\n%s", text)
	}
	if strings.Index(text, `test_host_config_write_a "alpha"`) > strings.Index(text, `test_host_config_write_b "beta"`) {
		t.Fatalf("expected archived cvars to be written deterministically in:\n%s", text)
	}
}

func TestHostConfigArchivedCVarRoundTrip(t *testing.T) {
	userDir := t.TempDir()
	cvarName := "test_host_config_roundtrip_value"
	cv := cvar.Register(cvarName, "0", cvar.FlagArchive, "")
	cvar.Set(cv.Name, "1337")

	writer := NewHost()
	writerSubs := &Subsystems{Commands: globalTestCommandBuffer{}}
	if err := writer.Init(&InitParams{BaseDir: ".", UserDir: userDir}, writerSubs); err != nil {
		t.Fatalf("writer Init failed: %v", err)
	}
	if err := writer.WriteConfig(writerSubs); err != nil {
		t.Fatalf("writer WriteConfig failed: %v", err)
	}

	cvar.Set(cv.Name, "9")
	reader := NewHost()
	readerSubs := &Subsystems{Commands: globalTestCommandBuffer{}}
	if err := reader.Init(&InitParams{BaseDir: ".", UserDir: userDir}, readerSubs); err != nil {
		t.Fatalf("reader Init failed: %v", err)
	}

	if got := cvar.StringValue(cv.Name); got != "1337" {
		t.Fatalf("archived cvar after config load = %q, want %q", got, "1337")
	}
}
