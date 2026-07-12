// Package mpv implements player.Player using libmpv.
package mpv

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	libmpv "github.com/gen2brain/go-mpv"

	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

const loadTimeout = 10 * time.Second

var errNoTrack = errors.New("mpv: no track loaded")

type mpvOption struct {
	name  string
	value string
}

type event struct {
	id        libmpv.EventID
	err       error
	endReason libmpv.Reason
	handled   chan struct{}
}

type client interface {
	SetOptionString(name, value string) error
	Initialize() error
	Command(command []string) error
	SetProperty(name string, format libmpv.Format, value any) error
	GetProperty(name string, format libmpv.Format) (any, error)
	WaitEvent(timeout float64) event
	Wakeup()
	Destroy()
	TerminateDestroy()
}

type realClient struct {
	client *libmpv.Mpv
}

func newRealClient() *realClient {
	return &realClient{client: libmpv.New()}
}

func (c *realClient) SetOptionString(name, value string) error {
	return c.client.SetOptionString(name, value)
}

func (c *realClient) Initialize() error { return c.client.Initialize() }

func (c *realClient) Command(command []string) error { return c.client.Command(command) }

func (c *realClient) SetProperty(name string, format libmpv.Format, value any) error {
	return c.client.SetProperty(name, format, value)
}

func (c *realClient) GetProperty(name string, format libmpv.Format) (any, error) {
	return c.client.GetProperty(name, format)
}

func (c *realClient) WaitEvent(timeout float64) event {
	ev := c.client.WaitEvent(timeout)
	result := event{id: ev.EventID, err: ev.Error}
	if ev.EventID == libmpv.EventEnd {
		end := ev.EndFile()
		result.endReason = end.Reason
		if result.err == nil {
			result.err = end.Error
		}
	}
	return result
}

func (c *realClient) Wakeup()           { c.client.Wakeup() }
func (c *realClient) Destroy()          { c.client.Destroy() }
func (c *realClient) TerminateDestroy() { c.client.TerminateDestroy() }

// Player is a single libmpv-backed player instance. Internal event delivery is
// synchronized with calls made by the application's single goroutine.
type Player struct {
	mu      sync.Mutex
	command sync.Mutex
	client  client

	state      player.State
	volume     int
	loaded     bool
	closed     bool
	naturalEnd bool

	loadWaiter chan error
	timeout    time.Duration
	done       chan struct{}
	closeOnce  sync.Once
}

// New creates and initializes a libmpv player with deterministic, audio-only
// settings. The user's mpv configuration and scripts are not loaded.
func New(options player.PlaybackOptions) (*Player, error) {
	return newPlayer(newRealClient(), loadTimeout, options)
}

func newPlayer(c client, timeout time.Duration, playbackOptions player.PlaybackOptions) (*Player, error) {
	p := &Player{
		client:  c,
		state:   player.StateStopped,
		volume:  100,
		timeout: timeout,
		done:    make(chan struct{}),
	}

	options := []mpvOption{
		{name: "config", value: "no"},
		{name: "terminal", value: "no"},
		{name: "input-default-bindings", value: "no"},
		{name: "input-vo-keyboard", value: "no"},
		{name: "osc", value: "no"},
		{name: "vid", value: "no"},
		{name: "audio-display", value: "no"},
		{name: "idle", value: "yes"},
		{name: "pause", value: "yes"},
	}
	replayGain, err := replayGainOptions(playbackOptions.ReplayGain)
	if err != nil {
		c.Destroy()
		return nil, err
	}
	options = append(options, replayGain...)
	for _, option := range options {
		if err := c.SetOptionString(option.name, option.value); err != nil {
			c.Destroy()
			return nil, fmt.Errorf("mpv: set option %s: %w", option.name, err)
		}
	}
	if err := c.Initialize(); err != nil {
		c.Destroy()
		return nil, fmt.Errorf("mpv: initialize: %w", err)
	}
	if err := c.SetProperty("volume", libmpv.FormatDouble, float64(p.volume)); err != nil {
		c.TerminateDestroy()
		return nil, fmt.Errorf("mpv: set initial volume: %w", err)
	}

	go p.eventLoop()
	return p, nil
}

