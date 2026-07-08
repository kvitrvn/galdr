package tui

import (
	"strings"
	"testing"

	"github.com/kvitrvn/galdr/internal/theme"
)

func TestLayout_ThreePanel_DefaultWidths(t *testing.T) {
	styles := theme.PaletteFor(theme.ModeAuto)
	cfg := DefaultUIConfig()

	cases := []struct {
		name           string
		width, height  int
		wantLib, wantQ int
		wantCenter     int
		wantStatusY    int
	}{
		{"80x24 minimum", 80, 24, 22, 22, 36, 23},
		{"120x40 typical", 120, 40, 22, 22, 76, 39},
		{"200x50 large", 200, 50, 22, 22, 156, 49},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			layout := Compute(c.width, c.height, cfg, styles)
			if layout.TooSmall {
				t.Fatalf("Compute(%d, %d) = TooSmall, want normal", c.width, c.height)
			}
			if got := layout.Library.W; got != c.wantLib {
				t.Errorf("Library.W = %d, want %d", got, c.wantLib)
			}
			if got := layout.Queue.W; got != c.wantQ {
				t.Errorf("Queue.W = %d, want %d", got, c.wantQ)
			}
			if got := layout.Tracks.W; got != c.wantCenter {
				t.Errorf("Tracks.W = %d, want %d", got, c.wantCenter)
			}
			if got := layout.StatusY; got != c.wantStatusY {
				t.Errorf("StatusY = %d, want %d", got, c.wantStatusY)
			}
			// All panels span the same Y range and have the same height.
			if layout.Library.H != c.height-1 || layout.Tracks.H != c.height-1 || layout.Queue.H != c.height-1 {
				t.Errorf("panel heights = %d/%d/%d, want all %d",
					layout.Library.H, layout.Tracks.H, layout.Queue.H, c.height-1)
			}
			// Panels are side by side, with no gaps and no overlap.
			if layout.Library.X != 0 {
				t.Errorf("Library.X = %d, want 0", layout.Library.X)
			}
			if layout.Tracks.X != layout.Library.W {
				t.Errorf("Tracks.X = %d, want %d (just after Library)",
					layout.Tracks.X, layout.Library.W)
			}
			if layout.Queue.X != layout.Library.W+layout.Tracks.W {
				t.Errorf("Queue.X = %d, want %d (just after Tracks)",
					layout.Queue.X, layout.Library.W+layout.Tracks.W)
			}
			if layout.Library.W+layout.Tracks.W+layout.Queue.W != c.width {
				t.Errorf("sum of panel widths = %d, want %d",
					layout.Library.W+layout.Tracks.W+layout.Queue.W, c.width)
			}
		})
	}
}

func TestLayout_Resize(t *testing.T) {
	styles := theme.PaletteFor(theme.ModeAuto)
	cfg := DefaultUIConfig()

	// Start wide.
	wide := Compute(200, 50, cfg, styles)
	if wide.TooSmall {
		t.Fatal("200x50 should not be TooSmall")
	}
	if wide.Tracks.W != 156 {
		t.Errorf("wide Tracks.W = %d, want 156", wide.Tracks.W)
	}

	// Shrink to minimum.
	small := Compute(80, 24, cfg, styles)
	if small.TooSmall {
		t.Fatal("80x24 should not be TooSmall")
	}
	if small.Tracks.W != 36 {
		t.Errorf("small Tracks.W = %d, want 36", small.Tracks.W)
	}
	if small.StatusY != 23 {
		t.Errorf("small StatusY = %d, want 23", small.StatusY)
	}

	// Then grow again.
	bigger := Compute(160, 60, cfg, styles)
	if bigger.TooSmall {
		t.Fatal("160x60 should not be TooSmall")
	}
	if bigger.Tracks.W != 116 {
		t.Errorf("bigger Tracks.W = %d, want 116", bigger.Tracks.W)
	}
	if bigger.StatusY != 59 {
		t.Errorf("bigger StatusY = %d, want 59", bigger.StatusY)
	}
}

