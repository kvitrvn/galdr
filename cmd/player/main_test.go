package main

import (
	"testing"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player"
)

func TestPlayerOptionsFromConfig(t *testing.T) {
	tests := []struct {
		name string
		mode config.ReplayGainMode
		want player.ReplayGainMode
	}{
		{name: "off", mode: config.ReplayGainOff, want: player.ReplayGainOff},
		{name: "track", mode: config.ReplayGainTrack, want: player.ReplayGainTrack},
		{name: "album", mode: config.ReplayGainAlbum, want: player.ReplayGainAlbum},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Audio.ReplayGain = tt.mode
			if got := playerOptionsFromConfig(cfg).ReplayGain; got != tt.want {
				t.Errorf("ReplayGain = %v, want %v", got, tt.want)
			}
		})
	}
}
