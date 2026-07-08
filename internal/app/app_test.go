package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player"
)

func newTestApp(t *testing.T, n int) (*App, *player.MockPlayer) {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		name := filepath.Join(dir, fmt.Sprintf("t%02d.mp3", i))
		if err := writeFile(name); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.MusicDir = dir

	mock := player.NewMock()
	app := New(cfg, mock)
	if err := app.LoadLibrary(dir); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}
	return app, mock
}

func writeFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func TestApp_LoadLibrary_PopulatesQueue(t *testing.T) {
	a, _ := newTestApp(t, 5)
	if got := a.Queue().Len(); got != 5 {
		t.Errorf("Queue.Len = %d, want 5", got)
	}
	if got := a.SelectedIndex(); got != 0 {
		t.Errorf("SelectedIndex = %d, want 0", got)
	}
	if a.Status() == "" {
		t.Error("Status empty after LoadLibrary")
	}
}

func TestApp_LoadLibrary_NonExistentRoot(t *testing.T) {
	cfg := config.Default()
	mock := player.NewMock()
	a := New(cfg, mock)
	if err := a.LoadLibrary("/does/not/exist"); err == nil {
		t.Error("expected error for missing root, got nil")
	}
	if a.Error() == nil {
		t.Error("Error() nil after failed LoadLibrary")
	}
}

func TestApp_SelectNextAndPrev(t *testing.T) {
	a, _ := newTestApp(t, 3)
	a.SelectNext()
	if got := a.SelectedIndex(); got != 1 {
		t.Errorf("SelectedIndex after SelectNext = %d, want 1", got)
	}
	a.SelectNext()
	if got := a.SelectedIndex(); got != 2 {
		t.Errorf("SelectedIndex after 2x SelectNext = %d, want 2", got)
	}
	a.SelectNext() // at end, no-op
	if got := a.SelectedIndex(); got != 2 {
		t.Errorf("SelectedIndex at end = %d, want 2", got)
	}
	a.SelectPrev()
	if got := a.SelectedIndex(); got != 1 {
		t.Errorf("SelectedIndex after SelectPrev = %d, want 1", got)
	}
	a.SelectPrev()
	a.SelectPrev() // at start, no-op
	if got := a.SelectedIndex(); got != 0 {
		t.Errorf("SelectedIndex at start = %d, want 0", got)
	}
}

func TestApp_SelectOnEmptyQueue(t *testing.T) {
	cfg := config.Default()
	mock := player.NewMock()
	a := New(cfg, mock)

	// No panic.
	a.SelectNext()
	a.SelectPrev()
	if got := a.SelectedIndex(); got != 0 {
		t.Errorf("SelectedIndex = %d, want 0", got)
	}
	if a.Selected() != nil {
		t.Errorf("Selected = %+v, want nil", a.Selected())
	}
}

func TestApp_PlaySelected_LoadsAndPlays(t *testing.T) {
	a, mock := newTestApp(t, 3)

	if err := a.PlaySelected(); err != nil {
		t.Fatalf("PlaySelected: %v", err)
	}
	if got := a.State(); got != player.StatePlaying {
		t.Errorf("State = %v, want %v", got, player.StatePlaying)
	}
	if a.Current() == nil {
		t.Fatal("Current is nil after PlaySelected")
	}
	if a.Current().Path != a.Selected().Path {
		t.Errorf("Current.Path = %q, want %q", a.Current().Path, a.Selected().Path)
	}
	if got := mock.LoadCalls; len(got) != 1 {
		t.Errorf("len(LoadCalls) = %d, want 1", len(got))
	}
}

func TestApp_PlaySelected_TogglesWhenSameTrack(t *testing.T) {
	a, mock := newTestApp(t, 3)

	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	// Now press Enter again on the same selection -> toggle to paused.
	if err := a.PlaySelected(); err != nil {
		t.Fatalf("PlaySelected (toggle): %v", err)
	}
	if got := a.State(); got != player.StatePaused {
		t.Errorf("State = %v, want %v", got, player.StatePaused)
	}
	// Toggle back to playing.
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	if got := a.State(); got != player.StatePlaying {
		t.Errorf("State after third toggle = %v, want %v", got, player.StatePlaying)
	}
	if got := len(mock.LoadCalls); got != 1 {
		t.Errorf("len(LoadCalls) = %d, want 1 (toggle should not reload)", got)
	}
}

