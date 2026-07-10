package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// formatDuration renders d as m:ss. Negative or zero durations render
// as "0:00".
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0:00"
	}
	total := int(d / time.Second)
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// cellTruncate truncates by terminal-cell width, preserving ANSI sequences and
// grapheme clusters. This keeps CJK, emoji and combining metadata aligned.
func cellTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

func padRight(s string, width int) string {
	s = cellTruncate(s, width)
	if missing := width - lipgloss.Width(s); missing > 0 {
		s += strings.Repeat(" ", missing)
	}
	return s
}

func padLeft(s string, width int) string {
	s = cellTruncate(s, width)
	if missing := width - lipgloss.Width(s); missing > 0 {
		s = strings.Repeat(" ", missing) + s
	}
	return s
}

// formatDurationOrUnknown renders d as m:ss. When known is false it
// returns "--:--" so the user can distinguish "no duration available"
// (MP3) from "track of length 0".
func formatDurationOrUnknown(d time.Duration, known bool) string {
	if !known {
		return "--:--"
	}
	return formatDuration(d)
}

// renderProgressBar renders a textual progress bar of width characters.
// If total is 0 or unknown, an empty bar of width characters is returned.
func renderProgressBar(pos, total time.Duration, width int) string {
	if width < 1 {
		return ""
	}
	if total <= 0 {
		return "[" + strings.Repeat("·", width) + "]"
	}
	if pos < 0 {
		pos = 0
	}
	if pos > total {
		pos = total
	}
	filled := int(float64(width) * float64(pos) / float64(total))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("▓", filled) + strings.Repeat("·", width-filled) + "]"
}
