package library

import "testing"

func queueTracks(paths ...string) []Track {
	tracks := make([]Track, len(paths))
	for i, path := range paths {
		tracks[i] = Track{Path: path, Title: path}
	}
	return tracks
}

func TestQueuePlaybackOrder(t *testing.T) {
	q := NewQueue(queueTracks("a", "b", "c"))
	if q.Len() != 3 || q.Current().Path != "a" {
		t.Fatalf("initial queue = %#v index %d", q.Tracks(), q.Index())
	}
	if !q.Next() || q.Current().Path != "b" {
		t.Fatalf("Next did not select b: %#v", q.Current())
	}
	if !q.Previous() || q.Current().Path != "a" {
		t.Fatalf("Previous did not select a: %#v", q.Current())
	}
	if q.Previous() {
		t.Fatal("Previous must not wrap")
	}
	q.SetIndex(99)
	if q.Current().Path != "c" || q.Next() {
		t.Fatal("SetIndex must clamp and Next must stop at the tail")
	}
}

func TestQueueMutationsKeepCurrentByPath(t *testing.T) {
	q := NewQueue(queueTracks("a", "b", "c"))
	q.SetIndex(1)
	if !q.MoveUp(2) {
		t.Fatal("MoveUp failed")
	}
	if got := q.Current().Path; got != "b" {
		t.Fatalf("current after move = %q, want b", got)
	}
	if !q.Remove(0) {
		t.Fatal("Remove failed")
	}
	if got := q.Current().Path; got != "b" {
		t.Fatalf("current after remove = %q, want b", got)
	}
	q.Clear()
	if q.Len() != 0 || q.Current() != nil || q.Index() != 0 {
		t.Fatalf("queue not empty after Clear: %#v index %d", q.Tracks(), q.Index())
	}
}

func TestQueueCopiesInputAndOutput(t *testing.T) {
	input := queueTracks("a", "b")
	q := NewQueue(input)
	input[0].Path = "changed"
	output := q.Tracks()
	output[0].Path = "also changed"
	if got := q.Tracks()[0].Path; got != "a" {
		t.Fatalf("queue leaked its storage: %q", got)
	}
}

func TestQueueNilReceiver(t *testing.T) {
	var q *Queue
	if q.Len() != 0 || q.Index() != 0 || q.Current() != nil || q.Next() || q.Previous() {
		t.Fatal("nil queue methods must be safe")
	}
	q.SetIndex(1)
	q.Replace(nil)
	q.Clear()
}

func TestQueueOccurrenceIDsSurviveMutationsAndReconcile(t *testing.T) {
	q := NewQueue(queueTracks("duplicate", "duplicate", "tail"))
	entries := q.Entries()
	if entries[0].ID == 0 || entries[0].ID == entries[1].ID {
		t.Fatalf("duplicate occurrence IDs = %d and %d", entries[0].ID, entries[1].ID)
	}
	q.SetCurrentID(entries[1].ID)
	if !q.MoveDown(1) {
		t.Fatal("MoveDown failed")
	}
	if got := q.CurrentEntry().ID; got != entries[1].ID {
		t.Fatalf("current ID after move = %d, want %d", got, entries[1].ID)
	}
	if !q.Remove(0) {
		t.Fatal("Remove failed")
	}
	if got := q.CurrentEntry().ID; got != entries[1].ID {
		t.Fatalf("current ID after remove = %d, want %d", got, entries[1].ID)
	}

	before := q.Entries()
	q.Reconcile([]Track{before[1].Track, before[0].Track})
	after := q.Entries()
	if after[0].ID != before[1].ID || after[1].ID != before[0].ID {
		t.Fatalf("IDs after reorder = [%d %d], want [%d %d]", after[0].ID, after[1].ID, before[1].ID, before[0].ID)
	}
	if got := q.CurrentEntry().ID; got != entries[1].ID {
		t.Fatalf("current ID after reconcile = %d, want %d", got, entries[1].ID)
	}
}

func TestQueueReplacementUsesMonotonicFreshIDs(t *testing.T) {
	q := NewQueue(queueTracks("a", "b"))
	old := q.Entries()
	q.Replace(queueTracks("a", "b"))
	fresh := q.Entries()
	if fresh[0].ID <= old[1].ID || fresh[1].ID <= fresh[0].ID {
		t.Fatalf("replacement IDs = [%d %d], previous maximum %d", fresh[0].ID, fresh[1].ID, old[1].ID)
	}
}
