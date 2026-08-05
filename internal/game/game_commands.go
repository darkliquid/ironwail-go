package game

import (
	"bufio"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
)

var UIScaleCVarNames = []string{
	"scr_conscale",
	"scr_menuscale",
	"scr_sbarscale",
	"scr_crosshairscale",
}

var uiScaleCVarNames = UIScaleCVarNames

func (g *Game) registerGameplayBindCommands() {
	g.Host.Cmd.AddCommand("bind", g.cmdBind, "Bind a key to a command")
	g.Host.Cmd.AddCommand("unbind", g.cmdUnbind, "Remove a key binding")
	g.Host.Cmd.AddCommand("unbindall", g.cmdUnbindAll, "Remove all key bindings")
	g.Host.Cmd.AddCommand("bindlist", g.cmdBindList, "List all key bindings")
	g.Host.Cmd.AddCommand("scr_autoscale", g.cmdScreenAutoScale, "Set UI scale cvars based on the current framebuffer size")
	g.Host.Cmd.AddCommand("sizeup", g.cmdSizeUp, "Increase screen view size")
	g.Host.Cmd.AddCommand("sizedown", g.cmdSizeDown, "Decrease screen view size")
	g.Host.Cmd.AddCommand("entities", g.cmdEntities, "List current client entities")
	g.Host.Cmd.AddCommand("camdebug", g.cmdCamDebug, "Dump camera vs entity origin and interpolation state")
	g.Host.Cmd.AddCommand("viewpos_json", g.cmdViewposJSON, "Print current viewpoint formatted as viewpoints.json entry")
	g.Host.Cmd.AddCommand("camjson", g.cmdViewposJSON, "Alias for viewpos_json")
	g.Host.Cmd.AddCommand("impulse", g.cmdImpulse, "Trigger an impulse command")
	g.Host.Cmd.AddCommand("toggleconsole", g.cmdToggleConsole, "Toggle the console")
	g.Host.Cmd.AddCommand("screenshot", g.cmdScreenshot, "Save a screenshot as PNG")
	g.Host.Cmd.AddCommand("profile_cpu_start", g.cmdProfileCPUStart, "Start writing a CPU pprof capture to disk")
	g.Host.Cmd.AddCommand("profile_cpu_stop", g.cmdProfileCPUStop, "Stop the active CPU pprof capture and flush it to disk")
	g.Host.Cmd.AddCommand("profile_dump_heap", g.cmdProfileDumpHeap, "Write a heap pprof capture to disk")
	g.Host.Cmd.AddCommand("profile_dump_allocs", g.cmdProfileDumpAllocs, "Write an allocs pprof capture to disk")
	g.Host.Cmd.AddCommand("perf_warmup", g.cmdPerfWarmup, "Enter per-frame allocation/profile warmup phase")
	g.Host.Cmd.AddCommand("perf_capture", g.cmdPerfCapture, "Start steady-state per-frame measurement capture")
	g.Host.Cmd.AddCommand("perf_reset", g.cmdPerfReset, "Reset any active warmup/measurement session")
	g.Host.Cmd.AddCommand("vid_restart", func(args []string) {
		if err := g.restartVideo(); err != nil {
			console.Printf("vid_restart failed: %v\n", err)
		}
	}, "Restart the video system")
	g.Host.Cmd.AddCommand("messagemode", g.cmdMessagemode, "Input a message to say")
	g.Host.Cmd.AddCommand("messagemode2", g.cmdMessagemode2, "Input a message to say_team")
	g.Host.Cmd.AddCommand("+showscores", g.cmdShowScores, "Show multiplayer scoreboard while held")
	g.Host.Cmd.AddCommand("-showscores", g.cmdHideScores, "Hide multiplayer scoreboard")

	// bf: bonus flash – gold item-pickup screen tint stuffed by the server.
	// Mirrors C Ironwail: view.c V_BonusFlash_f().
	g.Host.Cmd.AddCommand("bf", func(args []string) {
		if g.Client != nil {
			g.Client.BonusFlash()
		}
	}, "Trigger bonus-pickup screen flash")

	g.Host.Cmd.AddCommand("centerview", func(args []string) {
		if g.Client != nil {
			g.Client.StartPitchDrift()
		}
	}, "Recenter pitch drift")

	// v_cshift: custom screen tint command (used by some QC mods).
	// Usage: v_cshift <r> <G> <b> <percent>  (all 0–255)
	// Mirrors C Ironwail: view.c V_cshift_f().
	g.Host.Cmd.AddCommand("v_cshift", func(args []string) {
		if g.Client == nil || len(args) < 5 {
			return
		}
		parseArg := func(s string) float32 {
			var v float64
			_, _ = fmt.Sscanf(s, "%f", &v)
			return float32(v)
		}
		g.Client.SetCustomShift(parseArg(args[1]), parseArg(args[2]), parseArg(args[3]), parseArg(args[4]))
	}, "Set custom screen color shift (r G b percent, 0–255)")

	g.registerGameplayButtonCommand("forward", func(c *cl.Client) *cl.KButton { return &c.InputForward })
	g.registerGameplayButtonCommand("back", func(c *cl.Client) *cl.KButton { return &c.InputBack })
	g.registerGameplayButtonCommand("moveleft", func(c *cl.Client) *cl.KButton { return &c.InputMoveLeft })
	g.registerGameplayButtonCommand("moveright", func(c *cl.Client) *cl.KButton { return &c.InputMoveRight })
	g.registerGameplayButtonCommand("left", func(c *cl.Client) *cl.KButton { return &c.InputLeft })
	g.registerGameplayButtonCommand("right", func(c *cl.Client) *cl.KButton { return &c.InputRight })
	g.registerGameplayButtonCommand("speed", func(c *cl.Client) *cl.KButton { return &c.InputSpeed })
	g.registerGameplayButtonCommand("strafe", func(c *cl.Client) *cl.KButton { return &c.InputStrafe })
	g.registerGameplayButtonCommand("attack", func(c *cl.Client) *cl.KButton { return &c.InputAttack })
	g.registerGameplayButtonCommand("jump", func(c *cl.Client) *cl.KButton { return &c.InputJump })
	g.registerGameplayButtonCommand("use", func(c *cl.Client) *cl.KButton { return &c.InputUse })
	g.registerGameplayButtonCommand("mlook", func(c *cl.Client) *cl.KButton { return &c.InputMLook })
	g.registerGameplayButtonCommand("klook", func(c *cl.Client) *cl.KButton { return &c.InputKLook })
	g.registerGameplayButtonCommand("lookup", func(c *cl.Client) *cl.KButton { return &c.InputLookUp })
	g.registerGameplayButtonCommand("lookdown", func(c *cl.Client) *cl.KButton { return &c.InputLookDown })
	g.registerGameplayButtonCommand("up", func(c *cl.Client) *cl.KButton { return &c.InputUp })
	g.registerGameplayButtonCommand("down", func(c *cl.Client) *cl.KButton { return &c.InputDown })
}

