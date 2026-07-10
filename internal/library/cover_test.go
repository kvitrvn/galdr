package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindAlbumCover(t *testing.T) {
	dir := t.TempDir()
	track := filepath.Join(dir, "track.mp3")
	if got := FindAlbumCover(track); got != "" {
		t.Fatalf("missing cover = %q", got)
	}
	cover := filepath.Join(dir, "cover.png")
	if err := os.WriteFile(cover, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindAlbumCover(track); got != cover {
		t.Fatalf("cover = %q, want %q", got, cover)
	}
}

func TestFindAlbumCoverPrefersJPEG(t *testing.T) {
	dir := t.TempDir()
	track := filepath.Join(dir, "track.mp3")
	for _, name := range []string{"cover.png", "cover.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(dir, "cover.jpg")
	if got := FindAlbumCover(track); got != want {
		t.Fatalf("cover = %q, want %q", got, want)
	}
}
