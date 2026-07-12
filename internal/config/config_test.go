package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvitrvn/galdr/internal/i18n"
)

func TestTheme_Valid(t *testing.T) {
	cases := []struct {
		in   Theme
		want bool
	}{
		{ThemeAuto, true},
		{ThemeLight, true},
		{ThemeDark, true},
		{Theme(""), false},
		{Theme("rainbow"), false},
		{Theme("AUTO"), false}, // case-sensitive
	}
	for _, c := range cases {
		t.Run(c.in.String(), func(t *testing.T) {
			if got := c.in.Valid(); got != c.want {
				t.Errorf("Theme(%q).Valid() = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestReplayGainMode_Valid(t *testing.T) {
	tests := []struct {
		mode ReplayGainMode
		want bool
	}{
		{mode: ReplayGainOff, want: true},
		{mode: ReplayGainTrack, want: true},
		{mode: ReplayGainAlbum, want: true},
		{mode: ReplayGainMode(""), want: false},
		{mode: ReplayGainMode("TRACK"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			if got := tt.mode.Valid(); got != tt.want {
				t.Errorf("ReplayGainMode(%q).Valid() = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("Default() returned nil")
	}
	if cfg.MusicDir != "~/Music" {
		t.Errorf("MusicDir = %q, want %q", cfg.MusicDir, "~/Music")
	}
	if cfg.Volume != 100 {
		t.Errorf("Volume = %d, want 100", cfg.Volume)
	}
	if cfg.Theme != ThemeAuto {
		t.Errorf("Theme = %q, want %q", cfg.Theme, ThemeAuto)
	}
	if cfg.Language != i18n.Auto {
		t.Errorf("Language = %q, want %q", cfg.Language, i18n.Auto)
	}
	if cfg.Audio.ReplayGain != ReplayGainOff {
		t.Errorf("Audio.ReplayGain = %q, want %q", cfg.Audio.ReplayGain, ReplayGainOff)
	}
	if cfg.UI.MinWidth != 48 || cfg.UI.MinHeight != 14 {
		t.Errorf("UI minimum = %dx%d, want 48x14", cfg.UI.MinWidth, cfg.UI.MinHeight)
	}
}

func TestLoadFrom_Language(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     i18n.Language
	}{
		{name: "absent", contents: "volume = 75\n", want: i18n.Auto},
		{name: "auto", contents: "language = \"auto\"\n", want: i18n.Auto},
		{name: "English", contents: "language = \"en\"\n", want: i18n.English},
		{name: "French", contents: "language = \"fr\"\n", want: i18n.French},
		{name: "Spanish", contents: "language = \"es\"\n", want: i18n.Spanish},
		{name: "German", contents: "language = \"de\"\n", want: i18n.German},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tt.contents), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadFrom(path)
			if err != nil {
				t.Fatalf("LoadFrom: %v", err)
			}
			if cfg.Language != tt.want {
				t.Errorf("Language = %q, want %q", cfg.Language, tt.want)
			}
		})
	}
}

func TestLoadFrom_InvalidLanguage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("language = \"it\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFrom(path)
	if err == nil || !strings.Contains(err.Error(), `invalid language "it"`) {
		t.Fatalf("LoadFrom invalid language error = %v", err)
	}
}

func TestLoadFrom_ReplayGain(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     ReplayGainMode
	}{
		{name: "audio omitted", contents: "volume = 75\n", want: ReplayGainOff},
		{name: "empty audio section", contents: "[audio]\n", want: ReplayGainOff},
		{name: "off", contents: "[audio]\nreplaygain = \"off\"\n", want: ReplayGainOff},
		{name: "track", contents: "[audio]\nreplaygain = \"track\"\n", want: ReplayGainTrack},
		{name: "album", contents: "[audio]\nreplaygain = \"album\"\n", want: ReplayGainAlbum},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tt.contents), 0o644); err != nil {
				t.Fatal(err)
			}

			cfg, err := LoadFrom(path)
			if err != nil {
				t.Fatalf("LoadFrom: %v", err)
			}
			if cfg.Audio.ReplayGain != tt.want {
				t.Errorf("Audio.ReplayGain = %q, want %q", cfg.Audio.ReplayGain, tt.want)
			}
		})
	}
}

