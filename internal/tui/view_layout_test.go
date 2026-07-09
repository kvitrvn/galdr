package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

func TestView_ThreePanelLayout_RendersAllPanels(t *testing.T) {
	m := newTestModel(t, 3)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	view := m.View()
	for _, want := range []string{"Library", "Tracks", "Queue"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain panel title %q, got view: %q", want, view)
		}
	}
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) != 40 {
		t.Errorf("view has %d lines, want 40", len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "vol") {
		t.Errorf("last line should be the status bar, got: %q", lines[len(lines)-1])
	}
}

func TestView_TooSmallTerminal_RendersWarning(t *testing.T) {
	m := newTestModel(t, 1)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	view := m.View()
	if !strings.Contains(view, "galdr") {
		t.Errorf("too-small view should mention 'galdr', got: %q", view)
	}
	if !strings.Contains(view, "80x24") {
		t.Errorf("too-small view should mention the minimum size, got: %q", view)
	}
}

func TestView_Resize_ReflowsPanels(t *testing.T) {
	m := newTestModel(t, 1)

	// Start wide.
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	wide := m.View()
	wideLines := strings.Split(strings.TrimRight(wide, "\n"), "\n")
	if len(wideLines) != 50 {
		t.Errorf("wide view has %d lines, want 50", len(wideLines))
	}

	// Shrink.
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	small := m.View()
	smallLines := strings.Split(strings.TrimRight(small, "\n"), "\n")
	if len(smallLines) != 30 {
		t.Errorf("small view has %d lines, want 30", len(smallLines))
	}

	// Resize down to too-small.
	m.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
	tooSmall := m.View()
	if !strings.Contains(tooSmall, "galdr") || !strings.Contains(tooSmall, "80x24") {
		t.Errorf("after resize to 50x15, view should be the too-small message, got: %q", tooSmall)
	}
}

func TestView_PanelsHaveBoxDrawingBorders(t *testing.T) {
	m := newTestModel(t, 1)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	view := m.View()
	for _, want := range []string{"┌", "┐", "└", "┘", "─", "│"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain box-drawing rune %q, got: %q", want, view)
		}
	}
}

func TestView_SearchInputRendersInsideTracksPanel(t *testing.T) {
	m := newTestModel(t, 1)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	sendKey(t, m, "/")
	view := m.View()
	if !strings.Contains(view, "search") {
		t.Errorf("search-mode view should contain the search placeholder, got: %q", view)
	}
}

func TestView_FocusIndicator_VisibleInBorder(t *testing.T) {
	// Lip Gloss only emits color codes when a TTY color profile is
	// active. Without it, focused and dim borders render the same
	// text. Force a profile so we can detect the difference.
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	m := newTestModel(t, 1)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Default focus is PanelTracks.
	viewTracks := m.View()

	// Move focus to Library.
	m.focus.Set(PanelLibrary)
	viewLib := m.View()

	if viewTracks == viewLib {
		t.Error("Views with different focused panels should differ when colors are active")
	}
}

func TestView_RespectsCustomUIConfig(t *testing.T) {
	// Force a colour profile so the view's first line has a
	// deterministic width.
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	cfg := config.Default()
	// Custom narrow Library.
	cfg.UI.LeftWidth = 30
	cfg.UI.RightWidth = 30
	cfg.UI.MinWidth = 80
	cfg.UI.MinHeight = 24

	dir := t.TempDir()
	a := app.New(cfg, player.NewMock())
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}
	m := New(a, theme.PaletteFor(theme.ModeAuto), UIConfig{
		LeftWidth:  cfg.UI.LeftWidth,
		RightWidth: cfg.UI.RightWidth,
		MinWidth:   cfg.UI.MinWidth,
		MinHeight:  cfg.UI.MinHeight,
	})
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	view := m.View()
	lines := strings.Split(view, "\n")
	first := lines[0]
	// Visual width of the first row must be 200 (the requested
	// terminal width).
	if got := lipgloss.Width(first); got != 200 {
		t.Errorf("first line visual width = %d, want 200", got)
	}
	// The Library panel's right border should sit exactly at the
	// 30th visual cell.
	stripped := []rune(ansi.Strip(first))
	if len(stripped) < 32 {
		t.Fatalf("first line too short after stripping ANSI: %q", stripped)
	}
	if string(stripped[29]) != "┐" {
		t.Errorf("Library panel right border at visual col 29, got %q (expected ┐)", string(stripped[29]))
	}
	// The Tracks panel should start at col 30 with ┌.
	if string(stripped[30]) != "┌" {
		t.Errorf("Tracks panel left border at visual col 30, got %q (expected ┌)", string(stripped[30]))
	}
}
