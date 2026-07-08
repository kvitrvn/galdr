// Package state persists galdr's per-user playback state across
// launches. It is intentionally minimal: just enough to restore the
// last volume and a reference to the most recently played track.
//
// The state file is JSON for human readability and ease of debugging.
// It is written atomically: writes go to a sibling temp file and are
// then renamed into place. A missing or unparseable state file is
// treated as "no prior state" rather than as an error.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// State is the persistent slice of the App that survives a relaunch.
type State struct {
	// Volume is the value that will be applied to the Oto player at
	// startup. Out-of-range values are ignored.
	Volume int `json:"volume"`
	// CurrentPath is the path of the most recently played track. It
	// is informational: the TUI shows it as "last played" if no
	// playback is in progress, and Rescan uses it to decide whether
	// the current track has been removed.
	CurrentPath string `json:"current_path"`
}

// Default returns a zero-value state. Used when no state file is on
// disk or when the file is unparseable.
func Default() State {
	return State{Volume: 100}
}

// ErrNoState indicates that no state file exists. It is returned by
// Load when the file is genuinely missing. Other errors (permission,
// parse) are reported as-is.
var ErrNoState = errors.New("state: no prior state")

// Load reads the state file at path. If the file is missing it
// returns ErrNoState and the default state. Other errors (parse
// failure, permission) are returned alongside the default state so
// callers can decide whether to log them.
func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), ErrNoState
		}
		return Default(), err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return Default(), fmt.Errorf("state: parse %s: %w", path, err)
	}
	if s.Volume < 0 || s.Volume > 100 {
		s.Volume = 100
	}
	return s, nil
}

// Save writes s to path atomically. The file is created with mode
// 0o644. The parent directory must already exist.
func Save(path string, s State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("state: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("state: rename: %w", err)
	}
	return nil
}
