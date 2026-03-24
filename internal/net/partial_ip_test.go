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

func TestPartialIPAddress_ParserEdges(t *testing.T) {
	local := net.IPv4(192, 168, 1, 42)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "permissive_port_trailing_garbage",
			input: "100:27000xyz",
			want:  "192.168.1.100:27000",
		},
		{
			name:  "permissive_port_non_numeric",
			input: "100:abc",
			want:  "192.168.1.100:0",
		},
		{
			name:  "permissive_empty_port",
			input: "100:",
			want:  "192.168.1.100:0",
		},
		{
			name:  "consecutive_dots_generate_zero_octet",
			input: "1..3",
			want:  "192.1.0.3:26000",
		},
		{
			name:  "empty_input_sets_last_octet_zero",
			input: "",
			want:  "192.168.1.0:26000",
		},
		{
			name:  "single_dot_sets_last_octet_zero",
			input: ".",
			want:  "192.168.1.0:26000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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

func TestPartialIPAddress_InvalidOctetRun(t *testing.T) {
	local := net.IPv4(192, 168, 1, 42)
	_, err := PartialIPAddress("1234", local, 26000)
	if err == nil {
		t.Fatal("expected error for 4-digit octet run")
	}
}

func TestPartialIPAddress_InvalidInput(t *testing.T) {
	local := net.IPv4(192, 168, 1, 42)
	_, err := PartialIPAddress("abc", local, 26000)
	if err == nil {
		t.Fatal("expected error for non-numeric input")
	}
}
