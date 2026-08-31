// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package loc

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseLocalizationText(t *testing.T) {
	sample := "\xEF\xBB\xBF" + `// Comment line
# Another comment
m_start = "Start"
m_quit = "Quit Game"
map_skill_normal = "This hall selects NORMAL skill"
unquoted_val = Hello World
escaped_str = "Line1\nLine2\t\"Quoted\"\\Backslash"
`
	l := New()
	if err := l.Load(strings.NewReader(sample)); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	tests := []struct {
		key  string
		want string
	}{
		{"$m_start", "Start"},
		{"$m_quit", "Quit Game"},
		{"$map_skill_normal", "This hall selects NORMAL skill"},
		{"$unquoted_val", "Hello World"},
		{"$escaped_str", "Line1\nLine2\t\"Quoted\"\\Backslash"},
		{"$missing_key", "$missing_key"},
		{"plain string", "plain string"},
		{"", ""},
	}

	for _, tc := range tests {
		got := l.GetString(tc.key)
		if got != tc.want {
			t.Errorf("GetString(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}

func TestGetRawString(t *testing.T) {
	sample := `m_start = "Start"`
	l := New()
	if err := l.Load(strings.NewReader(sample)); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if got := l.GetRawString("$m_start"); got != "Start" {
		t.Errorf("GetRawString(\"$m_start\") = %q, want \"Start\"", got)
	}
	if got := l.GetRawString("$missing"); got != "" {
		t.Errorf("GetRawString(\"$missing\") = %q, want \"\"", got)
	}
	if got := l.GetRawString("not_a_key"); got != "" {
		t.Errorf("GetRawString(\"not_a_key\") = %q, want \"\"", got)
	}
}

func TestHasPlaceholders(t *testing.T) {
	tests := []struct {
		str  string
		want bool
	}{
		{"Hello {}", true},
		{"Hello {0}", true},
		{"Hello {1} and {2}", true},
		{"Hello world", false},
		{"Hello {not a number}", false},
		{"Hello {", false},
		{"Hello }", false},
		{"", false},
	}

	for _, tc := range tests {
		got := HasPlaceholders(tc.str)
		if got != tc.want {
			t.Errorf("HasPlaceholders(%q) = %v, want %v", tc.str, got, tc.want)
		}
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		format string
		args   []string
		want   string
	}{
		{
			format: "Found {} of {} secrets",
			args:   []string{"3", "10"},
			want:   "Found 3 of 10 secrets",
		},
		{
			format: "{1} is after {0}",
			args:   []string{"first", "second"},
			want:   "second is after first",
		},
		{
			format: "No placeholders here",
			args:   []string{"unused"},
			want:   "No placeholders here",
		},
		{
			format: "Arg out of bounds {5}",
			args:   []string{"a", "b"},
			want:   "Arg out of bounds ",
		},
	}

	for _, tc := range tests {
		got := Format(tc.format, func(idx int) string {
			if idx >= 0 && idx < len(tc.args) {
				return tc.args[idx]
			}
			return ""
		})
		if got != tc.want {
			t.Errorf("Format(%q, %v) = %q, want %q", tc.format, tc.args, got, tc.want)
		}
	}
}

func TestLoadZipArchive(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	f1, err := zw.Create("localization/loc_english.txt")
	if err != nil {
		t.Fatalf("Create zip entry 1: %v", err)
	}
	if _, err := f1.Write([]byte("map_skill_normal = \"This hall selects NORMAL skill\"\n")); err != nil {
		t.Fatalf("Write zip entry 1: %v", err)
	}

	f2, err := zw.Create("localization/loc_french.txt")
	if err != nil {
		t.Fatalf("Create zip entry 2: %v", err)
	}
	if _, err := f2.Write([]byte("map_skill_normal = \"Ce hall selectionne la competence NORMALE\"\n")); err != nil {
		t.Fatalf("Write zip entry 2: %v", err)
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("Close zip: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("NewReader zip: %v", err)
	}

	l := New()
	if err := l.LoadFromZip(zr, "localization/loc_english.txt"); err != nil {
		t.Fatalf("LoadFromZip english: %v", err)
	}

	if got := l.GetString("$map_skill_normal"); got != "This hall selects NORMAL skill" {
		t.Errorf("GetString() = %q, want English string", got)
	}

	l2 := New()
	if err := l2.LoadFromZip(zr, "localization/loc_french.txt"); err != nil {
		t.Fatalf("LoadFromZip french: %v", err)
	}
	if got := l2.GetString("$map_skill_normal"); got != "Ce hall selectionne la competence NORMALE" {
		t.Errorf("GetString() = %q, want French string", got)
	}
}

func TestRealQuakeEXInit(t *testing.T) {
	quakeDir := "/home/darkliquid/Games/Heroic/Quake Enhanced"
	if _, err := os.Stat(quakeDir); err != nil {
		t.Skip("Quake Enhanced not found")
	}
	Init(quakeDir, "english")
	got := GetString("$map_skill_normal")
	t.Logf("GetString($map_skill_normal) = %q", got)
	if got != "This hall selects NORMAL skill" {
		t.Errorf("Real QuakeEX.kpf init failed: got %q", got)
	}
}
