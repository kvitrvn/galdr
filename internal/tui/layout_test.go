package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/kvitrvn/galdr/internal/theme"
)

func TestCompute_ResponsiveGeometry(t *testing.T) {
	t.Parallel()
	styles := theme.PaletteFor(theme.ModeAuto)
	cfg := DefaultUIConfig()
	tests := []struct {
		name                 string
		width, height        int
		mode                 LayoutMode
		headerH, bodyH       int
		libraryW, tracksW    int
		queueW, separatorSum int
	}{
		{name: "wide 140x40", width: 140, height: 40, mode: LayoutWide, headerH: 5, bodyH: 33, libraryW: 22, tracksW: 94, queueW: 22, separatorSum: 2},
		{name: "medium 90x24", width: 90, height: 24, mode: LayoutMedium, headerH: 5, bodyH: 17, libraryW: 22, tracksW: 67, queueW: 67, separatorSum: 1},
		{name: "compact 60x18", width: 60, height: 18, mode: LayoutCompact, headerH: 3, bodyH: 13, libraryW: 60, tracksW: 60, queueW: 60},
		{name: "minimum 48x14", width: 48, height: 14, mode: LayoutCompact, headerH: 3, bodyH: 9, libraryW: 48, tracksW: 48, queueW: 48},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Compute(tt.width, tt.height, cfg, styles)
			if got.TooSmall {
				t.Fatalf("Compute() unexpectedly TooSmall: %s", got.TooSmallMsg)
			}
			if got.Mode != tt.mode {
				t.Errorf("Mode = %s, want %s", got.Mode, tt.mode)
			}
			if got.NowPlaying.H != tt.headerH || got.Library.H != tt.bodyH {
				t.Errorf("heights = header %d body %d, want %d/%d", got.NowPlaying.H, got.Library.H, tt.headerH, tt.bodyH)
			}
			if got.Footer.Y != tt.height-2 || got.Footer.H != 2 {
				t.Errorf("Footer = %+v, want y=%d h=2", got.Footer, tt.height-2)
			}
			if got.Library.W != tt.libraryW || got.Tracks.W != tt.tracksW || got.Queue.W != tt.queueW {
				t.Errorf("panel widths = %d/%d/%d, want %d/%d/%d", got.Library.W, got.Tracks.W, got.Queue.W, tt.libraryW, tt.tracksW, tt.queueW)
			}
			if tt.mode == LayoutWide {
				if got.Library.W+got.Tracks.W+got.Queue.W+tt.separatorSum != tt.width {
					t.Error("wide panels and separators do not cover the terminal width")
				}
				if got.Tracks.X != got.Library.W+1 || got.Queue.X != got.Tracks.X+got.Tracks.W+1 {
					t.Errorf("wide panel positions overlap: lib=%+v tracks=%+v queue=%+v", got.Library, got.Tracks, got.Queue)
				}
			}
			if tt.mode == LayoutMedium && (got.Tracks.X != got.Queue.X || got.Tracks.W != got.Queue.W) {
				t.Error("medium Tracks and Queue should share the main rectangle")
			}
		})
	}
}

func TestCompute_BreakpointsAndMinimum(t *testing.T) {
	t.Parallel()
	styles := theme.PaletteFor(theme.ModeAuto)
	cfg := DefaultUIConfig()
	for _, tt := range []struct {
		width int
		mode  LayoutMode
	}{
		{width: 48, mode: LayoutCompact},
		{width: 71, mode: LayoutCompact},
		{width: 72, mode: LayoutMedium},
		{width: 109, mode: LayoutMedium},
		{width: 110, mode: LayoutWide},
	} {
		got := Compute(tt.width, 24, cfg, styles)
		if got.Mode != tt.mode {
			t.Errorf("width %d: mode = %s, want %s", tt.width, got.Mode, tt.mode)
		}
	}
	for _, size := range [][2]int{{47, 14}, {48, 13}, {0, 0}} {
		got := Compute(size[0], size[1], cfg, styles)
		if !got.TooSmall || !strings.Contains(got.TooSmallMsg, "48x14") {
			t.Errorf("Compute(%d,%d) = %+v, want 48x14 warning", size[0], size[1], got)
		}
	}
}

func TestCompute_WidePreferredWidthsAreBounded(t *testing.T) {
	t.Parallel()
	cfg := DefaultUIConfig()
	cfg.LeftWidth = 80
	cfg.RightWidth = 90
	got := Compute(110, 30, cfg, theme.PaletteFor(theme.ModeAuto))
	if got.Tracks.W < 32 {
		t.Errorf("Tracks.W = %d, want at least 32", got.Tracks.W)
	}
	if got.Library.W+got.Tracks.W+got.Queue.W+2 != 110 {
		t.Error("bounded geometry does not cover width exactly")
	}
}

func TestPanelView_HasExactSizeAndTextualFocus(t *testing.T) {
	t.Parallel()
	p := Panel{
		W:       30,
		H:       7,
		Title:   "Tracks  2",
		Focused: true,
		styles:  theme.PaletteFor(theme.ModeAuto),
		Content: func(int, int) string { return "one\ntwo" },
	}
	view := p.View()
	lines := strings.Split(view, "\n")
	if len(lines) != 7 {
		t.Fatalf("lines = %d, want 7", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 30 {
			t.Errorf("line %d width = %d, want 30", i, got)
		}
	}
	if !strings.Contains(view, "● Tracks  2") || !strings.Contains(view, "──") {
		t.Errorf("panel lacks focus label or separator: %q", view)
	}
}

func TestLayout_DefaultUIConfig(t *testing.T) {
	t.Parallel()
	got := DefaultUIConfig()
	want := UIConfig{LeftWidth: 22, RightWidth: 22, MinWidth: 48, MinHeight: 14}
	if got != want {
		t.Errorf("DefaultUIConfig() = %+v, want %+v", got, want)
	}
}

func TestLayoutMode_String(t *testing.T) {
	t.Parallel()
	if LayoutWide.String() != "wide" || LayoutMedium.String() != "medium" || LayoutCompact.String() != "compact" {
		t.Error("unexpected LayoutMode labels")
	}
	if PanelID(99).String() != "unknown" {
		t.Error("unknown PanelID should render as unknown")
	}
}
