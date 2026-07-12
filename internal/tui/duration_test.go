package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeDurationResult struct {
	duration time.Duration
	err      error
}

type fakeDurationProber struct {
	mu      sync.Mutex
	calls   []string
	results map[string]fakeDurationResult
	block   bool
	started chan struct{}
}

func (f *fakeDurationProber) ProbeDuration(ctx context.Context, path string) (time.Duration, error) {
	f.mu.Lock()
	f.calls = append(f.calls, path)
	result := f.results[path]
	block := f.block
	started := f.started
	f.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if block {
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return result.duration, result.err
}

func (f *fakeDurationProber) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func runDurationCmd(t *testing.T, m *Model, cmd tea.Cmd) tea.Cmd {
	t.Helper()
	if cmd == nil {
		t.Fatal("duration command = nil")
	}
	msg := cmd()
	if _, ok := msg.(durationProbeMsg); !ok {
		t.Fatalf("duration command returned %T", msg)
	}
	_, next := m.Update(msg)
	return next
}

func TestDurationWorker_ProbesSequentiallyAndAppliesResults(t *testing.T) {
	m := newTestModel(t, 3)
	paths := m.app.MissingDurationPaths()
	fake := &fakeDurationProber{results: map[string]fakeDurationResult{
		paths[0]: {duration: 61 * time.Second},
		paths[1]: {duration: 62 * time.Second},
		paths[2]: {duration: 63 * time.Second},
	}}
	m.durations.prober = fake

	cmd := m.startDurationProbes()
	if got := m.durationFooterStatus(); got != "Durées 0/3" {
		t.Fatalf("initial status = %q", got)
	}
	if fake.callCount() != 0 {
		t.Fatal("probe ran before Bubble Tea executed its command")
	}
	cmd = runDurationCmd(t, m, cmd)
	if fake.callCount() != 1 || m.app.Selected().Duration != 61*time.Second {
		t.Fatal("first result was not applied sequentially")
	}
	if got := m.durationFooterStatus(); got != "Durées 1/3" {
		t.Fatalf("status after first result = %q", got)
	}
	cmd = runDurationCmd(t, m, cmd)
	if fake.callCount() != 2 {
		t.Fatalf("calls after second command = %d, want 2", fake.callCount())
	}
	summaryCmd := runDurationCmd(t, m, cmd)
	if fake.callCount() != 3 {
		t.Fatalf("calls after completion = %d, want 3", fake.callCount())
	}
	if got := m.durationFooterStatus(); got != "Durées 3/3 · 0 indisponibles" {
		t.Fatalf("summary = %q", got)
	}
	if !strings.Contains(m.footerMessage(120), "Loaded 3 tracks") || !strings.Contains(m.footerMessage(120), "Durées 3/3") {
		t.Fatalf("footer did not preserve the user message: %q", m.footerMessage(120))
	}

	if summaryCmd == nil {
		t.Fatal("completion did not schedule summary expiration")
	}
	_, _ = m.Update(durationSummaryExpiredMsg{generation: m.durations.generation})
	if got := m.durationFooterStatus(); got != "" {
		t.Fatalf("expired summary = %q, want empty", got)
	}
}

func TestDurationWorker_CountsErrorsWithoutSettingAppError(t *testing.T) {
	m := newTestModel(t, 2)
	paths := m.app.MissingDurationPaths()
	fake := &fakeDurationProber{results: map[string]fakeDurationResult{
		paths[0]: {err: errors.New("decode failed")},
		paths[1]: {duration: 90 * time.Second},
	}}
	m.durations.prober = fake
	cmd := m.startDurationProbes()
	cmd = runDurationCmd(t, m, cmd)
	_ = runDurationCmd(t, m, cmd)

	if m.app.Error() != nil {
		t.Fatalf("probe error leaked into user errors: %v", m.app.Error())
	}
	if got := m.durationFooterStatus(); got != "Durées 1/2 · 1 indisponibles" {
		t.Fatalf("summary = %q", got)
	}
}

func TestDurationWorker_IgnoresStaleGeneration(t *testing.T) {
	m := newTestModel(t, 1)
	path := m.app.MissingDurationPaths()[0]
	m.durations.prober = &fakeDurationProber{}
	_ = m.startDurationProbes()
	oldGeneration := m.durations.generation
	_ = m.startDurationProbes()

	_, cmd := m.Update(durationProbeMsg{
		generation: oldGeneration,
		path:       path,
		duration:   4 * time.Minute,
		completed:  1,
		total:      1,
	})
	if cmd != nil {
		t.Fatal("stale result scheduled another probe")
	}
	if m.app.Selected().Duration != 0 || m.durations.completed != 0 {
		t.Fatal("stale result changed current generation state")
	}
}

func TestDurationWorker_RescanStartsNewGeneration(t *testing.T) {
	m := newTestModel(t, 1)
	fake := &fakeDurationProber{}
	m.durations.prober = fake
	_ = m.startDurationProbes()
	oldGeneration := m.durations.generation
	newPath := filepath.Join(m.app.Config().MusicDir, "new.mp3")
	if err := os.WriteFile(newPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := sendKey(t, m, "r")
	if cmd == nil {
		t.Fatal("rescan did not schedule duration probing")
	}
	if m.durations.generation == oldGeneration {
		t.Fatal("rescan did not replace the probe generation")
	}
	if m.durations.total != 2 || m.durations.completed != 0 {
		t.Fatalf("rescan progress = %d/%d, want 0/2", m.durations.completed, m.durations.total)
	}
}

func TestDurationWorker_CloseCancelsAndWaitsForActiveCommand(t *testing.T) {
	m := newTestModel(t, 1)
	fake := &fakeDurationProber{block: true, started: make(chan struct{}, 1)}
	m.durations.prober = fake
	cmd := m.startDurationProbes()
	done := make(chan struct{})
	go func() {
		_ = cmd()
		close(done)
	}()
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}

	m.Close()
	m.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close returned before the command stopped")
	}
}
