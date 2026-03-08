// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"encoding/binary"
	stdnet "net"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

const defaultServerInfoHostname = "UNNAMED"

func serverInfoHostname() string {
	if value := cvar.StringValue("hostname"); value != "" {
		return value
	}
	return defaultServerInfoHostname
}

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

func DatagramCanSendMessage(sock *Socket) bool {
	if sock.sendNext {
		DatagramSendMessageNext(sock)
	}
	return sock.canSend
}

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
			continue
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

	// Send connection request
	buf := make([]byte, 1024)
	binary.BigEndian.PutUint32(buf[0:], uint32(HeaderSize+1+6+1)|FlagCtl) // Header + cmd + "QUAKE" + version
	binary.BigEndian.PutUint32(buf[4:], 0xffffffff)
	buf[8] = CCReqConnect
	copy(buf[9:], "QUAKE\x00")
	buf[15] = 3 // Protocol version

	if _, err := UDPWrite(conn, buf[:16], addr); err != nil {
		UDPCloseSocket(conn)
		return nil
	}

	// Wait for response (simplified for now)
	n, _, err := UDPRead(conn, buf)
	if err != nil || n < HeaderSize+1 {
		UDPCloseSocket(conn)
		return nil
	}

	cmd := buf[8]
	if cmd == CCRepAccept {
		// In Quake, the accept message contains a new port to connect to.
		// For simplicity in this port, we might just use the same port or handle it if needed.
		newPort := int(binary.LittleEndian.Uint32(buf[9:])) // Quake uses LittleEndian for the port in CCREP_ACCEPT? No, it uses MSG_WriteLong which is LittleEndian.
		sock.remoteAddr.Port = newPort
		return sock
	}

	UDPCloseSocket(conn)
	return nil
}

var (
	acceptSocket *stdnet.UDPConn
)

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
		// Send server info response
		resp := make([]byte, 1024)
		binary.BigEndian.PutUint32(resp[0:], uint32(HeaderSize+1+16+16+1+1+1)|FlagCtl)
		binary.BigEndian.PutUint32(resp[4:], 0xffffffff)
		resp[8] = CCRepServerInfo
		copy(resp[9:], "localhost:26000\x00")
		copy(resp[25:40], []byte(serverInfoHostname()))
		copy(resp[41:], "e1m1\x00")
		resp[57] = 0 // current players
		resp[58] = 8 // max players
		resp[59] = 3 // protocol version
		UDPWrite(acceptSocket, resp[:60], addr)
		return nil
	}

	if cmd == CCReqConnect {
		// Send accept response
		// In a real implementation, we would open a new socket for the client.
		// For now, we'll just accept it on the same port (not quite right but okay for a start).
		resp := make([]byte, HeaderSize+1+4)
		binary.BigEndian.PutUint32(resp[0:], uint32(HeaderSize+1+4)|FlagCtl)
		binary.BigEndian.PutUint32(resp[4:], 0xffffffff)
		resp[8] = CCRepAccept
		binary.LittleEndian.PutUint32(resp[9:], uint32(netHostPort))
		UDPWrite(acceptSocket, resp, addr)

		sock := NewSocket(addr.String())
		sock.driver = DriverDatagram
		sock.udpConn = acceptSocket // Should be a new socket in real Quake
		sock.remoteAddr = addr
		return sock
	}

	return nil
}
