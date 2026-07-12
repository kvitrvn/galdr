package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/library"
)

// queuePanelContent renders the contents of the right Queue
// panel. The panel shows the full queue, with the currently-playing
// track marked with `▶` and positions relative to it.
//
// The cursor (m.queueCursor) is a position in the queue, not in
// the visible list. The cursor's row is highlighted; pressing
// `K`/`J` (or `shift+up`/`shift+down`) moves the highlighted
// track up or down, `d` removes it, and `enter` plays it.
//
// Empty panels show a short help line.
func (m *Model) queuePanelContent(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	q := m.app.Queue()
	if q == nil || q.Len() == 0 {
		return m.styles.EmptyMsg.Render(m.tr.T(i18n.EmptyQueue))
	}
	all := q.Tracks()
	playingIdx := -1
	if current := m.app.Current(); current != nil {
		for i := range all {
			if all[i].Path == current.Path {
				playingIdx = i
				break
			}
		}
	}
	anchor := playingIdx
	if anchor < 0 {
		anchor = q.Index()
	}

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
		lines = append(lines, m.renderQueueRow(
			all[i],
			i,
			anchor,
			playingIdx,
			i == m.queueCursor,
			w,
		))
	}
	return strings.Join(lines, "\n")
}

// renderQueueRow formats one Queue row with a current/selection marker,
// relative position, title and right-aligned duration.
func (m *Model) renderQueueRow(t library.Track, pos, anchor, playingPos int, selected bool, w int) string {
	playing := pos == playingPos
	marker := " "
	switch {
	case playing && selected:
		marker = "▶"
	case playing:
		marker = "▶"
	case selected:
		marker = "›"
	}

	relative := pos - anchor
	position := m.tr.T(i18n.QueueSelected)
	if playing {
		position = m.tr.T(i18n.QueueNow)
	}
	if relative < 0 {
		position = fmt.Sprintf("%d", relative)
	} else if relative > 0 {
		position = fmt.Sprintf("+%d", relative)
	}
	duration := formatDurationOrUnknown(t.Duration, t.Duration > 0)
	prefix := fmt.Sprintf("%s %3s ", marker, position)
	titleW := w - lipgloss.Width(prefix) - lipgloss.Width(duration) - 1
	text := prefix + padRight(t.Title, max(0, titleW)) + " " + duration
	text = fitLine(text, w)

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
