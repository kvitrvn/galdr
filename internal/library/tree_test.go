package library

import (
	"path/filepath"
	"testing"
)

// mkTrack builds a Track with a path under root/artist/album/file,
// and the given title. Other metadata fields are populated for
// filter tests.
func mkTrack(root, artist, album, file, title string) Track {
	return Track{
		Path:   filepath.Join(root, artist, album, file),
		Title:  title,
		Artist: artist,
		Album:  album,
	}
}

// mkTrackNoAlbum is a helper for "loose file at root" tests.
func mkTrackRoot(root, file, title string) Track {
	return Track{
		Path:   filepath.Join(root, file),
		Title:  title,
		Artist: "",
		Album:  "",
	}
}

func TestTree_NilSafe(t *testing.T) {
	var tr *Tree
	if got := tr.Len(); got != 0 {
		t.Errorf("nil Len = %d, want 0", got)
	}
	if got := tr.TotalTracks(); got != 0 {
		t.Errorf("nil TotalTracks = %d, want 0", got)
	}
	if got := tr.Filter(); got != "" {
		t.Errorf("nil Filter = %q, want empty", got)
	}
	if tr.HasFilter() {
		t.Error("nil HasFilter = true, want false")
	}
	if got := tr.Artists(); got != nil {
		t.Errorf("nil Artists = %+v, want nil", got)
	}
	if got := tr.ArtistTracks("anything"); got != nil {
		t.Errorf("nil ArtistTracks = %+v, want nil", got)
	}
	if got := tr.AlbumTracks("a", "b"); got != nil {
		t.Errorf("nil AlbumTracks = %+v, want nil", got)
	}
	if tr.HasArtist("a") {
		t.Error("nil HasArtist = true, want false")
	}
	tr.SetFilter("x")
}

func TestTree_Empty(t *testing.T) {
	tr := NewTree("/music", nil)
	if tr.Len() != 0 {
		t.Errorf("Len = %d, want 0", tr.Len())
	}
	if tr.TotalTracks() != 0 {
		t.Errorf("TotalTracks = %d, want 0", tr.TotalTracks())
	}
	if artists := tr.Artists(); len(artists) != 0 {
		t.Errorf("len(Artists) = %d, want 0", len(artists))
	}
}

func TestTree_BuildFromPaths_StandardLayout(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "Iron Maiden", "Powerslave", "01-aces-high.mp3", "Aces High"),
		mkTrack(root, "Iron Maiden", "Powerslave", "02-2-minutes-to-midnight.mp3", "2 Minutes to Midnight"),
		mkTrack(root, "Iron Maiden", "Somewhere in Time", "01-caught-somewhere-in-time.mp3", "Caught Somewhere in Time"),
		mkTrack(root, "Helloween", "Keeper of the Seven Keys", "01-anthem.mp3", "Anthem"),
	}
	tr := NewTree(root, tracks)

	if got := tr.Len(); got != 2 {
		t.Errorf("Len = %d, want 2 artists (Helloween, Iron Maiden)", got)
	}

	artists := tr.Artists()
	if artists[0].Name != "Helloween" {
		t.Errorf("artists[0] = %q, want Helloween (alphabetical)", artists[0].Name)
	}
	if artists[1].Name != "Iron Maiden" {
		t.Errorf("artists[1] = %q, want Iron Maiden (alphabetical)", artists[1].Name)
	}

	im := artists[1]
	if len(im.Albums) != 2 {
		t.Fatalf("Iron Maiden has %d albums, want 2", len(im.Albums))
	}
	if im.Albums[0].Name != "Powerslave" {
		t.Errorf("albums[0] = %q, want Powerslave", im.Albums[0].Name)
	}
	if im.Albums[0].TrackCount != 2 {
		t.Errorf("Powerslave track count = %d, want 2", im.Albums[0].TrackCount)
	}
	if im.Albums[1].Name != "Somewhere in Time" {
		t.Errorf("albums[1] = %q, want Somewhere in Time", im.Albums[1].Name)
	}
}

