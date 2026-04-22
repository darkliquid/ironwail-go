package net

import "testing"

// TestNetworkIsolation verifies that two independent Network instances
// hold independent state. This is the regression guard for the phased
// DI migration: mutating one Network (host-port, IP ban, cvar wiring)
// must never affect a sibling Network.
func TestNetworkIsolation(t *testing.T) {
	a := NewNetwork()
	b := NewNetwork()

	// Host port: isolated.
	a.SetHostPort(27000)
	b.SetHostPort(28000)
	if got := a.HostPort(); got != 27000 {
		t.Errorf("a.HostPort() = %d, want 27000", got)
	}
	if got := b.HostPort(); got != 28000 {
		t.Errorf("b.HostPort() = %d, want 28000", got)
	}

	// IP ban: isolated.
	if err := a.SetIPBan("10.0.0.0", "255.255.0.0"); err != nil {
		t.Fatalf("a.SetIPBan: %v", err)
	}
	if err := b.SetIPBan("off", ""); err != nil {
		t.Fatalf("b.SetIPBan: %v", err)
	}
	if got := a.IPBanStatus(); got != "Banning 10.0.0.0 [255.255.0.0]" {
		t.Errorf("a.IPBanStatus() = %q", got)
	}
	if got := b.IPBanStatus(); got != "Banning not active" {
		t.Errorf("b.IPBanStatus() = %q", got)
	}

	// ServerInfoProvider: isolated.
	provA := &ServerInfoProvider{}
	b.SetServerInfoProvider(nil)
	a.SetServerInfoProvider(provA)
	if a.siProvider != provA {
		t.Errorf("a.siProvider not set")
	}
	if b.siProvider != nil {
		t.Errorf("b.siProvider leaked across instances")
	}

	// defaultNet must not be perturbed by per-instance mutations.
	if defaultNet == a || defaultNet == b {
		t.Errorf("NewNetwork returned the shared defaultNet")
	}

	// Cleanup: clear per-instance ban state so no other test observes it
	// via the package-level defaultNet (these instances aren't defaultNet,
	// but clear defensively).
	_ = a.SetIPBan("off", "")
	_ = b.SetIPBan("off", "")
}

// TestNetworkAcceptedSocketIsolation verifies that the tracked-accepted-
// socket list is per-Network so that Close on one Network does not
// mutate another Network's slice.
func TestNetworkAcceptedSocketIsolation(t *testing.T) {
	a := NewNetwork()
	b := NewNetwork()

	sockA := &Socket{driver: DriverDatagram}
	sockB := &Socket{driver: DriverDatagram}

	a.trackAcceptedServerSocket(sockA)
	b.trackAcceptedServerSocket(sockB)

	if len(a.accepted) != 1 || a.accepted[0] != sockA {
		t.Fatalf("a.accepted = %v, want [sockA]", a.accepted)
	}
	if len(b.accepted) != 1 || b.accepted[0] != sockB {
		t.Fatalf("b.accepted = %v, want [sockB]", b.accepted)
	}

	// Untracking sockA on a must leave b.accepted intact.
	a.untrackAcceptedServerSocket(sockA)
	if len(a.accepted) != 0 {
		t.Errorf("a.accepted after untrack = %v, want []", a.accepted)
	}
	if len(b.accepted) != 1 || b.accepted[0] != sockB {
		t.Errorf("b.accepted perturbed: %v", b.accepted)
	}

	// Cleanup.
	b.untrackAcceptedServerSocket(sockB)
}

// TestNetworkServerBrowserIsolation verifies that a ServerBrowser bound
// to a *Network reads that Network's default host port, not defaultNet's.
func TestNetworkServerBrowserIsolation(t *testing.T) {
	a := NewNetwork()
	a.defaultHostPort = 27777

	sb := a.NewServerBrowser()
	if sb.net != a {
		t.Fatalf("ServerBrowser.net not bound to owning Network")
	}

	// Process-wide constructor still binds to defaultNet.
	legacy := NewServerBrowser()
	if legacy.net != defaultNet {
		t.Fatalf("NewServerBrowser() did not bind to defaultNet")
	}
}
