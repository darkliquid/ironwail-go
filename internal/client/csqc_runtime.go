package client

// CSQCHandler handles Client-Side QuakeC runtime events from the client pump.
type CSQCHandler interface {
	Init() error
	Shutdown() error
	ParseStuffCmd(cmd string) bool
	EntUpdate(isNew bool)
}

func (c *Client) SetCSQCHandler(handler CSQCHandler) {
	if c == nil {
		return
	}
	c.CSQC = handler
}

func (c *Client) CSQCHandler() CSQCHandler {
	if c == nil {
		return nil
	}
	return c.CSQC
}

func (c *Client) handleCSQCInit() {
	if c != nil && c.CSQC != nil {
		_ = c.CSQC.Init()
	}
}

func (c *Client) handleCSQCShutdown() {
	if c != nil && c.CSQC != nil {
		_ = c.CSQC.Shutdown()
	}
}

func (c *Client) HandleCSQCEntUpdate(isNew bool) {
	if c != nil && c.CSQC != nil {
		c.CSQC.EntUpdate(isNew)
	}
}
