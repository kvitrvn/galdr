// Package app centralises application state and orchestrates the
// library, the audio player and the user-facing status.
//
// The TUI in internal/tui depends on App; App in turn depends on the
// Player interface from internal/player. App never imports TUI types.
package app

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

// VolumeStep is the amount by which VolumeUp and VolumeDown adjust the
// player volume. It is exported so the TUI can display the same value
// when rendering volume controls.
const VolumeStep = 5

// RepeatMode describes how the queue behaves when the current track
// reaches its end.
type RepeatMode int

const (
	// RepeatOff stops the player at the end of the queue.
	RepeatOff RepeatMode = iota
	// RepeatAll wraps to the first track when the queue ends.
	RepeatAll
	// RepeatOne reloads the current track when it ends.
	RepeatOne
)

// String returns the canonical name of the mode ("off", "all", "one").
func (r RepeatMode) String() string {
	switch r {
	case RepeatAll:
		return "all"
	case RepeatOne:
		return "one"
	default:
		return "off"
	}
}

// App is the central application state. It coordinates the library queue,
// the audio player and the user-facing status.
//
// App is not safe for concurrent use. The Bubble Tea loop in internal/tui
// is the only caller and runs on a single goroutine.
type App struct {
	queue  *library.Queue
	player player.Player
	config *config.Config

	currentTrack *library.Track

	shuffle       bool
	repeat        RepeatMode
	savedVolume   int
	mute          bool
	random        *rand.Rand
	lastPlayedIdx int // index of the most recently played track, for shuffle avoidance

	statusMessage string
	lastError     error
}

// New constructs an App with the given config and audio player.
// The library queue starts empty; call LoadLibrary to populate it.
func New(cfg *config.Config, pl player.Player) *App {
	if cfg == nil {
		cfg = config.Default()
	}
	return &App{
		queue:       library.NewQueue(nil),
		player:      pl,
		config:      cfg,
		savedVolume: pl.Volume(),
		random:      rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0xC0FFEE)),
	}
}

// LoadLibrary scans root for audio tracks and replaces the queue.
// The selection is reset to 0.
func (a *App) LoadLibrary(root string) error {
	tracks, err := library.Scan(root)
	if err != nil {
		a.lastError = fmt.Errorf("scan %s: %w", root, err)
		a.statusMessage = "Library scan failed"
		return err
	}
	a.queue.Replace(tracks)
	a.statusMessage = fmt.Sprintf("Loaded %d tracks", len(tracks))
	return nil
}

// Rescan re-scans the configured music directory. The currently
// selected and currently playing tracks are preserved by their path:
// if the file is still present, selection/currentTrack are restored;
// if a track has been removed, the selection is moved to a safe
// neighbour and the player is stopped.
//
// The shuffle and repeat settings are preserved.
func (a *App) Rescan() error {
	selectedPath := ""
	if sel := a.queue.Current(); sel != nil {
		selectedPath = sel.Path
	}
	currentPath := ""
	if a.currentTrack != nil {
		currentPath = a.currentTrack.Path
	}

	tracks, err := library.Scan(a.config.MusicDir)
	if err != nil {
		a.lastError = fmt.Errorf("rescan %s: %w", a.config.MusicDir, err)
		a.statusMessage = "Rescan failed"
		return err
	}
	a.queue.Replace(tracks)

	// Restore selection.
	if selectedPath != "" {
		for i, t := range tracks {
			if t.Path == selectedPath {
				a.queue.SetIndex(i)
				break
			}
		}
	}
	// If the current track was removed, stop playback.
	if currentPath != "" {
		stillThere := false
		for _, t := range tracks {
			if t.Path == currentPath {
				stillThere = true
				break
			}
		}
		if !stillThere {
			_ = a.Stop()
		}
	}
	a.statusMessage = fmt.Sprintf("Rescanned: %d tracks", len(tracks))
	return nil
}

// Queue exposes the underlying queue for read-only access.
func (a *App) Queue() *library.Queue { return a.queue }

// VisibleTracks returns the tracks that pass the current filter (or
// every track when no filter is set). The TUI renders this slice.
func (a *App) VisibleTracks() []library.Track { return a.queue.VisibleTracks() }

// VisibleLen returns the number of tracks visible under the current
// filter.
func (a *App) VisibleLen() int { return a.queue.VisibleLen() }

