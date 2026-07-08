// Package config loads user configuration from a TOML file and provides
// sane defaults so the app can run without any configuration present.
//
// The default config path is ~/.config/galdr/config.toml.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Theme selects how the TUI adapts to the terminal background.
type Theme string

const (
	ThemeAuto  Theme = "auto"
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
)

// Valid reports whether t is one of the supported theme modes.
func (t Theme) Valid() bool {
	switch t {
	case ThemeAuto, ThemeLight, ThemeDark:
		return true
	}
	return false
}

// String returns the theme value as a string.
func (t Theme) String() string { return string(t) }

// Config is the resolved application configuration.
//
// Values stored here are post-merge (defaults + user file) and post-expansion
// (a leading "~" in MusicDir is replaced by the current user's home dir).
type Config struct {
	MusicDir string
	Volume   int
	Theme    Theme
}

// Default returns the built-in configuration used when no user config exists.
//
// MusicDir is the literal "~/Music" string. Expansion to a real path happens
// in Load / LoadFrom so that Default() does not depend on the environment.
func Default() *Config {
	return &Config{
		MusicDir: "~/Music",
		Volume:   100,
		Theme:    ThemeAuto,
	}
}

// DefaultPath returns the conventional per-user config path:
//
//	$HOME/.config/galdr/config.toml
//
// It returns an error if the user's home directory cannot be determined.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: locate home dir: %w", err)
	}
	return filepath.Join(home, ".config", "galdr", "config.toml"), nil
}

// Load reads the configuration from the default path.
//
// If the file does not exist, Load returns Default() with a nil error so
// that the app can start without any configuration file present.
func Load() (*Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom reads the configuration from path, merging it with the defaults.
//
// Behavior:
//   - Missing file  -> returns Default() (with MusicDir expanded), nil.
//   - Empty file    -> returns Default() (with MusicDir expanded), nil.
//   - Valid TOML    -> any field present in the file overrides the default.
//     Fields omitted from the file keep their default value.
//   - Invalid TOML  -> returns nil and a wrapped error explaining the failure.
//
// In every successful path MusicDir is passed through expandHome, so a
// leading "~" is always resolved against the current user's home
// directory before the config is returned.
func LoadFrom(path string) (*Config, error) {
	cfg := Default()

	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return expandMusicDir(cfg)
	} else if err != nil {
		return nil, fmt.Errorf("config: stat %s: %w", path, err)
	}

	var file fileConfig
	if _, err := toml.DecodeFile(path, &file); err != nil {
		return nil, fmt.Errorf("config: decode %s: %w", path, err)
	}

	if file.MusicDir != nil {
		cfg.MusicDir = *file.MusicDir
	}
	if file.Volume != nil {
		cfg.Volume = *file.Volume
	}
	if file.Theme != nil {
		t := Theme(*file.Theme)
		if !t.Valid() {
			return nil, fmt.Errorf("config: invalid theme %q (want auto, light or dark)", *file.Theme)
		}
		cfg.Theme = t
	}

	return expandMusicDir(cfg)
}

// expandMusicDir applies expandHome to cfg.MusicDir and returns the
// (possibly modified) config. It centralises the "~" expansion so that
// both the default path and a user-supplied path end up resolved.
func expandMusicDir(cfg *Config) (*Config, error) {
	expanded, err := expandHome(cfg.MusicDir)
	if err != nil {
		return nil, fmt.Errorf("config: expand music_dir: %w", err)
	}
	cfg.MusicDir = expanded
	return cfg, nil
}

// fileConfig mirrors Config but uses pointer fields so we can distinguish
// "field absent from file" from "field present with a zero value".
type fileConfig struct {
	MusicDir *string `toml:"music_dir"`
	Volume   *int    `toml:"volume"`
	Theme    *string `toml:"theme"`
}

// expandHome replaces a leading "~" or "~/" in path with the current user's
// home directory. Paths that do not start with "~" are returned unchanged.
func expandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	if path[1] == '/' || path[1] == filepath.Separator {
		return filepath.Join(home, path[2:]), nil
	}
	// Something like "~something" - not a home reference.
	return path, nil
}
