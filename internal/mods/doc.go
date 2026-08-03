// Package mods implements the HTTP-backed addon downloader that mirrors
// C Ironwail's Modlist_* subsystem in host_cmd.c. It fetches an addons
// manifest (content.json) from a configurable URL, parses the available
// mods, and streams selected mods' pak archives into the user's install
// directory with background progress tracking.
//
// # Purpose
//
// Provides the "download mods" functionality accessible from the
// in-game Mods menu. The downloader runs in a background goroutine,
// reports progress via a callback, and writes pak files to the game's
// mod directory.
//
// # Original C lineage
//
// Mirrors Modlist_Init, Modlist_GetMod, Modlist_AddToUpdateList,
// Modlist_UpdateMod, and the HTTP download logic in host_cmd.c.
// The C version used platform-specific HTTP (SDL_net/libcurl);
// the Go version uses net/http.
//
// # Role in the engine
//
// Called by the menu system (internal/menu) and host command
// processor (internal/host). The downloaded mods appear as new
// game directories that the filesystem (internal/fs) can search.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/mods -count=1
package mods
