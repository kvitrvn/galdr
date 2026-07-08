package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

// newTestModel returns a Model backed by a freshly loaded library of n
// tracks. The player is a MockPlayer so the TUI never touches real
// audio.
func newTestModel(t *testing.T, n int) *Model {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		path := filepath.Join(dir, fmt.Sprintf("t%02d.mp3", i))
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.MusicDir = dir
	a := app.New(cfg, player.NewMock())
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatalf("LoadLibrary: %v", err)
	}
	return New(a, theme.PaletteFor(theme.ModeAuto))
}

// key sends a key message to m and returns the resulting tea.Cmd.
func sendKey(t *testing.T, m *Model, k string) tea.Cmd {
	t.Helper()
	var msg tea.KeyMsg
	switch k {
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "j":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	case "k":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	case "+":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}}
	case "-":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}}
	case "n":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	case "p":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	case "?":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}
	case "q":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	default:
		t.Fatalf("unknown key %q", k)
	}
	_, cmd := m.Update(msg)
	return cmd
}

func TestModel_QuitReturnsTeaQuit(t *testing.T) {
	m := newTestModel(t, 0)
	if cmd := sendKey(t, m, "q"); cmd == nil {
		t.Error("q should return a tea.Cmd, got nil")
	}
}

func TestModel_NavigationKeys(t *testing.T) {
	m := newTestModel(t, 5)
	if got := m.app.SelectedIndex(); got != 0 {
		t.Fatalf("initial index = %d, want 0", got)
	}

	sendKey(t, m, "down")
	if got := m.app.SelectedIndex(); got != 1 {
		t.Errorf("index after down = %d, want 1", got)
	}

	sendKey(t, m, "up")
	if got := m.app.SelectedIndex(); got != 0 {
		t.Errorf("index after up = %d, want 0", got)
	}
}

func TestModel_NavigationKeysJK(t *testing.T) {
	m := newTestModel(t, 5)
	sendKey(t, m, "j")
	if got := m.app.SelectedIndex(); got != 1 {
		t.Errorf("index after j = %d, want 1", got)
	}
	sendKey(t, m, "k")
	if got := m.app.SelectedIndex(); got != 0 {
		t.Errorf("index after k = %d, want 0", got)
	}
}

func TestModel_EnterPlaysSelected(t *testing.T) {
	m := newTestModel(t, 3)
	sendKey(t, m, "enter")
	if got := m.app.State(); got != player.StatePlaying {
		t.Errorf("State after enter = %v, want %v", got, player.StatePlaying)
	}
}

func TestModel_SpaceTogglesPlayPause(t *testing.T) {
	m := newTestModel(t, 2)
	sendKey(t, m, "enter")
	if got := m.app.State(); got != player.StatePlaying {
		t.Fatalf("State after enter = %v, want playing", got)
	}
	sendKey(t, m, " ")
	if got := m.app.State(); got != player.StatePaused {
		t.Errorf("State after space = %v, want paused", got)
	}
	sendKey(t, m, " ")
	if got := m.app.State(); got != player.StatePlaying {
		t.Errorf("State after second space = %v, want playing", got)
	}
}

func TestModel_NextPrevious(t *testing.T) {
	m := newTestModel(t, 3)
	sendKey(t, m, "enter") // playing index 0
	sendKey(t, m, "n")
	if got := m.app.SelectedIndex(); got != 1 {
		t.Errorf("index after n = %d, want 1", got)
	}
	sendKey(t, m, "p")
	if got := m.app.SelectedIndex(); got != 0 {
		t.Errorf("index after p = %d, want 0", got)
	}
}

func TestModel_VolumeKeys(t *testing.T) {
	m := newTestModel(t, 1)
	// Start from a known midpoint.
	mock := m.app.Queue() // not used, just to access app
	_ = mock
	// The default volume is 100; bring it down to 0 to make the test
	// unambiguous.
	for i := 0; i < 30; i++ {
		sendKey(t, m, "-")
	}
	if got := m.app.Volume(); got != 0 {
		t.Fatalf("baseline Volume = %d, want 0", got)
	}
	for i := 0; i < 3; i++ {
		sendKey(t, m, "+")
	}
	if got := m.app.Volume(); got != 15 {
		t.Errorf("Volume after 3x + = %d, want 15", got)
	}
	for i := 0; i < 4; i++ {
		sendKey(t, m, "-")
	}
	if got := m.app.Volume(); got != 0 {
		t.Errorf("Volume after 4x - (from 15) = %d, want 0", got)
	}
}

func TestModel_HelpToggle(t *testing.T) {
	m := newTestModel(t, 1)
	if m.help {
		t.Fatal("help starts false")
	}
	sendKey(t, m, "?")
	if !m.help {
		t.Error("help should be true after ?")
	}
	sendKey(t, m, "?")
	if m.help {
		t.Error("help should be false after second ?")
	}
}

