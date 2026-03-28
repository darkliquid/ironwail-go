// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

// datagram.go implements Quake's custom reliable/unreliable message protocol
// on top of UDP. This is the "datagram driver" — the layer between the
// high-level net.go dispatcher and the raw UDP transport in udp.go.
//
// The datagram layer predates modern networking libraries and implements
// its own reliability scheme:
//   - Reliable messages: stop-and-wait ARQ with sequence numbers, ACKs,
//     retransmission on timeout, and message fragmentation for payloads
//     larger than MaxDatagram (1400 bytes). Only one reliable message
//     can be in flight at a time.
//   - Unreliable messages: fire-and-forget with sequence numbers for
//     duplicate/out-of-order detection, but no retransmission.
//   - Control messages: connection handshake packets (connect request,
//     accept/reject, server info queries) identified by the FlagCtl bit.
//
// Each packet has an 8-byte header: 4 bytes of flags+length, 4 bytes of
// sequence number. The flags encode the packet type (data, ACK, unreliable,
// control) and whether this is the end-of-message for fragmented sends.
//
// This file also handles connection establishment (DatagramConnect) and
// server-side connection acceptance (DatagramCheckNewConnections), plus
// the server-info query/response flow used by the LAN server browser.

import (
	"encoding/binary"
	"fmt"
	"math"
	stdnet "net"
	"slices"
	"strings"
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// defaultServerInfoHostname is the fallback server name returned in server
// info responses when no hostname cvar has been configured. Matches the
// "UNNAMED" default from the original Quake engine.
const defaultServerInfoHostname = "UNNAMED"

// ServerInfoProvider is a callback that returns current server state
// for responding to LAN browser queries. When set, the server info
// response uses live data instead of placeholders.
type ServerInfoProvider struct {
	Hostname   func() string
	MapName    func() string
	Players    func() int
	MaxPlayers func() int
	Address    func() string
	PlayerInfo func(index int) (name string, topColor, bottomColor byte, frags int32, ping float32, ok bool)
}

// serverInfoProvider is the active callback for live server state. When nil,
// placeholder values are used in server info responses. The host package
// sets this once the server is running.
var serverInfoProvider *ServerInfoProvider

// SetServerInfoProvider installs a callback for live server info.
func SetServerInfoProvider(p *ServerInfoProvider) {
	serverInfoProvider = p
}

// serverInfoHostname returns the server's display name for LAN browser
// responses. It first checks the "hostname" cvar (settable by server
// admins) and falls back to defaultServerInfoHostname if unset.
func serverInfoHostname() string {
	if value := cvar.StringValue("hostname"); value != "" {
		return value
	}
	return defaultServerInfoHostname
}

// DatagramSendMessage initiates sending a reliable message over UDP.
// Large messages (> MaxDatagram bytes) are fragmented: only the first
// MaxDatagram-sized chunk is sent immediately, and subsequent chunks
// are sent as each is acknowledged (stop-and-wait). The FlagEOM bit
// in the packet header signals the final fragment.
//
// The message is copied into sock.sendMessage so the caller's buffer
// can be reused immediately. The socket's canSend flag is cleared until
// the entire message is acknowledged. Returns 1 on success, -1 on error.
// Corresponds to Datagram_SendMessage() in net_dgrm.c.
func DatagramSendMessage(sock *Socket, data []byte) int {
	if len(data) > MaxMessage {
		return -1
	}

	copy(sock.sendMessage, data)
	sock.sendMessageLength = len(data)

	var dataLen int
	var eom uint32
	if len(data) <= MaxDatagram {
		dataLen = len(data)
		eom = FlagEOM
	} else {
		dataLen = MaxDatagram
		eom = 0
	}

	packetLen := HeaderSize + dataLen
	buf := make([]byte, packetLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(packetLen)|(FlagData|eom))
	binary.BigEndian.PutUint32(buf[4:], sock.sendSequence)
	sock.sendSequence++
	copy(buf[8:], sock.sendMessage[:dataLen])

	sock.canSend = false

	if _, err := UDPWrite(sock.udpConn, buf, sock.remoteAddr); err != nil {
		return -1
	}

	sock.lastSendTime = NetTime()
	return 1
}

// DatagramSendUnreliableMessage sends a single unreliable packet over UDP.
// Unlike reliable messages, unreliable packets are not retransmitted if
// lost — they are used for rapidly-changing state (entity positions, etc.)
// where stale data would be replaced by the next update anyway.
//
// Each unreliable packet carries a sequence number for the receiver to
// detect and discard duplicates or out-of-order packets. The maximum
// payload size is MaxDatagram (1400 bytes); larger payloads are rejected.
// Returns 1 on success, -1 on error.
func DatagramSendUnreliableMessage(sock *Socket, data []byte) int {
	if len(data) > MaxDatagram {
		return -1
	}

	packetLen := HeaderSize + len(data)
	buf := make([]byte, packetLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(packetLen)|FlagUnreliable)
	binary.BigEndian.PutUint32(buf[4:], sock.unreliableSendSeq)
	sock.unreliableSendSeq++
	copy(buf[8:], data)

	if _, err := UDPWrite(sock.udpConn, buf, sock.remoteAddr); err != nil {
		return -1
	}

	return 1
}

// DatagramCanSendMessage reports whether the socket can accept a new
// reliable message. The datagram layer uses stop-and-wait ARQ, meaning
// only one reliable message can be in flight at a time. If the previous
// message was fragmented and the next chunk is pending, this triggers
// sending it via DatagramSendMessageNext before checking readiness.
func DatagramCanSendMessage(sock *Socket) bool {
	if sock.sendNext {
		DatagramSendMessageNext(sock)
	}
	return sock.canSend
}

// DatagramSendMessageNext sends the next fragment of a multi-part reliable
// message. After the remote end ACKs a fragment, the acknowledged bytes
// are removed from sendMessage and sendNext is set to true. This function
// constructs and sends the next fragment (or the final one, flagged with
// FlagEOM). This implements the "sliding window of size 1" fragmentation
// that Quake uses to send large reliable messages (up to MaxMessage bytes)
// over a transport limited to MaxDatagram-sized packets.
func DatagramSendMessageNext(sock *Socket) int {
	var dataLen int
	var eom uint32
	if sock.sendMessageLength <= MaxDatagram {
		dataLen = sock.sendMessageLength
		eom = FlagEOM
	} else {
		dataLen = MaxDatagram
		eom = 0
	}

	packetLen := HeaderSize + dataLen
	buf := make([]byte, packetLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(packetLen)|(FlagData|eom))
	binary.BigEndian.PutUint32(buf[4:], sock.sendSequence)
	sock.sendSequence++
	copy(buf[8:], sock.sendMessage[:dataLen])

	sock.sendNext = false

	if _, err := UDPWrite(sock.udpConn, buf, sock.remoteAddr); err != nil {
		return -1
	}

	sock.lastSendTime = NetTime()
	return 1
}

// DatagramReSendMessage retransmits the current reliable message fragment.
// Called when the ACK timeout (1 second) expires without receiving an
// acknowledgment. The retransmitted packet uses the same sequence number
// (sendSequence-1) as the original send, since the receiver uses sequence
// numbers to detect and ignore duplicates. This is the core of Quake's
// retransmission logic from Datagram_ReSendMessage() in net_dgrm.c.
func DatagramReSendMessage(sock *Socket) int {
	var dataLen int
	var eom uint32
	if sock.sendMessageLength <= MaxDatagram {
		dataLen = sock.sendMessageLength
		eom = FlagEOM
	} else {
		dataLen = MaxDatagram
		eom = 0
	}

	packetLen := HeaderSize + dataLen
	buf := make([]byte, packetLen)
	binary.BigEndian.PutUint32(buf[0:], uint32(packetLen)|(FlagData|eom))
	binary.BigEndian.PutUint32(buf[4:], sock.sendSequence-1)
	copy(buf[8:], sock.sendMessage[:dataLen])

	sock.sendNext = false

	if _, err := UDPWrite(sock.udpConn, buf, sock.remoteAddr); err != nil {
		return -1
	}

	sock.lastSendTime = NetTime()
	return 1
}

// DatagramGetMessage polls for incoming packets on a datagram socket and
// processes the reliability protocol. This is the heart of Quake's network
// receive loop, handling four packet types:
//
//   - Control (FlagCtl): Connection management packets, returned as type 3.
//   - Unreliable (FlagUnreliable): Fire-and-forget packets with duplicate
//     detection via sequence numbers, returned as type 2.
//   - ACK (FlagAck): Acknowledgments for reliable sends. On receipt, the
//     acknowledged fragment is removed and the next fragment (if any) is
//     queued via sendNext. Not returned to the caller.
//   - Data (FlagData): Reliable data packets. An ACK is sent immediately.
//     Fragments are accumulated in receiveMessage until FlagEOM is set,
//     then the complete message is returned as type 1.
//
// If a reliable send is pending and its timeout has expired (>1 second),
// the message is retransmitted before polling. Returns (0, nil) when no
// complete message is available. Corresponds to Datagram_GetMessage().
func DatagramGetMessage(sock *Socket) (int, []byte) {
	if !sock.canSend {
		if (NetTime() - sock.lastSendTime) > 1.0 {
			DatagramReSendMessage(sock)
		}
	}

	buf := make([]byte, MaxDatagram+HeaderSize)
	for {
		n, addr, err := UDPRead(sock.udpConn, buf)
		if err != nil || n == 0 {
			break
		}

		if addr.String() != sock.remoteAddr.String() {
			continue
		}

		if n < HeaderSize {
			continue
		}

		header := binary.BigEndian.Uint32(buf[0:])
		sequence := binary.BigEndian.Uint32(buf[4:])
		flags := header & (^uint32(LengthMask))
		length := int(header & uint32(LengthMask))

		if flags&FlagCtl != 0 {
			if n < HeaderSize+1 {
				continue
			}
			return 3, buf[HeaderSize:n]
		}

		if flags&FlagUnreliable != 0 {
			if sequence < sock.unreliableRecvSeq {
				continue
			}
			sock.unreliableRecvSeq = sequence + 1
			return 2, buf[HeaderSize:n]
		}

		if flags&FlagAck != 0 {
			if sequence != (sock.sendSequence - 1) {
				continue
			}
			if sequence == sock.ackSequence {
				sock.ackSequence++
			} else {
				continue
			}

			sock.sendMessageLength -= MaxDatagram
			if sock.sendMessageLength > 0 {
				copy(sock.sendMessage, sock.sendMessage[MaxDatagram:])
				sock.sendNext = true
			} else {
				sock.sendMessageLength = 0
				sock.canSend = true
			}
			continue
		}

		if flags&FlagData != 0 {
			// Send ACK
			ackBuf := make([]byte, HeaderSize)
			binary.BigEndian.PutUint32(ackBuf[0:], uint32(HeaderSize)|FlagAck)
			binary.BigEndian.PutUint32(ackBuf[4:], sequence)
			UDPWrite(sock.udpConn, ackBuf, addr)

			if sequence != sock.recvSequence {
				continue
			}
			sock.recvSequence++

			dataLen := length - HeaderSize
			if flags&FlagEOM != 0 {
				result := make([]byte, sock.receiveMessageLength+dataLen)
				copy(result, sock.receiveMessage[:sock.receiveMessageLength])
				copy(result[sock.receiveMessageLength:], buf[HeaderSize:n])
				sock.receiveMessageLength = 0
				return 1, result
			}

			copy(sock.receiveMessage[sock.receiveMessageLength:], buf[HeaderSize:n])
			sock.receiveMessageLength += dataLen
			continue
		}
	}

	if sock.sendNext {
		DatagramSendMessageNext(sock)
	}

	return 0, nil
}

// Connection control (CC) command bytes, sent inside FlagCtl packets during
// the connection handshake. These form Quake's connection establishment
// protocol:
//
// Client sends CCReqConnect → Server replies CCRepAccept (with port) or
// CCRepReject (with reason string). The server info query/response flow
// (CCReqServerInfo / CCRepServerInfo) is used by the LAN server browser
// to discover available games without establishing a full connection.
//
// Request codes have the high bit clear (0x01-0x04); response codes have
// the high bit set (0x81-0x85). This convention makes it easy to distinguish
// requests from responses in packet dumps.
const (
	CCReqConnect    = 0x01
	CCReqServerInfo = 0x02
	CCReqPlayerInfo = 0x03
	CCReqRuleInfo   = 0x04

	CCRepAccept     = 0x81
	CCRepReject     = 0x82
	CCRepServerInfo = 0x83
	CCRepPlayerInfo = 0x84
	CCRepRuleInfo   = 0x85
)

// DatagramConnect establishes a UDP connection to a remote Quake server.
// The handshake follows the original Quake protocol:
//  1. Resolve the host string to a UDP address.
//  2. Open a local UDP socket on a random port.
//  3. Send a CCReqConnect control packet containing the "QUAKE" magic
//     string and protocol version 3.
//  4. Wait for CCRepAccept (which includes the server's game port) or
//     CCRepReject (which includes a human-readable rejection reason).
//  5. Retry up to 3 times with a 2.5-second timeout per attempt.
//
// On success, the socket's remoteAddr is updated to the port number
// provided in the accept response (the server may redirect the client
// to a different port). Returns nil on failure.
func DatagramConnect(host string) *Socket {
	addr, err := UDPStringToAddr(host)
	if err != nil {
		return nil
	}

	conn, err := UDPOpenSocket(0) // Open on random port
	if err != nil {
		return nil
	}

	sock := NewSocket(host)
	sock.driver = DriverDatagram
	sock.udpConn = conn
	sock.remoteAddr = addr

	// Send connection request with retries
	buf := make([]byte, 1024)
	binary.BigEndian.PutUint32(buf[0:], uint32(HeaderSize+1+6+1)|FlagCtl) // Header + cmd + "QUAKE" + version
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = CCReqConnect
	copy(buf[9:], "QUAKE\x00")
	buf[15] = 3 // Protocol version

	// Match C net_dgrm.c: 2.5-second timeout per attempt, with 3 retries.
	const maxRetries = 3
	const timeout = 2500 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		if _, err := UDPWrite(conn, buf[:16], addr); err != nil {
			UDPCloseSocket(conn)
			return nil
		}

		conn.SetReadDeadline(time.Now().Add(timeout))
		n, recvAddr, err := UDPRead(conn, buf)
		if err == nil && n >= HeaderSize+1 {
			// Check if it's from the same server
			if recvAddr.IP.Equal(addr.IP) && recvAddr.Port == addr.Port {
				cmd := buf[8]
				if cmd == CCRepAccept {
					newPort := int(binary.LittleEndian.Uint32(buf[9:]))
					sock.remoteAddr.Port = newPort
					conn.SetReadDeadline(time.Time{}) // Reset deadline
					return sock
				}
				if cmd == CCRepReject {
					reason := string(buf[9:n])
					if idx := strings.Index(reason, "\x00"); idx != -1 {
						reason = reason[:idx]
					}
					sock.rejectionReason = reason
					break
				}
			}
		}
		// On timeout or wrong packet, retry or continue to failure
	}

	UDPCloseSocket(conn)
	return nil
}

