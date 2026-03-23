package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/input"
)

var vidRestartFunc = restartVideo

var uiScaleCVarNames = []string{
	"scr_conscale",
	"scr_menuscale",
	"scr_sbarscale",
	"scr_crosshairscale",
}

func registerGameplayBindCommands() {
	cmdsys.AddCommand("bind", cmdBind, "Bind a key to a command")
	cmdsys.AddCommand("unbind", cmdUnbind, "Remove a key binding")
	cmdsys.AddCommand("unbindall", cmdUnbindAll, "Remove all key bindings")
	cmdsys.AddCommand("bindlist", cmdBindList, "List all key bindings")
	cmdsys.AddCommand("scr_autoscale", cmdScreenAutoScale, "Set UI scale cvars based on the current framebuffer size")
	cmdsys.AddCommand("sizeup", cmdSizeUp, "Increase screen view size")
	cmdsys.AddCommand("sizedown", cmdSizeDown, "Decrease screen view size")
	cmdsys.AddCommand("impulse", cmdImpulse, "Trigger an impulse command")
	cmdsys.AddCommand("toggleconsole", cmdToggleConsole, "Toggle the console")
	cmdsys.AddCommand("screenshot", cmdScreenshot, "Save a screenshot as PNG")
	cmdsys.AddCommand("vid_restart", func(args []string) {
		if err := vidRestartFunc(); err != nil {
			console.Printf("vid_restart failed: %v\n", err)
		}
	}, "Restart the video system")
	cmdsys.AddCommand("messagemode", cmdMessagemode, "Input a message to say")
	cmdsys.AddCommand("messagemode2", cmdMessagemode2, "Input a message to say_team")
	cmdsys.AddCommand("+showscores", cmdShowScores, "Show multiplayer scoreboard while held")
	cmdsys.AddCommand("-showscores", cmdHideScores, "Hide multiplayer scoreboard")

	// bf: bonus flash – gold item-pickup screen tint stuffed by the server.
	// Mirrors C Ironwail: view.c V_BonusFlash_f().
	cmdsys.AddCommand("bf", func(args []string) {
		if g.Client != nil {
			g.Client.BonusFlash()
		}
	}, "Trigger bonus-pickup screen flash")

	// v_cshift: custom screen tint command (used by some QC mods).
	// Usage: v_cshift <r> <g> <b> <percent>  (all 0–255)
	// Mirrors C Ironwail: view.c V_cshift_f().
	cmdsys.AddCommand("v_cshift", func(args []string) {
		if g.Client == nil || len(args) < 5 {
			return
		}
		parseArg := func(s string) float32 {
			var v float64
			fmt.Sscanf(s, "%f", &v)
			return float32(v)
		}
		g.Client.SetCustomShift(parseArg(args[1]), parseArg(args[2]), parseArg(args[3]), parseArg(args[4]))
	}, "Set custom screen color shift (r g b percent, 0–255)")

	registerGameplayButtonCommand("forward", func(c *cl.Client) *cl.KButton { return &c.InputForward })
	registerGameplayButtonCommand("back", func(c *cl.Client) *cl.KButton { return &c.InputBack })
	registerGameplayButtonCommand("moveleft", func(c *cl.Client) *cl.KButton { return &c.InputMoveLeft })
	registerGameplayButtonCommand("moveright", func(c *cl.Client) *cl.KButton { return &c.InputMoveRight })
	registerGameplayButtonCommand("left", func(c *cl.Client) *cl.KButton { return &c.InputLeft })
	registerGameplayButtonCommand("right", func(c *cl.Client) *cl.KButton { return &c.InputRight })
	registerGameplayButtonCommand("speed", func(c *cl.Client) *cl.KButton { return &c.InputSpeed })
	registerGameplayButtonCommand("strafe", func(c *cl.Client) *cl.KButton { return &c.InputStrafe })
	registerGameplayButtonCommand("attack", func(c *cl.Client) *cl.KButton { return &c.InputAttack })
	registerGameplayButtonCommand("jump", func(c *cl.Client) *cl.KButton { return &c.InputJump })
	registerGameplayButtonCommand("use", func(c *cl.Client) *cl.KButton { return &c.InputUse })
	registerGameplayButtonCommand("mlook", func(c *cl.Client) *cl.KButton { return &c.InputMLook })
	registerGameplayButtonCommand("klook", func(c *cl.Client) *cl.KButton { return &c.InputKLook })
	registerGameplayButtonCommand("lookup", func(c *cl.Client) *cl.KButton { return &c.InputLookUp })
	registerGameplayButtonCommand("lookdown", func(c *cl.Client) *cl.KButton { return &c.InputLookDown })
	registerGameplayButtonCommand("up", func(c *cl.Client) *cl.KButton { return &c.InputUp })
	registerGameplayButtonCommand("down", func(c *cl.Client) *cl.KButton { return &c.InputDown })
}

