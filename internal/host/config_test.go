package host

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/input"
)

type globalTestCommandBuffer struct{}

func (globalTestCommandBuffer) Init()    {}
func (globalTestCommandBuffer) Execute() { cmdsys.Execute() }
func (globalTestCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {
	cmdsys.ExecuteWithSource(source)
}
func (globalTestCommandBuffer) AddText(text string) { cmdsys.AddText(text) }
func (globalTestCommandBuffer) InsertText(text string) {
	cmdsys.InsertText(text)
}
func (globalTestCommandBuffer) Shutdown() {}

type testClientWithState struct {
	state *cl.Client
}

func (c *testClientWithState) Init() error                    { return nil }
func (c *testClientWithState) Frame(float64) error            { return nil }
func (c *testClientWithState) Shutdown()                      {}
func (c *testClientWithState) State() ClientState             { return caDisconnected }
func (c *testClientWithState) ReadFromServer() error          { return nil }
func (c *testClientWithState) SendCommand() error             { return nil }
func (c *testClientWithState) SendStringCmd(cmd string) error { return nil }
func (c *testClientWithState) ClientState() *cl.Client {
	if c == nil {
		return nil
	}
	return c.state
}

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

	h.CmdExec([]string{"autoexec.cfg"}, &Subsystems{Commands: globalTestCommandBuffer{}})
	if executed != "loaded" {
		t.Fatalf("exec command payload = %q, want %q", executed, "loaded")
	}
}

func TestHostCmdExecConfigAliasUsesCanonicalConfigName(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	h.SetUserDir(userDir)

	if err := os.WriteFile(filepath.Join(userDir, configFileName), []byte("test_exec_alias canonical\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", configFileName, err)
	}
	if err := os.WriteFile(filepath.Join(userDir, legacyConfigName), []byte("test_exec_alias legacy\n"), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", legacyConfigName, err)
	}

	var executed string
	cmdsys.AddCommand("test_exec_alias", func(args []string) {
		executed = strings.Join(args, " ")
	}, "")
	defer cmdsys.RemoveCommand("test_exec_alias")

	h.CmdExec([]string{legacyConfigName}, &Subsystems{Commands: globalTestCommandBuffer{}})
	if executed != "canonical" {
		t.Fatalf("exec config alias payload = %q, want %q", executed, "canonical")
	}

	executed = ""
	h.CmdExec([]string{legacyConfigName, "pls"}, &Subsystems{Commands: globalTestCommandBuffer{}})
	if executed != "legacy" {
		t.Fatalf("exec literal config payload = %q, want %q", executed, "legacy")
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

	data, err := os.ReadFile(filepath.Join(userDir, configFileName))
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", configFileName, err)
	}
	text := string(data)
	for _, want := range []string{
		`bind w "+forward"`,
		`bind F10 "+attack"`,
		`test_host_config_write_a "alpha"`,
		`test_host_config_write_b "beta"`,
		`vid_restart`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q in:\n%s", configFileName, want, text)
		}
	}
	if strings.Index(text, `bind w "+forward"`) > strings.Index(text, `test_host_config_write_a "alpha"`) {
		t.Fatalf("expected bindings before archived cvars in:\n%s", text)
	}
	if strings.Index(text, `test_host_config_write_a "alpha"`) > strings.Index(text, `test_host_config_write_b "beta"`) {
		t.Fatalf("expected archived cvars to be written deterministically in:\n%s", text)
	}
	if strings.Index(text, `test_host_config_write_b "beta"`) > strings.Index(text, `vid_restart`) {
		t.Fatalf("expected vid_restart after archived cvars in:\n%s", text)
	}
}

func TestHostWriteConfigAppendsHeldMLookState(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	cv := cvar.Register("test_host_config_mlook_archived", "default", cvar.FlagArchive, "")
	cvar.Set(cv.Name, "value")
	subs := &Subsystems{
		Client: &testClientWithState{state: &cl.Client{InputMLook: cl.KButton{State: 1}}},
	}

	if err := h.Init(&InitParams{BaseDir: ".", UserDir: userDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := h.WriteConfig(subs); err != nil {
		t.Fatalf("WriteConfig failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(userDir, configFileName))
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", configFileName, err)
	}
	text := string(data)

	archived := `test_host_config_mlook_archived "value"`
	vidRestart := `vid_restart`
	mlook := `+mlook`
	if !strings.Contains(text, archived) {
		t.Fatalf("%s missing %q in:\n%s", configFileName, archived, text)
	}
	if !strings.Contains(text, vidRestart) {
		t.Fatalf("%s missing %q in:\n%s", configFileName, vidRestart, text)
	}
	if !strings.Contains(text, mlook) {
		t.Fatalf("%s missing %q in:\n%s", configFileName, mlook, text)
	}
	if strings.Index(text, archived) > strings.Index(text, vidRestart) {
		t.Fatalf("expected vid_restart after archived cvars in:\n%s", text)
	}
	if strings.Index(text, vidRestart) > strings.Index(text, mlook) {
		t.Fatalf("expected +mlook after vid_restart in:\n%s", text)
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

func TestHostWriteConfigNamedAddsCfgExtension(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	subs := &Subsystems{}
	if err := h.Init(&InitParams{BaseDir: ".", UserDir: userDir}, subs); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := h.WriteConfigNamed("custom", subs); err != nil {
		t.Fatalf("WriteConfigNamed failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(userDir, "custom.cfg")); err != nil {
		t.Fatalf("Stat(custom.cfg): %v", err)
	}
}
