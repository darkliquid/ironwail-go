// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"fmt"
	"strings"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

type signonCommandClient interface {
	SendSignonCommand(command string) error
}

type reconnectResetClient interface {
	ResetConnectionState() error
}

type remoteDatagramClient struct {
	inner  *cl.Client
	parser *cl.Parser
	socket *inet.Socket

	lastSignonReply   int
	signonName        string
	signonColor       int
	spawnArgs         string
	lastServerMessage []byte
}

func newRemoteDatagramClient(socket *inet.Socket) *remoteDatagramClient {
	inner := cl.NewClient()
	return &remoteDatagramClient{
		inner:       inner,
		parser:      cl.NewParser(inner),
		socket:      socket,
		signonName:  defaultClientName,
		signonColor: 0,
	}
}

func (c *remoteDatagramClient) Init() error {
	if c == nil {
		return fmt.Errorf("remote client is nil")
	}
	if c.inner == nil {
		c.inner = cl.NewClient()
	}
	if c.parser == nil {
		c.parser = cl.NewParser(c.inner)
	}
	c.inner.ClearState()
	c.lastSignonReply = 0
	c.lastServerMessage = nil
	return nil
}

func (c *remoteDatagramClient) RuntimeState() *cl.Client {
	if c == nil {
		return nil
	}
	return c.inner
}

func (c *remoteDatagramClient) Frame(frameTime float64) error {
	if c == nil || c.inner == nil {
		return nil
	}
	if c.inner.Signon < cl.Signons {
		return nil
	}
	c.inner.AccumulateCmd(float32(frameTime))
	return nil
}

func (c *remoteDatagramClient) Shutdown() {
	if c == nil {
		return
	}
	if c.socket != nil {
		inet.Close(c.socket)
		c.socket = nil
	}
	if c.inner != nil {
		c.inner.ClearState()
		c.inner.State = cl.StateDisconnected
	}
	c.lastSignonReply = 0
	c.lastServerMessage = nil
}

func (c *remoteDatagramClient) State() ClientState {
	if c == nil || c.inner == nil {
		return caDisconnected
	}
	switch c.inner.State {
	case cl.StateConnected:
		return caConnected
	case cl.StateActive:
		return caActive
	default:
		return caDisconnected
	}
}

func (c *remoteDatagramClient) ReadFromServer() error {
	if c == nil || c.socket == nil || c.inner == nil || c.parser == nil {
		return nil
	}
	for {
		msgType, data := inet.GetMessage(c.socket)
		if msgType <= 0 {
			break
		}
		if len(data) == 0 {
			continue
		}
		c.lastServerMessage = append(c.lastServerMessage[:0], data...)
		if msgType == 3 {
			// Control message (disconnect, etc.)
			if len(data) > 0 && data[0] == 0x82 { // CCRepReject used as disconnect
				c.inner.State = cl.StateDisconnected
				break
			}
			continue
		}
		if err := c.parser.ParseServerMessage(data); err != nil {
			return fmt.Errorf("parse remote server message: %w", err)
		}
	}
	return nil
}

func (c *remoteDatagramClient) LastServerMessage() []byte {
	if len(c.lastServerMessage) == 0 {
		return nil
	}
	return append([]byte(nil), c.lastServerMessage...)
}

func (c *remoteDatagramClient) SendCommand() error {
	if c == nil || c.socket == nil || c.inner == nil {
		return nil
	}
	if c.inner.Signon >= 1 && c.inner.Signon < cl.Signons {
		if c.inner.Signon != c.lastSignonReply {
			commands, ok := cl.SignonReplyCommands(c.inner.Signon, c.playerName(), c.playerColor(), c.spawnArgs)
			if !ok {
				return fmt.Errorf("unsupported signon stage %d", c.inner.Signon)
			}
			for _, command := range commands {
				if err := c.SendSignonCommand(command); err != nil {
					return err
				}
			}
			c.lastSignonReply = c.inner.Signon
		}
		return nil
	}
	if c.inner.Signon >= cl.Signons {
		c.lastSignonReply = 0
	}
	return c.inner.SendCmd(func(data []byte) error {
		if sent := inet.SendUnreliableMessage(c.socket, data); sent != 1 {
			return fmt.Errorf("failed to send unreliable command")
		}
		return nil
	})
}

func (c *remoteDatagramClient) playerName() string {
	if c == nil || strings.TrimSpace(c.signonName) == "" {
		return defaultClientName
	}
	return c.signonName
}

func (c *remoteDatagramClient) playerColor() int {
	if c == nil {
		return 0
	}
	return c.signonColor
}

func (c *remoteDatagramClient) SendSignonCommand(command string) error {
	if c == nil || c.socket == nil || c.inner == nil {
		return fmt.Errorf("remote client not connected")
	}
	data, err := c.inner.SendStringCmd(command)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if sent := inet.SendUnreliableMessage(c.socket, data); sent != 1 {
		return fmt.Errorf("failed to send signon command %q", command)
	}
	return nil
}

func (c *remoteDatagramClient) SendStringCmd(command string) error {
	if c == nil || c.socket == nil || c.inner == nil {
		return fmt.Errorf("remote client not connected")
	}
	data, err := c.inner.SendStringCmd(command)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if sent := inet.SendUnreliableMessage(c.socket, data); sent != 1 {
		return fmt.Errorf("failed to send command %q", command)
	}
	return nil
}

func (c *remoteDatagramClient) ResetConnectionState() error {
	if c == nil || c.inner == nil {
		return fmt.Errorf("remote client not initialized")
	}
	c.inner.ClearState()
	c.inner.State = cl.StateConnected
	c.lastSignonReply = 0
	return nil
}

func (c *remoteDatagramClient) ClientState() *cl.Client {
	if c == nil {
		return nil
	}
	return c.inner
}
