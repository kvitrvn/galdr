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
type Player struct {
	mu     sync.Mutex
	state  player.State
	volume int

	source     pcmSource
	reader     *countingReader
	otoPlayer  *oto.Player
	sampleRate int
	channels   int
}

// New returns a fresh Oto Player with the default volume of 100.
func New() *Player {
	return &Player{state: player.StateStopped, volume: 100}
}

// Load prepares the track at path for playback. The format is detected
// from the file extension via library.FormatFromPath.
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

	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   src.SampleRate(),
		ChannelCount: src.Channels(),
		Format:       oto.FormatSignedInt16LE,
	})
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("oto: new context: %w", err)
	}
	select {
	case <-ready:
		if cerr := ctx.Err(); cerr != nil {
			_ = src.Close()
			return fmt.Errorf("oto: context not ready: %w", cerr)
		}
	case <-time.After(2 * time.Second):
		_ = src.Close()
		return fmt.Errorf("oto: context init timed out")
	}

	cr := &countingReader{src: src}
	otoPl := ctx.NewPlayer(cr)
	otoPl.SetVolume(float64(p.volume) / 100.0)

	p.source = src
	p.reader = cr
	p.otoPlayer = otoPl
	p.sampleRate = src.SampleRate()
	p.channels = src.Channels()
	p.state = player.StateStopped
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
func (p *Player) State() player.State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// releaseLocked tears down the current source, reader and Oto player.
// It must be called with p.mu held.
func (p *Player) releaseLocked() {
	if p.otoPlayer != nil {
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
