package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/library"
)

// pathsOf returns the paths of every track in the queue.
func pathsOf(q *library.Queue) []string {
	all := q.Tracks()
	out := make([]string, len(all))
	for i, t := range all {
		out[i] = t.Path
	}
	return out
}

func newQueuedTestModel(t *testing.T, n int) *Model {
	t.Helper()
	m := newTestModel(t, n)
	if n > 0 {
		if err := m.app.PlaySelected(); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func TestQueuePanel_RendersAllTracks(t *testing.T) {
	m := newQueuedTestModel(t, 4)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	view := m.queuePanelContent(20, 10)
	if !strings.Contains(view, "now") {
		t.Errorf("queue panel should show the current track, got: %q", view)
	}
}

func TestQueuePanel_ShowsPositionNumber(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 1
	view := m.queuePanelContent(20, 10)
	if !strings.Contains(view, "+1") {
		t.Errorf("queue panel should show relative position '+1' for cursor row, got: %q", view)
	}
}

func TestQueuePanel_MarksPlayingTrack(t *testing.T) {
	m := newQueuedTestModel(t, 3)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	sendKey(t, m, "enter")
	view := m.queuePanelContent(20, 10)
	// The first track is the playing one (▶ marker).
	if !strings.Contains(view, "▶") {
		t.Errorf("queue panel should mark the playing track, got: %q", view)
	}
}

func TestQueuePanel_EmptyShowsHelp(t *testing.T) {
	m := newQueuedTestModel(t, 0)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	view := m.queuePanelContent(20, 10)
	if !strings.Contains(view, "Queue is empty") {
		t.Errorf("empty queue should show help, got: %q", view)
	}
}

func TestQueueNav_DownMovesCursor(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	if m.queueCursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.queueCursor)
	}
	sendKey(t, m, "down")
	if m.queueCursor != 1 {
		t.Errorf("after down, cursor = %d, want 1", m.queueCursor)
	}
	sendKey(t, m, "up")
	if m.queueCursor != 0 {
		t.Errorf("after up, cursor = %d, want 0", m.queueCursor)
	}
}

func TestQueueCursorDoesNotResetOnTick(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.focus.Set(PanelQueue)
	m.queueCursor = 3

	m.Update(tickMsg(time.Now()))

	if m.queueCursor != 3 {
		t.Fatalf("cursor after tick = %d, want 3", m.queueCursor)
	}
}

func TestQueueNav_ShiftK_MovesUp(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 2
	// K is shift+k. We need to use tea.KeyMsg directly.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if m.queueCursor != 1 {
		t.Errorf("queue cursor after move up = %d, want 1", m.queueCursor)
	}
	if m.app.Queue().Index() != 0 {
		t.Errorf("Index after MoveUp(2) = %d, want 0 (followed the moved track)", m.app.Queue().Index())
	}
}

func TestQueueNav_ShiftJ_MovesDown(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 1
	// J is shift+j.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if m.queueCursor != 2 {
		t.Errorf("queue cursor after move down = %d, want 2", m.queueCursor)
	}
	// Track at index 1 was moved to index 2; index is 0 (still playing the first).
	// Verify: the track at position 2 should now be the one that was at position 1.
	paths := pathsOf(m.app.Queue())
	// Initially: t00, t01, t02, t03, t04. MoveDown(1): t00, t02, t01, t03, t04.
	// So paths[2] should be t01 (the moved track).
	if len(paths) < 3 || !strings.HasSuffix(paths[2], "t01.mp3") {
		t.Errorf("after MoveDown(1), expected t01 at index 2, got: %v", paths)
	}
}

func TestQueueNav_ShiftArrowsReorder(t *testing.T) {
	m := newQueuedTestModel(t, 4)
	m.focus.Set(PanelQueue)
	m.queueCursor = 2
	m.Update(tea.KeyMsg{Type: tea.KeyShiftUp})
	if m.queueCursor != 1 || !strings.HasSuffix(pathsOf(m.app.Queue())[1], "t02.mp3") {
		t.Errorf("shift+up did not move highlighted item up: cursor=%d paths=%v", m.queueCursor, pathsOf(m.app.Queue()))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	if m.queueCursor != 2 || !strings.HasSuffix(pathsOf(m.app.Queue())[2], "t02.mp3") {
		t.Errorf("shift+down did not move highlighted item down: cursor=%d paths=%v", m.queueCursor, pathsOf(m.app.Queue()))
	}
}

func TestQueueNav_D_RemovesTrack(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 2
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if got := m.app.Queue().Len(); got != 4 {
		t.Errorf("Queue.Len after delete = %d, want 4", got)
	}
}

func TestQueueNav_D_PlayingIsNoOp(t *testing.T) {
	m := newQueuedTestModel(t, 3)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 0 // the playing track
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if got := m.app.Queue().Len(); got != 3 {
		t.Errorf("Queue.Len after delete on playing = %d, want 3 (no-op)", got)
	}
}

func TestQueueNav_C_ClearsQueue(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if got := m.app.Queue().Len(); got != 1 {
		t.Errorf("Queue.Len after clear = %d, want 1", got)
	}
}

func TestQueueNav_EnterPlaysTrack(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 2
	sendKey(t, m, "enter")
	if m.app.Current() == nil {
		t.Fatal("Current = nil after enter in Queue")
	}
	// The playing track should now be at index 2.
	if m.app.Queue().Index() != 2 {
		t.Errorf("Index after enter = %d, want 2", m.app.Queue().Index())
	}
}

func TestPlayingFromQueueAlignsTracksPanel(t *testing.T) {
	m := newQueuedTestModel(t, 8)
	m.focus.Set(PanelQueue)
	m.queueCursor = 7

	sendKey(t, m, "enter")

	if got := m.app.ScopedIndex(); got != 7 {
		t.Fatalf("Tracks selection after Queue play = %d, want 7", got)
	}
	view := m.listViewSized(30, 3)
	if !strings.Contains(view, "t07") || !strings.Contains(view, "▶") {
		t.Fatalf("Tracks panel is not aligned with current track: %q", view)
	}
}

func TestQueueCursorFollowsCurrentAfterShuffle(t *testing.T) {
	m := newQueuedTestModel(t, 5)
	m.focus.Set(PanelQueue)
	if err := m.app.PlayAtIndex(2); err != nil {
		t.Fatal(err)
	}
	m.queueCursor = 0
	sendKey(t, m, "s")
	if m.queueCursor != m.app.Queue().Index() {
		t.Fatalf("cursor/index after shuffle = %d/%d", m.queueCursor, m.app.Queue().Index())
	}
	if got := m.app.Queue().Tracks()[m.queueCursor].Path; got != m.app.Current().Path {
		t.Fatalf("cursor track = %q, current = %q", got, m.app.Current().Path)
	}
}
