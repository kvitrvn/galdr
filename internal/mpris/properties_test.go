package mpris

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

func TestStateConversions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		state  player.State
		status string
	}{
		{name: "stopped", state: player.StateStopped, status: "Stopped"},
		{name: "playing", state: player.StatePlaying, status: "Playing"},
		{name: "paused", state: player.StatePaused, status: "Paused"},
		{name: "unknown", state: player.State(99), status: "Stopped"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := playbackStatus(tt.state); got != tt.status {
				t.Fatalf("playbackStatus() = %q, want %q", got, tt.status)
			}
		})
	}
}

func TestRepeatConversions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode   app.RepeatMode
		status string
	}{
		{mode: app.RepeatOff, status: "None"},
		{mode: app.RepeatAll, status: "Playlist"},
		{mode: app.RepeatOne, status: "Track"},
	}
	for _, tt := range tests {
		if got := loopStatus(tt.mode); got != tt.status {
			t.Fatalf("loopStatus(%v) = %q, want %q", tt.mode, got, tt.status)
		}
		mode, ok := repeatMode(tt.status)
		if !ok || mode != tt.mode {
			t.Fatalf("repeatMode(%q) = %v, %v", tt.status, mode, ok)
		}
	}
	if _, ok := repeatMode("invalid"); ok {
		t.Fatal("repeatMode accepted an invalid value")
	}
}

func TestTimeConversions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		duration time.Duration
		want     int64
	}{
		{name: "negative", duration: -time.Second, want: 0},
		{name: "zero", duration: 0, want: 0},
		{name: "fractional", duration: 1500*time.Millisecond + 999*time.Nanosecond, want: 1_500_000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := durationToMicroseconds(tt.duration); got != tt.want {
				t.Fatalf("durationToMicroseconds() = %d, want %d", got, tt.want)
			}
		})
	}
	if got := microsecondsToDuration(math.MaxInt64); got != time.Duration(math.MaxInt64) {
		t.Fatalf("overflow = %v, want max duration", got)
	}
	if got := microsecondsToDuration(math.MinInt64); got != time.Duration(math.MinInt64) {
		t.Fatalf("underflow = %v, want min duration", got)
	}
}

func TestVolumeConversions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value float64
		want  int
		ok    bool
	}{
		{value: -0.1, ok: false},
		{value: math.NaN(), ok: false},
		{value: math.Inf(1), ok: false},
		{value: 0, want: 0, ok: true},
		{value: 0.555, want: 56, ok: true},
		{value: 2, want: 100, ok: true},
	}
	for _, tt := range tests {
		got, ok := volumeFromMPRIS(tt.value)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("volumeFromMPRIS(%v) = %d, %v; want %d, %v", tt.value, got, ok, tt.want, tt.ok)
		}
	}
	if got := volumeToMPRIS(55); got != 0.55 {
		t.Fatalf("volumeToMPRIS(55) = %v", got)
	}
}

func TestMetadataAndTrackOccurrenceIDs(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "Björk album")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	trackFile := filepath.Join(dir, "Jóga mix.flac")
	coverFile := filepath.Join(dir, "cover.jpg")
	if err := os.WriteFile(coverFile, []byte("cover"), 0o644); err != nil {
		t.Fatal(err)
	}
	track := &library.Track{
		Path:   trackFile,
		Title:  "Jóga",
		Artist: "Björk",
		Album:  "Homogenic",
	}
	first := metadata(app.PlaybackSnapshot{
		Track: track, TrackID: 7, Duration: 4*time.Minute + 2*time.Second,
	})
	second := metadata(app.PlaybackSnapshot{
		Track: track, TrackID: 8, Duration: 4*time.Minute + 2*time.Second,
	})
	if first["mpris:trackid"].Value() == second["mpris:trackid"].Value() {
		t.Fatal("duplicate files received the same occurrence id")
	}
	if got := first["xesam:artist"].Value().([]string); len(got) != 1 || got[0] != "Björk" {
		t.Fatalf("artist = %#v", got)
	}
	artURL := first["mpris:artUrl"].Value().(string)
	if !strings.Contains(artURL, "Bj%C3%B6rk%20album/cover.jpg") {
		t.Fatalf("art URL is not escaped: %q", artURL)
	}
	missing := metadata(app.PlaybackSnapshot{})
	if len(missing) != 1 || missing["mpris:trackid"].Value() != noTrackPath {
		t.Fatalf("empty metadata = %#v", missing)
	}
}

func TestChangedPropertiesIgnoresEqualValues(t *testing.T) {
	t.Parallel()
	before := map[string]dbus.Variant{"Volume": dbus.MakeVariant(0.5)}
	after := map[string]dbus.Variant{"Volume": dbus.MakeVariant(0.5), "Shuffle": dbus.MakeVariant(true)}
	changed := changedProperties(before, after)
	if len(changed) != 1 || changed["Shuffle"].Value() != true {
		t.Fatalf("changed = %#v", changed)
	}
}

func TestWireValuesDoNotChangeWithInterfaceLocale(t *testing.T) {
	for _, language := range []i18n.Language{i18n.English, i18n.French, i18n.Spanish, i18n.German} {
		a := app.New(config.Default(), player.NewMock(), app.WithTranslator(i18n.New(language)))
		a.CycleRepeat()
		snapshot := a.PlaybackSnapshot()
		if got := playbackStatus(snapshot.State); got != "Stopped" {
			t.Errorf("%s PlaybackStatus = %q", language, got)
		}
		if got := loopStatus(snapshot.Repeat); got != "Playlist" {
			t.Errorf("%s LoopStatus = %q", language, got)
		}
	}
}
