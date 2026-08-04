// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// This file belongs to the Debug subsystem: debug telemetry, trigger touch debugging, and multiplayer debug logging.
//
// Svdbg types, constants, and logging functions have been moved to
// internal/server/debug. Aliases below preserve backward compatibility.
// The svDebugPushDumpTriggersOnce function remains here because it
// requires access to Server internals (edicts, entity accessors).
package server

import (
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
)

// Type aliases for svdbg types moved to the debug sub-package.
// (No types to alias — svdbg uses only constants and functions.)

// Svdbg cvar name constants re-exported from the debug sub-package.
const (
	SvDebugMultiplayerCVarName = srvdebug.SvDebugMultiplayerCVarName
	SvDebugMoveCVarName        = srvdebug.SvDebugMoveCVarName
	SvDebugPushCVarName        = srvdebug.SvDebugPushCVarName
)

// RegisterSvdbgCVars is re-exported from the debug sub-package.
var RegisterSvdbgCVars = srvdebug.RegisterSvdbgCVars

// Svdbg logging functions re-exported from the debug sub-package.
var (
	SvdbgMultiplayerLogf   = srvdebug.SvdbgMultiplayerLogf
	SvdbgMultiplayerLogfAt = srvdebug.SvdbgMultiplayerLogfAt
	SvdbgMoveLogf          = srvdebug.SvdbgMoveLogf
	SvdbgMoveLogfAt        = srvdebug.SvdbgMoveLogfAt
	SvdbgPushLogf          = srvdebug.SvdbgPushLogf
	SvdbgPushLogfAt        = srvdebug.SvdbgPushLogfAt
)

// svdbg level checker functions — thin wrappers for internal use.
func svDebugMultiplayerLevel() int { return srvdebug.SvDebugMultiplayerLevel() }
func svDebugMoveLevel() int        { return srvdebug.SvDebugMoveLevel() }
func svDebugPushLevel() int         { return srvdebug.SvDebugPushLevel() }

// svDebugPushDumpTriggersOnce dumps all SOLID_TRIGGER entities once per
// session so we can see what triggers exist and where they are positioned.
// This helps diagnose cases where touchLinks reports candidates=0 because
// no trigger entities overlap the player's bbox.
func svDebugPushDumpTriggersOnce(s *Server) {
	if srvdebug.SvDebugPushTriggerDumpDone {
		return
	}
	srvdebug.SvDebugPushTriggerDumpDone = true
	for i := 1; i < s.NumEdicts; i++ {
		e := s.Edicts[i]
		if e == nil || e.Free {
			continue
		}
		if int(e.Solid(s)) != int(SolidTrigger) {
			continue
		}
		cn := qcString(s.QCVM, e.ClassName(s))
		SvdbgPushLogf("trigger_dump edict=%d classname=%q touch=%d absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f) origin=(%.1f %.1f %.1f)",
			i, cn, e.Touch(s),
			e.AbsMin(s)[0], e.AbsMin(s)[1], e.AbsMin(s)[2],
			e.AbsMax(s)[0], e.AbsMax(s)[1], e.AbsMax(s)[2],
			e.Origin(s)[0], e.Origin(s)[1], e.Origin(s)[2])
	}
}
