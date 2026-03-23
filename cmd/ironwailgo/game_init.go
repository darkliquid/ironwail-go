package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/audio"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/hud"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/menu"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/renderer"
	"github.com/ironwail/ironwail-go/internal/server"
)

type globalConsoleAdapter struct{}

var (
	startupVidWidth  = 1280
	startupVidHeight = 720
)

func (globalConsoleAdapter) Init() error                { return nil }
func (globalConsoleAdapter) Print(msg string)           { console.Printf("%s", msg) }
func (globalConsoleAdapter) Clear()                     { console.Clear() }
func (globalConsoleAdapter) Dump(filename string) error { return nil }
func (globalConsoleAdapter) Shutdown()                  { console.Close() }

func registerMirroredArchiveCvars(canonicalName, legacyName, defaultValue, description string) *cvar.CVar {
	canonical := cvar.Register(canonicalName, defaultValue, cvar.FlagArchive, description)
	legacy := cvar.Register(legacyName, canonical.String, cvar.FlagArchive, description+" (legacy alias)")

	canonicalCallback := canonical.Callback
	legacyCallback := legacy.Callback

	canonical.Callback = func(cv *cvar.CVar) {
		if legacy.String != cv.String {
			cvar.Set(legacy.Name, cv.String)
		}
		if canonicalCallback != nil {
			canonicalCallback(cv)
		}
	}
	legacy.Callback = func(cv *cvar.CVar) {
		if canonical.String != cv.String {
			cvar.Set(canonical.Name, cv.String)
		}
		if legacyCallback != nil {
			legacyCallback(cv)
		}
	}

	return canonical
}

func registerColorShiftPercentCvars(register func(name, defaultValue string, flags cvar.CVarFlags, desc string) *cvar.CVar) {
	register("gl_cshiftpercent", "100", cvar.FlagArchive, "Global color-shift intensity percentage (0–100)")
	register("gl_cshiftpercent_contents", "100", cvar.FlagArchive, "Contents color-shift intensity percentage (0–100)")
	register("gl_cshiftpercent_damage", "100", cvar.FlagArchive, "Damage color-shift intensity percentage (0–100)")
	register("gl_cshiftpercent_bonus", "100", cvar.FlagArchive, "Bonus color-shift intensity percentage (0–100)")
	register("gl_cshiftpercent_powerup", "100", cvar.FlagArchive, "Powerup color-shift intensity percentage (0–100)")
}

