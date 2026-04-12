package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/server"
)

type registrationModeTestFS struct {
	hasPop bool
}

func (fs registrationModeTestFS) FileExists(filename string) bool {
	return fs.hasPop && filename == "gfx/pop.lmp"
}

func TestConfigureRegistrationModeRegisteredWhenPopPresent(t *testing.T) {
	if cvar.Get("registered") == nil {
		cvar.Register("registered", "0", cvar.FlagNone, "")
	}
	cvar.Set("registered", "0")

	if err := configureRegistrationMode(registrationModeTestFS{hasPop: true}, "id1"); err != nil {
		t.Fatalf("configureRegistrationMode returned error: %v", err)
	}
	if got := cvar.IntValue("registered"); got != 1 {
		t.Fatalf("registered = %d, want 1", got)
	}
}

func TestConfigureRegistrationModeSharewareForID1(t *testing.T) {
	if cvar.Get("registered") == nil {
		cvar.Register("registered", "1", cvar.FlagNone, "")
	}
	cvar.Set("registered", "1")

	if err := configureRegistrationMode(registrationModeTestFS{hasPop: false}, "id1"); err != nil {
		t.Fatalf("configureRegistrationMode returned error: %v", err)
	}
	if got := cvar.IntValue("registered"); got != 0 {
		t.Fatalf("registered = %d, want 0", got)
	}
}

func TestConfigureRegistrationModeRejectsModsWithoutRegisteredData(t *testing.T) {
	if cvar.Get("registered") == nil {
		cvar.Register("registered", "1", cvar.FlagNone, "")
	}
	cvar.Set("registered", "1")

	err := configureRegistrationMode(registrationModeTestFS{hasPop: false}, "hipnotic")
	if err == nil {
		t.Fatal("configureRegistrationMode should fail for mod dir in shareware mode")
	}
	if got := cvar.IntValue("registered"); got != 0 {
		t.Fatalf("registered = %d, want 0", got)
	}
}

func TestShouldWarnAboutGoGPUX11Keyboard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		goos           string
		waylandDisplay string
		x11Display     string
		want           bool
	}{
		{
			name:       "warns on linux x11",
			goos:       "linux",
			x11Display: ":0",
			want:       true,
		},
		{
			name:           "skips when wayland is present",
			goos:           "linux",
			waylandDisplay: "wayland-0",
			x11Display:     ":0",
		},
		{
			name:       "skips without x11 display",
			goos:       "linux",
			x11Display: "",
		},
		{
			name:       "skips on non-linux",
			goos:       "darwin",
			x11Display: ":0",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldWarnAboutGoGPUX11Keyboard(tt.goos, tt.waylandDisplay, tt.x11Display)
			if got != tt.want {
				t.Fatalf("shouldWarnAboutGoGPUX11Keyboard(%q, %q, %q) = %v, want %v", tt.goos, tt.waylandDisplay, tt.x11Display, got, tt.want)
			}
		})
	}
}

func TestGoGPUX11KeyboardHint(t *testing.T) {
	t.Parallel()

	if got := gogpuX11KeyboardHint(); got != "run under Wayland for event-driven keyboard input" {
		t.Fatalf("gogpuX11KeyboardHint() = %q", got)
	}
}

func TestCurrentZoomSpeedUsesCanonicalZoomSpeedCVar(t *testing.T) {
	if cvar.Get("zoom_speed") == nil {
		cvar.Register("zoom_speed", "8", cvar.FlagArchive, "")
	}

	cvar.Set("zoom_speed", "12")
	t.Cleanup(func() {
		cvar.Set("zoom_speed", "8")
	})

	if got := currentZoomSpeed(); got != 12 {
		t.Fatalf("currentZoomSpeed() = %v, want 12", got)
	}
}

