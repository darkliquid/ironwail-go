// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package net

import (
	"fmt"
	stdnet "net"
)

var (
	netHostPort        = 26000
	defaultNetHostPort = 26000
	tcpipAvailable     = false
	myTCPIPAddress     string
)

func UDPInit() error {
	tcpipAvailable = true
	return nil
}

func UDPOpenSocket(port int) (*stdnet.UDPConn, error) {
	addr, err := stdnet.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}
	conn, err := stdnet.ListenUDP("udp4", addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func UDPCloseSocket(conn *stdnet.UDPConn) error {
	if conn == nil {
		return nil
	}
	return conn.Close()
}

func UDPRead(conn *stdnet.UDPConn, buf []byte) (int, *stdnet.UDPAddr, error) {
	return conn.ReadFromUDP(buf)
}

func UDPWrite(conn *stdnet.UDPConn, buf []byte, addr *stdnet.UDPAddr) (int, error) {
	return conn.WriteToUDP(buf, addr)
}

func UDPAddrToString(addr *stdnet.UDPAddr) string {
	if addr == nil {
		return ""
	}
	return addr.String()
}

func UDPStringToAddr(address string) (*stdnet.UDPAddr, error) {
	return stdnet.ResolveUDPAddr("udp4", address)
}
