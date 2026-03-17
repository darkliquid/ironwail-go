package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/input"
)

func registerGameplayBindCommands() {
	cmdsys.AddCommand("bind", cmdBind, "Bind a key to a command")
	cmdsys.AddCommand("unbind", cmdUnbind, "Remove a key binding")
	cmdsys.AddCommand("unbindall", cmdUnbindAll, "Remove all key bindings")
	cmdsys.AddCommand("bindlist", cmdBindList, "List all key bindings")
	cmdsys.AddCommand("impulse", cmdImpulse, "Trigger an impulse command")
	cmdsys.AddCommand("toggleconsole", cmdToggleConsole, "Toggle the console")
	cmdsys.AddCommand("screenshot", cmdScreenshot, "Save a screenshot as PNG")
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

func applyDefaultGameplayBindings() {
	if g.Input == nil {
		return
	}
	for _, binding := range gameplayDefaultBindings {
		g.Input.SetBinding(binding.key, binding.command)
	}
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
