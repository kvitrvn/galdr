package library

import "testing"

func TestTitleFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"song.mp3", "song"},
		{"/music/song.mp3", "song"},
		{"/music/Artist - Song.flac", "Artist - Song"},
		{"song.WAV", "song"},
		{"song.FLAC", "song"},
		{"song", "song"},
		{"deeply/nested/path/track.mp3", "track"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got := TitleFromPath(c.path)
			if got != c.want {
				t.Errorf("TitleFromPath(%q) = %q, want %q", c.path, got, c.want)
			}
		})
	}
}