// DisplayIndex returns the selected track's position in the visible
// (filtered) view, or -1 if the selection is hidden by the filter.
func (a *App) DisplayIndex() int { return a.queue.DisplayIndex() }

// Filter returns the active filter pattern. An empty string means
// no filter.
func (a *App) Filter() string { return a.queue.Filter() }

// HasFilter reports whether a filter pattern is currently active.
func (a *App) HasFilter() bool { return a.queue.Filter() != "" }

// SetFilter sets the active filter pattern. Pass an empty string to
// clear the filter. When a filter is set, navigation (Next, Previous,
// SelectNext, SelectPrev) and the playback queue all operate on the
// visible subset.
func (a *App) SetFilter(pattern string) {
	a.queue.SetFilter(pattern)
	if pattern == "" {
		a.statusMessage = "Filter cleared"
	} else {
		a.statusMessage = fmt.Sprintf("Filter: %s (%d/%d)",
			pattern, a.queue.VisibleLen(), a.queue.Len())
	}
}

// Selected returns the currently selected track, or nil if the queue
// is empty.
func (a *App) Selected() *library.Track {
	return a.queue.Current()
}

// SelectedIndex returns the queue selection index.
func (a *App) SelectedIndex() int { return a.queue.Index() }

// Current returns the currently playing track, or nil if none.
func (a *App) Current() *library.Track { return a.currentTrack }

// State returns the player state.
func (a *App) State() player.State { return a.player.State() }

// Volume returns the current effective volume in [0, 100]. When the
// app is muted this is 0; SavedVolume reports the last non-muted
// value.
func (a *App) Volume() int {
	if a.mute {
		return 0
	}
	return a.player.Volume()
}

// SavedVolume returns the volume that will be restored when mute is
// turned off. It is independent of the mute state.
func (a *App) SavedVolume() int { return a.savedVolume }

// Muted reports whether the app is currently muting output.
func (a *App) Muted() bool { return a.mute }

// Shuffle reports whether shuffle mode is enabled.
func (a *App) Shuffle() bool { return a.shuffle }

// RepeatMode reports the active repeat mode.
func (a *App) Repeat() RepeatMode { return a.repeat }

// Position returns the current playback position.
func (a *App) Position() time.Duration { return a.player.Position() }

// Duration returns the total length of the current track. A zero
// return value can mean either a track of length 0 or "duration is
// unknown" (see HasDuration). MP3 files report unknown duration
// because VBR-safe estimation requires decoding the whole stream.
func (a *App) Duration() time.Duration { return a.player.Duration() }

// HasDuration reports whether the player knows the duration of the
// current track. It is false for MP3 tracks; true for FLAC and WAV
// (including extended formats).
func (a *App) HasDuration() bool { return a.player.Duration() > 0 }

// Status returns the last user-facing status message.
func (a *App) Status() string { return a.statusMessage }

// Error returns the last error encountered by an action.
func (a *App) Error() error { return a.lastError }

// Config returns the active configuration.
func (a *App) Config() *config.Config { return a.config }

// Snapshot returns the current playback state for persistence. The
// returned value is what the app would restore on the next launch
// via ApplySnapshot.
func (a *App) Snapshot() (volume int, currentPath string) {
	volume = a.savedVolume
	if a.currentTrack != nil {
		currentPath = a.currentTrack.Path
	}
	return volume, currentPath
}

// ApplySnapshot restores the volume and the "last played" reference
// from a previously saved State. The current track is not loaded: the
// TUI uses Snapshot instead when the user wants to play the last
// track.
//
// A volume value outside [0, 100] is ignored (the underlying player
// clamps it anyway).
func (a *App) ApplySnapshot(volume int, currentPath string) {
	if volume < 0 || volume > 100 {
		volume = 100
	}
	a.savedVolume = volume
	_ = a.player.SetVolume(volume)
	a.statusMessage = fmt.Sprintf("Volume: %d%%", volume)
}

// SelectNext moves the selection down by one row. It is a no-op when the
// queue is empty or the selection is already at the last track.
func (a *App) SelectNext() {
	a.queue.Next()
}

// SelectPrev moves the selection up by one row. It is a no-op when the
// queue is empty or the selection is already at the first track.
func (a *App) SelectPrev() {
	a.queue.Previous()
}