func TestTree_BuildFromPaths_TwoSegments(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "Greatest Hits", "01-song.mp3", "", "Song"),
	}
	// mkTrack uses artist/album/file, so for a 2-segment layout we
	// build the path manually.
	tracks[0].Path = filepath.Join(root, "Greatest Hits", "01-song.mp3")
	tracks[0].Artist = ""
	tracks[0].Album = "Greatest Hits"

	tr := NewTree(root, tracks)

	artists := tr.Artists()
	if len(artists) != 1 {
		t.Fatalf("len(artists) = %d, want 1", len(artists))
	}
	if artists[0].Name != UnknownArtist {
		t.Errorf("artist = %q, want %q", artists[0].Name, UnknownArtist)
	}
	if len(artists[0].Albums) != 1 {
		t.Fatalf("len(albums) = %d, want 1", len(artists[0].Albums))
	}
	if artists[0].Albums[0].Name != "Greatest Hits" {
		t.Errorf("album = %q, want Greatest Hits", artists[0].Albums[0].Name)
	}
}

func TestTree_BuildFromPaths_LooseFiles(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrackRoot(root, "loose.mp3", "Loose"),
	}
	tr := NewTree(root, tracks)

	artists := tr.Artists()
	if len(artists) != 1 {
		t.Fatalf("len(artists) = %d, want 1", len(artists))
	}
	if artists[0].Name != UnknownArtist {
		t.Errorf("artist = %q, want %q", artists[0].Name, UnknownArtist)
	}
	if artists[0].Albums[0].Name != UnknownAlbum {
		t.Errorf("album = %q, want %q", artists[0].Albums[0].Name, UnknownAlbum)
	}
}

func TestTree_UnknownsSortLast(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "Zappa", "Joe's Garage", "01.mp3", "Zappa 1"),
		mkTrackRoot(root, "loose.mp3", "Loose"),
		mkTrack(root, "Abba", "Arrival", "01.mp3", "Abba 1"),
	}
	tr := NewTree(root, tracks)

	artists := tr.Artists()
	if len(artists) != 3 {
		t.Fatalf("len(artists) = %d, want 3", len(artists))
	}
	if artists[0].Name != "Abba" || artists[1].Name != "Zappa" {
		t.Errorf("first two artists = [%q, %q], want [Abba, Zappa]", artists[0].Name, artists[1].Name)
	}
	if artists[2].Name != UnknownArtist {
		t.Errorf("artists[2] = %q, want %q (last)", artists[2].Name, UnknownArtist)
	}
}

func TestTree_UnknownArtistHasUnknownAlbumBucket(t *testing.T) {
	// Files at the root of the music dir land in the
	// (Unknown Artist) / (Unknown Album) bucket. Multiple loose
	// files share the same bucket.
	root := "/music"
	tracks := []Track{
		mkTrackRoot(root, "a.mp3", "A"),
		mkTrackRoot(root, "b.mp3", "B"),
	}
	tr := NewTree(root, tracks)

	artists := tr.Artists()
	if len(artists) != 1 {
		t.Fatalf("len(artists) = %d, want 1", len(artists))
	}
	if artists[0].Name != UnknownArtist {
		t.Errorf("artist = %q, want %q", artists[0].Name, UnknownArtist)
	}
	if len(artists[0].Albums) != 1 {
		t.Fatalf("len(albums) = %d, want 1", len(artists[0].Albums))
	}
	if artists[0].Albums[0].Name != UnknownAlbum {
		t.Errorf("album = %q, want %q", artists[0].Albums[0].Name, UnknownAlbum)
	}
	if artists[0].Albums[0].TrackCount != 2 {
		t.Errorf("track count = %d, want 2", artists[0].Albums[0].TrackCount)
	}
}

