// completion.go — Tab completion for the Quake console
//
// This file implements tab-completion for console commands, console variables
// (cvars), and aliases. Tab completion is essential UX for any command-line
// interface: without it, users would need to memorise hundreds of command and
// variable names. Pressing Tab while typing a partial name cycles through all
// matching completions; pressing Shift+Tab cycles in reverse.
//
// The completion system bridges the console (which owns the input line) with
// the command system (cmdsys) and the cvar system by accepting "provider"
// functions that query each subsystem for matching names. This dependency-
// injection design keeps the console package free of hard imports on cmdsys
// or cvar, making it testable in isolation.
//
// Architecture:
//   - The TabCompleter holds a set of provider functions (CommandProvider,
//     CVarProvider, AliasProvider, FileProvider) that are injected at startup.
//   - When the user presses Tab, Complete() is called. It extracts the
//     partial word from the input, queries all providers, merges and sorts the
//     results, and cycles through them.
//   - GetHint() provides a non-destructive preview (auto-hint) of what the
//     completion would be, without modifying the input line.
//   - A global singleton (GlobalTabCompleter) and package-level convenience
//     functions mirror the pattern used in console.go.
package console

import (
	"sort"
	"strings"
	"sync"
)

// TabCompletionMode represents the type of completion action. The mode
// distinguishes between passive hinting (showing a ghost suffix as the user
// types) and active completion (the user explicitly pressed Tab).
type TabCompletionMode int

// Completion mode constants.
const (
	// TabCompleteAutoHint shows a hint without modifying the input. This is
	// used for inline "ghost text" that appears after the cursor as the user
	// types, similar to IDE autocompletion suggestions.
	TabCompleteAutoHint TabCompletionMode = iota

	// TabCompleteUser performs user-initiated completion — the user pressed
	// Tab, and the input line should be modified to contain the completed text.
	TabCompleteUser
)

// TabMatch represents a single completion candidate. Each match carries the
// name (the actual text that will be inserted), a type label (e.g. "command",
// "cvar", "alias") for display in the completion list, and a count used for
// deduplication bookkeeping.
type TabMatch struct {
	// Name is the completion string that will replace the user's partial input.
	Name string

	// Type identifies the source of this match (e.g. "command", "cvar",
	// "alias"). Displayed next to the name in the match list so the user can
	// distinguish between identically-named items from different sources.
	Type string

	// Count tracks how many providers returned this same name. Used during
	// deduplication — if both the command and alias providers return "map",
	// only one TabMatch is kept but its Count is incremented.
	Count int
}

// TabCompleter provides autocompletion for console input. It maintains the
// current completion state (the list of matches, the cycling index, and the
// partial string being completed) and a set of pluggable provider functions
// that supply candidate names from various engine subsystems.
//
// The completer is thread-safe: all public methods acquire the mutex. This
// is necessary because key-input processing and hint rendering may happen on
// different goroutines.
type TabCompleter struct {
	// mu guards all mutable state. Read-only queries (GetHint, MatchCount)
	// use RLock; mutating calls (Complete, Reset) use Lock.
	mu sync.RWMutex

	// --- Current completion session state ---

	// matches is the sorted, deduplicated list of candidates for the current
	// partial string. Rebuilt whenever the input changes.
	matches []*TabMatch

	// matchIndex is the position in matches[] that will be returned on the
	// next Tab press. It wraps around in both directions for forward/backward
	// cycling.
	matchIndex int

	// partial is the fragment of the input that is being completed (e.g. if
	// the user typed "sv_g", partial is "sv_g").
	partial string

	// lastInput caches the full input string from the previous Complete call.
	// If the input hasn't changed between Tab presses, the match list is
	// reused and only the cycling index advances.
	lastInput string

	// --- Providers: pluggable functions that supply completion candidates ---

	// cmdProvider queries the command system (cmdsys) for commands whose names
	// contain the partial string.
	cmdProvider CommandProvider

	// cvarProvider queries the cvar registry for variable names matching the
	// partial string.
	cvarProvider CVarProvider

	// aliasProvider queries the alias table for alias names matching the
	// partial string.
	aliasProvider AliasProvider

	// fileProvider queries the filesystem for file names matching a glob
	// pattern. Used for commands like "exec" or "map" that take file arguments.
	fileProvider FileProvider
}

