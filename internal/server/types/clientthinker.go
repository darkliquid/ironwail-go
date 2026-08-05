// clientthinker.go defines the ClientThinker interface for client-side
// QuakeC think dispatch, enabling server subpackages (physics, net) to invoke
// per-player QC think functions without importing the server facade.
package types

// ClientThinker abstracts the per-client QuakeC think dispatch used by the
// server physics loop and movement code. Implemented by *server.Server; the
// methods were exported from private server helpers so subpackages can depend
// on this narrow contract instead of the whole server struct.
type ClientThinker interface {
	// PlayerClient returns the active, spawned client bound to ent, or nil.
	PlayerClient(ent *Edict) *Client
	// RunClientQCThinkWithMode runs a named QC function (e.g. PlayerPreThink,
	// PlayerPostThink) against the client's edict, optionally re-syncing globals.
	RunClientQCThinkWithMode(client *Client, funcName string, fullSync bool)
	// SyncSpawnedEdictsFromQCVM re-links edicts spawned by QC since startEntNum.
	SyncSpawnedEdictsFromQCVM(startEntNum int)
}
