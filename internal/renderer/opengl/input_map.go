//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

import (
	iinput "github.com/darkliquid/ironwail-go/internal/input"
	"github.com/go-gl/glfw/v3.3/glfw"
)

// MapGLFWKey maps a GLFW key to a Quake engine key code. Returns -1 if unmapped.
func MapGLFWKey(k glfw.Key) int {
	switch k {
	case glfw.KeyEscape:
		return iinput.KEscape
	case glfw.KeyEnter, glfw.KeyKPEnter:
		return iinput.KEnter
	case glfw.KeyTab:
		return iinput.KTab
	case glfw.KeyBackspace:
		return iinput.KBackspace
	case glfw.KeySpace:
		return iinput.KSpace
	case glfw.KeyApostrophe:
		return int('\'')
	case glfw.KeyComma:
		return int(',')
	case glfw.KeyMinus:
		return int('-')
	case glfw.KeyPeriod:
		return int('.')
	case glfw.KeySlash:
		return int('/')
	case glfw.KeySemicolon:
		return int(';')
	case glfw.KeyEqual:
		return int('=')
	case glfw.KeyUp:
		return iinput.KUpArrow
	case glfw.KeyDown:
		return iinput.KDownArrow
	case glfw.KeyLeft:
		return iinput.KLeftArrow
	case glfw.KeyRight:
		return iinput.KRightArrow
	case glfw.KeyLeftShift, glfw.KeyRightShift:
		return iinput.KShift
	case glfw.KeyLeftControl, glfw.KeyRightControl:
		return iinput.KCtrl
	case glfw.KeyLeftAlt, glfw.KeyRightAlt:
		return iinput.KAlt
	case glfw.KeyLeftSuper, glfw.KeyRightSuper:
		return iinput.KCommand
	case glfw.KeyCapsLock:
		return iinput.KCapsLock
	case glfw.KeyScrollLock:
		return iinput.KScrollLock
	case glfw.KeyPrintScreen:
		return iinput.KPrintScreen
	case glfw.KeyPause:
		return iinput.KPause
	case glfw.KeyInsert:
		return iinput.KIns
	case glfw.KeyDelete:
		return iinput.KDel
	case glfw.KeyHome:
		return iinput.KHome
	case glfw.KeyEnd:
		return iinput.KEnd
	case glfw.KeyPageUp:
		return iinput.KPgUp
	case glfw.KeyPageDown:
		return iinput.KPgDn
	case glfw.KeyLeftBracket:
		return int('[')
	case glfw.KeyBackslash:
		return int('\\')
	case glfw.KeyRightBracket:
		return int(']')
	case glfw.KeyGraveAccent:
		return int('`')
	case glfw.KeyF1:
		return iinput.KF1
	case glfw.KeyF2:
		return iinput.KF2
	case glfw.KeyF3:
		return iinput.KF3
	case glfw.KeyF4:
		return iinput.KF4
	case glfw.KeyF5:
		return iinput.KF5
	case glfw.KeyF6:
		return iinput.KF6
	case glfw.KeyF7:
		return iinput.KF7
	case glfw.KeyF8:
		return iinput.KF8
	case glfw.KeyF9:
		return iinput.KF9
	case glfw.KeyF10:
		return iinput.KF10
	case glfw.KeyF11:
		return iinput.KF11
	case glfw.KeyF12:
		return iinput.KF12
	case glfw.KeyKP0:
		return iinput.KKpIns
	case glfw.KeyKP1:
		return iinput.KKpEnd
	case glfw.KeyKP2:
		return iinput.KKpDownArrow
	case glfw.KeyKP3:
		return iinput.KKpPgDn
	case glfw.KeyKP4:
		return iinput.KKpLeftArrow
	case glfw.KeyKP5:
		return iinput.KKp5
	case glfw.KeyKP6:
		return iinput.KKpRightArrow
	case glfw.KeyKP7:
		return iinput.KKpHome
	case glfw.KeyKP8:
		return iinput.KKpUpArrow
	case glfw.KeyKP9:
		return iinput.KKpPgUp
	case glfw.KeyKPDivide:
		return iinput.KKpSlash
	case glfw.KeyKPMultiply:
		return iinput.KKpStar
	case glfw.KeyKPSubtract:
		return iinput.KKpMinus
	case glfw.KeyKPAdd:
		return iinput.KKpPlus
	case glfw.KeyKPDecimal:
		return iinput.KKpDel
	case glfw.KeyNumLock:
		return iinput.KKpNumLock
	case glfw.KeyA:
		return int('a')
	case glfw.KeyB:
		return int('b')
	case glfw.KeyC:
		return int('c')
	case glfw.KeyD:
		return int('d')
	case glfw.KeyE:
		return int('e')
	case glfw.KeyF:
		return int('f')
	case glfw.KeyG:
		return int('g')
	case glfw.KeyH:
		return int('h')
	case glfw.KeyI:
		return int('i')
	case glfw.KeyJ:
		return int('j')
	case glfw.KeyK:
		return int('k')
	case glfw.KeyL:
		return int('l')
	case glfw.KeyM:
		return int('m')
	case glfw.KeyN:
		return int('n')
	case glfw.KeyO:
		return int('o')
	case glfw.KeyP:
		return int('p')
	case glfw.KeyQ:
		return int('q')
	case glfw.KeyR:
		return int('r')
	case glfw.KeyS:
		return int('s')
	case glfw.KeyT:
		return int('t')
	case glfw.KeyU:
		return int('u')
	case glfw.KeyV:
		return int('v')
	case glfw.KeyW:
		return int('w')
	case glfw.KeyX:
		return int('x')
	case glfw.KeyY:
		return int('y')
	case glfw.KeyZ:
		return int('z')
	case glfw.Key0:
		return int('0')
	case glfw.Key1:
		return int('1')
	case glfw.Key2:
		return int('2')
	case glfw.Key3:
		return int('3')
	case glfw.Key4:
		return int('4')
	case glfw.Key5:
		return int('5')
	case glfw.Key6:
		return int('6')
	case glfw.Key7:
		return int('7')
	case glfw.Key8:
		return int('8')
	case glfw.Key9:
		return int('9')
	}
	return -1
}

// MapGLFWMouseButton maps a GLFW mouse button to a Quake engine key code. Returns -1 if unmapped.
func MapGLFWMouseButton(b glfw.MouseButton) int {
	switch b {
	case glfw.MouseButtonLeft:
		return iinput.KMouse1
	case glfw.MouseButtonRight:
		return iinput.KMouse2
	case glfw.MouseButtonMiddle:
		return iinput.KMouse3
	case glfw.MouseButton4:
		return iinput.KMouse4
	case glfw.MouseButton5:
		return iinput.KMouse5
	}
	return -1
}
