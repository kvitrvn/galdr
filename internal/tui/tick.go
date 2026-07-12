package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/player"
)

// tickInterval is how often the TUI re-renders to refresh the playback
// position. It is intentionally short so the progress bar updates
// smoothly even for slow-position backends.
const tickInterval = 250 * time.Millisecond

// tickMsg is delivered by tickCmd every tickInterval.
type tickMsg time.Time

// tickCmd returns a tea.Cmd that emits a tickMsg after tickInterval.
// Bubble Tea dispatches the resulting tickMsg to Update, where we
// schedule the next tick.
func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type playbackEventMsg struct {
	event player.PlaybackEvent
	ok    bool
}

func waitPlaybackEventCmd(events <-chan player.PlaybackEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		return playbackEventMsg{event: event, ok: ok}
	}
}
