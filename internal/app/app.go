// Package app centralises application state and orchestrates the
// library, the audio player and the user-facing status.
//
// The TUI in internal/tui depends on App; App in turn depends on the
// Player interface from internal/player. App never imports TUI types.
package app

import (
	"errors"
	"fmt"
	"time"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

// VolumeStep is the amount by which VolumeUp and VolumeDown adjust the
// player volume. It is exported so the TUI can display the same value
// when rendering volume controls.
const VolumeStep = 5

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
		queue:  library.NewQueue(nil),
		player: pl,
		config: cfg,
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

// Queue exposes the underlying queue for read-only access.
func (a *App) Queue() *library.Queue { return a.queue }

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

// Volume returns the current volume in [0, 100].
func (a *App) Volume() int { return a.player.Volume() }

// Position returns the current playback position.
func (a *App) Position() time.Duration { return a.player.Position() }

// Duration returns the total length of the current track.
func (a *App) Duration() time.Duration { return a.player.Duration() }

// Status returns the last user-facing status message.
func (a *App) Status() string { return a.statusMessage }

// Error returns the last error encountered by an action.
func (a *App) Error() error { return a.lastError }

// Config returns the active configuration.
func (a *App) Config() *config.Config { return a.config }

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

// MaybeAdvance auto-advances to the next track if the current one ended
// naturally. It is meant to be called periodically by the TUI (e.g.
// from a tick message). It is a no-op when:
//
//   - playback is not in the stopped state, or
//   - there is no current track (nothing to advance from), or
//   - the last user action was an explicit Stop (in which case
//     currentTrack has been cleared by Stop).
//
// When the queue is exhausted, MaybeAdvance clears currentTrack so the
// next call returns silently.
func (a *App) MaybeAdvance() error {
	if a.player.State() != player.StateStopped {
		return nil
	}
	if a.currentTrack == nil {
		return nil
	}
	if a.queue.Len() == 0 {
		a.currentTrack = nil
		return nil
	}
	next := a.queue.Index() + 1
	if next >= a.queue.Len() {
		// End of queue: do not loop; mark as no longer playing.
		a.currentTrack = nil
		a.statusMessage = "End of queue"
		return nil
	}
	return a.playAt(next)
}

// Next advances to the next track in the queue and starts playback.
// At the end of the queue, playback is stopped.
func (a *App) Next() error {
	if a.queue.Len() == 0 {
		return errors.New("queue is empty")
	}
	next := a.queue.Index() + 1
	if next >= a.queue.Len() {
		return a.Stop()
	}
	return a.playAt(next)
}

// Previous moves to the previous track in the queue and starts playback.
// At the start of the queue, this is a no-op.
func (a *App) Previous() error {
	if a.queue.Len() == 0 {
		return errors.New("queue is empty")
	}
	prev := a.queue.Index() - 1
	if prev < 0 {
		return nil
	}
	return a.playAt(prev)
}

// VolumeUp raises the volume by VolumeStep (clamped to 100).
func (a *App) VolumeUp() error {
	return a.setVolume(a.player.Volume() + VolumeStep)
}

// VolumeDown lowers the volume by VolumeStep (clamped to 0).
func (a *App) VolumeDown() error {
	return a.setVolume(a.player.Volume() - VolumeStep)
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

func (a *App) setVolume(v int) error {
	if err := a.player.SetVolume(v); err != nil {
		a.lastError = err
		return err
	}
	a.statusMessage = fmt.Sprintf("Volume: %d%%", a.player.Volume())
	return nil
}
