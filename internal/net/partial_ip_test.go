package net

import (
	"net"
	"testing"
)

func TestPartialIPAddress(t *testing.T) {
	local := net.IPv4(192, 168, 1, 42)

	tests := []struct {
		input string
		want  string
	}{
		{"100", "192.168.1.100:26000"},
		{"2.100", "192.168.2.100:26000"},
		{"10.0.0.1", "10.0.0.1:26000"},
		{"100:27000", "192.168.1.100:27000"},
		{"10.0.0.1:27500", "10.0.0.1:27500"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := PartialIPAddress(tc.input, local, 26000)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPartialIPAddress_InvalidInput(t *testing.T) {
	local := net.IPv4(192, 168, 1, 42)
	_, err := PartialIPAddress("abc", local, 26000)
	if err == nil {
		t.Fatal("expected error for non-numeric input")
	}
}
