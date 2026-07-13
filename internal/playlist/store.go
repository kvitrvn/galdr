package playlist

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const extension = ".m3u8"

var (
	ErrInvalidName = errors.New("playlist: invalid name")
	ErrExists      = errors.New("playlist: already exists")
	ErrNotFound    = errors.New("playlist: not found")
)

// SkipReason explains why an M3U8 entry was not returned by Load.
type SkipReason string

const (
	SkipMalformed SkipReason = "malformed"
	SkipOutside   SkipReason = "outside music directory"
)

// SkippedEntry records one ignored line without making a partially valid
// playlist unusable.
type SkippedEntry struct {
	Line   int
	Entry  string
	Reason SkipReason
}

// LoadResult is the ordered set of accepted paths and any rejected entries.
type LoadResult struct {
	Paths   []string
	Skipped []SkippedEntry
}

// Store owns one playlist directory and confines every loaded track to root.
type Store struct {
	root string
	dir  string
}

// New constructs a store. Relative inputs are converted to absolute paths so
// later containment checks do not depend on the process working directory.
func New(root, dir string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("playlist: empty music directory")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("playlist: resolve music directory: %w", err)
	}
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join(root, "Playlists")
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("playlist: resolve playlist directory: %w", err)
	}
	return &Store{root: filepath.Clean(root), dir: filepath.Clean(dir)}, nil
}

// Dir returns the resolved directory containing playlist files.
func (s *Store) Dir() string { return s.dir }

// List returns playlist display names in case-insensitive lexical order.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("playlist: list %s: %w", s.dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), extension) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		names = append(names, strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())))
	}
	sort.Slice(names, func(i, j int) bool {
		left, right := strings.ToLower(names[i]), strings.ToLower(names[j])
		if left == right {
			return names[i] < names[j]
		}
		return left < right
	})
	return names, nil
}

// Save atomically writes paths in queue order. Existing files are preserved
// unless overwrite is explicitly true.
func (s *Store) Save(name string, paths []string, overwrite bool) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if collision, err := s.collidingName(name); err != nil {
		return err
	} else if collision != "" {
		if !overwrite {
			return fmt.Errorf("%w: %s", ErrExists, collision)
		}
		path, err = s.path(collision)
		if err != nil {
			return err
		}
	}

	var contents strings.Builder
	contents.WriteString("#EXTM3U\n")
	for _, trackPath := range paths {
		entry, err := s.encodePath(trackPath)
		if err != nil {
			return err
		}
		contents.WriteString(entry)
		contents.WriteByte('\n')
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("playlist: create directory %s: %w", s.dir, err)
	}
	tmp, err := os.CreateTemp(s.dir, ".galdr-playlist-*")
	if err != nil {
		return fmt.Errorf("playlist: create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if err := tmp.Chmod(0o644); err != nil {
		cleanup()
		return fmt.Errorf("playlist: set temporary file mode: %w", err)
	}
	if _, err := io.WriteString(tmp, contents.String()); err != nil {
		cleanup()
		return fmt.Errorf("playlist: write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("playlist: sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("playlist: close temporary file: %w", err)
	}
	if !overwrite {
		if _, err := os.Lstat(path); err == nil {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("%w: %s", ErrExists, name)
		} else if !errors.Is(err, os.ErrNotExist) {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("playlist: inspect destination: %w", err)
		}
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("playlist: replace %s: %w", path, err)
	}
	return nil
}

// Load decodes one M3U8 file. Invalid entries are reported individually while
// valid entries retain their original order and duplicates.
func (s *Store) Load(name string) (LoadResult, error) {
	path, err := s.path(name)
	if err != nil {
		return LoadResult{}, err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return LoadResult{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return LoadResult{}, fmt.Errorf("playlist: inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return LoadResult{}, fmt.Errorf("playlist: %s is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return LoadResult{}, fmt.Errorf("playlist: open %s: %w", path, err)
	}
	defer file.Close()

	var result LoadResult
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		entry := scanner.Text()
		if line == 1 {
			entry = strings.TrimPrefix(entry, "\ufeff")
		}
		if entry == "" || strings.HasPrefix(entry, "#") {
			continue
		}
		if !utf8.ValidString(entry) || strings.IndexByte(entry, 0) >= 0 {
			result.Skipped = append(result.Skipped, SkippedEntry{Line: line, Entry: entry, Reason: SkipMalformed})
			continue
		}
		resolved := filepath.FromSlash(entry)
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(s.dir, resolved)
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil || !s.withinRoot(resolved) {
			result.Skipped = append(result.Skipped, SkippedEntry{Line: line, Entry: entry, Reason: SkipOutside})
			continue
		}
		result.Paths = append(result.Paths, filepath.Clean(resolved))
	}
	if err := scanner.Err(); err != nil {
		return LoadResult{}, fmt.Errorf("playlist: read %s: %w", path, err)
	}
	return result, nil
}

func (s *Store) encodePath(path string) (string, error) {
	if strings.ContainsAny(path, "\r\n\x00") {
		return "", fmt.Errorf("playlist: path cannot be represented in M3U8: %q", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("playlist: resolve track %s: %w", path, err)
	}
	abs = filepath.Clean(abs)
	if !s.withinRoot(abs) {
		return "", fmt.Errorf("playlist: track outside music directory: %s", path)
	}
	rel, err := filepath.Rel(s.dir, abs)
	if err != nil {
		return "", fmt.Errorf("playlist: make track relative: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "#") {
		rel = "./" + rel
	}
	return rel, nil
}

func (s *Store) path(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.dir, name+extension), nil
}

func (s *Store) collidingName(name string) (string, error) {
	names, err := s.List()
	if err != nil {
		return "", err
	}
	for _, existing := range names {
		if strings.EqualFold(existing, name) {
			return existing, nil
		}
	}
	return "", nil
}

func validateName(name string) error {
	if name == "" || name != strings.TrimSpace(name) || name == "." || name == ".." || utf8.RuneCountInString(name) > 128 {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	if strings.HasPrefix(name, ".") || strings.EqualFold(filepath.Ext(name), extension) {
		return fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	for _, r := range name {
		if r == '/' || r == '\\' || unicode.IsControl(r) {
			return fmt.Errorf("%w: %q", ErrInvalidName, name)
		}
	}
	return nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func (s *Store) withinRoot(path string) bool {
	if !within(s.root, path) {
		return false
	}
	realRoot, rootErr := filepath.EvalSymlinks(s.root)
	realPath, pathErr := filepath.EvalSymlinks(path)
	if rootErr != nil || pathErr != nil {
		// Missing entries are handled by the application catalogue. The lexical
		// check still prevents them from naming a location outside the library.
		return true
	}
	return within(realRoot, realPath)
}
