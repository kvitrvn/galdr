package library

import "testing"

func mkTracks(n int) []Track {
	out := make([]Track, n)
	for i := range out {
		out[i] = Track{Path: string(rune('a'+i)) + ".mp3", Title: "T"}
	}
	return out
}

func TestQueue_Empty(t *testing.T) {
	q := NewQueue(nil)

	if q.Len() != 0 {
		t.Errorf("Len = %d, want 0", q.Len())
	}
	if got := q.Current(); got != nil {
		t.Errorf("Current = %+v, want nil", got)
	}
	if q.Index() != 0 {
		t.Errorf("Index = %d, want 0", q.Index())
	}
	if q.Next() {
		t.Error("Next on empty queue should return false")
	}
	if q.Previous() {
		t.Error("Previous on empty queue should return false")
	}
	if q.Index() != 0 {
		t.Errorf("Index after Next/Previous = %d, want 0", q.Index())
	}
	q.SetIndex(5)
	if q.Index() != 0 {
		t.Errorf("Index after SetIndex on empty = %d, want 0", q.Index())
	}
}

func TestQueue_NilReceiverSafe(t *testing.T) {
	var q *Queue
	if q.Len() != 0 {
		t.Error("nil Len should be 0")
	}
	if q.Index() != 0 {
		t.Error("nil Index should be 0")
	}
	if got := q.Current(); got != nil {
		t.Error("nil Current should be nil")
	}
	if q.Next() || q.Previous() {
		t.Error("nil Next/Previous should return false")
	}
	q.SetIndex(3)
	q.Replace(mkTracks(2))
}

func TestQueue_Single(t *testing.T) {
	tracks := mkTracks(1)
	q := NewQueue(tracks)

	if q.Len() != 1 {
		t.Errorf("Len = %d, want 1", q.Len())
	}
	if got := q.Current(); got == nil || got.Path != "a.mp3" {
		t.Errorf("Current = %+v, want a.mp3", got)
	}

	if q.Next() {
		t.Error("Next on single-track queue should return false")
	}
	if q.Index() != 0 {
		t.Errorf("Index after Next on single = %d, want 0", q.Index())
	}
	if q.Previous() {
		t.Error("Previous on single-track queue should return false")
	}
	if q.Index() != 0 {
		t.Errorf("Index after Previous on single = %d, want 0", q.Index())
	}
}

func TestQueue_NextPrevious_Multiple(t *testing.T) {
	tracks := mkTracks(3)
	q := NewQueue(tracks)

	if q.Index() != 0 {
		t.Fatalf("initial Index = %d, want 0", q.Index())
	}

	if !q.Next() || q.Index() != 1 {
		t.Errorf("Next: idx=%d ok=%v, want idx=1 ok=true", q.Index(), true)
	}
	if !q.Next() || q.Index() != 2 {
		t.Errorf("Next: idx=%d, want 2", q.Index())
	}
	if q.Next() {
		t.Errorf("Next at last track should return false (idx=%d)", q.Index())
	}
	if q.Index() != 2 {
		t.Errorf("Index after Next at end = %d, want 2", q.Index())
	}

	if !q.Previous() || q.Index() != 1 {
		t.Errorf("Previous: idx=%d, want 1", q.Index())
	}
	if !q.Previous() || q.Index() != 0 {
		t.Errorf("Previous: idx=%d, want 0", q.Index())
	}
	if q.Previous() {
		t.Errorf("Previous at first track should return false (idx=%d)", q.Index())
	}
	if q.Index() != 0 {
		t.Errorf("Index after Previous at start = %d, want 0", q.Index())
	}
}

func TestQueue_CurrentFollowsIndex(t *testing.T) {
	tracks := mkTracks(3)
	q := NewQueue(tracks)

	if got := q.Current(); got == nil || got.Path != "a.mp3" {
		t.Fatalf("Current @0 = %+v, want a.mp3", got)
	}
	q.Next()
	if got := q.Current(); got == nil || got.Path != "b.mp3" {
		t.Errorf("Current @1 = %+v, want b.mp3", got)
	}
	q.Next()
	if got := q.Current(); got == nil || got.Path != "c.mp3" {
		t.Errorf("Current @2 = %+v, want c.mp3", got)
	}
}

func TestQueue_SetIndexBounds(t *testing.T) {
	tracks := mkTracks(3)
	q := NewQueue(tracks)

	cases := []struct {
		name string
		in   int
		want int
	}{
		{"middle", 1, 1},
		{"last", 2, 2},
		{"negative clamps to 0", -5, 0},
		{"too high clamps to last", 99, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q.SetIndex(c.in)
			if q.Index() != c.want {
				t.Errorf("after SetIndex(%d): Index = %d, want %d", c.in, q.Index(), c.want)
			}
		})
	}
}

func TestQueue_Replace(t *testing.T) {
	q := NewQueue(mkTracks(3))
	q.SetIndex(2)

	q.Replace(mkTracks(2))
	if q.Len() != 2 {
		t.Errorf("Len after Replace = %d, want 2", q.Len())
	}
	if q.Index() != 0 {
		t.Errorf("Index after Replace = %d, want 0", q.Index())
	}

	q.Replace(nil)
	if q.Len() != 0 {
		t.Errorf("Len after Replace(nil) = %d, want 0", q.Len())
	}
	if got := q.Current(); got != nil {
		t.Errorf("Current after Replace(nil) = %+v, want nil", got)
	}
}

func TestQueue_TracksReturnsCopy(t *testing.T) {
	tracks := mkTracks(2)
	q := NewQueue(tracks)

	got := q.Tracks()
	if len(got) != 2 {
		t.Fatalf("len(Tracks()) = %d, want 2", len(got))
	}

	got[0].Title = "mutated"
	if q.Tracks()[0].Title == "mutated" {
		t.Error("Tracks() must return a defensive copy")
	}
}