func TestApp_PlaySelected_OnEmptyQueue(t *testing.T) {
	cfg := config.Default()
	mock := player.NewMock()
	a := New(cfg, mock)

	if err := a.PlaySelected(); err == nil {
		t.Error("PlaySelected on empty queue should error")
	}
}

func TestApp_TogglePlay_FromStoppedPlaysSelected(t *testing.T) {
	a, _ := newTestApp(t, 2)
	a.SelectNext()
	if err := a.TogglePlay(); err != nil {
		t.Fatal(err)
	}
	if got := a.State(); got != player.StatePlaying {
		t.Errorf("State = %v, want %v", got, player.StatePlaying)
	}
}

func TestApp_TogglePlay_FromPlayingPauses(t *testing.T) {
	a, _ := newTestApp(t, 1)
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	if err := a.TogglePlay(); err != nil {
		t.Fatal(err)
	}
	if got := a.State(); got != player.StatePaused {
		t.Errorf("State = %v, want %v", got, player.StatePaused)
	}
}

func TestApp_TogglePlay_FromPausedResumes(t *testing.T) {
	a, mock := newTestApp(t, 1)
	_ = a.PlaySelected()
	_ = a.TogglePlay() // paused
	if err := a.TogglePlay(); err != nil {
		t.Fatal(err)
	}
	if got := a.State(); got != player.StatePlaying {
		t.Errorf("State = %v, want %v", got, player.StatePlaying)
	}
	if got := mock.PlayCalls; got != 2 {
		t.Errorf("PlayCalls = %d, want 2", got)
	}
}

func TestApp_Next_AdvancesAndPlays(t *testing.T) {
	a, _ := newTestApp(t, 3)
	_ = a.PlaySelected() // playing index 0
	if err := a.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got := a.SelectedIndex(); got != 1 {
		t.Errorf("SelectedIndex = %d, want 1", got)
	}
	if a.Current() == nil || a.Current().Path != a.Selected().Path {
		t.Errorf("Current should follow selection")
	}
	if got := a.State(); got != player.StatePlaying {
		t.Errorf("State = %v, want %v", got, player.StatePlaying)
	}
}

func TestApp_Next_AtEndStops(t *testing.T) {
	a, _ := newTestApp(t, 2)
	_ = a.PlaySelected() // index 0
	_ = a.Next()         // index 1
	if err := a.Next(); err != nil {
		t.Fatalf("Next at end: %v", err)
	}
	if got := a.State(); got != player.StateStopped {
		t.Errorf("State = %v, want %v", got, player.StateStopped)
	}
}

func TestApp_Next_OnEmptyQueue(t *testing.T) {
	cfg := config.Default()
	mock := player.NewMock()
	a := New(cfg, mock)
	if err := a.Next(); err == nil {
		t.Error("Next on empty queue should error")
	}
}

func TestApp_Previous_DecrementsAndPlays(t *testing.T) {
	a, _ := newTestApp(t, 3)
	_ = a.PlaySelected() // index 0
	_ = a.Next()         // index 1
	if err := a.Previous(); err != nil {
		t.Fatalf("Previous: %v", err)
	}
	if got := a.SelectedIndex(); got != 0 {
		t.Errorf("SelectedIndex = %d, want 0", got)
	}
}

func TestApp_Previous_AtStartNoOp(t *testing.T) {
	a, _ := newTestApp(t, 3)
	_ = a.PlaySelected() // index 0
	if err := a.Previous(); err != nil {
		t.Fatalf("Previous at start: %v", err)
	}
	if got := a.SelectedIndex(); got != 0 {
		t.Errorf("SelectedIndex = %d, want 0", got)
	}
	if got := a.State(); got != player.StatePlaying {
		t.Errorf("State = %v, want %v", got, player.StatePlaying)
	}
}

func TestApp_Previous_OnEmptyQueue(t *testing.T) {
	cfg := config.Default()
	mock := player.NewMock()
	a := New(cfg, mock)
	if err := a.Previous(); err == nil {
		t.Error("Previous on empty queue should error")
	}
}

