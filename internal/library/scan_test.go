package library

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kvitrvn/galdr/internal/metadatatest"
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

func TestScan_PopulatesMetadata(t *testing.T) {
	dir := t.TempDir()

	mp3 := filepath.Join(dir, "tagged.mp3")
	metadatatest.WriteMP3(t, mp3, "Anthem", "Helloween", "Keeper of the Seven Keys", 1987, 1)

	flac := filepath.Join(dir, "tagged.flac")
	metadatatest.WriteFLAC(t, flac, "Hallowed Be Thy Name", "Iron Maiden", "The Number of the Beast", 1982, 9, 88200)

	wav := filepath.Join(dir, "tagged.wav")
	metadatatest.WriteWAV(t, wav, "Phantom of the Opera", "Iron Maiden", "Iron Maiden", 1980, 1, 88200)

	untagged := filepath.Join(dir, "plain.mp3")
	metadatatest.WriteMP3(t, untagged, "", "", "", 0, 0)

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 4 {
		t.Fatalf("len(tracks) = %d, want 4 (got %+v)", len(tracks), tracks)
	}

	byPath := make(map[string]Track, len(tracks))
	for _, tr := range tracks {
		byPath[tr.Path] = tr
	}

	if got := byPath[mp3]; got.Title != "Anthem" || got.Artist != "Helloween" || got.Album != "Keeper of the Seven Keys" {
		t.Errorf("MP3 enrichment failed: %+v", got)
	}
	if got := byPath[flac]; got.Title != "Hallowed Be Thy Name" || got.Artist != "Iron Maiden" {
		t.Errorf("FLAC enrichment failed: %+v", got)
	}
	if byPath[flac].Duration == 0 {
		t.Errorf("FLAC duration should be non-zero, got 0")
	}
	if got := byPath[wav]; got.Title != "Phantom of the Opera" || got.Artist != "Iron Maiden" {
		t.Errorf("WAV enrichment failed: %+v", got)
	}
	if byPath[wav].Duration == 0 {
		t.Errorf("WAV duration should be non-zero, got 0")
	}
	// Untagged MP3 should fall back to the filename.
	if got := byPath[untagged]; got.Title != "plain" || got.Artist != "" {
		t.Errorf("untagged MP3 fallback failed: %+v", got)
	}
}

func TestScan_CorruptFileContinues(t *testing.T) {
	dir := t.TempDir()

	good := filepath.Join(dir, "good.mp3")
	metadatatest.WriteMP3(t, good, "Anthem", "Helloween", "Keeper", 1987, 1)

	corrupt := filepath.Join(dir, "corrupt.mp3")
	if err := os.WriteFile(corrupt, []byte("not actually an mp3"), 0o644); err != nil {
		t.Fatal(err)
	}

	tracks, err := Scan(dir)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2 (got %+v)", len(tracks), tracks)
	}
	found := false
	for _, tr := range tracks {
		if tr.Path == good && tr.Title == "Anthem" {
			found = true
		}
	}
	if !found {
		t.Errorf("good track missing or unenriched in result: %+v", tracks)
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
