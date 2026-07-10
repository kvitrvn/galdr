package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

// newTestModelWithTree mirrors app.newTestAppWithTree so the TUI
// tests can build a real on-disk library with a chosen
// Artist -> Album -> files structure.
func newTestModelWithTree(t *testing.T, structure map[string]map[string][]string) *Model {
	t.Helper()
	dir := t.TempDir()
	for artist, albums := range structure {
		for album, files := range albums {
			var albumDir string
			if album == "" {
				albumDir = filepath.Join(dir, artist)
			} else {
				albumDir = filepath.Join(dir, artist, album)
			}
			if err := os.MkdirAll(albumDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, f := range files {
				p := filepath.Join(albumDir, f)
				if err := os.WriteFile(p, []byte{}, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	cfg := config.Default()
	cfg.MusicDir = dir
	a := app.New(cfg, player.NewMock())
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}
	return New(a, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig())
}

func TestFocus_CycleForward(t *testing.T) {
	f := NewFocusManager()
	if got := f.Current(); got != PanelTracks {
		t.Fatalf("initial focus = %v, want PanelTracks", got)
	}
	f.Cycle()
	if got := f.Current(); got != PanelQueue {
		t.Errorf("after Cycle, focus = %v, want PanelQueue", got)
	}
	f.Cycle()
	if got := f.Current(); got != PanelLibrary {
		t.Errorf("after Cycle, focus = %v, want PanelLibrary (wrap)", got)
	}
	f.Cycle()
	if got := f.Current(); got != PanelTracks {
		t.Errorf("after Cycle, focus = %v, want PanelTracks (wrap)", got)
	}
}

func TestFocus_CycleBack(t *testing.T) {
	f := NewFocusManager()
	f.CycleBack()
	if got := f.Current(); got != PanelLibrary {
		t.Errorf("after CycleBack from Tracks, focus = %v, want PanelLibrary (wrap)", got)
	}
	f.CycleBack()
	if got := f.Current(); got != PanelQueue {
		t.Errorf("after CycleBack, focus = %v, want PanelQueue", got)
	}
}

func TestFocus_Set(t *testing.T) {
	f := NewFocusManager()
	f.Set(PanelLibrary)
	if got := f.Current(); got != PanelLibrary {
		t.Errorf("after Set(Library), focus = %v, want Library", got)
	}
	f.Set(PanelQueue)
	if got := f.Current(); got != PanelQueue {
		t.Errorf("after Set(Queue), focus = %v, want Queue", got)
	}
}

func TestTab_CyclesFocus(t *testing.T) {
	m := newTestModel(t, 1)
	if m.focus.Current() != PanelTracks {
		t.Fatalf("initial focus = %v, want PanelTracks", m.focus.Current())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus.Current() != PanelQueue {
		t.Errorf("after Tab, focus = %v, want PanelQueue", m.focus.Current())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus.Current() != PanelLibrary {
		t.Errorf("after second Tab, focus = %v, want PanelLibrary", m.focus.Current())
	}
}

func TestShiftTab_CyclesFocusBack(t *testing.T) {
	m := newTestModel(t, 1)
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus.Current() != PanelLibrary {
		t.Errorf("after S-Tab, focus = %v, want PanelLibrary", m.focus.Current())
	}
}

func TestLibraryPanel_RendersArtistsAndAlbums(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"Iron Maiden": {
			"Powerslave":        {"01.mp3", "02.mp3"},
			"Somewhere in Time": {"01.mp3"},
		},
		"Helloween": {
			"Keeper": {"01.mp3"},
		},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.libExpanded["Iron Maiden"] = true
	m.libExpanded["Helloween"] = true
	view := m.libraryPanelContent(22, 20)
	for _, want := range []string{"Helloween", "Iron Maiden", "Powerslave", "Somewhere in", "Keeper"} {
		if !strings.Contains(view, want) {
			t.Errorf("library view should contain %q, got: %q", want, view)
		}
	}
}

func TestLibraryPanel_TrackCountForAlbums(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3", "2.mp3", "3.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.libExpanded["X"] = true
	view := m.libraryPanelContent(22, 20)
	if !strings.Contains(view, "(3)") {
		t.Errorf("album track count '(3)' should appear, got: %q", view)
	}
}

func TestLibraryNav_DownMovesCursor(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3"}},
		"Z": {"W": {"1.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelLibrary)
	if m.libCursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.libCursor)
	}
	sendKey(t, m, "down")
	if m.libCursor != 1 {
		t.Errorf("after down, cursor = %d, want 1", m.libCursor)
	}
}

func TestLibraryNav_RightExpandsArtist(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelLibrary)
	if m.libExpanded["X"] {
		t.Fatal("X should not be expanded initially")
	}
	sendKey(t, m, "right")
	if !m.libExpanded["X"] {
		t.Error("after right on X, X should be expanded")
	}
}

func TestLibraryNav_LeftCollapsesOrClears(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.libExpanded["X"] = true
	m.libCursor = 0
	m.focus.Set(PanelLibrary)
	sendKey(t, m, "left")
	if m.libExpanded["X"] {
		t.Error("after left, X should be collapsed")
	}
}

func TestLibraryNav_EnterOnAlbumSetsScope(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3", "2.mp3"}, "Z": {"1.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.libExpanded["X"] = true
	// Cursor on Y (the first album under X).
	m.libCursor = 1
	m.focus.Set(PanelLibrary)
	sendKey(t, m, "enter")
	artist, album := m.app.Scope()
	if artist != "X" || album != "Y" {
		t.Errorf("Scope = (%q, %q), want (X, Y)", artist, album)
	}
	if m.focus.Current() != PanelTracks {
		t.Errorf("focus after enter on album = %v, want PanelTracks", m.focus.Current())
	}
}

func TestLibraryNav_EnterOnArtistNarrowsToArtist(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3", "2.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.libExpanded["X"] = true
	m.libCursor = 0 // X
	m.focus.Set(PanelLibrary)
	sendKey(t, m, "enter")
	artist, album := m.app.Scope()
	if artist != "X" || album != "" {
		t.Errorf("Scope = (%q, %q), want (X, \"\")", artist, album)
	}
	if m.focus.Current() != PanelTracks {
		t.Errorf("focus = %v, want Tracks", m.focus.Current())
	}
}

func TestTracksPanel_RendersScopedTracks(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"y1.mp3", "y2.mp3"}, "Z": {"z1.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.app.SetScope("X", "Y")
	view := m.tracksPanelContent(60, 10)
	// The tracks panel shows the track title (the filename
	// without extension, as produced by TitleFromPath in the
	// scanner). Y's tracks render as "y1" and "y2".
	if !strings.Contains(view, "y1") {
		t.Errorf("tracks panel should contain y1, got: %q", view)
	}
	if !strings.Contains(view, "y2") {
		t.Errorf("tracks panel should contain y2, got: %q", view)
	}
	if strings.Contains(view, "z1") {
		t.Errorf("tracks panel should NOT contain z1 (different scope), got: %q", view)
	}
}

func TestTracksPanel_ScopedNavMovesWithinScope(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3", "2.mp3", "3.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.app.SetScope("X", "Y")
	// Default focus is Tracks.
	if m.focus.Current() != PanelTracks {
		t.Fatalf("initial focus = %v, want PanelTracks", m.focus.Current())
	}
	if got := m.app.ScopedIndex(); got != 0 {
		t.Fatalf("initial ScopedIndex = %d, want 0", got)
	}
	sendKey(t, m, "down")
	if got := m.app.ScopedIndex(); got != 1 {
		t.Errorf("ScopedIndex after down = %d, want 1", got)
	}
}

func TestStatusBar_ShowsScope(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"X": {"Y": {"1.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.app.SetScope("X", "Y")
	view := m.statusView(120)
	if !strings.Contains(view, "Scope: X/Y") {
		t.Errorf("status bar should show scope, got: %q", view)
	}
}
