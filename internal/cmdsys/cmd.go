// Package cmdsys implements the Quake engine's command execution system.
//
// In id Software's original Quake architecture (cmd.c), the engine uses a
// text-based command system that serves as the backbone for all user
// interaction. Players type commands into the console (e.g., "map e1m1",
// "bind mouse1 +attack", "quit"), config files are executed line-by-line,
// and key bindings generate command strings. This package replicates that
// architecture in Go.
//
// The command system is distinct from the cvar (console variable) system:
//   - Commands are *actions* with registered callback functions (e.g., "map"
//     loads a level, "quit" exits the game, "bind" assigns a key).
//   - Cvars are *persistent named values* (e.g., "volume 0.8", "sensitivity 3").
//
// The execution pipeline works as follows:
//  1. Text is appended to a command buffer via AddText or InsertText.
//  2. Execute() drains the buffer, splitting it into individual commands
//     (separated by semicolons or newlines, respecting quoted strings).
//  3. Each command line is tokenized into arguments (like shell argv parsing).
//  4. The first token is looked up in the command registry, then aliases,
//     then cvars, in that order — matching Quake's original lookup priority.
//  5. If a command is found, its callback is invoked with the remaining args.
//
// This design allows the entire engine to be controlled via text, enabling
// config files, remote console (rcon), demo playback, and scripting — all
// through the same mechanism.
package cmdsys

import (
	"fmt"
	"strings"
	"sync"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

// CommandFunc is the signature for a command handler callback. When a console
// command is executed, its registered CommandFunc is called with the arguments
// that followed the command name. For example, typing "map e1m1" in the console
// would call the "map" command's handler with args = ["e1m1"].
// This mirrors Quake's xcommand_t function pointer typedef in cmd.c.
type CommandFunc func(args []string)

// CommandSource identifies where command text originated from.
// This mirrors cmd_source_t in Quake's cmd.c and is used by command handlers
// that need to restrict execution based on origin (e.g., client vs server).
type CommandSource int

const (
	// SrcCommand indicates command text from local console/config execution.
	SrcCommand CommandSource = iota
	// SrcClient indicates command text came from a client.
	SrcClient
	// SrcServer indicates command text came from server command injection.
	SrcServer
)

// Command represents a single registered console command. In Quake's original
// architecture (cmd_function_t in cmd.c), each command has a name used for
// lookup, a function pointer to execute, and a description for the "help" or
// "cmdlist" commands. Commands are the "verbs" of the engine — they perform
// actions like loading maps, binding keys, spawning entities, or quitting.
type Command struct {
	Name        string      // Canonical lowercase name used for console lookup.
	Description string      // Human-readable help text shown by "cmdlist" or tab-completion.
	Func        CommandFunc // Callback invoked when this command is executed.
	SourceType  CommandSource
}

// CmdSystem is the central command execution engine, analogous to the global
// state in Quake's cmd.c. It maintains three key data structures:
//   - commands: a registry of named commands with their handler callbacks.
//   - aliases: user-defined shorthand names that expand to other command text
//     (e.g., "alias rj \"rocket_jump\"" lets the player type "rj" instead).
//   - buffer: a text buffer that accumulates pending command text to be
//     executed on the next call to Execute(). This is how config files, key
//     bindings, and stuffcmd (server-to-client commands) inject work.
//
// The RWMutex allows concurrent reads (e.g., tab-completion, command lookup)
// while serializing writes (e.g., registering new commands, modifying aliases).
type CmdSystem struct {
	mu          sync.RWMutex        // Protects concurrent access to commands, aliases, and buffer.
	commands    map[string]*Command // Registry of all named commands, keyed by lowercase name.
	aliases     map[string]string   // User-defined alias expansions, keyed by lowercase alias name.
	buffer      strings.Builder     // Accumulated command text waiting to be executed.
	waitCount   int                 // Tracks pending "wait" frames; see executeTextWithWait.
	source      CommandSource       // Current command source for handlers executing in this context.
	ForwardFunc func(line string)   // Called for unrecognized commands (e.g., forward to server).
}

// globalCmd is the package-level singleton CmdSystem instance. Quake's original
// engine uses global state extensively; this singleton preserves that pattern
// while allowing the package-level convenience functions (AddCommand, Execute,
// etc.) to delegate to a single shared instance. Test code or embedded usage
// can create isolated instances via NewCmdSystem().
var globalCmd = NewCmdSystem()
var printCallback = func(string) {}

func SetPrintCallback(fn func(string)) {
	if fn == nil {
		printCallback = func(string) {}
		return
	}
	printCallback = fn
}

// NewCmdSystem creates and returns a new, independent command system instance.
// It initializes the command and alias registries and registers the built-in
// "wait" command. In Quake's original architecture, "wait" is a special command
// that pauses command buffer execution until the next frame. This is essential
// for config scripts that need to space actions across frames — for example,
// a jump-shoot macro might use "wait" between +jump and +attack so the engine
// processes each action on separate simulation ticks.
func NewCmdSystem() *CmdSystem {
	cs := &CmdSystem{
		commands: make(map[string]*Command),
		aliases:  make(map[string]string),
		source:   SrcCommand,
	}
	// Register wait command
	cs.AddCommand("wait", func(args []string) {
		cs.waitCount++
	}, "Wait one frame before executing remaining commands")
	return cs
}

// Init is a placeholder for future initialization logic. In the original Quake
// engine, Cmd_Init() registers built-in commands like "exec", "echo", "alias",
// "wait", and "cmd". As this Go port matures, subsystem-specific command
// registration will be added here.
func (c *CmdSystem) Init() {
}

// AddCommand registers a new console command with the given name, handler
// function, and description. The name is normalized to lowercase because
// Quake's console is case-insensitive. If a command with the same name already
// exists, the registration is silently ignored — this prevents subsystems from
// accidentally overwriting each other's commands during initialization.
//
// This is the Go equivalent of Quake's Cmd_AddCommand() in cmd.c.
func (c *CmdSystem) AddCommand(name string, fn CommandFunc, desc string) {
	c.AddCommandForSource(name, fn, desc, SrcCommand)
}

func (c *CmdSystem) AddCommandForSource(name string, fn CommandFunc, desc string, sourceType CommandSource) {
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
		SourceType:  sourceType,
	}
}

