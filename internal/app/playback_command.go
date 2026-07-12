package app

import (
	"errors"
	"time"

	"github.com/kvitrvn/galdr/internal/library"
)

// PlaybackAction identifies a command entering the App owner goroutine.
type PlaybackAction uint8

const (
	PlaybackActionPlay PlaybackAction = iota + 1
	PlaybackActionPause
	PlaybackActionPlayPause
	PlaybackActionStop
	PlaybackActionNext
	PlaybackActionPrevious
	PlaybackActionSeek
	PlaybackActionSetPosition
	PlaybackActionSetVolume
	PlaybackActionSetShuffle
	PlaybackActionSetRepeat
)

// PlaybackCommand contains transport-independent playback intent.
type PlaybackCommand struct {
	Action   PlaybackAction
	Position time.Duration
	TrackID  library.QueueEntryID
	Volume   int
	Shuffle  bool
	Repeat   RepeatMode
}

// PlaybackRequest is a Bubble Tea message carrying a command and its reply.
type PlaybackRequest struct {
	PlaybackCommand
	reply chan PlaybackResult
}

// PlaybackResult reports the action result and newly published state.
type PlaybackResult struct {
	Snapshot PlaybackSnapshot
	Err      error
	Seeked   bool
}

// NewPlaybackRequest constructs a request with a non-blocking reply channel.
func NewPlaybackRequest(command PlaybackCommand) PlaybackRequest {
	return PlaybackRequest{
		PlaybackCommand: command,
		reply:           make(chan PlaybackResult, 1),
	}
}

// Apply executes the request on the App owner goroutine.
func (r PlaybackRequest) Apply(a *App) PlaybackResult {
	var err error
	seeked := false
	switch r.Action {
	case PlaybackActionPlay:
		err = a.Play()
	case PlaybackActionPause:
		err = a.Pause()
	case PlaybackActionPlayPause:
		err = a.TogglePlay()
	case PlaybackActionStop:
		err = a.Stop()
	case PlaybackActionNext:
		err = a.Next()
	case PlaybackActionPrevious:
		err = a.Previous()
	case PlaybackActionSeek:
		target := addPlaybackDuration(a.Position(), r.Position)
		err = a.Seek(target)
		seeked = err == nil
	case PlaybackActionSetPosition:
		if r.TrackID != a.PlaybackSnapshot().TrackID {
			break
		}
		err = a.Seek(r.Position)
		seeked = err == nil
	case PlaybackActionSetVolume:
		err = a.SetVolume(r.Volume)
	case PlaybackActionSetShuffle:
		a.SetShuffle(r.Shuffle)
	case PlaybackActionSetRepeat:
		a.SetRepeat(r.Repeat)
	default:
		err = errors.New("unsupported playback action")
	}
	return PlaybackResult{Snapshot: a.PlaybackSnapshot(), Err: err, Seeked: seeked}
}

// Respond completes a request without blocking the owner goroutine.
func (r PlaybackRequest) Respond(result PlaybackResult) {
	select {
	case r.reply <- result:
	default:
	}
}

// Reply returns the request's single-result channel.
func (r PlaybackRequest) Reply() <-chan PlaybackResult { return r.reply }

func addPlaybackDuration(left, right time.Duration) time.Duration {
	if right > 0 && left > time.Duration(1<<63-1)-right {
		return time.Duration(1<<63 - 1)
	}
	if right < 0 && left < time.Duration(-1<<63)-right {
		return time.Duration(-1 << 63)
	}
	return left + right
}
