package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

func writeModelPlaylist(t *testing.T, m *Model, name string, entries ...string) {
	t.Helper()
	dir := filepath.Join(m.app.Config().MusicDir, "Playlists")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	contents := "#EXTM3U\n" + strings.Join(entries, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".m3u8"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newHierarchicalTestModel(t *testing.T) *Model {
	t.Helper()
	dir := t.TempDir()
	for _, path := range []string{
		"Artist/First/01.mp3",
		"Artist/First/02.mp3",
		"Artist/Second/03.mp3",
		"Other/Only/04.mp3",
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.MusicDir = dir
	a := app.New(cfg, player.NewMock())
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatal(err)
	}
	return New(a, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig(), nil)
}

func TestPlaylistSourcesCycleWithoutChangingFocus(t *testing.T) {
	m := newTestModel(t, 2)
	m.setFocus(PanelQueue)

	sendKey(t, m, "]")
	if m.source != PlaylistSource || m.focus.Current() != PanelQueue {
		t.Fatalf("after ] source/focus = %v/%v", m.source, m.focus.Current())
	}
	sendKey(t, m, "]")
	if m.source != LibrarySource || m.focus.Current() != PanelQueue {
		t.Fatalf("after wrapped ] source/focus = %v/%v", m.source, m.focus.Current())
	}
	sendKey(t, m, "[")
	if m.source != PlaylistSource || m.focus.Current() != PanelQueue {
		t.Fatalf("after [ source/focus = %v/%v", m.source, m.focus.Current())
	}

	sendKey(t, m, "P")
	if m.source != PlaylistSource || m.focus.Current() != PanelLibrary {
		t.Fatalf("after P source/focus = %v/%v", m.source, m.focus.Current())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if m.focus.Current() != PanelTracks {
		t.Fatalf("Tab focus = %v", m.focus.Current())
	}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.focus.Current() != PanelLibrary {
		t.Fatalf("Shift+Tab focus = %v", m.focus.Current())
	}
	sendKey(t, m, "3")
	if m.focus.Current() != PanelQueue || m.source != PlaylistSource {
		t.Fatalf("3 changed source/focus = %v/%v", m.source, m.focus.Current())
	}
}

func TestPlaylistLeftNavigationPreviewsWithoutLoading(t *testing.T) {
	m := newTestModel(t, 3)
	writeModelPlaylist(t, m, "alpha", "../t00.mp3")
	writeModelPlaylist(t, m, "beta", "../t01.mp3", "../t02.mp3")
	sendKey(t, m, "P")

	sendKey(t, m, "j")
	if m.playlistPreview.Name != "beta" || m.app.Queue().Len() != 0 {
		t.Fatalf("preview selection/queue = %q/%d", m.playlistPreview.Name, m.app.Queue().Len())
	}
	sendKey(t, m, "enter")
	if got := m.app.Queue().Tracks(); len(got) != 2 || m.app.State() != player.StateStopped {
		t.Fatalf("explicit load queue/state = %v/%v", got, m.app.State())
	}
}

func TestPlaylistCentreActsOnSelectedOccurrence(t *testing.T) {
	m := newTestModel(t, 2)
	writeModelPlaylist(t, m, "loop", "../t00.mp3", "../t01.mp3", "../t00.mp3")
	sendKey(t, m, "P")
	sendKey(t, m, "2")
	sendKey(t, m, "j")
	sendKey(t, m, "j")
	sendKey(t, m, "enter")

	if m.app.Queue().Index() != 2 || m.queueCursor != 2 {
		t.Fatalf("played duplicate occurrence at %d, queue cursor %d", m.app.Queue().Index(), m.queueCursor)
	}
	if got := baseNames(m.app.Queue().Tracks()); !slices.Equal(got, []string{"t00.mp3", "t01.mp3", "t00.mp3"}) {
		t.Fatalf("playlist playback queue = %v", got)
	}
	sendKey(t, m, "a")
	if got := baseNames(m.app.Queue().Tracks()); got[len(got)-1] != "t00.mp3" {
		t.Fatalf("a appended wrong occurrence: %v", got)
	}
	sendKey(t, m, "N")
	got := baseNames(m.app.Queue().Tracks())
	if len(got) != 5 || got[3] != "t00.mp3" {
		t.Fatalf("N inserted wrong occurrence: %v", got)
	}
}

func TestPlaylistCentreRemovesSelectedOccurrenceWithoutChangingQueue(t *testing.T) {
	m := newTestModel(t, 2)
	writeModelPlaylist(t, m, "loop", "../t00.mp3", "../t01.mp3", "../t00.mp3")
	sendKey(t, m, "P")
	sendKey(t, m, "2")
	sendKey(t, m, "j")
	sendKey(t, m, "j")
	sendKey(t, m, "enter")
	m.app.SetShuffle(true)
	wantQueue := m.app.Queue().Entries()
	wantIndex := m.app.Queue().Index()
	wantCurrent := m.app.Current().Path
	wantState := m.app.State()

	sendKey(t, m, "d")
	if got := baseNames(m.playlistPreview.Tracks); !slices.Equal(got, []string{"t00.mp3", "t01.mp3"}) {
		t.Fatalf("playlist after duplicate removal = %v", got)
	}
	if m.previewCursor != 1 {
		t.Fatalf("cursor after removing last occurrence = %d, want 1", m.previewCursor)
	}
	if !slices.Equal(m.app.Queue().Entries(), wantQueue) || m.app.Queue().Index() != wantIndex ||
		m.app.Current().Path != wantCurrent || m.app.State() != wantState || !m.app.Shuffle() {
		t.Fatal("playlist removal changed queue, playback or shuffle")
	}
	if m.app.Status() != "Removed t00 from loop" {
		t.Fatalf("removal status = %q", m.app.Status())
	}

	sendKey(t, m, "d")
	if got := baseNames(m.playlistPreview.Tracks); !slices.Equal(got, []string{"t00.mp3"}) ||
		m.previewCursor != 0 {
		t.Fatalf("second removal tracks/cursor = %v/%d", got, m.previewCursor)
	}
}

func TestPlaylistRemoveShortcutOnlyActsInPlaylistCentre(t *testing.T) {
	m := newTestModel(t, 2)
	writeModelPlaylist(t, m, "mix", "../t00.mp3", "../t01.mp3")
	sendKey(t, m, "P")
	sendKey(t, m, "d")
	if len(m.playlistPreview.Tracks) != 2 {
		t.Fatal("d removed a track while the playlist list was focused")
	}
	sendKey(t, m, "2")
	if shortcuts := ansi.Strip(m.shortcutLine(220)); !strings.Contains(shortcuts, "d remove") {
		t.Fatalf("playlist centre shortcuts missing removal: %q", shortcuts)
	}
	if help := ansi.Strip(m.helpViewSized(140, 42)); !strings.Contains(help, "remove selected playlist occurrence") {
		t.Fatalf("general help missing playlist removal: %q", help)
	}
}

func TestPlaylistCentreRemovalRefreshesAfterExternalEdit(t *testing.T) {
	m := newTestModel(t, 2)
	writeModelPlaylist(t, m, "temporary", "../t00.mp3", "../t01.mp3")
	sendKey(t, m, "P")
	sendKey(t, m, "2")
	sendKey(t, m, "j")
	writeModelPlaylist(t, m, "temporary", "../t00.mp3")

	sendKey(t, m, "d")
	if got := baseNames(m.playlistPreview.Tracks); !slices.Equal(got, []string{"t00.mp3"}) ||
		m.previewCursor != 0 {
		t.Fatalf("stale removal refresh = %v/%d", got, m.previewCursor)
	}
	footer := ansi.Strip(m.footerMessage(120))
	if !strings.Contains(footer, "Track is no longer in the playlist") ||
		strings.Contains(footer, "playlist: track occurrence not found") {
		t.Fatalf("stale removal footer is not localized: %q", footer)
	}
}

func TestPlaylistSavePromptSupportsUnicodeValidationAndOverwrite(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter")
	sendKey(t, m, "P")
	sendKey(t, m, "S")
	for _, r := range "été mix" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	if m.playlistPrompt != playlistPromptClosed || m.playlistPreview.Name != "été mix" {
		t.Fatalf("unicode save prompt/selection = %v/%q", m.playlistPrompt, m.playlistPreview.Name)
	}

	sendKey(t, m, "S")
	for _, r := range " invalid" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	if m.playlistPrompt != playlistPromptName || m.playlistInput.Value() != " invalid" || m.playlistSaveError == "" {
		t.Fatalf("invalid name state = %v/%q/%q", m.playlistPrompt, m.playlistInput.Value(), m.playlistSaveError)
	}
	sendKey(t, m, "esc")
	if m.playlistPrompt != playlistPromptClosed {
		t.Fatalf("Esc did not close save prompt: %v", m.playlistPrompt)
	}

	sendKey(t, m, "S")
	for _, r := range "été mix" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	if m.playlistPrompt != playlistPromptOverwrite {
		t.Fatalf("collision prompt = %v", m.playlistPrompt)
	}
	sendKey(t, m, "n")
	if m.playlistPrompt != playlistPromptClosed {
		t.Fatalf("overwrite refusal prompt = %v", m.playlistPrompt)
	}
	sendKey(t, m, "S")
	for _, r := range "été mix" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	sendKey(t, m, "y")
	if m.playlistPrompt != playlistPromptClosed || m.playlistPreview.Name != "été mix" {
		t.Fatalf("overwrite success = %v/%q", m.playlistPrompt, m.playlistPreview.Name)
	}
}

func TestPlaylistSaveOnEmptyQueueReportsErrorWithoutPrompt(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "P")
	sendKey(t, m, "S")
	if m.playlistPrompt != playlistPromptClosed {
		t.Fatalf("empty queue opened prompt: %v", m.playlistPrompt)
	}
	if !strings.Contains(m.app.Status(), "Queue is empty") {
		t.Fatalf("empty queue status = %q", m.app.Status())
	}
	if view := ansi.Strip(m.View()); !strings.Contains(view, "Queue is empty") || strings.Contains(view, "cannot save") {
		t.Fatalf("empty queue footer is not localized: %q", view)
	}
}

func TestPlaylistTabsAndPreviewRenderAcrossWidths(t *testing.T) {
	m := newTestModel(t, 2)
	writeModelPlaylist(t, m, "a very long playlist name", "../t00.mp3", "../missing.mp3")
	sendKey(t, m, "P")

	m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	wide := ansi.Strip(m.View())
	for _, want := range []string{"● Library  [Playlists]", "Playlist: a very long playlist name", "1 entry ignored"} {
		if !strings.Contains(wide, want) {
			t.Errorf("wide playlist view missing %q", want)
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	if medium := ansi.Strip(m.View()); !strings.Contains(medium, "Playlist: a very long playlist name") {
		t.Errorf("medium playlist view lost context: %q", medium)
	}
	m.tr = i18n.New(i18n.French)
	if compactTitle := m.sourceTitle(18); !strings.Contains(compactTitle, "2/2") || !strings.Contains(compactTitle, "Playlists") {
		t.Fatalf("compact localized source title = %q", compactTitle)
	}
}

func TestDirectPlaylistAddFromLibraryRestoresContextWithoutChangingQueue(t *testing.T) {
	m := newTestModel(t, 3)
	writeModelPlaylist(t, m, "alpha", "../t00.mp3")
	writeModelPlaylist(t, m, "beta", "../t01.mp3")
	sendKey(t, m, "enter")
	sendKey(t, m, "j")
	m.app.SetShuffle(true)
	wantQueue := m.app.Queue().Entries()
	wantIndex := m.app.Queue().Index()
	wantState := m.app.State()
	wantSelected := m.app.Selected().Path
	wantLibCursor := m.libCursor
	wantQueueCursor := m.queueCursor
	wantMedium := m.mediumMain

	sendKey(t, m, "A")
	if m.playlistAdd.phase != playlistAddDestination || m.source != PlaylistSource ||
		m.focus.Current() != PanelLibrary || m.playlistAdd.cursor != 0 {
		t.Fatalf("direct add entry state = %#v, source/focus %v/%v", m.playlistAdd, m.source, m.focus.Current())
	}
	sendKey(t, m, "j")
	sendKey(t, m, "enter")

	if m.playlistAdd.phase != playlistAddClosed || m.source != LibrarySource || m.focus.Current() != PanelTracks {
		t.Fatalf("restored add state/source/focus = %v/%v/%v", m.playlistAdd.phase, m.source, m.focus.Current())
	}
	if m.libCursor != wantLibCursor || m.queueCursor != wantQueueCursor || m.mediumMain != wantMedium {
		t.Fatalf("restored cursors/main = %d/%d/%v", m.libCursor, m.queueCursor, m.mediumMain)
	}
	if !slices.Equal(m.app.Queue().Entries(), wantQueue) || m.app.Queue().Index() != wantIndex ||
		m.app.State() != wantState || !m.app.Shuffle() || m.app.Selected().Path != wantSelected {
		t.Fatal("direct add changed queue, playback, shuffle or Library selection")
	}
	preview, err := m.app.PreviewPlaylist("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if got := baseNames(preview.Tracks); !slices.Equal(got, []string{"t00.mp3", "t01.mp3"}) {
		t.Fatalf("destination tracks = %v", got)
	}
}

func TestDirectPlaylistAddFromPreviewKeepsOriginalOccurrence(t *testing.T) {
	m := newTestModel(t, 3)
	writeModelPlaylist(t, m, "origin", "../t00.mp3", "../t01.mp3", "../t00.mp3")
	writeModelPlaylist(t, m, "target", "../t02.mp3")
	sendKey(t, m, "P")
	sendKey(t, m, "2")
	sendKey(t, m, "j")
	if m.previewCursor != 1 {
		t.Fatal("setup did not select the second occurrence")
	}

	sendKey(t, m, "A")
	sendKey(t, m, "j")
	sendKey(t, m, "enter")

	if m.source != PlaylistSource || m.focus.Current() != PanelTracks ||
		m.playlistPreview.Name != "origin" || m.previewCursor != 1 {
		t.Fatalf("restored preview = %v/%v/%q/%d", m.source, m.focus.Current(), m.playlistPreview.Name, m.previewCursor)
	}
	if got := baseNames(m.playlistPreview.Tracks); !slices.Equal(got, []string{"t00.mp3", "t01.mp3", "t00.mp3", "t01.mp3"}) {
		t.Fatalf("refreshed origin occurrences = %v", got)
	}
	if m.app.Queue().Len() != 0 {
		t.Fatalf("preview add changed queue length to %d", m.app.Queue().Len())
	}
}

func TestDirectPlaylistAddIsUnavailableFromQueue(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter")
	sendKey(t, m, "3")
	sendKey(t, m, "A")
	if m.playlistAdd.phase != playlistAddClosed || m.source != LibrarySource || m.focus.Current() != PanelQueue {
		t.Fatalf("queue opened direct add: %v/%v/%v", m.playlistAdd.phase, m.source, m.focus.Current())
	}
}

func TestDirectPlaylistAddCancellationRestoresExactContext(t *testing.T) {
	m := newTestModel(t, 3)
	writeModelPlaylist(t, m, "origin", "../t00.mp3", "../t01.mp3")
	sendKey(t, m, "P")
	sendKey(t, m, "2")
	sendKey(t, m, "j")
	m.mediumMain = PanelQueue
	m.libCursor = 4
	m.queueCursor = 6
	wantNames := append([]string(nil), m.playlistNames...)
	wantPreview := clonePlaylistPreview(m.playlistPreview)
	wantPlaylistCursor := m.playlistCursor
	wantPreviewCursor := m.previewCursor

	sendKey(t, m, "A")
	sendKey(t, m, "enter")
	for _, r := range "temp" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "esc")
	if m.playlistAdd.phase != playlistAddDestination || m.playlistInput.Value() != "temp" {
		t.Fatalf("name-level Esc state/value = %v/%q", m.playlistAdd.phase, m.playlistInput.Value())
	}
	sendKey(t, m, "esc")

	if m.source != PlaylistSource || m.focus.Current() != PanelTracks || m.mediumMain != PanelQueue ||
		m.libCursor != 4 || m.queueCursor != 6 || m.playlistCursor != wantPlaylistCursor ||
		m.previewCursor != wantPreviewCursor || !slices.Equal(m.playlistNames, wantNames) ||
		m.playlistPreview.Name != wantPreview.Name || !slices.Equal(m.playlistPreview.Tracks, wantPreview.Tracks) {
		t.Fatalf("cancel did not restore exact context: %#v", m)
	}
	if _, err := os.Stat(filepath.Join(m.app.Config().MusicDir, "Playlists", "temp.m3u8")); !os.IsNotExist(err) {
		t.Fatalf("cancel created a playlist: %v", err)
	}
}

func TestDirectPlaylistAddCreationValidationAndRefresh(t *testing.T) {
	m := newTestModel(t, 1)
	writeModelPlaylist(t, m, "alpha", "../t00.mp3")
	sendKey(t, m, "A")
	sendKey(t, m, "enter")
	for _, r := range " invalid" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	if m.playlistAdd.phase != playlistAddName || m.playlistInput.Value() != " invalid" || m.playlistAdd.err == "" {
		t.Fatalf("invalid creation state = %v/%q/%q", m.playlistAdd.phase, m.playlistInput.Value(), m.playlistAdd.err)
	}
	sendKey(t, m, "esc")
	sendKey(t, m, "enter")
	for _, r := range "ALPHA" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	if m.playlistAdd.phase != playlistAddName || m.playlistAdd.err != "Playlist already exists; select it from the list" {
		t.Fatalf("existing creation state = %v/%q", m.playlistAdd.phase, m.playlistAdd.err)
	}
	preview, err := m.app.PreviewPlaylist("alpha")
	if err != nil || len(preview.Tracks) != 1 {
		t.Fatalf("existing-name attempt wrote a track: %#v, %v", preview, err)
	}

	sendKey(t, m, "esc")
	sendKey(t, m, "enter")
	for _, r := range "été" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	if m.playlistAdd.phase != playlistAddClosed || m.source != LibrarySource || m.focus.Current() != PanelTracks {
		t.Fatalf("creation did not restore context: %v/%v/%v", m.playlistAdd.phase, m.source, m.focus.Current())
	}
	created, err := m.app.PreviewPlaylist("été")
	if err != nil || !slices.Equal(baseNames(created.Tracks), []string{"t00.mp3"}) {
		t.Fatalf("created playlist = %#v, %v", created, err)
	}
}

func TestDirectPlaylistAddModeBlocksUnrelatedActions(t *testing.T) {
	m := newTestModel(t, 2)
	writeModelPlaylist(t, m, "alpha", "../t00.mp3")
	wantVolume := m.app.Volume()
	wantShuffle := m.app.Shuffle()
	sendKey(t, m, "A")
	sendKey(t, m, "3")
	sendKey(t, m, "s")
	sendKey(t, m, "-")
	sendKey(t, m, " ")
	if m.playlistAdd.phase != playlistAddDestination || m.focus.Current() != PanelLibrary ||
		m.app.Volume() != wantVolume || m.app.Shuffle() != wantShuffle || m.app.State() != player.StateStopped {
		t.Fatalf("unrelated action escaped destination mode: %#v", m.playlistAdd)
	}
}

func TestDirectPlaylistAddHandlesDisappearedDestination(t *testing.T) {
	m := newTestModel(t, 1)
	writeModelPlaylist(t, m, "temporary", "../t00.mp3")
	sendKey(t, m, "A")
	path := filepath.Join(m.app.Config().MusicDir, "Playlists", "temporary.m3u8")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	sendKey(t, m, "j")
	sendKey(t, m, "enter")
	if m.playlistAdd.phase != playlistAddDestination ||
		m.playlistAdd.err != "Playlist is no longer available" || len(m.playlistNames) != 0 {
		t.Fatalf("disappeared destination state = %v/%q/%v", m.playlistAdd.phase, m.playlistAdd.err, m.playlistNames)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("disappeared destination was recreated: %v", err)
	}
}

func TestDirectPlaylistAddRendersResponsiveLocalizedFlow(t *testing.T) {
	m := newTestModel(t, 1)
	writeModelPlaylist(t, m, "alpha", "../t00.mp3")
	m.tr = i18n.New(i18n.French)
	sendKey(t, m, "A")
	for _, size := range []tea.WindowSizeMsg{
		{Width: 140, Height: 24},
		{Width: 90, Height: 24},
		{Width: 60, Height: 18},
	} {
		m.Update(size)
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "＋ Nouvelle playlist") {
			t.Errorf("%dx%d add view missing new destination: %q", size.Width, size.Height, view)
		}
		if size.Width == 140 && !strings.Contains(view, "File d’attente") {
			t.Errorf("wide add view hid the queue: %q", view)
		}
	}
	sendKey(t, m, "enter")
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "Nouvelle playlist :") ||
		!strings.Contains(view, "Entrée créer et ajouter · Échap retour") {
		t.Fatalf("localized creation footer = %q", view)
	}
}

func TestDirectPlaylistAddShortcutAppearsInCentreAndGeneralHelp(t *testing.T) {
	m := newTestModel(t, 1)
	m.tr = i18n.New(i18n.French)
	m.setFocus(PanelTracks)
	if shortcuts := ansi.Strip(m.shortcutLine(220)); !strings.Contains(shortcuts, "A ajouter à une playlist") {
		t.Fatalf("centre shortcuts missing direct add: %q", shortcuts)
	}
	m.setFocus(PanelLibrary)
	if shortcuts := ansi.Strip(m.shortcutLine(220)); !strings.Contains(shortcuts, "A ajouter à une playlist") {
		t.Fatalf("Library shortcuts missing direct add: %q", shortcuts)
	}
	if help := ansi.Strip(m.helpViewSized(140, 40)); !strings.Contains(help, "ajouter une piste, un artiste ou un album") {
		t.Fatalf("general help missing direct add: %q", help)
	}
}

func TestDirectPlaylistAddFromArtistOrAlbum(t *testing.T) {
	tests := []struct {
		name       string
		selectRows []string
		want       []string
	}{
		{
			name: "artist",
			want: []string{"01.mp3", "02.mp3", "03.mp3"},
		},
		{
			name:       "album",
			selectRows: []string{"l", "j"},
			want:       []string{"01.mp3", "02.mp3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newHierarchicalTestModel(t)
			writeModelPlaylist(t, m, "target")
			sendKey(t, m, "enter")
			m.app.SetShuffle(true)
			wantQueue := m.app.Queue().Entries()
			wantCurrent := m.app.Current().Path
			sendKey(t, m, "1")
			for _, key := range tt.selectRows {
				sendKey(t, m, key)
			}
			wantCursor := m.libCursor

			sendKey(t, m, "A")
			if len(m.playlistAdd.tracks) != len(tt.want) {
				t.Fatalf("selected batch has %d tracks, want %d", len(m.playlistAdd.tracks), len(tt.want))
			}
			sendKey(t, m, "j")
			sendKey(t, m, "enter")

			if m.source != LibrarySource || m.focus.Current() != PanelLibrary || m.libCursor != wantCursor {
				t.Fatalf("restored Library context = %v/%v/%d", m.source, m.focus.Current(), m.libCursor)
			}
			preview, err := m.app.PreviewPlaylist("target")
			if err != nil {
				t.Fatal(err)
			}
			if got := baseNames(preview.Tracks); !slices.Equal(got, tt.want) {
				t.Fatalf("playlist batch = %v, want %v", got, tt.want)
			}
			if !slices.Equal(m.app.Queue().Entries(), wantQueue) || m.app.Current().Path != wantCurrent || !m.app.Shuffle() {
				t.Fatal("Library batch add changed queue, playback or shuffle")
			}
		})
	}
}

func TestDirectPlaylistAddCreatesPlaylistFromAlbum(t *testing.T) {
	m := newHierarchicalTestModel(t)
	sendKey(t, m, "1")
	sendKey(t, m, "l")
	sendKey(t, m, "j")
	sendKey(t, m, "j")
	if rows := m.libRows(); rows[m.libCursor].Album != "Second" {
		t.Fatalf("setup selected %q", rows[m.libCursor].Album)
	}
	sendKey(t, m, "A")
	sendKey(t, m, "enter")
	for _, r := range "second mix" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")

	preview, err := m.app.PreviewPlaylist("second mix")
	if err != nil {
		t.Fatal(err)
	}
	if got := baseNames(preview.Tracks); !slices.Equal(got, []string{"03.mp3"}) {
		t.Fatalf("created album playlist = %v", got)
	}
	if m.focus.Current() != PanelLibrary || m.libCursor != 2 {
		t.Fatalf("creation restored focus/cursor = %v/%d", m.focus.Current(), m.libCursor)
	}
}

func baseNames(tracks []library.Track) []string {
	names := make([]string, len(tracks))
	for i, track := range tracks {
		names[i] = filepath.Base(track.Path)
	}
	return names
}
