// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.

package server

import "strings"

// parseWorldspawnSkyboxName extracts the skybox name from the worldspawn
// entity's key-value pairs in the BSP entity lump. C Ironwail's sky parser
// accepts Quake's "sky" key plus common Half-Life/Quake Lives aliases,
// with later keys overriding earlier ones.
func parseWorldspawnSkyboxName(entities string) string {
	worldspawn, ok := firstEntityLumpObject(entities)
	if !ok {
		return ""
	}
	skyboxName := ""
	pos := 0
	for {
		key, next, ok := nextQuotedEntityToken(worldspawn, pos)
		if !ok {
			break
		}
		value, nextValue, ok := nextQuotedEntityToken(worldspawn, next)
		if !ok {
			break
		}
		key = strings.ToLower(strings.TrimSpace(key))
		key = strings.TrimPrefix(key, "_")
		switch key {
		case "sky", "skyname", "qlsky":
			skyboxName = value
		}
		pos = nextValue
	}
	return strings.TrimSpace(skyboxName)
}

func firstEntityLumpObject(data string) (string, bool) {
	start := strings.IndexByte(data, '{')
	if start < 0 {
		return "", false
	}
	end := strings.IndexByte(data[start+1:], '}')
	if end < 0 {
		return "", false
	}
	return data[start+1 : start+1+end], true
}

func nextQuotedEntityToken(data string, pos int) (string, int, bool) {
	start := strings.IndexByte(data[pos:], '"')
	if start < 0 {
		return "", pos, false
	}
	start += pos
	end := strings.IndexByte(data[start+1:], '"')
	if end < 0 {
		return "", pos, false
	}
	end += start + 1
	return data[start+1 : end], end + 1, true
}
