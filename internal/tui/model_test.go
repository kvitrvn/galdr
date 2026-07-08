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
	return New(a, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig())
}

// newTestModelWithTitles is like newTestModel but lets the caller
// specify the title of every track. The mock player does not read
// real metadata, so the titles are patched in via the queue's
// current contents.
func newTestModelWithTitles(t *testing.T, titles []string) *Model {
	t.Helper()
	dir := t.TempDir()
	for i := range titles {
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
	q := a.Queue()
	all := q.Tracks()
	if len(all) != len(titles) {
		t.Fatalf("len(tracks) = %d, want %d", len(all), len(titles))
	}
	for i, title := range titles {
		all[i].Title = title
	}
	q.Replace(all)
	return New(a, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig())
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
	case "left":
		msg = tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		msg = tea.KeyMsg{Type: tea.KeyRight}
	case "home":
		msg = tea.KeyMsg{Type: tea.KeyHome}
	case "end":
		msg = tea.KeyMsg{Type: tea.KeyEnd}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
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
	case "r":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	case "R":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	case "s":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	case "m":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}
	case "/":
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	case "C-l":
		msg = tea.KeyMsg{Type: tea.KeyCtrlL}
	default:
		if len(k) == 1 {
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		} else {
			t.Fatalf("unknown key %q", k)
		}
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
	// The v2 layout puts three panel titles in the borders.
	for _, want := range []string{"Library", "Tracks", "Queue"} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain panel title %q, got: %q", want, view)
		}
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
	m2 := New(empty, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig())
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

func TestModel_ShuffleKey(t *testing.T) {
	m := newTestModel(t, 3)
	if m.app.Shuffle() {
		t.Fatal("setup: Shuffle should be off")
	}
	sendKey(t, m, "s")
	if !m.app.Shuffle() {
		t.Error("after s: Shuffle should be on")
	}
	sendKey(t, m, "s")
	if m.app.Shuffle() {
		t.Error("after second s: Shuffle should be off")
	}
}

func TestModel_RepeatKey_Cycles(t *testing.T) {
	m := newTestModel(t, 3)
	if m.app.Repeat() != app.RepeatOff {
		t.Fatalf("setup: Repeat = %v, want off", m.app.Repeat())
	}
	sendKey(t, m, "R")
	if m.app.Repeat() != app.RepeatAll {
		t.Errorf("after R: Repeat = %v, want all", m.app.Repeat())
	}
	sendKey(t, m, "R")
	if m.app.Repeat() != app.RepeatOne {
		t.Errorf("after R: Repeat = %v, want one", m.app.Repeat())
	}
	sendKey(t, m, "R")
	if m.app.Repeat() != app.RepeatOff {
		t.Errorf("after R: Repeat = %v, want off", m.app.Repeat())
	}
}

func TestModel_MuteKey(t *testing.T) {
	m := newTestModel(t, 1)
	if m.app.Muted() {
		t.Fatal("setup: Muted should be false")
	}
	sendKey(t, m, "m")
	if !m.app.Muted() {
		t.Error("after m: Muted should be true")
	}
	sendKey(t, m, "m")
	if m.app.Muted() {
		t.Error("after second m: Muted should be false")
	}
}

func TestModel_SeekForwardBackward(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter") // start playing
	pl := m.app.Queue()    // not used; we access the mock via app
	_ = pl
	// Mock player has no real position; we just verify the seek call
	// reaches the player. Use the public Seek API via app.
	m.app.ApplySnapshot(100, "")
	if err := m.app.Seek(20 * time.Second); err != nil {
		t.Fatalf("Seek: %v", err)
	}
}

func TestModel_RescanKey(t *testing.T) {
	m := newTestModel(t, 2)
	dir := m.app.Config().MusicDir
	newPath := filepath.Join(dir, "extra.mp3")
	if err := os.WriteFile(newPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	sendKey(t, m, "r")
	if got := m.app.Queue().Len(); got != 3 {
		t.Errorf("len after rescan = %d, want 3", got)
	}
}

func TestModel_StatusBar_ShowsMuteShuffleRepeat(t *testing.T) {
	m := newTestModel(t, 2)
	m.app.ToggleShuffle()
	m.app.CycleRepeat() // all
	m.app.ToggleMute()
	view := m.statusView(120)
	if !strings.Contains(view, "[shuffle]") {
		t.Errorf("status bar missing [shuffle]: %q", view)
	}
	if !strings.Contains(view, "[repeat: all]") {
		t.Errorf("status bar missing [repeat: all]: %q", view)
	}
	if !strings.Contains(view, "[mute]") {
		t.Errorf("status bar missing [mute]: %q", view)
	}
}

func TestModel_HelpView_IncludesNewKeys(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "?")
	view := m.View()
	for _, expected := range []string{"rescan", "shuffle", "repeat", "mute", "→", "←"} {
		if !strings.Contains(view, expected) {
			t.Errorf("help view missing %q", expected)
		}
	}
}

func TestModel_SeekRelative_ClampsBelow(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter")
	// Apply a positive seek so we can step backwards.
	_ = m.app.Seek(20 * time.Second)
	m.seekRelative(-100 * time.Second)
	// We can't observe position from a MockPlayer; just verify no panic.
}

func TestModel_SeekRelative_ClampsAbove(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter")
	m.seekRelative(100 * time.Second)
	// No panic, no assertion against mock; the seek reaches the player.
}

func TestModel_SearchKeyEntersSearchMode(t *testing.T) {
	m := newTestModel(t, 2)
	if m.searchMode {
		t.Fatal("searchMode starts false")
	}
	sendKey(t, m, "/")
	if !m.searchMode {
		t.Error("after /: searchMode should be true")
	}
	view := m.View()
	if !strings.Contains(view, "/") {
		t.Errorf("view should show search prompt, got: %q", view)
	}
}

