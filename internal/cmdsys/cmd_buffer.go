package cmdsys

import (
	"fmt"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// AddText appends command text to the end of the command buffer. This is the
// primary way external systems inject commands for deferred execution. For
// example, when a config file is loaded via "exec autoexec.cfg", the entire
// file contents are passed to AddText, and the commands run on the next
// Execute() call. A trailing newline is appended if not already present.
//
// This corresponds to Cbuf_AddText() in Quake's cmd.c — "Cbuf" being the
// "command buffer" that accumulates text between frames.
func normalizeBufferedText(text string) string {
	if !strings.HasSuffix(text, "\n") {
		return text + "\n"
	}
	return text
}

func (c *CmdSystem) addBufferedTextLocked(text string, source CommandSource) {
	c.buffer = append(c.buffer, bufferedText{
		text:   normalizeBufferedText(text),
		source: source,
	})
}

func (c *CmdSystem) prependBufferedTextLocked(text string, source CommandSource) {
	c.buffer = append([]bufferedText{{
		text:   normalizeBufferedText(text),
		source: source,
	}}, c.buffer...)
}

func (c *CmdSystem) prependBufferedEntries(entries []bufferedText) {
	if len(entries) == 0 {
		return
	}
	c.mu.Lock()
	c.buffer = append(append([]bufferedText(nil), entries...), c.buffer...)
	c.mu.Unlock()
}

func (c *CmdSystem) AddText(text string) {
	c.AddTextWithSource(text, c.Source())
}

// AddTextWithSource appends command text to the end of the command buffer and
// records its command source for later buffered execution.
func (c *CmdSystem) AddTextWithSource(text string, source CommandSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.addBufferedTextLocked(text, source)
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
	c.InsertTextWithSource(text, c.Source())
}

// InsertTextWithSource prepends command text to the front of the command buffer
// and records its command source for later buffered execution.
func (c *CmdSystem) InsertTextWithSource(text string, source CommandSource) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prependBufferedTextLocked(text, source)
}

func (c *CmdSystem) drainBuffer() []bufferedText {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.buffer) == 0 {
		return nil
	}
	text := append([]bufferedText(nil), c.buffer...)
	c.buffer = nil
	return text
}

// Execute drains the command buffer and executes all buffered command text.
// This is called once per frame in the engine's main loop (analogous to
// Cbuf_Execute() in Quake's cmd.c). It atomically grabs the current buffer
// contents and resets it, then processes each command. The "wait" command can
// interrupt execution, deferring remaining commands to the next frame.
func (c *CmdSystem) Execute() {
	c.executeBufferedEntries(c.drainBuffer())
}

// ExecuteWithSource drains the command buffer and executes the buffered command
// text under the provided command source.
func (c *CmdSystem) ExecuteWithSource(source CommandSource) {
	entries := c.drainBuffer()
	for i := range entries {
		entries[i].source = source
	}
	c.executeBufferedEntries(entries)
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
	c.executeBufferedEntries([]bufferedText{{
		text:   text,
		source: c.Source(),
	}})
}

func (c *CmdSystem) executeBufferedEntries(entries []bufferedText) {
	pending := append([]bufferedText(nil), entries...)
	for len(pending) > 0 {
		entry := pending[0]
		pending = pending[1:]

		lines := splitCommands(entry.text)
		for len(lines) > 0 {
			line := strings.TrimSpace(lines[0])
			lines = lines[1:]
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}

			c.withSource(entry.source, func() {
				c.executeLine(line, nil)
			})

			if c.waitCount > 0 {
				c.waitCount--
				var requeue []bufferedText
				if len(lines) > 0 {
					requeue = append(requeue, bufferedText{
						text:   strings.Join(lines, "\n"),
						source: entry.source,
					})
				}
				requeue = append(requeue, pending...)
				c.prependBufferedEntries(requeue)
				return
			}

			if injected := c.drainBuffer(); len(injected) > 0 {
				if len(lines) > 0 {
					injected = append(injected, bufferedText{
						text:   strings.Join(lines, "\n"),
						source: entry.source,
					})
				}
				pending = append(injected, pending...)
				break
			}
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
// that mirrors Quake's original Cmd_ExecuteString() in cmd.c.
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
			if cv.DefaultValue != "" {
				if cv.String == cv.DefaultValue {
					printCallback(fmt.Sprintf("\"%s\" is \"%s\" (default)\n", cv.Name, cv.String))
				} else {
					printCallback(fmt.Sprintf("\"%s\" is \"%s\" (default: \"%s\")\n", cv.Name, cv.String, cv.DefaultValue))
				}
			} else {
				printCallback(fmt.Sprintf("\"%s\" is \"%s\"\n", cv.Name, cv.String))
			}
		}
		return
	}

	if c.ForwardFunc != nil {
		c.ForwardFunc(line)
		return
	}
	c.listAllContaining(args[0])
}

func splitCommands(text string) []string {
	var (
		commands       []string
		current        strings.Builder
		inQuote        bool
		escaped        bool
		inLineComment  bool
		inBlockComment bool
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
		if inLineComment {
			switch ch {
			case '\n':
				inLineComment = false
				flush()
			case '\r':
				inLineComment = false
				flush()
				if i+1 < len(text) && text[i+1] == '\n' {
					i++
				}
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(text) && text[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if ch == '\\' && inQuote {
			current.WriteByte(ch)
			escaped = true
			continue
		}
		if ch == '/' && !inQuote && i+1 < len(text) {
			switch text[i+1] {
			case '/':
				i++
				inLineComment = true
				continue
			case '*':
				i++
				inBlockComment = true
				continue
			}
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
		case ch == '/' && !inQuote && i+1 < len(line) && line[i+1] == '/':
			goto done
		case ch == '/' && !inQuote && i+1 < len(line) && line[i+1] == '*':
			i += 2
			for i+1 < len(line) && !(line[i] == '*' && line[i+1] == '/') {
				i++
			}
			if i+1 >= len(line) {
				goto done
			}
			i++
			if current.Len() > 0 && i+1 < len(line) && (line[i+1] == ' ' || line[i+1] == '\t') {
				args = append(args, current.String())
				current.Reset()
			}
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
