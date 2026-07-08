// Package oto implements the player.Player interface on top of the Oto v3
// audio library.
//
// The package exposes a single Player type which can be constructed with
// New and reused for as many tracks as needed.
//
// Thread-safety: Player is safe to call from a single goroutine. The
// Bubble Tea loop in internal/tui is the typical caller; no other
// goroutine should issue Player calls concurrently.
package oto

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"

	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

// Player is the Oto-backed implementation of player.Player.
//
// The Oto v3 backend enforces a single global audio context per process;
// the first Load picks the sample rate and channel count and all
// subsequent Loads reuse the same context. Tracks whose parameters
// differ will play at the wrong speed. This is acceptable for the MVP
// because the vast majority of music files are 44.1 kHz stereo.
type Player struct {
	mu     sync.Mutex
	state  player.State
	volume int

	source     pcmSource
	reader     *countingReader
	otoPlayer  *oto.Player
	sampleRate int
	channels   int

	// ctx is the Oto audio context, created lazily on the first Load
	// and reused for the lifetime of the process.
	ctx *oto.Context
}

// New returns a fresh Oto Player with the default volume of 100.
func New() *Player {
	return &Player{state: player.StateStopped, volume: 100}
}

// Load prepares the track at path for playback. The format is detected
// from the file extension via library.FormatFromPath.
//
// On the first call a shared Oto audio context is created using the
// source's sample rate and channel count; subsequent calls reuse that
// context. Tracks whose parameters differ from the first one will play
// at the wrong speed — this is a known MVP limitation.
func (p *Player) Load(path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.releaseLocked()

	format, ok := library.FormatFromPath(path)
	if !ok {
		return fmt.Errorf("oto: unsupported format for %s", path)
	}

	src, err := openSource(format, path)
	if err != nil {
		return fmt.Errorf("oto: open %s: %w", path, err)
	}

	if err := p.ensureContextLocked(src.SampleRate(), src.Channels()); err != nil {
		_ = src.Close()
		return err
	}

	cr := &countingReader{src: src}
	otoPl := p.ctx.NewPlayer(cr)
	otoPl.SetVolume(float64(p.volume) / 100.0)

	p.source = src
	p.reader = cr
	p.otoPlayer = otoPl
	p.sampleRate = src.SampleRate()
	p.channels = src.Channels()
	p.state = player.StateStopped
	return nil
}

// ensureContextLocked creates the shared Oto audio context on first use.
// It must be called with p.mu held.
func (p *Player) ensureContextLocked(sampleRate, channels int) error {
	if p.ctx != nil {
		return nil
	}
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channels,
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		return fmt.Errorf("oto: new context: %w", err)
	}
	select {
	case <-ready:
		if cerr := ctx.Err(); cerr != nil {
			return fmt.Errorf("oto: context not ready: %w", cerr)
		}
	case <-time.After(2 * time.Second):
		return fmt.Errorf("oto: context init timed out")
	}
	p.ctx = ctx
	return nil
}

// Play starts or resumes playback.
func (p *Player) Play() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.otoPlayer == nil {
		return fmt.Errorf("oto: no track loaded")
	}
	p.otoPlayer.Play()
	p.state = player.StatePlaying
	return nil
}

// Pause halts playback without releasing the track.
func (p *Player) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.otoPlayer == nil {
		return nil
	}
	p.otoPlayer.Pause()
	if p.state == player.StatePlaying {
		p.state = player.StatePaused
	}
	return nil
}

// Stop halts playback and releases the loaded track.
func (p *Player) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseLocked()
	p.state = player.StateStopped
	return nil
}

// SetVolume clamps vol into [0, 100] and applies it to the Oto player.
func (p *Player) SetVolume(vol int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	p.volume = vol
	if p.otoPlayer != nil {
		p.otoPlayer.SetVolume(float64(vol) / 100.0)
	}
	return nil
}

// Volume returns the current volume in [0, 100].
func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// Position returns an estimate of the current playback position derived
// from the number of PCM bytes consumed by Oto.
func (p *Player) Position() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return positionFromBytes(p.reader, p.sampleRate, p.channels)
}

