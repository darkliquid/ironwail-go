package host

import inet "github.com/ironwail/ironwail-go/internal/net"

func (h *Host) updateServerBrowserNetworking(subs *Subsystems) {
	if !h.serverActive || subs == nil || subs.Server == nil || subs.Server.GetMaxClients() <= 1 {
		inet.SetServerInfoProvider(nil)
		_ = inet.Listen(false)
		return
	}

	provider := makeServerInfoProvider(subs)
	if err := inet.Listen(true); err != nil {
		inet.SetServerInfoProvider(nil)
		_ = inet.Listen(false)
		return
	}
	inet.SetServerInfoProvider(provider)
}

func makeServerInfoProvider(subs *Subsystems) *inet.ServerInfoProvider {
	if subs == nil || subs.Server == nil {
		return nil
	}

	return &inet.ServerInfoProvider{
		Hostname: currentServerHostname,
		MapName: func() string {
			return subs.Server.GetMapName()
		},
		Players: func() int {
			active := 0
			maxClients := subs.Server.GetMaxClients()
			for i := 0; i < maxClients; i++ {
				if subs.Server.IsClientActive(i) {
					active++
				}
			}
			return active
		},
		MaxPlayers: func() int {
			return subs.Server.GetMaxClients()
		},
		PlayerInfo: func(index int) (name string, topColor, bottomColor byte, frags int32, ping float32, ok bool) {
			if index < 0 || index >= subs.Server.GetMaxClients() || !subs.Server.IsClientActive(index) {
				return "", 0, 0, 0, 0, false
			}
			color := subs.Server.GetClientColor(index)
			if edict := subs.Server.EdictNum(index + 1); edict != nil && edict.Vars != nil {
				frags = int32(edict.Vars.Frags)
			}
			return subs.Server.GetClientName(index), byte((color >> 4) & 0x0f), byte(color & 0x0f), frags, subs.Server.GetClientPing(index), true
		},
	}
}
