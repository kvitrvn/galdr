package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

func TestLocalizedRenderingWideAndCompact(t *testing.T) {
	tests := []struct {
		language i18n.Language
		terms    []string
		compact  string
	}{
		{language: i18n.English, terms: []string{"Library", "Tracks", "Queue", "Now Playing"}, compact: "Nothing playing"},
		{language: i18n.French, terms: []string{"Bibliothèque", "Pistes", "File d’attente", "Lecture en cours"}, compact: "Aucune lecture"},
		{language: i18n.Spanish, terms: []string{"Biblioteca", "Pistas", "Cola", "En reproducción"}, compact: "Nada en reproducción"},
		{language: i18n.German, terms: []string{"Bibliothek", "Titel", "Warteschlange", "Aktuelle Wiedergabe"}, compact: "Keine Wiedergabe"},
	}
	for _, tt := range tests {
		t.Run(string(tt.language), func(t *testing.T) {
			tr := i18n.New(tt.language)
			cfg := config.Default()
			a := app.New(cfg, player.NewMock(), app.WithTranslator(tr))
			m := New(a, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig(), nil, WithTranslator(tr))

			m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
			wide := ansi.Strip(m.View())
			for _, term := range tt.terms {
				if !strings.Contains(wide, term) {
					t.Errorf("wide rendering missing %q", term)
				}
			}

			m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
			compact := ansi.Strip(m.View())
			if !strings.Contains(compact, tt.terms[1]) || !strings.Contains(compact, tt.compact) {
				t.Errorf("compact rendering missing localized labels: %q", compact)
			}
		})
	}
}

func TestEnglishRenderingContainsNoLegacyFrenchDurationText(t *testing.T) {
	m := New(app.New(config.Default(), player.NewMock()), theme.PaletteFor(theme.ModeAuto), DefaultUIConfig(), nil)
	m.durations.running = true
	m.durations.completed = 1
	m.durations.total = 2
	status := m.durationFooterStatus()
	if strings.Contains(status, "Durées") || strings.Contains(status, "indisponibles") {
		t.Errorf("English duration status contains French: %q", status)
	}
}
