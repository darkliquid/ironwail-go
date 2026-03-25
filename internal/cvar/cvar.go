// Package cvar implements the Quake engine's console variable (cvar) system.
//
// In id Software's original Quake architecture (cvar.c/cvar.h), cvars are the
// engine's primary mechanism for persistent, named configuration values. While
// the command system (cmdsys) handles *actions* ("map", "quit", "bind"), cvars
// handle *state* — numeric and string settings that control engine behavior.
//
// Common examples of cvars in Quake:
//   - "volume" (0.0–1.0): master audio volume
//   - "sensitivity" (1–20+): mouse sensitivity
//   - "sv_gravity" (800): world gravity for physics
//   - "hostname" ("UNNAMED"): server name shown in browser
//
// Each cvar stores its value simultaneously as a string, float64, and int,
// because different subsystems need different representations. The string is
// the canonical form; numeric values are parsed from it. This "triple storage"
// avoids repeated parsing at runtime — a pattern from the original C code
// where Cvar_SetValue() updates all representations at once.
//
// Cvars are controlled by flags that govern their behavior:
//   - FlagArchive (CVAR_ARCHIVE): saved to config.cfg on shutdown, restored on
//     startup. Used for player preferences like volume and sensitivity.
//   - FlagServerInfo (CVAR_SERVERINFO): replicated to clients in multiplayer.
//     Changing these triggers a serverinfo update so all clients see the new
//     value (e.g., "sv_gravity", "deathmatch").
//   - FlagROM (CVAR_ROM): read-only, can only be set by engine internals. Used
//     for values like "version" that the player should not modify.
//   - FlagNoSet: cannot be set from the console at all.
//   - FlagLatched: value change is accepted but deferred — the new value takes
//     effect only at a specific point (e.g., map change).
//   - FlagNotify: prints a notification to all players when changed.
//   - FlagUserInfo: replicated to the server from each client (e.g., player
//     name, skin, color).
//
// The cvar system integrates tightly with the command system: when a player
// types a cvar name in the console (e.g., "sensitivity 5"), the command
// system's executeLine checks the cvar registry after failing to find a
// matching command or alias, and calls Set() to update the value.
package cvar

