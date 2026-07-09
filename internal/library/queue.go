package library

import "strings"

// Queue manages the current list of tracks, the selected index, and an
// optional substring filter.
//
// The queue holds two pieces of state:
//
//   - The full list of tracks (set by Replace).
//   - The selected index, which is always a position in the full list.
//
// When a filter is active, the "visible" view of the queue is the
// subset of tracks whose Title, Artist or Album contains the pattern
// (case-insensitive). The visible view is recomputed from the full
// list every time the pattern changes; it is never held as a separate
// authoritative copy. Next, Previous, SetIndex and the current track
// are all defined in terms of the visible view, so the user never
// lands on a track that has been filtered out.
//
// A Queue never wraps: Next stops at the last visible track and
// Previous stops at the first. Wrap-around is intentionally not part
// of the MVP.
type Queue struct {
	tracks  []Track
	index   int
	pattern string
	visible []int // indices into tracks; nil when the filter is empty
}

// NewQueue creates a queue preloaded with the given tracks.
//
// A nil tracks slice produces a valid empty queue.
func NewQueue(tracks []Track) *Queue {
	return &Queue{tracks: tracks}
}

// Len returns the total number of tracks in the queue, regardless of
// the active filter.
func (q *Queue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.tracks)
}

// VisibleLen returns the number of tracks matching the current
// filter. It equals Len() when no filter is set.
func (q *Queue) VisibleLen() int {
	if q == nil {
		return 0
	}
	if q.visible == nil {
		return len(q.tracks)
	}
	return len(q.visible)
}

// Index returns the selected track's position in the full list.
// Callers that render the visible view should use DisplayIndex
// instead.
func (q *Queue) Index() int {
	if q == nil {
		return 0
	}
	return q.index
}

// DisplayIndex returns the selected track's position within the
// visible (filtered) view. It equals Index() when no filter is set.
// It returns -1 when the selected track is not in the visible view
// (which can happen transiently when a new filter hides it).
func (q *Queue) DisplayIndex() int {
	if q == nil {
		return 0
	}
	if q.visible == nil {
		return q.index
	}
	for i, idx := range q.visible {
		if idx == q.index {
			return i
		}
	}
	return -1
}

// Current returns the currently selected track, or nil if the queue
// is empty or the selection is hidden by the active filter. The
// selection follows the filter: when a filter hides the current
// track, Current returns nil until the selection is moved into the
// visible view.
func (q *Queue) Current() *Track {
	if q == nil || len(q.tracks) == 0 {
		return nil
	}
	if q.index < 0 || q.index >= len(q.tracks) {
		return nil
	}
	if !q.matchesIndex(q.index) {
		return nil
	}
	return &q.tracks[q.index]
}

// Tracks returns a defensive copy of the full track slice. It is
// unaffected by the filter and is intended for the rescan path and
// the "N total tracks" status line.
func (q *Queue) Tracks() []Track {
	if q == nil {
		return nil
	}
	out := make([]Track, len(q.tracks))
	copy(out, q.tracks)
	return out
}

// VisibleTracks returns a defensive copy of the currently visible
// tracks (the subset that passes the filter). The TUI renders this
// slice; the App uses it to know what the user is looking at.
func (q *Queue) VisibleTracks() []Track {
	if q == nil || len(q.tracks) == 0 {
		return nil
	}
	if q.visible == nil {
		out := make([]Track, len(q.tracks))
		copy(out, q.tracks)
		return out
	}
	out := make([]Track, len(q.visible))
	for i, idx := range q.visible {
		out[i] = q.tracks[idx]
	}
	return out
}

// Filter returns the active filter pattern. An empty string means
// no filter.
func (q *Queue) Filter() string {
	if q == nil {
		return ""
	}
	return q.pattern
}

// SetFilter sets the filter pattern. The visible view is recomputed
// and the selection is moved to the first visible track when the
// current selection is no longer visible. An empty pattern clears
// the filter.
func (q *Queue) SetFilter(pattern string) {
	if q == nil {
		return
	}
	if pattern == q.pattern {
		return
	}
	q.pattern = pattern
	if pattern == "" {
		q.visible = nil
	} else {
		q.visible = q.computeVisible(pattern)
	}
	if len(q.tracks) > 0 && !q.matchesIndex(q.index) {
		if len(q.visible) > 0 {
			q.index = q.visible[0]
		} else {
			q.index = 0
		}
	}
}

