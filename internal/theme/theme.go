// Package theme provides theme-adaptive terminal styles.
package theme

import (
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
)

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
			selectF: lipgloss.Color("#242424"),
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
			selectF: lipgloss.Color("#E7E7E7"),
			selectB: lipgloss.Color("#17383E"),
			footerB: lipgloss.Color("#242424"),
			error:   lipgloss.Color("#FF8A80"),
		})
	default:
		return makeAutoPalette()
	}
}

type colorSet struct {
	fg      lipgloss.TerminalColor
	muted   lipgloss.TerminalColor
	subtle  lipgloss.TerminalColor
	accent  lipgloss.TerminalColor
	selectF lipgloss.TerminalColor
	selectB lipgloss.TerminalColor
	footerB lipgloss.TerminalColor
	error   lipgloss.TerminalColor
}

func makeAutoPalette() Palette {
	if colors, ok := loadOmarchyColors(); ok {
		return makePalette(colors)
	}

	row := lipgloss.NewStyle()
	muted := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))
	accent := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(4))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(1)).Bold(true)
	subtle := lipgloss.NewStyle().Foreground(lipgloss.ANSIColor(8))
	footer := row.Reverse(true)

	return Palette{
		Title:         accent.Bold(true),
		SelectedRow:   row.Bold(true).Reverse(true),
		Row:           row,
		PlayingRow:    accent.Bold(true),
		EmptyMsg:      muted.Italic(true),
		StatusBar:     footer,
		StatusKey:     muted.Bold(true),
		StatusVal:     row,
		ErrorMsg:      errorStyle,
		Divider:       subtle,
		HelpHeader:    accent.Bold(true),
		HelpEntry:     row,
		FocusedBorder: accent,
		DimBorder:     subtle,
		PanelTitle:    muted.Bold(true),
		FocusedTitle:  accent.Bold(true),
		NowPlaying:    row.Bold(true),
		State:         accent.Bold(true),
		Metadata:      muted,
		SearchBar:     row.Reverse(true).Bold(true),
		Footer:        footer,
		TooSmallMsg:   errorStyle,
	}
}

type omarchyTheme struct {
	Accent              string `toml:"accent"`
	Background          string `toml:"background"`
	Foreground          string `toml:"foreground"`
	SelectionForeground string `toml:"selection_foreground"`
	SelectionBackground string `toml:"selection_background"`
	Color0              string `toml:"color0"`
	Color1              string `toml:"color1"`
	Color7              string `toml:"color7"`
	Color8              string `toml:"color8"`
}

func loadOmarchyColors() (colorSet, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return colorSet{}, false
	}

	path := filepath.Join(home, ".config", "omarchy", "current", "theme", "colors.toml")
	var colors omarchyTheme
	if _, err := toml.DecodeFile(path, &colors); err != nil {
		return colorSet{}, false
	}

	values := []string{
		colors.Accent,
		colors.Background,
		colors.Foreground,
		colors.SelectionForeground,
		colors.SelectionBackground,
		colors.Color0,
		colors.Color1,
		colors.Color7,
		colors.Color8,
	}
	for _, value := range values {
		if !isHexColor(value) {
			return colorSet{}, false
		}
	}

	return colorSet{
		fg:      lipgloss.Color(colors.Foreground),
		muted:   lipgloss.Color(colors.Color7),
		subtle:  lipgloss.Color(colors.Color8),
		accent:  lipgloss.Color(colors.Accent),
		selectF: lipgloss.Color(selectionForeground(colors)),
		selectB: lipgloss.Color(colors.SelectionBackground),
		footerB: lipgloss.Color(colors.Color0),
		error:   lipgloss.Color(colors.Color1),
	}, true
}

func selectionForeground(colors omarchyTheme) string {
	const minimumContrast = 4.5

	best := colors.SelectionForeground
	bestContrast := contrastRatio(best, colors.SelectionBackground)
	if bestContrast >= minimumContrast {
		return best
	}

	for _, candidate := range []string{colors.Foreground, colors.Background, colors.Color0} {
		if contrast := contrastRatio(candidate, colors.SelectionBackground); contrast > bestContrast {
			best = candidate
			bestContrast = contrast
		}
	}
	return best
}

func contrastRatio(first, second string) float64 {
	firstLuminance := relativeLuminance(first)
	secondLuminance := relativeLuminance(second)
	lighter := math.Max(firstLuminance, secondLuminance)
	darker := math.Min(firstLuminance, secondLuminance)
	return (lighter + 0.05) / (darker + 0.05)
}

func relativeLuminance(color string) float64 {
	value, _ := strconv.ParseUint(color[1:], 16, 24)
	red := linearColorComponent(uint8(value >> 16))
	green := linearColorComponent(uint8(value >> 8))
	blue := linearColorComponent(uint8(value))
	return 0.2126*red + 0.7152*green + 0.0722*blue
}

func linearColorComponent(component uint8) float64 {
	value := float64(component) / 255
	if value <= 0.04045 {
		return value / 12.92
	}
	return math.Pow((value+0.055)/1.055, 2.4)
}

func isHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(value[1:], 16, 24)
	return err == nil
}

func makePalette(c colorSet) Palette {
	row := lipgloss.NewStyle().Foreground(c.fg)
	muted := lipgloss.NewStyle().Foreground(c.muted)
	accent := lipgloss.NewStyle().Foreground(c.accent)
	footer := row.Background(c.footerB)

	return Palette{
		Title:         accent.Bold(true),
		SelectedRow:   row.Foreground(c.selectF).Bold(true).Background(c.selectB),
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
