package tui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func writeTestCover(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	img.Set(1, 0, color.NRGBA{G: 255, A: 255})
	img.Set(0, 1, color.NRGBA{B: 255, A: 255})
	img.Set(1, 1, color.NRGBA{R: 255, G: 255, A: 255})
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRenderAlbumCoverDimensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.png")
	writeTestCover(t, path)
	art, err := renderAlbumCover(path, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(art, "\n")
	if len(lines) != 4 {
		t.Fatalf("cover height = %d, want 4", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 8 {
			t.Fatalf("line %d width = %d, want 8", i, got)
		}
	}
}

func TestNowPlayingRendersAlbumCover(t *testing.T) {
	m, _ := modelWithMock(t)
	writeTestCover(t, filepath.Join(m.app.Config().MusicDir, "cover.png"))
	sendKey(t, m, "enter")

	view := m.nowPlayingView(90, 5)
	if !strings.Contains(ansi.Strip(view), "▀") {
		t.Fatalf("Now Playing has no cover: %q", view)
	}
	for i, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got != 90 {
			t.Fatalf("line %d width = %d, want 90", i, got)
		}
	}
}

func TestNowPlayingHidesCoverInCompactHeader(t *testing.T) {
	m, _ := modelWithMock(t)
	writeTestCover(t, filepath.Join(m.app.Config().MusicDir, "cover.png"))
	sendKey(t, m, "enter")

	if view := ansi.Strip(m.nowPlayingView(60, 3)); strings.Contains(view, "▀") {
		t.Fatalf("compact header rendered cover: %q", view)
	}
}
