package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

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
