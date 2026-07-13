package playlist

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestStoreSaveLoadRoundTrip(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Playlists")
	tracks := []string{
		filepath.Join(root, "Björk", "Été", "01.flac"),
		filepath.Join(root, "Artist", "Album", "#02.mp3"),
		filepath.Join(root, "Björk", "Été", "01.flac"),
	}
	store, err := New(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("Dimanche d’été", tracks, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Dimanche d’été.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	wantBytes := "#EXTM3U\n../Björk/Été/01.flac\n../Artist/Album/#02.mp3\n../Björk/Été/01.flac\n"
	if string(data) != wantBytes {
		t.Fatalf("saved M3U8 = %q, want %q", data, wantBytes)
	}

	result, err := store.Load("Dimanche d’été")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Paths, tracks) || len(result.Skipped) != 0 {
		t.Fatalf("Load = %#v, want paths %#v", result, tracks)
	}
}

func TestStoreListSortsAndIgnoresNonPlaylists(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Playlists")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zebra.m3u8", "Alpha.M3U8", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#EXTM3U\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "ignored.m3u8"), 0o755); err != nil {
		t.Fatal(err)
	}
	store, _ := New(root, dir)
	names, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Alpha", "zebra"}; !slices.Equal(names, want) {
		t.Fatalf("List = %v, want %v", names, want)
	}
}

func TestStoreRejectsInvalidNames(t *testing.T) {
	store, _ := New(t.TempDir(), filepath.Join(t.TempDir(), "Playlists"))
	tests := []struct {
		name string
	}{
		{name: ""},
		{name: " leading"},
		{name: "trailing "},
		{name: ".hidden"},
		{name: "../escape"},
		{name: "nested/name"},
		{name: "already.m3u8"},
		{name: "line\nbreak"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.name, "\n", "newline"), func(t *testing.T) {
			if err := store.Save(tt.name, nil, false); !errors.Is(err, ErrInvalidName) {
				t.Fatalf("Save(%q) error = %v, want ErrInvalidName", tt.name, err)
			}
		})
	}
}

func TestStoreRequiresExplicitOverwrite(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Playlists")
	store, _ := New(root, dir)
	first := filepath.Join(root, "first.mp3")
	second := filepath.Join(root, "second.mp3")
	if err := store.Save("mix", []string{first}, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("mix", []string{second}, false); !errors.Is(err, ErrExists) {
		t.Fatalf("second Save error = %v, want ErrExists", err)
	}
	result, err := store.Load("mix")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(result.Paths, []string{first}) {
		t.Fatalf("non-overwrite changed playlist to %v", result.Paths)
	}
	if err := store.Save("mix", []string{second}, true); err != nil {
		t.Fatal(err)
	}
	result, err = store.Load("mix")
	if err != nil || !slices.Equal(result.Paths, []string{second}) {
		t.Fatalf("overwrite Load = %#v, %v", result, err)
	}
}

func TestStoreOverwriteUsesExistingCaseInsensitiveName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Playlists")
	store, _ := New(root, dir)
	if err := store.Save("Mix", []string{filepath.Join(root, "first.mp3")}, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("mix", []string{filepath.Join(root, "second.mp3")}, false); !errors.Is(err, ErrExists) {
		t.Fatalf("case collision error = %v, want ErrExists", err)
	}
	if err := store.Save("mix", []string{filepath.Join(root, "second.mp3")}, true); err != nil {
		t.Fatal(err)
	}
	names, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{"Mix"}) {
		t.Fatalf("names after overwrite = %v, want [Mix]", names)
	}
	result, err := store.Load("Mix")
	if err != nil || filepath.Base(result.Paths[0]) != "second.mp3" {
		t.Fatalf("overwritten playlist = %#v, %v", result, err)
	}
}

func TestStoreLoadSkipsEntriesOutsideMusicDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Playlists")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "\ufeff#EXTM3U\n../inside.mp3\n../../outside.mp3\n# comment\n../inside.mp3\n"
	if err := os.WriteFile(filepath.Join(dir, "edited.m3u8"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _ := New(root, dir)
	result, err := store.Load("edited")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "inside.mp3"), filepath.Join(root, "inside.mp3")}
	if !slices.Equal(result.Paths, want) || len(result.Skipped) != 1 || result.Skipped[0].Reason != SkipOutside {
		t.Fatalf("Load = %#v, want %v and one outside skip", result, want)
	}
}

func TestStoreSaveRejectsTrackOutsideMusicDirectory(t *testing.T) {
	root := t.TempDir()
	store, _ := New(root, filepath.Join(root, "Playlists"))
	err := store.Save("escape", []string{filepath.Join(filepath.Dir(root), "outside.mp3")}, false)
	if err == nil || !strings.Contains(err.Error(), "outside music directory") {
		t.Fatalf("Save outside error = %v", err)
	}
}

func TestStoreRejectsSymlinkEscapingMusicDirectory(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.mp3")
	if err := os.WriteFile(outside, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.mp3")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	store, _ := New(root, filepath.Join(root, "Playlists"))
	if err := store.Save("escape", []string{link}, false); err == nil || !strings.Contains(err.Error(), "outside music directory") {
		t.Fatalf("Save symlink error = %v", err)
	}

	dir := filepath.Join(root, "Playlists")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "edited.m3u8"), []byte("#EXTM3U\n../linked.mp3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := store.Load("edited")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Paths) != 0 || len(result.Skipped) != 1 || result.Skipped[0].Reason != SkipOutside {
		t.Fatalf("Load symlink escape = %#v", result)
	}
}

func TestStoreRelativeEntriesSurviveMovingLibrary(t *testing.T) {
	firstRoot := t.TempDir()
	firstDir := filepath.Join(firstRoot, "Playlists")
	first, _ := New(firstRoot, firstDir)
	if err := first.Save("portable", []string{filepath.Join(firstRoot, "Artist", "track.flac")}, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(firstDir, "portable.m3u8"))
	if err != nil {
		t.Fatal(err)
	}

	secondRoot := t.TempDir()
	secondDir := filepath.Join(secondRoot, "Playlists")
	if err := os.MkdirAll(secondDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secondDir, "portable.m3u8"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	second, _ := New(secondRoot, secondDir)
	result, err := second.Load("portable")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(secondRoot, "Artist", "track.flac")}
	if !slices.Equal(result.Paths, want) {
		t.Fatalf("moved paths = %v, want %v", result.Paths, want)
	}
}
