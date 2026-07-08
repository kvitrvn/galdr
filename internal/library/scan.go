package library

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kvitrvn/galdr/internal/metadata"
)

// Scan walks root recursively and returns the audio tracks it discovers.
//
// Behavior:
//   - Only files with extensions supported by the MVP (.mp3, .wav, .flac)
//     are returned. Matching is case-insensitive.
//   - Hidden entries (names starting with ".") are skipped; hidden
//     directories are not descended into.
//   - Unreadable entries are skipped; the scanner does not panic.
//   - Results are sorted by path to give a stable, predictable ordering
//     regardless of the underlying filesystem.
//   - Real metadata (artist, album, duration) is filled in by
//     internal/metadata. The Title field falls back to the filename
//     when the tag has no title (see TitleFromPath).
//
// Scan returns an error if root does not exist or is not a directory.
// Per-file metadata errors are NOT fatal: a corrupt or unreadable file
// is included in the result with a filename-derived title and the
// remaining tracks are still returned.
func Scan(root string) ([]Track, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("library: scan root %q is not a directory", root)
	}

	var tracks []Track
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		format, ok := FormatFromPath(path)
		if !ok {
			return nil
		}
		tracks = append(tracks, enrich(path, format))
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(tracks, func(i, j int) bool {
		return tracks[i].Path < tracks[j].Path
	})
	return tracks, nil
}

// enrich builds a Track for the given file path, populating metadata
// when available. A metadata read failure is swallowed: the track is
// still returned with a filename-derived title and zero values
// elsewhere. The caller (Scan) collects all such tracks into a stable
// ordering regardless of which files were successfully enriched.
func enrich(path string, format Format) Track {
	t := Track{
		Path:   path,
		Title:  TitleFromPath(path),
		Format: format,
	}
	tags, err := metadata.Read(path)
	if err != nil {
		return t
	}
	if tags.Title != "" {
		t.Title = tags.Title
	}
	t.Artist = tags.Artist
	t.Album = tags.Album
	t.Track = tags.Track
	t.Duration = tags.Duration
	return t
}
