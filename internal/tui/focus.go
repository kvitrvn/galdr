package tui

// FocusManager cycles focus between the panels of the TUI.
//
// It is a tiny stateful object: callers move the focus forward
// (Tab) or backward (Shift+Tab) and read the currently focused
// panel id. The TUI model owns a single instance for the lifetime
// of the program; the focused panel drives the title marker and accent.
type FocusManager struct {
	current PanelID
}

// NewFocusManager returns a manager focused on the Tracks panel,
// which is the panel with the most content on a fresh start.
func NewFocusManager() *FocusManager {
	return &FocusManager{current: PanelTracks}
}

// Current returns the panel id currently in focus.
func (f *FocusManager) Current() PanelID {
	if f == nil {
		return PanelTracks
	}
	return f.current
}

// Set moves the focus to id. Unknown ids are clamped to the
// nearest valid value.
func (f *FocusManager) Set(id PanelID) {
	if f == nil {
		return
	}
	if id < PanelLibrary {
		id = PanelLibrary
	}
	if id > PanelQueue {
		id = PanelQueue
	}
	f.current = id
}

// Cycle moves the focus to the next panel, wrapping back to
// PanelLibrary after PanelQueue.
func (f *FocusManager) Cycle() {
	if f == nil {
		return
	}
	f.current = nextPanelID(f.current, +1)
}

// CycleBack moves the focus to the previous panel, wrapping
// forward to PanelQueue after PanelLibrary.
func (f *FocusManager) CycleBack() {
	if f == nil {
		return
	}
	f.current = nextPanelID(f.current, -1)
}

// nextPanelID returns the id offset by delta positions, wrapping
// around the [PanelLibrary, PanelQueue] range.
func nextPanelID(id PanelID, delta int) PanelID {
	ids := []PanelID{PanelLibrary, PanelTracks, PanelQueue}
	cur := 0
	for i, x := range ids {
		if x == id {
			cur = i
			break
		}
	}
	n := len(ids)
	cur = (cur + delta) % n
	if cur < 0 {
		cur += n
	}
	return ids[cur]
}