func replayGainOptions(mode player.ReplayGainMode) ([]mpvOption, error) {
	switch mode {
	case player.ReplayGainOff:
		return []mpvOption{{name: "replaygain", value: "no"}}, nil
	case player.ReplayGainTrack:
		return activeReplayGainOptions("track"), nil
	case player.ReplayGainAlbum:
		return activeReplayGainOptions("album"), nil
	default:
		return nil, fmt.Errorf("mpv: invalid ReplayGain mode %d", mode)
	}
}

func activeReplayGainOptions(mode string) []mpvOption {
	return []mpvOption{
		{name: "replaygain", value: mode},
		// mpv lowers ReplayGain when needed instead of allowing clipping.
		{name: "replaygain-clip", value: "no"},
		// Untagged files keep their original level.
		{name: "replaygain-fallback", value: "0"},
	}
}

// Close stops the event loop and releases libmpv. It is idempotent.
func (p *Player) Close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.loaded = false
		p.state = player.StateStopped
		p.finishLoadLocked(errors.New("mpv: player closed"))
		p.mu.Unlock()

		p.client.Wakeup()
		<-p.done

		p.command.Lock()
		p.client.TerminateDestroy()
		p.command.Unlock()
	})
}

// Load validates and synchronously loads a supported local audio file. The
// track remains paused until Play is called.
func (p *Player) Load(path string) error {
	if _, ok := library.FormatFromPath(path); !ok {
		return fmt.Errorf("mpv: unsupported format for %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("mpv: open %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("mpv: open %s: not a regular file", path)
	}

	waiter := make(chan error, 1)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return errors.New("mpv: player closed")
	}
	p.loadWaiter = waiter
	p.loaded = false
	p.state = player.StateStopped
	p.naturalEnd = false
	p.mu.Unlock()

	p.command.Lock()
	err = p.client.SetProperty("pause", libmpv.FormatFlag, true)
	if err == nil {
		err = p.client.Command([]string{"loadfile", path, "replace"})
	}
	p.command.Unlock()
	if err != nil {
		p.cancelLoad(waiter)
		return fmt.Errorf("mpv: load %s: %w", path, err)
	}

	timer := time.NewTimer(p.timeout)
	defer timer.Stop()
	select {
	case err := <-waiter:
		if err != nil {
			return fmt.Errorf("mpv: load %s: %w", path, err)
		}
		return nil
	case <-timer.C:
		p.cancelLoad(waiter)
		return fmt.Errorf("mpv: load %s: timed out after %s", path, p.timeout)
	}
}

func (p *Player) cancelLoad(waiter chan error) {
	p.mu.Lock()
	if p.loadWaiter == waiter {
		p.loadWaiter = nil
	}
	if p.loadWaiter == nil {
		p.loaded = false
		p.state = player.StateStopped
		p.naturalEnd = false
	}
	p.mu.Unlock()
}

