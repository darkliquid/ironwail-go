// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func captureSvdbg(t *testing.T, mpLevel, moveLevel string) *strings.Builder {
	t.Helper()
	cv := cvar.NewCVarSystem()
	RegisterSvdbgCVars(cv)
	cv.Set(SvDebugMultiplayerCVarName, mpLevel)
	cv.Set(SvDebugMoveCVarName, moveLevel)
	var buf strings.Builder
	saved := SvdbgEmit
	SvdbgEmit = func(line string) { buf.WriteString(line); buf.WriteByte('\n') }
	t.Cleanup(func() {
		SvdbgEmit = saved
		svDebugMultiplayerCVar = nil
		svDebugMoveCVar = nil
	})
	return &buf
}

func TestSvdbgDisabledByDefault(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterSvdbgCVars(cv)
	t.Cleanup(func() {
		svDebugMultiplayerCVar = nil
		svDebugMoveCVar = nil
	})
	var got string
	saved := SvdbgEmit
	SvdbgEmit = func(line string) { got += line }
	t.Cleanup(func() { SvdbgEmit = saved })
	SvdbgMultiplayerLogf("ping")
	SvdbgMoveLogf("walk")
	if got != "" {
		t.Fatalf("expected no emission at cvar=0, got %q", got)
	}
}

func TestSvdbgLevelGates(t *testing.T) {
	buf := captureSvdbg(t, "1", "2")
	SvdbgMultiplayerLogf("ping host=%q", "127.0.0.1")
	SvdbgMultiplayerLogfAt(2, "verbose mp")
	SvdbgMoveLogf("origin=%v", [3]float32{1, 2, 3})
	SvdbgMoveLogfAt(2, "vel=%v", [3]float32{0, 0, -10})
	out := buf.String()
	if !strings.Contains(out, `[svdbg kind=multiplayer] ping host="127.0.0.1"`) {
		t.Fatalf("missing multiplayer line: %q", out)
	}
	if strings.Contains(out, "verbose mp") {
		t.Fatalf("verbose mp should be gated at cvar=1: %q", out)
	}
	if !strings.Contains(out, "[svdbg kind=move] origin=") {
		t.Fatalf("missing move line: %q", out)
	}
	if !strings.Contains(out, "[svdbg kind=move] vel=") {
		t.Fatalf("verbose move should emit at cvar=2: %q", out)
	}
}
