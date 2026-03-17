package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/ironwail/ironwail-go/internal/audio"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/hud"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/menu"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/renderer"
	"github.com/ironwail/ironwail-go/internal/server"
)

func initGameHost() error {
	// Initialize console and command system
	console.InitGlobal(0)

	// Initialize cvars for video, sound, gameplay
	cvar.Register("vid_width", "1280", cvar.FlagArchive, "Video width")
	cvar.Register("vid_height", "720", cvar.FlagArchive, "Video height")
	cvar.Register("vid_fullscreen", "0", cvar.FlagArchive, "Fullscreen mode (0=windowed, 1=fullscreen)")
	cvar.Register("vid_vsync", "1", cvar.FlagArchive, "Vertical sync")
	cvar.Register("host_maxfps", "250", cvar.FlagArchive, "Maximum frames per second")
	sVolume := cvar.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	sVolume.Callback = func(*cvar.CVar) {
		applySVolume()
	}
	cvar.Register("r_gamma", "1.0", cvar.FlagArchive, "Gamma correction")
	cvar.Register("r_drawviewmodel", "1", cvar.FlagArchive, "Draw first-person viewmodel")
	cvar.Register("v_gunkick", "2", 0, "Gun kick style (0=off, 1=instant, 2=interpolated)")
	cvar.Register(renderer.CvarRSkyFog, "0.5", cvar.FlagArchive, "Sky fog mix factor (0..1)")
	cvar.Register(renderer.CvarRShadows, "1", cvar.FlagArchive, "Enable entity shadows (0=off, 1=on)")
	cvar.Register(renderer.CvarRNoshadowList, "progs/eyes.mdl", cvar.FlagArchive, "Space-separated list of model names to exclude from shadows")
	// r_waterwarp: 0=off, 1=screen-space sinusoidal warp, 2=FOV oscillation.
	// Mirrors C Ironwail r_waterwarp. Default 1 (screen-space warp).
	cvar.Register(renderer.CvarRWaterwarp, "1", cvar.FlagArchive, "Underwater warp effect (0=off, 1=screen warp, 2=FOV warp)")
	// gl_polyblend: enable/disable the v_blend polyblend screen-tint pass.
	// Mirrors C Ironwail gl_polyblend. Default 1 (enabled).
	cvar.Register("gl_polyblend", "1", cvar.FlagArchive, "Enable polyblend screen-tint overlay (damage flash, powerups, etc.)")
	// gl_cshiftpercent: global scale for all color shifts (0–100).
	// Mirrors C Ironwail gl_cshiftpercent. Default 100 (full intensity).
	cvar.Register("gl_cshiftpercent", "100", cvar.FlagArchive, "Global color-shift intensity percentage (0–100)")
	cvar.Register("developer", "0", 0, "Developer mode")

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
	// scr_viewsize: screen view size percentage (100 = full), used by
	// r_viewmodel_quake fudge.
	cvar.Register("scr_viewsize", "100", cvar.FlagArchive, "Screen view size percentage")
	crosshair := cvar.Register("crosshair", "0", cvar.FlagArchive, "Crosshair style (0=off, 1='+', >1=dot, <0=custom char index)")
	crosshair.Callback = func(cv *cvar.CVar) {
		if gameHUD != nil {
			gameHUD.UpdateCrosshair(cv.Float)
		}
	}
	cvar.Register("scr_crosshairscale", "1", cvar.FlagArchive, "Crosshair scale factor (1-10)")
	registerControlCvars()

	// Create host instance
	gameHost = host.NewHost()

	return nil
}

