package cmdsys

import (
	"strings"
	"sync"
)

type CommandFunc func(args []string)

type Command struct {
	Name        string
	Description string
	Func        CommandFunc
}

type CmdSystem struct {
	mu        sync.RWMutex
	commands  map[string]*Command
	aliases   map[string]string
	buffer    strings.Builder
	waitCount int
}

var globalCmd = NewCmdSystem()

func NewCmdSystem() *CmdSystem {
	return &CmdSystem{
		commands: make(map[string]*Command),
		aliases:  make(map[string]string),
	}
}

func (c *CmdSystem) Init() {
}

func (c *CmdSystem) AddCommand(name string, fn CommandFunc, desc string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	name = strings.ToLower(name)
	if _, exists := c.commands[name]; exists {
		return
	}

	c.commands[name] = &Command{
		Name:        name,
		Func:        fn,
		Description: desc,
	}
}

func (c *CmdSystem) RemoveCommand(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.commands, strings.ToLower(name))
}

func (c *CmdSystem) AddAlias(name, command string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.aliases[strings.ToLower(name)] = command
}

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

func (c *CmdSystem) UnaliasAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.aliases)
}

func (c *CmdSystem) Alias(name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	alias, exists := c.aliases[strings.ToLower(name)]
	return alias, exists
}

func (c *CmdSystem) Aliases() map[string]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	aliases := make(map[string]string, len(c.aliases))
	for name, value := range c.aliases {
		aliases[name] = value
	}
	return aliases
}

func (c *CmdSystem) AddText(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buffer.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		c.buffer.WriteByte('\n')
	}
}

func (c *CmdSystem) InsertText(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing := c.buffer.String()
	c.buffer.Reset()
	c.buffer.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		c.buffer.WriteByte('\n')
	}
	c.buffer.WriteString(existing)
}

func (c *CmdSystem) Execute() {
	c.mu.Lock()
	text := c.buffer.String()
	c.buffer.Reset()
	c.mu.Unlock()

	c.ExecuteText(text)
}

func (c *CmdSystem) ExecuteText(text string) {
	c.executeText(text, nil)
}

func (c *CmdSystem) executeText(text string, expanding map[string]struct{}) {
	for _, line := range splitCommands(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		c.executeLine(line, expanding)
	}
}

func (c *CmdSystem) executeLine(line string, expanding map[string]struct{}) {
	args := parseCommand(line)
	if len(args) == 0 {
		return
	}

	cmdName := strings.ToLower(args[0])

	c.mu.RLock()
	cmd, exists := c.commands[cmdName]
	c.mu.RUnlock()

	if exists && cmd.Func != nil {
		cmd.Func(args[1:])
		return
	}

	c.mu.RLock()
	if alias, exists := c.aliases[cmdName]; exists {
		c.mu.RUnlock()
		if expanding == nil {
			expanding = make(map[string]struct{})
		}
		if _, exists := expanding[cmdName]; exists {
			return
		}
		expanding[cmdName] = struct{}{}
		c.executeText(alias, expanding)
		delete(expanding, cmdName)
		return
	}
	c.mu.RUnlock()
}

func (c *CmdSystem) Exists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.commands[strings.ToLower(name)]
	return exists
}

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

func splitCommands(text string) []string {
	var (
		commands []string
		current  strings.Builder
		inQuote  bool
		escaped  bool
	)

	flush := func() {
		command := strings.TrimSpace(current.String())
		if command != "" {
			commands = append(commands, command)
		}
		current.Reset()
	}

	for i := 0; i < len(text); i++ {
		ch := text[i]

		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inQuote {
			current.WriteByte(ch)
			escaped = true
			continue
		}

		switch ch {
		case '"':
			inQuote = !inQuote
			current.WriteByte(ch)
		case ';':
			if inQuote {
				current.WriteByte(ch)
				continue
			}
			flush()
		case '\n':
			if inQuote {
				current.WriteByte(ch)
				continue
			}
			flush()
		case '\r':
			if inQuote {
				current.WriteByte(ch)
				continue
			}
			flush()
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
		default:
			current.WriteByte(ch)
		}
	}

	flush()
	return commands
}

func parseCommand(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		switch {
		case ch == '\\' && inQuote && i+1 < len(line):
			switch line[i+1] {
			case '"', '\\':
				current.WriteByte(line[i+1])
				i++
			case 'n':
				current.WriteByte('\n')
				i++
			case 'r':
				current.WriteByte('\r')
				i++
			case 't':
				current.WriteByte('\t')
				i++
			default:
				current.WriteByte(ch)
			}
		case ch == '"':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

func AddCommand(name string, fn CommandFunc, desc string) {
	globalCmd.AddCommand(name, fn, desc)
}

func RemoveCommand(name string) {
	globalCmd.RemoveCommand(name)
}

func AddAlias(name, command string) {
	globalCmd.AddAlias(name, command)
}

func RemoveAlias(name string) bool {
	return globalCmd.RemoveAlias(name)
}

func UnaliasAll() {
	globalCmd.UnaliasAll()
}

func Alias(name string) (string, bool) {
	return globalCmd.Alias(name)
}

func Aliases() map[string]string {
	return globalCmd.Aliases()
}

func AddText(text string) {
	globalCmd.AddText(text)
}

func InsertText(text string) {
	globalCmd.InsertText(text)
}

func Execute() {
	globalCmd.Execute()
}

func ExecuteText(text string) {
	globalCmd.ExecuteText(text)
}

func Exists(name string) bool {
	return globalCmd.Exists(name)
}

func Complete(partial string) []string {
	return globalCmd.Complete(partial)
}

func CompleteAliases(partial string) []string {
	return globalCmd.CompleteAliases(partial)
}
