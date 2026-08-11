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
		wantArgs    []string
		wantErr     string
	}{
		{name: "defaults", args: []string{"+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 1, wantPort: 26000, wantArgs: []string{"+map", "start"}},
		{name: "dedicated default count", args: []string{"-dedicated", "+map", "start"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 8, wantPort: 26000, wantDed: true, wantArgs: []string{"+map", "start"}},
		{name: "listen explicit count and port", args: []string{"+map", "start", "-listen", "4", "-port", "27001"}, wantBaseDir: ".", wantGameDir: "id1", wantMax: 4, wantPort: 27001, wantListen: true, wantArgs: []string{"+map", "start"}},
		{name: "basedir and game anywhere", args: []string{"+map", "e1m1", "-game", "hipnotic", "-basedir", "/tmp/quake"}, wantBaseDir: "/tmp/quake", wantGameDir: "hipnotic", wantMax: 1, wantPort: 26000, wantArgs: []string{"+map", "e1m1"}},
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
			if got.BaseDir != tc.wantBaseDir || got.GameDir != tc.wantGameDir || got.MaxClients != tc.wantMax || got.Port != tc.wantPort || got.Dedicated != tc.wantDed || got.Listen != tc.wantListen || !reflect.DeepEqual(got.Args, tc.wantArgs) {
				t.Fatalf("parseStartupOptions(%v) = %+v, want base=%q game=%q max=%d port=%d dedicated=%v listen=%v args=%v", tc.args, got, tc.wantBaseDir, tc.wantGameDir, tc.wantMax, tc.wantPort, tc.wantDed, tc.wantListen, tc.wantArgs)
			}
		})
	}
}
