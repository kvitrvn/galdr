package library

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
}

func TestScan_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 0 {
		t.Errorf("len(tracks) = %d, want 0", len(tracks))
	}
}

func TestScan_FindsSupportedFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]Format{
		"a.mp3":       FormatMP3,
		"b.WAV":       FormatWAV,
		"c.Flac":      FormatFLAC,
		"notes.txt":   "",
		"picture.jpg": "",
		"archive.ogg": "",
	}
	for name := range files {
		writeFile(t, filepath.Join(dir, name))
	}

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("len(tracks) = %d, want 3 (got %+v)", len(tracks), tracks)
	}

	got := make(map[string]Format, len(tracks))
	for _, tr := range tracks {
		got[tr.Title] = tr.Format
	}
	want := map[string]Format{"a": FormatMP3, "b": FormatWAV, "c": FormatFLAC}
	for title, format := range want {
		if got[title] != format {
			t.Errorf("track %q format = %q, want %q", title, got[title], format)
		}
	}
}

func TestScan_SkipsHiddenFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "visible.mp3"))
	writeFile(t, filepath.Join(dir, ".hidden.mp3"))

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1 (got %+v)", len(tracks), tracks)
	}
	if tracks[0].Title != "visible" {
		t.Errorf("Title = %q, want %q", tracks[0].Title, "visible")
	}
}

func TestScan_SkipsHiddenDirectories(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, ".hidden"))
	writeFile(t, filepath.Join(dir, ".hidden", "inside.mp3"))
	writeFile(t, filepath.Join(dir, "top.mp3"))

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1 (got %+v)", len(tracks), tracks)
	}
	if tracks[0].Title != "top" {
		t.Errorf("Title = %q, want %q", tracks[0].Title, "top")
	}
}

func TestScan_Recursive(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "sub"))
	mkdir(t, filepath.Join(dir, "sub", "deeper"))
	writeFile(t, filepath.Join(dir, "root.mp3"))
	writeFile(t, filepath.Join(dir, "sub", "mid.flac"))
	writeFile(t, filepath.Join(dir, "sub", "deeper", "deep.wav"))
	writeFile(t, filepath.Join(dir, "sub", "ignored.txt"))

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 3 {
		t.Fatalf("len(tracks) = %d, want 3 (got %+v)", len(tracks), tracks)
	}
	titles := []string{tracks[0].Title, tracks[1].Title, tracks[2].Title}
	want := []string{"root", "deep", "mid"}
	for i, w := range want {
		if titles[i] != w {
			t.Errorf("tracks[%d].Title = %q, want %q", i, titles[i], w)
		}
	}
}

func TestScan_StableOrdering(t *testing.T) {
	dir := t.TempDir()
	files := []string{"c.mp3", "a.mp3", "b.mp3"}
	for _, f := range files {
		writeFile(t, filepath.Join(dir, f))
	}

	first, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan (1st): %v", err)
	}
	second, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan (2nd): %v", err)
	}

	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("expected 3 tracks each, got %d / %d", len(first), len(second))
	}

	want := []string{"a", "b", "c"}
	for i, w := range want {
		if first[i].Title != w || second[i].Title != w {
			t.Errorf("position %d: first=%q second=%q, want %q",
				i, first[i].Title, second[i].Title, w)
		}
	}

	firstPaths := titles(first)
	secondPaths := titles(second)
	sortedCopy := append([]string(nil), firstPaths...)
	sort.Strings(sortedCopy)
	for i := range firstPaths {
		if firstPaths[i] != sortedCopy[i] {
			t.Errorf("ordering is not sorted at %d: %v", i, firstPaths)
			break
		}
	}
	_ = secondPaths
}

func titles(tracks []Track) []string {
	out := make([]string, len(tracks))
	for i, tr := range tracks {
		out[i] = tr.Title
	}
	return out
}

func TestScan_NonExistentRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	tracks, err := Scan(missing)
	if err == nil {
		t.Fatalf("expected error for missing root, got %d tracks", len(tracks))
	}
}

func TestScan_RootIsFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "thing.mp3")
	writeFile(t, file)

	tracks, err := Scan(file)
	if err == nil {
		t.Fatalf("expected error when root is a file, got %d tracks", len(tracks))
	}
}

func TestScan_TitleFallbackFromFilename(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "My Cool Song.mp3"))

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 1 {
		t.Fatalf("len(tracks) = %d, want 1", len(tracks))
	}
	if tracks[0].Title != "My Cool Song" {
		t.Errorf("Title = %q, want %q", tracks[0].Title, "My Cool Song")
	}
	if tracks[0].Artist != "" || tracks[0].Album != "" {
		t.Errorf("expected empty Artist/Album from filename fallback, got %+v", tracks[0])
	}
}

func TestScan_SkipsUnreadableFileGracefully(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: unreadable permission tests are not reliable")
	}
	dir := t.TempDir()
	good := filepath.Join(dir, "good.mp3")
	bad := filepath.Join(dir, "bad.mp3")
	writeFile(t, good)
	writeFile(t, bad)
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan returned error: %v (want graceful skip)", err)
	}
	// Either we got both files (the OS allowed reading despite chmod) or only the
	// good one. Either way the scanner must not panic and must include good.mp3.
	found := false
	for _, tr := range tracks {
		if tr.Path == good {
			found = true
		}
	}
	if !found {
		t.Errorf("expected good.mp3 in results, got %+v", tracks)
	}
}
