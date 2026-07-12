package mpris

import (
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"reflect"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

const (
	busName             = "org.mpris.MediaPlayer2.galdr"
	objectPath          = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	rootInterface       = "org.mpris.MediaPlayer2"
	playerInterface     = "org.mpris.MediaPlayer2.Player"
	propertiesInterface = "org.freedesktop.DBus.Properties"
	noTrackPath         = dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack")
)

func playbackStatus(state player.State) string {
	switch state {
	case player.StatePlaying:
		return "Playing"
	case player.StatePaused:
		return "Paused"
	default:
		return "Stopped"
	}
}

func loopStatus(mode app.RepeatMode) string {
	switch mode {
	case app.RepeatAll:
		return "Playlist"
	case app.RepeatOne:
		return "Track"
	default:
		return "None"
	}
}

func repeatMode(status string) (app.RepeatMode, bool) {
	switch status {
	case "None":
		return app.RepeatOff, true
	case "Playlist":
		return app.RepeatAll, true
	case "Track":
		return app.RepeatOne, true
	default:
		return app.RepeatOff, false
	}
}

func durationToMicroseconds(duration time.Duration) int64 {
	if duration <= 0 {
		return 0
	}
	return int64(duration / time.Microsecond)
}

func microsecondsToDuration(value int64) time.Duration {
	const max = int64(^uint64(0)>>1) / int64(time.Microsecond)
	const min = -max - 1
	if value > max {
		return time.Duration(1<<63 - 1)
	}
	if value < min {
		return time.Duration(-1 << 63)
	}
	return time.Duration(value) * time.Microsecond
}

func volumeToMPRIS(volume int) float64 {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	return float64(volume) / 100
}

func volumeFromMPRIS(volume float64) (int, bool) {
	if math.IsNaN(volume) || math.IsInf(volume, 0) || volume < 0 {
		return 0, false
	}
	if volume > 1 {
		volume = 1
	}
	return int(math.Round(volume * 100)), true
}

func trackPath(id library.QueueEntryID) dbus.ObjectPath {
	if id == 0 {
		return noTrackPath
	}
	return dbus.ObjectPath(fmt.Sprintf("/org/mpris/MediaPlayer2/track/%d", id))
}

func localFileURI(path string) string {
	if path == "" {
		return ""
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	return (&url.URL{Scheme: "file", Path: absolute}).String()
}

func metadata(snapshot app.PlaybackSnapshot) map[string]dbus.Variant {
	result := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(trackPath(snapshot.TrackID)),
	}
	if snapshot.Track == nil {
		return result
	}
	track := snapshot.Track
	if track.Title != "" {
		result["xesam:title"] = dbus.MakeVariant(track.Title)
	}
	if track.Artist != "" {
		result["xesam:artist"] = dbus.MakeVariant([]string{track.Artist})
	}
	if track.Album != "" {
		result["xesam:album"] = dbus.MakeVariant(track.Album)
	}
	if length := durationToMicroseconds(snapshot.Duration); length > 0 {
		result["mpris:length"] = dbus.MakeVariant(length)
	}
	if cover := library.FindAlbumCover(track.Path); cover != "" {
		result["mpris:artUrl"] = dbus.MakeVariant(localFileURI(cover))
	}
	return result
}

func changedProperties(before, after map[string]dbus.Variant) map[string]dbus.Variant {
	changed := map[string]dbus.Variant{}
	for name, value := range after {
		previous, ok := before[name]
		if !ok || !reflect.DeepEqual(previous.Value(), value.Value()) {
			changed[name] = value
		}
	}
	return changed
}