func TestCurrentRuntimeFOVUsesCanonicalFOVCVar(t *testing.T) {
	if cvar.Get("fov") == nil {
		cvar.Register("fov", "90", cvar.FlagArchive, "")
	}

	cvar.Set("fov", "110")
	t.Cleanup(func() {
		cvar.Set("fov", "90")
	})

	if got := currentRuntimeFOV(); got != 110 {
		t.Fatalf("currentRuntimeFOV() = %v, want 110", got)
	}
}

func TestCurrentRuntimeViewSizeUsesCanonicalViewsizeCVar(t *testing.T) {
	registerMirroredArchiveCvars("viewsize", "scr_viewsize", "100", "")

	cvar.Set("scr_viewsize", "100")
	cvar.Set("viewsize", "130")
	t.Cleanup(func() {
		cvar.Set("viewsize", "100")
		cvar.Set("scr_viewsize", "100")
	})

	if got := currentRuntimeViewSize(); got != 130 {
		t.Fatalf("currentRuntimeViewSize() = %v, want 130", got)
	}
	if got := cvar.FloatValue("scr_viewsize"); got != 130 {
		t.Fatalf("legacy scr_viewsize alias = %v, want 130", got)
	}
}

func TestCurrentRuntimeZoomFOVUsesCanonicalZoomFOVCVar(t *testing.T) {
	if cvar.Get("zoom_fov") == nil {
		cvar.Register("zoom_fov", "30", cvar.FlagArchive, "")
	}

	cvar.Set("zoom_fov", "55")
	t.Cleanup(func() {
		cvar.Set("zoom_fov", "30")
	})

	if got := currentRuntimeZoomFOV(); got != 55 {
		t.Fatalf("currentRuntimeZoomFOV() = %v, want 55", got)
	}
}

func TestCurrentRuntimeFOVAdaptUsesCanonicalFOVAdaptCVar(t *testing.T) {
	if cvar.Get("fov_adapt") == nil {
		cvar.Register("fov_adapt", "1", cvar.FlagArchive, "")
	}

	cvar.Set("fov_adapt", "0")
	t.Cleanup(func() {
		cvar.Set("fov_adapt", "1")
	})

	if got := currentRuntimeFOVAdapt(); got {
		t.Fatal("currentRuntimeFOVAdapt() = true, want false")
	}
}

func TestCurrentShowTurtlePrefersCanonicalShowturtleCVar(t *testing.T) {
	registerMirroredArchiveCvars("showturtle", "scr_showturtle", "0", "")

	cvar.Set("scr_showturtle", "0")
	cvar.Set("showturtle", "1")
	t.Cleanup(func() {
		cvar.Set("showturtle", "0")
		cvar.Set("scr_showturtle", "0")
	})

	if got := currentShowTurtle(); !got {
		t.Fatal("currentShowTurtle() = false, want true")
	}
	if got := cvar.BoolValue("scr_showturtle"); !got {
		t.Fatal("legacy scr_showturtle alias did not mirror canonical showturtle")
	}
}

func TestRegisterColorShiftPercentCvarsRegistersDefaults(t *testing.T) {
	t.Parallel()

	registry := cvar.NewCVarSystem()
	registerColorShiftPercentCvars(registry.Register)

	tests := []struct {
		name string
	}{
		{name: "gl_cshiftpercent"},
		{name: "gl_cshiftpercent_contents"},
		{name: "gl_cshiftpercent_damage"},
		{name: "gl_cshiftpercent_bonus"},
		{name: "gl_cshiftpercent_powerup"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cv := registry.Get(tt.name)
			if cv == nil {
				t.Fatalf("%s should be registered", tt.name)
			}
			if cv.String != "100" {
				t.Fatalf("%s default = %q, want 100", tt.name, cv.String)
			}
			if cv.Flags&cvar.FlagArchive == 0 {
				t.Fatalf("%s should be archived", tt.name)
			}
		})
	}
}

func TestRendererRDynamicCVarName(t *testing.T) {
	if renderer.CvarRDynamic != "r_dynamic" {
		t.Fatalf("renderer.CvarRDynamic = %q, want %q", renderer.CvarRDynamic, "r_dynamic")
	}
}

