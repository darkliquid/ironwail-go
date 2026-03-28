package gogpu

import (
	iinput "github.com/darkliquid/ironwail-go/internal/input"
	ginput "github.com/gogpu/gogpu/input"
	"github.com/gogpu/gpucontext"
)

type PollingKeyPair struct {
	Src ginput.Key
	Dst int
}

type PollingMouseButtonPair struct {
	Src ginput.MouseButton
	Dst int
}

var PollingKeyMap = func() []PollingKeyPair {
	pairs := []PollingKeyPair{
		{Src: ginput.KeyEscape, Dst: iinput.KEscape},
		{Src: ginput.KeyEnter, Dst: iinput.KEnter},
		{Src: ginput.KeyNumpadEnter, Dst: iinput.KEnter},
		{Src: ginput.KeyTab, Dst: iinput.KTab},
		{Src: ginput.KeyBackspace, Dst: iinput.KBackspace},
		{Src: ginput.KeySpace, Dst: iinput.KSpace},
		{Src: ginput.KeyApostrophe, Dst: int('\'')},
		{Src: ginput.KeyComma, Dst: int(',')},
		{Src: ginput.KeyMinus, Dst: int('-')},
		{Src: ginput.KeyPeriod, Dst: int('.')},
		{Src: ginput.KeySlash, Dst: int('/')},
		{Src: ginput.KeySemicolon, Dst: int(';')},
		{Src: ginput.KeyEqual, Dst: int('=')},
		{Src: ginput.KeyUp, Dst: iinput.KUpArrow},
		{Src: ginput.KeyDown, Dst: iinput.KDownArrow},
		{Src: ginput.KeyLeft, Dst: iinput.KLeftArrow},
		{Src: ginput.KeyRight, Dst: iinput.KRightArrow},
		{Src: ginput.KeyShiftLeft, Dst: iinput.KShift},
		{Src: ginput.KeyShiftRight, Dst: iinput.KShift},
		{Src: ginput.KeyControlLeft, Dst: iinput.KCtrl},
		{Src: ginput.KeyControlRight, Dst: iinput.KCtrl},
		{Src: ginput.KeyAltLeft, Dst: iinput.KAlt},
		{Src: ginput.KeyAltRight, Dst: iinput.KAlt},
		{Src: ginput.KeySuperLeft, Dst: iinput.KCommand},
		{Src: ginput.KeySuperRight, Dst: iinput.KCommand},
		{Src: ginput.KeyCapsLock, Dst: iinput.KCapsLock},
		{Src: ginput.KeyScrollLock, Dst: iinput.KScrollLock},
		{Src: ginput.KeyPrintScreen, Dst: iinput.KPrintScreen},
		{Src: ginput.KeyPause, Dst: iinput.KPause},
		{Src: ginput.KeyInsert, Dst: iinput.KIns},
		{Src: ginput.KeyDelete, Dst: iinput.KDel},
		{Src: ginput.KeyHome, Dst: iinput.KHome},
		{Src: ginput.KeyEnd, Dst: iinput.KEnd},
		{Src: ginput.KeyPageUp, Dst: iinput.KPgUp},
		{Src: ginput.KeyPageDown, Dst: iinput.KPgDn},
		{Src: ginput.KeyLeftBracket, Dst: int('[')},
		{Src: ginput.KeyBackslash, Dst: int('\\')},
		{Src: ginput.KeyRightBracket, Dst: int(']')},
		{Src: ginput.KeyGrave, Dst: int('`')},
		{Src: ginput.KeyF1, Dst: iinput.KF1},
		{Src: ginput.KeyF2, Dst: iinput.KF2},
		{Src: ginput.KeyF3, Dst: iinput.KF3},
		{Src: ginput.KeyF4, Dst: iinput.KF4},
		{Src: ginput.KeyF5, Dst: iinput.KF5},
		{Src: ginput.KeyF6, Dst: iinput.KF6},
		{Src: ginput.KeyF7, Dst: iinput.KF7},
		{Src: ginput.KeyF8, Dst: iinput.KF8},
		{Src: ginput.KeyF9, Dst: iinput.KF9},
		{Src: ginput.KeyF10, Dst: iinput.KF10},
		{Src: ginput.KeyF11, Dst: iinput.KF11},
		{Src: ginput.KeyF12, Dst: iinput.KF12},
		{Src: ginput.KeyNumpad0, Dst: iinput.KKpIns},
		{Src: ginput.KeyNumpad1, Dst: iinput.KKpEnd},
		{Src: ginput.KeyNumpad2, Dst: iinput.KKpDownArrow},
		{Src: ginput.KeyNumpad3, Dst: iinput.KKpPgDn},
		{Src: ginput.KeyNumpad4, Dst: iinput.KKpLeftArrow},
		{Src: ginput.KeyNumpad5, Dst: iinput.KKp5},
		{Src: ginput.KeyNumpad6, Dst: iinput.KKpRightArrow},
		{Src: ginput.KeyNumpad7, Dst: iinput.KKpHome},
		{Src: ginput.KeyNumpad8, Dst: iinput.KKpUpArrow},
		{Src: ginput.KeyNumpad9, Dst: iinput.KKpPgUp},
		{Src: ginput.KeyNumpadDivide, Dst: iinput.KKpSlash},
		{Src: ginput.KeyNumpadMultiply, Dst: iinput.KKpStar},
		{Src: ginput.KeyNumpadSubtract, Dst: iinput.KKpMinus},
		{Src: ginput.KeyNumpadAdd, Dst: iinput.KKpPlus},
		{Src: ginput.KeyNumpadDecimal, Dst: iinput.KKpDel},
		{Src: ginput.KeyNumLock, Dst: iinput.KKpNumLock},
	}

	letterKeys := []ginput.Key{
		ginput.KeyA, ginput.KeyB, ginput.KeyC, ginput.KeyD, ginput.KeyE, ginput.KeyF,
		ginput.KeyG, ginput.KeyH, ginput.KeyI, ginput.KeyJ, ginput.KeyK, ginput.KeyL,
		ginput.KeyM, ginput.KeyN, ginput.KeyO, ginput.KeyP, ginput.KeyQ, ginput.KeyR,
		ginput.KeyS, ginput.KeyT, ginput.KeyU, ginput.KeyV, ginput.KeyW, ginput.KeyX,
		ginput.KeyY, ginput.KeyZ,
	}
	for index, key := range letterKeys {
		pairs = append(pairs, PollingKeyPair{Src: key, Dst: int('a' + index)})
	}

	numberKeys := []ginput.Key{
		ginput.Key0, ginput.Key1, ginput.Key2, ginput.Key3, ginput.Key4,
		ginput.Key5, ginput.Key6, ginput.Key7, ginput.Key8, ginput.Key9,
	}
	for index, key := range numberKeys {
		pairs = append(pairs, PollingKeyPair{Src: key, Dst: int('0' + index)})
	}

	return pairs
}()

