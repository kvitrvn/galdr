package theme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestPaletteFor_AutoUsesTerminalPalette(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := PaletteFor(ModeAuto)

	assertStylesHaveColor(t, lipgloss.ANSIColor(4), map[string]lipgloss.Style{
		"Title":         p.Title,
		"PlayingRow":    p.PlayingRow,
		"HelpHeader":    p.HelpHeader,
		"FocusedBorder": p.FocusedBorder,
		"FocusedTitle":  p.FocusedTitle,
		"State":         p.State,
	})
	assertStylesHaveColor(t, lipgloss.ANSIColor(8), map[string]lipgloss.Style{
		"EmptyMsg":   p.EmptyMsg,
		"StatusKey":  p.StatusKey,
		"Divider":    p.Divider,
		"DimBorder":  p.DimBorder,
		"PanelTitle": p.PanelTitle,
		"Metadata":   p.Metadata,
	})
	assertStylesHaveColor(t, lipgloss.ANSIColor(1), map[string]lipgloss.Style{
		"ErrorMsg":    p.ErrorMsg,
		"TooSmallMsg": p.TooSmallMsg,
	})
	assertStylesUseTerminalDefault(t, map[string]lipgloss.Style{
		"Row":        p.Row,
		"StatusVal":  p.StatusVal,
		"HelpEntry":  p.HelpEntry,
		"NowPlaying": p.NowPlaying,
	})
	assertStylesAreReversed(t, map[string]lipgloss.Style{
		"SelectedRow": p.SelectedRow,
		"StatusBar":   p.StatusBar,
		"SearchBar":   p.SearchBar,
		"Footer":      p.Footer,
	})
}

func TestPaletteFor_AutoUsesOmarchyTheme(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeOmarchyTheme(t, home, `
accent = "#7aa2f7"
background = "#1a1b26"
foreground = "#a9b1d6"
selection_foreground = "#c0caf5"
selection_background = "#7aa2f7"
color0 = "#32344a"
color1 = "#f7768e"
color7 = "#787c99"
color8 = "#444b6a"
`)

	p := PaletteFor(ModeAuto)
	assertColor(t, "Row foreground", p.Row.GetForeground(), lipgloss.Color("#a9b1d6"))
	assertColor(t, "Title foreground", p.Title.GetForeground(), lipgloss.Color("#7aa2f7"))
	assertColor(t, "Metadata foreground", p.Metadata.GetForeground(), lipgloss.Color("#787c99"))
	assertColor(t, "Divider foreground", p.Divider.GetForeground(), lipgloss.Color("#444b6a"))
	assertColor(t, "Error foreground", p.ErrorMsg.GetForeground(), lipgloss.Color("#f7768e"))
	assertColor(
		t,
		"SelectedRow foreground",
		p.SelectedRow.GetForeground(),
		lipgloss.Color("#1a1b26"),
	)
	assertColor(
		t,
		"SelectedRow background",
		p.SelectedRow.GetBackground(),
		lipgloss.Color("#7aa2f7"),
	)
	for name, style := range map[string]lipgloss.Style{
		"StatusBar": p.StatusBar,
		"SearchBar": p.SearchBar,
		"Footer":    p.Footer,
	} {
		assertColor(t, name+" background", style.GetBackground(), lipgloss.Color("#32344a"))
		if style.GetReverse() {
			t.Errorf("%s unexpectedly uses reverse video with an Omarchy palette", name)
		}
	}
}

func TestSelectionForeground_PreservesReadableOmarchyColor(t *testing.T) {
	colors := omarchyTheme{
		Background:          "#ffffff",
		Foreground:          "#222222",
		SelectionForeground: "#ffffff",
		SelectionBackground: "#222222",
		Color0:              "#000000",
	}

	if got := selectionForeground(colors); got != colors.SelectionForeground {
		t.Errorf("selectionForeground() = %q, want native readable color %q", got, colors.SelectionForeground)
	}
}

func TestContrastRatio_TokyoNightSelection(t *testing.T) {
	const minimumContrast = 4.5

	got := contrastRatio("#1a1b26", "#7aa2f7")
	if got < minimumContrast {
		t.Errorf("Tokyo Night corrected selection contrast = %.2f, want at least %.1f", got, minimumContrast)
	}
}

func TestPaletteFor_InvalidOmarchyThemeFallsBackToTerminal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeOmarchyTheme(t, home, `accent = "not-a-color"`)

	p := PaletteFor(ModeAuto)
	assertColor(t, "Title foreground", p.Title.GetForeground(), lipgloss.ANSIColor(4))
	if !p.SelectedRow.GetReverse() {
		t.Error("terminal fallback selected row does not use reverse video")
	}
}

func TestPaletteFor_UnknownFallsBackToAuto(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	unknown := paletteStyles(PaletteFor(Mode("rainbow")))
	auto := paletteStyles(PaletteFor(ModeAuto))

	for name, unknownStyle := range unknown {
		autoStyle := auto[name]
		if unknownStyle.Render("payload") != autoStyle.Render("payload") ||
			unknownStyle.GetForeground() != autoStyle.GetForeground() ||
			unknownStyle.GetBackground() != autoStyle.GetBackground() ||
			unknownStyle.GetReverse() != autoStyle.GetReverse() {
			t.Errorf("PaletteFor(unknown).%s differs from PaletteFor(auto).%s", name, name)
		}
	}
}

