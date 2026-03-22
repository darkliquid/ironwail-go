package host

import (
	"path/filepath"
	"strings"
)

const builtinDefaultCfg = "" +
	"unbindall\n" +
	"\n" +
	"bind ALT +strafe\n" +
	"\n" +
	"bind , +moveleft\n" +
	"bind a +moveleft\n" +
	"bind . +moveright\n" +
	"bind d +moveright\n" +
	"bind DEL +lookdown\n" +
	"bind PGDN +lookup\n" +
	"bind END centerview\n" +
	"\n" +
	"bind e +moveup\n" +
	"bind c +movedown\n" +
	"bind SHIFT +speed\n" +
	"bind CTRL +attack\n" +
	"bind UPARROW +forward\n" +
	"bind w +forward\n" +
	"bind DOWNARROW +back\n" +
	"bind s +back\n" +
	"bind LEFTARROW +left\n" +
	"bind RIGHTARROW +right\n" +
	"\n" +
	"bind SPACE +jump\n" +
	"\n" +
	"bind TAB +showscores\n" +
	"\n" +
	"bind 1 \"impulse 1\"\n" +
	"bind 2 \"impulse 2\"\n" +
	"bind 3 \"impulse 3\"\n" +
	"bind 4 \"impulse 4\"\n" +
	"bind 5 \"impulse 5\"\n" +
	"bind 6 \"impulse 6\"\n" +
	"bind 7 \"impulse 7\"\n" +
	"bind 8 \"impulse 8\"\n" +
	"\n" +
	"bind 0 \"impulse 0\"\n" +
	"\n" +
	"bind / \"impulse 10\"\n" +
	"bind MWHEELDOWN \"impulse 10\"\n" +
	"bind MWHEELUP \"impulse 12\"\n" +
	"\n" +
	"alias zoom_in \"togglezoom\"\n" +
	"alias zoom_out \"togglezoom\"\n" +
	"bind F11 zoom_in\n" +
	"\n" +
	"bind F1 \"help\"\n" +
	"bind F2 \"menu_save\"\n" +
	"bind F3 \"menu_load\"\n" +
	"bind F4 \"menu_options\"\n" +
	"bind F5 \"menu_multiplayer\"\n" +
	"bind F6 \"save quick\"\n" +
	"bind F9 \"load quick\"\n" +
	"bind F10 \"quit\"\n" +
	"bind F12 \"screenshot\"\n" +
	"\n" +
	"bind PRINTSCREEN \"screenshot\"\n" +
	"\n" +
	"bind \"\\\\\" +mlook\n" +
	"\n" +
	"bind PAUSE \"pause\"\n" +
	"bind ESCAPE \"togglemenu\"\n" +
	"bind TILDE \"toggleconsole\"\n" +
	"bind BACKQUOTE \"toggleconsole\"\n" +
	"\n" +
	"bind t \"messagemode\"\n" +
	"\n" +
	"bind + \"sizeup\"\n" +
	"bind = \"sizeup\"\n" +
	"bind - \"sizedown\"\n" +
	"\n" +
	"bind INS +klook\n" +
	"\n" +
	"bind MOUSE1 +attack\n" +
	"bind MOUSE2 +jump\n" +
	"\n" +
	"bind LSHOULDER \"impulse 12\"\n" +
	"bind RSHOULDER \"impulse 10\"\n" +
	"bind LTRIGGER +jump\n" +
	"bind RTRIGGER +attack\n" +
	"\n" +
	"r_gamma 0.95\n" +
	"volume 0.7\n" +
	"sensitivity 3\n" +
	"\n" +
	"viewsize 110\n" +
	"scr_autoscale\n" +
	"\n" +
	"+mlook\n"

func builtinExecConfigText(filename string) (string, bool) {
	cleaned := filepath.Clean(strings.TrimSpace(filename))
	if filepath.IsAbs(cleaned) {
		return "", false
	}
	if !strings.EqualFold(cleaned, "default.cfg") {
		return "", false
	}
	return builtinDefaultCfg, true
}