// CommandProvider is a function that returns command names matching a partial
// string. It is injected by the command system (cmdsys) at engine startup,
// decoupling the console package from the command implementation.
type CommandProvider func(partial string) []string

// CVarProvider is a function that returns console variable names matching a
// partial string. Injected by the cvar registry so the console can complete
// variable names without importing the cvar package directly.
type CVarProvider func(partial string) []string

// AliasProvider is a function that returns alias names matching a partial
// string. Aliases are user-defined shorthand commands (e.g. "alias rj
// +jump;+attack").
type AliasProvider func(partial string) []string

// FileProvider is a function that returns filesystem paths matching a glob
// pattern. Used for argument completion on commands that accept file names
// (e.g. "exec autoexec.cfg", "map e1m1").
type FileProvider func(pattern string) []string

// NewTabCompleter creates a new tab completer with empty provider slots.
// Providers must be registered via the Set*Provider methods before completion
// will return any results.
func NewTabCompleter() *TabCompleter {
	return &TabCompleter{
		matches: make([]*TabMatch, 0),
	}
}

// SetCommandProvider registers the function used to query available commands.
// This is typically called once during engine initialisation, passing a
// closure that calls into the cmdsys package.
func (tc *TabCompleter) SetCommandProvider(provider CommandProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cmdProvider = provider
}

// SetCVarProvider registers the function used to query console variables.
// Injected by the cvar subsystem so that typing "sv_" + Tab completes to
// matching cvar names like "sv_gravity".
func (tc *TabCompleter) SetCVarProvider(provider CVarProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cvarProvider = provider
}

// SetAliasProvider registers the function used to query alias definitions.
func (tc *TabCompleter) SetAliasProvider(provider AliasProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.aliasProvider = provider
}

// SetFileProvider registers the function used to query filesystem paths.
func (tc *TabCompleter) SetFileProvider(provider FileProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.fileProvider = provider
}

// Complete performs tab completion on the input string. This is the main
// entry point called when the user presses Tab (forward=true) or Shift+Tab
// (forward=false).
//
// On the first call (or when the input has changed), it rebuilds the match
// list by querying all registered providers. On subsequent calls with the
// same input, it simply advances the cycling index to the next match.
//
// Returns:
//   - The modified input string with the partial word replaced by the current
//     match's name.
//   - A slice of human-readable descriptions of all matches (for display in
//     a completion popup or list).
func (tc *TabCompleter) Complete(input string, forward bool) (string, []string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// Extract the partial string to complete
	partial := extractPartial(input)

	// If input changed or no matches, rebuild the match list
	if input != tc.lastInput || len(tc.matches) == 0 {
		tc.buildMatches(partial)
		tc.partial = partial
		tc.matchIndex = 0
		tc.lastInput = input
	}

	if len(tc.matches) == 0 {
		return input, nil
	}

	// Cycle through matches
	var match *TabMatch
	if forward {
		match = tc.matches[tc.matchIndex]
		tc.matchIndex = (tc.matchIndex + 1) % len(tc.matches)
	} else {
		tc.matchIndex--
		if tc.matchIndex < 0 {
			tc.matchIndex = len(tc.matches) - 1
		}
		match = tc.matches[tc.matchIndex]
	}

	// Build the completed string
	result := replacePartial(input, tc.partial, match.Name)

	// Return match descriptions for display
	descriptions := make([]string, len(tc.matches))
	for i, m := range tc.matches {
		if m.Type != "" {
			descriptions[i] = m.Name + " (" + m.Type + ")"
		} else {
			descriptions[i] = m.Name
		}
	}

	return result, descriptions
}

