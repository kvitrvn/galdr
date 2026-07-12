package mpv

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	libmpv "github.com/gen2brain/go-mpv"

	"github.com/kvitrvn/galdr/internal/library"
)

const durationProbeTimeout = 5 * time.Second

// DurationProber owns a dedicated, audio-silent libmpv instance used only to
// discover durations that the lightweight library scan could not determine.
// Probes are serialized even when callers invoke ProbeDuration concurrently.
type DurationProber struct {
	client  client
	timeout time.Duration

	operation sync.Mutex
	mu        sync.Mutex
	closed    bool
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// NewDurationProber creates an isolated libmpv duration probe. The user's mpv
// configuration and scripts are disabled, and no real audio or video output is
// opened.
func NewDurationProber() (*DurationProber, error) {
	return newDurationProber(newRealClient(), durationProbeTimeout)
}

func newDurationProber(c client, timeout time.Duration) (*DurationProber, error) {
	p := &DurationProber{client: c, timeout: timeout}
	options := []struct {
		name  string
		value string
	}{
		{name: "config", value: "no"},
		{name: "terminal", value: "no"},
		{name: "input-default-bindings", value: "no"},
		{name: "input-vo-keyboard", value: "no"},
		{name: "osc", value: "no"},
		{name: "vid", value: "no"},
		{name: "audio-display", value: "no"},
		{name: "idle", value: "yes"},
		{name: "pause", value: "yes"},
		{name: "ao", value: "null"},
	}
	for _, option := range options {
		if err := c.SetOptionString(option.name, option.value); err != nil {
			c.Destroy()
			return nil, fmt.Errorf("mpv duration probe: set option %s: %w", option.name, err)
		}
	}
	if err := c.Initialize(); err != nil {
		c.Destroy()
		return nil, fmt.Errorf("mpv duration probe: initialize: %w", err)
	}
	return p, nil
}

// ProbeDuration loads one supported local audio file, waits until libmpv has
// parsed it, and returns its duration. Each call has a five-second upper bound
// in addition to any earlier caller deadline.
func (p *DurationProber) ProbeDuration(ctx context.Context, path string) (duration time.Duration, err error) {
	if ctx == nil {
		return 0, errors.New("mpv duration probe: nil context")
	}
	if _, ok := library.FormatFromPath(path); !ok {
		return 0, fmt.Errorf("mpv duration probe: unsupported format for %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("mpv duration probe: open %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("mpv duration probe: open %s: not a regular file", path)
	}

	p.operation.Lock()
	defer p.operation.Unlock()

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, errors.New("mpv duration probe: closed")
	}
	probeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	p.cancel = cancel
	p.mu.Unlock()
	defer func() {
		cancel()
		p.mu.Lock()
		p.cancel = nil
		p.mu.Unlock()
	}()
	if ctxErr := probeCtx.Err(); ctxErr != nil {
		return 0, fmt.Errorf("mpv duration probe: probe %s: %w", path, ctxErr)
	}

	if err := p.client.Command([]string{"loadfile", path, "replace"}); err != nil {
		return 0, fmt.Errorf("mpv duration probe: load %s: %w", path, err)
	}
	defer func() {
		if stopErr := p.client.Command([]string{"stop"}); stopErr != nil && err == nil {
			duration = 0
			err = fmt.Errorf("mpv duration probe: stop %s: %w", path, stopErr)
		}
	}()

	for {
		if ctxErr := probeCtx.Err(); ctxErr != nil {
			return 0, fmt.Errorf("mpv duration probe: probe %s: %w", path, ctxErr)
		}
		ev := p.client.WaitEvent(0.1)
		if ctxErr := probeCtx.Err(); ctxErr != nil {
			return 0, fmt.Errorf("mpv duration probe: probe %s: %w", path, ctxErr)
		}
		switch ev.id {
		case libmpv.EventFileLoaded:
			if ev.err != nil {
				return 0, fmt.Errorf("mpv duration probe: load %s: %w", path, ev.err)
			}
			value, propertyErr := p.client.GetProperty("duration", libmpv.FormatDouble)
			if propertyErr != nil {
				return 0, fmt.Errorf("mpv duration probe: duration %s: %w", path, propertyErr)
			}
			seconds, ok := value.(float64)
			maxDurationSeconds := float64(time.Duration(1<<63-1)) / float64(time.Second)
			if !ok || seconds <= 0 || seconds > maxDurationSeconds || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
				return 0, fmt.Errorf("mpv duration probe: invalid duration for %s", path)
			}
			return time.Duration(seconds * float64(time.Second)), nil

		case libmpv.EventEnd:
			// loadfile replace and the stop at the end of the previous probe can
			// leave an old stop/redirect event ahead of this file's loaded event.
			if ev.endReason == libmpv.EndFileStop || ev.endReason == libmpv.EndFileRedirect {
				continue
			}
			if ev.err == nil {
				ev.err = libmpv.ErrLoadingFailed
			}
			return 0, fmt.Errorf("mpv duration probe: decode %s: %w", path, ev.err)

		case libmpv.EventShutdown:
			return 0, errors.New("mpv duration probe: shutdown")
		case libmpv.EventQueueOverflow:
			return 0, fmt.Errorf("mpv duration probe: events: %w", libmpv.ErrEventQueueFull)
		}
	}
}

// Close cancels an active probe, waits for it to release the client, and then
// destroys the dedicated libmpv instance. It is safe to call more than once.
func (p *DurationProber) Close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		cancel := p.cancel
		p.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		p.client.Wakeup()

		p.operation.Lock()
		p.client.TerminateDestroy()
		p.operation.Unlock()
	})
}