func (c *CmdSystem) AddClientCommand(name string, fn CommandFunc, desc string) {
	c.AddCommandForSource(name, fn, desc, SrcClient)
}

func (c *CmdSystem) AddServerCommand(name string, fn CommandFunc, desc string) {
	c.AddCommandForSource(name, fn, desc, SrcServer)
}

// RemoveCommand unregisters a console command by name. This is used when a
// subsystem shuts down and needs to clean up its commands — for example, when
// disconnecting from a server, game-specific commands might be removed.
func (c *CmdSystem) RemoveCommand(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.commands, strings.ToLower(name))
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

// AddText appends command text to the end of the command buffer. This is the
// primary way external systems inject commands for deferred execution. For
// example, when a config file is loaded via "exec autoexec.cfg", the entire
// file contents are passed to AddText, and the commands run on the next
// Execute() call. A trailing newline is appended if not already present.
//
// This corresponds to Cbuf_AddText() in Quake's cmd.c — "Cbuf" being the
// "command buffer" that accumulates text between frames.
func (c *CmdSystem) AddText(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buffer.WriteString(text)
	if !strings.HasSuffix(text, "\n") {
		c.buffer.WriteByte('\n')
	}
}

// InsertText prepends command text to the front of the command buffer, before
// any already-buffered text. This is critical for commands that need immediate
// execution before anything else in the queue — for example, Quake's "exec"
// command uses insert semantics so that the contents of the executed config
// file run before any commands that were already buffered after the "exec"
// call. Without this, command ordering would be violated.
//
// This corresponds to Cbuf_InsertText() in Quake's cmd.c.
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

func (c *CmdSystem) drainBuffer() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	text := c.buffer.String()
	c.buffer.Reset()
	return text
}

// Execute drains the command buffer and executes all buffered command text.
// This is called once per frame in the engine's main loop (analogous to
// Cbuf_Execute() in Quake's cmd.c). It atomically grabs the current buffer
// contents and resets it, then processes each command. The "wait" command can
// interrupt execution, deferring remaining commands to the next frame.
func (c *CmdSystem) Execute() {
	c.ExecuteWithSource(SrcCommand)
}

// ExecuteWithSource drains the command buffer and executes the buffered command
// text under the provided command source.
func (c *CmdSystem) ExecuteWithSource(source CommandSource) {
	text := c.drainBuffer()

	c.withSource(source, func() {
		c.executeTextWithWait(text)
	})
}

// ExecuteText immediately executes the given command text without going through
// the command buffer. This is used for commands that must run synchronously,
// such as processing a single console line typed by the player. Unlike
// AddText+Execute, this does not support the "wait" command's frame-delay.
func (c *CmdSystem) ExecuteText(text string) {
	c.ExecuteTextWithSource(text, SrcCommand)
}

// ExecuteTextWithSource immediately executes command text under the provided
// command source.
func (c *CmdSystem) ExecuteTextWithSource(text string, source CommandSource) {
	c.withSource(source, func() {
		c.executeText(text, nil)
	})
}

// SetSource sets the current command source.
func (c *CmdSystem) SetSource(source CommandSource) {
	c.mu.Lock()
	c.source = source
	c.mu.Unlock()
}

// Source returns the current command source.
func (c *CmdSystem) Source() CommandSource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.source
}