// PlaySelected loads the selected track and starts playback.
// If the selected track is already the current track, this toggles
// play/pause instead of reloading.
func (a *App) PlaySelected() error {
	sel := a.queue.Current()
	if sel == nil {
		return errors.New("nothing to play")
	}
	if a.currentTrack != nil && a.currentTrack.Path == sel.Path {
		return a.TogglePlay()
	}
	return a.playAt(a.queue.Index())
}

// TogglePlay toggles between play and pause. When the player is stopped
// it plays the currently selected track.
func (a *App) TogglePlay() error {
	switch a.player.State() {
	case player.StatePlaying:
		return a.pause()
	case player.StatePaused:
		return a.resume()
	default:
		sel := a.queue.Current()
		if sel == nil {
			return errors.New("nothing to play")
		}
		return a.playAt(a.queue.Index())
	}
}

// Stop stops playback and clears the current track reference.
func (a *App) Stop() error {
	if err := a.player.Stop(); err != nil {
		a.lastError = err
		return err
	}
	a.currentTrack = nil
	a.statusMessage = "Stopped"
	return nil
}

// Seek moves playback to the given absolute position. See
// player.Player.Seek for semantics.
func (a *App) Seek(position time.Duration) error {
	if err := a.player.Seek(position); err != nil {
		a.lastError = err
		return err
	}
	return nil
}

// MaybeAdvance auto-advances to the next track if the current one ended
// naturally. It honours the shuffle, repeat and filter settings:
//
//   - RepeatOne reloads the current track.
//   - RepeatAll wraps to the first visible track when the visible view
//     ends.
//   - Shuffle picks a random other visible track (avoiding the one
//     that just ended when the visible view has more than one track).
//   - Otherwise, when the visible view ends, playback stops.
//
// It is a no-op when playback is not in the stopped state or when
// there is no current track.
func (a *App) MaybeAdvance() error {
	if a.player.State() != player.StateStopped {
		return nil
	}
	if a.currentTrack == nil {
		return nil
	}
	if a.queue.VisibleLen() == 0 {
		a.currentTrack = nil
		return nil
	}
	if a.repeat == RepeatOne {
		return a.playAt(a.queue.Index())
	}
	if a.shuffle {
		next, ok := a.randomIndex(a.queue.Index())
		if !ok {
			a.currentTrack = nil
			return nil
		}
		return a.playAt(next)
	}
	if a.queue.Next() {
		return a.playAt(a.queue.Index())
	}
	if a.repeat == RepeatAll {
		if a.queue.FirstVisible() {
			return a.playAt(a.queue.Index())
		}
	}
	a.currentTrack = nil
	a.statusMessage = "End of queue"
	return nil
}

// Next advances to the next visible track and starts playback.
// Behaviour depends on the shuffle, repeat and filter settings:
//
//   - Shuffle on: a random other visible track is picked.
//   - RepeatAll at the end of the visible view: wraps to the first
//     visible track.
//   - Otherwise: at the end of the visible view, playback is stopped.
func (a *App) Next() error {
	if a.queue.VisibleLen() == 0 {
		return errors.New("queue is empty")
	}
	if a.shuffle {
		next, ok := a.randomIndex(a.queue.Index())
		if !ok {
			return a.Stop()
		}
		return a.playAt(next)
	}
	if a.queue.Next() {
		return a.playAt(a.queue.Index())
	}
	if a.repeat == RepeatAll {
		if a.queue.FirstVisible() {
			return a.playAt(a.queue.Index())
		}
	}
	return a.Stop()
}

// Previous moves to the previous visible track and starts playback.
// When shuffle is on, a random other visible track is picked instead
// of the literal previous index. At the start of the visible view (or
// when shuffle fails to find a candidate), this is a no-op.
func (a *App) Previous() error {
	if a.queue.VisibleLen() == 0 {
		return errors.New("queue is empty")
	}
	if a.shuffle {
		next, ok := a.randomIndex(a.queue.Index())
		if !ok {
			return nil
		}
		return a.playAt(next)
	}
	if !a.queue.Previous() {
		return nil
	}
	return a.playAt(a.queue.Index())
}