func initGameHost() error {
	fmt.Printf("Detected %d CPUs.\n", runtime.NumCPU())
	fmt.Println("Host_Init")
	fmt.Println()

	// Initialize console and command system
	console.InitGlobal(0)
	console.SetPrintCallback(func(msg string) {
		fmt.Print(msg)
	})
	cmdsys.SetPrintCallback(func(msg string) {
		console.Printf("%s", msg)
	})

	// Initialize cvars for video, sound, gameplay
	cvar.Register("vid_width", strconv.Itoa(startupVidWidth), cvar.FlagArchive, "Video width")
	cvar.Register("vid_height", strconv.Itoa(startupVidHeight), cvar.FlagArchive, "Video height")
	cvar.Register("vid_fullscreen", "0", cvar.FlagArchive, "Fullscreen mode (0=windowed, 1=fullscreen)")
	cvar.Register("vid_vsync", "1", cvar.FlagArchive, "Vertical sync")
	cvar.Register("host_maxfps", "250", cvar.FlagArchive, "Maximum frames per second")
	sVolume := cvar.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	sVolume.Callback = func(*cvar.CVar) {
		applySVolume()
	}
	cvar.Register("r_gamma", "1.0", cvar.FlagArchive, "Gamma correction")
	cvar.Register(renderer.CvarRAlphaSort, "1", cvar.FlagArchive, "Sort translucent surfaces back-to-front")
	cvar.Register(renderer.CvarROIT, "1", cvar.FlagArchive, "Enable order-independent transparency")
	cvar.Register("r_drawentities", "1", 0, "Draw entities")
	cvar.Register("r_drawviewmodel", "1", cvar.FlagArchive, "Draw first-person viewmodel")
	cvar.Register("v_gunkick", "2", 0, "Gun kick style (0=off, 1=instant, 2=interpolated)")
	cvar.Register(renderer.CvarRSkyFog, "0.5", cvar.FlagArchive, "Sky fog mix factor (0..1)")
	cvar.Register(renderer.CvarRShadows, "1", cvar.FlagArchive, "Enable entity shadows (0=off, 1=on)")
	cvar.Register(renderer.CvarRNoshadowList, "progs/eyes.mdl", cvar.FlagArchive, "Space-separated list of model names to exclude from shadows")
	// r_waterwarp: 0=off, 1=screen-space sinusoidal warp, 2=FOV oscillation.
	// Mirrors C Ironwail r_waterwarp. Default 1 (screen-space warp).
	cvar.Register(renderer.CvarRWaterwarp, "1", cvar.FlagArchive, "Underwater warp effect (0=off, 1=screen warp, 2=FOV warp)")
	cvar.Register(renderer.CvarRLitWater, "1", cvar.FlagArchive, "Enable lightmapped water when map has lit water data (0=off, 1=on)")
	// gl_polyblend: enable/disable the v_blend polyblend screen-tint pass.
	// Mirrors C Ironwail gl_polyblend. Default 1 (enabled).
	cvar.Register("gl_polyblend", "1", cvar.FlagArchive, "Enable polyblend screen-tint overlay (damage flash, powerups, etc.)")
	// gl_cshiftpercent and gl_cshiftpercent_*: global/per-channel scales for color shifts (0–100).
	// Mirror C Ironwail defaults (all 100 = full intensity).
	registerColorShiftPercentCvars(cvar.Register)
	cvar.Register("developer", "0", 0, "Developer mode")
	registerDebugViewTelemetryCVar()

	// View-bob cvars (V_CalcBob).
	cvar.Register("cl_bob", "0.02", cvar.FlagArchive, "View bobbing scale")
	cvar.Register("cl_bobcycle", "0.6", 0, "View bobbing cycle length in seconds")
	cvar.Register("cl_bobup", "0.5", 0, "Fraction of bob cycle spent moving upward")

	// View-roll cvars (V_CalcViewRoll).
	cvar.Register("cl_rollangle", "2.0", cvar.FlagArchive, "Camera roll angle when strafing")
	cvar.Register("cl_rollspeed", "200", 0, "Lateral speed at which full roll is applied")

	// View kick effects (V_ParseDamage damage kick).
	cvar.Register("v_kicktime", "0.5", 0, "Duration of damage kick effect")
	cvar.Register("v_kickroll", "0.6", 0, "Damage kick roll intensity")
	cvar.Register("v_kickpitch", "0.6", 0, "Damage kick pitch intensity")

	// Idle-sway cvars (V_AddIdle / CalcGunAngle).
	cvar.Register("v_idlescale", "0", 0, "Idle sway scale (0 = off)")
	cvar.Register("v_iyaw_cycle", "2", 0, "Idle sway yaw cycle frequency")
	cvar.Register("v_iroll_cycle", "0.5", 0, "Idle sway roll cycle frequency")
	cvar.Register("v_ipitch_cycle", "1", 0, "Idle sway pitch cycle frequency")
	cvar.Register("v_iyaw_level", "0.3", 0, "Idle sway yaw amplitude")
	cvar.Register("v_iroll_level", "0.1", 0, "Idle sway roll amplitude")
	cvar.Register("v_ipitch_level", "0.3", 0, "Idle sway pitch amplitude")

	// r_viewmodel_quake: origin fudge for different view sizes.
	cvar.Register("r_viewmodel_quake", "0", 0, "Apply Quake-style viewmodel origin fudge based on scr_viewsize")
	cvar.Register("chase_active", "0", 0, "Enable third-person chase camera")
	cvar.Register("chase_back", "100", cvar.FlagArchive, "Chase camera distance behind player")
	cvar.Register("chase_up", "16", cvar.FlagArchive, "Chase camera height above player")
	cvar.Register("chase_right", "0", cvar.FlagArchive, "Chase camera right offset")
	// viewsize: screen view size percentage (100 = full), used by
	// r_viewmodel_quake fudge. Keep scr_viewsize as a legacy alias.
	registerMirroredArchiveCvars("viewsize", "scr_viewsize", "100", "Screen view size percentage")
	cvar.Register("scr_sbarscale", "1", cvar.FlagArchive, "Status bar scale multiplier")
	cvar.Register("scr_sbaralpha", "0.75", cvar.FlagArchive, "Status bar background alpha")
	cvar.Register("scr_menuscale", "1", cvar.FlagArchive, "Menu scale multiplier")
	cvar.Register("scr_pixelaspect", "1", cvar.FlagArchive, "GUI pixel aspect ratio (float or width:height)")
	cvar.Register("scr_conwidth", "0", cvar.FlagArchive, "Console virtual width (0 = auto)")
	cvar.Register("scr_conscale", "1", cvar.FlagArchive, "Console scale factor")
	cvar.Register("scr_conspeed", "300", cvar.FlagArchive, "Console slide speed")
	cvar.Register("con_notifytime", "3", cvar.FlagArchive, "Notify line lifetime in seconds")
	cvar.Register("con_notifycenter", "0", cvar.FlagArchive, "Center notify lines over the gameplay view")
	cvar.Register("scr_showfps", "0", cvar.FlagArchive, "Show FPS counter in the corner (negative values show frame time in ms)")
	registerMirroredArchiveCvars("showturtle", "scr_showturtle", "0", "Show the turtle icon when frame time is very slow")
	cvar.Register("scr_showspeed", "0", cvar.FlagArchive, "Show horizontal player speed near the crosshair")
	cvar.Register("scr_showspeed_ofs", "0", cvar.FlagArchive, "Vertical offset for the speed readout")
	cvar.Register("scr_demobar_timeout", "1", cvar.FlagArchive, "Seconds to show the demo controls overlay after speed changes (0 = always, <0 = never)")
	cvar.Register("scr_clock", "0", cvar.FlagArchive, "Show level clock in the corner")
	cvar.Register("fov", "90", cvar.FlagArchive, "Horizontal field of view")
	cvar.Register("fov_adapt", "1", cvar.FlagArchive, "Adapt horizontal field of view to the window aspect ratio")
	cvar.Register("zoom_fov", "30", cvar.FlagArchive, "Target field of view while zoomed")
	cvar.Register("scr_centertime", "2", 0, "Regular centerprint hold time in seconds")
	cvar.Register("scr_centerprintbg", "2", cvar.FlagArchive, "Centerprint background style (0=off, 1=text box, 2=panel, 3=strip)")
	cvar.Register("zoom_speed", "8", cvar.FlagArchive, "Zoom transition speed")
	cvar.Register("scr_printspeed", "8", 0, "Finale/cutscene centerprint reveal speed in characters per second")
	cvar.Register("scr_menubgalpha", "0.7", cvar.FlagArchive, "Menu background fade alpha")
	cvar.Register("con_notifyfade", "0", cvar.FlagArchive, "Enable notify-style fade tail for centerprints")
	cvar.Register("con_notifyfadetime", "0.5", cvar.FlagArchive, "Centerprint fade-tail duration in seconds when con_notifyfade is enabled")
	crosshair := cvar.Register("crosshair", "0", cvar.FlagArchive, "Crosshair style (0=off, 1='+', >1=dot, <0=custom char index)")
	crosshair.Callback = func(cv *cvar.CVar) {
		if g.HUD != nil {
			g.HUD.UpdateCrosshair(cv.Float)
		}
	}
	cvar.Register("showpause", "1", cvar.FlagArchive, "Show pause overlay")
	cvar.Register("scr_crosshairscale", "1", cvar.FlagArchive, "Crosshair scale factor (1-10)")
	registerControlCvars()

	// Create host instance
	g.Host = host.NewHost()
	hostMaxFPS := cvar.Get("host_maxfps")
	if hostMaxFPS != nil {
		hostMaxFPS.Callback = func(cv *cvar.CVar) {
			if g.Host != nil {
				g.Host.SetMaxFPS(cv.Float)
			}
		}
		g.Host.SetMaxFPS(hostMaxFPS.Float)
	}

	return nil
}

