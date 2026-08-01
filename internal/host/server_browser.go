package host

import (
	"github.com/darkliquid/ironwail-go/internal/server"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func (h *Host) updateServerBrowserNetworking(subs *Subsystems) {
	if !h.serverActive || subs == nil || subs.Server == nil || subs.Server.MaxClients() <= 1 {
		h.Net.SetServerInfoProvider(nil)
		_ = h.Net.Listen(false)
		return
	}

	provider := h.makeServerInfoProvider(subs)
	if err := h.Net.Listen(true); err != nil {
		h.Net.SetServerInfoProvider(nil)
		_ = h.Net.Listen(false)
		return
	}
	h.Net.SetServerInfoProvider(provider)
}

func (h *Host) makeServerInfoProvider(subs *Subsystems) *inet.ServerInfoProvider {
	if subs == nil || subs.Server == nil {
		return nil
	}

	return &inet.ServerInfoProvider{
		Hostname: h.currentServerHostname,
		MapName: func() string {
			return subs.Server.MapName()
		},
		Players: func() int {
			active := 0
			maxClients := subs.Server.MaxClients()
			for i := 0; i < maxClients; i++ {
				if subs.Server.IsClientActive(i) {
					active++
				}
			}
			return active
		},
		MaxPlayers: func() int {
			return subs.Server.MaxClients()
		},
		PlayerInfo: func(index int) (name string, topColor, bottomColor byte, frags int32, ping float32, ok bool) {
			if index < 0 || index >= subs.Server.MaxClients() || !subs.Server.IsClientActive(index) {
				return "", 0, 0, 0, 0, false
			}
			color := subs.Server.ClientColor(index)
			if edict := subs.Server.EdictNum(index + 1); edict != nil {
				srv, _ := subs.Server.(*server.Server)
				frags = int32(edict.Frags(srv))
			}
			return subs.Server.ClientName(index), byte((color >> 4) & 0x0f), byte(color & 0x0f), frags, subs.Server.ClientPing(index), true
		},
	}
}