func (c *CmdSystem) withSource(source CommandSource, fn func()) {
	c.mu.Lock()
	prev := c.source
	c.source = source
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.source = prev
		c.mu.Unlock()
	}()

	fn()
}

// executeTextWithWait processes command text with support for the "wait"
// command. When "wait" is encountered, execution stops and any remaining
// commands are pushed back into the buffer for the next frame's Execute()
// call. This mechanism allows config scripts and aliases to space actions
// across multiple engine frames — essential for sequences like:
//
//	alias rocket_jump "+jump; wait; +attack; wait; -attack; -jump"
//
// Each "wait" causes the engine to process one simulation frame before
// continuing, so the jump happens before the attack.
func (c *CmdSystem) executeTextWithWait(text string) {
	pending := splitCommands(text)
	for len(pending) > 0 {
		line := strings.TrimSpace(pending[0])
		pending = pending[1:]
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		c.executeLine(line, nil)

		// If a wait command was executed, put remaining commands back in buffer
		// ahead of any newly buffered text and stop until the next Execute().
		if c.waitCount > 0 {
			c.waitCount--
			if len(pending) > 0 {
				remaining := strings.Join(pending, "\n")
				c.mu.Lock()
				existing := c.buffer.String()
				c.buffer.Reset()
				c.buffer.WriteString(remaining)
				if existing != "" {
					c.buffer.WriteString("\n")
					c.buffer.WriteString(existing)
				}
				c.mu.Unlock()
			}
			return
		}

		// Quake's command buffer processes inserted text immediately, before
		// any remaining commands from the current execution pass. This matters
		// for `exec`/`stuffcmds`, whose InsertText calls must preempt later
		// lines like `startdemos` in quake.rc.
		if injected := c.drainBuffer(); injected != "" {
			if len(pending) > 0 {
				injected += "\n" + strings.Join(pending, "\n")
			}
			pending = splitCommands(injected)
		}
	}
}

// executeText splits text into individual commands and executes each one. The
// "expanding" parameter tracks which aliases are currently being expanded in the
// call stack to prevent infinite recursion (e.g., "alias loop loop" would loop
// forever without this guard). When called from ExecuteText, expanding starts
// as nil and is lazily initialized on first alias expansion.
func (c *CmdSystem) executeText(text string, expanding map[string]struct{}) {
	for _, line := range splitCommands(text) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		c.executeLine(line, expanding)
	}
}

// executeLine processes a single command line through the three-tier lookup
// that mirrors Quake's original Cmd_ExecuteString() in cmd.c:
//
//  1. Commands: Check the registered command map first. If found, invoke its
//     callback with the parsed arguments (args[1:], since args[0] is the name).
//  2. Aliases: If no command matched, check aliases. If found, recursively
//     execute the alias expansion text. The "expanding" set prevents infinite
//     alias recursion (e.g., "alias a a" would deadlock without this guard).
//  3. Cvars: If neither command nor alias matched, check if the name is a
//     known cvar. If so and arguments were provided, set the cvar's value.
//     This is how typing "sensitivity 5" in the console sets the cvar.
//
// This three-tier priority (command > alias > cvar) is a defining feature of
// Quake's console architecture and is preserved exactly here.
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
		switch c.Source() {
		case SrcClient:
			if cmd.SourceType != SrcClient {
				return
			}
		case SrcCommand:
			if cmd.SourceType == SrcServer {
				goto fallback
			}
		case SrcServer:
			if cmd.SourceType != SrcServer {
				return
			}
		}
		cmd.Func(args[1:])
		return
	}

fallback:
	if c.Source() != SrcCommand {
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

	if cv := cvar.Get(cmdName); cv != nil {
		if len(args) > 1 {
			cvar.Set(cmdName, strings.Join(args[1:], " "))
		} else {
			printCallback(fmt.Sprintf("\"%s\" is \"%s\"\n", cv.Name, cv.String))
		}
		return
	}

	// No command, alias, or cvar matched — forward to server if connected.
	// This matches C Cmd_ForwardToServer() which sends unrecognized commands
	// as clc_stringcmd messages so "say", "name", "color" etc. work in MP.
	if c.ForwardFunc != nil {
		c.ForwardFunc(line)
		return
	}
	printCallback(fmt.Sprintf("Unknown command \"%s\"\n", args[0]))
}

// Exists checks whether a command with the given name is registered. This is
// used by other subsystems to avoid re-registering commands or to check
// whether a particular engine feature is available.
func (c *CmdSystem) Exists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.commands[strings.ToLower(name)]
	return exists
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