// acceptSocket is the UDP socket on which the server listens for incoming
// connection requests and server info queries. It is opened on the
// configured host port (default 26000) when Listen(true) is called.
var (
	acceptSocket *stdnet.UDPConn
	// acceptedServerSockets tracks currently accepted datagram sockets so
	// reconnects from the same remote endpoint can close stale sockets before
	// creating a new one (matching Quake's duplicate-address handling).
	acceptedServerSockets []*Socket
)

func sameUDPAddress(a, b *stdnet.UDPAddr) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Port == b.Port && a.Zone == b.Zone && a.IP.Equal(b.IP)
}

func closeDuplicateAcceptedServerSockets(addr *stdnet.UDPAddr) {
	var duplicates []*Socket
	for _, sock := range acceptedServerSockets {
		if sameUDPAddress(sock.remoteAddr, addr) {
			duplicates = append(duplicates, sock)
		}
	}
	for _, sock := range duplicates {
		Close(sock)
	}
}

func trackAcceptedServerSocket(sock *Socket) {
	if sock == nil {
		return
	}
	acceptedServerSockets = append(acceptedServerSockets, sock)
}

func untrackAcceptedServerSocket(sock *Socket) {
	if sock == nil {
		return
	}
	for i, tracked := range acceptedServerSockets {
		if tracked != sock {
			continue
		}
		acceptedServerSockets = append(acceptedServerSockets[:i], acceptedServerSockets[i+1:]...)
		return
	}
}