func TestTree_TracksSortedByTrackNumber(t *testing.T) {
	root := "/music"
	tracks := []Track{
		{Path: filepath.Join(root, "X", "Y", "03.mp3"), Title: "Three", Artist: "X", Album: "Y", Track: 3},
		{Path: filepath.Join(root, "X", "Y", "01.mp3"), Title: "One", Artist: "X", Album: "Y", Track: 1},
		{Path: filepath.Join(root, "X", "Y", "02.mp3"), Title: "Two", Artist: "X", Album: "Y", Track: 2},
	}
	tr := NewTree(root, tracks)

	got := tr.AlbumTracks("X", "Y")
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Title != "One" || got[1].Title != "Two" || got[2].Title != "Three" {
		t.Errorf("order = [%q, %q, %q], want [One, Two, Three]",
			got[0].Title, got[1].Title, got[2].Title)
	}
}

func TestTree_TracksSortedByPathWhenNoNumber(t *testing.T) {
	root := "/music"
	// Track numbers all 0 -> falls back to path sort.
	tracks := []Track{
		{Path: filepath.Join(root, "X", "Y", "b.mp3"), Title: "B", Artist: "X", Album: "Y"},
		{Path: filepath.Join(root, "X", "Y", "a.mp3"), Title: "A", Artist: "X", Album: "Y"},
		{Path: filepath.Join(root, "X", "Y", "c.mp3"), Title: "C", Artist: "X", Album: "Y"},
	}
	tr := NewTree(root, tracks)

	got := tr.AlbumTracks("X", "Y")
	if got[0].Title != "A" || got[1].Title != "B" || got[2].Title != "C" {
		t.Errorf("order = [%q, %q, %q], want [A, B, C]",
			got[0].Title, got[1].Title, got[2].Title)
	}
}

func TestTree_ArtistTracks(t *testing.T) {
	root := "/music"
	tracks := []Track{
		{Path: filepath.Join(root, "X", "Y", "01.mp3"), Title: "Y1", Artist: "X", Album: "Y", Track: 1},
		{Path: filepath.Join(root, "X", "Z", "01.mp3"), Title: "Z1", Artist: "X", Album: "Z", Track: 1},
		{Path: filepath.Join(root, "X", "Y", "02.mp3"), Title: "Y2", Artist: "X", Album: "Y", Track: 2},
		{Path: filepath.Join(root, "Other", "Q", "01.mp3"), Title: "Q1", Artist: "Other", Album: "Q", Track: 1},
	}
	tr := NewTree(root, tracks)

	got := tr.ArtistTracks("X")
	if len(got) != 3 {
		t.Fatalf("len(ArtistTracks(X)) = %d, want 3", len(got))
	}
	// Y1, Y2, Z1 in album order, with track number within album.
	wantOrder := []string{"Y1", "Y2", "Z1"}
	for i, w := range wantOrder {
		if got[i].Title != w {
			t.Errorf("ArtistTracks[%d] = %q, want %q", i, got[i].Title, w)
		}
	}

	if got := tr.ArtistTracks("Nonexistent"); got != nil {
		t.Errorf("ArtistTracks(unknown) = %+v, want nil", got)
	}
}

func TestTree_AlbumTracks_UnknownReturnsNil(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "X", "Y", "01.mp3", "Y1"),
	}
	tr := NewTree(root, tracks)

	if got := tr.AlbumTracks("X", "Nonexistent"); got != nil {
		t.Errorf("AlbumTracks unknown album = %+v, want nil", got)
	}
	if got := tr.AlbumTracks("Nonexistent", "Y"); got != nil {
		t.Errorf("AlbumTracks unknown artist = %+v, want nil", got)
	}
}

