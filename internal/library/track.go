package library

import "time"

// Format identifies the audio container of a Track.
type Format string

const (
	FormatMP3  Format = "mp3"
	FormatWAV  Format = "wav"
	FormatFLAC Format = "flac"
)

// String returns the format as a lowercase string.
func (f Format) String() string { return string(f) }

// Track represents a single audio file with optional metadata.
//
// A Track is independent from the TUI and the audio backend: it is the
// domain value produced by the scanner (internal/library) and consumed by
// the app layer (internal/app).
type Track struct {
	Path     string
	Title    string
	Artist   string
	Album    string
	Duration time.Duration
	Format   Format
}
