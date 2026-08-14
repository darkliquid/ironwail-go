package commands

import "testing"

func TestClientStringCommandVerb(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"say hello there", "say"},
		{"  GIVE  aim\\good", "give"},
		{"color 12 5", "color"},
	}
	for _, c := range cases {
		if got := ClientStringCommandVerb(c.cmd); got != c.want {
			t.Errorf("ClientStringCommandVerb(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestClientStringCommandArgs(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"", ""},
		{"name", ""},
		{"name \"Player One\"", "\"Player One\""},
		{"color 12 5", "12 5"},
	}
	for _, c := range cases {
		if got := ClientStringCommandArgs(c.cmd); got != c.want {
			t.Errorf("ClientStringCommandArgs(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestParseClientNameCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want string
	}{
		{"name", ""},
		{"name \"Player One\"", "Player One"},
		{"name averylongplayernameoverfifteen", "averylongplayer"},
	}
	for _, c := range cases {
		if got := ParseClientNameCommand(c.cmd); got != c.want {
			t.Errorf("ParseClientNameCommand(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestParseClientColorCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want int
	}{
		{"color", 0},
		{"color 3", 3},
		{"color 12 5", 12*16 + 5},
		{"color 17 18", 1*16 + 2},
	}
	for _, c := range cases {
		if got := ParseClientColorCommand(c.cmd); got != c.want {
			t.Errorf("ParseClientColorCommand(%q) = %d, want %d", c.cmd, got, c.want)
		}
	}
}
