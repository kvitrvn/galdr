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

const (
	loadTimeout             = 10 * time.Second
	timePositionObservation = 1
	durationObservation     = 2
	outputSampleRate        = "48000"
)

var errNoTrack = errors.New("mpv: no track loaded")

type mpvOption struct {
	name  string
	value string
}

type event struct {
	id        libmpv.EventID
	err       error
	endReason libmpv.Reason
	entryID   int64
	property  string
	value     any
	handled   chan struct{}
}

type client interface {
	SetOptionString(name, value string) error
	Initialize() error
	Command(command []string) error
	SetProperty(name string, format libmpv.Format, value any) error
	GetProperty(name string, format libmpv.Format) (any, error)
	ObserveProperty(replyUserdata uint64, name string, format libmpv.Format) error
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

func (c *realClient) ObserveProperty(replyUserdata uint64, name string, format libmpv.Format) error {
	return c.client.ObserveProperty(replyUserdata, name, format)
}

func (c *realClient) WaitEvent(timeout float64) event {
	ev := c.client.WaitEvent(timeout)
	result := event{id: ev.EventID, err: ev.Error}
	if ev.EventID == libmpv.EventStart {
		result.entryID = ev.StartFile().EntryID
	}
	if ev.EventID == libmpv.EventEnd {
		end := ev.EndFile()
		result.endReason = end.Reason
		result.entryID = end.EntryID
		if result.err == nil {
			result.err = end.Error
		}
	}
	if ev.EventID == libmpv.EventPropertyChange {
		property := ev.Property()
		result.property = property.Name
		result.value = property.Data
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
	position   time.Duration
	duration   time.Duration

	activeToken      player.PlaybackToken
	prepared         *player.PreparedEntry
	pendingLoadToken player.PlaybackToken
	entryTokens      map[int64]player.PlaybackToken
	startingEntryID  int64
	events           chan player.PlaybackEvent

	loadWaiter chan error
	timeout    time.Duration
	done       chan struct{}
	closing    chan struct{}
	closeOnce  sync.Once
}

// New creates and initializes a libmpv player with deterministic, audio-only
// settings. The user's mpv configuration and scripts are not loaded.
func New(options player.PlaybackOptions) (*Player, error) {
	return newPlayer(newRealClient(), loadTimeout, options)
}

func newPlayer(c client, timeout time.Duration, playbackOptions player.PlaybackOptions) (*Player, error) {
	p := &Player{
		client:      c,
		state:       player.StateStopped,
		volume:      100,
		timeout:     timeout,
		done:        make(chan struct{}),
		closing:     make(chan struct{}),
		entryTokens: make(map[int64]player.PlaybackToken),
		events:      make(chan player.PlaybackEvent, 32),
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
		{name: "gapless-audio", value: "weak"},
		{name: "audio-samplerate", value: outputSampleRate},
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
	if err := c.ObserveProperty(timePositionObservation, "time-pos", libmpv.FormatDouble); err != nil {
		c.TerminateDestroy()
		return nil, fmt.Errorf("mpv: observe time-pos: %w", err)
	}
	if err := c.ObserveProperty(durationObservation, "duration", libmpv.FormatDouble); err != nil {
		c.TerminateDestroy()
		return nil, fmt.Errorf("mpv: observe duration: %w", err)
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
		p.position = 0
		p.duration = 0
		p.finishLoadLocked(errors.New("mpv: player closed"))
		p.mu.Unlock()
		close(p.closing)

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
	return p.LoadEntry(player.PreparedEntry{Path: path})
}

// LoadEntry replaces the native playlist with entry and waits until libmpv
// has loaded it. Playback remains paused until Play is called.
func (p *Player) LoadEntry(entry player.PreparedEntry) error {
	path := entry.Path
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
	p.position = 0
	p.duration = 0
	p.activeToken = 0
	p.prepared = nil
	p.pendingLoadToken = entry.Token
	p.entryTokens = make(map[int64]player.PlaybackToken)
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

// SyncNext keeps only the active native entry and the requested successor.
func (p *Player) SyncNext(
	expected player.PlaybackToken,
	next *player.PreparedEntry,
) (player.PlaybackToken, error) {
	p.mu.Lock()
	active := p.activeToken
	if active != expected || !p.loaded || p.closed {
		p.mu.Unlock()
		return active, nil
	}
	p.mu.Unlock()

	p.command.Lock()
	err := p.client.Command([]string{"playlist-clear"})
	if err == nil && next != nil {
		err = p.client.Command([]string{"loadfile", next.Path, "insert-next"})
	}
	var playlistID any
	if err == nil && next != nil {
		playlistID, err = p.client.GetProperty("playlist/1/id", libmpv.FormatInt64)
	}
	if err != nil && next != nil {
		if rollbackErr := p.client.Command([]string{"playlist-clear"}); rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("clear failed successor: %w", rollbackErr))
		}
	}
	p.command.Unlock()
	if err != nil {
		return active, fmt.Errorf("mpv: synchronize successor: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.activeToken != expected {
		return p.activeToken, nil
	}
	p.prepared = copyPreparedEntry(next)
	for id, token := range p.entryTokens {
		if token != p.activeToken {
			delete(p.entryTokens, id)
		}
	}
	if next != nil {
		id, ok := playlistID.(int64)
		if !ok {
			return p.activeToken, errors.New("mpv: successor playlist ID has unexpected type")
		}
		p.entryTokens[id] = next.Token
	}
	return p.activeToken, nil
}

// ActivateNext asks libmpv to play the already prepared successor.
func (p *Player) ActivateNext(expected player.PlaybackToken) (player.PlaybackToken, error) {
	p.mu.Lock()
	active := p.activeToken
	prepared := p.prepared != nil
	p.mu.Unlock()
	if active != expected || !prepared {
		return active, nil
	}
	p.command.Lock()
	err := p.client.Command([]string{"playlist-play-index", "1"})
	p.command.Unlock()
	if err != nil {
		return active, fmt.Errorf("mpv: activate successor: %w", err)
	}
	return active, nil
}

// PlaybackEvents returns backend transitions for consumption by Bubble Tea.
func (p *Player) PlaybackEvents() <-chan player.PlaybackEvent { return p.events }

func (p *Player) cancelLoad(waiter chan error) {
	p.mu.Lock()
	if p.loadWaiter == waiter {
		p.loadWaiter = nil
	}
	if p.loadWaiter == nil {
		p.loaded = false
		p.state = player.StateStopped
		p.naturalEnd = false
		p.position = 0
		p.duration = 0
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
	p.position = 0
	p.duration = 0
	p.activeToken = 0
	p.prepared = nil
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

func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.position
}

func (p *Player) Duration() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.duration
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
		var notification *player.PlaybackEvent
		p.mu.Lock()
		switch ev.id {
		case libmpv.EventStart:
			p.startingEntryID = ev.entryID
		case libmpv.EventFileLoaded:
			if ev.err != nil {
				p.loaded = false
				p.state = player.StateStopped
				p.finishLoadLocked(ev.err)
			} else {
				token := p.pendingLoadToken
				if mapped, ok := p.entryTokens[p.startingEntryID]; ok {
					token = mapped
				} else if p.loadWaiter == nil && p.prepared != nil {
					token = p.prepared.Token
				}
				p.loaded = true
				if p.loadWaiter != nil {
					p.state = player.StateStopped
				} else if p.state != player.StatePaused {
					p.state = player.StatePlaying
				}
				p.naturalEnd = false
				p.activeToken = token
				p.pendingLoadToken = 0
				if p.prepared != nil && p.prepared.Token == token {
					p.prepared = nil
				}
				p.finishLoadLocked(nil)
				notification = &player.PlaybackEvent{Kind: player.PlaybackStarted, Token: token}
			}
		case libmpv.EventEnd:
			token := p.activeToken
			if mapped, ok := p.entryTokens[ev.entryID]; ok {
				token = mapped
			}
			if ev.endReason == libmpv.EndFileEOF {
				if p.prepared == nil {
					p.loaded = false
					p.state = player.StateStopped
					p.position = 0
					p.duration = 0
				}
				p.naturalEnd = true
				notification = &player.PlaybackEvent{Kind: player.PlaybackEnded, Token: token}
			} else if ev.endReason == libmpv.EndFileError {
				if p.prepared == nil {
					p.loaded = false
					p.state = player.StateStopped
				}
				p.naturalEnd = false
				if ev.err == nil {
					ev.err = libmpv.ErrLoadingFailed
				}
				p.finishLoadLocked(ev.err)
				notification = &player.PlaybackEvent{
					Kind:  player.PlaybackFailed,
					Token: token,
					Err:   ev.err,
				}
			}
		case libmpv.EventPropertyChange:
			value := observedDuration(ev.value)
			switch ev.property {
			case "time-pos":
				p.position = value
			case "duration":
				p.duration = value
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
		if notification != nil {
			p.publish(*notification)
		}
	}
}

func (p *Player) publish(ev player.PlaybackEvent) {
	select {
	case p.events <- ev:
	case <-p.closing:
	}
}

func observedDuration(value any) time.Duration {
	seconds, ok := value.(float64)
	if !ok || seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func copyPreparedEntry(entry *player.PreparedEntry) *player.PreparedEntry {
	if entry == nil {
		return nil
	}
	copy := *entry
	return &copy
}

func (p *Player) finishLoadLocked(err error) {
	if p.loadWaiter == nil {
		return
	}
	p.loadWaiter <- err
	p.loadWaiter = nil
}

var _ player.Player = (*Player)(nil)
var _ player.GaplessPlayer = (*Player)(nil)