func registerControlCvars() {
	alwaysRun := cvar.Register("cl_alwaysrun", "1", cvar.FlagArchive, "Always run movement by default")
	freelook := cvar.Register("freelook", "1", cvar.FlagArchive, "Enable mouse freelook")
	lookspring := cvar.Register("lookspring", "0", cvar.FlagArchive, "Center view when look key released")
	cvar.Register("lookstrafe", "0", cvar.FlagArchive, "Use mouse X for strafing when +strafe held")
	cvar.Register("sensitivity", "6.8", cvar.FlagArchive, "Mouse sensitivity scale")
	cvar.Register("m_pitch", "0.0176", cvar.FlagArchive, "Mouse pitch scale")
	cvar.Register("m_yaw", "0.022", cvar.FlagArchive, "Mouse yaw scale")
	cvar.Register("m_forward", "1", cvar.FlagArchive, "Mouse forward scale")
	cvar.Register("m_side", "0.8", cvar.FlagArchive, "Mouse side scale")
	for _, cv := range []*cvar.CVar{alwaysRun, freelook, lookspring} {
		cv.Callback = func(*cvar.CVar) {
			syncControlCvarsToClient()
		}
	}
}

func syncControlCvarsToClient() {
	if gameClient == nil {
		return
	}
	gameClient.AlwaysRun = cvar.BoolValue("cl_alwaysrun")
	gameClient.FreeLook = cvar.BoolValue("freelook")
	gameClient.LookSpring = cvar.BoolValue("lookspring")
}

func initGameServer() error {
	// Create server instance
	gameServer = server.NewServer()

	return nil
}

func initGameQC() error {
	// Create QC VM instance
	gameQC = qc.NewVM()
	// slog.Info("QC loaded") - moved to main for deterministic logs

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
	gameRenderer = tr

	return nil
}

func preferWaylandForGoGPU() {
	if runtime.GOOS != "linux" {
		return
	}

	if strings.EqualFold(os.Getenv("IW_INPUT_BACKEND"), "sdl3") {
		return
	}

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return
	}

	if os.Getenv("DISPLAY") != "" {
		slog.Warn("Using X11 backend; gogpu X11 keyboard events are currently not implemented")
	}
}

