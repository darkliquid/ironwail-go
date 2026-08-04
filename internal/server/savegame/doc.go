// Package savegame contains portable types and constants for the server's
// save/load game system. The serialization logic itself remains in the
// server package because it requires deep access to Server internals
// (Edicts, QCVM, area nodes, etc.), but the portable snapshot types
// live here so they can be referenced by the host package and future
// sub-packages without importing the full server package.
//
// The server package re-exports all types and constants via aliases,
// so existing code referencing server.SaveGameState continues to work.
//
// # Original C lineage
//
// Save/load functionality in the original C engine spans sv_save.c,
// sv_load.c, and the savegame section of server.h.
package savegame
