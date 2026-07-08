package library

import (
	"testing"
	"time"
)

func TestFormat_String(t *testing.T) {
	cases := []struct {
		in   Format
		want string
	}{
		{FormatMP3, "mp3"},
		{FormatWAV, "wav"},
		{FormatFLAC, "flac"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Format(%q).String() = %q, want %q", string(c.in), got, c.want)
		}
	}
}

func TestTrack_Fields(t *testing.T) {
	tr := Track{
		Path:     "/music/song.mp3",
		Title:    "Song",
		Artist:   "Artist",
		Album:    "Album",
		Duration: 3 * time.Minute,
		Format:   FormatMP3,
	}

	if tr.Path != "/music/song.mp3" {
		t.Errorf("Path = %q, want %q", tr.Path, "/music/song.mp3")
	}
	if tr.Title != "Song" {
		t.Errorf("Title = %q, want %q", tr.Title, "Song")
	}
	if tr.Artist != "Artist" {
		t.Errorf("Artist = %q, want %q", tr.Artist, "Artist")
	}
	if tr.Album != "Album" {
		t.Errorf("Album = %q, want %q", tr.Album, "Album")
	}
	if tr.Duration != 3*time.Minute {
		t.Errorf("Duration = %v, want %v", tr.Duration, 3*time.Minute)
	}
	if tr.Format != FormatMP3 {
		t.Errorf("Format = %q, want %q", tr.Format, FormatMP3)
	}
}

func TestTrack_ZeroValue(t *testing.T) {
	var tr Track
	if tr.Path != "" || tr.Title != "" || tr.Artist != "" || tr.Album != "" {
		t.Errorf("expected empty string fields on zero Track, got %+v", tr)
	}
	if tr.Duration != 0 {
		t.Errorf("Duration = %v, want 0", tr.Duration)
	}
	if tr.Format != Format("") {
		t.Errorf("Format = %q, want empty", tr.Format)
	}
}
