package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/mpris"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/player/mpv"
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
	tr := i18n.New(i18n.Resolve(i18n.Auto, os.Getenv))
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("%s: %w", tr.T(i18n.DiagLoadConfig), err)
	}
	tr = i18n.New(i18n.Resolve(cfg.Language, os.Getenv))

	pl, err := mpv.New(playerOptionsFromConfig(cfg))
	if err != nil {
		return fmt.Errorf("%s: %w", tr.T(i18n.DiagInitAudio), err)
	}
	defer pl.Close()
	a := app.New(cfg, pl, app.WithTranslator(tr))

	statePath := stateFilePath()
	if s, err := state.Load(statePath); err == nil {
		// Apply only on success; a corrupt or missing state file is
		// silently ignored and the app starts with defaults.
		a.ApplySnapshot(s.Volume, s.CurrentPath)
	} else if !errors.Is(err, state.ErrNoState) {
		fmt.Fprintln(os.Stderr, "galdr:", tr.T(i18n.DiagLoadState), err)
	}

	// LoadLibrary failures are non-fatal: the TUI shows them as a status
	// message and the user can fix their config or pick another folder.
	_ = a.LoadLibrary(cfg.MusicDir)

	// Duration probing uses a second libmpv instance so inspecting library
	// files can never replace or pause the track loaded by the player. Failure
	// is non-fatal: durations already found by the scanner remain available.
	var durationProber tui.DurationProber
	probe, probeErr := mpv.NewDurationProber()
	if probeErr != nil {
		fmt.Fprintln(os.Stderr, "galdr:", tr.T(i18n.DiagDurationUnavailable), probeErr)
	} else {
		durationProber = probe
	}

	palette := theme.PaletteFor(theme.Mode(cfg.Theme))
	model := tui.New(a, palette, uiConfigFromConfig(cfg), durationProber, tui.WithTranslator(tr))
	var p *tea.Program
	mprisService := mpris.New(
		func(request app.PlaybackRequest) {
			p.Send(request)
		},
		func(err error) {
			fmt.Fprintln(os.Stderr, "galdr:", tr.T(i18n.DiagMPRISLost), err)
		},
	)
	model.SetPlaybackPublisher(mprisService)
	mprisService.Publish(a.PlaybackSnapshot())
	p = tea.NewProgram(model)
	if err := mprisService.StartSession(); err != nil {
		fmt.Fprintln(os.Stderr, "galdr:", tr.T(i18n.DiagMPRISUnavailable), err)
	}
	_, runErr := p.Run()
	if err := mprisService.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "galdr:", tr.T(i18n.DiagMPRISStop), err)
	}
	model.Close()
	if probe != nil {
		probe.Close()
	}
	pl.Close()
	if runErr != nil {
		return fmt.Errorf("%s: %w", tr.T(i18n.DiagTUI), runErr)
	}

	// Persist the snapshot on graceful exit. Failures here are
	// non-fatal: a corrupt state file on the next launch is harmless.
	volume, currentPath := a.Snapshot()
	if err := state.Save(statePath, state.State{Volume: volume, CurrentPath: currentPath}); err != nil {
		fmt.Fprintln(os.Stderr, "galdr:", tr.T(i18n.DiagSaveState), err)
	}
	return nil
}

func playerOptionsFromConfig(cfg *config.Config) player.PlaybackOptions {
	mode := player.ReplayGainOff
	switch cfg.Audio.ReplayGain {
	case config.ReplayGainTrack:
		mode = player.ReplayGainTrack
	case config.ReplayGainAlbum:
		mode = player.ReplayGainAlbum
	}
	return player.PlaybackOptions{ReplayGain: mode}
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
