package tui

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const durationSummaryDisplay = 3 * time.Second

// DurationProber is owned by the TUI consumer so tests and alternative audio
// backends do not need to depend on the concrete libmpv implementation.
type DurationProber interface {
	ProbeDuration(ctx context.Context, path string) (time.Duration, error)
}

type durationProbeMsg struct {
	generation uint64
	path       string
	duration   time.Duration
	err        error
	completed  int
	total      int
}

type durationSummaryExpiredMsg struct {
	generation uint64
}

type durationProbeState struct {
	prober DurationProber

	generation  uint64
	ctx         context.Context
	cancel      context.CancelFunc
	paths       []string
	next        int
	completed   int
	total       int
	unavailable int
	running     bool
	showSummary bool

	wg        sync.WaitGroup
	closeOnce sync.Once
}

func (m *Model) startDurationProbes() tea.Cmd {
	m.cancelDurationProbeGeneration()
	m.durations.paths = m.app.MissingDurationPaths()
	m.durations.next = 0
	m.durations.completed = 0
	m.durations.total = len(m.durations.paths)
	m.durations.unavailable = 0
	m.durations.showSummary = false
	if m.durations.prober == nil || m.durations.total == 0 {
		m.durations.running = false
		return nil
	}
	m.durations.ctx, m.durations.cancel = context.WithCancel(context.Background())
	m.durations.running = true
	return m.nextDurationProbeCmd()
}

func (m *Model) nextDurationProbeCmd() tea.Cmd {
	if !m.durations.running || m.durations.next >= m.durations.total {
		return nil
	}
	index := m.durations.next
	m.durations.next++
	ctx := m.durations.ctx
	generation := m.durations.generation
	path := m.durations.paths[index]
	total := m.durations.total
	prober := m.durations.prober
	m.durations.wg.Add(1)
	return func() tea.Msg {
		defer m.durations.wg.Done()
		duration, err := prober.ProbeDuration(ctx, path)
		return durationProbeMsg{
			generation: generation,
			path:       path,
			duration:   duration,
			err:        err,
			completed:  index + 1,
			total:      total,
		}
	}
}

func (m *Model) handleDurationProbe(msg durationProbeMsg) tea.Cmd {
	if msg.generation != m.durations.generation || !m.durations.running {
		return nil
	}
	m.durations.completed = msg.completed
	if msg.err != nil || !m.app.ApplyTrackDuration(msg.path, msg.duration) {
		m.durations.unavailable++
	}
	if msg.completed < msg.total {
		return m.nextDurationProbeCmd()
	}
	if m.durations.cancel != nil {
		m.durations.cancel()
	}
	m.durations.cancel = nil
	m.durations.ctx = nil
	m.durations.running = false
	m.durations.showSummary = true
	generation := m.durations.generation
	return tea.Tick(durationSummaryDisplay, func(time.Time) tea.Msg {
		return durationSummaryExpiredMsg{generation: generation}
	})
}

func (m *Model) cancelDurationProbeGeneration() {
	if m.durations.cancel != nil {
		m.durations.cancel()
	}
	m.durations.cancel = nil
	m.durations.ctx = nil
	m.durations.running = false
	m.durations.showSummary = false
	m.durations.generation++
}

// Close cancels any duration command still running and waits for it to return.
// The concrete prober remains owned by cmd/player and is closed afterwards.
func (m *Model) Close() {
	m.durations.closeOnce.Do(func() {
		m.cancelDurationProbeGeneration()
		m.durations.wg.Wait()
	})
}

func (m *Model) durationFooterStatus() string {
	switch {
	case m.durations.running:
		return fmt.Sprintf("Durées %d/%d", m.durations.completed, m.durations.total)
	case m.durations.showSummary:
		return fmt.Sprintf(
			"Durées %d/%d · %d indisponibles",
			m.durations.total-m.durations.unavailable,
			m.durations.total,
			m.durations.unavailable,
		)
	default:
		return ""
	}
}
