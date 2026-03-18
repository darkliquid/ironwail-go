package net

import "testing"

func TestMaxMessageMatchesCNETMAXMESSAGE(t *testing.T) {
	const cNETMAXMESSAGE = 65535
	if MaxMessage != cNETMAXMESSAGE {
		t.Fatalf("MaxMessage = %d, want %d (C NET_MAXMESSAGE)", MaxMessage, cNETMAXMESSAGE)
	}
}

func TestNewSocketAllocatesMaxMessageBuffers(t *testing.T) {
	sock := NewSocket("test")
	if got := len(sock.sendMessage); got != MaxMessage {
		t.Fatalf("sendMessage len = %d, want %d", got, MaxMessage)
	}
	if got := len(sock.receiveMessage); got != MaxMessage {
		t.Fatalf("receiveMessage len = %d, want %d", got, MaxMessage)
	}
}
