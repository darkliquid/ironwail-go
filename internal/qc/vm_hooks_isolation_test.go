// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package qc

import "testing"

// TestVMServerHooksIsolation verifies that each *VM owns its own
// ServerHooks. Mutating hooks on one VM must not leak into another.
// This is the regression guard for the Q1..Q6 migration that replaced
// the package-level serverBuiltinHooks global with VM.ServerHooks.
func TestVMServerHooksIsolation(t *testing.T) {
	a := newBuiltinsTestVM(4)
	b := newBuiltinsTestVM(4)

	var aGot, bGot string
	a.ServerHooks = ServerBuiltinHooks{
		BroadcastPrint: func(_ *VM, msg string) { aGot = msg },
	}
	b.ServerHooks = ServerBuiltinHooks{
		BroadcastPrint: func(_ *VM, msg string) { bGot = msg },
	}

	if a.ServerHooks.BroadcastPrint == nil || b.ServerHooks.BroadcastPrint == nil {
		t.Fatal("BroadcastPrint hooks not installed on one or both VMs")
	}

	a.ServerHooks.BroadcastPrint(a, "from-a")
	b.ServerHooks.BroadcastPrint(b, "from-b")

	if aGot != "from-a" {
		t.Errorf("a got %q, want from-a", aGot)
	}
	if bGot != "from-b" {
		t.Errorf("b got %q, want from-b", bGot)
	}

	// Clearing hooks on one VM must not affect the other.
	a.SetServerHooks(ServerBuiltinHooks{})
	if a.ServerHooks.BroadcastPrint != nil {
		t.Error("a.ServerHooks.BroadcastPrint not cleared")
	}
	if b.ServerHooks.BroadcastPrint == nil {
		t.Error("b.ServerHooks.BroadcastPrint leaked clear from a")
	}
}