func TestTree_HasArtistAndAlbum(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "X", "Y", "01.mp3", "Y1"),
	}
	tr := NewTree(root, tracks)

	if !tr.HasArtist("X") {
		t.Error("HasArtist(X) = false, want true")
	}
	if tr.HasArtist("Y") {
		t.Error("HasArtist(Y) = true, want false")
	}
	if !tr.HasAlbum("X", "Y") {
		t.Error("HasAlbum(X, Y) = false, want true")
	}
	if tr.HasAlbum("X", "Z") {
		t.Error("HasAlbum(X, Z) = true, want false")
	}
	if tr.HasAlbum("Z", "Y") {
		t.Error("HasAlbum(Z, Y) = true, want false")
	}
}

func TestTree_Filter_HidesEmptyArtist(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "Helloween", "Keeper", "anthem.mp3", "Anthem"),
		mkTrack(root, "Helloween", "Keeper", "tale.mp3", "A Tale That Wasn't Right"),
		mkTrack(root, "Igorrr", "Amen", "limbo.mp3", "Limbo"),
	}
	tr := NewTree(root, tracks)
	tr.SetFilter("limbo")

	artists := tr.Artists()
	if len(artists) != 2 {
		t.Fatalf("len(artists) = %d, want 2", len(artists))
	}
	if !artists[0].Hidden {
		t.Errorf("Helloween should be Hidden under filter 'limbo'")
	}
	if artists[1].Hidden {
		t.Errorf("Igorrr should be visible under filter 'limbo'")
	}
}

func TestTree_Filter_HidesEmptyAlbum(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "Helloween", "Keeper", "anthem.mp3", "Anthem"),
		mkTrack(root, "Helloween", "Walls of Jericho", "ride-the-sky.mp3", "Ride the Sky"),
	}
	tr := NewTree(root, tracks)
	tr.SetFilter("anthem")

	artists := tr.Artists()
	if artists[0].Hidden {
		t.Errorf("Helloween should not be Hidden (Keeper has a match)")
	}
	// Albums are alphabetical: Keeper first, then Walls of Jericho.
	if len(artists[0].Albums) != 2 {
		t.Fatalf("len(albums) = %d, want 2", len(artists[0].Albums))
	}
	if artists[0].Albums[0].Name != "Keeper" {
		t.Errorf("albums[0] = %q, want Keeper", artists[0].Albums[0].Name)
	}
	if artists[0].Albums[0].Hidden {
		t.Errorf("Keeper should be visible (has 'anthem')")
	}
	if !artists[0].Albums[1].Hidden {
		t.Errorf("Walls of Jericho should be Hidden (no 'anthem' match)")
	}
}

func TestTree_Filter_EmptyPatternShowsAll(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "Helloween", "Keeper", "anthem.mp3", "Anthem"),
		mkTrack(root, "Igorrr", "Amen", "limbo.mp3", "Limbo"),
	}
	tr := NewTree(root, tracks)
	tr.SetFilter("limbo")
	tr.SetFilter("")

	if tr.HasFilter() {
		t.Error("HasFilter after clear = true, want false")
	}
	for _, a := range tr.Artists() {
		if a.Hidden {
			t.Errorf("artist %q hidden after filter clear", a.Name)
		}
		for _, al := range a.Albums {
			if al.Hidden {
				t.Errorf("album %q/%q hidden after filter clear", a.Name, al.Name)
			}
		}
	}
}

func TestTree_Filter_MatchesOnArtistTag(t *testing.T) {
	root := "/music"
	tracks := []Track{
		mkTrack(root, "Helloween", "Keeper", "anthem.mp3", "Anthem"),
		mkTrack(root, "Helloween", "Keeper", "tale.mp3", "A Tale That Wasn't Right"),
		mkTrack(root, "Igorrr", "Amen", "limbo.mp3", "Limbo"),
	}
	tr := NewTree(root, tracks)
	tr.SetFilter("HELLOWEEN") // case insensitive

	artists := tr.Artists()
	for _, a := range artists {
		if a.Name == "Helloween" && a.Hidden {
			t.Errorf("Helloween should be visible under 'HELLOWEEN'")
		}
		if a.Name == "Igorrr" && !a.Hidden {
			t.Errorf("Igorrr should be Hidden under 'HELLOWEEN'")
		}
	}
}