// SetIndex moves the selection to i, a position in the full list.
// Values outside [0, Len()) are clamped. When a filter is active,
// the index is only set if the target is part of the visible view;
// otherwise the call is a no-op.
func (q *Queue) SetIndex(i int) {
	if q == nil || len(q.tracks) == 0 {
		return
	}
	if i < 0 {
		i = 0
	}
	if i >= len(q.tracks) {
		i = len(q.tracks) - 1
	}
	if !q.matchesIndex(i) {
		return
	}
	q.index = i
}

// Next moves the selection to the next visible track. It returns
// true if the selection changed. On an empty queue, on a
// single-track visible view, or when already at the last visible
// track, Next leaves the index unchanged and returns false.
func (q *Queue) Next() bool {
	if q == nil || len(q.tracks) == 0 {
		return false
	}
	cur := q.DisplayIndex()
	if cur < 0 {
		return q.firstVisible()
	}
	if cur >= q.VisibleLen()-1 {
		return false
	}
	q.index = q.fullAt(cur + 1)
	return true
}

// Previous moves the selection to the previous visible track. It
// returns true if the selection changed. On an empty queue, on a
// single-track visible view, or when already at the first visible
// track, Previous leaves the index unchanged and returns false.
func (q *Queue) Previous() bool {
	if q == nil || len(q.tracks) == 0 {
		return false
	}
	cur := q.DisplayIndex()
	if cur <= 0 {
		return false
	}
	q.index = q.fullAt(cur - 1)
	return true
}

// FirstVisible moves the selection to the first visible track. It
// returns true if the selection changed (or if the queue became
// non-empty). It is used by repeat-all to wrap to the start of the
// visible view at the end of the queue.
func (q *Queue) FirstVisible() bool {
	if q == nil {
		return false
	}
	return q.firstVisible()
}

// SetDisplayIndex moves the selection to the i-th visible track. It
// is equivalent to SetIndex(fullAt(i)) and is provided so that
// callers (notably the shuffle path) can pick a visible position
// without having to walk the mapping themselves. It returns false
// when the visible view is empty or the index is out of range.
func (q *Queue) SetDisplayIndex(i int) bool {
	if q == nil {
		return false
	}
	n := q.VisibleLen()
	if n == 0 || i < 0 || i >= n {
		return false
	}
	q.index = q.fullAt(i)
	return true
}

// Replace swaps the full track list and clears the active filter.
// The index resets to 0.
//
// A rescan is treated as a "fresh start": the user has asked to
// re-read the library from disk, so any earlier search pattern is
// discarded. This is simpler and less surprising than preserving a
// filter that the new library might not satisfy.
func (q *Queue) Replace(tracks []Track) {
	if q == nil {
		return
	}
	q.tracks = tracks
	q.index = 0
	q.pattern = ""
	q.visible = nil
}

// MoveUp swaps the track at position i with the track at position
// i-1, moving the track one step toward the head of the queue.
// Returns true when the swap happened.
//
// The current selection (queue.Index) follows the moving track by
// path, so the playing track remains "selected" across the move.
// No-op when the queue is empty, when i is out of range, or when
// i is 0 (already at the head).
func (q *Queue) MoveUp(i int) bool {
	if q == nil {
		return false
	}
	if i <= 0 || i >= len(q.tracks) {
		return false
	}
	snap := q.snapshot()
	q.tracks[i-1], q.tracks[i] = q.tracks[i], q.tracks[i-1]
	q.restore(snap)
	return true
}

// MoveDown swaps the track at position i with the track at position
// i+1, moving the track one step toward the tail of the queue.
// Returns true when the swap happened.
//
// The current selection (queue.Index) follows the moving track by
// path. No-op when the queue is empty, when i is out of range, or
// when i is the last position.
func (q *Queue) MoveDown(i int) bool {
	if q == nil {
		return false
	}
	if i < 0 || i >= len(q.tracks)-1 {
		return false
	}
	snap := q.snapshot()
	q.tracks[i], q.tracks[i+1] = q.tracks[i+1], q.tracks[i]
	q.restore(snap)
	return true
}