func (g *Game) registerConsoleCompletionProviders() {
	console.SetGlobalCommandProvider(g.Host.Cmd.Complete)
	console.SetGlobalCVarProvider(g.Host.CVar.Complete)
	console.SetGlobalAliasProvider(g.Host.Cmd.CompleteAliases)
	console.SetGlobalCommandArgsProvider(func(command string, args []string, partial string) []string {
		return g.Host.Cmd.CompleteCommandArgs(command, args, partial)
	})
	console.SetGlobalCVarValueProvider(func(cvarName string, partial string) []string {
		return g.Host.CVar.CompleteValue(cvarName, partial)
	})
	console.SetGlobalCompletionPrintFunc(console.Printf)
	if g.Subs != nil {
		if fileSys, ok := g.Subs.Files.(*fs.FileSystem); ok {
			console.SetGlobalFileProvider(fileSys.ListFiles)
			return
		}
	}
	console.SetGlobalFileProvider(nil)
}

func (g *Game) registerGameplayButtonCommand(name string, selectButton func(*cl.Client) *cl.KButton) {
	g.Host.Cmd.AddCommand("+"+name, func(args []string) {
		g.runGameplayButtonCommand(selectButton, true, args)
	}, "Gameplay button press")
	g.Host.Cmd.AddCommand("-"+name, func(args []string) {
		g.runGameplayButtonCommand(selectButton, false, args)
	}, "Gameplay button release")
}

