package game

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/gameconfig"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/server"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

type registrationModeTestFS struct {
	hasPop bool
}

func (fs registrationModeTestFS) FileExists(filename string) bool {
	return fs.hasPop && filename == "gfx/pop.lmp"
}

func TestConfigureRegistrationModeRegisteredWhenPopPresent(t *testing.T) {
	g := New()
	g.Config = gameconfig.Default()
	if g.Host.CVar.Get("registered") == nil {
		g.Host.CVar.Register("registered", "0", cvar.FlagNone, "")
	}
	g.Host.CVar.Set("registered", "0")

	if err := g.configureRegistrationMode(registrationModeTestFS{hasPop: true}, "id1"); err != nil {
		t.Fatalf("configureRegistrationMode returned error: %v", err)
	}
	if got := g.Host.CVar.IntValue("registered"); got != 1 {
		t.Fatalf("registered = %d, want 1", got)
	}
}

func TestConfigureRegistrationModeSharewareForID1(t *testing.T) {
	g := New()
	g.Config = gameconfig.Default()
	if g.Host.CVar.Get("registered") == nil {
		g.Host.CVar.Register("registered", "1", cvar.FlagNone, "")
	}
	g.Host.CVar.Set("registered", "1")

	if err := g.configureRegistrationMode(registrationModeTestFS{hasPop: false}, "id1"); err != nil {
		t.Fatalf("configureRegistrationMode returned error: %v", err)
	}
	if got := g.Host.CVar.IntValue("registered"); got != 0 {
		t.Fatalf("registered = %d, want 0", got)
	}
}

func TestConfigureRegistrationModeRejectsModsWithoutRegisteredData(t *testing.T) {
	g := New()
	g.Config = gameconfig.Default()
	if g.Host.CVar.Get("registered") == nil {
		g.Host.CVar.Register("registered", "1", cvar.FlagNone, "")
	}
	g.Host.CVar.Set("registered", "1")

	err := g.configureRegistrationMode(registrationModeTestFS{hasPop: false}, "hipnotic")
	if err == nil {
		t.Fatal("configureRegistrationMode should fail for mod dir in shareware mode")
	}
	if got := g.Host.CVar.IntValue("registered"); got != 0 {
		t.Fatalf("registered = %d, want 0", got)
	}
}

func TestStartupVideoOverridesWinOverArchivedConfig(t *testing.T) {
	cv := cvar.NewCVarSystem()
	cv.Register("vid_width", "1920", cvar.FlagArchive, "")
	cv.Register("vid_height", "1080", cvar.FlagArchive, "")
	cv.Register("vid_fullscreen", "1", cvar.FlagArchive, "")

	oldWidth, oldHeight := startupVidWidthOverride, startupVidHeightOverride
	oldHasWidth, oldHasHeight := startupVidWidthOverridden, startupVidHeightOverridden
	oldWindowed, oldHasWindowed := startupVidWindowedOverride, startupVidWindowedOverridden
	t.Cleanup(func() {
		startupVidWidthOverride = oldWidth
		startupVidHeightOverride = oldHeight
		startupVidWidthOverridden = oldHasWidth
		startupVidHeightOverridden = oldHasHeight
		startupVidWindowedOverride = oldWindowed
		startupVidWindowedOverridden = oldHasWindowed
	})

	SetStartupVideoOverrides(640, 360, true, true, true)
	applyStartupVideoOverrides(cv)

	if got := cv.IntValue("vid_width"); got != 640 {
		t.Fatalf("vid_width = %d, want command-line override 640", got)
	}
	if got := cv.IntValue("vid_height"); got != 360 {
		t.Fatalf("vid_height = %d, want command-line override 360", got)
	}
	if got := cv.BoolValue("vid_fullscreen"); got {
		t.Fatal("vid_fullscreen = true, want false from -window override")
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

			g := New()
			got := g.shouldWarnAboutGoGPUX11Keyboard(tt.goos, tt.waylandDisplay, tt.x11Display)
			if got != tt.want {
				t.Fatalf("shouldWarnAboutGoGPUX11Keyboard(%q, %q, %q) = %v, want %v", tt.goos, tt.waylandDisplay, tt.x11Display, got, tt.want)
			}
		})
	}
}

func TestGoGPUX11KeyboardHint(t *testing.T) {
	t.Parallel()

	g := New()
	if got := g.gogpuX11KeyboardHint(); got != "run under Wayland for event-driven keyboard input" {
		t.Fatalf("gogpuX11KeyboardHint() = %q", got)
	}
}

