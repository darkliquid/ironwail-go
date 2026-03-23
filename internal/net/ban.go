package net

// ban.go implements server-side IP banning matching C Quake's SV_Ban_f.
// IPBanList maintains a list of banned IP/mask pairs with thread-safe
// Add, Remove, Check, and List operations.

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

var serverIPBan IPBan

// IPBan implements a simple IP banning mechanism with address and mask,
// matching C Ironwail's net_dgrm.c banAddr/banMask.
type IPBan struct {
	mu   sync.RWMutex
	addr net.IP     // Ban address (nil = not active)
	mask net.IPMask // Ban mask
}

// SetBan configures the IP ban. Pass empty addr to disable.
// addr is an IPv4 address string, mask is an optional subnet mask string.
// With no mask, a full /32 mask is used (single IP).
func (b *IPBan) SetBan(addr, mask string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if addr == "" || strings.EqualFold(addr, "off") {
		b.addr = nil
		b.mask = nil
		return nil
	}

	ip := net.ParseIP(addr)
	if ip == nil {
		return fmt.Errorf("invalid ban address: %s", addr)
	}
	ip = ip.To4()
	if ip == nil {
		return fmt.Errorf("IPv6 not supported for banning: %s", addr)
	}

	if mask == "" {
		// Default: full mask (single IP ban)
		b.mask = net.CIDRMask(32, 32)
	} else {
		m := net.ParseIP(mask)
		if m == nil {
			return fmt.Errorf("invalid ban mask: %s", mask)
		}
		m4 := m.To4()
		if m4 == nil {
			return fmt.Errorf("IPv6 mask not supported: %s", mask)
		}
		b.mask = net.IPMask(m4)
	}

	b.addr = ip
	return nil
}

// IsBanned checks if the given address string matches the ban.
func (b *IPBan) IsBanned(remoteAddr string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.addr == nil {
		return false
	}

	// Parse host from addr (may include ":port")
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}

	// Check: (ip & mask) == (banAddr & mask)
	for i := 0; i < 4; i++ {
		if ip[i]&b.mask[i] != b.addr[i]&b.mask[i] {
			return false
		}
	}
	return true
}

// Active reports whether a ban is currently configured.
func (b *IPBan) Active() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.addr != nil
}

// String returns a human-readable description of the current ban.
func (b *IPBan) String() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.addr == nil {
		return "Banning not active"
	}
	return fmt.Sprintf("Banning %s [%s]", b.addr.String(), net.IP(b.mask).String())
}

// SetIPBan configures the single active server IP ban used by the datagram
// accept path and the host-facing ban command.
func SetIPBan(addr, mask string) error {
	return serverIPBan.SetBan(addr, mask)
}

// IPBanStatus returns the human-readable status string for the active server ban.
func IPBanStatus() string {
	return serverIPBan.String()
}

func isServerIPBanned(remoteAddr string) bool {
	return serverIPBan.IsBanned(remoteAddr)
}