func TestRegisterRendererLightingAndParticleCvarsRegistersParityDefaults(t *testing.T) {
	t.Parallel()

	registry := cvar.NewCVarSystem()
	registerRendererLightingAndParticleCvars(registry.Register)

	tests := []struct {
		name         string
		defaultValue string
	}{
		{name: renderer.CvarRDynamic, defaultValue: "1"},
		{name: renderer.CvarRParticles, defaultValue: "2"},
		{name: renderer.CvarRNoLerpList, defaultValue: "progs/flame.mdl progs/flame2.mdl progs/braztall.mdl progs/brazshrt.mdl progs/longtrch.mdl progs/flame_pyre.mdl progs/v_saw.mdl progs/v_xfist.mdl progs/h2stuff/newfire.mdl"},
		{name: renderer.CvarGLTextureMode, defaultValue: "GL_NEAREST_MIPMAP_LINEAR"},
		{name: renderer.CvarGLLodBias, defaultValue: "0"},
		{name: renderer.CvarGLAnisotropy, defaultValue: "1"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cv := registry.Get(tt.name)
			if cv == nil {
				t.Fatalf("%s should be registered", tt.name)
			}
			if cv.String != tt.defaultValue {
				t.Fatalf("%s default = %q, want %s", tt.name, cv.String, tt.defaultValue)
			}
			if cv.Flags&cvar.FlagArchive == 0 {
				t.Fatalf("%s should be archived", tt.name)
			}
		})
	}
}

func TestBuildCSQCClientHooksExposeStatAndPlayerBuiltins(t *testing.T) {
	originalClient := g.Client
	t.Cleanup(func() {
		g.Client = originalClient
	})

	g.Client = cl.NewClient()
	g.Client.Stats[3] = 77
	g.Client.Stats[5] = 0xAB
	g.Client.StatsF[5] = 12.5
	g.Client.PlayerNames[1] = "Ranger"
	g.Client.Frags[1] = 42
	g.Client.PlayerColors[1] = 0x2d

	hooks := buildCSQCClientHooks()

	if got := hooks.GetStatInt(3); got != 77 {
		t.Fatalf("GetStatInt(3) = %d, want 77", got)
	}
	if got := hooks.GetStatFloat(5, 0, 0); got != 12.5 {
		t.Fatalf("GetStatFloat(5,0,0) = %v, want 12.5", got)
	}
	if got := hooks.GetStatFloat(5, 4, 4); got != 0xA {
		t.Fatalf("GetStatFloat(5,4,4) = %v, want 10", got)
	}
	if got := hooks.GetStatString(3); got != "77" {
		t.Fatalf("GetStatString(3) = %q, want 77", got)
	}
	if got := hooks.GetPlayerKeyValue(1, "name"); got != "Ranger" {
		t.Fatalf("GetPlayerKeyValue(name) = %q, want Ranger", got)
	}
	if got := hooks.GetPlayerKeyValue(1, "frags"); got != "42" {
		t.Fatalf("GetPlayerKeyValue(frags) = %q, want 42", got)
	}
	if got := hooks.GetPlayerKeyValue(1, "topcolor"); got != strconv.Itoa(int((0x2d&0xf0)>>4)) {
		t.Fatalf("GetPlayerKeyValue(topcolor) = %q", got)
	}
	if got := hooks.GetPlayerKeyValue(1, "bottomcolor"); got != strconv.Itoa(int(0x2d&0x0f)) {
		t.Fatalf("GetPlayerKeyValue(bottomcolor) = %q", got)
	}
	if got := hooks.GetPlayerKeyValue(1, "team"); got != strconv.Itoa(int(0x2d&0x0f)+1) {
		t.Fatalf("GetPlayerKeyValue(team) = %q", got)
	}
}