// DatagramCheckNewConnections checks the server's accept socket for
// incoming control packets from clients. It handles two cases:
//   - CCReqServerInfo: responds with server details (hostname, map,
//     player count) for the LAN browser, then returns nil.
//   - CCReqConnect: accepts the connection by sending CCRepAccept with
//     the server's port, creates a new Socket for the client, and
//     returns it to the caller.
//
// This is called once per server frame when the server is listening.
// Corresponds to Datagram_CheckNewConnections() in net_dgrm.c.
func DatagramCheckNewConnections() *Socket {
	if acceptSocket == nil {
		return nil
	}

	buf := make([]byte, 1024)
	n, addr, err := UDPRead(acceptSocket, buf)
	if err != nil || n < HeaderSize+1 {
		return nil
	}

	header := binary.BigEndian.Uint32(buf[0:])
	flags := header & (^uint32(LengthMask))
	if flags&FlagCtl == 0 {
		return nil
	}

	cmd := buf[8]
	if cmd == CCReqServerInfo {
		sendServerInfoResponse(acceptSocket, addr)
		return nil
	}
	if cmd == CCReqRuleInfo {
		sendRuleInfoResponse(acceptSocket, addr, strings.TrimRight(string(buf[9:n]), "\x00"))
		return nil
	}
	if cmd == CCReqPlayerInfo {
		if n < HeaderSize+2 {
			return nil
		}
		sendPlayerInfoResponse(acceptSocket, addr, int(buf[9]))
		return nil
	}

	if cmd == CCReqConnect {
		if isServerIPBanned(addr.String()) {
			resp := make([]byte, HeaderSize+1+len("You have been banned.\n")+1)
			binary.BigEndian.PutUint32(resp[0:], uint32(len(resp))|FlagCtl)
			binary.BigEndian.PutUint32(resp[4:], 0xffffffff)
			resp[8] = CCRepReject
			copy(resp[9:], "You have been banned.\n")
			UDPWrite(acceptSocket, resp, addr)
			return nil
		}

		// Quake closes stale server-side sockets if a client reconnects from
		// the same address:port. Do that before accepting the replacement.
		closeDuplicateAcceptedServerSockets(addr)

		// Create a new per-client socket on a random port (matching C's dfunc.Open_Socket(0)).
		// Each client gets its own socket so packets are demultiplexed by the OS.
		clientConn, err := UDPOpenSocket(0)
		if err != nil {
			return nil
		}

		// Get the port assigned to the new socket
		newPort := clientConn.LocalAddr().(*stdnet.UDPAddr).Port

		// Send CCREP_ACCEPT with the new socket's port (not the accept socket port).
		// The response is sent via the accept socket since the client is still listening there.
		resp := make([]byte, HeaderSize+1+4)
		binary.BigEndian.PutUint32(resp[0:], uint32(HeaderSize+1+4)|FlagCtl)
		binary.BigEndian.PutUint32(resp[4:], 0xffffffff)
		resp[8] = CCRepAccept
		binary.LittleEndian.PutUint32(resp[9:], uint32(newPort))
		UDPWrite(acceptSocket, resp, addr)

		sock := NewSocket(addr.String())
		sock.driver = DriverDatagram
		sock.udpConn = clientConn
		sock.remoteAddr = addr
		trackAcceptedServerSocket(sock)
		return sock
	}

	return nil
}

