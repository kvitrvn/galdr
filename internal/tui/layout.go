package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kvitrvn/galdr/internal/theme"
)

// PanelID identifies one of the three navigation spaces.
type PanelID int

const (
	PanelLibrary PanelID = iota
	PanelTracks
	PanelQueue
)

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

// LayoutMode describes how many navigation panels fit in the terminal.
type LayoutMode int

const (
	LayoutCompact LayoutMode = iota
	LayoutMedium
	LayoutWide
)

func (m LayoutMode) String() string {
	switch m {
	case LayoutWide:
		return "wide"
	case LayoutMedium:
		return "medium"
	default:
		return "compact"
	}
}

// Rect is a terminal-cell rectangle.
type Rect struct {
	X, Y int
	W, H int
}

// UIConfig mirrors the [ui] TOML section without importing config.
type UIConfig struct {
	LeftWidth  int
	RightWidth int
	MinWidth   int
	MinHeight  int
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		LeftWidth:  22,
		RightWidth: 22,
		MinWidth:   48,
		MinHeight:  14,
	}
}

// Panel is a borderless navigation section. A title and a horizontal
// separator consume the first two rows; the remaining rows are content.
type Panel struct {
	ID      PanelID
	X, Y    int
	W, H    int
	Title   string
	Focused bool
	styles  theme.Palette
	Content func(w, h int) string
}

func (p Panel) View() string {
	if p.W <= 0 || p.H <= 0 {
		return ""
	}

	marker := "·"
	titleStyle := p.styles.PanelTitle
	if p.Focused {
		marker = "●"
		titleStyle = p.styles.FocusedTitle
	}
	title := titleStyle.Render(cellTruncate(marker+" "+p.Title, p.W))
	lines := []string{fitLine(title, p.W)}
	if p.H > 1 {
		lines = append(lines, p.styles.Divider.Render(strings.Repeat("─", p.W)))
	}

	contentH := p.H - len(lines)
	content := ""
	if p.Content != nil && contentH > 0 {
		content = p.Content(p.W, contentH)
	}
	lines = append(lines, fitLines(content, p.W, contentH)...)
	return strings.Join(lines, "\n")
}

// Layout contains the complete vertical geometry plus the three possible
// navigation rectangles. In medium mode Tracks and Queue intentionally share
// a rectangle; in compact mode all three do.
type Layout struct {
	Width, Height int
	Mode          LayoutMode
	NowPlaying    Rect
	Library       Panel
	Tracks        Panel
	Queue         Panel
	Footer        Rect
	StatusY       int
	TooSmall      bool
	TooSmallMsg   string
}

func Compute(width, height int, cfg UIConfig, styles theme.Palette) Layout {
	if width < cfg.MinWidth || height < cfg.MinHeight {
		return Layout{
			Width:       width,
			Height:      height,
			TooSmall:    true,
			TooSmallMsg: tooSmallMessage(width, height, cfg),
		}
	}

	mode := layoutMode(width)
	headerH := 5
	if height < 20 {
		headerH = 3
	}
	footerH := 2
	bodyY := headerH
	bodyH := height - headerH - footerH
	footerY := height - footerH

	base := Layout{
		Width:      width,
		Height:     height,
		Mode:       mode,
		NowPlaying: Rect{X: 0, Y: 0, W: width, H: headerH},
		Footer:     Rect{X: 0, Y: footerY, W: width, H: footerH},
		StatusY:    footerY,
	}
	base.Library = Panel{ID: PanelLibrary, Y: bodyY, H: bodyH, Title: "Library", styles: styles}
	base.Tracks = Panel{ID: PanelTracks, Y: bodyY, H: bodyH, Title: "Tracks", styles: styles}
	base.Queue = Panel{ID: PanelQueue, Y: bodyY, H: bodyH, Title: "Queue", styles: styles}

	switch mode {
	case LayoutWide:
		available := width - 2 // two one-cell vertical separators
		left, right := wideSideWidths(available, cfg.LeftWidth, cfg.RightWidth)
		center := available - left - right
		base.Library.X, base.Library.W = 0, left
		base.Tracks.X, base.Tracks.W = left+1, center
		base.Queue.X, base.Queue.W = left+1+center+1, right
	case LayoutMedium:
		left := clamp(cfg.LeftWidth, 18, width-33)
		main := width - left - 1
		base.Library.X, base.Library.W = 0, left
		base.Tracks.X, base.Tracks.W = left+1, main
		base.Queue.X, base.Queue.W = left+1, main
	case LayoutCompact:
		base.Library.W = width
		base.Tracks.W = width
		base.Queue.W = width
	}

	return base
}

func layoutMode(width int) LayoutMode {
	switch {
	case width >= 110:
		return LayoutWide
	case width >= 72:
		return LayoutMedium
	default:
		return LayoutCompact
	}
}

func wideSideWidths(available, preferredLeft, preferredRight int) (int, int) {
	const centerMin = 32
	left := clamp(preferredLeft, 18, 32)
	right := clamp(preferredRight, 18, 32)
	excess := left + right + centerMin - available
	for excess > 0 && (left > 18 || right > 18) {
		if left >= right && left > 18 {
			left--
		} else if right > 18 {
			right--
		}
		excess--
	}
	return left, right
}

func clamp(value, minValue, maxValue int) int {
	if maxValue < minValue {
		maxValue = minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func tooSmallMessage(width, height int, cfg UIConfig) string {
	return fmt.Sprintf(
		"galdr needs at least %dx%d — you have %dx%d",
		cfg.MinWidth,
		cfg.MinHeight,
		width,
		height,
	)
}

func verticalDivider(styles theme.Palette, height int) string {
	if height <= 0 {
		return ""
	}
	lines := make([]string, height)
	for i := range lines {
		lines[i] = styles.Divider.Render("│")
	}
	return strings.Join(lines, "\n")
}

func fitLines(block string, width, height int) []string {
	if height <= 0 {
		return []string{}
	}
	input := strings.Split(block, "\n")
	if block == "" {
		input = []string{}
	}
	lines := make([]string, height)
	for i := range height {
		if i < len(input) {
			lines[i] = fitLine(input[i], width)
		} else {
			lines[i] = strings.Repeat(" ", max(0, width))
		}
	}
	return lines
}

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	line = cellTruncate(line, width)
	missing := width - lipgloss.Width(line)
	if missing > 0 {
		line += strings.Repeat(" ", missing)
	}
	return line
}