func TestPaletteFor_FixedModesRemainDistinct(t *testing.T) {
	light := PaletteFor(ModeLight)
	dark := PaletteFor(ModeDark)

	if light.Title.GetForeground() == dark.Title.GetForeground() {
		t.Error("light and dark palettes should differ on Title foreground")
	}
	if light.Row.GetForeground() == dark.Row.GetForeground() {
		t.Error("light and dark palettes should differ on Row foreground")
	}
	if isNoColor(light.SelectedRow.GetBackground()) || isNoColor(dark.SelectedRow.GetBackground()) {
		t.Error("fixed palettes should retain selected-row background colors")
	}
	if light.SelectedRow.GetReverse() || dark.SelectedRow.GetReverse() {
		t.Error("fixed palettes should not use reverse video for selected rows")
	}
}

func TestPalette_RendersAcrossColorProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	previousProfile := lipgloss.ColorProfile()
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	profiles := []struct {
		name    string
		profile termenv.Profile
	}{
		{name: "truecolor", profile: termenv.TrueColor},
		{name: "ansi", profile: termenv.ANSI},
		{name: "no_color", profile: termenv.Ascii},
	}
	modes := []Mode{ModeAuto, ModeLight, ModeDark}

	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			lipgloss.SetColorProfile(profile.profile)
			for _, mode := range modes {
				t.Run(string(mode), func(t *testing.T) {
					for name, style := range paletteStyles(PaletteFor(mode)) {
						out := style.Render("payload")
						if !strings.Contains(out, "payload") {
							t.Errorf("%s render of %s lost payload: %q", profile.name, name, out)
						}
					}
				})
			}
		})
	}
}

func assertStylesHaveColor(
	t *testing.T,
	want lipgloss.TerminalColor,
	styles map[string]lipgloss.Style,
) {
	t.Helper()
	for name, style := range styles {
		if got := style.GetForeground(); got != want {
			t.Errorf("%s foreground = %v (%T), want %v (%T)", name, got, got, want, want)
		}
		if !isNoColor(style.GetBackground()) {
			t.Errorf("%s has a hard-coded background: %v", name, style.GetBackground())
		}
		if style.GetReverse() {
			t.Errorf("%s unexpectedly uses reverse video", name)
		}
	}
}

func assertStylesUseTerminalDefault(t *testing.T, styles map[string]lipgloss.Style) {
	t.Helper()
	for name, style := range styles {
		if !isNoColor(style.GetForeground()) {
			t.Errorf("%s foreground = %v, want terminal default", name, style.GetForeground())
		}
		if !isNoColor(style.GetBackground()) {
			t.Errorf("%s background = %v, want terminal default", name, style.GetBackground())
		}
		if style.GetReverse() {
			t.Errorf("%s unexpectedly uses reverse video", name)
		}
	}
}

func assertStylesAreReversed(t *testing.T, styles map[string]lipgloss.Style) {
	t.Helper()
	for name, style := range styles {
		if !style.GetReverse() {
			t.Errorf("%s does not use reverse video", name)
		}
		if !isNoColor(style.GetForeground()) || !isNoColor(style.GetBackground()) {
			t.Errorf(
				"%s uses explicit colors: foreground=%v background=%v",
				name,
				style.GetForeground(),
				style.GetBackground(),
			)
		}
	}
}

func isNoColor(color lipgloss.TerminalColor) bool {
	_, ok := color.(lipgloss.NoColor)
	return ok
}

func assertColor(
	t *testing.T,
	name string,
	got lipgloss.TerminalColor,
	want lipgloss.TerminalColor,
) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v (%T), want %v (%T)", name, got, got, want, want)
	}
}

func writeOmarchyTheme(t *testing.T, home, contents string) {
	t.Helper()
	dir := filepath.Join(home, ".config", "omarchy", "current", "theme")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create Omarchy theme directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "colors.toml"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write Omarchy theme: %v", err)
	}
}

func paletteStyles(p Palette) map[string]lipgloss.Style {
	return map[string]lipgloss.Style{
		"Title":         p.Title,
		"SelectedRow":   p.SelectedRow,
		"Row":           p.Row,
		"PlayingRow":    p.PlayingRow,
		"EmptyMsg":      p.EmptyMsg,
		"StatusBar":     p.StatusBar,
		"StatusKey":     p.StatusKey,
		"StatusVal":     p.StatusVal,
		"ErrorMsg":      p.ErrorMsg,
		"Divider":       p.Divider,
		"HelpHeader":    p.HelpHeader,
		"HelpEntry":     p.HelpEntry,
		"FocusedBorder": p.FocusedBorder,
		"DimBorder":     p.DimBorder,
		"PanelTitle":    p.PanelTitle,
		"FocusedTitle":  p.FocusedTitle,
		"NowPlaying":    p.NowPlaying,
		"State":         p.State,
		"Metadata":      p.Metadata,
		"SearchBar":     p.SearchBar,
		"Footer":        p.Footer,
		"TooSmallMsg":   p.TooSmallMsg,
	}
}