var PollingMouseButtonMap = []PollingMouseButtonPair{
	{Src: ginput.MouseButtonLeft, Dst: iinput.KMouse1},
	{Src: ginput.MouseButtonRight, Dst: iinput.KMouse2},
	{Src: ginput.MouseButtonMiddle, Dst: iinput.KMouse3},
	{Src: ginput.MouseButton4, Dst: iinput.KMouse4},
	{Src: ginput.MouseButton5, Dst: iinput.KMouse5},
}

// MapGPUContextMouseButton maps a gpucontext mouse button to a Quake engine key code.
func MapGPUContextMouseButton(button gpucontext.MouseButton) int {
	switch button {
	case gpucontext.MouseButtonLeft:
		return iinput.KMouse1
	case gpucontext.MouseButtonRight:
		return iinput.KMouse2
	case gpucontext.MouseButtonMiddle:
		return iinput.KMouse3
	case gpucontext.MouseButton4:
		return iinput.KMouse4
	case gpucontext.MouseButton5:
		return iinput.KMouse5
	default:
		return -1
	}
}

// MapGPUContextKey maps a gpucontext key to a Quake engine key code.
func MapGPUContextKey(key gpucontext.Key) int {
	switch key {
	case gpucontext.KeyEscape:
		return iinput.KEscape
	case gpucontext.KeyEnter, gpucontext.KeyNumpadEnter:
		return iinput.KEnter
	case gpucontext.KeyTab:
		return iinput.KTab
	case gpucontext.KeyBackspace:
		return iinput.KBackspace
	case gpucontext.KeySpace:
		return iinput.KSpace
	case gpucontext.KeyApostrophe:
		return int('\'')
	case gpucontext.KeyComma:
		return int(',')
	case gpucontext.KeyMinus:
		return int('-')
	case gpucontext.KeyPeriod:
		return int('.')
	case gpucontext.KeySlash:
		return int('/')
	case gpucontext.KeySemicolon:
		return int(';')
	case gpucontext.KeyEqual:
		return int('=')
	case gpucontext.KeyUp:
		return iinput.KUpArrow
	case gpucontext.KeyDown:
		return iinput.KDownArrow
	case gpucontext.KeyLeft:
		return iinput.KLeftArrow
	case gpucontext.KeyRight:
		return iinput.KRightArrow
	case gpucontext.KeyLeftShift, gpucontext.KeyRightShift:
		return iinput.KShift
	case gpucontext.KeyLeftControl, gpucontext.KeyRightControl:
		return iinput.KCtrl
	case gpucontext.KeyLeftAlt, gpucontext.KeyRightAlt:
		return iinput.KAlt
	case gpucontext.KeyLeftSuper, gpucontext.KeyRightSuper:
		return iinput.KCommand
	case gpucontext.KeyCapsLock:
		return iinput.KCapsLock
	case gpucontext.KeyScrollLock:
		return iinput.KScrollLock
	case gpucontext.KeyPrintScreen:
		return iinput.KPrintScreen
	case gpucontext.KeyPause:
		return iinput.KPause
	case gpucontext.KeyInsert:
		return iinput.KIns
	case gpucontext.KeyDelete:
		return iinput.KDel
	case gpucontext.KeyHome:
		return iinput.KHome
	case gpucontext.KeyEnd:
		return iinput.KEnd
	case gpucontext.KeyPageUp:
		return iinput.KPgUp
	case gpucontext.KeyPageDown:
		return iinput.KPgDn
	case gpucontext.KeyLeftBracket:
		return int('[')
	case gpucontext.KeyBackslash:
		return int('\\')
	case gpucontext.KeyRightBracket:
		return int(']')
	case gpucontext.KeyGrave:
		return int('`')
	case gpucontext.KeyF1:
		return iinput.KF1
	case gpucontext.KeyF2:
		return iinput.KF2
	case gpucontext.KeyF3:
		return iinput.KF3
	case gpucontext.KeyF4:
		return iinput.KF4
	case gpucontext.KeyF5:
		return iinput.KF5
	case gpucontext.KeyF6:
		return iinput.KF6
	case gpucontext.KeyF7:
		return iinput.KF7
	case gpucontext.KeyF8:
		return iinput.KF8
	case gpucontext.KeyF9:
		return iinput.KF9
	case gpucontext.KeyF10:
		return iinput.KF10
	case gpucontext.KeyF11:
		return iinput.KF11
	case gpucontext.KeyF12:
		return iinput.KF12
	case gpucontext.KeyNumpad0:
		return iinput.KKpIns
	case gpucontext.KeyNumpad1:
		return iinput.KKpEnd
	case gpucontext.KeyNumpad2:
		return iinput.KKpDownArrow
	case gpucontext.KeyNumpad3:
		return iinput.KKpPgDn
	case gpucontext.KeyNumpad4:
		return iinput.KKpLeftArrow
	case gpucontext.KeyNumpad5:
		return iinput.KKp5
	case gpucontext.KeyNumpad6:
		return iinput.KKpRightArrow
	case gpucontext.KeyNumpad7:
		return iinput.KKpHome
	case gpucontext.KeyNumpad8:
		return iinput.KKpUpArrow
	case gpucontext.KeyNumpad9:
		return iinput.KKpPgUp
	case gpucontext.KeyNumpadDivide:
		return iinput.KKpSlash
	case gpucontext.KeyNumpadMultiply:
		return iinput.KKpStar
	case gpucontext.KeyNumpadSubtract:
		return iinput.KKpMinus
	case gpucontext.KeyNumpadAdd:
		return iinput.KKpPlus
	case gpucontext.KeyNumpadDecimal:
		return iinput.KKpDel
	case gpucontext.KeyNumLock:
		return iinput.KKpNumLock
	}

	if key >= gpucontext.KeyA && key <= gpucontext.KeyZ {
		return int('a' + (key - gpucontext.KeyA))
	}
	if key >= gpucontext.Key0 && key <= gpucontext.Key9 {
		return int('0' + (key - gpucontext.Key0))
	}

	return -1
}