func TestModel_WindowSize(t *testing.T) {
	m := newTestModel(t, 1)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(*Model)
	if got.width != 120 || got.height != 40 {
		t.Errorf("after WindowSizeMsg: w=%d h=%d, want 120/40", got.width, got.height)
	}
}

func TestView_EmptyLibrary(t *testing.T) {
	m := newTestModel(t, 0)
	view := m.View()
	if !strings.Contains(view, "No tracks") {
		t.Errorf("empty view should mention 'No tracks', got: %q", view)
	}
	if !strings.Contains(view, "galdr") {
		t.Errorf("view should contain title 'galdr', got: %q", view)
	}
}

func TestView_TrackList(t *testing.T) {
	m := newTestModel(t, 3)
	view := m.View()
	// Track titles derived from filenames (no extension).
	for _, want := range []string{"t00", "t01", "t02"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain track %q, got: %q", want, view)
		}
	}
	if !strings.Contains(view, "▶") {
		t.Errorf("view should mark selected row with ▶, got: %q", view)
	}
}

func TestView_StatusBar(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter")
	view := m.View()
	for _, want := range []string{"playing", "vol", "100%", "t00"} {
		if !strings.Contains(view, want) {
			t.Errorf("status bar should contain %q, got: %q", want, view)
		}
	}
}

func TestView_ProgressBar(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter")
	view := m.View()
	if !strings.Contains(view, "[") || !strings.Contains(view, "]") {
		t.Errorf("progress bar should appear in view, got: %q", view)
	}
}

func TestView_Help(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "?")
	view := m.View()
	for _, want := range []string{"Keybindings", "play", "quit", "next", "prev"} {
		if !strings.Contains(view, want) {
			t.Errorf("help view should contain %q, got: %q", want, view)
		}
	}
}

func TestView_ShowsError(t *testing.T) {
	m := newTestModel(t, 1)
	m.app.PlaySelected() // populate error if any (here, no error since mock)
	// Force an error path: stop everything and try play on empty queue.
	_ = m.app.Stop()
	cfg := m.app.Config()
	cfg2 := *cfg
	empty := app.New(&cfg2, player.NewMock())
	m2 := New(empty, theme.PaletteFor(theme.ModeAuto))
	if err := empty.LoadLibrary("/does/not/exist"); err == nil {
		t.Fatal("expected error from LoadLibrary")
	}
	view := m2.View()
	if !strings.Contains(view, "error") {
		t.Errorf("view should surface 'error' after LoadLibrary failure, got: %q", view)
	}
}

func TestTick_ReschedulesItself(t *testing.T) {
	m := newTestModel(t, 0)
	_, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Error("tickMsg should produce a follow-up tickCmd, got nil")
	}
}

func TestScrollStart(t *testing.T) {
	cases := []struct {
		name            string
		total, sel, win int
		want            int
	}{
		{"empty", 0, 0, 5, 0},
		{"small-fits", 3, 1, 5, 0},
		{"selected-near-top", 20, 2, 5, 0},
		{"selected-near-bottom", 20, 18, 5, 14},
		{"selected-middle", 20, 10, 5, 6},
		{"exact-fit", 10, 9, 10, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := scrollStart(c.total, c.sel, c.win); got != c.want {
				t.Errorf("scrollStart(%d,%d,%d) = %d, want %d",
					c.total, c.sel, c.win, got, c.want)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0:00"},
		{-time.Second, "0:00"},
		{time.Second, "0:01"},
		{59 * time.Second, "0:59"},
		{time.Minute, "1:00"},
		{3*time.Minute + 42*time.Second, "3:42"},
		{125 * time.Second, "2:05"},
	}
	for _, c := range cases {
		if got := formatDuration(c.d); got != c.want {
			t.Errorf("formatDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestRenderProgressBar(t *testing.T) {
	zero := renderProgressBar(0, 0, 5)
	if zero != "[·····]" {
		t.Errorf("zero total bar = %q, want [·····]", zero)
	}
	half := renderProgressBar(30*time.Second, time.Minute, 6)
	if half != "[▓▓▓···]" {
		t.Errorf("half bar = %q, want [▓▓▓···]", half)
	}
	full := renderProgressBar(time.Minute, time.Minute, 4)
	if full != "[▓▓▓▓]" {
		t.Errorf("full bar = %q, want [▓▓▓▓]", full)
	}
	over := renderProgressBar(2*time.Minute, time.Minute, 4)
	if over != "[▓▓▓▓]" {
		t.Errorf("over-100%% bar = %q, want [▓▓▓▓]", over)
	}
	zeroWidth := renderProgressBar(time.Second, time.Second, 0)
	if zeroWidth != "" {
		t.Errorf("zero-width bar = %q, want empty", zeroWidth)
	}
}

func TestModel_DoesNotPanicOnUnknownMsg(t *testing.T) {
	m := newTestModel(t, 1)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Update panicked on unknown msg: %v", r)
		}
	}()
	_, _ = m.Update(struct{ msg string }{"hello"})
}
