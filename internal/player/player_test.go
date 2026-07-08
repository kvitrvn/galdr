package player

import (
	"errors"
	"testing"
	"time"
)

func TestState_String(t *testing.T) {
	cases := []struct {
		s    State
		want string
	}{
		{StateStopped, "stopped"},
		{StatePlaying, "playing"},
		{StatePaused, "paused"},
		{State(99), "state(99)"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("State(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

func TestMockPlayer_Seek_RequiresLoaded(t *testing.T) {
	m := NewMock()
	err := m.Seek(time.Second)
	if err == nil {
		t.Fatal("Seek without loaded track: expected error, got nil")
	}
}

func TestMockPlayer_Seek_Clamps(t *testing.T) {
	m := NewMock()
	_ = m.Load("track.mp3")
	m.SetDuration(2 * time.Minute)
	if err := m.Seek(5 * time.Minute); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if got := m.Position(); got != 2*time.Minute {
		t.Errorf("Position after clamp = %v, want 2m", got)
	}
	if err := m.Seek(-time.Hour); err != nil {
		t.Fatalf("Seek negative: %v", err)
	}
	if got := m.Position(); got != 0 {
		t.Errorf("Position after negative clamp = %v, want 0", got)
	}
}

func TestMockPlayer_Seek_Records(t *testing.T) {
	m := NewMock()
	_ = m.Load("track.mp3")
	if err := m.Seek(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	if err := m.Seek(20 * time.Second); err != nil {
		t.Fatal(err)
	}
	if len(m.SeekTargets) != 2 {
		t.Fatalf("SeekTargets len = %d, want 2", len(m.SeekTargets))
	}
	if m.SeekTargets[0] != 10*time.Second || m.SeekTargets[1] != 20*time.Second {
		t.Errorf("SeekTargets = %v, want [10s 20s]", m.SeekTargets)
	}
}

func TestMockPlayer_Seek_ErrInjected(t *testing.T) {
	m := NewMock()
	_ = m.Load("track.mp3")
	want := errors.New("boom")
	m.SeekErr = want
	if err := m.Seek(time.Second); !errors.Is(err, want) {
		t.Errorf("Seek err = %v, want %v", err, want)
	}
}
