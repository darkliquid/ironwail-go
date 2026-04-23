// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/mods"
)

// ExtraModsAddonsURLCVar names the archive cvar that overrides the
// addons server URL. Mirrors extramods_addons_url in host_cmd.c.
const ExtraModsAddonsURLCVar = "extramods_addons_url"

var (
	extraModsURLCVar *cvar.CVar
)

// RegisterExtraModsCVars registers the addons-URL cvar. Safe to call
// once during host CVar registration.
func RegisterExtraModsCVars(cv *cvar.CVarSystem) {
	if cv == nil {
		return
	}
	extraModsURLCVar = cv.Register(ExtraModsAddonsURLCVar,
		mods.DefaultAddonServer, cvar.FlagArchive,
		"Addons server root URL for extramods downloads")
}

// extraModsURL returns the current addons URL from cvar state,
// falling back to the default when unregistered or empty.
func extraModsURL() string {
	if extraModsURLCVar == nil {
		return mods.DefaultAddonServer
	}
	if s := extraModsURLCVar.String; s != "" {
		return s
	}
	return mods.DefaultAddonServer
}

type modsDownloaderState struct {
	once sync.Once
	dl   *mods.Downloader
}

func (h *Host) modsDownloader() *mods.Downloader {
	h.modsDL.once.Do(func() {
		installDir := h.userDir
		if installDir == "" {
			installDir = h.baseDir
		}
		h.modsDL.dl = mods.New(mods.Config{
			BaseURL:    extraModsURL(),
			CacheDir:   h.userDir,
			InstallDir: installDir,
			OnEvent:    h.onModsEvent,
		})
	})
	// Track cvar changes without racing the lazy init path.
	if h.modsDL.dl != nil {
		if u := extraModsURL(); u != h.modsDL.dl.BaseURL() {
			h.modsDL.dl.SetBaseURL(u)
		}
	}
	return h.modsDL.dl
}

// ModsDownloader returns the addons downloader, lazy-initializing it
// on first use. Safe to call from the main thread only.
func (h *Host) ModsDownloader() *mods.Downloader {
	return h.modsDownloader()
}

// FetchModManifest fetches content.json on a background goroutine and
// invokes onDone on the main thread with the parsed manifest (or an
// error). Returns immediately.
func (h *Host) FetchModManifest(onDone func(*mods.Manifest, error)) {
	dl := h.modsDownloader()
	go func() {
		m, err := dl.FetchManifest(context.Background())
		cb := func() {
			if onDone != nil {
				onDone(m, err)
			}
		}
		if !h.InvokeOnMainThread(cb) {
			cb()
		}
	}()
}

// StartModInstall begins installing the given addon. Returns nil when
// another install is already in progress. onDone fires on the main
// thread when the install finishes (or fails). Mirrors
// Modlist_StartInstalling in host_cmd.c.
func (h *Host) StartModInstall(mod mods.RemoteMod, onDone func(error)) (*mods.InstallState, error) {
	dl := h.modsDownloader()
	return dl.StartInstall(context.Background(), mod, func(err error) {
		cb := func() {
			if onDone != nil {
				onDone(err)
			}
		}
		if !h.InvokeOnMainThread(cb) {
			cb()
		}
	})
}

// IsInstallingMod reports whether a mod install is currently in
// progress. Mirrors Modlist_IsInstalling.
func (h *Host) IsInstallingMod() bool {
	if h.modsDL.dl == nil {
		return false
	}
	return h.modsDL.dl.IsInstalling()
}

// onModsEvent translates downloader progress into sysdbg kind=modlist
// lines. Runs on the downloader goroutine; keep it side-effect free.
func (h *Host) onModsEvent(e mods.Event) {
	kind := "modlist"
	switch e.Kind {
	case "manifest_fetch_begin":
		hostDebugSysLogf(kind, "manifest_fetch_begin url=%s", e.URL)
	case "manifest_fetch_done":
		hostDebugSysLogf(kind, "manifest_fetch_done url=%s http=%d bytes=%d elapsed=%s",
			e.URL, e.HTTP, e.Bytes, e.Elapsed)
	case "manifest_fetch_error", "manifest_parse_error":
		hostDebugSysLogf(kind, "%s url=%s http=%d err=%v", e.Kind, e.URL, e.HTTP, e.Err)
	case "manifest_cache":
		hostDebugSysLogf(kind, "manifest_cache url=%s bytes=%d", e.URL, e.Bytes)
	case "install_begin":
		hostDebugSysLogf(kind, "install_begin mod=%s url=%s bytes_total=%d", e.Mod, e.URL, e.Bytes)
	case "install_chunk":
		hostDebugSysLogfAt(2, kind, "install_chunk mod=%s bytes=%d", e.Mod, e.Bytes)
	case "install_done":
		hostDebugSysLogf(kind, "install_done mod=%s bytes=%d elapsed=%s", e.Mod, e.Bytes, e.Elapsed)
	case "install_error":
		hostDebugSysLogf(kind, "install_error mod=%s url=%s http=%d err=%v", e.Mod, e.URL, e.HTTP, e.Err)
	default:
		hostDebugSysLogf(kind, "event kind=%s %s", e.Kind, fmt.Sprint(e))
	}
}
