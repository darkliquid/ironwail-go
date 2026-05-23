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
	"strings"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/cvar"
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
	// SrcAny marks commands that are explicitly safe from any command source.
	SrcAny
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
	Completion  func(args []string, partial string) []string
	SourceType  CommandSource
}

type bufferedText struct {
	text   string
	source CommandSource
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
	mu            sync.RWMutex        // Protects concurrent access to commands, aliases, and buffer.
	commands      map[string]*Command // Registry of all named commands, keyed by lowercase name.
	aliases       map[string]string   // User-defined alias expansions, keyed by lowercase alias name.
	buffer        []bufferedText      // Accumulated command text chunks waiting to be executed.
	waitCount     int                 // Tracks pending "wait" frames; see executeTextWithWait.
	source        CommandSource       // Current command source for handlers executing in this context.
	ForwardFunc   func(line string)   // Called for unrecognized commands (e.g., forward to server).
	printCallback func(string)        // Sink for command/cvar listing output; defaults to no-op.

	// CVar is the cvar registry used by cvar-coupled command handlers
	// (cvarlist, toggle, inc, reset, etc.) and by executeLine's fallback
	// "treat unknown command as cvar set". Set by the Host after constructing
	// both systems.
	CVar *cvar.CVarSystem
}

// SetPrintCallback installs the sink used for command/cvar listing output
// (cvarlist, cmdlist, apropos, etc). Passing nil resets the sink to a no-op.
func (c *CmdSystem) SetPrintCallback(fn func(string)) {
	if fn == nil {
		c.printCallback = func(string) {}
		return
	}
	c.printCallback = fn
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
		commands:      make(map[string]*Command),
		aliases:       make(map[string]string),
		source:        SrcCommand,
		printCallback: func(string) {},
	}
	// Register wait command
	cs.AddCommand("wait", func(args []string) {
		cs.waitCount++
	}, "Wait one frame before executing remaining commands")
	cs.AddCommand("cmdlist", func(args []string) {
		cs.cmdList(args)
	}, "List registered commands")
	cs.AddCommand("apropos", func(args []string) {
		cs.cmdApropos("apropos", args)
	}, "Search commands and cvars by substring")
	cs.AddCommand("find", func(args []string) {
		cs.cmdApropos("find", args)
	}, "Search commands and cvars by substring")
	cs.AddCommand("aliaslist", func(args []string) {
		cs.cmdAliasList()
	}, "List defined command aliases")
	return cs
}

// Init satisfies the host CommandBuffer interface; the constructor already
// installs the built-in commands so this is a no-op kept for symmetry with
// other host-managed subsystems.
func (c *CmdSystem) Init() {}

// Shutdown satisfies the host CommandBuffer interface. Nothing to release
// today, but the hook lets future implementations flush pending state.
func (c *CmdSystem) Shutdown() {}

func isReservedName(name string) bool {
	return strings.HasPrefix(name, "__")
}
