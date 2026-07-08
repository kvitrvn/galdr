package player

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestMock_InitialState(t *testing.T) {
	m := NewMock()
	if got := m.State(); got != StateStopped {
		t.Errorf("State = %v, want %v", got, StateStopped)
	}
	if got := m.Volume(); got != 100 {
		t.Errorf("Volume = %d, want 100", got)
	}
	if got := m.Position(); got != 0 {
		t.Errorf("Position = %v, want 0", got)
	}
	if got := m.Duration(); got != 0 {
		t.Errorf("Duration = %v, want 0", got)
	}
	if got := m.Path(); got != "" {
		t.Errorf("Path = %q, want empty", got)
	}
}

func TestMock_LoadRecordsPath(t *testing.T) {
	m := NewMock()
	if err := m.Load("/tmp/song.mp3"); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.LoadCalls) != 1 || m.LoadCalls[0] != "/tmp/song.mp3" {
		t.Errorf("LoadCalls = %v, want [/tmp/song.mp3]", m.LoadCalls)
	}
	if got := m.Path(); got != "/tmp/song.mp3" {
		t.Errorf("Path = %q, want /tmp/song.mp3", got)
	}
	if got := m.State(); got != StateStopped {
		t.Errorf("State after Load = %v, want %v", got, StateStopped)
	}
}

func TestMock_LoadPropagatesError(t *testing.T) {
	m := NewMock()
	wantErr := errors.New("disk on fire")
	m.LoadErr = wantErr
	err := m.Load("/x.mp3")
	if !errors.Is(err, wantErr) {
		t.Errorf("Load err = %v, want %v", err, wantErr)
	}
	if got := m.Path(); got != "" {
		t.Errorf("Path after failed Load = %q, want empty", got)
	}
}

func TestMock_PlayRequiresLoad(t *testing.T) {
	m := NewMock()
	if err := m.Play(); err == nil {
		t.Error("Play without Load should error")
	}
	if got := m.State(); got != StateStopped {
		t.Errorf("State = %v, want %v", got, StateStopped)
	}
}

func TestMock_PlayPauseStopTransitions(t *testing.T) {
	m := NewMock()
	if err := m.Load("/x.mp3"); err != nil {
		t.Fatal(err)
	}
	if err := m.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if got := m.State(); got != StatePlaying {
		t.Errorf("State after Play = %v, want %v", got, StatePlaying)
	}
	if m.PlayCalls != 1 {
		t.Errorf("PlayCalls = %d, want 1", m.PlayCalls)
	}

	if err := m.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if got := m.State(); got != StatePaused {
		t.Errorf("State after Pause = %v, want %v", got, StatePaused)
	}

	// Pause again is a no-op (does not transition, does not error).
	if err := m.Pause(); err != nil {
		t.Fatalf("Pause again: %v", err)
	}
	if got := m.State(); got != StatePaused {
		t.Errorf("State after second Pause = %v, want %v", got, StatePaused)
	}

	if err := m.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if got := m.State(); got != StatePlaying {
		t.Errorf("State after Resume = %v, want %v", got, StatePlaying)
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := m.State(); got != StateStopped {
		t.Errorf("State after Stop = %v, want %v", got, StateStopped)
	}
	if got := m.Position(); got != 0 {
		t.Errorf("Position after Stop = %v, want 0", got)
	}
}

func TestMock_VolumeClamped(t *testing.T) {
	m := NewMock()
	cases := []struct {
		in   int
		want int
	}{
		{-50, 0},
		{0, 0},
		{50, 50},
		{100, 100},
		{150, 100},
	}
	for _, c := range cases {
		if err := m.SetVolume(c.in); err != nil {
			t.Errorf("SetVolume(%d) err = %v", c.in, err)
		}
		if got := m.Volume(); got != c.want {
			t.Errorf("Volume after SetVolume(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestMock_PositionFromFn(t *testing.T) {
	m := NewMock()
	m.PositionFn = func() time.Duration { return 42 * time.Second }
	if got := m.Position(); got != 42*time.Second {
		t.Errorf("Position via PositionFn = %v, want 42s", got)
	}
}

func TestMock_SetPositionAndDuration(t *testing.T) {
	m := NewMock()
	m.SetPosition(3 * time.Second)
	m.SetDuration(180 * time.Second)
	if got := m.Position(); got != 3*time.Second {
		t.Errorf("Position = %v, want 3s", got)
	}
	if got := m.Duration(); got != 180*time.Second {
		t.Errorf("Duration = %v, want 180s", got)
	}
}

func TestMock_ConcurrentSafe(t *testing.T) {
	m := NewMock()
	_ = m.Load("/x.mp3")
	_ = m.Play()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); _ = m.Volume() }()
		go func() { defer wg.Done(); _ = m.State() }()
		go func() {
			defer wg.Done()
			_ = m.SetVolume(50)
		}()
	}
	wg.Wait()

	if got := m.Volume(); got != 50 {
		t.Errorf("Volume = %d, want 50", got)
	}
}