// Play starts or resumes the loaded track.
func (p *Player) Play() error {
	p.mu.Lock()
	if !p.loaded || p.closed {
		p.mu.Unlock()
		return errNoTrack
	}
	if p.state == player.StatePlaying {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	if err := p.setPause(false); err != nil {
		return fmt.Errorf("mpv: play: %w", err)
	}
	p.mu.Lock()
	if p.loaded && !p.closed {
		p.state = player.StatePlaying
	}
	p.mu.Unlock()
	return nil
}

// Pause pauses playback without unloading the track.
func (p *Player) Pause() error {
	p.mu.Lock()
	if !p.loaded || p.closed || p.state != player.StatePlaying {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	if err := p.setPause(true); err != nil {
		return fmt.Errorf("mpv: pause: %w", err)
	}
	p.mu.Lock()
	if p.loaded && !p.closed {
		p.state = player.StatePaused
	}
	p.mu.Unlock()
	return nil
}

func (p *Player) setPause(paused bool) error {
	p.command.Lock()
	defer p.command.Unlock()
	return p.client.SetProperty("pause", libmpv.FormatFlag, paused)
}

// Stop stops playback and clears all locally loaded-track state.
func (p *Player) Stop() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	wasLoaded := p.loaded || p.loadWaiter != nil
	p.mu.Unlock()
	if wasLoaded {
		p.command.Lock()
		err := p.client.Command([]string{"stop"})
		p.command.Unlock()
		if err != nil {
			return fmt.Errorf("mpv: stop: %w", err)
		}
	}
	p.mu.Lock()
	p.loaded = false
	p.state = player.StateStopped
	p.naturalEnd = false
	p.finishLoadLocked(errors.New("mpv: load stopped"))
	p.mu.Unlock()
	return nil
}

// SetVolume clamps volume to [0, 100] and applies it to libmpv.
func (p *Player) SetVolume(volume int) error {
	volume = max(0, min(100, volume))
	p.command.Lock()
	err := p.client.SetProperty("volume", libmpv.FormatDouble, float64(volume))
	p.command.Unlock()
	if err != nil {
		return fmt.Errorf("mpv: set volume: %w", err)
	}
	p.mu.Lock()
	p.volume = volume
	p.mu.Unlock()
	return nil
}

func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

func (p *Player) Position() time.Duration { return p.durationProperty("time-pos") }

func (p *Player) Duration() time.Duration { return p.durationProperty("duration") }

func (p *Player) durationProperty(name string) time.Duration {
	p.mu.Lock()
	loaded := p.loaded && !p.closed
	p.mu.Unlock()
	if !loaded {
		return 0
	}
	p.command.Lock()
	value, err := p.client.GetProperty(name, libmpv.FormatDouble)
	p.command.Unlock()
	if err != nil {
		return 0
	}
	seconds, ok := value.(float64)
	if !ok || seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func (p *Player) State() player.State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// ConsumeNaturalEnd reports a natural EOF once. internal/app uses this
// optional capability to avoid treating decode errors as queue completion.
func (p *Player) ConsumeNaturalEnd() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	ended := p.naturalEnd
	p.naturalEnd = false
	return ended
}

// Seek moves to an absolute position, clamped to the loaded duration when it
// is available.
func (p *Player) Seek(position time.Duration) error {
	p.mu.Lock()
	loaded := p.loaded && !p.closed
	p.mu.Unlock()
	if !loaded {
		return errNoTrack
	}
	if position < 0 {
		position = 0
	}
	if duration := p.Duration(); duration > 0 && position > duration {
		position = duration
	}
	seconds := strconv.FormatFloat(position.Seconds(), 'f', 6, 64)
	p.command.Lock()
	err := p.client.Command([]string{"seek", seconds, "absolute+exact"})
	p.command.Unlock()
	if err != nil {
		return fmt.Errorf("mpv: seek: %w", err)
	}
	return nil
}

func (p *Player) eventLoop() {
	defer close(p.done)
	for {
		p.mu.Lock()
		closed := p.closed
		p.mu.Unlock()
		if closed {
			return
		}

		ev := p.client.WaitEvent(0.1)
		p.mu.Lock()
		switch ev.id {
		case libmpv.EventFileLoaded:
			if p.loadWaiter == nil {
				break
			}
			if ev.err != nil {
				p.loaded = false
				p.state = player.StateStopped
				p.finishLoadLocked(ev.err)
			} else {
				p.loaded = true
				p.state = player.StateStopped
				p.naturalEnd = false
				p.finishLoadLocked(nil)
			}
		case libmpv.EventEnd:
			if ev.endReason == libmpv.EndFileEOF {
				p.loaded = false
				p.state = player.StateStopped
				p.naturalEnd = true
			} else if ev.endReason == libmpv.EndFileError {
				p.loaded = false
				p.state = player.StateStopped
				p.naturalEnd = false
				if ev.err == nil {
					ev.err = libmpv.ErrLoadingFailed
				}
				p.finishLoadLocked(ev.err)
			}
		case libmpv.EventShutdown:
			p.closed = true
			p.loaded = false
			p.state = player.StateStopped
			p.finishLoadLocked(errors.New("mpv: shutdown"))
		case libmpv.EventQueueOverflow:
			p.finishLoadLocked(libmpv.ErrEventQueueFull)
		}
		if ev.handled != nil {
			close(ev.handled)
		}
		p.mu.Unlock()
	}
}

func (p *Player) finishLoadLocked(err error) {
	if p.loadWaiter == nil {
		return
	}
	p.loadWaiter <- err
	p.loadWaiter = nil
}

var _ player.Player = (*Player)(nil)
