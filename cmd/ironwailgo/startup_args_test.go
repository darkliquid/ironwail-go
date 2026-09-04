//go:build !js || !wasm

package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestStartupMapArg(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "plus map", args: []string{"+map", "start"}, want: "start"},
		{name: "positional map", args: []string{"start"}, want: "start"},
		{name: "plus map wins", args: []string{"start", "+map", "e1m1"}, want: "e1m1"},
		{name: "no map", args: []string{"+skill", "2"}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := startupMapArg(tc.args); got != tc.want {
				t.Fatalf("startupMapArg(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestHasPlusMapArg(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "plus map", args: []string{"+map", "start"}, want: true},
		{name: "plus map missing value", args: []string{"+map"}, want: false},
		{name: "positional map only", args: []string{"start"}, want: false},
		{name: "other plus command", args: []string{"+skill", "2"}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasPlusMapArg(tc.args); got != tc.want {
				t.Fatalf("hasPlusMapArg(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestParseStartupOptions(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		wantBaseDir string
		wantGameDir string
		wantMax     int
		wantPort    int
		wantDed     bool
		wantListen  bool
		wantQCDbg   bool
		wantQCWait  bool
		wantQCDPort int
		wantArgs    []string
		wantErr     string
	}{
		{name: "defaults", args: []string{"+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 1, wantPort: 26000, wantQCDPort: 2345, wantArgs: []string{"+map", "start"}},
		{name: "dedicated default count", args: []string{"-dedicated", "+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 8, wantPort: 26000, wantQCDPort: 2345, wantDed: true, wantArgs: []string{"+map", "start"}},
		{name: "listen explicit count and port", args: []string{"+map", "start", "-listen", "4", "-port", "27001"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 4, wantPort: 27001, wantQCDPort: 2345, wantListen: true, wantArgs: []string{"+map", "start"}},
		{name: "basedir and game anywhere", args: []string{"+map", "e1m1", "-game", "hipnotic", "-basedir", "/tmp/quake"}, wantBaseDir: "/tmp/quake", wantGameDir: "hipnotic", wantMax: 1, wantPort: 26000, wantQCDPort: 2345, wantArgs: []string{"+map", "e1m1"}},
		{name: "qcdbg flag default port", args: []string{"-qcdbg", "+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 1, wantPort: 26000, wantQCDbg: true, wantQCDPort: 2345, wantArgs: []string{"+map", "start"}},
		{name: "qcdbg flag custom port and wait", args: []string{"-qcdbg", "3344", "-qcdbg-wait", "+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 1, wantPort: 26000, wantQCDbg: true, wantQCWait: true, wantQCDPort: 3344, wantArgs: []string{"+map", "start"}},
		{name: "dedicated and listen conflict", args: []string{"-dedicated", "-listen"}, wantErr: "mutually exclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseStartupOptions(tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseStartupOptions(%v) error = %v, want substring %q", tc.args, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseStartupOptions(%v) failed: %v", tc.args, err)
			}
			if got.BaseDir != tc.wantBaseDir || got.GameDir != tc.wantGameDir || got.MaxClients != tc.wantMax || got.Port != tc.wantPort || got.Dedicated != tc.wantDed || got.Listen != tc.wantListen || got.QCDebug != tc.wantQCDbg || got.QCDebugWait != tc.wantQCWait || got.QCDebugPort != tc.wantQCDPort || !reflect.DeepEqual(got.Args, tc.wantArgs) {
				t.Fatalf("parseStartupOptions(%v) = %+v, want base=%q game=%q max=%d port=%d dedicated=%v listen=%v qcdbg=%v qcdbg_wait=%v qcdbg_port=%d args=%v", tc.args, got, tc.wantBaseDir, tc.wantGameDir, tc.wantMax, tc.wantPort, tc.wantDed, tc.wantListen, tc.wantQCDbg, tc.wantQCWait, tc.wantQCDPort, tc.wantArgs)
			}
		})
	}
}

func testFlagLookup(name string) (bool, bool) {
	switch name {
	case "headless", "window", "dedicated":
		return true, true
	case "screenshot", "width", "height", "loglvl", "basedir":
		return true, false
	default:
		return false, false
	}
}

func TestReorderFlagsFirst(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"empty", nil, nil},
		{"only plus args", []string{"+r_crt", "1"}, []string{"+r_crt", "1"}},
		{"flag before plus", []string{"-screenshot", "x.png", "+r_crt", "1"}, []string{"-screenshot", "x.png", "+r_crt", "1"}},
		{"plus before flag", []string{"+r_crt", "1", "-screenshot", "x.png"}, []string{"-screenshot", "x.png", "+r_crt", "1"}},
		{"interleaved mix", []string{"+r_bloom", "1", "-loglvl", "info", "+r_ssao", "1", "-width", "640"},
			[]string{"-loglvl", "info", "-width", "640", "+r_bloom", "1", "+r_ssao", "1"}},
		{"bool flag keeps plus arg", []string{"+r_crt", "1", "-window", "+r_ssao", "1"},
			[]string{"-window", "+r_crt", "1", "+r_ssao", "1"}},
		{"value flag consumes plus-looking value", []string{"-screenshot", "+weird.png", "+r_crt", "1"},
			[]string{"-screenshot", "+weird.png", "+r_crt", "1"}},
		{"equals form", []string{"-width=640", "+r_crt", "1"}, []string{"-width=640", "+r_crt", "1"}},
		{"double dash terminator", []string{"-width", "640", "--", "+r_crt", "1"},
			[]string{"-width", "640", "--", "+r_crt", "1"}},
		{"unknown flag preserved", []string{"+r_crt", "1", "-notaflag"}, []string{"-notaflag", "+r_crt", "1"}},
		{"positional order preserved", []string{"start", "+r_crt", "1", "-window"}, []string{"-window", "start", "+r_crt", "1"}},
		{"lone dash is positional", []string{"-", "-window"}, []string{"-window", "-"}},
		{"double-dash names", []string{"--width", "640", "+r_crt", "1"}, []string{"--width", "640", "+r_crt", "1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reorderFlagsFirst(tc.args, testFlagLookup)
			if len(got) != len(tc.want) {
				t.Fatalf("reorderFlagsFirst(%v) = %v (len %d), want %v (len %d)", tc.args, got, len(got), tc.want, len(tc.want))
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("reorderFlagsFirst(%v)[%d] = %q, want %q (full: %v)", tc.args, i, got[i], tc.want[i], got)
				}
			}
		})
	}
}