func (g *Game) runGameplayButtonCommand(selectButton func(*cl.Client) *cl.KButton, down bool, args []string) {
	if g.Client == nil {
		return
	}
	key := -1
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil {
			key = parsed
		}
	}
	button := selectButton(g.Client)
	if down {
		g.Client.KeyDown(button, key)
		return
	}
	g.Client.KeyUp(button, key)
}

func (g *Game) currentAutoScaleFactor() float64 {
	width, height := 0, 0
	if g.Renderer != nil {
		width, height = g.Renderer.Size()
	}
	if width <= 0 {
		width = g.Host.CVar.IntValue("vid_width")
	}
	if height <= 0 {
		height = g.Host.CVar.IntValue("vid_height")
	}
	scaleW := float64(width) / 640.0
	scaleH := float64(height) / 480.0
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	if scale < 1 {
		return 1
	}
	return scale
}

func (g *Game) currentVideoCVarAutoScaleFactor() float64 {
	width := g.Host.CVar.IntValue("vid_width")
	height := g.Host.CVar.IntValue("vid_height")
	if width <= 0 || height <= 0 {
		return 1
	}
	scale := min(float64(width)/640.0, float64(height)/480.0)
	if scale < 1 {
		return 1
	}
	return scale
}

func (g *Game) cmdScreenAutoScale(_ []string) {
	scale := g.currentAutoScaleFactor()
	for _, name := range uiScaleCVarNames {
		g.Host.CVar.SetFloat(name, scale)
	}
}

const (
	minBoundViewSize = 30.0
	maxHUDViewSize   = 110.0
)

func (g *Game) boundedRuntimeViewSize(v float64) float64 {
	return g.clampf64(v, minBoundViewSize, maxHUDViewSize)
}

func (g *Game) cmdSizeUp(_ []string) {
	g.Host.CVar.SetFloat("scr_viewsize", g.boundedRuntimeViewSize(g.Host.CVar.FloatValue("scr_viewsize")+10))
}

func (g *Game) cmdSizeDown(_ []string) {
	g.Host.CVar.SetFloat("scr_viewsize", g.boundedRuntimeViewSize(g.Host.CVar.FloatValue("scr_viewsize")-10))
}

func (g *Game) cmdEntities(_ []string) {
	if g.Client == nil || g.Client.State == cl.StateDisconnected {
		return
	}

	maxEntity := -1
	for entityNum := range g.Client.Entities {
		if entityNum > maxEntity {
			maxEntity = entityNum
		}
	}
	if maxEntity < 0 {
		return
	}

	for entityNum := 0; entityNum <= maxEntity; entityNum++ {
		console.Printf("%3d:", entityNum)
		state, ok := g.Client.Entities[entityNum]
		modelName := ""
		if ok {
			modelName = g.clientEntityModelName(state)
		}
		if !ok || modelName == "" {
			console.Printf("EMPTY\n")
			continue
		}
		console.Printf("%s:%2d  (%5.1f,%5.1f,%5.1f) [%5.1f %5.1f %5.1f]\n",
			modelName,
			state.Frame,
			state.Origin[0], state.Origin[1], state.Origin[2],
			state.Angles[0], state.Angles[1], state.Angles[2],
		)
	}
}

