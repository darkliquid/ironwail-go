// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.

package server

import (
	"strings"

	"github.com/darkliquid/ironwail-go/pkg/bsp"
)

// parseWorldspawnSkyboxName extracts the skybox name from the worldspawn
// entity's key-value pairs in the BSP entity lump. C Ironwail's sky parser
// accepts Quake's "sky" key plus common Half-Life/Quake Lives aliases,
// with later keys overriding earlier ones.
func parseWorldspawnSkyboxName(entities string) string {
	ent, ok := bsp.ParseFirstEntity(entities)
	if !ok {
		return ""
	}
	for _, key := range []string{"qlsky", "skyname", "sky"} {
		if val := ent.String(key); val != "" {
			return strings.TrimSpace(val)
		}
	}
	return ""
}
