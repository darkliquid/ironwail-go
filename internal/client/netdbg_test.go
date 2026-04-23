// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package client

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func captureNetDebug(t *testing.T, level string) *strings.Builder {
	t.Helper()
	cv := cvar.NewCVarSystem()
	RegisterNetDebugCVars(cv)
	cv.Set(NetDebugCVarName, level)
	var buf strings.Builder
	saved := NetDebugEmit
	NetDebugEmit = func(line string) { buf.WriteString(line); buf.WriteByte('\n') }
	t.Cleanup(func() {
		NetDebugEmit = saved
		netDebugCVar = nil
	})
	return &buf
}

func TestNetDebugDisabledByDefault(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterNetDebugCVars(cv)
	defer func() { netDebugCVar = nil }()
	var got string
	saved := NetDebugEmit
	NetDebugEmit = func(line string) { got += line }
	t.Cleanup(func() { NetDebugEmit = saved })
	netDebugLogf("baseline", "ent=%d", 42)
	if got != "" {
		t.Fatalf("expected no emission while cvar is 0, got %q", got)
	}
}

func TestNetDebugLevelGates(t *testing.T) {
	buf := captureNetDebug(t, "1")
	netDebugLogf("tent", "type=%d", 3)
	netDebugLogfAt(2, "tent", "verbose only")
	out := buf.String()
	if !strings.Contains(out, "[netdbg kind=tent] type=3") {
		t.Fatalf("missing level-1 line in %q", out)
	}
	if strings.Contains(out, "verbose only") {
		t.Fatalf("level-2 should be gated at cvar=1: %q", out)
	}
}

func TestCoordAngleEncodingNames(t *testing.T) {
	for _, tc := range []struct {
		flags uint32
		coord string
		angle string
	}{
		{0, "short16", "byte"},
		{inet.PRFL_24BITCOORD, "24bit", "byte"},
		{inet.PRFL_INT32COORD, "int32", "byte"},
		{inet.PRFL_FLOATCOORD, "float", "byte"},
		{inet.PRFL_SHORTANGLE, "short16", "short"},
		{inet.PRFL_FLOATANGLE, "short16", "float"},
		{inet.PRFL_FLOATCOORD | inet.PRFL_FLOATANGLE, "float", "float"},
	} {
		if got := coordEncodingName(tc.flags); got != tc.coord {
			t.Errorf("coord flags=%x: got %q want %q", tc.flags, got, tc.coord)
		}
		if got := angleEncodingName(tc.flags); got != tc.angle {
			t.Errorf("angle flags=%x: got %q want %q", tc.flags, got, tc.angle)
		}
	}
}
