// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import "github.com/darkliquid/ironwail-go/internal/menu"

func (h *Host) CmdToggleMenu() {
	if h.menu == nil {
		return
	}
	h.menu.ToggleMenu()
}

func (h *Host) CmdMenuMain() {
	if h.menu == nil {
		return
	}
	h.menu.ShowMenu()
}

func (h *Host) CmdMenuState(state menu.MenuState) {
	if h.menu == nil {
		return
	}
	h.menu.ShowState(state)
}

func (h *Host) CmdMenuQuit() {
	if h.menu == nil {
		return
	}
	h.menu.ShowQuitPrompt()
}