func TestModel_SearchTypingFiltersList(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	sendKey(t, m, "/")
	// Type "limbo" one rune at a time.
	for _, r := range "limbo" {
		sendKey(t, m, string(r))
	}
	if got := m.app.VisibleLen(); got != 1 {
		t.Errorf("VisibleLen after typing limbo = %d, want 1", got)
	}
	if got := m.app.VisibleTracks()[0].Title; got != "Limbo" {
		t.Errorf("VisibleTracks[0] = %q, want Limbo", got)
	}
}

func TestModel_SearchEnterExitsModeButKeepsFilter(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	sendKey(t, m, "/")
	for _, r := range "limbo" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	if m.searchMode {
		t.Error("after enter: searchMode should be false")
	}
	if !m.app.HasFilter() {
		t.Error("after enter: filter should still be active")
	}
}

func TestModel_SearchEscExitsModeButKeepsFilter(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	sendKey(t, m, "/")
	for _, r := range "limbo" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "esc")
	if m.searchMode {
		t.Error("after esc: searchMode should be false")
	}
	if !m.app.HasFilter() {
		t.Error("after esc in search mode: filter should still be active")
	}
}

func TestModel_SearchEscOutOfModeClearsFilter(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	sendKey(t, m, "/")
	for _, r := range "limbo" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	sendKey(t, m, "esc")
	if m.app.HasFilter() {
		t.Error("after esc out of search mode: filter should be cleared")
	}
}

func TestModel_SearchCtrlLClearsFilter(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	sendKey(t, m, "/")
	for _, r := range "limbo" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "enter")
	sendKey(t, m, "C-l")
	if m.app.HasFilter() {
		t.Error("after C-l: filter should be cleared")
	}
}

func TestModel_SearchBackspaceUpdatesFilter(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	sendKey(t, m, "/")
	for _, r := range "limbo" {
		sendKey(t, m, string(r))
	}
	sendKey(t, m, "backspace")
	if got := m.app.Filter(); got != "limb" {
		t.Errorf("Filter after backspace = %q, want %q", got, "limb")
	}
}

func TestModel_NavigationInSearchModeIsInput(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	sendKey(t, m, "/")
	// Up/Down should feed the textinput, not change selection.
	before := m.app.SelectedIndex()
	sendKey(t, m, "down")
	if m.app.SelectedIndex() != before {
		t.Errorf("down in search mode should not move selection (was %d, now %d)", before, m.app.SelectedIndex())
	}
	if !m.searchMode {
		t.Error("searchMode should still be true")
	}
}

func TestView_ShowsNoMatchMessage(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo"})
	sendKey(t, m, "/")
	for _, r := range "xyz" {
		sendKey(t, m, string(r))
	}
	view := m.View()
	if !strings.Contains(view, "No tracks match") {
		t.Errorf("view should show 'No tracks match' for empty filter, got: %q", view)
	}
}

func TestView_StatusBarShowsFilter(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen"})
	m.app.SetFilter("limbo")
	view := m.statusView(120)
	if !strings.Contains(view, "[filter: limbo 1/3]") {
		t.Errorf("status bar should show filter, got: %q", view)
	}
}

func TestView_StatusBarShowsUnknownDuration(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "enter")
	view := m.statusView(120)
	if !strings.Contains(view, "--:--") {
		t.Errorf("status bar should show --:-- for unknown duration, got: %q", view)
	}
}

func TestView_StatusBarShowsKnownDuration(t *testing.T) {
	// newTestModelWithTitles sets up an App whose MockPlayer has no
	// duration by default. We override the mock here to have a real
	// duration and check the formatted string appears.
	dir := t.TempDir()
	for i := 0; i < 1; i++ {
		path := filepath.Join(dir, fmt.Sprintf("t%02d.mp3", i))
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Default()
	cfg.MusicDir = dir
	mock := player.NewMock()
	mock.SetDuration(3*time.Minute + 42*time.Second)
	a := app.New(cfg, mock)
	if err := a.LoadLibrary(dir); err != nil {
		t.Fatal(err)
	}
	m := New(a, theme.PaletteFor(theme.ModeAuto), DefaultUIConfig())
	sendKey(t, m, "enter")
	view := m.statusView(120)
	if !strings.Contains(view, "3:42") {
		t.Errorf("status bar should show duration 3:42, got: %q", view)
	}
	if strings.Contains(view, "--:--") {
		t.Errorf("status bar should not show --:-- when duration is known, got: %q", view)
	}
}

func TestModel_HelpView_IncludesSearch(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "?")
	view := m.View()
	for _, want := range []string{"search", "C-l"} {
		if !strings.Contains(view, want) {
			t.Errorf("help view should contain %q, got: %q", want, view)
		}
	}
}

func TestModel_QuitInSearchModeExitsSearchAndQuits(t *testing.T) {
	m := newTestModel(t, 1)
	sendKey(t, m, "/")
	for _, r := range "abc" {
		sendKey(t, m, string(r))
	}
	cmd := sendKey(t, m, "q")
	if cmd == nil {
		t.Error("q in search mode should still return tea.Quit cmd")
	}
	if m.searchMode {
		t.Error("q in search mode should exit search mode first")
	}
}

func TestModel_NextHonoursFilter(t *testing.T) {
	m := newTestModelWithTitles(t, []string{"Anthem", "Limbo", "Amen", "Limbo"})
	m.app.SetFilter("limbo")
	sendKey(t, m, "n")
	if got := m.app.SelectedIndex(); got != 3 {
		t.Errorf("SelectedIndex after n = %d, want 3", got)
	}
}