import (
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// CVarFlags is a bitmask type for cvar behavior flags. Each flag controls a
// specific aspect of how the cvar is stored, replicated, or protected. Flags
// are combined with bitwise OR (e.g., FlagArchive|FlagServerInfo) to create
// cvars with multiple behaviors. This mirrors Quake's CVAR_* #define constants
// in cvar.h.
type CVarFlags int

// Cvar flag constants, defined as a bitmask using iota. These correspond to
// the CVAR_* flags in the original Quake engine:
//
//   - FlagNone: no special behavior; the cvar is a simple in-memory variable.
//   - FlagArchive: CVAR_ARCHIVE — saved to config.cfg on exit. This is what
//     makes player settings like "volume" and "sensitivity" persist.
//   - FlagNotify: CVAR_NOTIFY — changing this cvar prints a message to all
//     connected players. Used for server gameplay settings (e.g., "fraglimit").
//   - FlagServerInfo: CVAR_SERVERINFO — included in the server info string
//     broadcast to clients. Used for "hostname", "maxplayers", etc.
//   - FlagUserInfo: CVAR_USERINFO — sent from client to server as part of the
//     user info string. Used for "name", "skin", "topcolor".
//   - FlagNoSet: cannot be changed from the console at all.
//   - FlagLatched: CVAR_LATCH — value change is recorded but deferred until
//     a specific engine event (typically a map change or video restart).
//   - FlagROM: CVAR_ROM — read-only from the player's perspective. Only engine
//     internals can modify ROM cvars. Used for "version", "gamedir", etc.
const (
	FlagNone    CVarFlags = 0
	FlagArchive CVarFlags = 1 << iota
	FlagNotify
	FlagServerInfo
	FlagUserInfo
	FlagNoSet
	FlagLatched
	FlagROM
	FlagLocked   // Temporarily locked during gameplay; rejects Set until unlocked.
	FlagAutoCvar // Automatically syncs value to QC global variable autocvar_<name>.
)

// CVar represents a single console variable. It corresponds to cvar_t in
// Quake's cvar.h. Each cvar simultaneously stores its value in three
// representations (String, Float, Int) to avoid repeated parsing — subsystems
// that need a float (e.g., audio volume) read cv.Float directly, while those
// that need an integer (e.g., screen width) read cv.Int.
//
// The Callback field allows subsystems to react immediately when a cvar changes.
// For example, changing "volume" might trigger an audio subsystem callback that
// adjusts the mixer in real time, without waiting for the next frame.
//
// The modified field tracks whether the cvar has been changed from its default
// value, which is useful for serialization and detecting runtime changes.
type CVar struct {
	Name         string         // Canonical lowercase name (e.g., "sv_gravity").
	String       string         // Current value as a string — the canonical representation.
	Float        float64        // Current value parsed as float64 (0 if non-numeric).
	Int          int            // Current value parsed as int (truncated from float).
	Flags        CVarFlags      // Bitmask of behavior flags (archive, serverinfo, ROM, etc.).
	DefaultValue string         // Initial value set at registration time, used for "reset".
	Description  string         // Human-readable help text shown by "cvarlist".
	Callback     func(cv *CVar) // Optional function called after value changes (nil = none).
	Completion   func(currentValue, partial string) []string
	modified     bool // True if value has been changed since registration.
}

// Bool returns the cvar value as a boolean.
// Returns true if Int != 0.
func (cv *CVar) Bool() bool {
	return cv.Int != 0
}

// Float32 returns the cvar value as a float32.
func (cv *CVar) Float32() float32 {
	return float32(cv.Float)
}

// CVarSystem is the central registry for all console variables, analogous to
// the global cvar list in Quake's cvar.c. It provides thread-safe access to
// cvars via a RWMutex — multiple goroutines can read cvar values concurrently
// (common during rendering and physics), while writes (setting values,
// registering new cvars) are serialized.
type CVarSystem struct {
	mu              sync.RWMutex     // Protects concurrent access to the vars map.
	vars            map[string]*CVar // All registered cvars, keyed by lowercase name.
	AutoCvarChanged func(cv *CVar)   // Called when a FlagAutoCvar cvar's value changes.
}

// globalCVar is the package-level singleton CVarSystem instance. Like the
// command system's globalCmd, this mirrors Quake's use of global state for
// the cvar registry. The package-level convenience functions (Get, Set,
// Register, etc.) all delegate to this instance.
var globalCVar = NewCVarSystem()

// NewCVarSystem creates and returns a new, independent cvar registry with an
// empty variable map. Used by the global singleton and in tests for isolation.
func NewCVarSystem() *CVarSystem {
	return &CVarSystem{
		vars: make(map[string]*CVar),
	}
}

// Get retrieves a cvar by name, returning nil if it does not exist. The lookup
// is case-insensitive (name is lowercased). This is the Go equivalent of
// Cvar_FindVar() in Quake's cvar.c and is the most frequently called cvar
// function — used every time the engine needs to read a setting.
func (c *CVarSystem) Get(name string) *CVar {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.vars[strings.ToLower(name)]
}

// Register creates a new cvar with the given name, default value, flags, and
// description. If a cvar with the same name already exists, the existing cvar
// is returned unchanged — this makes registration idempotent, which is
// important because multiple subsystems might attempt to register the same
// cvar during initialization.
//
// This is the Go equivalent of Cvar_RegisterVariable() / Cvar_Get() in
// Quake's cvar.c. Typical usage at subsystem init time:
//
//	var volume = cvar.Register("volume", "0.7", cvar.FlagArchive, "Master audio volume")
//
// The returned pointer is usually stored in a package-level variable so the
// subsystem can read cv.Float directly without repeated map lookups.
func (c *CVarSystem) Register(name, defaultValue string, flags CVarFlags, desc string) *CVar {
	c.mu.Lock()
	defer c.mu.Unlock()

	name = strings.ToLower(name)
	if cv, exists := c.vars[name]; exists {
		return cv
	}

	cv := &CVar{
		Name:         name,
		String:       defaultValue,
		DefaultValue: defaultValue,
		Flags:        flags,
		Description:  desc,
		modified:     false,
	}
	c.parseValue(cv, defaultValue)
	c.vars[name] = cv
	return cv
}

// Set updates the value of a cvar by name. If the cvar doesn't exist, it is
// created implicitly with FlagNone — this allows config files to set cvars
// before the subsystem that owns them has been initialized.
//
// The method enforces flag-based restrictions in the following priority:
//  1. FlagNoSet: the set is silently ignored.
//  2. FlagROM: the set is rejected with a log message ("read-only").
//  3. FlagLatched: the value is accepted but the Callback is NOT invoked.
//  4. Normal: the value is updated, and the Callback (if any) is invoked
//     outside the lock to allow re-entrant cvar reads in the callback.
//
// This is the Go equivalent of Cvar_Set() in Quake's cvar.c.
func (c *CVarSystem) Set(name, value string) {
	name = strings.ToLower(name)
	c.mu.Lock()
	cv, exists := c.vars[name]
	if !exists {
		cv = &CVar{
			Name:   name,
			String: value,
			Flags:  FlagNone,
		}
		c.parseValue(cv, value)
		c.vars[name] = cv
		c.mu.Unlock()
		return
	}

	if cv.Flags&FlagNoSet != 0 {
		c.mu.Unlock()
		return
	}

	if cv.Flags&FlagROM != 0 {
		c.mu.Unlock()
		slog.Info("cvar is read-only", "name", name)
		return
	}

	if cv.Flags&FlagLocked != 0 {
		c.mu.Unlock()
		slog.Info("cvar is locked", "name", name)
		return
	}

	if cv.Flags&FlagLatched != 0 {
		cv.String = value
		c.parseValue(cv, value)
		cv.modified = true
		c.mu.Unlock()
		return
	}

	cv.String = value
	c.parseValue(cv, value)
	cv.modified = true
	callback := cv.Callback
	autoCvarCb := c.AutoCvarChanged
	isAutoCvar := cv.Flags&FlagAutoCvar != 0
	c.mu.Unlock()

	if callback != nil {
		callback(cv)
	}
	if isAutoCvar && autoCvarCb != nil {
		autoCvarCb(cv)
	}
}

// SetFloat is a convenience wrapper that converts a float64 to its string
// representation and calls Set. The Go equivalent of Cvar_SetValue() in cvar.c.
func (c *CVarSystem) SetFloat(name string, value float64) {
	c.Set(name, strconv.FormatFloat(value, 'f', -1, 64))
}

// SetInt is a convenience wrapper that converts an int to its string
// representation and calls Set.
func (c *CVarSystem) SetInt(name string, value int) {
	c.Set(name, strconv.Itoa(value))
}

// SetBool is a convenience wrapper that converts a bool to "1" (true) or "0"
// (false) and calls Set. Quake uses integer 0/1 for boolean cvars rather than
// "true"/"false" strings, so this follows that convention.
func (c *CVarSystem) SetBool(name string, value bool) {
	if value {
		c.Set(name, "1")
	} else {
		c.Set(name, "0")
	}
}

// parseValue updates a cvar's String, Float, and Int fields from a string
// value. This is the "triple storage" update — it parses the string into
// numeric representations so that subsystems can read cv.Float or cv.Int
// directly without repeated parsing. If the string is not a valid number,
// Float and Int are set to 0 (matching C's atof("notanumber") == 0).
func (c *CVarSystem) parseValue(cv *CVar, value string) {
	cv.String = value

	if f, err := strconv.ParseFloat(value, 64); err == nil {
		cv.Float = f
		cv.Int = int(f)
	} else {
		cv.Float = 0
		cv.Int = 0
	}
}

// FloatValue is a shorthand that looks up a cvar by name and returns its Float
// representation, or 0 if the cvar does not exist.
func (c *CVarSystem) FloatValue(name string) float64 {
	if cv := c.Get(name); cv != nil {
		return cv.Float
	}
	return 0
}

// IntValue is a shorthand that looks up a cvar by name and returns its Int
// representation, or 0 if the cvar does not exist.
func (c *CVarSystem) IntValue(name string) int {
	if cv := c.Get(name); cv != nil {
		return cv.Int
	}
	return 0
}

// BoolValue is a shorthand that returns true if the cvar's integer value is
// non-zero. Quake uses 0 for false and non-zero (typically 1) for true.
func (c *CVarSystem) BoolValue(name string) bool {
	return c.IntValue(name) != 0
}

// StringValue is a shorthand that looks up a cvar by name and returns its
// String representation, or "" if the cvar does not exist.
func (c *CVarSystem) StringValue(name string) string {
	if cv := c.Get(name); cv != nil {
		return cv.String
	}
	return ""
}

// All returns a slice of all registered cvars. The slice is a snapshot; the
// caller may iterate it freely. Used by "cvarlist" to enumerate all cvars.
func (c *CVarSystem) All() []*CVar {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*CVar, 0, len(c.vars))
	for _, cv := range c.vars {
		result = append(result, cv)
	}
	return result
}

