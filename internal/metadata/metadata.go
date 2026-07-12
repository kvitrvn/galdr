// Package metadata extracts tags and durations for supported audio files
// (MP3, FLAC, WAV).
//
// The Read function dispatches on file extension. When tags are missing
// the returned Tags is empty but the error is nil; callers are expected
// to fall back to deriving a readable title from the filename
// (see internal/library.TitleFromPath).
//
// MP3 duration is intentionally not computed during the library scan. The
// playback backend obtains it efficiently from libmpv when a track is loaded.
package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrUnsupportedFormat is returned by Read when the file extension is
// not one of the supported audio formats (.mp3, .flac, .wav).
var ErrUnsupportedFormat = errors.New("metadata: unsupported format")

// Tags is the metadata extracted from a single audio file.
//
// Zero values are used for fields that the source file did not
// provide. The library layer fills in the Title from the filename
// when Tags.Title is empty.
type Tags struct {
	Title    string
	Artist   string
	Album    string
	Year     int
	Track    int
	Duration time.Duration
	Format   string // "mp3" | "flac" | "wav"
}

// Read extracts the tags (and, for FLAC and WAV, the duration) of the
// audio file at path.
//
// Behavior:
//   - Returns ErrUnsupportedFormat if the file extension is not one
//     of .mp3, .flac or .wav.
//   - Returns a non-nil error for I/O failures or unrecoverable
//     parse errors.
//   - Returns (Tags{}, nil) when the file is recognised as the right
//     format but no tags are present.
//
// The format-specific helpers never panic on corrupt or truncated
// files; they return a partial Tags with the fields they could read.
func Read(path string) (Tags, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return readMP3(path)
	case ".flac":
		return readFLAC(path)
	case ".wav":
		return readWAV(path)
	}
	return Tags{}, ErrUnsupportedFormat
}

// openForRead opens path for reading and returns the file. The caller
// is responsible for closing it. It is shared by the format-specific
// helpers.
func openForRead(path string) (*os.File, error) {
	return os.Open(path)
}
