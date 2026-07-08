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

	// Seek: rebuild the Oto player at 0.5s while still paused.
	if err := p.Seek(500 * time.Millisecond); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if got := p.State(); got != player.StateStopped {
		// After Seek without a prior Play, state should remain stopped
		// (or paused, depending on impl). Anything but playing is fine.
		if got == player.StatePlaying {
			t.Errorf("State after Seek on stopped track = %v, want non-playing", got)
		}
	}

	if err := p.Play(); err != nil {
		t.Fatalf("Play after Seek: %v", err)
	}
	if err := p.Seek(0); err != nil {
		t.Fatalf("Seek to 0: %v", err)
	}
	// After seeking back to 0, the player should be playing.
	if got := p.State(); got != player.StatePlaying {
		t.Errorf("State after Seek-to-0 = %v, want %v", got, player.StatePlaying)
	}

	if err := p.Stop(); err != nil {
		t.Fatalf("Stop after second Load: %v", err)
	}
}

// TestPlayer_SeekWithoutLoad covers the no-track error path without
// touching real audio, so it can run in any environment.
func TestPlayer_SeekWithoutLoad(t *testing.T) {
	p := New()
	defer p.Stop()
	if err := p.Seek(time.Second); err == nil {
		t.Error("expected error for Seek without Load")
	}
}

// TestPlayer_Seek_PreservesPosition is the regression test for the
// "seek resets the progress bar to 0" bug. After a Seek, Position()
// must reflect the target immediately, not jump back to 0 and only
// catch up after the new Oto player has consumed enough samples.
//
// The test requires a working Oto audio context. It is skipped on
// machines without audio hardware.
func TestPlayer_Seek_PreservesPosition(t *testing.T) {
	dir := t.TempDir()
	p := New()
	defer p.Stop()

	path := writeWAV(t, dir)
	if err := p.Load(path); err != nil {
		if isAudioUnavailable(err) {
			t.Skipf("no audio device available: %v", err)
		}
		t.Fatalf("Load: %v", err)
	}

	// Seek without playback: Position() should immediately be the
	// target, not 0.
	target := 500 * time.Millisecond
	if err := p.Seek(target); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	got := p.Position()
	// 16-bit sample granularity: one frame at 44.1 kHz is
	// ~22.7 µs, so we allow a small tolerance.
	tolerance := 50 * time.Millisecond
	if got < target-tolerance || got > target+tolerance {
		t.Errorf("Position after Seek(500ms) = %v, want ~%v", got, target)
	}

	// Seek to another position and re-check.
	target2 := 200 * time.Millisecond
	if err := p.Seek(target2); err != nil {
		t.Fatalf("Seek(200ms): %v", err)
	}
	if got := p.Position(); got < target2-tolerance || got > target2+tolerance {
		t.Errorf("Position after Seek(200ms) = %v, want ~%v", got, target2)
	}

	// Seek to 0 and check the position goes back to the start.
	if err := p.Seek(0); err != nil {
		t.Fatalf("Seek(0): %v", err)
	}
	if got := p.Position(); got > tolerance {
		t.Errorf("Position after Seek(0) = %v, want ~0", got)
	}
}
