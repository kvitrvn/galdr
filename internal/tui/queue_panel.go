package tui

import (
	"fmt"
	"strings"

	"github.com/kvitrvn/galdr/internal/library"
)

// queuePanelContent renders the contents of the right Queue
// panel. The panel shows the full queue (the underlying list
// of tracks), with the currently-playing track marked with `▶`
// at the top of the list.
//
// The cursor (m.queueCursor) is a position in the queue, not in
// the visible list. The cursor's row is highlighted; pressing
// `J`/`K` (or `shift+up`/`shift+down`) moves the highlighted
// track up or down, `d` removes it, and `enter` plays it.
//
// Empty panels show a short help line.
func (m *Model) queuePanelContent(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	q := m.app.Queue()
	if q == nil || q.Len() == 0 {
		return m.styles.EmptyMsg.Width(w).Height(h).
			Render("Queue empty.\nPress r to scan.")
	}
	all := q.Tracks()
	playingIdx := q.Index()

	// Keep cursor in range.
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}
	if m.queueCursor >= len(all) {
		m.queueCursor = len(all) - 1
	}

	win := h
	start := scrollStart(len(all), m.queueCursor, win)

	var lines []string
	for i := start; i < len(all) && i < start+win; i++ {
		lines = append(lines, m.renderQueueRow(all[i], i, i == playingIdx, i == m.queueCursor, w))
	}
	return strings.Join(lines, "\n")
}

// renderQueueRow formats a single Queue row. The marker is:
//   - `▶` for the currently-playing track (always, regardless of
//     cursor position).
//   - ` ` for the cursor row when it is not the playing track.
//   - ` ` otherwise.
func (m *Model) renderQueueRow(t library.Track, pos int, playing, selected bool, w int) string {
	var marker string
	switch {
	case playing && selected:
		marker = "▶▶"
	case playing:
		marker = "▶ "
	case selected:
		marker = "▶ "
	default:
		marker = "  "
	}

	// Show the position number plus the title, with a "now
	// playing" tag for the active row. Numbers help the user
	// keep track of "next" vs "history" by visual position.
	var prefix string
	if playing {
		prefix = fmt.Sprintf("now ")
	} else if pos < m.app.Queue().Index() {
		prefix = "←   " // already played
	} else {
		prefix = fmt.Sprintf("%2d. ", pos+1)
	}
	label := prefix + t.Title
	text := fmt.Sprintf("%s %s", marker, truncate(label, w-3))

	switch {
	case selected && playing:
		return m.styles.SelectedRow.Render(text)
	case selected:
		return m.styles.SelectedRow.Render(text)
	case playing:
		return m.styles.PlayingRow.Render(text)
	default:
		return m.styles.Row.Render(text)
	}
}
