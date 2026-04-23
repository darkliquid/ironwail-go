// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package client

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestInpdbgGating(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterInDebugCVars(cv)
	t.Cleanup(func() { inDebugCVar = nil })

	var buf strings.Builder
	saved := InpdbgEmit
	InpdbgEmit = func(line string) { buf.WriteString(line); buf.WriteByte('\n') }
	t.Cleanup(func() { InpdbgEmit = saved })

	InpdbgLogf("silent")
	if buf.Len() != 0 {
		t.Fatalf("unexpected emission at level 0: %q", buf.String())
	}

	cv.Set(InDebugCVarName, "1")
	InpdbgLogf("wheel=%.3f", float32(0.125))
	InpdbgLogfAt(2, "verbose")
	if !strings.Contains(buf.String(), "[inpdbg] wheel=0.125") {
		t.Fatalf("expected wheel line, got %q", buf.String())
	}
	if strings.Contains(buf.String(), "verbose") {
		t.Fatalf("verbose should be gated at level 1, got %q", buf.String())
	}

	buf.Reset()
	cv.Set(InDebugCVarName, "2")
	InpdbgLogfAt(2, "verbose-ok pitch=%.2f", float32(1.5))
	if !strings.Contains(buf.String(), "verbose-ok") {
		t.Fatalf("verbose should emit at level 2, got %q", buf.String())
	}
}