func TestBuildCSQCClientHooksRegistersCommandOnce(t *testing.T) {
	hooks := buildCSQCClientHooks()
	cmdName := "csqc_unit_registercommand_test"
	cmdsys.RemoveCommand(cmdName)
	t.Cleanup(func() {
		cmdsys.RemoveCommand(cmdName)
	})

	hooks.RegisterCommand(cmdName)
	if !cmdsys.Exists(cmdName) {
		t.Fatalf("command %q not registered", cmdName)
	}
	hooks.RegisterCommand(cmdName)
	if !cmdsys.Exists(cmdName) {
		t.Fatalf("command %q should remain registered", cmdName)
	}
}

type reloadTestRenderer struct{}

func (reloadTestRenderer) OnDraw(func(renderer.RenderContext))                    {}
func (reloadTestRenderer) OnUpdate(func(float64))                                 {}
func (reloadTestRenderer) Size() (int, int)                                       { return 320, 200 }
func (reloadTestRenderer) SetConfig(renderer.Config)                              {}
func (reloadTestRenderer) Run() error                                             { return nil }
func (reloadTestRenderer) Stop()                                                  {}
func (reloadTestRenderer) Shutdown()                                              {}
func (reloadTestRenderer) SetPalette([]byte)                                      {}
func (reloadTestRenderer) SetConchars([]byte)                                     {}
func (reloadTestRenderer) SetExternalSkybox(string, func(string) ([]byte, error)) {}
func (reloadTestRenderer) UpdateCamera(renderer.CameraState, float32, float32)    {}
func (reloadTestRenderer) UploadWorld(*bsp.Tree) error                            { return nil }
func (reloadTestRenderer) HasWorldData() bool                                     { return false }
func (reloadTestRenderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	return [3]float32{}, [3]float32{}, false
}
func (reloadTestRenderer) SpawnDynamicLight(renderer.DynamicLight) bool      { return false }
func (reloadTestRenderer) SpawnKeyedDynamicLight(renderer.DynamicLight) bool { return false }
func (reloadTestRenderer) UpdateLights(float32)                              {}
func (reloadTestRenderer) ClearDynamicLights()                               {}
func (reloadTestRenderer) InputBackendForSystem(*input.System) input.Backend { return nil }

func TestRuntimeMenuModsUsesCurrentSubsystemFilesystem(t *testing.T) {
	original := g
	t.Cleanup(func() { g = original })

	baseA := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseA, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseA, "id1", "progs.dat"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write id1 progs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseA, "hipnotic", "pak0.pak"), []byte("pak"), 0o644); err != nil {
		t.Fatalf("write hipnotic pak: %v", err)
	}

	baseB := t.TempDir()
	for _, dir := range []string{"id1", "rogue"} {
		if err := os.MkdirAll(filepath.Join(baseB, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseB, "id1", "progs.dat"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write id1 progs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseB, "rogue", "pak0.pak"), []byte("pak"), 0o644); err != nil {
		t.Fatalf("write rogue pak: %v", err)
	}

	fsA := fs.NewFileSystem()
	if err := fsA.Init(baseA, "id1"); err != nil {
		t.Fatalf("init fsA: %v", err)
	}
	defer fsA.Close()
	fsB := fs.NewFileSystem()
	if err := fsB.Init(baseB, "id1"); err != nil {
		t.Fatalf("init fsB: %v", err)
	}
	defer fsB.Close()

	g.Subs = &host.Subsystems{Files: fsA}
	modsA := runtimeMenuMods(g.Subs)
	if len(modsA) != 1 || modsA[0].Name != "hipnotic" {
		t.Fatalf("mods from fsA = %#v, want hipnotic", modsA)
	}

	g.Subs.Files = fsB
	modsB := runtimeMenuMods(g.Subs)
	if len(modsB) != 1 || modsB[0].Name != "rogue" {
		t.Fatalf("mods from fsB = %#v, want rogue", modsB)
	}
}