func nextServerInfoCvarAfter(previous string) *cvar.CVar {
	var names []string
	serverInfoVars := make(map[string]*cvar.CVar)
	for _, cv := range cvar.All() {
		if cv.Flags&cvar.FlagServerInfo == 0 {
			continue
		}
		names = append(names, cv.Name)
		serverInfoVars[cv.Name] = cv
	}
	slices.Sort(names)
	for _, name := range names {
		if strings.Compare(name, previous) > 0 {
			return serverInfoVars[name]
		}
	}
	return nil
}

// sendServerInfoResponse writes a CCRepServerInfo control packet back to the
// querying client. If a ServerInfoProvider is installed, live server state is
// used; otherwise placeholder values are returned.
func sendServerInfoResponse(conn *stdnet.UDPConn, addr *stdnet.UDPAddr) {
	hostname := serverInfoHostname()
	mapName := "e1m1"
	var players, maxPlayers byte
	maxPlayers = 8
	address := fmt.Sprintf("%s:%d", myTCPIPAddress, netHostPort)
	if address == ":26000" || address == ":" {
		address = addr.IP.String() + fmt.Sprintf(":%d", netHostPort)
	}

	if serverInfoProvider != nil {
		if serverInfoProvider.Hostname != nil {
			hostname = serverInfoProvider.Hostname()
		}
		if serverInfoProvider.MapName != nil {
			mapName = serverInfoProvider.MapName()
		}
		if serverInfoProvider.Players != nil {
			players = byte(serverInfoProvider.Players())
		}
		if serverInfoProvider.MaxPlayers != nil {
			maxPlayers = byte(serverInfoProvider.MaxPlayers())
		}
		if serverInfoProvider.Address != nil {
			address = serverInfoProvider.Address()
		}
	}

	// Build response: header(8) + cmd(1) + address\0 + hostname\0 + mapname\0 + players + maxplayers + proto
	var payload []byte
	payload = append(payload, CCRepServerInfo)
	payload = append(payload, []byte(address)...)
	payload = append(payload, 0)
	payload = append(payload, []byte(hostname)...)
	payload = append(payload, 0)
	payload = append(payload, []byte(mapName)...)
	payload = append(payload, 0)
	payload = append(payload, players, maxPlayers, 3) // protocol version 3

	resp := make([]byte, HeaderSize+len(payload))
	binary.BigEndian.PutUint32(resp[0:], uint32(HeaderSize+len(payload))|FlagCtl)
	binary.BigEndian.PutUint32(resp[4:], 0xffffffff)
	copy(resp[HeaderSize:], payload)
	UDPWrite(conn, resp, addr)
}

