package game

import "github.com/darkliquid/ironwail-go/internal/input"

// KeyBinding represents a key-to-command binding.
type KeyBinding struct {
	Key     int
	Command string
}

// GameplayBindings returns the default gameplay key bindings.
func GameplayBindings() []KeyBinding {
	return gameplayDefaultBindings
}

// EssentialBindings returns fallback key bindings for critical functions.
func EssentialBindings() []KeyBinding {
	return essentialFallbackBindings
}

var gameplayDefaultBindings = []KeyBinding{
	{Key: int('`'), Command: "toggleconsole"},
	{Key: int('w'), Command: "+forward"},
	{Key: input.KUpArrow, Command: "+forward"},
	{Key: int('s'), Command: "+back"},
	{Key: input.KDownArrow, Command: "+back"},
	{Key: int('a'), Command: "+moveleft"},
	{Key: int('d'), Command: "+moveright"},
	{Key: input.KLeftArrow, Command: "+left"},
	{Key: input.KRightArrow, Command: "+right"},
	{Key: input.KShift, Command: "+speed"},
	{Key: input.KAlt, Command: "+strafe"},
	{Key: input.KTab, Command: "+showscores"},
	{Key: input.KCtrl, Command: "+attack"},
	{Key: input.KMouse1, Command: "+attack"},
	{Key: input.KSpace, Command: "+jump"},
	{Key: input.KMouse2, Command: "+jump"},
	{Key: int('e'), Command: "+use"},
	{Key: input.KMouse3, Command: "+mlook"},
	{Key: input.KMWheelUp, Command: "impulse 10"},
	{Key: input.KMWheelDown, Command: "impulse 12"},
}

var essentialFallbackBindings = []KeyBinding{
	{Key: input.KEscape, Command: "togglemenu"},
	{Key: int('`'), Command: "toggleconsole"},
}
