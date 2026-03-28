package audio

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestRegisterCVarsUsesCanonicalFilterQualityDefault(t *testing.T) {
	RegisterCVars()

	cv := cvar.Get("snd_filterquality")
	if cv == nil {
		t.Fatal("snd_filterquality not registered")
	}
	if got := cv.DefaultValue; got != "5" {
		t.Fatalf("snd_filterquality default = %q, want %q", got, "5")
	}
}

func TestUpdateFromCVarsAppliesFilterQuality(t *testing.T) {
	RegisterCVars()

	oldVolume := cvar.StringValue("volume")
	oldFilterQuality := cvar.StringValue("snd_filterquality")
	t.Cleanup(func() {
		cvar.Set("volume", oldVolume)
		cvar.Set("snd_filterquality", oldFilterQuality)
	})

	tests := []struct {
		name          string
		filterQuality string
		wantQuality   int
	}{
		{name: "valid quality", filterQuality: "3", wantQuality: 3},
		{name: "out of range falls back", filterQuality: "99", wantQuality: 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mixer := NewMixer()
			sys := &System{
				initialized: true,
				mixer:       mixer,
			}

			cvar.Set("volume", "0.4")
			cvar.Set("snd_filterquality", tc.filterQuality)

			sys.UpdateFromCVars()

			if got := mixer.Volume(); got != 0.4 {
				t.Fatalf("volume = %v, want 0.4", got)
			}
			if got := mixer.filterQuality; got != tc.wantQuality {
				t.Fatalf("filter quality = %d, want %d", got, tc.wantQuality)
			}
		})
	}
}
