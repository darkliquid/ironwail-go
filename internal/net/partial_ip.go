package net

// partial_ip.go implements abbreviated IP address resolution matching C
// Quake's NET_PartialIPAddress. It allows specifying partial addresses like
// "192.168" or ":26001" and fills in defaults for omitted octets and port.

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// PartialIPAddress resolves an abbreviated IP address string into a full
// address. Unspecified octets are filled from localAddr (the machine's own IP).
// An optional :port suffix overrides the default port.
//
// Examples (assuming localAddr is 192.168.1.42):
//
//	"100"         → 192.168.1.100:defaultPort
//	"2.100"       → 192.168.2.100:defaultPort
//	"10.0.0.1"    → 10.0.0.1:defaultPort
//	"100:27000"   → 192.168.1.100:27000
//
// Mirrors C Quake net_udp.c:PartialIPAddress().
func PartialIPAddress(input string, localAddr net.IP, defaultPort int) (string, error) {
	if localAddr == nil {
		localAddr = net.IPv4(127, 0, 0, 1)
	}
	local4 := localAddr.To4()
	if local4 == nil {
		local4 = net.IPv4(127, 0, 0, 1).To4()
	}

	// Split off optional :port
	port := defaultPort
	addrPart := input
	if idx := strings.LastIndex(input, ":"); idx >= 0 {
		p, err := strconv.Atoi(input[idx+1:])
		if err != nil {
			return "", fmt.Errorf("invalid port in %q: %w", input, err)
		}
		port = p
		addrPart = input[:idx]
	}

	// Parse the octets from right to left (abbreviated octets from the end).
	parts := strings.Split(addrPart, ".")
	if len(parts) > 4 {
		return "", fmt.Errorf("too many octets in %q", addrPart)
	}

	// Build the result from local address, replacing octets from the right.
	result := [4]byte{local4[0], local4[1], local4[2], local4[3]}
	offset := 4 - len(parts)
	for i, p := range parts {
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 || v > 255 {
			return "", fmt.Errorf("invalid octet %q in %q", p, addrPart)
		}
		result[offset+i] = byte(v)
	}

	return fmt.Sprintf("%d.%d.%d.%d:%d", result[0], result[1], result[2], result[3], port), nil
}
