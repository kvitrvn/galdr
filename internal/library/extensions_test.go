package library

import "testing"

func TestFormatFromPath(t *testing.T) {
	cases := []struct {
		path      string
		want      Format
		supported bool
	}{
		{"song.mp3", FormatMP3, true},
		{"song.MP3", FormatMP3, true},
		{"song.Mp3", FormatMP3, true},
		{"/music/Song.Flac", FormatFLAC, true},
		{"a.b.c.WAV", FormatWAV, true},
		{"song.ogg", "", false},
		{"song", "", false},
		{".mp3", FormatMP3, true},
		{"song.txt", "", false},
		{"song.aac", "", false},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			got, ok := FormatFromPath(c.path)
			if ok != c.supported {
				t.Errorf("FormatFromPath(%q) supported=%v, want %v", c.path, ok, c.supported)
			}
			if got != c.want {
				t.Errorf("FormatFromPath(%q) = %q, want %q", c.path, got, c.want)
			}
			if gotIsSupported := IsSupported(c.path); gotIsSupported != c.supported {
				t.Errorf("IsSupported(%q) = %v, want %v", c.path, gotIsSupported, c.supported)
			}
		})
	}
}
