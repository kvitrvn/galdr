package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/kvitrvn/galdr/internal/theme"
)

func TestView_ResponsiveModesShowExpectedPanels(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		focus     PanelID
		want      []string
		doNotWant []string
	}{
		{name: "wide", width: 140, focus: PanelTracks, want: []string{"Library", "Tracks", "Queue"}},
		{name: "medium tracks", width: 90, focus: PanelTracks, want: []string{"Library", "Tracks"}, doNotWant: []string{"Queue  3"}},
		{name: "medium queue", width: 90, focus: PanelQueue, want: []string{"Library", "Queue"}, doNotWant: []string{"Tracks  3"}},
		{name: "compact library", width: 60, focus: PanelLibrary, want: []string{"Library"}, doNotWant: []string{"Tracks  3", "Queue  3"}},
		{name: "compact queue", width: 60, focus: PanelQueue, want: []string{"Queue"}, doNotWant: []string{"Library  1", "Tracks  3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t, 3)
			m.Update(tea.WindowSizeMsg{Width: tt.width, Height: 24})
			m.setFocus(tt.focus)
			view := ansi.Strip(m.View())
			for _, want := range tt.want {
				if !strings.Contains(view, want) {
					t.Errorf("view missing %q", want)
				}
			}
			for _, notWanted := range tt.doNotWant {
				if strings.Contains(view, notWanted) {
					t.Errorf("view unexpectedly contains %q", notWanted)
				}
			}
		})
	}
}

func TestView_ExactANSIDimensions(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	for _, size := range [][2]int{{140, 40}, {90, 24}, {60, 18}, {48, 14}} {
		m := newTestModel(t, 8)
		m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		lines := strings.Split(m.View(), "\n")
		if len(lines) != size[1] {
			t.Errorf("%dx%d: line count = %d", size[0], size[1], len(lines))
			continue
		}
		for row, line := range lines {
			if got := lipgloss.Width(line); got != size[0] {
				t.Errorf("%dx%d row %d: width = %d", size[0], size[1], row, got)
			}
		}
	}
}

func TestView_LightAndDarkThemesRemainReadableAndExact(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })

	for _, mode := range []theme.Mode{theme.ModeLight, theme.ModeDark} {
		m := newTestModel(t, 3)
		m.styles = theme.PaletteFor(mode)
		m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
		view := m.View()
		if !strings.Contains(ansi.Strip(view), "● Tracks") {
			t.Errorf("%s theme lost focus indicator", mode)
		}
		for row, line := range strings.Split(view, "\n") {
			if got := lipgloss.Width(line); got != 60 {
				t.Errorf("%s theme row %d width = %d, want 60", mode, row, got)
			}
		}
	}
}

func TestView_TooSmallTerminal(t *testing.T) {
	m := newTestModel(t, 1)
	m.Update(tea.WindowSizeMsg{Width: 47, Height: 14})
	view := m.View()
	if !strings.Contains(view, "galdr") || !strings.Contains(view, "48x14") {
		t.Errorf("unexpected too-small view: %q", view)
	}
}

func TestView_UsesSoberSeparatorsAndTextualFocus(t *testing.T) {
	m := newTestModel(t, 1)
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "● Tracks") || !strings.Contains(view, "· Library") {
		t.Errorf("focus is not expressed in text and symbol: %q", view)
	}
	if !strings.Contains(view, "│") || !strings.Contains(view, "─") {
		t.Errorf("view lacks section separators: %q", view)
	}
	for _, heavy := range []string{"┌", "┐", "└", "┘"} {
		if strings.Contains(view, heavy) {
			t.Errorf("view contains old box corner %q", heavy)
		}
	}
}

func TestView_SearchUsesDedicatedFooterBar(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	sendKey(t, m, "/")
	for _, r := range "limbo" {
		sendKey(t, m, string(r))
	}
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	footer := strings.Join(lines[len(lines)-2:], "\n")
	if !strings.Contains(footer, "/ limbo") || !strings.Contains(footer, "1/3 results") {
		t.Errorf("search footer = %q", footer)
	}
}

func TestView_MediumRemembersMainPanel(t *testing.T) {
	m := newTestModel(t, 3)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	m.setFocus(PanelQueue)
	m.setFocus(PanelLibrary)
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Queue  0") || strings.Contains(view, "Tracks  3") {
		t.Errorf("medium main panel was not remembered: %q", view)
	}
}

func TestView_CustomWideSideWidths(t *testing.T) {
	m := newTestModel(t, 1)
	m.uiCfg.LeftWidth = 30
	m.uiCfg.RightWidth = 28
	m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	firstBodyLine := strings.Split(ansi.Strip(m.View()), "\n")[5]
	runes := []rune(firstBodyLine)
	if runes[30] != '│' {
		t.Errorf("left separator = %q at cell 30, want │", runes[30])
	}
}

func TestHelp_ResponsiveAndClosesWithEscape(t *testing.T) {
	for _, width := range []int{140, 60} {
		m := newTestModel(t, 1)
		m.Update(tea.WindowSizeMsg{Width: width, Height: 18})
		sendKey(t, m, "?")
		view := m.View()
		if !strings.Contains(view, "Keybindings") || !strings.Contains(view, "stop") {
			t.Errorf("help at width %d is missing content", width)
		}
		if len(strings.Split(view, "\n")) != 18 {
			t.Errorf("help at width %d does not fit height", width)
		}
		sendKey(t, m, "esc")
		if m.help {
			t.Errorf("Esc did not close help at width %d", width)
		}
	}
}