func (g *Game) cmdCamDebug(_ []string) {
	if g.Client == nil {
		console.Printf("camdebug: no client\n")
		return
	}

	viewEnt := g.Client.ViewEntity
	if viewEnt == 0 {
		viewEnt = 1
	}

	state, ok := g.Client.Entities[viewEnt]
	if !ok {
		console.Printf("camdebug: view entity %d not in entity map!\n", viewEnt)
		return
	}

	console.Printf("=== camdebug frame=%d ===\n", g.Host.FrameCount())
	console.Printf("client.time=%.6f  oldtime=%.6f\n", g.Client.Time, g.Client.OldTime)
	console.Printf("mtime[0]=%.6f  mtime[1]=%.6f  delta=%.6f\n",
		g.Client.MTime[0], g.Client.MTime[1], g.Client.MTime[0]-g.Client.MTime[1])
	console.Printf("localServerFast=%v  noLerp=%v  demoPlayback=%v\n",
		g.Client.LocalServerFast, g.Client.NoLerp, g.Client.DemoPlayback)
	console.Printf("viewEntity=%d  onGround=%v  viewHeight=%.1f\n",
		viewEnt, g.Client.OnGround, g.Client.ViewHeight)
	console.Printf("localViewTeleport=%v\n", g.Client.LocalViewTeleport)

	console.Printf("entity.renderOrigin  = (%.2f, %.2f, %.2f)\n",
		state.Origin[0], state.Origin[1], state.Origin[2])
	console.Printf("entity.msgOrigin[0]  = (%.2f, %.2f, %.2f)\n",
		state.MsgOrigins[0][0], state.MsgOrigins[0][1], state.MsgOrigins[0][2])
	console.Printf("entity.msgOrigin[1]  = (%.2f, %.2f, %.2f)\n",
		state.MsgOrigins[1][0], state.MsgOrigins[1][1], state.MsgOrigins[1][2])
	console.Printf("entity.msgTime=%.6f  matchesMTime0=%v  modelIndex=%d  forceLink=%v\n",
		state.MsgTime, state.MsgTime == g.Client.MTime[0], state.ModelIndex, state.ForceLink)

	serverOrigin := [3]float32{}
	if g.Server != nil && g.Server.Edicts != nil {
		if ent := g.Server.Edicts[viewEnt]; ent != nil {
			serverOrigin = ent.Origin(g.Server)
		}
	}
	console.Printf("server.playerOrigin  = (%.2f, %.2f, %.2f)\n",
		serverOrigin[0], serverOrigin[1], serverOrigin[2])

	camOrigin, camAngles := g.runtimeViewState()
	console.Printf("camera.viewOrigin   = (%.2f, %.2f, %.2f)\n",
		camOrigin[0], camOrigin[1], camOrigin[2])
	console.Printf("camera.viewAngles   = (%.2f, %.2f, %.2f)\n",
		camAngles[0], camAngles[1], camAngles[2])

	dx := camOrigin[0] - state.Origin[0]
	dy := camOrigin[1] - state.Origin[1]
	dz := camOrigin[2] - state.Origin[2]
	console.Printf("delta(cam-entity)    = (%.2f, %.2f, %.2f)  dist=%.2f\n",
		dx, dy, dz, math.Sqrt(float64(dx*dx+dy*dy+dz*dz)))

	forward, _, _ := g.runtimeAngleVectors(camAngles)
	hit, ok := g.traceCrosshairFace(camOrigin, forward)
	if !ok {
		console.Printf("crosshair.face       = none (no geometry hit within 16384 units)\n")
	} else {
		console.Printf("crosshair.face       = #%d  dist=%.2f  hitPos=(%.2f, %.2f, %.2f)\n",
			hit.faceIndex, hit.distance, hit.hitPos[0], hit.hitPos[1], hit.hitPos[2])
		if hit.modelIndex > 0 {
			console.Printf("crosshair.entity     = edict #%d (submodel *%d)\n", hit.entIndex, hit.modelIndex)
		} else {
			console.Printf("crosshair.entity     = worldspawn (model 0)\n")
		}
		console.Printf("crosshair.plane      = #%d  side=%d  normal=(%.4f, %.4f, %.4f)  dist=%.2f\n",
			hit.planeIndex, hit.planeSide, hit.planeNormal[0], hit.planeNormal[1], hit.planeNormal[2], hit.planeDist)
		console.Printf("crosshair.texture    = %s (%dx%d)  type=%v  uv=(%.2f, %.2f)\n",
			hit.texName, hit.texWidth, hit.texHeight, hit.texType, hit.hitU, hit.hitV)
		console.Printf("crosshair.texinfo    = #%d  rawFlags=0x%X  derivedFlags=0x%X  edges=%d  lightOfs=%d  styles=%v\n",
			hit.texinfoIdx, hit.texFlags, hit.derivedFlags, hit.numEdges, hit.lightOfs, hit.styles)
	}
	console.Printf("=========================\n")
}

