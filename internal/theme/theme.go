// Package theme provides terminal styling primitives built on Lip Gloss.
//
// The goal is adaptive readability on both light and dark terminal themes
// and graceful degradation in basic 16-color terminals.
//
// Styles are exposed as a Palette value built by PaletteFor. The TUI
// requests a palette once at startup based on the user's configured
// theme (auto / light / dark) and re-uses it for every render.
package theme

import (
	"github.com/charmbracelet/lipgloss"
)

// Mode selects the style variant. It mirrors config.Theme without
// importing the config package to keep the theme package self-contained.
type Mode string

const (
	ModeAuto  Mode = "auto"
	ModeLight Mode = "light"
	ModeDark  Mode = "dark"
)

// Palette is a bundle of named styles used by the view.
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
	TooSmallMsg   lipgloss.Style
}

// PaletteFor returns the palette matching mode. Unknown modes fall back
// to ModeAuto, which uses Lip Gloss AdaptiveColor so that styles adapt
// to whatever background the terminal reports.
func PaletteFor(mode Mode) Palette {
	switch mode {
	case ModeLight:
		return lightPalette()
	case ModeDark:
		return darkPalette()
	default:
		return autoPalette()
	}
}

// autoPalette uses AdaptiveColor so that every element renders correctly
// on either a light or a dark terminal. This is the MVP default.
func autoPalette() Palette {
	dim := lipgloss.AdaptiveColor{Light: "#7A7A7A", Dark: "#969696"}
	accent := lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#9B95FF"}
	subtle := lipgloss.AdaptiveColor{Light: "#A0A0A0", Dark: "#606060"}
	err := lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#F28B82"}
	fg := lipgloss.AdaptiveColor{Light: "#1F1F1F", Dark: "#E6E6E6"}
	statusBg := lipgloss.AdaptiveColor{Light: "#EAEAEA", Dark: "#262626"}

	return Palette{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			Padding(0, 1),

		SelectedRow: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accent),

		Row: lipgloss.NewStyle().
			Foreground(fg),

		PlayingRow: lipgloss.NewStyle().
			Foreground(accent),

		EmptyMsg: lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true).
			Padding(1, 2),

		StatusBar: lipgloss.NewStyle().
			Foreground(fg).
			Background(statusBg).
			Padding(0, 1),

		StatusKey: lipgloss.NewStyle().
			Foreground(dim).
			Bold(true),

		StatusVal: lipgloss.NewStyle().
			Foreground(fg),

		ErrorMsg: lipgloss.NewStyle().
			Foreground(err).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(dim),

		HelpHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			MarginTop(1).
			MarginBottom(1),

		HelpEntry: lipgloss.NewStyle().
			Foreground(fg),

		FocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(accent),

		DimBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(subtle),

		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(accent).
			Padding(0, 1),

		TooSmallMsg: lipgloss.NewStyle().
			Foreground(err).
			Bold(true).
			Padding(2, 4),
	}
}

// lightPalette forces the contrast expected on a light background:
// dark text on a colored selection background, dark dividers, and a
// slightly tinted status bar.
func lightPalette() Palette {
	return Palette{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5A56E0")).
			Padding(0, 1),

		SelectedRow: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5A56E0")),

		Row: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1F1F1F")),

		PlayingRow: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5A56E0")),

		EmptyMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			Italic(true).
			Padding(1, 2),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1F1F1F")).
			Background(lipgloss.Color("#EAEAEA")).
			Padding(0, 1),

		StatusKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7A7A7A")).
			Bold(true),

		StatusVal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1F1F1F")),

		ErrorMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B3261E")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7A7A7A")),

		HelpHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5A56E0")).
			MarginTop(1).
			MarginBottom(1),

		HelpEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1F1F1F")),

		FocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#5A56E0")),

		DimBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#A0A0A0")),

		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#5A56E0")).
			Padding(0, 1),

		TooSmallMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#B3261E")).
			Bold(true).
			Padding(2, 4),
	}
}

// darkPalette forces the contrast expected on a dark background:
// light text on a brighter selection background, lighter dividers.
func darkPalette() Palette {
	return Palette{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9B95FF")).
			Padding(0, 1),

		SelectedRow: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7B61FF")),

		Row: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6E6E6")),

		PlayingRow: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9B95FF")),

		EmptyMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#606060")).
			Italic(true).
			Padding(1, 2),

		StatusBar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6E6E6")).
			Background(lipgloss.Color("#262626")).
			Padding(0, 1),

		StatusKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#969696")).
			Bold(true),

		StatusVal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6E6E6")),

		ErrorMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F28B82")).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#969696")),

		HelpHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9B95FF")).
			MarginTop(1).
			MarginBottom(1),

		HelpEntry: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E6E6E6")),

		FocusedBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#9B95FF")),

		DimBorder: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#606060")),

		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9B95FF")).
			Padding(0, 1),

		TooSmallMsg: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F28B82")).
			Bold(true).
			Padding(2, 4),
	}
}