// splitCommands splits a block of command text into individual command strings.
// Commands are delimited by semicolons (;) or newlines, but these delimiters
// are ignored inside quoted strings so that arguments containing semicolons
// (e.g., "say hello; world") are not incorrectly split. Backslash escapes
// within quotes are also respected.
//
// This is the tokenization stage of the command pipeline — it takes raw text
// from the command buffer (which might contain an entire config file or a
// multi-command alias) and produces a slice of individual command strings,
// each ready to be parsed into argv-style tokens by parseCommand.
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

// parseCommand tokenizes a single command line into an argv-style slice of
// arguments, analogous to Cmd_TokenizeString() in Quake's cmd.c. Arguments
// are separated by spaces, with double-quoted strings treated as single tokens
// (allowing spaces within arguments). Within quotes, standard backslash escape
// sequences are supported: \\" (literal quote), \\\\ (literal backslash),
// \\n (newline), \\r (carriage return), \\t (tab).
//
// The first element of the returned slice is always the command name; the
// remaining elements are the arguments passed to the command's handler.
func parseCommand(line string) []string {
	var args []string
	var current strings.Builder
	inQuote := false

	for i := 0; i < len(line); i++ {
		ch := line[i]

		switch {
		case ch == '/' && !inQuote && i+1 < len(line) && line[i+1] == '/':
			// Strip // comments outside quotes, matching C COM_Parse behavior
			goto done
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
		case (ch == ' ' || ch == '\t') && !inQuote:
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(ch)
		}
	}
done:

	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args
}

// ---------------------------------------------------------------------------
// Package-level convenience functions
// ---------------------------------------------------------------------------
// The functions below delegate to the global singleton CmdSystem (globalCmd).
// They provide a flat API for the rest of the engine, matching the procedural
// style of the original C codebase where functions like Cmd_AddCommand() and
// Cbuf_AddText() operate on implicit global state.
// ---------------------------------------------------------------------------

// AddCommand registers a command on the global command system.
// See [CmdSystem.AddCommand] for details.
func AddCommand(name string, fn CommandFunc, desc string) {
	globalCmd.AddCommand(name, fn, desc)
}

func AddClientCommand(name string, fn CommandFunc, desc string) {
	globalCmd.AddClientCommand(name, fn, desc)
}

func AddServerCommand(name string, fn CommandFunc, desc string) {
	globalCmd.AddServerCommand(name, fn, desc)
}

// RemoveCommand unregisters a command from the global command system.
func RemoveCommand(name string) {
	globalCmd.RemoveCommand(name)
}

// AddAlias creates or overwrites an alias on the global command system.
func AddAlias(name, command string) {
	globalCmd.AddAlias(name, command)
}

// RemoveAlias removes a single alias from the global command system.
func RemoveAlias(name string) bool {
	return globalCmd.RemoveAlias(name)
}

// UnaliasAll removes all aliases from the global command system.
func UnaliasAll() {
	globalCmd.UnaliasAll()
}

// Alias looks up an alias on the global command system.
func Alias(name string) (string, bool) {
	return globalCmd.Alias(name)
}

// Aliases returns all aliases from the global command system.
func Aliases() map[string]string {
	return globalCmd.Aliases()
}

// AddText appends command text to the global command buffer.
func AddText(text string) {
	globalCmd.AddText(text)
}

// InsertText prepends command text to the front of the global command buffer.
func InsertText(text string) {
	globalCmd.InsertText(text)
}

// Execute drains and executes the global command buffer.
func Execute() {
	globalCmd.Execute()
}

// ExecuteWithSource drains and executes the global command buffer using source.
func ExecuteWithSource(source CommandSource) {
	globalCmd.ExecuteWithSource(source)
}

// ExecuteText immediately executes command text on the global command system.
func ExecuteText(text string) {
	globalCmd.ExecuteText(text)
}

// ExecuteTextWithSource immediately executes command text using source.
func ExecuteTextWithSource(text string, source CommandSource) {
	globalCmd.ExecuteTextWithSource(text, source)
}

// SetSource sets the current source on the global command system.
func SetSource(source CommandSource) {
	globalCmd.SetSource(source)
}

// Source returns the current source from the global command system.
func Source() CommandSource {
	return globalCmd.Source()
}

// Exists checks whether a command is registered on the global command system.
func Exists(name string) bool {
	return globalCmd.Exists(name)
}

// Complete returns command name completions from the global command system.
func Complete(partial string) []string {
	return globalCmd.Complete(partial)
}

// CompleteAliases returns alias name completions from the global command system.
func CompleteAliases(partial string) []string {
	return globalCmd.CompleteAliases(partial)
}

// SetForwardFunc sets the callback for unrecognized commands on the global
// command system. When a command is not found as a registered command, alias,
// or cvar, this function is called with the full command line. In Quake, this
// forwards the command to the remote server (Cmd_ForwardToServer).
func SetForwardFunc(fn func(line string)) {
	globalCmd.ForwardFunc = fn
}
