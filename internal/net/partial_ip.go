package net

// partial_ip.go implements abbreviated IP address resolution matching C
// Quake's NET_PartialIPAddress. It allows specifying partial addresses like
// "192.168" or ":26001" and fills in defaults for omitted octets and port.

import (
	"fmt"
	"net"
	"unicode"
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

	buf := "." + input
	idx := 0
	if len(buf) > 1 && buf[1] == '.' {
		idx++
	}

	octets := make([]byte, 0, 4)
	for idx < len(buf) && buf[idx] == '.' {
		idx++

		num := 0
		run := 0
		for idx < len(buf) && buf[idx] >= '0' && buf[idx] <= '9' {
			num = num*10 + int(buf[idx]-'0')
			idx++
			run++
			if run > 3 {
				return "", fmt.Errorf("invalid octet run in %q", input)
			}
		}

		if idx < len(buf) {
			c := buf[idx]
			if c != '.' && c != ':' && (c < '0' || c > '9') {
				return "", fmt.Errorf("invalid octet in %q", input)
			}
		}
		if num < 0 || num > 255 {
			return "", fmt.Errorf("invalid octet in %q", input)
		}
		octets = append(octets, byte(num))
	}

	port := defaultPort
	if idx < len(buf) && buf[idx] == ':' {
		port = quakeAtoi(buf[idx+1:])
	}

	result := [4]byte{local4[0], local4[1], local4[2], local4[3]}
	if len(octets) > 4 {
		octets = octets[len(octets)-4:]
	}
	offset := 4 - len(octets)
	copy(result[offset:], octets)

	return fmt.Sprintf("%d.%d.%d.%d:%d", result[0], result[1], result[2], result[3], port), nil
}

func quakeAtoi(s string) int {
	i := 0
	for i < len(s) && unicode.IsSpace(rune(s[i])) {
		i++
	}

	sign := 1
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
	}

	n := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		n = n*10 + int(s[i]-'0')
		i++
	}

	return sign * n
}
