package host

import (
	"fmt"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/client"
)

// WindowTitleSetter is implemented by renderers that expose a
// runtime window title hook. The host probes subs.Renderer for this
// optional interface on each UpdateWindowTitle tick and skips the
// update cleanly when a backend doesn't support it (e.g. the headless
// test adapter). This mirrors C Ironwail's VID_SetWindowTitle, which
// is only wired on SDL backends.
type WindowTitleSetter interface {
	SetWindowTitle(title string)
}

// DefaultWindowTitle is used when no level is loaded.
const DefaultWindowTitle = "Ironwail-Go"

// updateTitleInterval matches UpdateWindowTitle's 0.125s cadence in
// host.c so we don't thrash the compositor when stats change rapidly.
const updateTitleInterval = 0.125

// windowTitleState tracks the last emitted title so we only call
// SetWindowTitle when the computed value actually changes.
type windowTitleState struct {
	timeleft float64
	lastMap  string
	lastSkil int
	lastTtl  string
}

// computeWindowTitle formats a window title from the active server
// map name and skill. Mirrors the minimal shape of UpdateWindowTitle
// from host.c (map + skill + engine name); kill/secret stats will
// be plumbed in once the per-client summary struct is available.
func computeWindowTitle(levelName, mapName string, skill int) string {
	mapName = strings.TrimSpace(mapName)
	levelName = strings.TrimSpace(levelName)
	if mapName == "" && levelName == "" {
		return DefaultWindowTitle
	}
	name := levelName
	if name == "" {
		name = mapName
	}
	if mapName != "" && !strings.EqualFold(name, mapName) {
		return fmt.Sprintf("%s (%s)  |  skill %d  -  %s", name, mapName, skill, DefaultWindowTitle)
	}
	return fmt.Sprintf("%s  |  skill %d  -  %s", name, skill, DefaultWindowTitle)
}

// updateWindowTitle is called once per Host.Frame. It rate-limits to
// updateTitleInterval and pushes the computed title to any renderer
// that implements WindowTitleSetter.
func (h *Host) updateWindowTitle(subs *Subsystems, dt float64) {
	if h == nil {
		return
	}
	h.title.timeleft -= dt
	if h.title.timeleft > 0 {
		return
	}
	h.title.timeleft = updateTitleInterval

	var mapName, levelName string
	skill := h.currentSkill
	if subs != nil {
		if subs.Server != nil {
			mapName = subs.Server.MapName()
		}
		if cs, ok := any(subs.Client).(interface{ RuntimeState() *client.Client }); ok {
			if state := cs.RuntimeState(); state != nil {
				levelName = state.LevelName
			}
		}
	}

	title := computeWindowTitle(levelName, mapName, skill)
	if title == h.title.lastTtl && mapName == h.title.lastMap && skill == h.title.lastSkil {
		return
	}
	h.title.lastTtl = title
	h.title.lastMap = mapName
	h.title.lastSkil = skill

	hostDebugSysLogf("windowtitle", "title=%q map=%q skill=%d", title, mapName, skill)

	if subs != nil && subs.Renderer != nil {
		if setter, ok := subs.Renderer.(WindowTitleSetter); ok {
			setter.SetWindowTitle(title)
		}
	}
}