func (g *Game) startupConfigPinsAnyCVar(userDir string, names []string) bool {
	userDir = strings.TrimSpace(userDir)
	if userDir == "" || len(names) == 0 {
		return false
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	for _, filename := range []string{"ironwail.cfg", "config.cfg", "autoexec.cfg"} {
		path := filepath.Join(userDir, filename)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) == 0 {
				continue
			}
			if _, ok := allowed[fields[0]]; !ok {
				continue
			}
			if len(fields) < 2 {
				_ = f.Close()
				return true
			}
			value := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
			if unquoted, err := strconv.Unquote(value); err == nil {
				value = unquoted
			}
			if parsed, err := strconv.ParseFloat(strings.Fields(value)[0], 64); err == nil {
				if parsed == 1 {
					continue
				}
			}
			_ = f.Close()
			return true
		}
		_ = f.Close()
	}
	return false
}

func (g *Game) shouldBootstrapStartupUIScale() bool {
	if g.Renderer == nil || g.Host == nil {
		return false
	}
	actualScale := g.currentAutoScaleFactor()
	legacyScale := g.currentVideoCVarAutoScaleFactor()
	allMatchLegacy := legacyScale > 0
	for _, name := range uiScaleCVarNames {
		if math.Abs(g.Host.CVar.FloatValue(name)-legacyScale) > 0.0001 {
			allMatchLegacy = false
			break
		}
	}
	if allMatchLegacy && actualScale > legacyScale+0.0001 {
		return true
	}
	if g.startupConfigPinsAnyCVar(g.Host.UserDir(), uiScaleCVarNames) {
		return false
	}
	for _, name := range uiScaleCVarNames {
		if g.Host.CVar.FloatValue(name) != 1 {
			return false
		}
	}
	return true
}

func (g *Game) ensureStartupUIScale() {
	if g.shouldBootstrapStartupUIScale() {
		g.cmdScreenAutoScale(nil)
	}
}