func initSubsystems(headless, dedicated bool, basedir, gamedir string, args []string) error {
	gameModDir = strings.ToLower(strings.TrimSpace(gamedir))
	gameInput = nil
	gameDraw = nil
	gameMenu = nil
	gameHUD = nil

	// Initialize base input system (used for binds/console routing even in dedicated mode).
	gameInput = input.NewSystem(nil) // No backend yet - will be set by renderer when available.
	if err := gameInput.Init(); err != nil {
		return fmt.Errorf("failed to init input system: %w", err)
	}

	if !dedicated {
		// Initialize draw manager
		gameDraw = draw.NewManager()

		// Initialize menu system
		gameMenu = menu.NewManager(gameDraw, gameInput)
		gameMenu.SetSoundPlayer(playMenuSound)

		// Set up menu input callbacks
		gameInput.OnMenuKey = handleMenuKeyEvent
		gameInput.OnMenuChar = handleMenuCharEvent
		gameInput.OnKey = handleGameKeyEvent
		gameInput.OnChar = handleGameCharEvent
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

	// Load progs.dat into QC VM
	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		return fmt.Errorf("failed to load progs.dat: %w", err)
	}
	if err := gameQC.LoadProgs(bytes.NewReader(progsData)); err != nil {
		return fmt.Errorf("failed to parse progs.dat: %w", err)
	}
	// Link the builtins and set up entity sizes
	qc.RegisterBuiltins(gameQC)

	if err := initGameServer(); err != nil {
		return err
	}
	// Link QC VM into the server
	gameServer.QCVM = gameQC

	if !headless {
		if err := initGameRenderer(); err != nil {
			return err
		}
	}

	// If renderer was created, wire its input backend into the input system
	if !dedicated && gameRenderer != nil && gameInput != nil {
		// Some renderers provide a backend factory to adapt window events
		// to the engine input system.
		if bb := gameRenderer.InputBackendForSystem(gameInput); bb != nil {
			if err := gameInput.SetBackend(bb); err != nil {
				return fmt.Errorf("failed to set renderer input backend: %w", err)
			}
		}
	}

	// Optional override to force SDL3 input even when renderer backend exists.
	// Useful when platform-specific window backends do not emit keyboard events.
	if !dedicated && gameInput != nil && strings.EqualFold(os.Getenv("IW_INPUT_BACKEND"), "sdl3") {
		previousBackend := gameInput.Backend()
		if b := input.NewSDL3Backend(gameInput); b != nil {
			if err := gameInput.SetBackend(b); err != nil {
				slog.Warn("failed to force SDL3 input backend; keeping previous backend", "error", err)
				if previousBackend != nil {
					if restoreErr := gameInput.SetBackend(previousBackend); restoreErr != nil {
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
	if !dedicated && gameInput != nil {
		if err := func() error {
			// Only set SDL3 backend if renderer didn't provide one
			if gameInput.Backend() != nil {
				return nil
			}
			if b := input.NewSDL3Backend(gameInput); b != nil {
				return gameInput.SetBackend(b)
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
	gameAudio = audioAdapter
	resetRuntimeSoundState()
	gameSubs = &host.Subsystems{
		Files:    fileSys,
		Commands: globalCommandBuffer{},
		Server:   gameServer,
		Input:    gameInput,
		Audio:    audioAdapter,
	}
	// Wire the loopback client to the server so server→client messages are parsed (M3).
	host.SetupLoopbackClientServer(gameSubs, gameServer)
	registerGameplayBindCommands()
	registerConsoleCompletionProviders()
	applyDefaultGameplayBindings()

	if err := gameHost.Init(&host.InitParams{
		BaseDir:      basedir,
		GameDir:      gamedir,
		UserDir:      "",
		Args:         append([]string(nil), args...),
		MaxClients:   1,
		VersionMajor: VersionMajor,
		VersionMinor: VersionMinor,
		VersionPatch: VersionPatch,
	}, gameSubs); err != nil {
		return fmt.Errorf("failed to initialize host: %w", err)
	}
	applySVolume()

	// Set menu in host
	gameHost.SetMenu(gameMenu)
	if gameMenu != nil {
		gameMenu.SetSaveSlotProvider(func(slotCount int) []menu.SaveSlotInfo {
			hostSlots := gameHost.ListSaveSlots(slotCount)
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
		gameMenu.SetModsProvider(func() []menu.ModInfo {
			mods := fileSys.ListMods()
			out := make([]menu.ModInfo, 0, len(mods))
			for _, m := range mods {
				out = append(out, menu.ModInfo{Name: m.Name})
			}
			return out
		})
		gameMenu.SetCurrentMod(gameModDir)
	}

	// Initialize draw manager from the game filesystem (loads gfx.wad from pak files)
	if gameDraw != nil {
		drawErr := gameDraw.Init(fileSys)
		if drawErr != nil {
			// Fall back to local "data" directory for development/testing
			slog.Warn("Failed to initialize draw manager from filesystem, trying data/", "error", drawErr)
			drawErr = gameDraw.InitFromDir("data")
		}
		if drawErr != nil {
			slog.Warn("Failed to initialize draw manager", "error", drawErr)
		} else if gameRenderer != nil {
			if pal := gameDraw.Palette(); len(pal) >= 768 {
				gameRenderer.SetPalette(pal)
			}
			if conchars := gameDraw.GetConcharsData(); len(conchars) >= 128*128 {
				gameRenderer.SetConchars(conchars)
			}
		}

		// Initialize HUD
		gameHUD = hud.NewHUD(gameDraw)
		gameHUD.UpdateCrosshair(cvar.FloatValue("crosshair"))
	}
	gameClient = host.ActiveClientState(gameSubs)
	syncControlCvarsToClient()
	resetRuntimeVisualState()

	// Wire ModelFlagsFunc callback for EF_ROTATE support
	if gameClient != nil {
		gameClient.ModelFlagsFunc = func(modelName string) int {
			if mdl, ok := loadAliasModel(modelName); ok && mdl != nil {
				return mdl.Flags
			}
			return 0
		}
	}

	// Make sure the menu is visible at startup
	if gameMenu != nil {
		gameMenu.ShowMenu()
	}
	// slog.Info("menu active") - moved to main for deterministic logs

	slog.Info("All subsystems initialized")
	return nil
}
