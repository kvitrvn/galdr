package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kvitrvn/galdr/internal/metadatatest"
)

const (
	testTitle  = "Hallowed Be Thy Name"
	testArtist = "Iron Maiden"
	testAlbum  = "The Number of the Beast"
	testYear   = 1982
	testTrack  = 9
)

func TestRead_MP3_ID3v2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.mp3")
	metadatatest.WriteMP3(t, path, testTitle, testArtist, testAlbum, testYear, testTrack)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != testTitle {
		t.Errorf("Title = %q, want %q", got.Title, testTitle)
	}
	if got.Artist != testArtist {
		t.Errorf("Artist = %q, want %q", got.Artist, testArtist)
	}
	if got.Album != testAlbum {
		t.Errorf("Album = %q, want %q", got.Album, testAlbum)
	}
	if got.Year != testYear {
		t.Errorf("Year = %d, want %d", got.Year, testYear)
	}
	if got.Track != testTrack {
		t.Errorf("Track = %d, want %d", got.Track, testTrack)
	}
	if got.Format != "mp3" {
		t.Errorf("Format = %q, want %q", got.Format, "mp3")
	}
}

func TestRead_MP3_NoTags_FallsBackToEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notags.mp3")
	metadatatest.WriteMP3NoTags(t, path)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != "" || got.Artist != "" || got.Album != "" {
		t.Errorf("expected empty tags for tagless file, got %+v", got)
	}
	if got.Format != "mp3" {
		t.Errorf("Format = %q, want %q", got.Format, "mp3")
	}
}

func TestRead_MP3_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.mp3")
	_, err := Read(path)
	if err == nil {
		t.Fatal("Read missing file: expected error, got nil")
	}
}

func TestRead_MP3_CorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.mp3")
	if err := os.WriteFile(path, []byte("not really an mp3"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	// dhowden/tag may either succeed with empty tags (no ID3 detected)
	// or fail with a parse error. Either is acceptable as long as the
	// scan caller can still fall back to the filename.
	if err != nil {
		return
	}
	if got.Title != "" {
		t.Errorf("Title for corrupt file = %q, want empty", got.Title)
	}
}

func TestRead_FLAC_Vorbis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.flac")
	metadatatest.WriteFLAC(t, path, testTitle, testArtist, testAlbum, testYear, testTrack, 88200)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != testTitle {
		t.Errorf("Title = %q, want %q", got.Title, testTitle)
	}
	if got.Artist != testArtist {
		t.Errorf("Artist = %q, want %q", got.Artist, testArtist)
	}
	if got.Album != testAlbum {
		t.Errorf("Album = %q, want %q", got.Album, testAlbum)
	}
	if got.Year != testYear {
		t.Errorf("Year = %d, want %d", got.Year, testYear)
	}
	if got.Track != testTrack {
		t.Errorf("Track = %d, want %d", got.Track, testTrack)
	}
	if got.Format != "flac" {
		t.Errorf("Format = %q, want %q", got.Format, "flac")
	}
	if want := 2 * time.Second; got.Duration != want {
		t.Errorf("Duration = %v, want %v", got.Duration, want)
	}
}

func TestRead_FLAC_NoTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notags.flac")
	metadatatest.WriteFLAC(t, path, "", "", "", 0, 0, 44100)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != "" {
		t.Errorf("Title for tagless FLAC = %q, want empty", got.Title)
	}
	if got.Format != "flac" {
		t.Errorf("Format = %q, want %q", got.Format, "flac")
	}
}

func TestRead_WAV_RIFF_INFO(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.wav")
	metadatatest.WriteWAV(t, path, testTitle, testArtist, testAlbum, testYear, testTrack, 88200)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != testTitle {
		t.Errorf("Title = %q, want %q", got.Title, testTitle)
	}
	if got.Artist != testArtist {
		t.Errorf("Artist = %q, want %q", got.Artist, testArtist)
	}
	if got.Album != testAlbum {
		t.Errorf("Album = %q, want %q", got.Album, testAlbum)
	}
	if got.Year != testYear {
		t.Errorf("Year = %d, want %d", got.Year, testYear)
	}
	if got.Track != testTrack {
		t.Errorf("Track = %d, want %d", got.Track, testTrack)
	}
	if got.Format != "wav" {
		t.Errorf("Format = %q, want %q", got.Format, "wav")
	}
	if want := time.Second; got.Duration != want {
		t.Errorf("Duration = %v, want %v", got.Duration, want)
	}
}

func TestRead_WAV_NoTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notags.wav")
	metadatatest.WriteWAV(t, path, "", "", "", 0, 0, 44100)

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Title != "" {
		t.Errorf("Title for tagless WAV = %q, want empty", got.Title)
	}
	if got.Format != "wav" {
		t.Errorf("Format = %q, want %q", got.Format, "wav")
	}
}

func TestRead_UnsupportedExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "track.ogg")
	if err := os.WriteFile(path, []byte("anything"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Read(path)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("Read ogg: err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestRead_MissingExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "noext")
	if err := os.WriteFile(path, []byte("anything"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Read(path)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("Read no-ext: err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestParseLeadingInt(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"2024", 2024, true},
		{"2024-05-21", 2024, true},
		{"3", 3, true},
		{"3/12", 3, true},
		{"  9", 9, true},
		{"", 0, false},
		{"abc", 0, false},
		{"0", 0, true},
	}
	for _, c := range cases {
		got, ok := parseLeadingInt(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("parseLeadingInt(%q) = (%d, %v), want (%d, %v)",
				c.in, got, ok, c.want, c.ok)
		}
	}
}