func TestTree_Filter_NoOpOnSamePattern(t *testing.T) {
	// The pattern must not reset the Hidden flags when it does not
	// change. SetFilter already early-returns, but we still want to
	// make sure calling it twice yields the same view.
	root := "/music"
	tracks := []Track{
		mkTrack(root, "X", "Y", "01.mp3", "Foo"),
	}
	tr := NewTree(root, tracks)
	tr.SetFilter("foo")
	tr.SetFilter("foo")
	if !tr.HasFilter() {
		t.Error("HasFilter after double-set = false, want true")
	}
}

func TestTree_RootEmptyBucketsAllAsUnknown(t *testing.T) {
	// Defensive: an empty root must not panic. Every track lands in
	// the unknown bucket.
	tr := NewTree("", []Track{
		{Path: "/some/abs/path.mp3", Title: "T"},
	})
	artists := tr.Artists()
	if len(artists) != 1 || artists[0].Name != UnknownArtist {
		t.Errorf("empty root should bucket everything as Unknown, got %+v", artists)
	}
}

func TestTree_RebuildIsIdempotent(t *testing.T) {
	// Same input -> same structure. Two passes over the same input
	// should be equal.
	root := "/music"
	tracks := []Track{
		mkTrack(root, "X", "Y", "01.mp3", "Foo"),
		mkTrack(root, "X", "Z", "01.mp3", "Bar"),
		mkTrack(root, "A", "B", "01.mp3", "Baz"),
	}
	tr1 := NewTree(root, tracks)
	tr2 := NewTree(root, tracks)

	names1 := artistNames(tr1.Artists())
	names2 := artistNames(tr2.Artists())
	if !equalStringSlices(names1, names2) {
		t.Errorf("rebuild not idempotent: %v vs %v", names1, names2)
	}
}

func TestTree_FromScanOutput(t *testing.T) {
	// End-to-end with a real on-disk layout: simulate a Scan() result
	// and verify the tree is consistent with it.
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "Iron Maiden", "Powerslave"))
	mkdir(t, filepath.Join(root, "Helloween", "Keeper of the Seven Keys"))
	mkdir(t, filepath.Join(root, "Helloween", "Walls of Jericho"))
	writeFile(t, filepath.Join(root, "Iron Maiden", "Powerslave", "01-aces-high.mp3"))
	writeFile(t, filepath.Join(root, "Iron Maiden", "Powerslave", "02-2-minutes.mp3"))
	writeFile(t, filepath.Join(root, "Helloween", "Keeper of the Seven Keys", "01-anthem.mp3"))
	writeFile(t, filepath.Join(root, "Helloween", "Walls of Jericho", "01-ride-the-sky.mp3"))

	tracks, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 4 {
		t.Fatalf("Scan returned %d tracks, want 4", len(tracks))
	}

	tr := NewTree(root, tracks)
	if tr.Len() != 2 {
		t.Errorf("tree artists = %d, want 2", tr.Len())
	}
	artists := tr.Artists()
	if artists[0].Name != "Helloween" {
		t.Errorf("artists[0] = %q, want Helloween", artists[0].Name)
	}
	if artists[0].Albums[0].Name != "Keeper of the Seven Keys" {
		t.Errorf("first album = %q, want Keeper of the Seven Keys", artists[0].Albums[0].Name)
	}
	if artists[0].Albums[1].Name != "Walls of Jericho" {
		t.Errorf("second album = %q, want Walls of Jericho", artists[0].Albums[1].Name)
	}
	if artists[1].Name != "Iron Maiden" {
		t.Errorf("artists[1] = %q, want Iron Maiden", artists[1].Name)
	}
}

func artistNames(artists []ArtistView) []string {
	out := make([]string, len(artists))
	for i, a := range artists {
		out[i] = a.Name
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