func registerConsoleCompletionProviders() {
	console.SetGlobalCommandProvider(cmdsys.Complete)
	console.SetGlobalCVarProvider(cvar.Complete)
	console.SetGlobalAliasProvider(cmdsys.CompleteAliases)
	if g.Subs != nil {
		if fileSys, ok := g.Subs.Files.(*fs.FileSystem); ok {
			console.SetGlobalFileProvider(fileSys.ListFiles)
			return
		}
	}
	console.SetGlobalFileProvider(nil)
}

func registerGameplayButtonCommand(name string, selectButton func(*cl.Client) *cl.KButton) {
	cmdsys.AddCommand("+"+name, func(args []string) {
		runGameplayButtonCommand(selectButton, true, args)
	}, "Gameplay button press")
	cmdsys.AddCommand("-"+name, func(args []string) {
		runGameplayButtonCommand(selectButton, false, args)
	}, "Gameplay button release")
}

func runGameplayButtonCommand(selectButton func(*cl.Client) *cl.KButton, down bool, args []string) {
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

func currentAutoScaleFactor() float64 {
	width, height := 0, 0
	if g.Renderer != nil {
		width, height = g.Renderer.Size()
	}
	if width <= 0 {
		width = cvar.IntValue("vid_width")
	}
	if height <= 0 {
		height = cvar.IntValue("vid_height")
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

func currentVideoCVarAutoScaleFactor() float64 {
	width := cvar.IntValue("vid_width")
	height := cvar.IntValue("vid_height")
	if width <= 0 || height <= 0 {
		return 1
	}
	scale := min(float64(width)/640.0, float64(height)/480.0)
	if scale < 1 {
		return 1
	}
	return scale
}

func cmdScreenAutoScale(_ []string) {
	scale := currentAutoScaleFactor()
	for _, name := range uiScaleCVarNames {
		cvar.SetFloat(name, scale)
	}
}

func cmdSizeUp(_ []string) {
	cvar.SetFloat("scr_viewsize", cvar.FloatValue("scr_viewsize")+10)
}

func cmdSizeDown(_ []string) {
	cvar.SetFloat("scr_viewsize", cvar.FloatValue("scr_viewsize")-10)
}

func startupConfigPinsAnyCVar(userDir string, names []string) bool {
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

func shouldBootstrapStartupUIScale() bool {
	if g.Renderer == nil || g.Host == nil {
		return false
	}
	actualScale := currentAutoScaleFactor()
	legacyScale := currentVideoCVarAutoScaleFactor()
	allMatchLegacy := legacyScale > 0
	for _, name := range uiScaleCVarNames {
		if math.Abs(cvar.FloatValue(name)-legacyScale) > 0.0001 {
			allMatchLegacy = false
			break
		}
	}
	if allMatchLegacy && actualScale > legacyScale+0.0001 {
		return true
	}
	if startupConfigPinsAnyCVar(g.Host.UserDir(), uiScaleCVarNames) {
		return false
	}
	for _, name := range uiScaleCVarNames {
		if cvar.FloatValue(name) != 1 {
			return false
		}
	}
	return true
}

func ensureStartupUIScale() {
	if shouldBootstrapStartupUIScale() {
		cmdScreenAutoScale(nil)
	}
}

func restartVideo() error {
	if g.Renderer == nil {
		return nil
	}

	if g.Input != nil {
		if backend := g.Input.Backend(); backend != nil {
			backend.Shutdown()
		}
	}
	g.Renderer.Shutdown()

	if err := initGameRenderer(); err != nil {
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

func applyDefaultGameplayBindings() {
	if g.Input == nil {
		return
	}
	for _, binding := range gameplayDefaultBindings {
		g.Input.SetBinding(binding.key, binding.command)
	}
}

func hasAnyGameplayBindings() bool {
	if g.Input == nil {
		return false
	}
	for key := 0; key < input.NumKeycode; key++ {
		if strings.TrimSpace(g.Input.GetBinding(key)) != "" {
			return true
		}
	}
	return false
}

func ensureGameplayBindings() {
	if hasAnyGameplayBindings() {
		return
	}
	applyDefaultGameplayBindings()
}

func parseBindingKey(name string) (int, bool) {
	key := input.StringToKey(strings.ToUpper(name))
	if key <= 0 || key >= input.NumKeycode {
		return 0, false
	}
	return key, true
}

func cmdBind(args []string) {
	if g.Input == nil {
		return
	}
	if len(args) < 1 {
		console.Printf("usage: bind <key> [command]\n")
		return
	}
	key, ok := parseBindingKey(args[0])
	if !ok {
		console.Printf("bind: \"%s\" is not a valid key\n", args[0])
		return
	}
	if len(args) == 1 {
		binding := g.Input.GetBinding(key)
		if binding == "" {
			console.Printf("\"%s\" is not bound\n", args[0])
		} else {
			console.Printf("\"%s\" = \"%s\"\n", args[0], binding)
		}
		return
	}
	g.Input.SetBinding(key, strings.Join(args[1:], " "))
}

func cmdUnbind(args []string) {
	if g.Input == nil {
		return
	}
	if len(args) != 1 {
		console.Printf("usage: unbind <key>\n")
		return
	}
	key, ok := parseBindingKey(args[0])
	if !ok {
		console.Printf("unbind: \"%s\" is not a valid key\n", args[0])
		return
	}
	g.Input.SetBinding(key, "")
}

func cmdUnbindAll(_ []string) {
	if g.Input == nil {
		return
	}
	for key := 0; key < input.NumKeycode; key++ {
		g.Input.SetBinding(key, "")
	}
}

func cmdBindList(_ []string) {
	if g.Input == nil {
		return
	}
	count := 0
	for key := 0; key < input.NumKeycode; key++ {
		binding := g.Input.GetBinding(key)
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

func cmdImpulse(args []string) {
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

func cmdToggleConsole(_ []string) {
	if g.Input == nil {
		return
	}

	if g.Input.GetKeyDest() == input.KeyConsole {
		console.ResetCompletion()
		g.Input.SetKeyDest(input.KeyGame)
		syncGameplayInputMode()
		return
	}

	if g.Menu != nil && g.Menu.IsActive() {
		g.Menu.HideMenu()
	}
	console.ResetCompletion()
	g.Input.SetKeyDest(input.KeyConsole)
	syncGameplayInputMode()
}

func cmdScreenshot(args []string) {
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

	if err := captureScreenshot(outputPath, baseDir, modDir); err != nil {
		console.Printf("screenshot failed: %v\n", err)
		return
	}
}

func cmdShowScores(_ []string) {
	if g.Client == nil {
		return
	}
	g.ShowScores = true
}

func cmdHideScores(_ []string) {
	g.ShowScores = false
}

// Global chat state shared with main.go
var (
	chatBuffer string
	chatTeam   bool
)

func cmdMessagemode(_ []string) {
	if g.Input == nil {
		return
	}
	chatBuffer = ""
	chatTeam = false
	g.Input.SetKeyDest(input.KeyMessage)
}

func cmdMessagemode2(_ []string) {
	if g.Input == nil {
		return
	}
	chatBuffer = ""
	chatTeam = true
	g.Input.SetKeyDest(input.KeyMessage)
}
