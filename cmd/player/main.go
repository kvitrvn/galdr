package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player/oto"
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

	// LoadLibrary failures are non-fatal: the TUI shows them as a status
	// message and the user can fix their config or pick another folder.
	_ = a.LoadLibrary(cfg.MusicDir)

	palette := theme.PaletteFor(theme.Mode(cfg.Theme))
	model := tui.New(a, palette)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
