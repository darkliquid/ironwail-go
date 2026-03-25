package console

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

type TabCompletionMode int

const (
	TabCompleteAutoHint TabCompletionMode = iota
	TabCompleteUser
)

type TabMatch struct {
	Name  string
	Type  string
	Count int
}

type CommandProvider func(partial string) []string
type CVarProvider func(partial string) []string
type AliasProvider func(partial string) []string
type FileProvider func(pattern string) []string
type CommandArgsProvider func(command string, args []string, partial string) []string
type CVarValueProvider func(cvarName string, partial string) []string
type PrintFunc func(format string, args ...interface{})

type TabCompleter struct {
	mu sync.RWMutex

	matches    []*TabMatch
	matchIndex int
	partial    string
	lastInput  string

	cmdProvider       CommandProvider
	cvarProvider      CVarProvider
	aliasProvider     AliasProvider
	fileProvider      FileProvider
	cmdArgsProvider   CommandArgsProvider
	cvarValueProvider CVarValueProvider
	printFn           PrintFunc
}

func NewTabCompleter() *TabCompleter { return &TabCompleter{} }

func (tc *TabCompleter) SetCommandProvider(provider CommandProvider) {
	tc.mu.Lock()
	tc.cmdProvider = provider
	tc.mu.Unlock()
}
func (tc *TabCompleter) SetCVarProvider(provider CVarProvider) {
	tc.mu.Lock()
	tc.cvarProvider = provider
	tc.mu.Unlock()
}
func (tc *TabCompleter) SetAliasProvider(provider AliasProvider) {
	tc.mu.Lock()
	tc.aliasProvider = provider
	tc.mu.Unlock()
}
func (tc *TabCompleter) SetFileProvider(provider FileProvider) {
	tc.mu.Lock()
	tc.fileProvider = provider
	tc.mu.Unlock()
}
func (tc *TabCompleter) SetCommandArgsProvider(provider CommandArgsProvider) {
	tc.mu.Lock()
	tc.cmdArgsProvider = provider
	tc.mu.Unlock()
}
func (tc *TabCompleter) SetCVarValueProvider(provider CVarValueProvider) {
	tc.mu.Lock()
	tc.cvarValueProvider = provider
	tc.mu.Unlock()
}
func (tc *TabCompleter) SetPrintFunc(printFn PrintFunc) {
	tc.mu.Lock()
	tc.printFn = printFn
	tc.mu.Unlock()
}

func (tc *TabCompleter) Complete(input string, forward bool) (string, []string) {
	return tc.complete(input, forward, TabCompleteUser)
}

func (tc *TabCompleter) GetHint(input string) string {
	completed, _ := tc.complete(input, true, TabCompleteAutoHint)
	if len(completed) <= len(input) || !strings.HasPrefix(completed, input) {
		return ""
	}
	return completed[len(input):]
}

func (tc *TabCompleter) complete(input string, forward bool, mode TabCompletionMode) (string, []string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	partial := extractPartial(input)
	if input != tc.lastInput || len(tc.matches) == 0 {
		tc.matches = tc.buildMatches(input)
		tc.matchIndex = 0
		tc.partial = partial
		tc.lastInput = input
		if mode == TabCompleteUser && len(tc.matches) > 1 {
			if tc.printFn != nil {
				tc.printFn("\n")
				tc.printMatchList(tc.matches)
				tc.printFn("]%s", input)
			}
			common := longestCommonPrefix(matchNames(tc.matches))
			if len(common) >= len(partial) {
				return replacePartial(input, partial, common), describeMatches(tc.matches)
			}
		}
	}

	if len(tc.matches) == 0 {
		return input, nil
	}

	if mode == TabCompleteAutoHint {
		common := longestCommonPrefix(matchNames(tc.matches))
		if len(common) >= len(partial) {
			return replacePartial(input, partial, common), describeMatches(tc.matches)
		}
		return input, describeMatches(tc.matches)
	}

	if forward {
		m := tc.matches[tc.matchIndex]
		tc.matchIndex = (tc.matchIndex + 1) % len(tc.matches)
		return replacePartial(input, partial, m.Name), describeMatches(tc.matches)
	}
	tc.matchIndex--
	if tc.matchIndex < 0 {
		tc.matchIndex = len(tc.matches) - 1
	}
	m := tc.matches[tc.matchIndex]
	return replacePartial(input, partial, m.Name), describeMatches(tc.matches)
}

