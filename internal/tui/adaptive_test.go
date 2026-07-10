package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

func modelWithMock(t *testing.T) (*Model, *player.MockPlayer) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.MusicDir = dir
	mock := player.NewMock()
	a := app.New(cfg, mock)
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatal(err)
	}
	return New(a, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig()), mock
}

func TestNowPlaying_StatesMetadataAndControls(t *testing.T) {
	m, mock := modelWithMock(t)

	stopped := ansi.Strip(m.nowPlayingView(90, 5))
	if !strings.Contains(stopped, "Stopped") || !strings.Contains(stopped, "Nothing playing") {
		t.Errorf("stopped header = %q", stopped)
	}

	mock.SetDuration(3*time.Minute + 42*time.Second)
	sendKey(t, m, "enter")
	mock.SetPosition(75 * time.Second)
	m.app.ToggleShuffle()
	m.app.CycleRepeat()
	m.app.ToggleMute()
	playing := ansi.Strip(m.nowPlayingView(90, 5))
	for _, want := range []string{"Playing", "track", "1:15 / 3:42", "Mute on", "Shuffle on", "Repeat all"} {
		if !strings.Contains(playing, want) {
			t.Errorf("playing header missing %q: %q", want, playing)
		}
	}
	if tracks := ansi.Strip(m.listViewSized(48, 5)); !strings.Contains(tracks, "3:42") {
		t.Errorf("Tracks panel did not learn loaded duration: %q", tracks)
	}

	sendKey(t, m, " ")
	if paused := ansi.Strip(m.nowPlayingView(90, 5)); !strings.Contains(paused, "Paused") {
		t.Errorf("paused header = %q", paused)
	}
	sendKey(t, m, "x")
	if stoppedAgain := ansi.Strip(m.nowPlayingView(90, 5)); !strings.Contains(stoppedAgain, "Stopped") {
		t.Errorf("stopped header after x = %q", stoppedAgain)
	}
}

func TestNowPlaying_UnknownDurationAndUnicodeCellWidth(t *testing.T) {
	m, _ := modelWithMock(t)
	sendKey(t, m, "enter")
	view := m.nowPlayingView(48, 3)
	if !strings.Contains(view, "--:--") {
		t.Errorf("unknown duration missing: %q", view)
	}
	for row, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got != 48 {
			t.Errorf("row %d width = %d, want 48", row, got)
		}
	}
}

func TestTrackColumns_ProgressivelyHideSecondaryMetadata(t *testing.T) {
	m := newTestModel(t, 1)
	track := library.Track{
		Title:    "Track title",
		Artist:   "Artist name",
		Album:    "Album name",
		Duration: 2*time.Minute + 5*time.Second,
	}

	m.width = 140
	wide := ansi.Strip(m.renderRow(
		track,
		1,
		false,
		false,
		90,
	))
	if !strings.Contains(wide, "Artist name") || !strings.Contains(wide, "Album n") || !strings.HasSuffix(wide, "2:05") {
		t.Errorf("wide row = %q", wide)
	}
	m.width = 90
	medium := ansi.Strip(m.renderRow(
		track,
		1,
		false,
		false,
		60,
	))
	if !strings.Contains(medium, "Artist name") || strings.Contains(medium, "Album name") {
		t.Errorf("medium row = %q", medium)
	}
	m.width = 60
	compact := ansi.Strip(m.renderRow(
		track,
		1,
		false,
		false,
		48,
	))
	if strings.Contains(compact, "Artist name") || strings.Contains(compact, "Album name") {
		t.Errorf("compact row retained secondary metadata: %q", compact)
	}
}

func TestModel_DirectPanelsStopAndContextualHL(t *testing.T) {
	m := newTestModelWithTree(t, map[string]map[string][]string{
		"Artist": {"Album": {"track.mp3"}},
	})
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})

	sendKey(t, m, "1")
	if m.focus.Current() != PanelLibrary {
		t.Fatal("1 did not focus Library")
	}
	sendKey(t, m, "l")
	if !m.libExpanded["Artist"] {
		t.Error("l did not expand the Library artist")
	}
	sendKey(t, m, "h")
	if m.libExpanded["Artist"] {
		t.Error("h did not collapse the Library artist")
	}

	sendKey(t, m, "2")
	if m.focus.Current() != PanelTracks {
		t.Fatal("2 did not focus Tracks")
	}
	sendKey(t, m, "enter")
	if m.app.State() != player.StatePlaying {
		t.Fatal("enter did not start playback")
	}
	sendKey(t, m, "h")
	sendKey(t, m, "l")
	sendKey(t, m, "x")
	if m.app.State() != player.StateStopped || m.app.Current() != nil {
		t.Error("x did not stop and clear the current track")
	}

	sendKey(t, m, "3")
	if m.focus.Current() != PanelQueue {
		t.Fatal("3 did not focus Queue")
	}
}

func TestEmptyStates_AreActionable(t *testing.T) {
	m := newTestModel(t, 0)
	if got := ansi.Strip(m.listViewSized(48, 5)); !strings.Contains(got, "music_dir") || !strings.Contains(got, "press r") {
		t.Errorf("library empty state = %q", got)
	}
	if got := ansi.Strip(m.queuePanelContent(48, 5)); !strings.Contains(got, "Play a track") {
		t.Errorf("queue empty state = %q", got)
	}

	filtered := newTestModelWithTitles(t, []string{"Anthem"})
	filtered.app.SetFilter("missing")
	if got := ansi.Strip(filtered.listViewSized(48, 5)); !strings.Contains(got, "Press Esc") {
		t.Errorf("filter empty state = %q", got)
	}
}