// LockVar sets FlagLocked on the named cvar, preventing it from being
// changed via Set until UnlockVar is called. Matches C Cvar_LockVar.
func (c *CVarSystem) LockVar(name string) {
	name = strings.ToLower(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if cv, ok := c.vars[name]; ok {
		cv.Flags |= FlagLocked
	}
}

// UnlockVar clears FlagLocked on the named cvar. Matches C Cvar_UnlockVar.
func (c *CVarSystem) UnlockVar(name string) {
	name = strings.ToLower(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if cv, ok := c.vars[name]; ok {
		cv.Flags &^= FlagLocked
	}
}

// ArchiveVars returns a sorted slice of 'name "value"' strings for every cvar
// that has FlagArchive set. This is used by the "host_writeconfig" command to
// generate config.cfg — the file that persists player settings across sessions.
// Only archive-flagged cvars are included because transient or engine-internal
// cvars should not be saved to the player's config. The output is sorted
// alphabetically for deterministic, diffable config files.
func (c *CVarSystem) ArchiveVars() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []string
	for _, cv := range c.vars {
		if cv.Flags&FlagArchive != 0 {
			result = append(result, cv.Name+" \""+cv.String+"\"")
		}
	}
	slices.Sort(result)
	return result
}

// Complete returns all cvar names that begin with the given partial string.
// This powers the console's tab-completion feature for cvars. Combined with
// the command system's Complete and CompleteAliases, this gives the console
// full tab-completion across commands, aliases, and cvars — the three
// namespaces that make up Quake's console identifier space.
func (c *CVarSystem) Complete(partial string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	partial = strings.ToLower(partial)
	var matches []string
	for name := range c.vars {
		if strings.HasPrefix(name, partial) {
			matches = append(matches, name)
		}
	}
	return matches
}

func (c *CVarSystem) SetCompletion(name string, completion func(currentValue, partial string) []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cv, ok := c.vars[strings.ToLower(name)]; ok {
		cv.Completion = completion
	}
}

func (c *CVarSystem) CompleteValue(name, partial string) []string {
	c.mu.RLock()
	cv := c.vars[strings.ToLower(name)]
	c.mu.RUnlock()
	if cv == nil || cv.Completion == nil {
		return nil
	}
	return cv.Completion(cv.String, partial)
}

// ---------------------------------------------------------------------------
// Package-level convenience functions
// ---------------------------------------------------------------------------
// The functions below delegate to the global singleton CVarSystem (globalCVar).
// They provide a flat, procedural API matching the original C codebase where
// functions like Cvar_Set() and Cvar_VariableValue() operate on implicit
// global state. This keeps call sites concise throughout the engine.
// ---------------------------------------------------------------------------

// Get retrieves a cvar from the global registry by name.
func Get(name string) *CVar {
	return globalCVar.Get(name)
}

// Register creates or retrieves a cvar in the global registry.
func Register(name, defaultValue string, flags CVarFlags, desc string) *CVar {
	return globalCVar.Register(name, defaultValue, flags, desc)
}

// Set updates a cvar's value in the global registry.
func Set(name, value string) {
	globalCVar.Set(name, value)
}

// SetFloat sets a cvar's value from a float64 in the global registry.
func SetFloat(name string, value float64) {
	globalCVar.SetFloat(name, value)
}

// SetInt sets a cvar's value from an int in the global registry.
func SetInt(name string, value int) {
	globalCVar.SetInt(name, value)
}

// SetBool sets a cvar's value from a bool in the global registry.
func SetBool(name string, value bool) {
	globalCVar.SetBool(name, value)
}

// FloatValue retrieves a cvar's float64 value from the global registry.
func FloatValue(name string) float64 {
	return globalCVar.FloatValue(name)
}

// IntValue retrieves a cvar's int value from the global registry.
func IntValue(name string) int {
	return globalCVar.IntValue(name)
}

// BoolValue retrieves a cvar's boolean value from the global registry.
func BoolValue(name string) bool {
	return globalCVar.BoolValue(name)
}

// StringValue retrieves a cvar's string value from the global registry.
func StringValue(name string) string {
	return globalCVar.StringValue(name)
}

// All returns all registered cvars from the global registry.
func All() []*CVar {
	return globalCVar.All()
}

// ArchiveVars returns archived cvar settings from the global registry.
func ArchiveVars() []string {
	return globalCVar.ArchiveVars()
}

// Complete returns cvar name completions from the global registry.
func Complete(partial string) []string {
	return globalCVar.Complete(partial)
}

func SetCompletion(name string, completion func(currentValue, partial string) []string) {
	globalCVar.SetCompletion(name, completion)
}

func CompleteValue(name, partial string) []string {
	return globalCVar.CompleteValue(name, partial)
}

// LockVar locks a cvar in the global registry, preventing changes via Set.
func LockVar(name string) {
	globalCVar.LockVar(name)
}

// UnlockVar unlocks a cvar in the global registry, allowing changes again.
func UnlockVar(name string) {
	globalCVar.UnlockVar(name)
}

// SetAutoCvarCallback registers a function to call when any FlagAutoCvar cvar
// value changes. Used by the QC VM integration to sync cvar values to QC
// globals named autocvar_<cvarname>.
func SetAutoCvarCallback(fn func(cv *CVar)) {
	globalCVar.mu.Lock()
	globalCVar.AutoCvarChanged = fn
	globalCVar.mu.Unlock()
}

// MarkAutoCvar sets the FlagAutoCvar flag on a cvar within this registry,
// indicating its value should be synced to a QC global variable.
func (c *CVarSystem) MarkAutoCvar(name string) {
	name = strings.ToLower(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if cv, ok := c.vars[name]; ok {
		cv.Flags |= FlagAutoCvar
	}
}

// MarkAutoCvar sets the FlagAutoCvar flag on a cvar, indicating its value
// should be synced to a QC global variable.
func MarkAutoCvar(name string) {
	globalCVar.MarkAutoCvar(name)
}
