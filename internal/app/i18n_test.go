package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/library"
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

func TestWithTranslatorLocalizesDirectPlaylistAddStatuses(t *testing.T) {
	dir := t.TempDir()
	trackPath := filepath.Join(dir, "chanson.mp3")
	if err := os.WriteFile(trackPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.MusicDir = dir
	a := New(cfg, player.NewMock(), WithTranslator(i18n.New(i18n.French)))
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatal(err)
	}
	track := *a.Selected()
	if err := a.CreatePlaylistWithTrack("été", track); err != nil {
		t.Fatal(err)
	}
	if got := a.Status(); got != "été créée et chanson ajoutée" {
		t.Fatalf("localized creation status = %q", got)
	}
	if err := a.AddTrackToPlaylist("été", track); err != nil {
		t.Fatal(err)
	}
	if got := a.Status(); got != "chanson ajoutée à été" {
		t.Fatalf("localized append status = %q", got)
	}
	if err := a.AddTracksToPlaylist("été", []library.Track{track, track}); err != nil {
		t.Fatal(err)
	}
	if got := a.Status(); got != "2 pistes ajoutées à été" {
		t.Fatalf("localized batch append status = %q", got)
	}
	if err := a.CreatePlaylistWithTracks("album", []library.Track{track, track}); err != nil {
		t.Fatal(err)
	}
	if got := a.Status(); got != "album créée avec 2 pistes" {
		t.Fatalf("localized batch creation status = %q", got)
	}
	if err := os.Remove(filepath.Join(dir, "Playlists", "été.m3u8")); err != nil {
		t.Fatal(err)
	}
	if err := a.AddTrackToPlaylist("été", track); !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("missing destination error = %v", err)
	}
	if got := a.Status(); got != "La playlist n’est plus disponible" {
		t.Fatalf("localized missing status = %q", got)
	}
}
