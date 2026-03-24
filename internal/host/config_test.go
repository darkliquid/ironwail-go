package host

import (
	"os"
	"path/filepath"
	"reflect"
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

type staticTestFilesystem struct {
	files map[string]string
}

func (f *staticTestFilesystem) Init(baseDir, gameDir string) error { return nil }
func (f *staticTestFilesystem) Close()                             {}
func (f *staticTestFilesystem) LoadFile(filename string) ([]byte, error) {
	if data, ok := f.files[filename]; ok {
		return []byte(data), nil
	}
	return nil, os.ErrNotExist
}
func (f *staticTestFilesystem) LoadFirstAvailable(filenames []string) (string, []byte, error) {
	for _, filename := range filenames {
		if data, ok := f.files[filename]; ok {
			return filename, []byte(data), nil
		}
	}
	return "", nil, os.ErrNotExist
}
func (f *staticTestFilesystem) FileExists(filename string) bool {
	_, ok := f.files[filename]
	return ok
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

func TestHostCmdExecStripsCStyleCommentsFromScriptedCommands(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	h.SetUserDir(userDir)

	configPath := filepath.Join(userDir, "autoexec.cfg")
	if err := os.WriteFile(configPath, []byte(strings.Join([]string{
		"test_exec_comment first // trailing line comment",
		"/* block comment with ; semicolon should not spawn commands */",
		"test_exec_comment second",
	}, "\n")), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", configPath, err)
	}

	var executed []string
	cmdsys.AddCommand("test_exec_comment", func(args []string) {
		executed = append(executed, strings.Join(args, " "))
	}, "")
	defer cmdsys.RemoveCommand("test_exec_comment")

	h.CmdExec([]string{"autoexec.cfg"}, &Subsystems{Commands: globalTestCommandBuffer{}})

	want := []string{"first", "second"}
	if !reflect.DeepEqual(executed, want) {
		t.Fatalf("exec command payloads = %v, want %v", executed, want)
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

func TestHostCmdExecDefaultCfgUsesBuiltinFallback(t *testing.T) {
	h := NewHost()

	var (
		unbindAllCalled bool
		scrAutoscale    bool
		mlook           bool
		bindings        = map[string]string{}
	)

	cmdsys.AddCommand("unbindall", func(args []string) {
		unbindAllCalled = true
		clear(bindings)
	}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("unbindall") })
	cmdsys.AddCommand("bind", func(args []string) {
		if len(args) >= 2 {
			bindings[args[0]] = args[1]
		}
	}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("bind") })
	cmdsys.AddCommand("alias", func(args []string) {}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("alias") })
	cmdsys.AddCommand("scr_autoscale", func(args []string) { scrAutoscale = true }, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("scr_autoscale") })
	cmdsys.AddCommand("+mlook", func(args []string) { mlook = true }, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("+mlook") })

	h.CmdExec([]string{"default.cfg"}, &Subsystems{
		Files:    &staticTestFilesystem{files: map[string]string{}},
		Commands: globalTestCommandBuffer{},
	})

	if !unbindAllCalled {
		t.Fatalf("default.cfg fallback did not execute unbindall")
	}
	if !scrAutoscale {
		t.Fatalf("default.cfg fallback did not execute scr_autoscale")
	}
	if !mlook {
		t.Fatalf("default.cfg fallback did not execute +mlook")
	}
	for key, want := range map[string]string{
		"ESCAPE":    "togglemenu",
		"TILDE":     "toggleconsole",
		"BACKQUOTE": "toggleconsole",
		"t":         "messagemode",
	} {
		if got := bindings[key]; got != want {
			t.Fatalf("binding %q = %q, want %q", key, got, want)
		}
	}
}

func TestHostCmdExecDefaultCfgPrefersBuiltinOverFilesystem(t *testing.T) {
	h := NewHost()

	var (
		unbindAllCalled bool
		scrAutoscale    bool
		bindings        = map[string]string{}
	)

	cmdsys.AddCommand("unbindall", func(args []string) {
		unbindAllCalled = true
		clear(bindings)
	}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("unbindall") })
	cmdsys.AddCommand("bind", func(args []string) {
		if len(args) >= 2 {
			bindings[args[0]] = args[1]
		}
	}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("bind") })
	cmdsys.AddCommand("alias", func(args []string) {}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("alias") })
	cmdsys.AddCommand("scr_autoscale", func(args []string) { scrAutoscale = true }, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("scr_autoscale") })

	h.CmdExec([]string{"default.cfg"}, &Subsystems{
		Files: &staticTestFilesystem{files: map[string]string{
			"default.cfg": "bind ESCAPE oldmenu\nbind F6 oldquicksave\ngamma 0.95\n",
		}},
		Commands: globalTestCommandBuffer{},
	})

	if !unbindAllCalled {
		t.Fatalf("default.cfg builtin did not execute unbindall")
	}
	if !scrAutoscale {
		t.Fatalf("default.cfg builtin did not execute scr_autoscale")
	}
	if got := bindings["ESCAPE"]; got != "togglemenu" {
		t.Fatalf("binding ESCAPE = %q, want %q", got, "togglemenu")
	}
	if got := bindings["F6"]; got != "save quick" {
		t.Fatalf("binding F6 = %q, want %q", got, "save quick")
	}
}

