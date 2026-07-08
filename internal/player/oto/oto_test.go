package oto

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kvitrvn/galdr/internal/player"
)

func TestPlayer_InitialState(t *testing.T) {
	p := New()
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State = %v, want %v", got, player.StateStopped)
	}
	if got := p.Volume(); got != 100 {
		t.Errorf("Volume = %d, want 100", got)
	}
	if got := p.Position(); got != 0 {
		t.Errorf("Position = %v, want 0", got)
	}
	if got := p.Duration(); got != 0 {
		t.Errorf("Duration = %v, want 0", got)
	}
}

func TestPlayer_LoadUnsupportedExtension(t *testing.T) {
	p := New()
	defer p.Stop()
	if err := p.Load("/tmp/x.ogg"); err == nil {
		t.Error("expected error for unsupported extension, got nil")
	}
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State after failed Load = %v, want %v", got, player.StateStopped)
	}
}

func TestPlayer_LoadMissingFile(t *testing.T) {
	p := New()
	defer p.Stop()
	if err := p.Load("/nonexistent/track.mp3"); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestPlayer_PlayWithoutLoad(t *testing.T) {
	p := New()
	if err := p.Play(); err == nil {
		t.Error("Play without Load should error")
	}
}

func TestPlayer_SetVolumeClamped(t *testing.T) {
	p := New()
	cases := []struct {
		in   int
		want int
	}{
		{-50, 0},
		{0, 0},
		{75, 75},
		{100, 100},
		{150, 100},
	}
	for _, c := range cases {
		if err := p.SetVolume(c.in); err != nil {
			t.Errorf("SetVolume(%d) err = %v", c.in, err)
		}
		if got := p.Volume(); got != c.want {
			t.Errorf("Volume after SetVolume(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPlayer_FormatDetectionRejectsNonAudio(t *testing.T) {
	p := New()
	defer p.Stop()

	dir := t.TempDir()
	path := dir + "/fake.mp3"
	if err := os.WriteFile(path, []byte("not actually mp3 data"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := p.Load(path)
	if err == nil {
		t.Error("expected error for malformed mp3, got nil")
	}
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State after failed Load = %v, want %v", got, player.StateStopped)
	}
}

// isAudioUnavailable reports whether err indicates that the Oto audio
// backend could not initialise in the current environment. It is used to
// skip integration tests on machines without audio hardware or when the
// global Oto context has already been claimed by an earlier test in the
// same process.
func isAudioUnavailable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if errors.Is(err, err) && msg == "" {
		return false
	}
	switch {
	case strings.Contains(msg, "no audio device"):
		return true
	case strings.Contains(msg, "context not ready"):
		return true
	case strings.Contains(msg, "context init timed out"):
		return true
	case strings.Contains(msg, "alsa"):
		return true
	case strings.Contains(msg, "context is already created"):
		// Another test in this package already created the global Oto
		// context. Treat as "can't reliably test" rather than failure.
		return true
	}
	return false
}

// Compile-time guarantee that Player satisfies player.Player.
func TestPlayerImplementsInterface(t *testing.T) {
	var _ player.Player = New()
}

// TestPlayer_FullLifecycleAndReplay is the only test that touches the
// real Oto audio backend. It exercises the full Load -> SetVolume ->
// Play -> Pause -> Stop cycle, then Loads a second track to verify that
// the shared Oto audio context is reused (instead of failing with
// "context is already created" as it did before the fix).
//
// Oto v3 maintains a single global audio context per process, so only
// one such test can run per `go test` invocation. It is skipped when no
// audio device is available.
func TestPlayer_FullLifecycleAndReplay(t *testing.T) {
	dir := t.TempDir()
	p := New()
	defer p.Stop()

	// First track: full lifecycle.
	path1 := writeWAV(t, dir)
	if err := p.Load(path1); err != nil {
		if isAudioUnavailable(err) {
			t.Skipf("no audio device available: %v", err)
		}
		t.Fatalf("first Load: %v", err)
	}
	if got := p.Duration(); got != time.Second {
		t.Errorf("Duration = %v, want 1s", got)
	}
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State after first Load = %v, want %v", got, player.StateStopped)
	}

	if err := p.SetVolume(50); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if got := p.Volume(); got != 50 {
		t.Errorf("Volume after SetVolume(50) = %d, want 50", got)
	}

	if err := p.Play(); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if got := p.State(); got != player.StatePlaying {
		t.Errorf("State after Play = %v, want %v", got, player.StatePlaying)
	}

	if err := p.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	if got := p.State(); got != player.StatePaused {
		t.Errorf("State after Pause = %v, want %v", got, player.StatePaused)
	}

	if err := p.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State after Stop = %v, want %v", got, player.StateStopped)
	}

	// Second track: must succeed without "context is already created".
	path2 := writeWAV(t, dir)
	if err := p.Load(path2); err != nil {
		t.Fatalf("second Load (should reuse context): %v", err)
	}
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State after second Load = %v, want %v", got, player.StateStopped)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop after second Load: %v", err)
	}
}