// Seek moves playback to the given absolute position within the
// loaded track. The Oto backend has no native seek, so this method
// tears down the current Oto player, seeks the underlying source,
// and rebuilds the player. Playback state (playing vs paused) is
// preserved across the rebuild.
//
// position is clamped to [0, Duration]. Returns an error if no track
// is loaded or if the underlying source cannot seek.
func (p *Player) Seek(position time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.source == nil || p.otoPlayer == nil {
		return fmt.Errorf("oto: no track loaded")
	}

	wasPlaying := p.otoPlayer.IsPlaying()
	p.otoPlayer.Pause()
	_ = p.otoPlayer.Close()
	p.otoPlayer = nil

	sampleRate := p.source.SampleRate()
	if sampleRate == 0 {
		return fmt.Errorf("oto: source has no sample rate")
	}
	total := p.source.TotalSamples()
	max := time.Duration(total) * time.Second / time.Duration(sampleRate)
	if position < 0 {
		position = 0
	}
	if max > 0 && position > max {
		position = max
	}
	targetSample := int64(position) * int64(sampleRate) / int64(time.Second)

	if err := p.source.SeekSample(targetSample); err != nil {
		return fmt.Errorf("oto: source seek: %w", err)
	}

	// Seed the byte counter at the byte offset of the target sample
	// so Position() reflects the new position immediately after the
	// seek, rather than jumping back to 0 and only catching up once
	// the new Oto player has consumed enough samples.
	bytesPerFrame := int64(p.channels) * 2 // signed 16-bit LE output
	p.reader = &countingReader{
		src:       p.source,
		bytesRead: targetSample * bytesPerFrame,
	}

	otoPl := p.ctx.NewPlayer(p.reader)
	otoPl.SetVolume(float64(p.volume) / 100.0)
	p.otoPlayer = otoPl

	if wasPlaying {
		otoPl.Play()
		p.state = player.StatePlaying
	} else {
		otoPl.Pause()
		p.state = player.StatePaused
	}
	return nil
}

// Duration returns the total length of the loaded track, computed from
// the source's declared sample count and sample rate.
func (p *Player) Duration() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.source == nil || p.sampleRate == 0 {
		return 0
	}
	return time.Duration(p.source.TotalSamples()) * time.Second / time.Duration(p.sampleRate)
}

// State returns the high-level playback state.
//
// It consults the underlying Oto player so that natural end-of-track
// transitions (Oto internally pauses itself once the source hits EOF)
// are surfaced as StateStopped rather than left dangling as
// StatePlaying.
func (p *Player) State() player.State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.actualStateLocked()
}

// actualStateLocked computes the effective playback state. It must be
// called with p.mu held.
func (p *Player) actualStateLocked() player.State {
	if p.otoPlayer == nil {
		return player.StateStopped
	}
	if err := p.otoPlayer.Err(); err != nil {
		return player.StateStopped
	}
	if p.otoPlayer.IsPlaying() {
		return player.StatePlaying
	}
	// Oto says we are not playing. Distinguish user-initiated pause from
	// natural end-of-track: if our last user action was Play, the track
	// ended.
	if p.state == player.StatePlaying {
		return player.StateStopped
	}
	return p.state
}

// releaseLocked tears down the current source, reader and Oto player.
// It does not touch the shared audio context. It must be called with
// p.mu held.
func (p *Player) releaseLocked() {
	if p.otoPlayer != nil {
		// Pause first so the mux stops pulling samples from the
		// about-to-be-closed source. Close is a no-op since Oto v3.4
		// but is kept for forward compatibility.
		p.otoPlayer.Pause()
		_ = p.otoPlayer.Close()
		p.otoPlayer = nil
	}
	if p.source != nil {
		_ = p.source.Close()
		p.source = nil
	}
	p.reader = nil
	p.sampleRate = 0
	p.channels = 0
	p.state = player.StateStopped
}

// countingReader wraps a pcmSource and tracks the number of PCM bytes
// consumed by Oto so Position can be derived.
type countingReader struct {
	src       pcmSource
	bytesRead int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.src.ReadPCM(p)
	c.bytesRead += int64(n)
	return n, err
}

func positionFromBytes(cr *countingReader, sampleRate, channels int) time.Duration {
	if cr == nil || sampleRate == 0 || channels == 0 {
		return 0
	}
	bytesPerSecond := int64(sampleRate) * int64(channels) * 2 // 16-bit samples
	if bytesPerSecond == 0 {
		return 0
	}
	return time.Duration(cr.bytesRead) * time.Second / time.Duration(bytesPerSecond)
}

// Compile-time check that Player satisfies the player.Player interface.
var _ player.Player = (*Player)(nil)

// silenceUnusedImport keeps "io" available for future helpers without
// triggering an unused-import error on minimal builds.
var _ = io.EOF