// GetHint returns a completion hint (the suffix that would be appended) without
// modifying the input line. This powers "ghost text" inline hints that show
// the user what Tab would complete to as they type.
//
// If multiple matches exist, the hint is the longest common prefix among all
// matches beyond what the user has already typed. This ensures the hint only
// shows characters that are unambiguous.
func (tc *TabCompleter) GetHint(input string) string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if len(input) == 0 {
		return ""
	}

	partial := extractPartial(input)
	if len(partial) == 0 {
		return ""
	}

	// Build temporary match list
	tc.mu.RUnlock()
	tc.mu.Lock()
	tc.buildMatches(partial)
	tc.mu.Unlock()
	tc.mu.RLock()

	if len(tc.matches) == 0 {
		return ""
	}

	// Find the common prefix among all matches
	commonPrefix := tc.matches[0].Name
	for _, m := range tc.matches[1:] {
		commonPrefix = commonPrefixPrefix(commonPrefix, m.Name)
		if len(commonPrefix) == 0 {
			break
		}
	}

	// Return only the part after the partial
	if len(commonPrefix) > len(partial) {
		return commonPrefix[len(partial):]
	}

	return ""
}

// buildMatches queries all registered providers and assembles a sorted,
// deduplicated list of completion candidates that contain the partial string
// (case-insensitive substring match). This is called whenever the input
// changes between Tab presses.
//
// The algorithm:
//  1. Query each provider (commands, cvars, aliases) for names containing
//     the partial string.
//  2. Tag each result with its source type for display purposes.
//  3. Sort all results alphabetically (case-insensitive).
//  4. Deduplicate: if the same name appears from multiple sources, keep one
//     entry and increment its Count.
func (tc *TabCompleter) buildMatches(partial string) {
	tc.matches = make([]*TabMatch, 0)
	partialLower := strings.ToLower(partial)

	// Collect all matches with their types
	var allMatches []*TabMatch

	// Add commands
	if tc.cmdProvider != nil {
		commands := tc.cmdProvider(partial)
		for _, cmd := range commands {
			if strings.Contains(strings.ToLower(cmd), partialLower) {
				allMatches = append(allMatches, &TabMatch{Name: cmd, Type: "command"})
			}
		}
	}

	// Add cvars
	if tc.cvarProvider != nil {
		cvars := tc.cvarProvider(partial)
		for _, cv := range cvars {
			if strings.Contains(strings.ToLower(cv), partialLower) {
				allMatches = append(allMatches, &TabMatch{Name: cv, Type: "cvar"})
			}
		}
	}

	// Add aliases
	if tc.aliasProvider != nil {
		aliases := tc.aliasProvider(partial)
		for _, alias := range aliases {
			if strings.Contains(strings.ToLower(alias), partialLower) {
				allMatches = append(allMatches, &TabMatch{Name: alias, Type: "alias"})
			}
		}
	}

	// Sort matches by name (natural sort)
	sort.Slice(allMatches, func(i, j int) bool {
		return strings.ToLower(allMatches[i].Name) < strings.ToLower(allMatches[j].Name)
	})

	// Deduplicate and count
	seen := make(map[string]bool)
	for _, m := range allMatches {
		key := strings.ToLower(m.Name)
		if existing, ok := seen[key]; ok {
			if existing {
				// Find and increment count
				for _, existingMatch := range tc.matches {
					if strings.EqualFold(existingMatch.Name, m.Name) {
						existingMatch.Count++
						break
					}
				}
			}
		} else {
			seen[key] = true
			tc.matches = append(tc.matches, m)
		}
	}
}

// extractPartial extracts the "word" at the end of the input that should be
// completed. It walks backward from the end of the string, stopping at spaces,
// semicolons (which separate chained Quake commands), or the beginning of the
// string. Quoted strings are handled so that a space inside quotes doesn't
// split the word.
//
// Examples:
//
//	"map e1"    → "e1"     (completing a map name argument)
//	"sv_grav"   → "sv_grav" (completing a cvar name)
//	"bind x \"" → ""       (cursor is inside a quoted string)
func extractPartial(input string) string {
	// Find the start of the current word
	input = strings.TrimLeft(input, " ")

	// Handle quoted strings
	inQuote := false
	start := 0

	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == '"' {
			inQuote = !inQuote
		} else if ch == ' ' && !inQuote {
			start = i + 1
			break
		} else if ch == ';' && !inQuote {
			start = i + 1
			break
		}
	}

	if start > len(input) {
		start = len(input)
	}

	// Skip leading space after start
	for start < len(input) && input[start] == ' ' {
		start++
	}

	return input[start:]
}

