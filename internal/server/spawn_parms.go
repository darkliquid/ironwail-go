package server

const (
	spawnParmItemShotgun      = 1
	spawnParmItemSuperShotgun = 2
	spawnParmItemNailgun      = 4
	spawnParmItemSuperNailgun = 8
	spawnParmItemLightning    = 64
	spawnParmItemAxe          = 4096
	defaultFreshSpawnItems    = spawnParmItemShotgun | spawnParmItemAxe
	defaultFreshSpawnHealth   = 100
	defaultFreshSpawnShells   = 25
)

// repairMissingWeaponSpawnParms preserves Quake's assumption that fresh or
// carried spawn parms always describe a usable weapon. Some start/hub maps leave
// parm8 unset, which makes stock QC clear the viewmodel after DecodeLevelParms.
func repairMissingWeaponSpawnParms(client *Client) {
	if client == nil || client.SpawnParms[7] != 0 {
		return
	}
	items := int(client.SpawnParms[0])
	if items == 0 {
		client.SpawnParms[0] = float32(defaultFreshSpawnItems)
		items = defaultFreshSpawnItems
		if client.SpawnParms[1] == 0 {
			client.SpawnParms[1] = defaultFreshSpawnHealth
		}
		if client.SpawnParms[3] == 0 {
			client.SpawnParms[3] = defaultFreshSpawnShells
		}
	}
	client.SpawnParms[7] = float32(bestSpawnParmWeapon(client, items))
}

func bestSpawnParmWeapon(client *Client, items int) int {
	if client == nil {
		return spawnParmItemAxe
	}
	switch {
	case client.SpawnParms[6] >= 1 && items&spawnParmItemLightning != 0:
		return spawnParmItemLightning
	case client.SpawnParms[4] >= 2 && items&spawnParmItemSuperNailgun != 0:
		return spawnParmItemSuperNailgun
	case client.SpawnParms[3] >= 2 && items&spawnParmItemSuperShotgun != 0:
		return spawnParmItemSuperShotgun
	case client.SpawnParms[4] >= 1 && items&spawnParmItemNailgun != 0:
		return spawnParmItemNailgun
	case client.SpawnParms[3] >= 1 && items&spawnParmItemShotgun != 0:
		return spawnParmItemShotgun
	default:
		return spawnParmItemAxe
	}
}
