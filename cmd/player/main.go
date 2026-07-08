package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player/oto"
	"github.com/kvitrvn/galdr/internal/state"
	"github.com/kvitrvn/galdr/internal/theme"
	"github.com/kvitrvn/galdr/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "galdr:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pl := oto.New()
	a := app.New(cfg, pl)

	statePath := stateFilePath()
	if s, err := state.Load(statePath); err == nil {
		// Apply only on success; a corrupt or missing state file is
		// silently ignored and the app starts with defaults.
		a.ApplySnapshot(s.Volume, s.CurrentPath)
	} else if !errors.Is(err, state.ErrNoState) {
		fmt.Fprintln(os.Stderr, "galdr: could not load state:", err)
	}

	// LoadLibrary failures are non-fatal: the TUI shows them as a status
	// message and the user can fix their config or pick another folder.
	_ = a.LoadLibrary(cfg.MusicDir)

	palette := theme.PaletteFor(theme.Mode(cfg.Theme))
	model := tui.New(a, palette, uiConfigFromConfig(cfg))
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}

	// Persist the snapshot on graceful exit. Failures here are
	// non-fatal: a corrupt state file on the next launch is harmless.
	volume, currentPath := a.Snapshot()
	if err := state.Save(statePath, state.State{Volume: volume, CurrentPath: currentPath}); err != nil {
		fmt.Fprintln(os.Stderr, "galdr: could not save state:", err)
	}
	return nil
}

// stateFilePath returns the absolute path of the state file. The
// parent directory is created by state.Save. We use the same
// ~/.config/galdr/ directory as the config so the two files are
// colocated.
func stateFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "galdr", "state.json")
	}
	return filepath.Join(home, ".config", "galdr", "state.json")
}

// uiConfigFromConfig converts a config.UIConfig into a tui.UIConfig.
// Keeping the conversion in main.go avoids a tui -> config
// import cycle (the TUI package only sees its own type).
func uiConfigFromConfig(cfg *config.Config) tui.UIConfig {
	return tui.UIConfig{
		LeftWidth:  cfg.UI.LeftWidth,
		RightWidth: cfg.UI.RightWidth,
		MinWidth:   cfg.UI.MinWidth,
		MinHeight:  cfg.UI.MinHeight,
	}
}