func registerControlCvars() {
	alwaysRun := cvar.Register("cl_alwaysrun", "1", cvar.FlagArchive, "Always run movement by default")
	freelook := cvar.Register("freelook", "1", cvar.FlagArchive, "Enable mouse freelook")
	lookspring := cvar.Register("lookspring", "0", cvar.FlagArchive, "Center view when look key released")
	noLerp := cvar.Register("cl_nolerp", "0", cvar.FlagArchive, "Disable view interpolation")
	centerMove := cvar.Register("v_centermove", "0.15", 0, "Seconds of forward movement before pitch drift recenters the view")
	centerSpeed := cvar.Register("v_centerspeed", "500", 0, "Pitch drift recenter acceleration speed")
	cvar.Register("lookstrafe", "0", cvar.FlagArchive, "Use mouse X for strafing when +strafe held")
	cvar.Register("sensitivity", "6.8", cvar.FlagArchive, "Mouse sensitivity scale")
	cvar.Register("m_pitch", "0.0176", cvar.FlagArchive, "Mouse pitch scale")
	cvar.Register("m_yaw", "0.022", cvar.FlagArchive, "Mouse yaw scale")
	cvar.Register("m_forward", "1", cvar.FlagArchive, "Mouse forward scale")
	cvar.Register("m_side", "0.8", cvar.FlagArchive, "Mouse side scale")
	for _, cv := range []*cvar.CVar{alwaysRun, freelook, lookspring, noLerp, centerMove, centerSpeed} {
		cv.Callback = func(*cvar.CVar) {
			syncControlCvarsToClient()
		}
	}
}