func TestReloadRuntimeAfterGameDirChangeResetsSessionAndKeepsRenderer(t *testing.T) {
	original := g
	t.Cleanup(func() { g = original })

	progsData := []byte("test progs")

	baseDir := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseDir, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(baseDir, dir, "progs.dat"), progsData, 0o644); err != nil {
			t.Fatalf("write %s/progs.dat: %v", dir, err)
		}
	}

	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(baseDir, "hipnotic"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer fileSys.Close()

	testRenderer := reloadTestRenderer{}
	g.Renderer = testRenderer
	g.Host = host.NewHost()
	g.Menu = menu.NewManager(nil, nil)
	g.Server = server.NewServer()
	g.QC = g.Server.QCVM
	g.CSQC = qc.NewCSQC()
	originalServer := g.Server
	originalQC := g.QC
	originalCSQC := g.CSQC
	g.Subs = &host.Subsystems{
		Files:  fileSys,
		Server: g.Server,
	}
	g.Host.SetMenu(g.Menu)
	g.ModDir = "id1"
	g.AliasModelCache = map[string]*model.Model{"progs/player.mdl": nil}
	g.SpriteModelCache = map[string]*runtimeSpriteModel{"progs/flame.spr": nil}
	g.ShowScores = true
	g.WorldUploadKey = "old-world"

	if err := reloadRuntimeAfterGameDirChange(g.Subs, fileSys); err != nil {
		t.Fatalf("reloadRuntimeAfterGameDirChange failed: %v", err)
	}

	if g.Renderer != testRenderer {
		t.Fatal("reload replaced renderer; expected renderer/window stack to be preserved")
	}
	if g.Server != originalServer {
		t.Fatal("reload replaced server; expected map bootstrap to keep server instance")
	}
	if g.QC != originalQC {
		t.Fatal("reload replaced server qc; expected server-owned VM to be preserved")
	}
	if g.CSQC != originalCSQC {
		t.Fatal("reload replaced CSQC container; expected in-place unload")
	}
	if g.CSQC.IsLoaded() {
		t.Fatal("CSQC should be unloaded after mod reload")
	}
	if g.ModDir != "hipnotic" {
		t.Fatalf("mod dir = %q, want hipnotic", g.ModDir)
	}
	if g.Menu == nil || !g.Menu.IsActive() || g.Menu.GetState() != menu.MenuMain {
		t.Fatalf("menu state = active:%v state:%v, want active main menu", g.Menu != nil && g.Menu.IsActive(), g.Menu.GetState())
	}
	if g.AliasModelCache != nil {
		t.Fatalf("alias model cache should reset, got %#v", g.AliasModelCache)
	}
	if g.SpriteModelCache != nil {
		t.Fatalf("sprite model cache should reset, got %#v", g.SpriteModelCache)
	}
	if g.ShowScores {
		t.Fatal("show scores should reset to false")
	}
	if g.WorldUploadKey != "" {
		t.Fatalf("world upload key = %q, want empty", g.WorldUploadKey)
	}
}

func TestRuntimeDrawFileSystemPrefersCurrentSubsystemFilesystem(t *testing.T) {
	original := g
	t.Cleanup(func() { g = original })

	baseA := t.TempDir()
	baseB := t.TempDir()
	for _, dir := range []string{"id1", "hipnotic"} {
		if err := os.MkdirAll(filepath.Join(baseA, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s in baseA: %v", dir, err)
		}
		if err := os.MkdirAll(filepath.Join(baseB, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s in baseB: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(baseA, "hipnotic", "progs.dat"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write baseA hipnotic progs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseB, "hipnotic", "progs.dat"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write baseB hipnotic progs: %v", err)
	}

	fallback := fs.NewFileSystem()
	if err := fallback.Init(baseA, "id1"); err != nil {
		t.Fatalf("init fallback fs: %v", err)
	}
	defer fallback.Close()

	current := fs.NewFileSystem()
	if err := current.Init(baseB, "id1"); err != nil {
		t.Fatalf("init current fs: %v", err)
	}
	defer current.Close()

	g.Subs = &host.Subsystems{Files: current}

	if got := runtimeDrawFileSystem(fallback); got != current {
		t.Fatalf("runtimeDrawFileSystem() = %p, want current subsystem fs %p", got, current)
	}
}
