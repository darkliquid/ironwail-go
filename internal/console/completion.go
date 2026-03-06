// Package console provides tab completion for commands, variables, and aliases.
// This file implements the tab completion system that allows users to
// auto-complete partial input by cycling through matching options.
package console

import (
	"sort"
	"strings"
	"sync"
)

// TabCompletionMode represents the type of completion action.
type TabCompletionMode int

const (
	// TabCompleteAutoHint shows a hint without modifying the input.
	TabCompleteAutoHint TabCompletionMode = iota
	// TabCompleteUser performs user-initiated completion.
	TabCompleteUser
)

// TabMatch represents a single completion match.
type TabMatch struct {
	Name  string
	Type  string
	Count int
}

// TabCompleter provides autocompletion for console input.
type TabCompleter struct {
	mu sync.RWMutex

	// Current completion state
	matches    []*TabMatch
	matchIndex int
	partial    string
	lastInput  string

	// Providers for completion sources
	cmdProvider   CommandProvider
	cvarProvider  CVarProvider
	aliasProvider AliasProvider
	fileProvider  FileProvider
}

// CommandProvider returns available commands for completion.
type CommandProvider func(partial string) []string

// CVarProvider returns available console variables for completion.
type CVarProvider func(partial string) []string

// AliasProvider returns available aliases for completion.
type AliasProvider func(partial string) []string

// FileProvider returns files matching a pattern for completion.
type FileProvider func(pattern string) []string

// NewTabCompleter creates a new tab completer with the given providers.
func NewTabCompleter() *TabCompleter {
	return &TabCompleter{
		matches: make([]*TabMatch, 0),
	}
}

// SetCommandProvider sets the command provider function.
func (tc *TabCompleter) SetCommandProvider(provider CommandProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cmdProvider = provider
}

// SetCVarProvider sets the cvar provider function.
func (tc *TabCompleter) SetCVarProvider(provider CVarProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.cvarProvider = provider
}

// SetAliasProvider sets the alias provider function.
func (tc *TabCompleter) SetAliasProvider(provider AliasProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.aliasProvider = provider
}

// SetFileProvider sets the file provider function.
func (tc *TabCompleter) SetFileProvider(provider FileProvider) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.fileProvider = provider
}

// Complete performs tab completion on the input string.
// It returns the completed string and a list of possible matches.
// If forward is true, cycles forward through matches; otherwise backward.
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

// GetHint returns a completion hint without modifying input.
// This is used to show the user what the completion would be.
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

// buildMatches builds the list of completion matches for the partial string.
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

// extractPartial extracts the partial word to complete from the input.
func extractPartial(input string) string {
	// Find the start of the current word
	input = strings.TrimLeft(input, " ")

	// Handle quoted strings
	inQuote := false
	start := len(input)

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

// replacePartial replaces the partial word in input with the completion.
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

// commonPrefixPrefix finds the common prefix between two strings.
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

// Reset resets the completer state for a new input line.
func (tc *TabCompleter) Reset() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.matches = nil
	tc.matchIndex = 0
	tc.partial = ""
	tc.lastInput = ""
}

// MatchCount returns the number of current matches.
func (tc *TabCompleter) MatchCount() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.matches)
}

// GetCurrentMatches returns the current list of matches.
func (tc *TabCompleter) GetCurrentMatches() []*TabMatch {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	result := make([]*TabMatch, len(tc.matches))
	copy(result, tc.matches)
	return result
}

// GlobalTabCompleter is the global tab completer instance.
var GlobalTabCompleter = NewTabCompleter()

// SetGlobalCommandProvider sets the command provider for the global completer.
func SetGlobalCommandProvider(provider CommandProvider) {
	GlobalTabCompleter.SetCommandProvider(provider)
}

// SetGlobalCVarProvider sets the cvar provider for the global completer.
func SetGlobalCVarProvider(provider CVarProvider) {
	GlobalTabCompleter.SetCVarProvider(provider)
}

// SetGlobalAliasProvider sets the alias provider for the global completer.
func SetGlobalAliasProvider(provider AliasProvider) {
	GlobalTabCompleter.SetAliasProvider(provider)
}

// SetGlobalFileProvider sets the file provider for the global completer.
func SetGlobalFileProvider(provider FileProvider) {
	GlobalTabCompleter.SetFileProvider(provider)
}

// CompleteInput performs tab completion using the global completer.
func CompleteInput(input string, forward bool) (string, []string) {
	return GlobalTabCompleter.Complete(input, forward)
}

// GetCompletionHint returns a completion hint using the global completer.
func GetCompletionHint(input string) string {
	return GlobalTabCompleter.GetHint(input)
}

// ResetCompletion resets the global completer state.
func ResetCompletion() {
	GlobalTabCompleter.Reset()
}