func syncControlCvarsToClient() {
	if g.Client == nil {
		return
	}
	g.Client.AlwaysRun = cvar.BoolValue("cl_alwaysrun")
	g.Client.FreeLook = cvar.BoolValue("freelook")
	g.Client.LookSpring = cvar.BoolValue("lookspring")
	g.Client.NoLerp = cvar.BoolValue("cl_nolerp")
	g.Client.CenterMove = float32(cvar.FloatValue("v_centermove"))
	g.Client.CenterSpeed = float32(cvar.FloatValue("v_centerspeed"))
}

func initGameServer() error {
	if err := inet.Init(); err != nil {
		return fmt.Errorf("failed to initialize networking: %w", err)
	}
	console.Printf("UDP Initialized\n")

	// Create server instance
	g.Server = server.NewServer()
	console.Printf("Server using protocol %d (%s)\n", g.Server.Protocol, serverProtocolName(g.Server.Protocol))

	return nil
}

func serverProtocolName(protocol int) string {
	switch protocol {
	case server.ProtocolNetQuake:
		return "NetQuake"
	case server.ProtocolFitzQuake:
		return "FitzQuake"
	case server.ProtocolRMQ:
		return "RMQ"
	default:
		return "Unknown"
	}
}

func initGameQC() error {
	// The authoritative server VM is owned by server.NewServer(). Keep only
	// the client-side VM here so app init uses the same QCVM path as host/server
	// tests instead of swapping in a parallel VM later.
	g.QC = nil
	g.CSQC = qc.NewCSQC()
	// slog.Info("QC loaded") - moved to main for deterministic logs

	// Register server and CSQC builtins with their respective VMs.
	qc.RegisterBuiltins(g.CSQC.VM)
	qc.SetCSQCClientHooks(qc.CSQCClientHooks{
		PrecacheModel: func(name string) int {
			if g.CSQC == nil {
				return 0
			}
			return g.CSQC.PrecacheModel(name)
		},
		PrecacheSound: func(name string) int {
			if g.CSQC == nil {
				return 0
			}
			return g.CSQC.PrecacheSound(name)
		},
	})

	return nil
}

func initGameRenderer() error {
	preferWaylandForGoGPU()

	// Create renderer instance from cvars
	cfg := renderer.ConfigFromCvars()

	tr, err := renderer.NewWithConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}
	g.Renderer = tr

	return nil
}

func preferWaylandForGoGPU() {
	if !shouldWarnAboutGoGPUX11Keyboard(runtime.GOOS, os.Getenv("IW_INPUT_BACKEND"), os.Getenv("WAYLAND_DISPLAY"), os.Getenv("DISPLAY")) {
		return
	}

	sdl3Available := input.NewSDL3Backend(nil) != nil
	slog.Info(
		"Using X11 backend; gogpu falls back to polling-based keyboard input",
		"display_server", "x11",
		"keyboard_input_mode", "polling",
		"preferred_keyboard_input_mode", "event-driven",
		"sdl3_input_available", sdl3Available,
		"hint", gogpuX11KeyboardHint(sdl3Available),
	)
}

func shouldWarnAboutGoGPUX11Keyboard(goos, requestedInputBackend, waylandDisplay, x11Display string) bool {
	if goos != "linux" {
		return false
	}
	if strings.EqualFold(requestedInputBackend, "sdl3") {
		return false
	}
	if waylandDisplay != "" {
		return false
	}
	return x11Display != ""
}

func gogpuX11KeyboardHint(sdl3Available bool) string {
	if sdl3Available {
		return "set IW_INPUT_BACKEND=sdl3 for event-driven keyboard input on X11"
	}
	return "rebuild with `mise run build-gogpu-sdl3` or run under Wayland for event-driven keyboard input"
}

