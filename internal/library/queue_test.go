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

// --- Phase 15: queue manipulation ---

func TestQueue_MoveUp(t *testing.T) {
	tracks := []Track{
		{Path: "a", Title: "A"},
		{Path: "b", Title: "B"},
		{Path: "c", Title: "C"},
	}
	q := NewQueue(tracks)
	q.SetIndex(2) // playing C

	if !q.MoveUp(2) {
		t.Fatal("MoveUp(2) = false, want true")
	}
	// Order: A, C, B
	if q.Tracks()[0].Title != "A" || q.Tracks()[1].Title != "C" || q.Tracks()[2].Title != "B" {
		t.Errorf("order after MoveUp = %v, want [A, C, B]", titlesOf(q.Tracks()))
	}
	// Index follows C (was at 2, now at 1).
	if q.Index() != 1 {
		t.Errorf("Index = %d, want 1 (followed C)", q.Index())
	}
}

func TestQueue_MoveUp_NoOpAtHead(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}})
	if q.MoveUp(0) {
		t.Error("MoveUp(0) should be a no-op")
	}
	if q.Index() != 0 {
		t.Errorf("Index = %d, want 0", q.Index())
	}
}

func TestQueue_MoveUp_OutOfRange(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}})
	if q.MoveUp(5) {
		t.Error("MoveUp(5) should be a no-op")
	}
	if q.MoveUp(-1) {
		t.Error("MoveUp(-1) should be a no-op")
	}
}

func TestQueue_MoveUp_DoesNotAffectOtherIndex(t *testing.T) {
	q := NewQueue([]Track{
		{Path: "a", Title: "A"},
		{Path: "b", Title: "B"},
		{Path: "c", Title: "C"},
	})
	q.SetIndex(0) // playing A
	q.MoveUp(2)   // moves C up
	// Order: A, C, B
	if q.Tracks()[0].Title != "A" || q.Tracks()[1].Title != "C" || q.Tracks()[2].Title != "B" {
		t.Errorf("order = %v, want [A, C, B]", titlesOf(q.Tracks()))
	}
	// Index is still 0 (A is still there at the same position).
	if q.Index() != 0 {
		t.Errorf("Index = %d, want 0 (A did not move)", q.Index())
	}
}

func TestQueue_MoveDown(t *testing.T) {
	tracks := []Track{
		{Path: "a", Title: "A"},
		{Path: "b", Title: "B"},
		{Path: "c", Title: "C"},
	}
	q := NewQueue(tracks)
	q.SetIndex(0) // playing A

	if !q.MoveDown(0) {
		t.Fatal("MoveDown(0) = false, want true")
	}
	// Order: B, A, C
	if q.Tracks()[0].Title != "B" || q.Tracks()[1].Title != "A" || q.Tracks()[2].Title != "C" {
		t.Errorf("order after MoveDown = %v, want [B, A, C]", titlesOf(q.Tracks()))
	}
	if q.Index() != 1 {
		t.Errorf("Index = %d, want 1 (followed A)", q.Index())
	}
}

func TestQueue_MoveDown_NoOpAtTail(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}})
	if q.MoveDown(1) {
		t.Error("MoveDown(1) should be a no-op")
	}
}

func TestQueue_Remove_NonPlaying(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}, {Path: "c"}})
	q.SetIndex(1) // playing B

	if !q.Remove(2) {
		t.Fatal("Remove(2) = false, want true")
	}
	// Order: A, B
	if got := q.Tracks(); len(got) != 2 || got[0].Path != "a" || got[1].Path != "b" {
		t.Errorf("after Remove(2) = %v, want [a, b]", titlesOf(got))
	}
	if q.Index() != 1 {
		t.Errorf("Index = %d, want 1 (B still playing)", q.Index())
	}
}

func TestQueue_Remove_PlayingIsNoOp(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}, {Path: "c"}})
	q.SetIndex(1)
	if q.Remove(1) {
		t.Error("Remove on the currently-playing track should be a no-op")
	}
	if got := q.Tracks(); len(got) != 3 {
		t.Errorf("queue should still have 3 tracks, got %d", len(got))
	}
}

func TestQueue_Remove_ShiftsIndexDown(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}, {Path: "c"}})
	q.SetIndex(2) // playing C
	q.Remove(0)   // remove A
	// Order: B, C; C is now at index 1.
	if got := q.Tracks(); len(got) != 2 || got[0].Path != "b" || got[1].Path != "c" {
		t.Errorf("after Remove(0) = %v, want [b, c]", titlesOf(got))
	}
	if q.Index() != 1 {
		t.Errorf("Index = %d, want 1", q.Index())
	}
}

func TestQueue_Remove_OutOfRange(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}})
	if q.Remove(5) || q.Remove(-1) {
		t.Error("Remove out-of-range should be a no-op")
	}
	if len(q.Tracks()) != 2 {
		t.Error("queue should still have 2 tracks")
	}
}

func TestQueue_Clear_KeepsPlaying(t *testing.T) {
	q := NewQueue([]Track{{Path: "a"}, {Path: "b"}, {Path: "c"}})
	q.SetIndex(1) // playing B
	q.Clear()
	if got := q.Tracks(); len(got) != 1 || got[0].Path != "b" {
		t.Errorf("after Clear = %v, want [b]", titlesOf(got))
	}
	if q.Index() != 0 {
		t.Errorf("Index after Clear = %d, want 0", q.Index())
	}
}

func TestQueue_Clear_OnEmpty(t *testing.T) {
	q := NewQueue(nil)
	q.Clear()
	if got := q.Tracks(); len(got) != 0 {
		t.Errorf("after Clear on empty = %v, want []", titlesOf(got))
	}
}

func titlesOf(tracks []Track) []string {
	out := make([]string, len(tracks))
	for i, t := range tracks {
		out[i] = t.Title
	}
	return out
}
