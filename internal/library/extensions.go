package library

import (
	"path/filepath"
	"strings"
)

// FormatFromPath returns the audio Format corresponding to the file extension
// of path. Matching is case-insensitive.
//
// The second return value reports whether the extension is one of the MVP
// supported formats (.mp3, .wav, .flac).
func FormatFromPath(path string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		return FormatMP3, true
	case ".wav":
		return FormatWAV, true
	case ".flac":
		return FormatFLAC, true
	default:
		return "", false
	}
}

// IsSupported reports whether path has an extension corresponding to one of
// the MVP supported audio formats.
func IsSupported(path string) bool {
	_, ok := FormatFromPath(path)
	return ok
}
