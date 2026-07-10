package library

// Queue is the active playback order. Its index identifies the track that is
// currently loaded (or was most recently loaded). Catalogue navigation and
// search deliberately live in app.App, outside this type.
type Queue struct {
	tracks []Track
	index  int
}

// NewQueue creates a queue from tracks. The input slice is copied.
func NewQueue(tracks []Track) *Queue {
	q := &Queue{}
	q.Replace(tracks)
	return q
}

func (q *Queue) Len() int {
	if q == nil {
		return 0
	}
	return len(q.tracks)
}

func (q *Queue) Index() int {
	if q == nil {
		return 0
	}
	return q.index
}

func (q *Queue) Current() *Track {
	if q == nil || q.index < 0 || q.index >= len(q.tracks) {
		return nil
	}
	track := q.tracks[q.index]
	return &track
}

func (q *Queue) Tracks() []Track {
	if q == nil {
		return nil
	}
	return append([]Track(nil), q.tracks...)
}

func (q *Queue) SetIndex(index int) {
	if q == nil || len(q.tracks) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(q.tracks) {
		index = len(q.tracks) - 1
	}
	q.index = index
}

func (q *Queue) Next() bool {
	if q == nil || q.index >= len(q.tracks)-1 {
		return false
	}
	q.index++
	return true
}

func (q *Queue) Previous() bool {
	if q == nil || q.index <= 0 || len(q.tracks) == 0 {
		return false
	}
	q.index--
	return true
}

func (q *Queue) First() bool {
	if q == nil || len(q.tracks) == 0 {
		return false
	}
	changed := q.index != 0
	q.index = 0
	return changed
}

// FirstVisible is retained as a compatibility alias. A playback queue has no
// filtered view, so its first visible item is simply its first item.
func (q *Queue) FirstVisible() bool { return q.First() }

// Replace installs a new active order and resets the current index.
func (q *Queue) Replace(tracks []Track) {
	if q == nil {
		return
	}
	q.tracks = append([]Track(nil), tracks...)
	q.index = 0
}

func (q *Queue) MoveUp(index int) bool {
	if q == nil || index <= 0 || index >= len(q.tracks) {
		return false
	}
	current := q.currentPath()
	q.tracks[index-1], q.tracks[index] = q.tracks[index], q.tracks[index-1]
	q.relocate(current)
	return true
}

func (q *Queue) MoveDown(index int) bool {
	if q == nil || index < 0 || index >= len(q.tracks)-1 {
		return false
	}
	current := q.currentPath()
	q.tracks[index], q.tracks[index+1] = q.tracks[index+1], q.tracks[index]
	q.relocate(current)
	return true
}

// Remove removes an item from the active order. Protection of the actually
// playing track belongs to App, which owns player state.
func (q *Queue) Remove(index int) bool {
	if q == nil || index < 0 || index >= len(q.tracks) {
		return false
	}
	current := q.currentPath()
	q.tracks = append(q.tracks[:index], q.tracks[index+1:]...)
	q.relocate(current)
	return true
}

// Clear empties the playback order.
func (q *Queue) Clear() {
	if q != nil {
		q.Replace(nil)
	}
}

func (q *Queue) currentPath() string {
	if current := q.Current(); current != nil {
		return current.Path
	}
	return ""
}

func (q *Queue) relocate(path string) {
	for i := range q.tracks {
		if q.tracks[i].Path == path {
			q.index = i
			return
		}
	}
	if len(q.tracks) == 0 {
		q.index = 0
	} else if q.index >= len(q.tracks) {
		q.index = len(q.tracks) - 1
	}
}
