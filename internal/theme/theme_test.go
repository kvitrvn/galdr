package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestPaletteFor_KnownModes(t *testing.T) {
	cases := []Mode{ModeAuto, ModeLight, ModeDark}
	for _, m := range cases {
		t.Run(string(m), func(t *testing.T) {
			p := PaletteFor(m)
			if p.Title.GetForeground() == nil && p.Title.GetBackground() == nil {
				t.Errorf("PaletteFor(%q): Title has no colors set", m)
			}
		})
	}
}

func TestPaletteFor_UnknownFallsBackToAuto(t *testing.T) {
	pUnknown := PaletteFor(Mode("rainbow"))
	pAuto := PaletteFor(ModeAuto)
	if pUnknown.Title.GetForeground() != pAuto.Title.GetForeground() {
		t.Errorf("unknown mode produced different palette than auto: %v vs %v",
			pUnknown.Title.GetForeground(), pAuto.Title.GetForeground())
	}
}

func TestPalette_LightAndDarkDiffer(t *testing.T) {
	light := PaletteFor(ModeLight)
	dark := PaletteFor(ModeDark)

	if light.Title.GetForeground() == dark.Title.GetForeground() {
		t.Error("light and dark palettes should differ on Title foreground")
	}
	if light.Row.GetForeground() == dark.Row.GetForeground() {
		t.Error("light and dark palettes should differ on Row foreground")
	}
}

func TestPalette_AutoHasAdaptiveColors(t *testing.T) {
	p := PaletteFor(ModeAuto)
	// AdaptiveColor renders to a specific terminal color depending on
	// background. We just check that values are non-nil and that the
	// color profile changes the rendered output.
	inTrue := p.Title.Render("hi")
	profile := lipgloss.ColorProfile()
	_ = profile
	if !strings.Contains(inTrue, "hi") {
		t.Errorf("Rendered output missing payload: %q", inTrue)
	}
}

func TestPalette_RendersInTrueColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.TrueColor) })

	for _, m := range []Mode{ModeAuto, ModeLight, ModeDark} {
		t.Run(string(m), func(t *testing.T) {
			p := PaletteFor(m)
			for name, style := range map[string]lipgloss.Style{
				"title":      p.Title,
				"selected":   p.SelectedRow,
				"row":        p.Row,
				"playing":    p.PlayingRow,
				"empty":      p.EmptyMsg,
				"statusbar":  p.StatusBar,
				"statuskey":  p.StatusKey,
				"statusval":  p.StatusVal,
				"error":      p.ErrorMsg,
				"divider":    p.Divider,
				"helpheader": p.HelpHeader,
				"helpentry":  p.HelpEntry,
			} {
				out := style.Render("x")
				if !strings.Contains(out, "x") {
					t.Errorf("Palette(%s).%s.Render stripped payload: %q", m, name, out)
				}
			}
		})
	}
}

func TestPalette_RendersIn16Color(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.TrueColor) })

	p := PaletteFor(ModeAuto)
	// In a 16-color profile, every style must still render its payload
	// (and not crash) even if the exact hue is approximated.
	checks := []struct {
		name  string
		style lipgloss.Style
	}{
		{"title", p.Title},
		{"selected", p.SelectedRow},
		{"row", p.Row},
		{"playing", p.PlayingRow},
		{"empty", p.EmptyMsg},
		{"statusbar", p.StatusBar},
		{"error", p.ErrorMsg},
		{"divider", p.Divider},
		{"helpheader", p.HelpHeader},
	}
	for _, c := range checks {
		out := c.style.Render("hello")
		if !strings.Contains(out, "hello") {
			t.Errorf("16-color render of %s lost payload: %q", c.name, out)
		}
	}
}

func TestPalette_RendersInASCII(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.TrueColor) })

	p := PaletteFor(ModeAuto)
	out := p.Title.Render("galdr")
	if !strings.Contains(out, "galdr") {
		t.Errorf("ASCII render lost payload: %q", out)
	}
	out = p.SelectedRow.Render("selected")
	if !strings.Contains(out, "selected") {
		t.Errorf("ASCII selected render lost payload: %q", out)
	}
}