func (tc *TabCompleter) buildMatches(input string) []*TabMatch {
	seg := currentCommandSegment(input)
	fields := strings.Fields(seg)
	trailingSpace := strings.HasSuffix(seg, " ")
	partial := extractPartial(input)

	all := []*TabMatch{}
	add := func(name, typ string) {
		if name == "" {
			return
		}
		all = append(all, &TabMatch{Name: name, Type: typ, Count: 1})
	}

	commandNameMode := len(fields) == 0 || (len(fields) == 1 && !trailingSpace)
	if commandNameMode {
		if tc.cmdProvider != nil {
			for _, n := range tc.cmdProvider(partial) {
				if strings.HasPrefix(strings.ToLower(n), strings.ToLower(partial)) {
					add(n, "command")
				}
			}
		}
		if tc.cvarProvider != nil {
			for _, n := range tc.cvarProvider(partial) {
				if strings.HasPrefix(strings.ToLower(n), strings.ToLower(partial)) {
					add(n, "cvar")
				}
			}
		}
		if tc.aliasProvider != nil {
			for _, n := range tc.aliasProvider(partial) {
				if strings.HasPrefix(strings.ToLower(n), strings.ToLower(partial)) {
					add(n, "alias")
				}
			}
		}
	} else {
		cmd := strings.ToLower(fields[0])
		args := fields[1:]
		if trailingSpace {
			args = append(args, "")
		}
		if tc.cmdArgsProvider != nil {
			for _, n := range tc.cmdArgsProvider(cmd, args, partial) {
				if strings.HasPrefix(strings.ToLower(n), strings.ToLower(partial)) {
					add(n, "arg")
				}
			}
		}
		if tc.cvarValueProvider != nil && len(fields) > 0 {
			for _, n := range tc.cvarValueProvider(cmd, partial) {
				if strings.HasPrefix(strings.ToLower(n), strings.ToLower(partial)) {
					add(n, "value")
				}
			}
		}
		if tc.fileProvider != nil {
			for _, spec := range fileCompletionSpecs(seg) {
				for _, file := range tc.fileProvider(spec.Pattern) {
					name := spec.Normalize(file)
					if strings.HasPrefix(strings.ToLower(name), strings.ToLower(partial)) {
						add(name, spec.Type)
					}
				}
			}
		}
	}

	sort.Slice(all, func(i, j int) bool { return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name) })
	uniq := make([]*TabMatch, 0, len(all))
	seen := map[string]int{}
	for _, m := range all {
		key := strings.ToLower(m.Name)
		if idx, ok := seen[key]; ok {
			uniq[idx].Count++
			continue
		}
		seen[key] = len(uniq)
		uniq = append(uniq, m)
	}
	return uniq
}

func (tc *TabCompleter) printMatchList(matches []*TabMatch) {
	if len(matches) == 0 || tc.printFn == nil {
		return
	}
	maxLen := 0
	for _, m := range matches {
		if len(m.Name) > maxLen {
			maxLen = len(m.Name)
		}
	}
	colWidth := maxLen + 2
	lineWidth := LineWidth()
	if lineWidth <= 0 {
		lineWidth = 78
	}
	cols := lineWidth / colWidth
	if cols < 1 {
		cols = 1
	}
	if mx := cvar.IntValue("con_maxcols"); mx > 0 && mx < cols {
		cols = mx
	}
	for i, m := range matches {
		if i%cols == 0 {
			tc.printFn("\n")
		}
		name := m.Name
		if len(name) < colWidth {
			name += strings.Repeat(" ", colWidth-len(name))
		}
		tc.printFn("%s", name)
	}
	tc.printFn("\n")
}

type fileCompletionSpec struct {
	Pattern   string
	Type      string
	Normalize func(string) string
}

func fileCompletionSpecs(segment string) []fileCompletionSpec {
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return nil
	}
	if len(fields) == 1 && !strings.HasSuffix(segment, " ") {
		return nil
	}
	switch strings.ToLower(fields[0]) {
	case "map", "changelevel":
		return []fileCompletionSpec{{Pattern: "maps/*.bsp", Type: "map", Normalize: func(path string) string { return strings.TrimSuffix(filepath.Base(path), ".bsp") }}}
	case "exec":
		return []fileCompletionSpec{{Pattern: "*.cfg", Type: "config", Normalize: func(path string) string { return filepath.Base(path) }}}
	case "playdemo", "timedemo", "record":
		return []fileCompletionSpec{{Pattern: "*.dem", Type: "demo", Normalize: func(path string) string { return strings.TrimSuffix(filepath.Base(path), ".dem") }}}
	case "load", "save":
		return []fileCompletionSpec{{Pattern: "*.sav", Type: "save", Normalize: func(path string) string { return strings.TrimSuffix(filepath.Base(path), ".sav") }}}
	case "sky":
		return []fileCompletionSpec{{Pattern: "gfx/env/*.tga", Type: "sky", Normalize: func(path string) string {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			for _, suf := range []string{"rt", "bk", "lf", "ft", "up", "dn"} {
				base = strings.TrimSuffix(base, suf)
			}
			return base
		}}}
	default:
		return nil
	}
}

