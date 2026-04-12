package cmdsys

import (
	"fmt"
	"slices"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func (c *CmdSystem) visibleCommands() []*Command {
	c.mu.RLock()
	defer c.mu.RUnlock()

	commands := make([]*Command, 0, len(c.commands))
	for _, cmd := range c.commands {
		if cmd.SourceType == SrcServer || isReservedName(cmd.Name) {
			continue
		}
		commands = append(commands, cmd)
	}

	slices.SortFunc(commands, func(a, b *Command) int {
		return strings.Compare(a.Name, b.Name)
	})
	return commands
}

func (c *CmdSystem) cmdList(args []string) {
	partial := ""
	if len(args) > 0 {
		partial = strings.ToLower(args[0])
	}

	count := 0
	for _, cmd := range c.visibleCommands() {
		if partial != "" && !strings.HasPrefix(cmd.Name, partial) {
			continue
		}
		printCallback(fmt.Sprintf("   %s\n", cmd.Name))
		count++
	}

	msg := fmt.Sprintf("%d commands", count)
	if partial != "" {
		msg += fmt.Sprintf(" beginning with %q", partial)
	}
	printCallback(msg + "\n")
}

func (c *CmdSystem) cmdApropos(commandName string, args []string) {
	if len(args) == 0 || args[0] == "" {
		printCallback(fmt.Sprintf("%s <substring> : search through commands and cvars for the given substring\n", commandName))
		return
	}

	c.listAllContaining(args[0])
}

func (c *CmdSystem) listAllContaining(substr string) {
	lowerSubstr := strings.ToLower(substr)
	hits := 0
	for _, cmd := range c.visibleCommands() {
		if strings.Contains(strings.ToLower(cmd.Name), lowerSubstr) || strings.Contains(strings.ToLower(cmd.Description), lowerSubstr) {
			printCallback(fmt.Sprintf("   %s\n", cmd.Name))
			hits++
		}
	}

	vars := cvar.All()
	slices.SortFunc(vars, func(a, b *cvar.CVar) int {
		return strings.Compare(a.Name, b.Name)
	})
	for _, cv := range vars {
		if strings.Contains(strings.ToLower(cv.Name), lowerSubstr) || strings.Contains(strings.ToLower(cv.Description), lowerSubstr) {
			printCallback(fmt.Sprintf("   %s (current value: %q)\n", cv.Name, cv.String))
			hits++
		}
	}

	if hits == 0 {
		printCallback(fmt.Sprintf("no cvars/commands contain %q\n", substr))
		return
	}

	plural := "s"
	if hits == 1 {
		plural = ""
	}
	printCallback(fmt.Sprintf("%d cvar%s/command%s containing %q\n", hits, plural, plural, substr))
}

func (c *CmdSystem) cmdAliasList() {
	aliases := c.Aliases()
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		printCallback(fmt.Sprintf("   %s : %s\n", name, aliases[name]))
	}

	printCallback(fmt.Sprintf("%d alias%s\n", len(names), pluralSuffix(len(names))))
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "es"
}

// AddAlias creates or overwrites a console alias. Aliases are user-defined
// shorthand: typing the alias name in the console expands to the stored command
// text. For example, "alias rj +jump;wait;+attack" lets the player type "rj"
// to execute a rocket-jump sequence. In Quake's original engine (Cmd_Alias_f
// in cmd.c), aliases are stored as a linked list; here we use a map for O(1)
// lookup. Alias expansion happens during executeLine after the command registry
// is checked, matching the original lookup priority.
func (c *CmdSystem) AddAlias(name, command string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.aliases[strings.ToLower(name)] = command
}

// RemoveAlias deletes a single alias by name. Returns true if the alias existed
// and was removed, false if no such alias was found. This is used by the
// "unalias" console command to let players remove individual aliases.
func (c *CmdSystem) RemoveAlias(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := strings.ToLower(name)
	if _, exists := c.aliases[key]; !exists {
		return false
	}
	delete(c.aliases, key)
	return true
}

// UnaliasAll removes every alias in the system. This is typically called when
// loading a new config or resetting to defaults, ensuring no stale aliases
// from a previous session persist.
func (c *CmdSystem) UnaliasAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.aliases)
}

// Alias looks up a single alias by name and returns its expansion text along
// with a boolean indicating whether the alias exists.
func (c *CmdSystem) Alias(name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	alias, exists := c.aliases[strings.ToLower(name)]
	return alias, exists
}

// Aliases returns a snapshot (shallow copy) of all registered aliases. The copy
// ensures callers can iterate safely without holding the lock. This is used by
// commands like "aliaslist" that enumerate all user-defined aliases.
func (c *CmdSystem) Aliases() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	aliases := make(map[string]string, len(c.aliases))
	for name, value := range c.aliases {
		aliases[name] = value
	}
	return aliases
}

// Complete returns all registered command names that begin with the given
// partial string. This powers the console's tab-completion feature — when the
// player presses Tab, the console collects matches from commands, aliases, and
// cvars to offer suggestions.
func (c *CmdSystem) Complete(partial string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	partial = strings.ToLower(partial)
	var matches []string
	for name := range c.commands {
		if strings.HasPrefix(name, partial) {
			matches = append(matches, name)
		}
	}
	return matches
}

// CompleteAliases returns all alias names that begin with the given partial
// string. This is the alias counterpart to Complete and is used by the
// console's tab-completion system to include aliases in the suggestion list
// alongside commands and cvars.
func (c *CmdSystem) CompleteAliases(partial string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	partial = strings.ToLower(partial)
	var matches []string
	for name := range c.aliases {
		if strings.HasPrefix(name, partial) {
			matches = append(matches, name)
		}
	}
	return matches
}

func (c *CmdSystem) CompleteCommandArgs(cmdName string, args []string, partial string) []string {
	c.mu.RLock()
	cmd := c.commands[strings.ToLower(cmdName)]
	c.mu.RUnlock()
	if cmd == nil || cmd.Completion == nil {
		return nil
	}
	return cmd.Completion(args, partial)
}