func initSubsystems(headless, dedicated bool, maxClients int, basedir, gamedir string, args []string) error {
	g.ModDir = strings.ToLower(strings.TrimSpace(gamedir))
	g.Input = nil
	g.Draw = nil
	g.Menu = nil
	g.HUD = nil

	// Initialize base input system (used for binds/console routing even in dedicated mode).
	g.Input = input.NewSystem(nil) // No backend yet - will be set by renderer when available.
	if err := g.Input.Init(); err != nil {
		return fmt.Errorf("failed to init input system: %w", err)
	}

	if !dedicated {
		// Initialize draw manager
		g.Draw = draw.NewManager()

		// Initialize menu system
		g.Menu = menu.NewManager(g.Draw, g.Input)
		g.Menu.SetSoundPlayer(playMenuSound)

		// Set up menu input callbacks
		g.Input.OnMenuKey = handleMenuKeyEvent
		g.Input.OnMenuChar = handleMenuCharEvent
		g.Input.OnKey = handleGameKeyEvent
		g.Input.OnChar = handleGameCharEvent
	}

	if err := initGameHost(); err != nil {
		return err
	}
	// Initialize filesystem
	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(basedir, gamedir); err != nil {
		return fmt.Errorf("failed to init filesystem: %w", err)
	}
	// slog.Info("FS mounted") - moved to main for deterministic logs

	// Initialize QuakeC VM
	if err := initGameQC(); err != nil {
		return err
	}

	if err := initGameServer(); err != nil {
		return err
	}

	// Use the server-owned VM so app startup matches the direct host/server path.
	g.QC = g.Server.QCVM
	if g.QC == nil {
		return fmt.Errorf("server QC VM not initialized")
	}

	// Load progs.dat into QC VM
	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		return fmt.Errorf("failed to load progs.dat: %w", err)
	}
	if err := g.QC.LoadProgs(bytes.NewReader(progsData)); err != nil {
		return fmt.Errorf("failed to parse progs.dat: %w", err)
	}
	// Link the builtins and set up entity sizes
	qc.RegisterBuiltins(g.QC)

	if !headless {
		startupUserDir, err := host.ResolveUserDir(basedir, "")
		if err != nil {
			return err
		}
		if err := host.LoadArchivedCvars(startupUserDir, []string{
			"vid_width",
			"vid_height",
			"vid_fullscreen",
			"vid_vsync",
			"host_maxfps",
			"r_gamma",
		}); err != nil {
			return fmt.Errorf("failed to load startup video cvars: %w", err)
		}
		if err := initGameRenderer(); err != nil {
			return err
		}
	}

	// If renderer was created, wire its input backend into the input system
	if !dedicated && g.Renderer != nil && g.Input != nil {
		// Some renderers provide a backend factory to adapt window events
		// to the engine input system.
		if bb := g.Renderer.InputBackendForSystem(g.Input); bb != nil {
			if err := g.Input.SetBackend(bb); err != nil {
				return fmt.Errorf("failed to set renderer input backend: %w", err)
			}
		}
	}

	// Optional override to force SDL3 input even when renderer backend exists.
	// Useful when platform-specific window backends do not emit keyboard events.
	if !dedicated && g.Input != nil && strings.EqualFold(os.Getenv("IW_INPUT_BACKEND"), "sdl3") {
		previousBackend := g.Input.Backend()
		if b := input.NewSDL3Backend(g.Input); b != nil {
			if err := g.Input.SetBackend(b); err != nil {
				slog.Warn("failed to force SDL3 input backend; keeping previous backend", "error", err)
				if previousBackend != nil {
					if restoreErr := g.Input.SetBackend(previousBackend); restoreErr != nil {
						return fmt.Errorf("failed to restore previous input backend after SDL3 override failure: %w", restoreErr)
					}
				}
			} else {
				slog.Warn("input backend override active", "backend", "sdl3")
			}
		} else {
			slog.Warn("IW_INPUT_BACKEND=sdl3 requested but SDL3 backend is not available in this build")
		}
	}

	// If no backend was provided by the renderer, allow other build-tagged
	// backends (e.g. SDL3) to provide system input. input.NewSDL3Backend
	// is a no-op stub when the sdl3 build tag is not present.
	if !dedicated && g.Input != nil {
		if err := func() error {
			// Only set SDL3 backend if renderer didn't provide one
			if g.Input.Backend() != nil {
				return nil
			}
			if b := input.NewSDL3Backend(g.Input); b != nil {
				return g.Input.SetBackend(b)
			}
			return nil
		}(); err != nil {
			return fmt.Errorf("failed to set input backend: %w", err)
		}
	}

	// Wire subsystems together through Host.Init
	var audioAdapter *audio.AudioAdapter
	if !dedicated {
		audioAdapter = audio.NewAudioAdapter(audio.NewSystem())
	}
	// Audio init is deferred to host.Init to avoid double-initialization.
	g.Audio = audioAdapter
	resetRuntimeSoundState()
	g.Subs = &host.Subsystems{
		Files:    fileSys,
		Commands: globalCommandBuffer{},
		Console:  globalConsoleAdapter{},
		Server:   g.Server,
		Input:    g.Input,
		Audio:    audioAdapter,
	}
	if g.Renderer != nil {
		g.Subs.Renderer = renderer.NewRendererAdapter(g.Renderer)
	}
	// Wire the loopback client to the server so server→client messages are parsed (M3).
	if !dedicated {
		host.SetupLoopbackClientServer(g.Subs, g.Server)
	}
	registerGameplayBindCommands()
	registerConsoleCompletionProviders()
	applyDefaultGameplayBindings()

	if err := g.Host.Init(&host.InitParams{
		BaseDir:      basedir,
		GameDir:      gamedir,
		UserDir:      "",
		Args:         append([]string(nil), args...),
		MaxClients:   maxClients,
		Dedicated:    dedicated,
		VersionMajor: VersionMajor,
		VersionMinor: VersionMinor,
		VersionPatch: VersionPatch,
	}, g.Subs); err != nil {
		return fmt.Errorf("failed to initialize host: %w", err)
	}
	if !dedicated {
		ensureStartupUIScale()
		ensureGameplayBindings()
	}
	applySVolume()

	// Set menu in host
	g.Host.SetMenu(g.Menu)
	if g.Menu != nil {
		g.Menu.SetSaveSlotProvider(func(slotCount int) []menu.SaveSlotInfo {
			hostSlots := g.Host.ListSaveSlots(slotCount)
			menuSlots := make([]menu.SaveSlotInfo, 0, len(hostSlots))
			for _, slot := range hostSlots {
				menuSlots = append(menuSlots, menu.SaveSlotInfo{
					Name:        slot.Name,
					DisplayName: slot.DisplayName,
				})
			}
			return menuSlots
		})
		// Wire mod enumeration and current-mod tracking into the menu.
		g.Menu.SetModsProvider(func() []menu.ModInfo {
			mods := fileSys.ListMods()
			out := make([]menu.ModInfo, 0, len(mods))
			for _, m := range mods {
				out = append(out, menu.ModInfo{Name: m.Name})
			}
			return out
		})
		g.Menu.SetCurrentMod(g.ModDir)
	}

	// Initialize draw manager from the game filesystem (loads gfx.wad from pak files)
	if g.Draw != nil {
		drawErr := g.Draw.Init(fileSys)
		if drawErr != nil {
			// Fall back to local "data" directory for development/testing
			slog.Warn("Failed to initialize draw manager from filesystem, trying data/", "error", drawErr)
			drawErr = g.Draw.InitFromDir("data")
		}
		if drawErr != nil {
			slog.Warn("Failed to initialize draw manager", "error", drawErr)
		} else if g.Renderer != nil {
			if pal := g.Draw.Palette(); len(pal) >= 768 {
				g.Renderer.SetPalette(pal)
			}
			if conchars := g.Draw.GetConcharsData(); len(conchars) >= 128*128 {
				g.Renderer.SetConchars(conchars)
			}
		}

		// Initialize HUD
		g.HUD = hud.NewHUD(g.Draw)
		g.HUD.UpdateCrosshair(cvar.FloatValue("crosshair"))
	}
	g.Client = host.ActiveClientState(g.Subs)
	syncControlCvarsToClient()
	resetRuntimeVisualState()

	// Wire ModelFlagsFunc callback for EF_ROTATE support
	if g.Client != nil {
		g.Client.ModelFlagsFunc = func(modelName string) int {
			if mdl, ok := loadAliasModel(modelName); ok && mdl != nil {
				return mdl.Flags
			}
			return 0
		}
	}

	// Make sure the menu is visible at startup
	if g.Menu != nil {
		g.Menu.ShowMenu()
	}
	// slog.Info("menu active") - moved to main for deterministic logs

	slog.Info("All subsystems initialized")
	return nil
}
