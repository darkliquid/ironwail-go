// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package renderer

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestRendbgGating(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterRendbgCVars(cv)
	t.Cleanup(func() { rDebugPassesCVar = nil })

	var buf strings.Builder
	saved := RendbgEmit
	RendbgEmit = func(line string) { buf.WriteString(line); buf.WriteByte('\n') }
	t.Cleanup(func() { RendbgEmit = saved })

	RendbgLogf("silent")
	if buf.Len() != 0 {
		t.Fatalf("unexpected emission at level 0: %q", buf.String())
	}

	cv.Set(RDebugPassesCVarName, "1")
	RendbgLogf("alpha passes=%d", 3)
	RendbgLogfAt(2, "verbose")
	if !strings.Contains(buf.String(), "[rendbg] alpha passes=3") {
		t.Fatalf("expected alpha line, got %q", buf.String())
	}
	if strings.Contains(buf.String(), "verbose") {
		t.Fatalf("verbose should be gated at level 1, got %q", buf.String())
	}

	buf.Reset()
	cv.Set(RDebugPassesCVarName, "2")
	RendbgLogfAt(2, "call=%d", 42)
	if !strings.Contains(buf.String(), "call=42") {
		t.Fatalf("verbose should emit at level 2, got %q", buf.String())
	}
}
