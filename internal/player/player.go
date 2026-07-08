// Package player defines the playback interface and provides playback
// implementations.
//
// The TUI and app layers must depend on the Player interface defined in
// this package and never on a concrete audio backend.
package player

import (
	"fmt"
	"time"
)

// State reports the current high-level playback state of a Player.
type State int

const (
	// StateStopped means no track is loaded or playback has been stopped.
	StateStopped State = iota
	// StatePlaying means a track is loaded and actively playing.
	StatePlaying
	// StatePaused means a track is loaded but playback is paused.
	StatePaused
)

// String returns a human-readable label for the state.
func (s State) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StatePlaying:
		return "playing"
	case StatePaused:
		return "paused"
	default:
		return fmt.Sprintf("state(%d)", int(s))
	}
}

// Player is the audio backend contract used by the app layer.
//
// All methods must be safe to call from a single goroutine. The TUI and
// app layers depend on this interface; concrete backends (Oto, mock, ...)
// live in subpackages or alongside this file.
type Player interface {
	// Load prepares the track at path for playback. It does not start
	// playback; call Play afterwards. Calling Load while a track is
	// already loaded replaces it.
	Load(path string) error

	// Play starts or resumes playback of the loaded track.
	// Calling Play on a stopped or freshly loaded track starts from the
	// beginning. Calling Play when already playing is a no-op.
	Play() error

	// Pause halts playback without releasing the loaded track.
	// Calling Pause when already paused or stopped is a no-op.
	Pause() error

	// Stop halts playback and releases the loaded track. After Stop,
	// Position is reset to 0 and Duration is 0 until Load is called again.
	Stop() error

	// SetVolume adjusts the output volume. The value is clamped to the
	// inclusive range [0, 100]; values outside the range are clamped
	// rather than rejected.
	SetVolume(vol int) error

	// Volume returns the current volume in the range [0, 100].
	Volume() int

	// Position returns the current playback position. Position is 0 when
	// nothing is loaded.
	Position() time.Duration

	// Duration returns the total length of the loaded track. Duration is
	// 0 when nothing is loaded.
	Duration() time.Duration

	// State returns the current high-level playback state.
	State() State

	// Seek moves playback to the given absolute position within the
	// loaded track. The value is clamped to [0, Duration]. Calling
	// Seek without a loaded track returns an error.
	//
	// For MP3 sources, seeking is implemented by re-decoding from the
	// start of the file and discarding samples, which is correct for
	// VBR but can be slow. FLAC and WAV seek efficiently.
	Seek(position time.Duration) error
}