// randomIndex returns a full-list index in the visible view that is
// distinct from exclude, when possible. It returns ok=false when the
// visible view has fewer than 2 entries. The choice is made via the
// app's RNG; the queue's selection is updated as a side effect so the
// caller can read the resulting full index via queue.Index().
func (a *App) randomIndex(exclude int) (int, bool) {
	n := a.queue.VisibleLen()
	if n < 2 {
		return 0, n >= 1
	}
	excludeVis := a.queue.DisplayIndex()
	for tries := 0; tries < 64; tries++ {
		vis := a.random.IntN(n)
		if vis == excludeVis {
			continue
		}
		if a.queue.SetDisplayIndex(vis) {
			return a.queue.Index(), true
		}
	}
	return 0, false
}

// ToggleShuffle flips the shuffle mode.
func (a *App) ToggleShuffle() {
	a.shuffle = !a.shuffle
	if a.shuffle {
		a.statusMessage = "Shuffle on"
	} else {
		a.statusMessage = "Shuffle off"
	}
}

// CycleRepeat rotates off -> all -> one -> off.
func (a *App) CycleRepeat() {
	switch a.repeat {
	case RepeatOff:
		a.repeat = RepeatAll
		a.statusMessage = "Repeat: all"
	case RepeatAll:
		a.repeat = RepeatOne
		a.statusMessage = "Repeat: one"
	default:
		a.repeat = RepeatOff
		a.statusMessage = "Repeat: off"
	}
}

// ToggleMute flips the mute state. The underlying Oto player is held
// at 0 while mute is on, but the saved volume is preserved so that
// unmuting restores the previous value.
func (a *App) ToggleMute() {
	if !a.mute {
		a.savedVolume = a.player.Volume()
		_ = a.player.SetVolume(0)
		a.mute = true
		a.statusMessage = "Muted"
		return
	}
	_ = a.player.SetVolume(a.savedVolume)
	a.mute = false
	a.statusMessage = fmt.Sprintf("Unmuted (%d%%)", a.savedVolume)
}

// VolumeUp raises the volume by VolumeStep (clamped to 100). While
// muted, the saved value moves but the Oto player stays at 0.
func (a *App) VolumeUp() error {
	target := a.savedVolume + VolumeStep
	if target > 100 {
		target = 100
	}
	a.savedVolume = target
	if a.mute {
		a.statusMessage = fmt.Sprintf("Muted (volume: %d%%)", target)
		return nil
	}
	return a.applyVolume(target)
}

// VolumeDown lowers the volume by VolumeStep (clamped to 0). While
// muted, the saved value moves but the Oto player stays at 0.
func (a *App) VolumeDown() error {
	target := a.savedVolume - VolumeStep
	if target < 0 {
		target = 0
	}
	a.savedVolume = target
	if a.mute {
		return nil
	}
	return a.applyVolume(target)
}

func (a *App) applyVolume(v int) error {
	if err := a.player.SetVolume(v); err != nil {
		a.lastError = err
		return err
	}
	a.statusMessage = fmt.Sprintf("Volume: %d%%", a.player.Volume())
	return nil
}

// playAt loads and plays the track at idx. On success the selection is
// moved to idx and currentTrack is updated. On failure lastError and
// statusMessage reflect the failure, currentTrack is cleared so that
// auto-advance does not loop on a broken track, and the selection is
// preserved.
func (a *App) playAt(idx int) error {
	if idx < 0 || idx >= a.queue.Len() {
		return fmt.Errorf("app: track index %d out of range", idx)
	}
	tracks := a.queue.Tracks()
	t := &tracks[idx]

	if err := a.player.Load(t.Path); err != nil {
		a.lastError = err
		a.statusMessage = "Failed to load track"
		a.currentTrack = nil
		return err
	}
	if err := a.player.Play(); err != nil {
		a.lastError = err
		a.statusMessage = "Failed to start playback"
		a.currentTrack = nil
		return err
	}

	a.currentTrack = t
	a.queue.SetIndex(idx)
	a.lastPlayedIdx = idx
	a.statusMessage = fmt.Sprintf("Playing: %s", t.Title)
	return nil
}

func (a *App) pause() error {
	if err := a.player.Pause(); err != nil {
		a.lastError = err
		return err
	}
	a.statusMessage = "Paused"
	return nil
}

func (a *App) resume() error {
	if err := a.player.Play(); err != nil {
		a.lastError = err
		return err
	}
	a.statusMessage = "Resumed"
	return nil
}
