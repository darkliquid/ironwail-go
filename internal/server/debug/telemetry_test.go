package debug

import (
	"testing"
)

func TestParseEventMask(t *testing.T) {
	t.Run("named mask", func(t *testing.T) {
		got := ParseEventMask("trigger,qc|frame")
		want := EventMaskTrigger | EventMaskQC | EventMaskFrame
		if got != want {
			t.Fatalf("ParseEventMask() = %v, want %v", got, want)
		}
	})

	t.Run("numeric mask", func(t *testing.T) {
		if got := ParseEventMask("0x21"); got != EventMaskTrigger|EventMaskPhysics {
			t.Fatalf("ParseEventMask() = %v, want %v", got, EventMaskTrigger|EventMaskPhysics)
		}
	})

	t.Run("default all", func(t *testing.T) {
		if got := ParseEventMask(""); got != EventMaskAll {
			t.Fatalf("ParseEventMask() = %v, want %v", got, EventMaskAll)
		}
	})
}

func TestMatchesClassnameFilter(t *testing.T) {
	if !MatchesClassnameFilter("trigger_*", "trigger_once") {
		t.Fatalf("expected trigger_once to match trigger_*")
	}
	if MatchesClassnameFilter("trigger_*", "monster_army") {
		t.Fatalf("expected monster_army not to match trigger_*")
	}
}
