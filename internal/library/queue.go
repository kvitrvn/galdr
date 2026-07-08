package library

// Queue manages the current list of tracks and the selected index.
//
// A Queue never wraps: Next stops at the last track and Previous stops at
// the first. Wrap-around is intentionally not part of the MVP.
type Queue struct {
	tracks []Track
	index  int
}

// NewQueue creates a queue preloaded with the given tracks.
//
// A nil tracks slice produces a valid empty queue.
func NewQueue(tracks []Track) *Queue {
	return &Queue{tracks: tracks}
}

// Len returns the number of tracks in the queue.
func (q *Queue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.tracks)
}

// Index returns the current selection index.
func (q *Queue) Index() int {
	if q == nil {
		return 0
	}
	return q.index
}

// Current returns the currently selected track, or nil if the queue is empty.
func (q *Queue) Current() *Track {
	if q == nil || len(q.tracks) == 0 {
		return nil
	}
	return &q.tracks[q.index]
}

// Tracks returns a copy of the underlying track slice.
func (q *Queue) Tracks() []Track {
	if q == nil {
		return nil
	}
	out := make([]Track, len(q.tracks))
	copy(out, q.tracks)
	return out
}

// SetIndex moves the selection to i. Values outside [0, Len()) are clamped.
//
// SetIndex on an empty queue is a no-op.
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
	q.index = i
}

// Next advances the selection by one. It returns true if the index changed.
//
// On an empty queue, on a single-track queue, or when already at the last
// track, Next leaves the index unchanged and returns false.
func (q *Queue) Next() bool {
	if q == nil || len(q.tracks) == 0 {
		return false
	}
	if q.index >= len(q.tracks)-1 {
		return false
	}
	q.index++
	return true
}

// Previous moves the selection back by one. It returns true if the index
// changed.
//
// On an empty queue, on a single-track queue, or when already at the first
// track, Previous leaves the index unchanged and returns false.
func (q *Queue) Previous() bool {
	if q == nil || len(q.tracks) == 0 {
		return false
	}
	if q.index <= 0 {
		return false
	}
	q.index--
	return true
}

// Replace swaps the queue contents. The index resets to 0.
func (q *Queue) Replace(tracks []Track) {
	if q == nil {
		return
	}
	q.tracks = tracks
	q.index = 0
}
