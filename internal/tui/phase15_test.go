package tui

import (
	"strings"
	"testing"

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

func TestQueuePanel_RendersAllTracks(t *testing.T) {
	m := newTestModel(t, 4)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	view := m.queuePanelContent(20, 10)
	// 4 tracks, "now" prefix on the first one (which is the playing
	// track).
	if !strings.Contains(view, "now") {
		t.Errorf("queue panel should show 'now' for the playing track, got: %q", view)
	}
}

func TestQueuePanel_ShowsPositionNumber(t *testing.T) {
	m := newTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 1
	view := m.queuePanelContent(20, 10)
	if !strings.Contains(view, "2.") {
		t.Errorf("queue panel should show position '2.' for cursor row, got: %q", view)
	}
}

func TestQueuePanel_MarksPlayingTrack(t *testing.T) {
	m := newTestModel(t, 3)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	view := m.queuePanelContent(20, 10)
	// The first track is the playing one (▶ marker).
	if !strings.Contains(view, "▶") {
		t.Errorf("queue panel should mark the playing track, got: %q", view)
	}
}

func TestQueuePanel_EmptyShowsHelp(t *testing.T) {
	m := newTestModel(t, 0)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	view := m.queuePanelContent(20, 10)
	if !strings.Contains(view, "Queue empty") {
		t.Errorf("empty queue should show help, got: %q", view)
	}
}

func TestQueueNav_DownMovesCursor(t *testing.T) {
	m := newTestModel(t, 5)
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

func TestQueueNav_ShiftJ_MovesUp(t *testing.T) {
	m := newTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 2
	// J is shift+j. We need to use tea.KeyMsg directly.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	if m.app.Queue().Index() != 0 {
		t.Errorf("Index after MoveUp(2) = %d, want 0 (followed the moved track)", m.app.Queue().Index())
	}
}

func TestQueueNav_ShiftK_MovesDown(t *testing.T) {
	m := newTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 1
	// K is shift+k.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	// Track at index 1 was moved to index 2; index is 0 (still playing the first).
	// Verify: the track at position 2 should now be the one that was at position 1.
	paths := pathsOf(m.app.Queue())
	// Initially: t00, t01, t02, t03, t04. MoveDown(1): t00, t02, t01, t03, t04.
	// So paths[2] should be t01 (the moved track).
	if len(paths) < 3 || !strings.HasSuffix(paths[2], "t01.mp3") {
		t.Errorf("after MoveDown(1), expected t01 at index 2, got: %v", paths)
	}
}

func TestQueueNav_D_RemovesTrack(t *testing.T) {
	m := newTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 2
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if got := m.app.Queue().Len(); got != 4 {
		t.Errorf("Queue.Len after delete = %d, want 4", got)
	}
}

func TestQueueNav_D_PlayingIsNoOp(t *testing.T) {
	m := newTestModel(t, 3)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.queueCursor = 0 // the playing track
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if got := m.app.Queue().Len(); got != 3 {
		t.Errorf("Queue.Len after delete on playing = %d, want 3 (no-op)", got)
	}
}

func TestQueueNav_C_ClearsQueue(t *testing.T) {
	m := newTestModel(t, 5)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.focus.Set(PanelQueue)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if got := m.app.Queue().Len(); got != 1 {
		t.Errorf("Queue.Len after clear = %d, want 1", got)
	}
}

func TestQueueNav_EnterPlaysTrack(t *testing.T) {
	m := newTestModel(t, 5)
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