func TestEnsureGameplayBindingsRestoresMissingMovementBindings(t *testing.T) {
	g := New()
	g.Input = input.NewSystem(nil)
	g.Input.SetBinding(input.KEscape, "togglemenu")

	g.ensureGameplayBindings()

	for key, want := range map[int]string{
		int('w'): "+forward",
		int('s'): "+back",
		int('a'): "+moveleft",
		int('d'): "+moveright",
	} {
		if got := g.Input.Binding(key); got != want {
			t.Fatalf("binding %q = %q, want %q", input.KeyToString(key), got, want)
		}
	}
}

func TestEnsureGameplayBindingsKeepsCustomMovementBindings(t *testing.T) {
	g := New()
	g.Input = input.NewSystem(nil)
	g.Input.SetBinding(int('i'), "+forward")
	g.Input.SetBinding(int('k'), "+back")
	g.Input.SetBinding(int('j'), "+moveleft")
	g.Input.SetBinding(int('l'), "+moveright")

	g.ensureGameplayBindings()

	for key, command := range map[int]string{
		int('w'): "+forward",
		int('s'): "+back",
		int('a'): "+moveleft",
		int('d'): "+moveright",
	} {
		if got := g.Input.Binding(key); got == command {
			t.Fatalf("default binding %q was restored despite custom %q binding", input.KeyToString(key), command)
		}
	}
	for key, want := range map[int]string{
		int('i'): "+forward",
		int('k'): "+back",
		int('j'): "+moveleft",
		int('l'): "+moveright",
	} {
		if got := g.Input.Binding(key); got != want {
			t.Fatalf("custom binding %q = %q, want %q", input.KeyToString(key), got, want)
		}
	}
}

func TestCurrentZoomSpeedUsesCanonicalZoomSpeedCVar(t *testing.T) {
	g := New()
	if g.Host.CVar.Get("zoom_speed") == nil {
		g.Host.CVar.Register("zoom_speed", "8", cvar.FlagArchive, "")
	}

	g.Host.CVar.Set("zoom_speed", "12")
	t.Cleanup(func() {
		g.Host.CVar.Set("zoom_speed", "8")
	})

	if got := g.currentZoomSpeed(); got != 12 {
		t.Fatalf("currentZoomSpeed() = %v, want 12", got)
	}
}

func TestCurrentRuntimeFOVUsesCanonicalFOVCVar(t *testing.T) {
	g := New()
	if g.Host.CVar.Get("fov") == nil {
		g.Host.CVar.Register("fov", "90", cvar.FlagArchive, "")
	}

	g.Host.CVar.Set("fov", "110")
	t.Cleanup(func() {
		g.Host.CVar.Set("fov", "90")
	})

	if got := g.currentRuntimeFOV(); got != 110 {
		t.Fatalf("currentRuntimeFOV() = %v, want 110", got)
	}
}

func TestCurrentRuntimeViewSizeUsesCanonicalViewsizeCVar(t *testing.T) {
	g := New()
	g.registerMirroredArchiveCvars("viewsize", "scr_viewsize", "100", "")

	g.Host.CVar.Set("scr_viewsize", "100")
	g.Host.CVar.Set("viewsize", "130")
	t.Cleanup(func() {
		g.Host.CVar.Set("viewsize", "100")
		g.Host.CVar.Set("scr_viewsize", "100")
	})

	if got := g.currentRuntimeViewSize(); got != 130 {
		t.Fatalf("currentRuntimeViewSize() = %v, want 130", got)
	}
	if got := g.Host.CVar.FloatValue("scr_viewsize"); got != 130 {
		t.Fatalf("legacy scr_viewsize alias = %v, want 130", got)
	}
}

func TestCurrentRuntimeZoomFOVUsesCanonicalZoomFOVCVar(t *testing.T) {
	g := New()
	if g.Host.CVar.Get("zoom_fov") == nil {
		g.Host.CVar.Register("zoom_fov", "30", cvar.FlagArchive, "")
	}

	g.Host.CVar.Set("zoom_fov", "55")
	t.Cleanup(func() {
		g.Host.CVar.Set("zoom_fov", "30")
	})

	if got := g.currentRuntimeZoomFOV(); got != 55 {
		t.Fatalf("currentRuntimeZoomFOV() = %v, want 55", got)
	}
}

func TestCurrentRuntimeFOVAdaptUsesCanonicalFOVAdaptCVar(t *testing.T) {
	g := New()
	if g.Host.CVar.Get("fov_adapt") == nil {
		g.Host.CVar.Register("fov_adapt", "1", cvar.FlagArchive, "")
	}

	g.Host.CVar.Set("fov_adapt", "0")
	t.Cleanup(func() {
		g.Host.CVar.Set("fov_adapt", "1")
	})

	if got := g.currentRuntimeFOVAdapt(); got {
		t.Fatal("currentRuntimeFOVAdapt() = true, want false")
	}
}

