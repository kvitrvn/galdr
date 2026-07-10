// Package theme provides theme-adaptive terminal styles.
package theme

import "github.com/charmbracelet/lipgloss"

type Mode string

const (
	ModeAuto  Mode = "auto"
	ModeLight Mode = "light"
	ModeDark  Mode = "dark"
)

// Palette keeps presentation tokens in one place. Several legacy names are
// retained because they remain useful semantic aliases in the TUI.
type Palette struct {
	Title         lipgloss.Style
	SelectedRow   lipgloss.Style
	Row           lipgloss.Style
	PlayingRow    lipgloss.Style
	EmptyMsg      lipgloss.Style
	StatusBar     lipgloss.Style
	StatusKey     lipgloss.Style
	StatusVal     lipgloss.Style
	ErrorMsg      lipgloss.Style
	Divider       lipgloss.Style
	HelpHeader    lipgloss.Style
	HelpEntry     lipgloss.Style
	FocusedBorder lipgloss.Style
	DimBorder     lipgloss.Style
	PanelTitle    lipgloss.Style
	FocusedTitle  lipgloss.Style
	NowPlaying    lipgloss.Style
	State         lipgloss.Style
	Metadata      lipgloss.Style
	SearchBar     lipgloss.Style
	Footer        lipgloss.Style
	TooSmallMsg   lipgloss.Style
}

func PaletteFor(mode Mode) Palette {
	switch mode {
	case ModeLight:
		return makePalette(colorSet{
			fg:      lipgloss.Color("#242424"),
			muted:   lipgloss.Color("#666666"),
			subtle:  lipgloss.Color("#A3A3A3"),
			accent:  lipgloss.Color("#0E7490"),
			selectB: lipgloss.Color("#DCEFF2"),
			footerB: lipgloss.Color("#F0F0F0"),
			error:   lipgloss.Color("#B42318"),
		})
	case ModeDark:
		return makePalette(colorSet{
			fg:      lipgloss.Color("#E7E7E7"),
			muted:   lipgloss.Color("#A0A0A0"),
			subtle:  lipgloss.Color("#555555"),
			accent:  lipgloss.Color("#67B7C7"),
			selectB: lipgloss.Color("#17383E"),
			footerB: lipgloss.Color("#242424"),
			error:   lipgloss.Color("#FF8A80"),
		})
	default:
		return makePalette(colorSet{
			fg:      lipgloss.AdaptiveColor{Light: "#242424", Dark: "#E7E7E7"},
			muted:   lipgloss.AdaptiveColor{Light: "#666666", Dark: "#A0A0A0"},
			subtle:  lipgloss.AdaptiveColor{Light: "#A3A3A3", Dark: "#555555"},
			accent:  lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#67B7C7"},
			selectB: lipgloss.AdaptiveColor{Light: "#DCEFF2", Dark: "#17383E"},
			footerB: lipgloss.AdaptiveColor{Light: "#F0F0F0", Dark: "#242424"},
			error:   lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF8A80"},
		})
	}
}

type colorSet struct {
	fg      lipgloss.TerminalColor
	muted   lipgloss.TerminalColor
	subtle  lipgloss.TerminalColor
	accent  lipgloss.TerminalColor
	selectB lipgloss.TerminalColor
	footerB lipgloss.TerminalColor
	error   lipgloss.TerminalColor
}

func makePalette(c colorSet) Palette {
	row := lipgloss.NewStyle().Foreground(c.fg)
	muted := lipgloss.NewStyle().Foreground(c.muted)
	accent := lipgloss.NewStyle().Foreground(c.accent)
	footer := row.Background(c.footerB)

	return Palette{
		Title:         accent.Bold(true),
		SelectedRow:   row.Bold(true).Background(c.selectB),
		Row:           row,
		PlayingRow:    accent.Bold(true),
		EmptyMsg:      muted.Italic(true),
		StatusBar:     footer,
		StatusKey:     muted.Bold(true),
		StatusVal:     row,
		ErrorMsg:      lipgloss.NewStyle().Foreground(c.error).Bold(true),
		Divider:       lipgloss.NewStyle().Foreground(c.subtle),
		HelpHeader:    accent.Bold(true),
		HelpEntry:     row,
		FocusedBorder: lipgloss.NewStyle().Foreground(c.accent),
		DimBorder:     lipgloss.NewStyle().Foreground(c.subtle),
		PanelTitle:    muted.Bold(true),
		FocusedTitle:  accent.Bold(true),
		NowPlaying:    row.Bold(true),
		State:         accent.Bold(true),
		Metadata:      muted,
		SearchBar:     row.Background(c.footerB).Bold(true),
		Footer:        footer,
		TooSmallMsg:   lipgloss.NewStyle().Foreground(c.error).Bold(true),
	}
}
