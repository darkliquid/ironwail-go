// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"encoding/binary"
	stdnet "net"
	"strings"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestUDPConnection(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26001 // Use a different port for testing
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	// Client connect in a goroutine because Connect blocks waiting for response
	var clientSock *Socket
	done := make(chan bool)
	go func() {
		clientSock = defaultNet.Connect("127.0.0.1:26001")
		done <- true
	}()

	// Server check connections
	var serverSock *Socket
	for i := 0; i < 100; i++ {
		serverSock = defaultNet.CheckNewConnections()
		if serverSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	<-done

	if clientSock == nil {
		t.Fatal("Failed to connect client")
	}
	defer defaultNet.Close(clientSock)

	if serverSock == nil {
		t.Fatal("Server failed to accept connection")
	}
	defer defaultNet.Close(serverSock)

	// Client send message
	msg := []byte("Hello Server")
	defaultNet.SendMessage(clientSock, msg)

	// Server receive message
	var receivedMsg []byte
	var msgType int
	for i := 0; i < 100; i++ {
		msgType, receivedMsg = defaultNet.GetMessage(serverSock)
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
	defaultNet.SendMessage(serverSock, []byte("Hello Client"))

	// Client receive message
	for i := 0; i < 100; i++ {
		msgType, receivedMsg = defaultNet.GetMessage(clientSock)
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

func TestUDPRejectsBannedConnection(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26007
	if err := defaultNet.SetIPBan("127.0.0.1", ""); err != nil {
		t.Fatalf("SetIPBan failed: %v", err)
	}
	defer func() {
		if err := defaultNet.SetIPBan("off", ""); err != nil {
			t.Fatalf("clearing ban failed: %v", err)
		}
	}()
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	clientConn, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("failed to open client udp socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(clientConn) }()

	serverAddr, err := UDPStringToAddr("127.0.0.1:26007")
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

	if defaultNet.acceptSocket == nil {
		t.Fatal("accept socket should be open while defaultNet.listening")
	}
	_ = defaultNet.acceptSocket.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if sock := defaultNet.CheckNewConnections(); sock != nil {
		_ = defaultNet.acceptSocket.SetReadDeadline(time.Time{})
		defaultNet.Close(sock)
		t.Fatal("server should reject banned connection before accept")
	}
	_ = defaultNet.acceptSocket.SetReadDeadline(time.Time{})

	resp := make([]byte, 1024)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := UDPRead(clientConn, resp)
	_ = clientConn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatalf("failed to read reject response: %v", err)
	}
	if n < HeaderSize+1+1 {
		t.Fatalf("reject response too short: %d bytes", n)
	}
	if resp[8] != CCRepReject {
		t.Fatalf("response command = %d, want %d", resp[8], CCRepReject)
	}
	reason := string(resp[9:n])
	reason = strings.TrimRight(reason, "\x00")
	if reason != "You have been banned.\n" {
		t.Fatalf("reject reason = %q, want %q", reason, "You have been banned.\n")
	}
}

func TestUDPUnreliable(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26002
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	var clientSock *Socket
	done := make(chan bool)
	go func() {
		clientSock = defaultNet.Connect("127.0.0.1:26002")
		done <- true
	}()

	var serverSock *Socket
	for i := 0; i < 100; i++ {
		serverSock = defaultNet.CheckNewConnections()
		if serverSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	<-done

	if clientSock == nil || serverSock == nil {
		t.Fatal("Failed to establish connection")
	}
	defer defaultNet.Close(clientSock)
	defer defaultNet.Close(serverSock)

	// Client send unreliable message
	defaultNet.SendUnreliableMessage(clientSock, []byte("Unreliable Server"))

	// Server receive message
	var receivedMsg []byte
	var msgType int
	for i := 0; i < 100; i++ {
		msgType, receivedMsg = defaultNet.GetMessage(serverSock)
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
	if !defaultNet.CanSendUnreliableMessage(clientSock) {
		t.Fatal("defaultNet.CanSendUnreliableMessage(clientSock) = false, want true")
	}
	if !defaultNet.CanSendUnreliableMessage(serverSock) {
		t.Fatal("defaultNet.CanSendUnreliableMessage(serverSock) = false, want true")
	}
}

func TestServerInfoHostnameFallback(t *testing.T) {
	cv := cvar.NewCVarSystem()
	defaultNet.SetCVarSystem(cv)
	t.Cleanup(func() { defaultNet.SetCVarSystem(nil) })

	hostname := cv.Register("hostname", defaultServerInfoHostname, cvar.FlagServerInfo, "")

	cv.Set(hostname.Name, "")
	if got := serverInfoHostname(); got != defaultServerInfoHostname {
		t.Fatalf("serverInfoHostname() with empty cvar = %q, want %q", got, defaultServerInfoHostname)
	}

	cv.Set(hostname.Name, "LAN Party")
	if got := serverInfoHostname(); got != "LAN Party" {
		t.Fatalf("serverInfoHostname() = %q, want %q", got, "LAN Party")
	}
}

func TestUDPRespondsToRuleInfoRequests(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26008
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	cv := cvar.NewCVarSystem()
	defaultNet.SetCVarSystem(cv)
	t.Cleanup(func() { defaultNet.SetCVarSystem(nil) })
	cv.Register("zz_rule_alpha", "1", cvar.FlagServerInfo, "")
	cv.Register("zz_rule_beta", "2", cvar.FlagServerInfo, "")

	clientConn, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("failed to open client udp socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(clientConn) }()

	serverAddr, err := UDPStringToAddr("127.0.0.1:26008")
	if err != nil {
		t.Fatalf("failed to parse server address: %v", err)
	}

	req := buildRuleInfoQuery("zz_rule_alpha")
	if _, err := UDPWrite(clientConn, req, serverAddr); err != nil {
		t.Fatalf("failed to send rule info request: %v", err)
	}

	if defaultNet.acceptSocket == nil {
		t.Fatal("accept socket should be open while defaultNet.listening")
	}
	_ = defaultNet.acceptSocket.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if sock := defaultNet.CheckNewConnections(); sock != nil {
		_ = defaultNet.acceptSocket.SetReadDeadline(time.Time{})
		defaultNet.Close(sock)
		t.Fatal("rule info request should not create an accepted socket")
	}
	_ = defaultNet.acceptSocket.SetReadDeadline(time.Time{})

	resp := make([]byte, 1024)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := UDPRead(clientConn, resp)
	_ = clientConn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatalf("failed to read rule info response: %v", err)
	}
	entry, ok := parseRuleInfoResponse(resp[:n])
	if !ok {
		t.Fatalf("parseRuleInfoResponse returned false for %d-byte response", n)
	}
	if entry.Name != "zz_rule_beta" || entry.Value != "2" {
		t.Fatalf("rule info response = %#v, want zz_rule_beta=2", entry)
	}
}

func TestUDPRespondsToPlayerInfoRequests(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26009
	playerProvider := &ServerInfoProvider{
		PlayerInfo: func(index int) (name string, topColor, bottomColor byte, frags int32, ping float32, ok bool) {
			switch index {
			case 0:
				return "Ranger", 4, 9, 15, 32.5, true
			default:
				return "", 0, 0, 0, 0, false
			}
		},
	}
	defaultNet.SetServerInfoProvider(playerProvider)
	t.Cleanup(func() {
		defaultNet.SetServerInfoProvider(nil)
	})
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	clientConn, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("failed to open client udp socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(clientConn) }()

	serverAddr, err := UDPStringToAddr("127.0.0.1:26009")
	if err != nil {
		t.Fatalf("failed to parse server address: %v", err)
	}

	req := buildPlayerInfoQuery(0)
	if _, err := UDPWrite(clientConn, req, serverAddr); err != nil {
		t.Fatalf("failed to send player info request: %v", err)
	}

	if defaultNet.acceptSocket == nil {
		t.Fatal("accept socket should be open while defaultNet.listening")
	}
	_ = defaultNet.acceptSocket.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if sock := defaultNet.CheckNewConnections(); sock != nil {
		_ = defaultNet.acceptSocket.SetReadDeadline(time.Time{})
		defaultNet.Close(sock)
		t.Fatal("player info request should not create an accepted socket")
	}
	_ = defaultNet.acceptSocket.SetReadDeadline(time.Time{})

	resp := make([]byte, 1024)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := UDPRead(clientConn, resp)
	_ = clientConn.SetReadDeadline(time.Time{})
	if err != nil {
		t.Fatalf("failed to read player info response: %v", err)
	}
	entry, ok := parsePlayerInfoResponse(resp[:n])
	if !ok {
		t.Fatalf("parsePlayerInfoResponse returned false for %d-byte response", n)
	}
	if entry.Name != "Ranger" || entry.Colors != [2]byte{4, 9} || entry.Frags != 15 || entry.Ping != float32(32.5) {
		t.Fatalf("player info response = %#v, want Ranger 4/9 15 32.5", entry)
	}
}

func TestUDPConnectionsUsePerClientSockets(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26003
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	type connectResult struct {
		sock *Socket
	}

	results := make(chan connectResult, 2)
	go func() { results <- connectResult{sock: defaultNet.Connect("127.0.0.1:26003")} }()
	go func() { results <- connectResult{sock: defaultNet.Connect("127.0.0.1:26003")} }()

	serverSocks := make([]*Socket, 0, 2)
	deadline := time.Now().Add(2 * time.Second)
	for len(serverSocks) < 2 && time.Now().Before(deadline) {
		if sock := defaultNet.CheckNewConnections(); sock != nil {
			serverSocks = append(serverSocks, sock)
		} else {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if len(serverSocks) != 2 {
		t.Fatalf("expected 2 server sockets, got %d", len(serverSocks))
	}
	defer defaultNet.Close(serverSocks[0])
	defer defaultNet.Close(serverSocks[1])

	clientA := (<-results).sock
	clientB := (<-results).sock
	if clientA == nil || clientB == nil {
		t.Fatal("failed to connect both clients")
	}
	defer defaultNet.Close(clientA)
	defer defaultNet.Close(clientB)

	if defaultNet.acceptSocket == nil {
		t.Fatal("accept socket should be open while defaultNet.listening")
	}
	acceptPort := defaultNet.acceptSocket.LocalAddr().(*stdnet.UDPAddr).Port

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
	if defaultNet.SendMessage(clientA, []byte("for-client-a")) != 1 {
		t.Fatal("failed to send message from client A")
	}

	for i := 0; i < 20; i++ {
		_ = serverForB.udpConn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
		msgType, _ := defaultNet.GetMessage(serverForB)
		if msgType != 0 {
			t.Fatalf("server socket for client B received unexpected message type %d", msgType)
		}
	}
	_ = serverForB.udpConn.SetReadDeadline(time.Time{})

	var gotType int
	var gotData []byte
	for i := 0; i < 100; i++ {
		_ = serverForA.udpConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		gotType, gotData = defaultNet.GetMessage(serverForA)
		if gotType != 0 {
			break
		}
	}
	_ = serverForA.udpConn.SetReadDeadline(time.Time{})
	if gotType != 1 || string(gotData) != "for-client-a" {
		t.Fatalf("server socket for client A got type=%d data=%q, want type=1 data=%q", gotType, string(gotData), "for-client-a")
	}
}

func TestShutdownClosesAcceptAndAcceptedSockets(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26006
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}

	done := make(chan *Socket, 1)
	go func() {
		done <- defaultNet.Connect("127.0.0.1:26006")
	}()

	var serverSock *Socket
	deadline := time.Now().Add(2 * time.Second)
	for serverSock == nil && time.Now().Before(deadline) {
		serverSock = defaultNet.CheckNewConnections()
		if serverSock == nil {
			time.Sleep(10 * time.Millisecond)
		}
	}

	clientSock := <-done
	if clientSock == nil || serverSock == nil {
		t.Fatal("failed to establish UDP connection before shutdown")
	}
	defer defaultNet.Close(clientSock)

	defaultNet.Shutdown()

	if defaultNet.acceptSocket != nil {
		t.Fatal("Shutdown left accept socket open")
	}
	if defaultNet.listening {
		t.Fatal("Shutdown left listener active")
	}
	if len(defaultNet.accepted) != 0 {
		t.Fatalf("Shutdown left %d accepted sockets tracked", len(defaultNet.accepted))
	}
	if clientSock.udpConn == nil {
		t.Fatal("Shutdown should not close independent client sockets")
	}
	if serverSock.udpConn != nil {
		t.Fatal("Shutdown left accepted server socket open")
	}
}

func TestCCRepAcceptReportsPerClientSocketPort(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26004
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	if defaultNet.acceptSocket == nil {
		t.Fatal("accept socket should be open while defaultNet.listening")
	}
	acceptPort := defaultNet.acceptSocket.LocalAddr().(*stdnet.UDPAddr).Port

	clientConn, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("failed to open client udp socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(clientConn) }()

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
		serverSock = defaultNet.CheckNewConnections()
		if serverSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if serverSock == nil {
		t.Fatal("server failed to accept connection")
	}
	defer defaultNet.Close(serverSock)

	resp := make([]byte, 1024)
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := UDPRead(clientConn, resp)
	_ = clientConn.SetReadDeadline(time.Time{})
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

func TestDuplicateConnectClosesOldServerSocket(t *testing.T) {
	_ = defaultNet.Init()
	defaultNet.hostPort = 26005
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	defer func() {
		if err := defaultNet.Listen(false); err != nil {
			t.Fatalf("defaultNet.Listen(false) failed: %v", err)
		}
	}()

	clientConn, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("failed to open client udp socket: %v", err)
	}
	defer func() { _ = UDPCloseSocket(clientConn) }()

	serverAddr, err := UDPStringToAddr("127.0.0.1:26005")
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
		t.Fatalf("failed to send first connection request: %v", err)
	}

	var firstServerSock *Socket
	for i := 0; i < 100; i++ {
		firstServerSock = defaultNet.CheckNewConnections()
		if firstServerSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if firstServerSock == nil {
		t.Fatal("server failed to accept first connection")
	}
	defer defaultNet.Close(firstServerSock)

	if _, err := UDPWrite(clientConn, req, serverAddr); err != nil {
		t.Fatalf("failed to send second connection request: %v", err)
	}

	var secondServerSock *Socket
	for i := 0; i < 100; i++ {
		secondServerSock = defaultNet.CheckNewConnections()
		if secondServerSock != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if secondServerSock == nil {
		t.Fatal("server failed to accept second connection")
	}
	defer defaultNet.Close(secondServerSock)

	if rc := defaultNet.SendUnreliableMessage(firstServerSock, []byte("stale-connection-payload")); rc != -1 {
		t.Fatalf("old server socket should be closed after duplicate connect, send returned %d", rc)
	}

	if rc := defaultNet.SendUnreliableMessage(secondServerSock, []byte("fresh-connection-payload")); rc != 1 {
		t.Fatalf("new server socket should be usable, send returned %d", rc)
	}
}

func TestListenEnableFailureReturnsErrorAndLeavesClosed(t *testing.T) {
	_ = defaultNet.Init()

	blocker, err := UDPOpenSocket(0)
	if err != nil {
		t.Fatalf("failed to open blocker socket: %v", err)
	}
	blockerPort := blocker.LocalAddr().(*stdnet.UDPAddr).Port
	defer func() { _ = UDPCloseSocket(blocker) }()

	defaultNet.hostPort = blockerPort
	if err := defaultNet.Listen(true); err == nil {
		t.Fatalf("defaultNet.Listen(true) succeeded unexpectedly on occupied port %d", blockerPort)
	}
	if defaultNet.listening {
		t.Fatal("defaultNet.listening should remain false after defaultNet.Listen(true) bind failure")
	}
	if defaultNet.acceptSocket != nil {
		t.Fatal("accept socket should remain nil after defaultNet.Listen(true) bind failure")
	}
	if err := defaultNet.Listen(false); err != nil {
		t.Fatalf("defaultNet.Listen(false) after failed enable returned error: %v", err)
	}
}

func TestUDPStringToAddr_ExpandsNumericLeadingPartialIP(t *testing.T) {
	oldMyIP := defaultNet.myTCPIPAddress
	oldHostPort := defaultNet.hostPort
	t.Cleanup(func() {
		defaultNet.myTCPIPAddress = oldMyIP
		defaultNet.hostPort = oldHostPort
	})

	defaultNet.myTCPIPAddress = "192.168.1.42"
	defaultNet.hostPort = 26000

	addr, err := UDPStringToAddr("2.100:27000")
	if err != nil {
		t.Fatalf("UDPStringToAddr failed: %v", err)
	}

	if got, want := addr.IP.String(), "192.168.2.100"; got != want {
		t.Fatalf("resolved IP = %q, want %q", got, want)
	}
	if got, want := addr.Port, 27000; got != want {
		t.Fatalf("resolved port = %d, want %d", got, want)
	}
}

func TestUDPStringToAddr_HostnameUsesNormalResolution(t *testing.T) {
	oldMyIP := defaultNet.myTCPIPAddress
	t.Cleanup(func() {
		defaultNet.myTCPIPAddress = oldMyIP
	})

	defaultNet.myTCPIPAddress = "192.168.1.42"

	addr, err := UDPStringToAddr("localhost:26000")
	if err != nil {
		t.Fatalf("UDPStringToAddr failed: %v", err)
	}
	if !addr.IP.IsLoopback() {
		t.Fatalf("hostname resolution IP = %q, want loopback", addr.IP.String())
	}
	if got, want := addr.IP.String(), defaultNet.myTCPIPAddress; got == want {
		t.Fatalf("hostname path incorrectly used partial-IP expansion: got %q", got)
	}
}

func TestUDPStringToAddr_HostnameWithoutPortUsesDefaultHostPort(t *testing.T) {
	oldHostPort := defaultNet.hostPort
	t.Cleanup(func() {
		defaultNet.hostPort = oldHostPort
	})

	defaultNet.hostPort = 27500

	addr, err := UDPStringToAddr("localhost")
	if err != nil {
		t.Fatalf("UDPStringToAddr failed: %v", err)
	}
	if got, want := addr.Port, 27500; got != want {
		t.Fatalf("resolved port = %d, want %d", got, want)
	}
	if !addr.IP.IsLoopback() {
		t.Fatalf("hostname resolution IP = %q, want loopback", addr.IP.String())
	}
}

func TestSetHostPortValidationAndHostPortAccessor(t *testing.T) {
	oldHostPort := defaultNet.hostPort
	oldDefaultHostPort := defaultNet.defaultHostPort
	t.Cleanup(func() {
		defaultNet.hostPort = oldHostPort
		defaultNet.defaultHostPort = oldDefaultHostPort
	})

	defaultNet.SetHostPort(0)
	if got := defaultNet.HostPort(); got != oldHostPort {
		t.Fatalf("HostPort after defaultNet.SetHostPort(0) = %d, want unchanged %d", got, oldHostPort)
	}

	defaultNet.SetHostPort(65535)
	if got := defaultNet.HostPort(); got != oldHostPort {
		t.Fatalf("HostPort after defaultNet.SetHostPort(65535) = %d, want unchanged %d", got, oldHostPort)
	}

	defaultNet.SetHostPort(27500)
	if got := defaultNet.HostPort(); got != 27500 {
		t.Fatalf("HostPort = %d, want 27500", got)
	}
	if got := defaultNet.defaultHostPort; got != 27500 {
		t.Fatalf("defaultNet.defaultHostPort = %d, want 27500", got)
	}
}

func TestIsListeningTracksListenState(t *testing.T) {
	_ = defaultNet.Init()
	port := defaultNet.hostPort + 20
	defaultNet.SetHostPort(port)
	_ = defaultNet.Listen(false)
	t.Cleanup(Shutdown)

	if defaultNet.IsListening() {
		t.Fatal("defaultNet.IsListening() true before enabling listen")
	}
	if err := defaultNet.Listen(true); err != nil {
		t.Fatalf("defaultNet.Listen(true) failed: %v", err)
	}
	if !defaultNet.IsListening() {
		t.Fatal("defaultNet.IsListening() false after enabling listen")
	}
	if err := defaultNet.Listen(false); err != nil {
		t.Fatalf("defaultNet.Listen(false) failed: %v", err)
	}
	if defaultNet.IsListening() {
		t.Fatal("defaultNet.IsListening() true after disabling listen")
	}
}