func TestCurrentShowTurtlePrefersCanonicalShowturtleCVar(t *testing.T) {
	g := New()
	g.registerMirroredArchiveCvars("showturtle", "scr_showturtle", "0", "")

	g.Host.CVar.Set("scr_showturtle", "0")
	g.Host.CVar.Set("showturtle", "1")
	t.Cleanup(func() {
		g.Host.CVar.Set("showturtle", "0")
		g.Host.CVar.Set("scr_showturtle", "0")
	})

	if got := g.currentShowTurtle(); !got {
		t.Fatal("currentShowTurtle() = false, want true")
	}
	if got := g.Host.CVar.BoolValue("scr_showturtle"); !got {
		t.Fatal("legacy scr_showturtle alias did not mirror canonical showturtle")
	}
}

func TestRegisterColorShiftPercentCvarsRegistersDefaults(t *testing.T) {
	t.Parallel()

	g := New()
	registry := cvar.NewCVarSystem()
	g.registerColorShiftPercentCvars(registry.Register)

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

	g := New()
	registry := cvar.NewCVarSystem()
	g.registerRendererLightingAndParticleCvars(registry.Register)

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
	g := New()
	g.Client = cl.NewClient()
	g.Client.Stats[3] = 77
	g.Client.Stats[5] = 0xAB
	g.Client.StatsF[5] = 12.5
	g.Client.PlayerNames[1] = "Ranger"
	g.Client.Frags[1] = 42
	g.Client.PlayerColors[1] = 0x2d

	hooks := g.buildCSQCClientHooks()

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
	g := New()
	hooks := g.buildCSQCClientHooks()
	cmdName := "csqc_unit_registercommand_test"

	hooks.RegisterCommand(cmdName)
	if !g.Host.Cmd.Exists(cmdName) {
		t.Fatalf("command %q not registered", cmdName)
	}
	hooks.RegisterCommand(cmdName)
	if !g.Host.Cmd.Exists(cmdName) {
		t.Fatalf("command %q should remain registered", cmdName)
	}
}

type reloadTestRenderer struct{}

func (reloadTestRenderer) OnDraw(func(renderer.RenderContext))                    {}
func (reloadTestRenderer) OnUpdate(func(float64))                                 {}
func (reloadTestRenderer) RequestRedraw()                                         {}
func (reloadTestRenderer) StepWasmFrame(float64)                                  {}
func (reloadTestRenderer) Size() (int, int)                                       { return 320, 200 }
func (reloadTestRenderer) SetConfig(renderer.Config)                              {}
func (reloadTestRenderer) Run() error                                             { return nil }
func (reloadTestRenderer) Stop()                                                  {}
func (reloadTestRenderer) Shutdown()                                              {}
func (reloadTestRenderer) SetPalette([]byte)                                      {}
func (reloadTestRenderer) SetConchars([]byte)                                     {}
func (reloadTestRenderer) SetExternalSkybox(string, func(string) ([]byte, error)) {}
func (reloadTestRenderer) UploadPendingExternalSkybox() error                     { return nil }
func (reloadTestRenderer) UpdateCamera(renderer.CameraState, float32, float32)    {}
func (reloadTestRenderer) UploadWorld(*bsp.Tree) error                            { return nil }
func (reloadTestRenderer) HasWorldData() bool                                     { return false }
func (reloadTestRenderer) WorldBounds() (min types.Vec3, max types.Vec3, ok bool) {
	return types.Vec3{}, types.Vec3{}, false
}
func (reloadTestRenderer) PreloadBrushEntities([]renderer.BrushEntity)       {}
func (reloadTestRenderer) SpawnDynamicLight(renderer.DynamicLight) bool      { return false }
func (reloadTestRenderer) SpawnKeyedDynamicLight(renderer.DynamicLight) bool { return false }
func (reloadTestRenderer) UpdateLights(float32)                              {}
func (reloadTestRenderer) ClearDynamicLights()                               {}
func (reloadTestRenderer) InputBackendForSystem(*input.System) input.Backend { return nil }

func TestRuntimeMenuModsUsesCurrentSubsystemFilesystem(t *testing.T) {
	g := New()

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
	modsA := g.runtimeMenuMods(g.Subs)
	if len(modsA) != 1 || modsA[0].Name != "hipnotic" {
		t.Fatalf("mods from fsA = %#v, want hipnotic", modsA)
	}

	g.Subs.Files = fsB
	modsB := g.runtimeMenuMods(g.Subs)
	if len(modsB) != 1 || modsB[0].Name != "rogue" {
		t.Fatalf("mods from fsB = %#v, want rogue", modsB)
	}
}