func TestLoadFrom_InvalidReplayGain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[audio]\nreplaygain = \"auto\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom(invalid ReplayGain): expected error, got nil")
	}
	for _, want := range []string{"audio.replaygain", `"auto"`, "off, track or album"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestDefaultPath(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	want := filepath.Join(fakeHome, ".config", "galdr", "config.toml")
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestLoadFrom_MissingFile(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	missing := filepath.Join(t.TempDir(), "nope.toml")

	cfg, err := LoadFrom(missing)
	if err != nil {
		t.Fatalf("LoadFrom missing: %v", err)
	}
	def := Default()
	wantMusic := filepath.Join(fakeHome, "Music")
	if cfg.MusicDir != wantMusic || cfg.Volume != def.Volume || cfg.Theme != def.Theme {
		t.Errorf("LoadFrom missing = %+v, want defaults (MusicDir expanded to %q)", cfg, wantMusic)
	}
}

func TestLoadFrom_EmptyFile(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom empty: %v", err)
	}
	def := Default()
	wantMusic := filepath.Join(fakeHome, "Music")
	if cfg.MusicDir != wantMusic || cfg.Volume != def.Volume || cfg.Theme != def.Theme {
		t.Errorf("LoadFrom empty = %+v, want defaults (MusicDir expanded to %q)", cfg, wantMusic)
	}
}

func TestLoadFrom_ValidConfig(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	contents := `
music_dir = "~/songs"
volume = 42
theme = "dark"
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	wantMusic := filepath.Join(fakeHome, "songs")
	if cfg.MusicDir != wantMusic {
		t.Errorf("MusicDir = %q, want %q", cfg.MusicDir, wantMusic)
	}
	if cfg.Volume != 42 {
		t.Errorf("Volume = %d, want 42", cfg.Volume)
	}
	if cfg.Theme != ThemeDark {
		t.Errorf("Theme = %q, want %q", cfg.Theme, ThemeDark)
	}
}

func TestLoadFrom_PartialConfig(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	contents := `volume = 75
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.Volume != 75 {
		t.Errorf("Volume = %d, want 75", cfg.Volume)
	}
	// Missing fields keep defaults; MusicDir is still expanded.
	wantMusic := filepath.Join(fakeHome, "Music")
	if cfg.MusicDir != wantMusic {
		t.Errorf("MusicDir = %q, want expanded default %q", cfg.MusicDir, wantMusic)
	}
	if cfg.Theme != Default().Theme {
		t.Errorf("Theme = %q, want default %q", cfg.Theme, Default().Theme)
	}
}

func TestLoadFrom_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// Unterminated string: malformed TOML.
	contents := `music_dir = "~/songs`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom(invalid): expected error, got nil")
	}
}

func TestLoadFrom_InvalidTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	contents := `theme = "rainbow"
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom(invalid theme): expected error, got nil")
	}
}

func TestLoadFrom_TypeMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	// Volume must be an integer; a string is a type error.
	contents := `volume = "loud"
`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFrom(path)
	if err == nil {
		t.Fatal("LoadFrom(type mismatch): expected error, got nil")
	}
}

func TestLoadFrom_ExpandsHome(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"tilde-slash", "~/Music", filepath.Join(fakeHome, "Music")},
		{"tilde-only", "~", fakeHome},
		{"absolute", "/srv/music", "/srv/music"},
		{"relative", "music", "music"},
		{"tilde-no-slash treated literally", "~bob", "~bob"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.toml")
			contents := "music_dir = " + `"` + c.in + `"`
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadFrom(path)
			if err != nil {
				t.Fatalf("LoadFrom: %v", err)
			}
			if cfg.MusicDir != c.want {
				t.Errorf("MusicDir = %q, want %q", cfg.MusicDir, c.want)
			}
		})
	}
}
