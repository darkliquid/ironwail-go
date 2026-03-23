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
	}
}
