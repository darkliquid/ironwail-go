package cmdsys

import "strings"

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

func (c *CmdSystem) SetCommandCompletion(name string, completion func(args []string, partial string) []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cmd, ok := c.commands[strings.ToLower(name)]; ok {
		cmd.Completion = completion
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

// Exists checks whether a command with the given name is registered. This is
// used by other subsystems to avoid re-registering commands or to check
// whether a particular engine feature is available.
func (c *CmdSystem) Exists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.commands[strings.ToLower(name)]
	return exists
}