func currentCommandSegment(input string) string {
	input = strings.TrimLeft(input, " ")
	inQuote := false
	start := 0
	for i := len(input) - 1; i >= 0; i-- {
		if input[i] == '"' {
			inQuote = !inQuote
			continue
		}
		if input[i] == ';' && !inQuote {
			start = i + 1
			break
		}
	}
	for start < len(input) && input[start] == ' ' {
		start++
	}
	return input[start:]
}

func extractPartial(input string) string {
	input = strings.TrimLeft(input, " ")
	if strings.HasSuffix(input, " ") {
		return ""
	}
	inQuote := false
	start := 0
	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == '"' {
			inQuote = !inQuote
		} else if (ch == ' ' || ch == ';') && !inQuote {
			start = i + 1
			break
		}
	}
	for start < len(input) && input[start] == ' ' {
		start++
	}
	return input[start:]
}

func replacePartial(input, partial, completion string) string {
	if partial == "" {
		if input == "" || strings.HasSuffix(input, " ") {
			return input + completion
		}
		return input + " " + completion
	}
	idx := strings.LastIndex(input, partial)
	if idx < 0 {
		return input
	}
	return input[:idx] + completion + input[idx+len(partial):]
}

func longestCommonPrefix(values []string) string {
	if len(values) == 0 {
		return ""
	}
	prefix := values[0]
	for _, v := range values[1:] {
		n := len(prefix)
		if len(v) < n {
			n = len(v)
		}
		i := 0
		for i < n && strings.EqualFold(prefix[i:i+1], v[i:i+1]) {
			i++
		}
		prefix = prefix[:i]
		if prefix == "" {
			break
		}
	}
	return prefix
}

func matchNames(matches []*TabMatch) []string {
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m.Name)
	}
	return out
}

func describeMatches(matches []*TabMatch) []string {
	desc := make([]string, 0, len(matches))
	for _, m := range matches {
		if m.Type != "" {
			desc = append(desc, m.Name+" ("+m.Type+")")
		} else {
			desc = append(desc, m.Name)
		}
	}
	return desc
}

func (tc *TabCompleter) Reset() {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.matches = nil
	tc.matchIndex = 0
	tc.partial = ""
	tc.lastInput = ""
}
func (tc *TabCompleter) MatchCount() int {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return len(tc.matches)
}
func (tc *TabCompleter) GetCurrentMatches() []*TabMatch {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	out := make([]*TabMatch, len(tc.matches))
	copy(out, tc.matches)
	return out
}

var GlobalTabCompleter = NewTabCompleter()

func SetGlobalCommandProvider(provider CommandProvider) {
	GlobalTabCompleter.SetCommandProvider(provider)
}
func SetGlobalCVarProvider(provider CVarProvider)   { GlobalTabCompleter.SetCVarProvider(provider) }
func SetGlobalAliasProvider(provider AliasProvider) { GlobalTabCompleter.SetAliasProvider(provider) }
func SetGlobalFileProvider(provider FileProvider)   { GlobalTabCompleter.SetFileProvider(provider) }
func SetGlobalCommandArgsProvider(provider CommandArgsProvider) {
	GlobalTabCompleter.SetCommandArgsProvider(provider)
}
func SetGlobalCVarValueProvider(provider CVarValueProvider) {
	GlobalTabCompleter.SetCVarValueProvider(provider)
}
func SetGlobalCompletionPrintFunc(printFn PrintFunc) { GlobalTabCompleter.SetPrintFunc(printFn) }
func CompleteInput(input string, forward bool) (string, []string) {
	return GlobalTabCompleter.Complete(input, forward)
}
func GetCompletionHint(input string) string { return GlobalTabCompleter.GetHint(input) }
func ResetCompletion()                      { GlobalTabCompleter.Reset() }
