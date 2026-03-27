package stub

import iinput "github.com/ironwail/ironwail-go/internal/input"

// InputBackendForSystem returns no backend for pure-stub builds.
func InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return nil
}
