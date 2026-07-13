package playlist

import (
	"bufio"
	"bytes"
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
	ErrInvalidName   = errors.New("playlist: invalid name")
	ErrExists        = errors.New("playlist: already exists")
	ErrNotFound      = errors.New("playlist: not found")
	ErrTrackNotFound = errors.New("playlist: track occurrence not found")
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

type writePolicy int

const (
	createOnly writePolicy = iota
	replaceRegular
)

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
	policy := createOnly
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
		policy = replaceRegular
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

	return s.writeAtomic(path, contents.String(), policy)
}

// Append adds one path occurrence to the end of an existing playlist without
// parsing or rewriting its current lines. Comments, ignored entries and their
// byte order are preserved exactly.
func (s *Store) Append(name, trackPath string) error {
	return s.AppendMany(name, []string{trackPath})
}

// AppendMany adds path occurrences to the end of an existing playlist in one
// atomic replacement. The supplied order and duplicates are retained.
func (s *Store) AppendMany(name string, trackPaths []string) error {
	if len(trackPaths) == 0 {
		return errors.New("playlist: no tracks to append")
	}
	path, contents, err := s.readExisting(name)
	if err != nil {
		return err
	}
	entries := make([]string, 0, len(trackPaths))
	entriesSize := 0
	for _, trackPath := range trackPaths {
		entry, err := s.encodePath(trackPath)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		entriesSize += len(entry) + 1
	}

	var updated strings.Builder
	updated.Grow(len(contents) + entriesSize + 1)
	updated.Write(contents)
	if len(contents) > 0 && contents[len(contents)-1] != '\n' {
		updated.WriteByte('\n')
	}
	for _, entry := range entries {
		updated.WriteString(entry)
		updated.WriteByte('\n')
	}

	return s.writeAtomic(path, updated.String(), replaceRegular)
}

// RemoveOccurrence removes the zero-based matching occurrence of trackPath
// from an existing playlist while preserving every other byte and line.
func (s *Store) RemoveOccurrence(name, trackPath string, occurrence int) error {
	if occurrence < 0 {
		return fmt.Errorf("%w: %d", ErrTrackNotFound, occurrence)
	}
	if _, err := s.encodePath(trackPath); err != nil {
		return err
	}
	target, err := filepath.Abs(trackPath)
	if err != nil {
		return fmt.Errorf("playlist: resolve track %s: %w", trackPath, err)
	}
	target = filepath.Clean(target)

	path, contents, err := s.readExisting(name)
	if err != nil {
		return err
	}
	var updated strings.Builder
	updated.Grow(len(contents))
	matched := 0
	removed := false
	lineStart := 0
	line := 0
	for lineStart < len(contents) {
		line++
		entryEnd := len(contents)
		segmentEnd := len(contents)
		if newline := bytes.IndexByte(contents[lineStart:], '\n'); newline >= 0 {
			entryEnd = lineStart + newline
			segmentEnd = entryEnd + 1
		}
		entry := string(contents[lineStart:entryEnd])
		resolved, accepted := s.resolveStoredPath(entry, line == 1)
		isMatch := accepted && resolved == target
		remove := isMatch && matched == occurrence
		if isMatch {
			matched++
		}
		if remove {
			removed = true
		} else {
			updated.Write(contents[lineStart:segmentEnd])
		}
		lineStart = segmentEnd
	}
	if !removed {
		return fmt.Errorf(
			"%w: %s occurrence %d",
			ErrTrackNotFound,
			trackPath,
			occurrence,
		)
	}
	return s.writeAtomic(path, updated.String(), replaceRegular)
}

func (s *Store) readExisting(name string) (string, []byte, error) {
	existing, err := s.collidingName(name)
	if err != nil {
		return "", nil, err
	}
	if existing == "" {
		return "", nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	path, err := s.path(existing)
	if err != nil {
		return "", nil, err
	}
	if err := requireRegularFile(path, existing); err != nil {
		return "", nil, err
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return "", nil, fmt.Errorf("playlist: open directory %s: %w", s.dir, err)
	}
	file, err := root.Open(filepath.Base(path))
	if err != nil {
		closeErr := root.Close()
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, errors.Join(
				fmt.Errorf("%w: %s", ErrNotFound, existing),
				wrapCloseError(s.dir, closeErr),
			)
		}
		return "", nil, fmt.Errorf(
			"playlist: open %s: %w",
			path,
			errors.Join(err, wrapCloseError(s.dir, closeErr)),
		)
	}
	contents, readErr := io.ReadAll(file)
	closeErr := errors.Join(file.Close(), root.Close())
	if readErr != nil {
		return "", nil, fmt.Errorf("playlist: read %s: %w", path, readErr)
	}
	if closeErr != nil {
		return "", nil, fmt.Errorf("playlist: close %s: %w", path, closeErr)
	}
	return path, contents, nil
}

func (s *Store) resolveStoredPath(entry string, first bool) (string, bool) {
	if first {
		entry = strings.TrimPrefix(entry, "\ufeff")
	}
	if entry == "" || strings.HasPrefix(entry, "#") {
		return "", false
	}
	if !utf8.ValidString(entry) || strings.IndexByte(entry, 0) >= 0 {
		return "", false
	}
	resolved := filepath.FromSlash(entry)
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(s.dir, resolved)
	}
	resolved, err := filepath.Abs(resolved)
	if err != nil || !s.withinRoot(resolved) {
		return "", false
	}
	return filepath.Clean(resolved), true
}

func wrapCloseError(path string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("close %s: %w", path, err)
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
	if err := validateName(name); err != nil {
		return "", err
	}
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

func (s *Store) writeAtomic(path, contents string, policy writePolicy) error {
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
	if _, err := io.WriteString(tmp, contents); err != nil {
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
	if err := checkDestination(path, policy); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("playlist: replace %s: %w", path, err)
	}
	return nil
}

func checkDestination(path string, policy writePolicy) error {
	info, err := os.Lstat(path)
	switch policy {
	case createOnly:
		if err == nil {
			return fmt.Errorf("%w: %s", ErrExists, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("playlist: inspect destination: %w", err)
		}
		return nil
	case replaceRegular:
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrNotFound, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		}
		if err != nil {
			return fmt.Errorf("playlist: inspect destination: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("playlist: %s is not a regular file", path)
		}
		return nil
	default:
		return errors.New("playlist: invalid write policy")
	}
}

func requireRegularFile(path, name string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err != nil {
		return fmt.Errorf("playlist: inspect %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("playlist: %s is not a regular file", path)
	}
	return nil
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