func TestLayout_TooSmall_RendersWarning(t *testing.T) {
	styles := theme.PaletteFor(theme.ModeAuto)
	cfg := DefaultUIConfig()

	cases := []struct {
		name          string
		width, height int
	}{
		{"too narrow", 79, 40},
		{"too short", 120, 23},
		{"both too small", 40, 10},
		{"zero width", 0, 40},
		{"zero height", 120, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			layout := Compute(c.width, c.height, cfg, styles)
			if !layout.TooSmall {
				t.Fatalf("Compute(%d, %d) = not TooSmall, want TooSmall", c.width, c.height)
			}
			if !strings.Contains(layout.TooSmallMsg, "galdr") {
				t.Errorf("TooSmallMsg = %q, want to mention 'galdr'", layout.TooSmallMsg)
			}
			// Library/Tracks/Queue geometry should be zero so that
			// callers don't accidentally render them.
			if layout.Library.W != 0 || layout.Tracks.W != 0 || layout.Queue.W != 0 {
				t.Errorf("panels not zeroed on TooSmall: %+v", layout)
			}
		})
	}
}

func TestLayout_PanelView_RendersBorderAndTitle(t *testing.T) {
	styles := theme.PaletteFor(theme.ModeAuto)
	p := Panel{
		ID:      PanelTracks,
		X:       0,
		Y:       0,
		W:       40,
		H:       10,
		Title:   " Tracks ",
		Focused: false,
		styles:  styles,
		Content: func(w, h int) string {
			return "hello"
		},
	}
	view := p.View()
	if !strings.Contains(view, "Tracks") {
		t.Errorf("panel view should contain its title, got: %q", view)
	}
	if !strings.Contains(view, "hello") {
		t.Errorf("panel view should contain content, got: %q", view)
	}
	// Should contain box-drawing characters.
	if !strings.Contains(view, "┌") || !strings.Contains(view, "┐") ||
		!strings.Contains(view, "└") || !strings.Contains(view, "┘") {
		t.Errorf("panel view should have a box border, got: %q", view)
	}
	// t.Logf for visual inspection.
	t.Logf("view = %q", view)
	// 10 rows.
	if got := strings.Count(view, "\n") + 1; got != 10 {
		t.Errorf("panel view rows = %d, want 10 (got %d newlines)", got, strings.Count(view, "\n"))
	}
}

func TestLayout_PanelView_FocusedVsDim(t *testing.T) {
	styles := theme.PaletteFor(theme.ModeAuto)

	focused := Panel{
		W: 20, H: 5, Title: " F ", Focused: true, styles: styles,
		Content: func(int, int) string { return "" },
	}
	dim := Panel{
		W: 20, H: 5, Title: " D ", Focused: false, styles: styles,
		Content: func(int, int) string { return "" },
	}
	focusedView := focused.View()
	dimView := dim.View()
	if focusedView == dimView {
		t.Errorf("focused and dim panels should render differently, got equal output")
	}
}

func TestLayout_PanelID_String(t *testing.T) {
	cases := map[PanelID]string{
		PanelLibrary: "library",
		PanelTracks:  "tracks",
		PanelQueue:   "queue",
		PanelID(99):  "unknown",
	}
	for id, want := range cases {
		if got := id.String(); got != want {
			t.Errorf("PanelID(%d).String() = %q, want %q", int(id), got, want)
		}
	}
}

func TestLayout_DefaultUIConfig(t *testing.T) {
	cfg := DefaultUIConfig()
	if cfg.LeftWidth != 22 {
		t.Errorf("LeftWidth = %d, want 22", cfg.LeftWidth)
	}
	if cfg.RightWidth != 22 {
		t.Errorf("RightWidth = %d, want 22", cfg.RightWidth)
	}
	if cfg.MinWidth != 80 {
		t.Errorf("MinWidth = %d, want 80", cfg.MinWidth)
	}
	if cfg.MinHeight != 24 {
		t.Errorf("MinHeight = %d, want 24", cfg.MinHeight)
	}
}
