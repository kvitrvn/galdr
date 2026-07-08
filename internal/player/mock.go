package player

import (
	"fmt"
	"sync"
	"time"
)

// MockPlayer is a non-audio implementation of Player used by tests and as a
// fallback when no audio hardware is available.
//
// MockPlayer is safe for concurrent use.
type MockPlayer struct {
	mu       sync.Mutex
	state    State
	volume   int
	path     string
	position time.Duration
	duration time.Duration

	// Recording of method calls; tests inspect these to assert behaviour.
	LoadCalls  []string
	PlayCalls  int
	PauseCalls int
	StopCalls  int
	Volumes    []int

	// Optional error injection. If non-nil, the corresponding method
	// returns this error without performing its side effect.
	LoadErr  error
	PlayErr  error
	PauseErr error
	StopErr  error

	// PositionFn, when non-nil, is called by Position to produce a
	// dynamic value. Tests use this to simulate playback advancing.
	PositionFn func() time.Duration
}

// NewMock returns a MockPlayer in the stopped state with volume 100.
func NewMock() *MockPlayer {
	return &MockPlayer{state: StateStopped, volume: 100}
}

// Load records the call and stores the path. If LoadErr is set, it is
// returned and state is left untouched.
func (m *MockPlayer) Load(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LoadCalls = append(m.LoadCalls, path)
	if m.LoadErr != nil {
		return m.LoadErr
	}
	m.path = path
	m.position = 0
	m.duration = 0
	m.state = StateStopped
	return nil
}

// Play moves to the playing state.
func (m *MockPlayer) Play() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PlayCalls++
	if m.PlayErr != nil {
		return m.PlayErr
	}
	if m.path == "" {
		return fmt.Errorf("mock: no track loaded")
	}
	m.state = StatePlaying
	return nil
}

// Pause moves to the paused state.
func (m *MockPlayer) Pause() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PauseCalls++
	if m.PauseErr != nil {
		return m.PauseErr
	}
	if m.state == StatePlaying {
		m.state = StatePaused
	}
	return nil
}

// Stop releases the loaded track.
func (m *MockPlayer) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StopCalls++
	if m.StopErr != nil {
		return m.StopErr
	}
	m.state = StateStopped
	m.position = 0
	return nil
}

// SetVolume clamps vol into [0, 100] and stores it.
func (m *MockPlayer) SetVolume(vol int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	m.volume = vol
	m.Volumes = append(m.Volumes, vol)
	return nil
}

// Volume returns the stored volume.
func (m *MockPlayer) Volume() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.volume
}

// Position returns the current position, or the result of PositionFn if set.
func (m *MockPlayer) Position() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.PositionFn != nil {
		return m.PositionFn()
	}
	return m.position
}

// SetPosition updates the position reported by Position. Useful in tests
// that want to assert specific progress values.
func (m *MockPlayer) SetPosition(pos time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.position = pos
}

// Duration returns the stored duration.
func (m *MockPlayer) Duration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.duration
}

// SetDuration updates the duration reported by Duration. Useful for tests.
func (m *MockPlayer) SetDuration(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.duration = d
}

// State returns the current state.
func (m *MockPlayer) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

// Path returns the currently loaded path, or "" if none.
func (m *MockPlayer) Path() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.path
}