func TestHostCmdExecLegacyConfigIgnoresFilesystemIronwailCfg(t *testing.T) {
	h := NewHost()

	var (
		bindings        = map[string]string{}
		scrAutoscale    bool
		unbindAllCalled bool
	)

	cmdsys.AddCommand("unbindall", func(args []string) {
		unbindAllCalled = true
		clear(bindings)
	}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("unbindall") })
	cmdsys.AddCommand("bind", func(args []string) {
		if len(args) >= 2 {
			bindings[args[0]] = args[1]
		}
	}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("bind") })
	cmdsys.AddCommand("alias", func(args []string) {}, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("alias") })
	cmdsys.AddCommand("scr_autoscale", func(args []string) { scrAutoscale = true }, "")
	t.Cleanup(func() { cmdsys.RemoveCommand("scr_autoscale") })

	h.CmdExec([]string{"config.cfg"}, &Subsystems{
		Files: &staticTestFilesystem{files: map[string]string{
			"ironwail.cfg": "bind ESCAPE oldmenu\nbind F6 oldquicksave\ngamma 0.95\n",
		}},
		Commands: globalTestCommandBuffer{},
	})

	if unbindAllCalled {
		t.Fatalf("legacy config exec should not have executed packaged ironwail.cfg")
	}
	if scrAutoscale {
		t.Fatalf("legacy config exec should not have executed scr_autoscale from packaged ironwail.cfg")
	}
	if len(bindings) != 0 {
		t.Fatalf("legacy config exec should not have loaded packaged ironwail.cfg bindings: %#v", bindings)
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
		`bind "w" "+forward"`,
		`bind "F10" "+attack"`,
		`test_host_config_write_a "alpha"`,
		`test_host_config_write_b "beta"`,
		`vid_restart`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q in:\n%s", configFileName, want, text)
		}
	}
	if strings.Index(text, `bind "w" "+forward"`) > strings.Index(text, `test_host_config_write_a "alpha"`) {
		t.Fatalf("expected bindings before archived cvars in:\n%s", text)
	}
	if strings.Index(text, `test_host_config_write_a "alpha"`) > strings.Index(text, `test_host_config_write_b "beta"`) {
		t.Fatalf("expected archived cvars to be written deterministically in:\n%s", text)
	}
	if strings.Index(text, `test_host_config_write_b "beta"`) > strings.Index(text, `vid_restart`) {
		t.Fatalf("expected vid_restart after archived cvars in:\n%s", text)
	}
}

func TestHostWriteConfigQuotesSpecialKeyBindings(t *testing.T) {
	h := NewHost()
	userDir := t.TempDir()
	inputSystem := input.NewSystem(nil)
	inputSystem.SetBinding(int('`'), "toggleconsole")
	inputSystem.SetBinding(int('\\'), "+mlook")
	subs := &Subsystems{Input: inputSystem}

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
		`bind "BACKQUOTE" "toggleconsole"`,
		`bind "\\" "+mlook"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("%s missing %q in:\n%s", configFileName, want, text)
		}
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

func TestLoadArchivedCvarsPrefersCanonicalConfigAndOnlyAppliesWhitelist(t *testing.T) {
	userDir := t.TempDir()
	width := cvar.Register("test_startup_vid_width", "640", cvar.FlagArchive, "")
	height := cvar.Register("test_startup_vid_height", "480", cvar.FlagArchive, "")
	unrelated := cvar.Register("test_startup_unrelated", "keep", cvar.FlagArchive, "")

	cvar.Set(width.Name, "640")
	cvar.Set(height.Name, "480")
	cvar.Set(unrelated.Name, "keep")

	if err := os.WriteFile(filepath.Join(userDir, configFileName), []byte(strings.Join([]string{
		`test_startup_vid_width "1280"`,
		`test_startup_vid_height "720"`,
		`bind w "+forward"`,
		`test_startup_unrelated "changed"`,
		`vid_restart`,
	}, "\n")), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", configFileName, err)
	}
	if err := os.WriteFile(filepath.Join(userDir, legacyConfigName), []byte(strings.Join([]string{
		`test_startup_vid_width "320"`,
		`test_startup_vid_height "200"`,
	}, "\n")), 0644); err != nil {
		t.Fatalf("WriteFile(%q): %v", legacyConfigName, err)
	}

	if err := LoadArchivedCvars(userDir, []string{width.Name, height.Name}); err != nil {
		t.Fatalf("LoadArchivedCvars failed: %v", err)
	}

	if got := cvar.StringValue(width.Name); got != "1280" {
		t.Fatalf("%s = %q, want %q", width.Name, got, "1280")
	}
	if got := cvar.StringValue(height.Name); got != "720" {
		t.Fatalf("%s = %q, want %q", height.Name, got, "720")
	}
	if got := cvar.StringValue(unrelated.Name); got != "keep" {
		t.Fatalf("%s = %q, want unchanged %q", unrelated.Name, got, "keep")
	}
}