func TestApp_VolumeUpAndDown(t *testing.T) {
	cfg := config.Default()
	mock := player.NewMock()
	a := New(cfg, mock)

	for i := 0; i < 25; i++ {
		_ = a.VolumeUp()
	}
	if got := a.Volume(); got != 100 {
		t.Errorf("Volume after many ups = %d, want 100", got)
	}

	for i := 0; i < 30; i++ {
		_ = a.VolumeDown()
	}
	if got := a.Volume(); got != 0 {
		t.Errorf("Volume after many downs = %d, want 0", got)
	}

	if err := a.VolumeUp(); err != nil {
		t.Errorf("VolumeUp err = %v", err)
	}
	if got := a.Volume(); got != VolumeStep {
		t.Errorf("Volume after one up = %d, want %d", got, VolumeStep)
	}
}

func TestApp_LoadFailure_DoesNotCrash(t *testing.T) {
	cfg := config.Default()
	mock := player.NewMock()
	a := New(cfg, mock)

	// Populate the queue first so PlaySelected has something to play.
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		if err := writeFile(filepath.Join(dir, fmt.Sprintf("t%02d.mp3", i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}

	// Inject a load error after the library is loaded.
	mock.LoadErr = errors.New("decoder exploded")
	if err := a.PlaySelected(); err == nil {
		t.Error("PlaySelected with LoadErr should return error")
	}
	if got := a.State(); got != player.StateStopped {
		t.Errorf("State = %v, want %v", got, player.StateStopped)
	}
	if a.Error() == nil {
		t.Error("Error() should be set after failed PlaySelected")
	}
	// App must remain usable: SelectNext / VolumeUp should still work.
	a.SelectNext()
	if got := a.SelectedIndex(); got != 1 {
		t.Errorf("SelectedIndex after SelectNext = %d, want 1", got)
	}
	if err := a.VolumeUp(); err != nil {
		t.Errorf("VolumeUp err = %v", err)
	}
}

func TestApp_Stop(t *testing.T) {
	a, _ := newTestApp(t, 2)
	_ = a.PlaySelected()
	if err := a.Stop(); err != nil {
		t.Fatal(err)
	}
	if got := a.State(); got != player.StateStopped {
		t.Errorf("State = %v, want %v", got, player.StateStopped)
	}
	if a.Current() != nil {
		t.Error("Current should be nil after Stop")
	}
}

func TestApp_MaybeAdvance_WhilePlaying_NoOp(t *testing.T) {
	a, _ := newTestApp(t, 3)
	_ = a.PlaySelected() // playing index 0

	if err := a.MaybeAdvance(); err != nil {
		t.Errorf("MaybeAdvance while playing: %v", err)
	}
	if got := a.SelectedIndex(); got != 0 {
		t.Errorf("MaybeAdvance should not change index while playing, got %d", got)
	}
}

func TestApp_MaybeAdvance_AfterUserStop_NoOp(t *testing.T) {
	a, _ := newTestApp(t, 3)
	_ = a.PlaySelected()
	_ = a.Stop() // user-initiated, clears currentTrack

	if err := a.MaybeAdvance(); err != nil {
		t.Errorf("MaybeAdvance after user Stop: %v", err)
	}
	if got := a.SelectedIndex(); got != 0 {
		t.Errorf("MaybeAdvance after Stop should not change index, got %d", got)
	}
}

func TestApp_MaybeAdvance_AfterNaturalEnd_Advances(t *testing.T) {
	a, mock := newTestApp(t, 3)
	_ = a.PlaySelected() // index 0

	// Simulate natural end-of-track: player stops but currentTrack stays.
	if err := mock.Stop(); err != nil {
		t.Fatal(err)
	}
	// currentTrack is still set (Stop on the player, not on the app).
	if a.Current() == nil {
		t.Fatal("Current should still be set after natural end")
	}

	if err := a.MaybeAdvance(); err != nil {
		t.Fatalf("MaybeAdvance: %v", err)
	}
	if got := a.SelectedIndex(); got != 1 {
		t.Errorf("MaybeAdvance should advance to index 1, got %d", got)
	}
	if a.Current() == nil || a.Current().Path != a.Queue().Tracks()[1].Path {
		t.Errorf("Current should be the new track after advance")
	}
}

func TestApp_MaybeAdvance_AtEndOfQueue_Stops(t *testing.T) {
	a, mock := newTestApp(t, 2)
	_ = a.PlaySelected() // index 0
	_ = a.Next()         // index 1 (now playing)
	_ = mock.Stop()      // simulate end-of-track at last track

	if err := a.MaybeAdvance(); err != nil {
		t.Errorf("MaybeAdvance at end: %v", err)
	}
	if a.Current() != nil {
		t.Errorf("Current should be nil after reaching end of queue")
	}
	if got := a.Status(); got != "End of queue" {
		t.Errorf("Status = %q, want %q", got, "End of queue")
	}
}

func TestApp_MaybeAdvance_EmptyQueue_NoOp(t *testing.T) {
	a, _ := newTestApp(t, 0)
	if err := a.MaybeAdvance(); err != nil {
		t.Errorf("MaybeAdvance on empty queue: %v", err)
	}
}

func TestApp_PositionDuration_DelegatesToPlayer(t *testing.T) {
	a, mock := newTestApp(t, 1)
	mock.SetPosition(10 * 1_000_000_000) // 10s
	mock.SetDuration(180 * 1_000_000_000)
	if got := a.Position(); got != 10*1_000_000_000 {
		t.Errorf("Position = %v, want 10s", got)
	}
	if got := a.Duration(); got != 180*1_000_000_000 {
		t.Errorf("Duration = %v, want 180s", got)
	}
}

func TestApp_Rescan_PicksUpNewFiles(t *testing.T) {
	a, _ := newTestApp(t, 3)
	if a.Queue().Len() != 3 {
		t.Fatalf("initial len = %d, want 3", a.Queue().Len())
	}

	// Drop a new file in the same directory.
	dir := a.Config().MusicDir
	if err := writeFile(filepath.Join(dir, "new.mp3")); err != nil {
		t.Fatal(err)
	}

	if err := a.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if got := a.Queue().Len(); got != 4 {
		t.Errorf("len after Rescan = %d, want 4", got)
	}
}

func TestApp_Rescan_PreservesSelection(t *testing.T) {
	a, _ := newTestApp(t, 3)
	a.SelectNext() // now at index 1

	if err := a.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if got := a.SelectedIndex(); got != 1 {
		t.Errorf("SelectedIndex after Rescan = %d, want 1", got)
	}
}

func TestApp_Rescan_StopsWhenCurrentRemoved(t *testing.T) {
	a, mock := newTestApp(t, 3)
	_ = a.PlaySelected()
	if err := mock.Stop(); err != nil {
		t.Fatal(err)
	}
	if a.Current() == nil {
		t.Fatal("setup: expected Current to be set after PlaySelected")
	}

	// Remove the file from disk.
	if err := os.Remove(a.Current().Path); err != nil {
		t.Fatal(err)
	}
	if err := a.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if a.Current() != nil {
		t.Errorf("Current after Rescan-removed = %+v, want nil", a.Current())
	}
}

func TestApp_ToggleShuffle(t *testing.T) {
	a, _ := newTestApp(t, 5)
	if a.Shuffle() {
		t.Fatal("Shuffle should start off")
	}
	a.ToggleShuffle()
	if !a.Shuffle() {
		t.Error("Shuffle after toggle = false, want true")
	}
	a.ToggleShuffle()
	if a.Shuffle() {
		t.Error("Shuffle after second toggle = true, want false")
	}
}

func TestApp_ShuffleNext_NotSame(t *testing.T) {
	a, _ := newTestApp(t, 5)
	_ = a.PlaySelected() // index 0
	a.ToggleShuffle()
	// Run Next many times. Each should land on a different index.
	seen := map[int]bool{0: true}
	for i := 0; i < 30; i++ {
		if err := a.Next(); err != nil {
			t.Fatalf("Next: %v", err)
		}
		seen[a.SelectedIndex()] = true
	}
	if len(seen) < 4 {
		t.Errorf("shuffle Next only visited %d distinct indices, want most of 5", len(seen))
	}
}

func TestApp_ShufflePrevious_NotSame(t *testing.T) {
	a, _ := newTestApp(t, 5)
	_ = a.PlaySelected() // index 0
	a.ToggleShuffle()
	// A handful of consecutive Previous calls must not all land on
	// the same track. This is a weak statistical check; on a 5-track
	// queue the chance of 5 identical picks is 4/5^4 ≈ 5%.
	seen := map[int]bool{0: true}
	for i := 0; i < 10; i++ {
		if err := a.Previous(); err != nil {
			t.Fatalf("Previous: %v", err)
		}
		seen[a.SelectedIndex()] = true
	}
	if len(seen) < 3 {
		t.Errorf("shuffle Previous only visited %d distinct indices, want at least 3", len(seen))
	}
}

func TestApp_CycleRepeat(t *testing.T) {
	a, _ := newTestApp(t, 3)
	if a.Repeat() != RepeatOff {
		t.Fatalf("initial Repeat = %v, want off", a.Repeat())
	}
	a.CycleRepeat()
	if a.Repeat() != RepeatAll {
		t.Errorf("after first cycle = %v, want all", a.Repeat())
	}
	a.CycleRepeat()
	if a.Repeat() != RepeatOne {
		t.Errorf("after second cycle = %v, want one", a.Repeat())
	}
	a.CycleRepeat()
	if a.Repeat() != RepeatOff {
		t.Errorf("after third cycle = %v, want off", a.Repeat())
	}
}

func TestApp_RepeatMode_String(t *testing.T) {
	cases := []struct {
		m    RepeatMode
		want string
	}{
		{RepeatOff, "off"},
		{RepeatAll, "all"},
		{RepeatOne, "one"},
		{RepeatMode(99), "off"},
	}
	for _, c := range cases {
		if got := c.m.String(); got != c.want {
			t.Errorf("%d.String() = %q, want %q", c.m, got, c.want)
		}
	}
}

func TestApp_RepeatAll_WrapsAtEnd(t *testing.T) {
	a, mock := newTestApp(t, 3)
	_ = a.PlaySelected() // index 0
	a.CycleRepeat()      // all
	_ = a.Next()         // index 1
	_ = a.Next()         // index 2
	_ = mock.Stop()      // simulate end-of-track at last
	_ = a.MaybeAdvance()
	if a.SelectedIndex() != 0 {
		t.Errorf("RepeatAll wrap: index = %d, want 0", a.SelectedIndex())
	}
}

func TestApp_RepeatOne_Reloads(t *testing.T) {
	a, mock := newTestApp(t, 3)
	_ = a.PlaySelected() // index 0
	a.CycleRepeat()
	a.CycleRepeat() // one
	_ = mock.Stop() // simulate end-of-track
	_ = a.MaybeAdvance()
	if a.SelectedIndex() != 0 {
		t.Errorf("RepeatOne: index = %d, want 0 (reload same)", a.SelectedIndex())
	}
}

func TestApp_ToggleMute(t *testing.T) {
	a, mock := newTestApp(t, 1)
	_ = a.VolumeUp() // already 100, no change
	if a.Volume() != 100 {
		t.Fatalf("setup: Volume = %d, want 100", a.Volume())
	}
	if a.Muted() {
		t.Fatal("setup: Muted = true, want false")
	}
	a.ToggleMute()
	if !a.Muted() {
		t.Error("after mute: Muted = false, want true")
	}
	if a.Volume() != 0 {
		t.Errorf("Volume while muted = %d, want 0", a.Volume())
	}
	if got := mock.Volume(); got != 0 {
		t.Errorf("mock Volume while muted = %d, want 0", got)
	}
	a.ToggleMute()
	if a.Muted() {
		t.Error("after unmute: Muted = true, want false")
	}
	if a.Volume() != 100 {
		t.Errorf("Volume after unmute = %d, want 100", a.Volume())
	}
}

func TestApp_VolumeWhileMuted_UpdatesSaved(t *testing.T) {
	a, _ := newTestApp(t, 1)
	// Bring volume down to 0 first, then mute, then up.
	for i := 0; i < 20; i++ {
		_ = a.VolumeDown()
	}
	if a.Volume() != 0 {
		t.Fatalf("setup: Volume = %d, want 0", a.Volume())
	}
	a.ToggleMute()
	_ = a.VolumeUp()
	if got := a.SavedVolume(); got != 5 {
		t.Errorf("SavedVolume after mute+up = %d, want 5", got)
	}
	if got := a.Volume(); got != 0 {
		t.Errorf("Volume while muted = %d, want 0", got)
	}
	a.ToggleMute()
	if got := a.Volume(); got != 5 {
		t.Errorf("Volume after unmute = %d, want 5", got)
	}
}

func TestApp_Seek_DelegatesToPlayer(t *testing.T) {
	a, mock := newTestApp(t, 1)
	_ = a.PlaySelected()
	if err := a.Seek(30 * time.Second); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if len(mock.SeekTargets) != 1 {
		t.Fatalf("SeekTargets len = %d, want 1", len(mock.SeekTargets))
	}
	if mock.SeekTargets[0] != 30*time.Second {
		t.Errorf("SeekTargets[0] = %v, want 30s", mock.SeekTargets[0])
	}
}

func TestApp_Snapshot_AndApplySnapshot(t *testing.T) {
	a, _ := newTestApp(t, 3)
	_ = a.PlaySelected() // index 0
	_ = a.VolumeDown()
	_ = a.VolumeDown()

	vol, path := a.Snapshot()
	if vol != 90 {
		t.Errorf("Snapshot Volume = %d, want 90", vol)
	}
	if path == "" {
		t.Error("Snapshot CurrentPath = empty, want a path")
	}

	// Build a new app, apply snapshot, verify.
	mock2 := player.NewMock()
	a2 := New(a.Config(), mock2)
	a2.ApplySnapshot(42, "/music/foo.mp3")
	if got := a2.Volume(); got != 42 {
		t.Errorf("Volume after ApplySnapshot = %d, want 42", got)
	}
	if got := a2.SavedVolume(); got != 42 {
		t.Errorf("SavedVolume after ApplySnapshot = %d, want 42", got)
	}

	// Out-of-range volume is clamped to 100.
	a2.ApplySnapshot(250, "")
	if got := a2.SavedVolume(); got != 100 {
		t.Errorf("SavedVolume after clamp = %d, want 100", got)
	}
}

func TestApp_TracksRoundTrip(t *testing.T) {
	a, mock := newTestApp(t, 4)
	tracks := a.Queue().Tracks()
	if len(tracks) != 4 {
		t.Fatalf("len(Tracks()) = %d, want 4", len(tracks))
	}
	// Play the second track so the mock records a load.
	a.SelectNext()
	if err := a.PlaySelected(); err != nil {
		t.Fatalf("PlaySelected: %v", err)
	}
	if len(mock.LoadCalls) != 1 {
		t.Fatalf("len(LoadCalls) = %d, want 1", len(mock.LoadCalls))
	}
	if mock.LoadCalls[0] != tracks[1].Path {
		t.Errorf("LoadCalls[0] = %q, want %q", mock.LoadCalls[0], tracks[1].Path)
	}
	if a.Current() == nil || a.Current().Path != tracks[1].Path {
		t.Errorf("Current.Path = %+v, want %q", a.Current(), tracks[1].Path)
	}
}

func TestApp_New_NilConfigUsesDefault(t *testing.T) {
	a := New(nil, player.NewMock())
	if a.Config() == nil {
		t.Fatal("Config() is nil with nil config passed to New")
	}
	if a.Config().MusicDir == "" {
		t.Error("default config has empty MusicDir")
	}
}

func TestApp_StatusUpdates(t *testing.T) {
	a, _ := newTestApp(t, 1)

	if got := a.Status(); got == "" {
		t.Error("Status empty after LoadLibrary")
	}

	_ = a.PlaySelected()
	if got := a.Status(); got != "Playing: "+tracksTitles(a)[0] {
		t.Errorf("Status = %q, want %q", got, "Playing: t00")
	}

	_ = a.TogglePlay()
	if got := a.Status(); got != "Paused" {
		t.Errorf("Status after pause = %q, want %q", got, "Paused")
	}

	_ = a.TogglePlay()
	if got := a.Status(); got != "Resumed" {
		t.Errorf("Status after resume = %q, want %q", got, "Resumed")
	}

	_ = a.Stop()
	if got := a.Status(); got != "Stopped" {
		t.Errorf("Status after stop = %q, want %q", got, "Stopped")
	}
}

func tracksTitles(a *App) []string {
	out := []string{}
	for _, tr := range a.Queue().Tracks() {
		out = append(out, tr.Title)
	}
	return out
}

// newTestAppWithTitles builds an app whose library contains tracks
// with the given titles. Tracks are stored in a real temp directory
// so that library.Scan discovers them.
func newTestAppWithTitles(t *testing.T, titles []string) (*App, *player.MockPlayer) {
	t.Helper()
	dir := t.TempDir()
	for i := range titles {
		name := filepath.Join(dir, fmt.Sprintf("t%02d.mp3", i))
		if err := os.WriteFile(name, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.MusicDir = dir
	mock := player.NewMock()
	a := New(cfg, mock)
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}
	// The mock player doesn't read real metadata, so patch the
	// tracks' titles in place via the queue's current contents.
	q := a.Queue()
	all := q.Tracks()
	if len(all) != len(titles) {
		t.Fatalf("len(tracks) = %d, want %d", len(all), len(titles))
	}
	for i, title := range titles {
		all[i].Title = title
	}
	q.Replace(all)
	return a, mock
}

func TestApp_HasDuration(t *testing.T) {
	a, mock := newTestApp(t, 1)
	// Default mock has no duration set.
	if a.HasDuration() {
		t.Error("HasDuration = true with no duration set, want false")
	}
	mock.SetDuration(3 * time.Minute)
	if !a.HasDuration() {
		t.Error("HasDuration = false after SetDuration(3m), want true")
	}
}

func TestApp_SetFilter_RestrictsVisible(t *testing.T) {
	a, _ := newTestAppWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	a.SetFilter("limbo")
	if got := a.VisibleLen(); got != 1 {
		t.Errorf("VisibleLen = %d, want 1", got)
	}
	if !a.HasFilter() {
		t.Error("HasFilter = false, want true")
	}
	if got := a.Filter(); got != "limbo" {
		t.Errorf("Filter = %q, want %q", got, "limbo")
	}
	if a.VisibleTracks()[0].Title != "Limbo" {
		t.Errorf("VisibleTracks[0] = %q, want Limbo", a.VisibleTracks()[0].Title)
	}
}

func TestApp_SetFilter_Clears(t *testing.T) {
	a, _ := newTestAppWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	a.SetFilter("limbo")
	a.SetFilter("")
	if a.HasFilter() {
		t.Error("HasFilter = true after clearing, want false")
	}
	if got := a.VisibleLen(); got != 3 {
		t.Errorf("VisibleLen = %d, want 3", got)
	}
}

func TestApp_Next_HonoursFilter(t *testing.T) {
	a, _ := newTestAppWithTitles(t, []string{"Anthem", "Limbo", "Amen", "Limbo"})
	a.SetFilter("limbo")
	// Selection moves to first match ("Limbo" at index 1).
	if a.SelectedIndex() != 1 {
		t.Errorf("SelectedIndex after filter = %d, want 1", a.SelectedIndex())
	}
	if err := a.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if a.SelectedIndex() != 3 {
		t.Errorf("SelectedIndex after Next = %d, want 3", a.SelectedIndex())
	}
	// At end of filtered view, Next with no repeat stops playback.
	_ = a.Next()
	if a.Current() != nil {
		t.Error("Current should be nil after Next at end with no repeat")
	}
}

func TestApp_Next_RepeatAllWrapsToFirstVisible(t *testing.T) {
	a, _ := newTestAppWithTitles(t, []string{"Anthem", "Limbo", "Amen", "Limbo"})
	a.SetFilter("limbo")
	a.CycleRepeat() // all
	if a.SelectedIndex() != 1 {
		t.Fatalf("setup: SelectedIndex = %d, want 1", a.SelectedIndex())
	}
	if err := a.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if a.SelectedIndex() != 3 {
		t.Fatalf("SelectedIndex after Next = %d, want 3", a.SelectedIndex())
	}
	// Next at end + repeat all -> wrap to first visible.
	if err := a.Next(); err != nil {
		t.Fatalf("Next (wrap): %v", err)
	}
	if a.SelectedIndex() != 1 {
		t.Errorf("SelectedIndex after wrap = %d, want 1", a.SelectedIndex())
	}
}

func TestApp_Rescan_ClearsFilter(t *testing.T) {
	a, _ := newTestAppWithTitles(t, []string{"Anthem", "Limbo"})
	a.SetFilter("limbo")
	if err := a.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	if a.HasFilter() {
		t.Error("Rescan should clear the filter")
	}
}
