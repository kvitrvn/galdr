package app

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

func testApp(t *testing.T, paths ...string) (*App, *player.MockPlayer, string) {
	t.Helper()
	root := t.TempDir()
	for _, path := range paths {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.MusicDir = root
	mock := player.NewMock()
	a := New(cfg, mock)
	if err := a.LoadLibrary(root); err != nil {
		t.Fatal(err)
	}
	return a, mock, root
}

func paths(tracks []library.Track) []string {
	out := make([]string, len(tracks))
	for i := range tracks {
		out[i] = filepath.Base(tracks[i].Path)
	}
	return out
}

func TestLoadLibraryLeavesQueueEmptyAndTracksNavigable(t *testing.T) {
	a, _, _ := testApp(t, "Artist/Album/01.mp3", "Artist/Album/02.mp3")
	if a.Queue().Len() != 0 {
		t.Fatalf("queue length = %d, want 0", a.Queue().Len())
	}
	if a.Selected() == nil || a.ScopedIndex() != 0 {
		t.Fatal("Tracks selection was not initialised")
	}
	a.SelectNextScoped()
	if a.ScopedIndex() != 1 {
		t.Fatalf("selection = %d, want 1", a.ScopedIndex())
	}
}

func TestPlaySelectedBuildsContextualQueue(t *testing.T) {
	tests := []struct {
		name   string
		artist string
		album  string
		filter string
		want   []string
	}{
		{name: "global", want: []string{"01.mp3", "02.mp3", "03.mp3", "04.mp3"}},
		{name: "artist", artist: "A", want: []string{"01.mp3", "02.mp3", "03.mp3"}},
		{name: "album", artist: "A", album: "First", want: []string{"01.mp3", "02.mp3"}},
		{name: "filtered album", artist: "A", album: "First", filter: "02", want: []string{"02.mp3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, _, _ := testApp(t,
				"A/First/01.mp3", "A/First/02.mp3", "A/Second/03.mp3", "B/Only/04.mp3")
			a.SetScope(tt.artist, tt.album)
			a.SetFilter(tt.filter)
			if err := a.PlaySelected(); err != nil {
				t.Fatal(err)
			}
			if got := paths(a.Queue().Tracks()); !slices.Equal(got, tt.want) {
				t.Fatalf("queue = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlaySelectedKeepsArbitraryCanonicalPosition(t *testing.T) {
	a, _, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	a.SelectNextScoped()
	a.SelectNextScoped()
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	if a.Queue().Index() != 2 || filepath.Base(a.Current().Path) != "03.mp3" {
		t.Fatalf("queue index/current = %d/%v", a.Queue().Index(), a.Current())
	}
}

func TestPlayLearnsDurationForTracksAndQueue(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3")
	mock.SetDuration(4*time.Minute + 10*time.Second)

	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}

	want := 4*time.Minute + 10*time.Second
	if got := a.Selected().Duration; got != want {
		t.Fatalf("selected duration = %v, want %v", got, want)
	}
	if got := a.ScopedTracks()[0].Duration; got != want {
		t.Fatalf("Tracks duration = %v, want %v", got, want)
	}
	if got := a.Queue().Tracks()[0].Duration; got != want {
		t.Fatalf("Queue duration = %v, want %v", got, want)
	}
	if got := a.Current().Duration; got != want {
		t.Fatalf("current duration = %v, want %v", got, want)
	}

	a.ToggleShuffle()
	a.ToggleShuffle()
	if got := a.Queue().Tracks()[0].Duration; got != want {
		t.Fatalf("restored reference duration = %v, want %v", got, want)
	}
}

func TestScopeAndFilterChangesDoNotMutateActiveQueue(t *testing.T) {
	a, _, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "B/Y/03.mp3")
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	want := paths(a.Queue().Tracks())
	a.SetScope("B", "Y")
	a.SetFilter("does-not-match")
	if got := paths(a.Queue().Tracks()); !slices.Equal(got, want) {
		t.Fatalf("queue changed with Tracks context: %v, want %v", got, want)
	}
}

func TestPlayAtIndexDoesNotRebuildQueue(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	a.MoveQueueDown(0)
	want := paths(a.Queue().Tracks())
	if err := a.PlayAtIndex(2); err != nil {
		t.Fatal(err)
	}
	if got := paths(a.Queue().Tracks()); !slices.Equal(got, want) {
		t.Fatalf("PlayAtIndex rebuilt queue: %v, want %v", got, want)
	}
	if len(mock.LoadCalls) != 2 {
		t.Fatalf("Load calls = %d, want 2", len(mock.LoadCalls))
	}
}

func TestShuffleReordersWithoutReloadAndRestoresReference(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3", "A/X/04.mp3")
	a.SelectNextScoped()
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	a.random = rand.New(rand.NewPCG(1, 2))
	want := paths(a.Queue().Tracks())
	index := a.Queue().Index()
	current := a.Current().Path
	loads := len(mock.LoadCalls)
	if err := a.TogglePlay(); err != nil {
		t.Fatal(err)
	}
	if a.State() != player.StatePaused {
		t.Fatal("setup: player was not paused")
	}
	a.ToggleShuffle()
	if a.Queue().Index() != index || a.Current().Path != current || len(mock.LoadCalls) != loads || a.State() != player.StatePaused {
		t.Fatal("shuffle changed current track, index, state, or reloaded audio")
	}
	if got := paths(a.Queue().Tracks()); !slices.Equal(got, []string{"03.mp3", "02.mp3", "01.mp3", "04.mp3"}) {
		t.Fatalf("deterministic shuffle = %v", got)
	}
	firstShuffle := paths(a.Queue().Tracks())
	a.ToggleShuffle()
	if got := paths(a.Queue().Tracks()); !slices.Equal(got, want) {
		t.Fatalf("restored order = %v, want %v", got, want)
	}
	a.ToggleShuffle()
	if got := paths(a.Queue().Tracks()); slices.Equal(got, firstShuffle) {
		t.Fatalf("second activation reused shuffle order %v", got)
	}
	a.ToggleShuffle()
}

func TestQueueEditsSurviveShuffleToggles(t *testing.T) {
	a, _, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3", "A/X/04.mp3")
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	a.MoveQueueUp(3)
	if !a.RemoveFromQueue(1) {
		t.Fatal("remove non-playing track failed")
	}
	want := paths(a.Queue().Tracks())
	a.ToggleShuffle()
	a.ToggleShuffle()
	if got := paths(a.Queue().Tracks()); !slices.Equal(got, want) {
		t.Fatalf("manual order restored as %v, want %v", got, want)
	}
	if a.RemoveFromQueue(a.Queue().Index()) {
		t.Fatal("playing track was removable")
	}
	a.ClearQueue()
	if a.Queue().Len() != 1 || a.Queue().Current().Path != a.Current().Path {
		t.Fatal("Clear did not retain only the playing track")
	}
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	a.ClearQueue()
	if a.Queue().Len() != 0 {
		t.Fatal("Clear while stopped did not empty queue")
	}
}

func TestNavigationAndRepeatUseActiveOrder(t *testing.T) {
	a, _, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	a.MoveQueueDown(0)
	if err := a.PlayAtIndex(0); err != nil {
		t.Fatal(err)
	}
	first := a.Current().Path
	if err := a.Next(); err != nil {
		t.Fatal(err)
	}
	if a.Queue().Index() != 1 {
		t.Fatal("Next did not traverse active order")
	}
	if err := a.Previous(); err != nil || a.Current().Path != first {
		t.Fatal("Previous did not traverse active order")
	}
	if err := a.PlayAtIndex(a.Queue().Len() - 1); err != nil {
		t.Fatal(err)
	}
	if err := a.Next(); err != nil || a.State() != player.StateStopped || a.Current() != nil {
		t.Fatal("repeat off did not stop at the end of the active queue")
	}
	a.CycleRepeat()
	if err := a.PlayAtIndex(a.Queue().Len() - 1); err != nil {
		t.Fatal(err)
	}
	if err := a.Next(); err != nil || a.Queue().Index() != 0 {
		t.Fatal("repeat all did not wrap")
	}
	a.CycleRepeat()
	loads := len(a.player.(*player.MockPlayer).LoadCalls)
	a.player.(*player.MockPlayer).Stop()
	if err := a.MaybeAdvance(); err != nil || len(a.player.(*player.MockPlayer).LoadCalls) != loads+1 {
		t.Fatal("repeat one did not reload current track")
	}
}

func TestRescanPreservesSelectionAndQueueSnapshot(t *testing.T) {
	a, _, root := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	a.SelectNextScoped()
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	removed := a.Queue().Tracks()[2].Path
	if err := os.Remove(removed); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "A/X/04.mp3"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	selected := a.Selected().Path
	if err := a.Rescan(); err != nil {
		t.Fatal(err)
	}
	if a.Selected().Path != selected {
		t.Fatal("Tracks selection was not preserved")
	}
	if got := paths(a.Queue().Tracks()); !slices.Equal(got, []string{"01.mp3", "02.mp3"}) {
		t.Fatalf("rescanned queue = %v", got)
	}
	if len(a.ScopedTracks()) != 3 {
		t.Fatal("new catalogue track is missing")
	}
}

func TestRescanStopsWhenPlayingTrackDisappears(t *testing.T) {
	a, _, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3")
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(a.Current().Path); err != nil {
		t.Fatal(err)
	}
	if err := a.Rescan(); err != nil {
		t.Fatal(err)
	}
	if a.Current() != nil || a.State() != player.StateStopped {
		t.Fatal("removed playing track did not stop playback")
	}
}

func TestEmptyVisibleScopeDoesNotDisturbPlayback(t *testing.T) {
	a, _, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3")
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	want := paths(a.Queue().Tracks())
	current := a.Current().Path
	a.SetFilter("missing")
	if err := a.PlaySelected(); err == nil {
		t.Fatal("expected explicit empty-view error")
	}
	if a.Current().Path != current || !slices.Equal(paths(a.Queue().Tracks()), want) {
		t.Fatal("empty launch disturbed current playback")
	}
}