func (g *Game) restartVideo() error {
	if g.Renderer == nil {
		return nil
	}

	if g.Input != nil {
		if backend := g.Input.Backend(); backend != nil {
			backend.Shutdown()
		}
	}
	g.Renderer.Shutdown()

	if err := g.initGameRenderer(); err != nil {
		return err
	}

	if g.Input != nil {
		if backend := g.Renderer.InputBackendForSystem(g.Input); backend != nil {
			if err := g.Input.SetBackend(backend); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Game) applyDefaultGameplayBindings() {
	if g.Input == nil {
		return
	}
	for _, binding := range gameplayDefaultBindings {
		g.Input.SetBinding(binding.Key, binding.Command)
	}
}

func (g *Game) hasAnyGameplayBindings() bool {
	if g.Input == nil {
		return false
	}
	for key := 0; key < input.NumKeycode; key++ {
		if strings.TrimSpace(g.Input.Binding(key)) != "" {
			return true
		}
	}
	return false
}

func (g *Game) hasBindingForCommand(command string) bool {
	if g.Input == nil {
		return false
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return false
	}
	for key := 0; key < input.NumKeycode; key++ {
		if strings.TrimSpace(g.Input.Binding(key)) == command {
			return true
		}
	}
	return false
}

func (g *Game) ensureEssentialFallbackBindings() {
	if g.Input == nil {
		return
	}
	for _, binding := range essentialFallbackBindings {
		if g.hasBindingForCommand(binding.Command) {
			continue
		}
		g.Input.SetBinding(binding.Key, binding.Command)
	}
}

func (g *Game) ensureRequiredGameplayBindings() {
	if g.Input == nil {
		return
	}
	for _, command := range startupRequiredGameplayCommands {
		if g.hasBindingForCommand(command) {
			continue
		}
		for _, binding := range gameplayDefaultBindings {
			if binding.Command == command {
				g.Input.SetBinding(binding.Key, binding.Command)
			}
		}
	}
}

func (g *Game) ensureGameplayBindings() {
	if g.Input != nil {
		for _, binding := range gameplayDefaultBindings {
			if strings.TrimSpace(g.Input.Binding(binding.Key)) == "" && !g.hasBindingForCommand(binding.Command) {
				g.Input.SetBinding(binding.Key, binding.Command)
			}
		}
	}
	g.ensureRequiredGameplayBindings()
	g.ensureEssentialFallbackBindings()
}

func (g *Game) keyDestName(dest input.KeyDest) string {
	switch dest {
	case input.KeyGame:
		return "game"
	case input.KeyConsole:
		return "console"
	case input.KeyMessage:
		return "message"
	case input.KeyMenu:
		return "menu"
	default:
		return fmt.Sprintf("unknown(%d)", dest)
	}
}

func (g *Game) logStartupInputDiagnostics() {
	if g.Input == nil {
		return
	}
	bindings := make([]string, 0, len(essentialFallbackBindings)+4)
	missingActions := make([]string, 0, len(essentialFallbackBindings))
	diagnosticBindings := []KeyBinding{
		{Key: input.KEscape, Command: "togglemenu"},
		{Key: int('`'), Command: "toggleconsole"},
		{Key: input.KUpArrow, Command: "+forward"},
		{Key: input.KDownArrow, Command: "+back"},
		{Key: input.KLeftArrow, Command: "+left"},
		{Key: input.KRightArrow, Command: "+right"},
	}
	for _, binding := range diagnosticBindings {
		keyName := input.KeyToString(binding.Key)
		if keyName == "" {
			keyName = fmt.Sprintf("KEY%d", binding.Key)
		}
		command := strings.TrimSpace(g.Input.Binding(binding.Key))
		bindings = append(bindings, fmt.Sprintf("%s=%q", keyName, command))
		if !g.hasBindingForCommand(binding.Command) {
			missingActions = append(missingActions, binding.Command)
		}
	}
	backendType := "<nil>"
	if backend := g.Input.Backend(); backend != nil {
		backendType = fmt.Sprintf("%T", backend)
	}
	menuActive := g.Menu != nil && g.Menu.IsActive()
	menuState := "none"
	if g.Menu != nil {
		menuState = fmt.Sprintf("%v", g.Menu.State())
	}
	slog.Debug("startup input diagnostics",
		"menu_active", menuActive,
		"menu_state", menuState,
		"key_dest", g.keyDestName(g.Input.KeyDest()),
		"backend", backendType,
		"bindings", strings.Join(bindings, ", "),
		"missing_actions", strings.Join(missingActions, ", "),
		"menu_raw_keys", "confirm=ENTER|SPACE|MOUSE1,start=A|START cancel=ESCAPE|BACKSPACE|MOUSE2|B arrows=UP|DOWN|LEFT|RIGHT",
	)
}

func (g *Game) parseBindingKey(name string) (int, bool) {
	key := input.StringToKey(strings.ToUpper(name))
	if key <= 0 || key >= input.NumKeycode {
		return 0, false
	}
	return key, true
}

func (g *Game) cmdBind(args []string) {
	if g.Input == nil {
		return
	}
	if len(args) < 1 {
		console.Printf("usage: bind <key> [command]\n")
		return
	}
	key, ok := g.parseBindingKey(args[0])
	if !ok {
		console.Printf("bind: \"%s\" is not a valid key\n", args[0])
		return
	}
	if len(args) == 1 {
		binding := g.Input.Binding(key)
		if binding == "" {
			console.Printf("\"%s\" is not bound\n", args[0])
		} else {
			console.Printf("\"%s\" = \"%s\"\n", args[0], binding)
		}
		return
	}
	g.Input.SetBinding(key, strings.Join(args[1:], " "))
}

func (g *Game) cmdUnbind(args []string) {
	if g.Input == nil {
		return
	}
	if len(args) != 1 {
		console.Printf("usage: unbind <key>\n")
		return
	}
	key, ok := g.parseBindingKey(args[0])
	if !ok {
		console.Printf("unbind: \"%s\" is not a valid key\n", args[0])
		return
	}
	g.Input.SetBinding(key, "")
}

func (g *Game) cmdUnbindAll(_ []string) {
	if g.Input == nil {
		return
	}
	for key := 0; key < input.NumKeycode; key++ {
		g.Input.SetBinding(key, "")
	}
}

func (g *Game) cmdBindList(_ []string) {
	if g.Input == nil {
		return
	}
	count := 0
	for key := 0; key < input.NumKeycode; key++ {
		binding := g.Input.Binding(key)
		if binding == "" {
			continue
		}
		keyName := input.KeyToString(key)
		if keyName == "" {
			keyName = strconv.Itoa(key)
		}
		console.Printf("\"%s\" = \"%s\"\n", keyName, binding)
		count++
	}
	console.Printf("%d bindings\n", count)
}

func (g *Game) cmdImpulse(args []string) {
	if g.Client == nil {
		return
	}
	if len(args) < 1 {
		console.Printf("usage: impulse <value>\n")
		return
	}
	impulse, err := strconv.Atoi(args[0])
	if err != nil {
		console.Printf("impulse: \"%s\" is not a number\n", args[0])
		return
	}
	g.Client.InImpulse = impulse
}

func (g *Game) cmdToggleConsole(_ []string) {
	if g.Input == nil {
		return
	}

	if g.Input.KeyDest() == input.KeyConsole {
		console.ResetCompletion()
		if g.runtimeConsoleForcedUp() && g.Menu != nil {
			g.showRuntimeMenuState(menu.MenuMain)
			g.syncGameplayInputMode()
			return
		}
		g.Input.SetKeyDest(input.KeyGame)
		g.syncGameplayInputMode()
		return
	}

	if g.Menu != nil && g.Menu.IsActive() {
		g.Menu.HideMenu()
	}
	console.ResetCompletion()
	g.Input.SetKeyDest(input.KeyConsole)
	g.syncGameplayInputMode()
}

func (g *Game) cmdScreenshot(args []string) {
	if len(args) > 1 {
		console.Printf("usage: screenshot [filename]\n")
		return
	}

	filename := ""
	if len(args) == 1 {
		filename = strings.TrimSpace(args[0])
	}
	if filename == "" {
		filename = fmt.Sprintf("ironwail_%s.png", time.Now().Format("20060102_150405"))
	}

	baseDir := "."
	if g.Host != nil && strings.TrimSpace(g.Host.BaseDir()) != "" {
		baseDir = g.Host.BaseDir()
	}
	modDir := strings.TrimSpace(g.ModDir)
	if modDir == "" {
		modDir = "id1"
	}

	outputPath := filename
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(baseDir, modDir, outputPath)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		console.Printf("screenshot: create output directory: %v\n", err)
		return
	}

	if err := g.CaptureScreenshot(outputPath, baseDir, modDir); err != nil {
		console.Printf("screenshot failed: %v\n", err)
		return
	}
}

func (g *Game) cmdShowScores(_ []string) {
	if g.Client == nil {
		return
	}
	g.ShowScores = true
}

func (g *Game) cmdHideScores(_ []string) {
	g.ShowScores = false
}

func (g *Game) cmdMessagemode(_ []string) {
	if g.Input == nil {
		return
	}
	g.chatBuffer = ""
	g.chatTeam = false
	g.Input.SetKeyDest(input.KeyMessage)
}

func (g *Game) cmdMessagemode2(_ []string) {
	if g.Input == nil {
		return
	}
	g.chatBuffer = ""
	g.chatTeam = true
	g.Input.SetKeyDest(input.KeyMessage)
}
