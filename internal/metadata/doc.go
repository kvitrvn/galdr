// Package metadata extracts tags and durations for supported audio files
// (MP3, FLAC, WAV).
//
// When tags are missing, callers must fall back to deriving a readable
// title from the filename (see internal/library).
package metadata