// replacePartial substitutes the partial word at the end of the input string
// with the completed name. It searches backward from the end of input for the
// partial string and replaces it, preserving any prefix (e.g. "map " in
// "map e1m1" is kept when replacing "e1m1" with "e1m2").
func replacePartial(input, partial, completion string) string {
	// Find where the partial starts in the input
	inputLen := len(input)
	partialLen := len(partial)

	if partialLen == 0 {
		return input + completion
	}

	// Find the start of the partial in input
	start := inputLen - partialLen
	for start >= 0 {
		if input[start:] == partial {
			break
		}
		start--
	}

	if start < 0 {
		return input + completion
	}
	return input[:start] + completion + input[inputLen:]
}

// commonPrefixPrefix computes the longest case-insensitive common prefix of
// two strings. This is used by GetHint to determine how much of a completion
// is unambiguous when multiple matches exist. For example, if the matches are
// "sv_gravity" and "sv_greet", the common prefix is "sv_gr".
func commonPrefixPrefix(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	i := 0
	for i < minLen {
		if strings.ToLower(string(a[i])) != strings.ToLower(string(b[i])) {
			break
		}
		i++
	}

	return a[:i]
}

// Reset clears all completion state, forcing the next Complete call to rebuild
// the match list from scratch. This should be called when the user commits
// the input line (presses Enter), clears the input, or otherwise starts a
// new editing context.
func (tc *TabCompleter) Reset() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.matches = nil
	tc.matchIndex = 0
	tc.partial = ""
	tc.lastInput = ""
}

// MatchCount returns the number of candidates in the current completion set.
// Useful for UI code that wants to show "N matches" or decide whether to
// display a completion popup.
func (tc *TabCompleter) MatchCount() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.matches)
}

// GetCurrentMatches returns a defensive copy of all current completion
// candidates. The copy prevents callers from mutating the completer's
// internal state.
func (tc *TabCompleter) GetCurrentMatches() []*TabMatch {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	result := make([]*TabMatch, len(tc.matches))
	copy(result, tc.matches)
	return result
}

// ---------------------------------------------------------------------------
// Global singleton and package-level convenience functions
// ---------------------------------------------------------------------------
// Like the Console type, the TabCompleter has a global singleton so that most
// callers can use simple package-level functions without threading a
// *TabCompleter through every subsystem.
// ---------------------------------------------------------------------------

// GlobalTabCompleter is the process-wide tab completer instance. It is used
// by the package-level convenience functions below and by the console's key
// handling code.
var GlobalTabCompleter = NewTabCompleter()

// SetGlobalCommandProvider registers a command provider on the global completer.
func SetGlobalCommandProvider(provider CommandProvider) {
	GlobalTabCompleter.SetCommandProvider(provider)
}

// SetGlobalCVarProvider registers a cvar provider on the global completer.
func SetGlobalCVarProvider(provider CVarProvider) {
	GlobalTabCompleter.SetCVarProvider(provider)
}

// SetGlobalAliasProvider registers an alias provider on the global completer.
func SetGlobalAliasProvider(provider AliasProvider) {
	GlobalTabCompleter.SetAliasProvider(provider)
}

// SetGlobalFileProvider registers a file provider on the global completer.
func SetGlobalFileProvider(provider FileProvider) {
	GlobalTabCompleter.SetFileProvider(provider)
}

// CompleteInput performs tab completion on the global completer. This is the
// function called by the console's key handler when the user presses Tab.
func CompleteInput(input string, forward bool) (string, []string) {
	return GlobalTabCompleter.Complete(input, forward)
}

// GetCompletionHint returns a non-destructive hint from the global completer.
func GetCompletionHint(input string) string {
	return GlobalTabCompleter.GetHint(input)
}

// ResetCompletion clears the global completer's state for a new input session.
func ResetCompletion() {
	GlobalTabCompleter.Reset()
}
