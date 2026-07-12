package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/player"
)

func TestWithTranslatorLocalizesStatusWithoutChangingTechnicalError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chanson.mp3"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a := New(config.Default(), player.NewMock(), WithTranslator(i18n.New(i18n.French)))
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}
	if got := a.Status(); got != "1 piste chargée" {
		t.Errorf("localized status = %q", got)
	}

	missing := filepath.Join(dir, "absent")
	if err := a.LoadLibrary(missing); err == nil {
		t.Fatal("LoadLibrary(missing) returned nil")
	}
	if got := a.Status(); got != "Échec de l’analyse de la bibliothèque" {
		t.Errorf("localized failure status = %q", got)
	}
	if err := a.Error(); err == nil || !strings.Contains(err.Error(), missing) {
		t.Errorf("technical error was translated or lost: %v", err)
	}
}
