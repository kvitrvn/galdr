package tui

import (
	"fmt"
	"strings"

	"github.com/kvitrvn/galdr/internal/theme"
)

// PanelID identifies one of the three panels in the main TUI
// layout. The Library panel is on the left, the Tracks panel in
// the centre, and the Queue panel on the right.
type PanelID int

const (
	PanelLibrary PanelID = iota
	PanelTracks
	PanelQueue
)

// String returns the canonical name of the panel, lower-case.
func (p PanelID) String() string {
	switch p {
	case PanelLibrary:
		return "library"
	case PanelTracks:
		return "tracks"
	case PanelQueue:
		return "queue"
	default:
		return "unknown"
	}
}

// UIConfig controls the TUI layout: panel widths and the minimum
// terminal size below which galdr renders a "too small" message.
//
// It mirrors the [ui] section of config.toml; the model reads
// cfg.UI and feeds it to Compute. We keep a local copy of the
// struct so the TUI package does not import the config package —
// the conversion is done in model.go.
type UIConfig struct {
	// LeftWidth is the width in columns of the Library panel.
	LeftWidth int
	// RightWidth is the width in columns of the Queue panel.
	RightWidth int
	// MinWidth is the minimum terminal width below which the TUI
	// refuses to render.
	MinWidth int
	// MinHeight is the minimum terminal height below which the
	// TUI refuses to render.
	MinHeight int
}

// DefaultUIConfig returns the built-in UI config used when the
// user has no [ui] section in their config.toml.
func DefaultUIConfig() UIConfig {
	return UIConfig{
		LeftWidth:  22,
		RightWidth: 22,
		MinWidth:   80,
		MinHeight:  24,
	}
}

// Panel describes the rectangle of a single panel inside the TUI
// and how to render its content. A Panel is purely a rendering
// primitive: it does not know about input or focus cycling — that
// lives in the Bubble Tea model.
//
// The Content function is called with the inner dimensions of the
// panel (Width - 2, Height - 2, accounting for the box-drawing
// border). It must return a string of at most those dimensions.
type Panel struct {
	ID      PanelID
	X, Y    int
	W, H    int
	Title   string
	Focused bool
	styles  theme.Palette
	Content func(w, h int) string
}

// View renders the panel: applies the appropriate border (focused
// or dim) and lays the Content inside, with a title bar on the
// first line.
//
// Layout of a rendered panel:
//
//	┌────────────┐
//	│ Title      │  <- styled title, one row
//	│ content    │  <- Content output (H-3 rows)
//	│ ...        │
//	└────────────┘
//
// The Content function is called with the inner dimensions minus
// the title row: (W-2, H-3). When p.W or p.H is below the
// minimum that can host a title + a one-row content, View falls
// back to plain border rendering.
//
// Note on Lip Gloss dimensions (v1.1.0): on a style with a
// border, Width(N) produces a CELL WIDTH of N+2 (the border adds
// one cell per side) and Height(N) produces a CELL HEIGHT of N+2
// as well. To get a panel whose outer rectangle is exactly p.W
// cells wide and p.H cells tall, we pass Width(p.W-2) and
// Height(p.H-2). The visual width / height stay equal to p.W /
// p.H as long as p.W >= 2 and p.H >= 2.
func (p Panel) View() string {
	if p.W < 2 || p.H < 2 {
		return ""
	}
	style := p.styles.DimBorder
	if p.Focused {
		style = p.styles.FocusedBorder
	}
	innerW := p.W - 2
	innerH := p.H - 2
	hasTitle := p.Title != "" && innerW > 0 && innerH >= 2

	var content string
	if p.Content != nil {
		h := innerH
		if hasTitle {
			h = innerH - 1
		}
		content = p.Content(innerW, h)
	}

	if hasTitle {
		title := p.styles.PanelTitle.Width(innerW).Render(p.Title)
		content = title + "\n" + content
	} else if content == "" {
		content = strings.Repeat(" ", innerW)
	}

	return style.
		Width(p.W - 2).
		Height(p.H - 2).
		Render(content)
}

// Layout is the geometry of the main TUI for a given terminal
// size. It is computed on every View (and on every WindowSizeMsg)
// so the panels always match the actual terminal dimensions.
//
// When the terminal is below the configured minimum size, the
// other fields are zeroed and the caller should render
// TooSmallMsg instead of the panels.
type Layout struct {
	Width, Height int
	Library       Panel
	Tracks        Panel
	Queue         Panel
	// StatusY is the y row of the bottom status bar (a single
	// line). It is height - 1 for a normal layout.
	StatusY int

	// TooSmall is true when the terminal is below cfg.MinWidth
	// or cfg.MinHeight.
	TooSmall    bool
	TooSmallMsg string
}

// Compute returns the layout for a terminal of the given size.
// When the size is below the configured minimum, the returned
// layout's TooSmall flag is set and TooSmallMsg holds a
// human-readable explanation.
//
// The three panels are arranged horizontally:
// Library (cfg.LeftWidth) | Tracks (rest) | Queue (cfg.RightWidth).
// The status bar is the last row. The panel rectangles always
// cover the full width and (height - 1) rows.
func Compute(width, height int, cfg UIConfig, styles theme.Palette) Layout {
	if width < cfg.MinWidth || height < cfg.MinHeight {
		return Layout{
			Width:       width,
			Height:      height,
			TooSmall:    true,
			TooSmallMsg: tooSmallMessage(width, height, cfg),
		}
	}

	panelH := height - 1
	centerW := width - cfg.LeftWidth - cfg.RightWidth
	if centerW < 1 {
		centerW = 1
	}

	return Layout{
		Width:  width,
		Height: height,
		Library: Panel{
			ID:     PanelLibrary,
			X:      0,
			Y:      0,
			W:      cfg.LeftWidth,
			H:      panelH,
			Title:  " Library ",
			styles: styles,
			Content: placeholderContent(
				styles, "Library — Phase 14",
			),
		},
		Tracks: Panel{
			ID:     PanelTracks,
			X:      cfg.LeftWidth,
			Y:      0,
			W:      centerW,
			H:      panelH,
			Title:  " Tracks ",
			styles: styles,
			Content: placeholderContent(
				styles, "",
			),
		},
		Queue: Panel{
			ID:     PanelQueue,
			X:      cfg.LeftWidth + centerW,
			Y:      0,
			W:      cfg.RightWidth,
			H:      panelH,
			Title:  " Queue ",
			styles: styles,
			Content: placeholderContent(
				styles, "Queue — Phase 15",
			),
		},
		StatusY: height - 1,
	}
}

func tooSmallMessage(width, height int, cfg UIConfig) string {
	return fmt.Sprintf(
		"galdr needs at least %dx%d — you have %dx%d",
		cfg.MinWidth, cfg.MinHeight, width, height,
	)
}

// placeholderContent returns a Content function that fills the
// panel with an italic muted message. An empty msg produces an
// empty content function (the panel renders as a blank box, which
// is the right default for the Tracks panel — the model fills it
// with the queue list at render time).
func placeholderContent(styles theme.Palette, msg string) func(w, h int) string {
	if msg == "" {
		return func(int, int) string { return "" }
	}
	return func(w, h int) string {
		if w <= 0 || h <= 0 {
			return ""
		}
		return styles.EmptyMsg.Width(w).Height(h).Render(msg)
	}
}
