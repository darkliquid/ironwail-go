package menu

import (
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/gogpu/ui/event"
)

// keyEventToEngine maps a gogpu/ui key event to an engine key code for the
// menu action path (M_Key). Only press events are forwarded (Quake menus act
// on press); text runes go through M_Char separately. Returns -1 for keys
// with no engine equivalent.
func keyEventToEngine(ke *event.KeyEvent) int {
	if ke == nil || ke.KeyType != event.KeyPress {
		return -1
	}
	switch ke.Key {
	case event.KeyUp:
		return input.KUpArrow
	case event.KeyDown:
		return input.KDownArrow
	case event.KeyLeft:
		return input.KLeftArrow
	case event.KeyRight:
		return input.KRightArrow
	case event.KeyEnter:
		return input.KEnter
	case event.KeyEscape:
		return input.KEscape
	case event.KeyTab:
		return input.KTab
	case event.KeyBackspace:
		return input.KBackspace
	case event.KeyHome:
		return input.KHome
	case event.KeyEnd:
		return input.KEnd
	case event.KeyPageUp:
		return input.KPgUp
	case event.KeyPageDown:
		return input.KPgDn
	case event.KeyInsert:
		return input.KIns
	case event.KeyDelete:
		return input.KDel
	case event.KeyF1:
		return input.KF1
	case event.KeyF2:
		return input.KF2
	case event.KeyF3:
		return input.KF3
	case event.KeyF4:
		return input.KF4
	case event.KeyF5:
		return input.KF5
	case event.KeyF6:
		return input.KF6
	case event.KeyF7:
		return input.KF7
	case event.KeyF8:
		return input.KF8
	case event.KeyF9:
		return input.KF9
	case event.KeyF10:
		return input.KF10
	case event.KeyF11:
		return input.KF11
	case event.KeyF12:
		return input.KF12
	}
	// Printable ASCII keys (letters/digits) map via their rune when present.
	if ke.Rune != 0 && ke.Rune < 128 {
		return int(ke.Rune)
	}
	return -1
}
