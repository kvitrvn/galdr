package library

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
//   - Real metadata (artist, album, duration) is left empty; it will be
//     filled in by internal/metadata in a later phase. The Title field is
//     populated from the filename as a fallback (see TitleFromPath).
//
// Scan returns an error if root does not exist or is not a directory.
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
		tracks = append(tracks, Track{
			Path:   path,
			Title:  TitleFromPath(path),
			Format: format,
		})
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
