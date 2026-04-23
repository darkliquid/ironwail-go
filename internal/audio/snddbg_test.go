// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestSnddbgGating(t *testing.T) {
	cv := cvar.NewCVarSystem()
	RegisterSnddbgCVars(cv)
	t.Cleanup(func() { sndDebugMixerCVar = nil })

	var buf strings.Builder
	saved := SnddbgEmit
	SnddbgEmit = func(line string) { buf.WriteString(line); buf.WriteByte('\n') }
	t.Cleanup(func() { SnddbgEmit = saved })

	// Disabled by default.
	SnddbgLogf("silent")
	if buf.Len() != 0 {
		t.Fatalf("unexpected emission at level 0: %q", buf.String())
	}

	cv.Set(SndDebugMixerCVarName, "1")
	SnddbgLogf("ping=%d", 1)
	SnddbgLogfAt(2, "verbose")
	if !strings.Contains(buf.String(), "[snddbg] ping=1") {
		t.Fatalf("expected ping line, got %q", buf.String())
	}
	if strings.Contains(buf.String(), "verbose") {
		t.Fatalf("verbose should be gated at level 1, got %q", buf.String())
	}

	buf.Reset()
	cv.Set(SndDebugMixerCVarName, "2")
	SnddbgLogfAt(2, "verbose-ok")
	if !strings.Contains(buf.String(), "verbose-ok") {
		t.Fatalf("verbose should emit at level 2, got %q", buf.String())
	}
}
