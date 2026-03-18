// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"encoding/binary"
	stdnet "net"
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

func TestUDPConnectionsUsePerClientSockets(t *testing.T) {
	Init()
	netHostPort = 26003
	Listen(true)
	defer Listen(false)

	type connectResult struct {
		sock *Socket
	}

	results := make(chan connectResult, 2)
	go func() { results <- connectResult{sock: Connect("127.0.0.1:26003")} }()
	go func() { results <- connectResult{sock: Connect("127.0.0.1:26003")} }()

	serverSocks := make([]*Socket, 0, 2)
	deadline := time.Now().Add(2 * time.Second)
	for len(serverSocks) < 2 && time.Now().Before(deadline) {
		if sock := CheckNewConnections(); sock != nil {
			serverSocks = append(serverSocks, sock)
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if len(serverSocks) != 2 {
		t.Fatalf("expected 2 server sockets, got %d", len(serverSocks))
	}
	defer Close(serverSocks[0])
	defer Close(serverSocks[1])

	clientA := (<-results).sock
	clientB := (<-results).sock
	if clientA == nil || clientB == nil {
		t.Fatal("failed to connect both clients")
	}
	defer Close(clientA)
	defer Close(clientB)

	if acceptSocket == nil {
		t.Fatal("accept socket should be open while listening")
	}
	acceptPort := acceptSocket.LocalAddr().(*stdnet.UDPAddr).Port

	serverPortA := serverSocks[0].udpConn.LocalAddr().(*stdnet.UDPAddr).Port
	serverPortB := serverSocks[1].udpConn.LocalAddr().(*stdnet.UDPAddr).Port
	if serverPortA == acceptPort || serverPortB == acceptPort {
		t.Fatalf("server client socket reused accept socket port: accept=%d, clientPorts=%d/%d", acceptPort, serverPortA, serverPortB)
	}
	if serverPortA == serverPortB {
		t.Fatalf("expected distinct per-client server ports, both were %d", serverPortA)
	}

	clientPortA := clientA.udpConn.LocalAddr().(*stdnet.UDPAddr).Port
	clientPortB := clientB.udpConn.LocalAddr().(*stdnet.UDPAddr).Port
	var serverForA, serverForB *Socket
	for _, s := range serverSocks {
		switch s.remoteAddr.Port {
		case clientPortA:
			serverForA = s
		case clientPortB:
			serverForB = s
		}
	}
	if serverForA == nil || serverForB == nil {
		t.Fatalf("could not map server sockets to clients (client ports %d/%d)", clientPortA, clientPortB)
	}

	// Regression guard: if sockets are shared, polling the wrong server socket
	// can consume-and-drop another client's packet.
	if SendMessage(clientA, []byte("for-client-a")) != 1 {
		t.Fatal("failed to send message from client A")
	}

	for i := 0; i < 20; i++ {
		serverForB.udpConn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
		msgType, _ := GetMessage(serverForB)
		if msgType != 0 {
			t.Fatalf("server socket for client B received unexpected message type %d", msgType)
		}
	}
	serverForB.udpConn.SetReadDeadline(time.Time{})

	var gotType int
	var gotData []byte
	for i := 0; i < 100; i++ {
		serverForA.udpConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		gotType, gotData = GetMessage(serverForA)
		if gotType != 0 {
			break
		}
	}
	serverForA.udpConn.SetReadDeadline(time.Time{})
	if gotType != 1 || string(gotData) != "for-client-a" {
		t.Fatalf("server socket for client A got type=%d data=%q, want type=1 data=%q", gotType, string(gotData), "for-client-a")
	}
}

func TestCCRepAcceptReportsPerClientSocketPort(t *testing.T) {
	Init()
	netHostPort = 26004
	Listen(true)
	defer Listen(false)

	if acceptSocket == nil {
		t.Fatal("accept socket should be open while listening")
	}
	acceptPort := acceptSocket.LocalAddr().(*stdnet.UDPAddr).Port

	clientConn, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("failed to open client udp socket: %v", err)
	}
	defer UDPCloseSocket(clientConn)

	serverAddr, err := UDPStringToAddr("127.0.0.1:26004")
	if err != nil {
		t.Fatalf("failed to parse server address: %v", err)
	}

	req := make([]byte, HeaderSize+1+6+1)
	binary.BigEndian.PutUint32(req[0:], uint32(len(req))|FlagCtl)
	binary.BigEndian.PutUint32(req[4:], 0xffffffff)
	req[8] = CCReqConnect
	copy(req[9:], "QUAKE\x00")
	req[15] = 3

	if _, err := UDPWrite(clientConn, req, serverAddr); err != nil {
		t.Fatalf("failed to send connection request: %v", err)
	}

	var serverSock *Socket
	for i := 0; i < 100; i++ {
		serverSock = CheckNewConnections()
		if serverSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if serverSock == nil {
		t.Fatal("server failed to accept connection")
	}
	defer Close(serverSock)

	resp := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := UDPRead(clientConn, resp)
	clientConn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatalf("failed to read accept response: %v", err)
	}
	if n < HeaderSize+1+4 {
		t.Fatalf("accept response too short: %d bytes", n)
	}
	if resp[8] != CCRepAccept {
		t.Fatalf("expected CCRepAccept (%#x), got %#x", CCRepAccept, resp[8])
	}

	reportedPort := int(binary.LittleEndian.Uint32(resp[9:13]))
	serverPort := serverSock.udpConn.LocalAddr().(*stdnet.UDPAddr).Port

	if reportedPort == acceptPort {
		t.Fatalf("CCRepAccept reported accept socket port %d; expected per-client socket port", acceptPort)
	}
	if reportedPort != serverPort {
		t.Fatalf("CCRepAccept reported port %d, but accepted socket is on port %d", reportedPort, serverPort)
	}
}