// Remove deletes the track at position i. The currently-playing
// track (queue.Index) cannot be removed: it is a no-op when i
// equals the playing index. Returns true when a track was removed.
//
// When a track is removed at position < index, the playing
// index shifts down by one so the same track remains selected.
func (q *Queue) Remove(i int) bool {
	if q == nil {
		return false
	}
	if i < 0 || i >= len(q.tracks) {
		return false
	}
	if i == q.index {
		return false
	}
	snap := q.snapshot()
	q.tracks = append(q.tracks[:i], q.tracks[i+1:]...)
	q.restore(snap)
	// Recompute the visible view: with one track gone, the
	// filter's index list may now point past the end.
	if q.pattern != "" {
		q.visible = q.computeVisible(q.pattern)
	}
	return true
}

// Clear removes every track except the currently-playing one. The
// playing track becomes the only entry, and the index resets to 0.
// The filter is preserved (the user can keep searching within the
// remaining single track, although it is rarely useful).
func (q *Queue) Clear() {
	if q == nil || len(q.tracks) == 0 {
		return
	}
	if q.index < 0 || q.index >= len(q.tracks) {
		q.tracks = nil
		q.index = 0
		q.visible = nil
		return
	}
	cur := q.tracks[q.index]
	q.tracks = []Track{cur}
	q.index = 0
	q.visible = nil
}

// snapshot returns a defensive copy of the currently-playing
// track. An empty Track means "no track was selected". Used by
// the mutation methods to relocate the selection after a move or
// removal.
func (q *Queue) snapshot() Track {
	if q.index < 0 || q.index >= len(q.tracks) {
		return Track{}
	}
	return q.tracks[q.index]
}

// restore updates q.index so that it points to the same path as
// snap, after a mutation that may have changed the order. When
// snap's path is no longer in the list, the index is clamped to a
// valid range (0 when the queue is empty, len-1 otherwise).
func (q *Queue) restore(snap Track) {
	if snap.Path == "" {
		if len(q.tracks) == 0 {
			q.index = 0
		} else if q.index >= len(q.tracks) {
			q.index = len(q.tracks) - 1
		}
		return
	}
	for i, t := range q.tracks {
		if t.Path == snap.Path {
			q.index = i
			return
		}
	}
	// Track is gone. Clamp to a safe index.
	if len(q.tracks) == 0 {
		q.index = 0
	} else {
		q.index = len(q.tracks) - 1
	}
}

// matchesIndex reports whether the full-list position i is part of
// the visible view.
func (q *Queue) matchesIndex(i int) bool {
	if q.visible == nil {
		return i >= 0 && i < len(q.tracks)
	}
	for _, idx := range q.visible {
		if idx == i {
			return true
		}
	}
	return false
}

// fullAt returns the full-list position of the i-th visible track.
// When the filter is empty, fullAt is the identity function.
func (q *Queue) fullAt(i int) int {
	if q.visible == nil {
		return i
	}
	return q.visible[i]
}

// firstVisible moves the selection to the first visible track and
// returns true when the move happened. An empty visible view leaves
// the index at 0 and returns false.
func (q *Queue) firstVisible() bool {
	if q.VisibleLen() == 0 {
		q.index = 0
		return false
	}
	q.index = q.fullAt(0)
	return true
}

// computeVisible returns the sorted list of full-list indices whose
// track matches the substring pattern (case-insensitive) on
// Title, Artist or Album.
func (q *Queue) computeVisible(pattern string) []int {
	p := strings.ToLower(pattern)
	out := make([]int, 0, len(q.tracks))
	for i, t := range q.tracks {
		if strings.Contains(strings.ToLower(t.Title), p) ||
			strings.Contains(strings.ToLower(t.Artist), p) ||
			strings.Contains(strings.ToLower(t.Album), p) {
			out = append(out, i)
		}
	}
	return out
}