func sendRuleInfoResponse(conn *stdnet.UDPConn, addr *stdnet.UDPAddr, previous string) {
	var payload []byte
	payload = append(payload, CCRepRuleInfo)
	if cv := nextServerInfoCvarAfter(previous); cv != nil {
		payload = append(payload, []byte(cv.Name)...)
		payload = append(payload, 0)
		payload = append(payload, []byte(cv.String)...)
		payload = append(payload, 0)
	}

	resp := make([]byte, HeaderSize+len(payload))
	binary.BigEndian.PutUint32(resp[0:], uint32(len(resp))|FlagCtl)
	binary.BigEndian.PutUint32(resp[4:], 0xffffffff)
	copy(resp[HeaderSize:], payload)
	UDPWrite(conn, resp, addr)
}

func sendPlayerInfoResponse(conn *stdnet.UDPConn, addr *stdnet.UDPAddr, index int) {
	var payload []byte
	payload = append(payload, CCRepPlayerInfo)
	payload = append(payload, byte(index))
	if serverInfoProvider != nil && serverInfoProvider.PlayerInfo != nil {
		name, top, bottom, frags, ping, ok := serverInfoProvider.PlayerInfo(index)
		if ok && name != "" {
			payload = append(payload, []byte(name)...)
			payload = append(payload, 0)
			payload = append(payload, top, bottom)
			fragsBuf := make([]byte, 4)
			binary.LittleEndian.PutUint32(fragsBuf, uint32(frags))
			payload = append(payload, fragsBuf...)
			pingBuf := make([]byte, 4)
			binary.LittleEndian.PutUint32(pingBuf, math.Float32bits(ping))
			payload = append(payload, pingBuf...)
		} else {
			payload = append(payload, 0)
		}
	} else {
		payload = append(payload, 0)
	}

	resp := make([]byte, HeaderSize+len(payload))
	binary.BigEndian.PutUint32(resp[0:], uint32(len(resp))|FlagCtl)
	binary.BigEndian.PutUint32(resp[4:], 0xffffffff)
	copy(resp[HeaderSize:], payload)
	UDPWrite(conn, resp, addr)
}
