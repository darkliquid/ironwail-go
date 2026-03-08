// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"testing"
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

func TestUDPConnection(t *testing.T) {
	Init()
	netHostPort = 26001 // Use a different port for testing
	Listen(true)
	defer Listen(false)

	// Client connect in a goroutine because Connect blocks waiting for response
	var clientSock *Socket
	done := make(chan bool)
	go func() {
		clientSock = Connect("127.0.0.1:26001")
		done <- true
	}()

	// Server check connections
	var serverSock *Socket
	for i := 0; i < 100; i++ {
		serverSock = CheckNewConnections()
		if serverSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	<-done

	if clientSock == nil {
		t.Fatal("Failed to connect client")
	}
	defer Close(clientSock)

	if serverSock == nil {
		t.Fatal("Server failed to accept connection")
	}
	defer Close(serverSock)

	// Client send message
	msg := []byte("Hello Server")
	SendMessage(clientSock, msg)

	// Server receive message
	var receivedMsg []byte
	var msgType int
	for i := 0; i < 100; i++ {
		msgType, receivedMsg = GetMessage(serverSock)
		if msgType != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if msgType != 1 {
		t.Fatalf("Expected reliable message (1), got %d", msgType)
	}
	if string(receivedMsg) != "Hello Server" {
		t.Fatalf("Expected 'Hello Server', got '%s'", string(receivedMsg))
	}

	// Server send message
	SendMessage(serverSock, []byte("Hello Client"))

	// Client receive message
	for i := 0; i < 100; i++ {
		msgType, receivedMsg = GetMessage(clientSock)
		if msgType != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if msgType != 1 {
		t.Fatalf("Expected reliable message (1), got %d", msgType)
	}
	if string(receivedMsg) != "Hello Client" {
		t.Fatalf("Expected 'Hello Client', got '%s'", string(receivedMsg))
	}
}

func TestUDPUnreliable(t *testing.T) {
	Init()
	netHostPort = 26002
	Listen(true)
	defer Listen(false)

	var clientSock *Socket
	done := make(chan bool)
	go func() {
		clientSock = Connect("127.0.0.1:26002")
		done <- true
	}()

	var serverSock *Socket
	for i := 0; i < 100; i++ {
		serverSock = CheckNewConnections()
		if serverSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	<-done

	if clientSock == nil || serverSock == nil {
		t.Fatal("Failed to establish connection")
	}
	defer Close(clientSock)
	defer Close(serverSock)

	// Client send unreliable message
	SendUnreliableMessage(clientSock, []byte("Unreliable Server"))

	// Server receive message
	var receivedMsg []byte
	var msgType int
	for i := 0; i < 100; i++ {
		msgType, receivedMsg = GetMessage(serverSock)
		if msgType != 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if msgType != 2 {
		t.Fatalf("Expected unreliable message (2), got %d", msgType)
	}
	if string(receivedMsg) != "Unreliable Server" {
		t.Fatalf("Expected 'Unreliable Server', got '%s'", string(receivedMsg))
	}
}

func TestServerInfoHostnameFallback(t *testing.T) {
	hostname := cvar.Register("hostname", defaultServerInfoHostname, cvar.FlagServerInfo, "")
	oldHostname := hostname.String
	t.Cleanup(func() {
		cvar.Set(hostname.Name, oldHostname)
	})

	cvar.Set(hostname.Name, "")
	if got := serverInfoHostname(); got != defaultServerInfoHostname {
		t.Fatalf("serverInfoHostname() with empty cvar = %q, want %q", got, defaultServerInfoHostname)
	}

	cvar.Set(hostname.Name, "LAN Party")
	if got := serverInfoHostname(); got != "LAN Party" {
		t.Fatalf("serverInfoHostname() = %q, want %q", got, "LAN Party")
	}
}