func TestReloadRuntimeAfterGameDirChangeResetsSessionAndKeepsRenderer(t *testing.T) {
	g := New()

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
	g.Menu = menu.NewManager(nil, nil, nil)
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
	g.SpriteModelCache = map[string]*SpriteModel{"progs/flame.spr": nil}
	g.ShowScores = true
	g.WorldUploadKey = "old-world"

	if err := g.reloadRuntimeAfterGameDirChange(g.Subs, fileSys); err != nil {
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
	if g.Menu == nil || !g.Menu.IsActive() || g.Menu.State() != menu.MenuMain {
		t.Fatalf("menu state = active:%v state:%v, want active main menu", g.Menu != nil && g.Menu.IsActive(), g.Menu.State())
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
	g := New()

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

	if got := g.runtimeDrawFileSystem(fallback); got != current {
		t.Fatalf("runtimeDrawFileSystem() = %p, want current subsystem fs %p", got, current)
	}
}

// TestLoadRuntimeProgramsCompilesProgsWithNoAssets verifies the plan 22
// Phase A in-memory fallback directly against loadRuntimePrograms: with an
// EMPTY filesystem (no progs.dat — the wasm/no-assets case), the engine must
// compile its own QuakeGo sources in-memory instead of failing, and the VM
// must end up with functions loaded. Where in C: the game ships progs.dat;
// here the engine's sources ARE the mod.
func TestLoadRuntimeProgramsCompilesProgsWithNoAssets(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skips deterministic progs compile")
	}
	g := New()
	g.QC = qc.NewVM()

	empty := t.TempDir()
	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(empty, "id1"); err != nil {
		t.Fatalf("fs.Init: %v", err)
	}

	if err := g.loadRuntimePrograms(fileSys, 1); err != nil {
		t.Fatalf("loadRuntimePrograms on empty fs: %v", err)
	}
	if g.QC.FindFunction("StartFrame") < 0 {
		t.Fatal("StartFrame not found — progs was not loaded from in-memory compile")
	}
}

func TestRenderPassCvars(t *testing.T) {
	g := New()
	g.registerRenderPassCvars(g.Host.CVar.Register)

	renderer.SetGlobalPassFlags(renderer.PassAll)
	if !renderer.IsGlobalPassEnabled(renderer.PassSky) {
		t.Fatalf("expected sky pass enabled initially")
	}

	g.Host.CVar.Set("r_drawsky", "0")
	if renderer.IsGlobalPassEnabled(renderer.PassSky) {
		t.Errorf("expected sky pass disabled after setting r_drawsky 0")
	}

	g.Host.CVar.Set("r_drawsky", "1")
	if !renderer.IsGlobalPassEnabled(renderer.PassSky) {
		t.Errorf("expected sky pass enabled after setting r_drawsky 1")
	}

	g.Host.CVar.Set("r_passes", "0") // Should not clear if zero/invalid
	g.Host.CVar.Set("r_passes", strconv.Itoa(int(renderer.PassWorldOpaque|renderer.Pass2DOverlay)))
	if renderer.IsGlobalPassEnabled(renderer.PassSky) {
		t.Errorf("expected sky pass disabled by r_passes")
	}
	if !renderer.IsGlobalPassEnabled(renderer.PassWorldOpaque) {
		t.Errorf("expected world pass enabled by r_passes")
	}
	if !renderer.IsGlobalPassEnabled(renderer.Pass2DOverlay) {
		t.Errorf("expected overlay pass enabled by r_passes")
	}

	if cv := g.Host.CVar.Get(renderer.CvarRDumpPasses); cv == nil {
		t.Errorf("expected r_dump_passes to be registered")
	}

	if cv := g.Host.CVar.Get(renderer.CvarRPassIsolate); cv == nil {
		t.Errorf("expected r_pass_isolate to be registered")
	} else {
		g.Host.CVar.Set(renderer.CvarRPassIsolate, "2")
		if renderer.GetPassIsolateMode() != renderer.PassIsolateReveal {
			t.Errorf("expected pass isolate mode reveal, got %v", renderer.GetPassIsolateMode())
		}
		g.Host.CVar.Set(renderer.CvarRPassIsolate, "0")
		if renderer.GetPassIsolateMode() != renderer.PassIsolateNormal {
			t.Errorf("expected pass isolate mode normal, got %v", renderer.GetPassIsolateMode())
		}
	}

	// Reset to all
	renderer.SetGlobalPassFlags(renderer.PassAll)
}
