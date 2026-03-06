// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"time"
)

var (
	startTime = time.Now()
)

func NetTime() float64 {
	return time.Since(startTime).Seconds()
}

func Init() error {
	UDPInit()
	return nil
}

func Shutdown() {
	// Shutdown drivers
}

func Connect(host string) *Socket {
	if host == "local" || host == "localhost" {
		// Loopback
		l := NewLoopback()
		l.Init()
		sock := l.Connect()
		sock.driver = DriverLoopback
		return sock
	}
	return DatagramConnect(host)
}

func GetMessage(sock *Socket) (int, []byte) {
	if sock.driver == DriverLoopback {
		return GetMessageLoopback(sock, nil)
	}
	return DatagramGetMessage(sock)
}

func SendMessage(sock *Socket, data []byte) int {
	if sock.driver == DriverLoopback {
		return SendMessageLoopback(sock, data)
	}
	return DatagramSendMessage(sock, data)
}

func SendUnreliableMessage(sock *Socket, data []byte) int {
	if sock.driver == DriverLoopback {
		return SendUnreliableMessageLoopback(sock, data)
	}
	return DatagramSendUnreliableMessage(sock, data)
}

func CanSendMessage(sock *Socket) bool {
	if sock.driver == DriverLoopback {
		return true
	}
	return DatagramCanSendMessage(sock)
}

func Close(sock *Socket) {
	if sock.driver == DriverLoopback {
		CloseLoopback(sock)
	} else {
		UDPCloseSocket(sock.udpConn)
	}
}

var (
	loopback  *Loopback
	listening bool
)

func Listen(state bool) {
	listening = state
	if listening {
		if acceptSocket == nil {
			var err error
			acceptSocket, err = UDPOpenSocket(netHostPort)
			if err != nil {
				// Handle error
			}
		}
	} else {
		if acceptSocket != nil {
			UDPCloseSocket(acceptSocket)
			acceptSocket = nil
		}
	}
}

func CheckNewConnections() *Socket {
	if loopback != nil {
		if sock := loopback.CheckNewConnections(); sock != nil {
			sock.driver = DriverLoopback
			return sock
		}
	}

	if listening {
		return DatagramCheckNewConnections()
	}

	return nil
}
