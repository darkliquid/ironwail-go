package ui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// GUIDimensions calculates effective GUI dimensions taking pixel aspect ratio into account.
func GUIDimensions(framebufferW, framebufferH, defaultVidW, defaultVidH int, pixelAspect float64) (int, int) {
	guiW := framebufferW
	guiH := framebufferH
	if guiW <= 0 {
		guiW = defaultVidW
	}
	if guiH <= 0 {
		guiH = defaultVidH
	}
	if pixelAspect > 1 {
		guiW = int(float64(guiW)/pixelAspect + 0.5)
	} else if pixelAspect > 0 && pixelAspect < 1 {
		guiH = int(float64(guiH)*pixelAspect + 0.5)
	}
	return guiW, guiH
}

// ConsoleDimensions calculates console dimensions clamped between 320 and guiW, matching FitzQuake rules.
func ConsoleDimensions(guiW, guiH int, overrideConWidth, scaleConScale float64) (int, int) {
	if guiW <= 0 || guiH <= 0 {
		return 0, 0
	}
	conWidth := guiW
	if overrideConWidth > 0 {
		conWidth = int(overrideConWidth)
	} else if scaleConScale > 0 {
		conWidth = int(float64(guiW) / scaleConScale)
	}
	if conWidth < 320 {
		conWidth = 320
	}
	if conWidth > guiW {
		conWidth = guiW
	}
	conWidth &^= 7
	if conWidth <= 0 {
		conWidth = guiW
	}
	conHeight := conWidth * guiH / guiW
	if conHeight <= 0 {
		conHeight = guiH
	}
	return conWidth, conHeight
}

// CanvasParams calculates canvas transformation parameters for 2D overlay rendering passes.
func CanvasParams(framebufferW, framebufferH, defaultVidW, defaultVidH int, pixelAspect float64, overrideConWidth, scaleConScale, menuScale, sbarScale, crosshairScale float64, slideFraction float32) renderer.CanvasTransformParams {
	guiW, guiH := GUIDimensions(framebufferW, framebufferH, defaultVidW, defaultVidH, pixelAspect)
	conW, conH := ConsoleDimensions(guiW, guiH, overrideConWidth, scaleConScale)
	return renderer.CanvasTransformParams{
		GUIWidth:         float32(guiW),
		GUIHeight:        float32(guiH),
		GLWidth:          float32(framebufferW),
		GLHeight:         float32(framebufferH),
		ConWidth:         float32(conW),
		ConHeight:        float32(conH),
		MenuScale:        float32(menuScale),
		SbarScale:        float32(sbarScale),
		CrosshairScale:   float32(crosshairScale),
		ConSlideFraction: slideFraction,
	}
}

// StepConsoleSlide calculates the new slide fraction based on speed, frame delta time, and target fraction.
func StepConsoleSlide(currentFraction float32, speed float32, frameTime float64, targetFraction float32) (newFraction float32, animating bool) {
	if speed <= 0 {
		speed = 2.0
	}
	step := float32(float64(speed) * frameTime)
	if currentFraction < targetFraction {
		currentFraction += step
		if currentFraction >= targetFraction {
			currentFraction = targetFraction
			animating = false
		} else {
			animating = true
		}
	} else if currentFraction > targetFraction {
		currentFraction -= step
		if currentFraction <= targetFraction {
			currentFraction = targetFraction
			animating = false
		} else {
			animating = true
		}
	} else {
		animating = false
	}
	if currentFraction < 0 {
		currentFraction = 0
	}
	if currentFraction > 1 {
		currentFraction = 1
	}
	return currentFraction, animating
}

// ClampF64 clamps v into [min, max].
func ClampF64(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// DemoName strips the directory and extension from a demo path, truncating
// the base name to 30 characters for display.
func DemoName(name string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if len(base) > 30 {
		base = base[:30]
	}
	return base
}

// FormatDemoBaseSpeed renders a demo playback speed multiplier for display:
// "2x" for >=1x, "1/2x" for fractions, "" for zero.
func FormatDemoBaseSpeed(speed float32) string {
	if speed == 0 {
		return ""
	}
	absSpeed := math.Abs(float64(speed))
	if absSpeed >= 1 {
		return fmt.Sprintf("%gx", absSpeed)
	}
	return fmt.Sprintf("1/%gx", 1/absSpeed)
}
