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

func mkLibraryTracks() []Track {
	return []Track{
		{Path: "a.mp3", Title: "Anthem", Artist: "Helloween", Album: "Keeper"},
		{Path: "b.mp3", Title: "A Tale That Wasn't Right", Artist: "Helloween", Album: "Keeper"},
		{Path: "c.mp3", Title: "Limbo", Artist: "Igorrr", Album: "Amen"},
		{Path: "d.mp3", Title: "Paranoid", Artist: "Black Sabbath", Album: "Paranoid"},
		{Path: "e.mp3", Title: "Amen", Artist: "Igorrr", Album: "Amen"},
	}
}

func TestQueue_Filter_NoPattern_AllVisible(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	if got := q.VisibleLen(); got != 5 {
		t.Errorf("VisibleLen = %d, want 5", got)
	}
	if got := q.VisibleTracks(); len(got) != 5 {
		t.Errorf("len(VisibleTracks) = %d, want 5", len(got))
	}
	if got := q.DisplayIndex(); got != 0 {
		t.Errorf("DisplayIndex = %d, want 0", got)
	}
}

func TestQueue_Filter_TitleMatch(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("limbo")

	if got := q.VisibleLen(); got != 1 {
		t.Errorf("VisibleLen = %d, want 1", got)
	}
	if got := q.VisibleTracks()[0].Path; got != "c.mp3" {
		t.Errorf("VisibleTracks[0] = %q, want c.mp3", got)
	}
}

func TestQueue_Filter_ArtistMatch(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("igorr")

	if got := q.VisibleLen(); got != 2 {
		t.Errorf("VisibleLen = %d, want 2", got)
	}
	if got := q.VisibleTracks()[0].Path; got != "c.mp3" {
		t.Errorf("VisibleTracks[0] = %q, want c.mp3", got)
	}
	if got := q.VisibleTracks()[1].Path; got != "e.mp3" {
		t.Errorf("VisibleTracks[1] = %q, want e.mp3", got)
	}
}

func TestQueue_Filter_AlbumMatch(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("amen")

	if got := q.VisibleLen(); got != 2 {
		t.Errorf("VisibleLen = %d, want 2", got)
	}
}

func TestQueue_Filter_CaseInsensitive(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("IGORR")
	if got := q.VisibleLen(); got != 2 {
		t.Errorf("VisibleLen (case) = %d, want 2", got)
	}
}

func TestQueue_Filter_NoMatch(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("nope")

	if got := q.VisibleLen(); got != 0 {
		t.Errorf("VisibleLen = %d, want 0", got)
	}
	if got := q.Current(); got != nil {
		t.Errorf("Current with no match = %+v, want nil", got)
	}
	if got := q.DisplayIndex(); got != -1 {
		t.Errorf("DisplayIndex with no match = %d, want -1", got)
	}
}

func TestQueue_Filter_NextSkipsHidden(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("igorr") // matches c (index 2) and e (index 4)

	if got := q.DisplayIndex(); got != 0 {
		t.Errorf("initial DisplayIndex = %d, want 0", got)
	}
	if !q.Next() {
		t.Fatal("Next should succeed")
	}
	if got := q.Index(); got != 4 {
		t.Errorf("Index after Next = %d, want 4", got)
	}
	if q.Next() {
		t.Error("Next at end of filtered view should return false")
	}
}

func TestQueue_Filter_PreviousSkipsHidden(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("igorr")
	// Selection jumped to first match (index 2). Move forward then back.
	q.Next() // -> index 4
	if !q.Previous() {
		t.Fatal("Previous should succeed")
	}
	if got := q.Index(); got != 2 {
		t.Errorf("Index after Previous = %d, want 2", got)
	}
	if q.Previous() {
		t.Error("Previous at first match should return false")
	}
}

func TestQueue_Filter_MovesSelectionWhenHidden(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetIndex(3) // Paranoid, not a match for "igorr"
	q.SetFilter("igorr")

	if got := q.DisplayIndex(); got != 0 {
		t.Errorf("DisplayIndex after SetFilter = %d, want 0 (moved to first match)", got)
	}
	if got := q.Index(); got != 2 {
		t.Errorf("Index after SetFilter = %d, want 2", got)
	}
}

func TestQueue_Filter_ReplaceClearsFilter(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("igorr")
	q.Replace(mkLibraryTracks())

	if got := q.Filter(); got != "" {
		t.Errorf("Filter after Replace = %q, want empty", got)
	}
	if got := q.VisibleLen(); got != 5 {
		t.Errorf("VisibleLen after Replace = %d, want 5", got)
	}
}

func TestQueue_Filter_VisibleTracksReturnsCopy(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("igorr")
	got := q.VisibleTracks()
	if len(got) != 2 {
		t.Fatalf("len(VisibleTracks) = %d, want 2", len(got))
	}
	got[0].Title = "mutated"
	if q.VisibleTracks()[0].Title == "mutated" {
		t.Error("VisibleTracks() must return a defensive copy")
	}
}

func TestQueue_Filter_SetIndexRejectsHidden(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("igorr")
	q.SetIndex(0) // Anthem, hidden by filter
	if got := q.Index(); got == 0 {
		t.Errorf("SetIndex on hidden track should be a no-op, got Index=0")
	}
}

func TestQueue_Filter_FirstVisible(t *testing.T) {
	q := NewQueue(mkLibraryTracks())
	q.SetFilter("igorr")
	q.SetIndex(0) // rejected (hidden)
	q.FirstVisible()
	if got := q.Index(); got != 2 {
		t.Errorf("Index after FirstVisible = %d, want 2", got)
	}
}
