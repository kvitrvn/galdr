package library

// Queue is the active playback order. Its index identifies the track that is
// currently loaded (or was most recently loaded). Catalogue navigation and
// search deliberately live in app.App, outside this type.
type Queue struct {
	entries []QueueEntry
	index   int
	nextID  QueueEntryID
}

// QueueEntryID identifies one occurrence in a playback queue. The same Track
// may occur more than once; each occurrence receives a distinct ID.
type QueueEntryID uint64

// QueueEntry couples a track with its stable identity in the active queue.
type QueueEntry struct {
	ID    QueueEntryID
	Track Track
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
	return len(q.entries)
}

func (q *Queue) Index() int {
	if q == nil {
		return 0
	}
	return q.index
}

func (q *Queue) Current() *Track {
	entry := q.CurrentEntry()
	if entry == nil {
		return nil
	}
	track := entry.Track
	return &track
}

// CurrentEntry returns a copy of the current queue occurrence.
func (q *Queue) CurrentEntry() *QueueEntry {
	if q == nil || q.index < 0 || q.index >= len(q.entries) {
		return nil
	}
	entry := q.entries[q.index]
	return &entry
}

func (q *Queue) Tracks() []Track {
	if q == nil {
		return nil
	}
	tracks := make([]Track, len(q.entries))
	for i, entry := range q.entries {
		tracks[i] = entry.Track
	}
	return tracks
}

// Entries returns a defensive copy of the active queue occurrences.
func (q *Queue) Entries() []QueueEntry {
	if q == nil {
		return nil
	}
	return append([]QueueEntry(nil), q.entries...)
}

func (q *Queue) SetIndex(index int) {
	if q == nil || len(q.entries) == 0 {
		return
	}
	if index < 0 {
		index = 0
	}
	if index >= len(q.entries) {
		index = len(q.entries) - 1
	}
	q.index = index
}

func (q *Queue) Next() bool {
	if q == nil || q.index >= len(q.entries)-1 {
		return false
	}
	q.index++
	return true
}

func (q *Queue) Previous() bool {
	if q == nil || q.index <= 0 || len(q.entries) == 0 {
		return false
	}
	q.index--
	return true
}

func (q *Queue) First() bool {
	if q == nil || len(q.entries) == 0 {
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
	q.entries = make([]QueueEntry, len(tracks))
	for i, track := range tracks {
		q.nextID++
		q.entries[i] = QueueEntry{ID: q.nextID, Track: track}
	}
	q.index = 0
}

// Append adds one occurrence to the tail of the active order.
func (q *Queue) Append(track Track) {
	q.Insert(len(q.entries), track)
}

// Insert adds one occurrence at index while preserving the currently selected
// occurrence. Out-of-range indexes are clamped to the queue boundaries.
func (q *Queue) Insert(index int, track Track) {
	if q == nil {
		return
	}
	if index < 0 {
		index = 0
	}
	if index > len(q.entries) {
		index = len(q.entries)
	}
	current := q.currentID()
	q.nextID++
	q.entries = append(q.entries, QueueEntry{})
	copy(q.entries[index+1:], q.entries[index:])
	q.entries[index] = QueueEntry{ID: q.nextID, Track: track}
	if current != 0 {
		q.relocate(current)
	} else {
		q.index = 0
	}
}

// ReplaceEntries installs occurrences without changing their identities.
// It is used when the application materializes a new order of an existing
// queue, such as enabling or disabling shuffle.
func (q *Queue) ReplaceEntries(entries []QueueEntry) {
	if q == nil {
		return
	}
	q.entries = append([]QueueEntry(nil), entries...)
	q.index = 0
	for _, entry := range entries {
		if entry.ID > q.nextID {
			q.nextID = entry.ID
		}
	}
}

// Reconcile installs tracks in the requested order while preserving the ID of
// each matching existing occurrence. Duplicate paths are matched once, from
// left to right. Newly introduced occurrences receive fresh IDs.
func (q *Queue) Reconcile(tracks []Track) {
	if q == nil {
		return
	}
	current := q.currentID()
	available := q.Entries()
	used := make([]bool, len(available))
	entries := make([]QueueEntry, len(tracks))
	for i, track := range tracks {
		matched := -1
		for j, entry := range available {
			if !used[j] && entry.Track.Path == track.Path {
				matched = j
				break
			}
		}
		if matched >= 0 {
			used[matched] = true
			entries[i] = QueueEntry{ID: available[matched].ID, Track: track}
			continue
		}
		q.nextID++
		entries[i] = QueueEntry{ID: q.nextID, Track: track}
	}
	q.entries = entries
	q.relocate(current)
}

// SetCurrentID selects the occurrence identified by id.
func (q *Queue) SetCurrentID(id QueueEntryID) bool {
	if q == nil {
		return false
	}
	for i, entry := range q.entries {
		if entry.ID == id {
			q.index = i
			return true
		}
	}
	return false
}

func (q *Queue) MoveUp(index int) bool {
	if q == nil || index <= 0 || index >= len(q.entries) {
		return false
	}
	current := q.currentID()
	q.entries[index-1], q.entries[index] = q.entries[index], q.entries[index-1]
	q.relocate(current)
	return true
}

func (q *Queue) MoveDown(index int) bool {
	if q == nil || index < 0 || index >= len(q.entries)-1 {
		return false
	}
	current := q.currentID()
	q.entries[index], q.entries[index+1] = q.entries[index+1], q.entries[index]
	q.relocate(current)
	return true
}

// Remove removes an item from the active order. Protection of the actually
// playing track belongs to App, which owns player state.
func (q *Queue) Remove(index int) bool {
	if q == nil || index < 0 || index >= len(q.entries) {
		return false
	}
	current := q.currentID()
	q.entries = append(q.entries[:index], q.entries[index+1:]...)
	q.relocate(current)
	return true
}

// Clear empties the playback order.
func (q *Queue) Clear() {
	if q != nil {
		q.Replace(nil)
	}
}

func (q *Queue) currentID() QueueEntryID {
	if current := q.CurrentEntry(); current != nil {
		return current.ID
	}
	return 0
}

func (q *Queue) relocate(id QueueEntryID) {
	for i := range q.entries {
		if q.entries[i].ID == id {
			q.index = i
			return
		}
	}
	if len(q.entries) == 0 {
		q.index = 0
	} else if q.index >= len(q.entries) {
		q.index = len(q.entries) - 1
	}
}
