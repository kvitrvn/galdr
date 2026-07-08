package library

import (
	"path/filepath"
	"sort"
	"strings"
)

// Bucket names used when a track cannot be classified by path.
//
// Tracks that sit at the root of the music directory (no artist or
// album folder above them) are bucketed under UnknownArtist /
// UnknownAlbum. The two constants are exported so the TUI can
// compare against them.
const (
	UnknownArtist = "(Unknown Artist)"
	UnknownAlbum  = "(Unknown Album)"
)

// Tree is a navigable view of a track slice grouped by path:
//
//	Artist -> Album -> Track
//
// The Tree is a pure data layer. It does not depend on the TUI or
// the audio backend. It is rebuilt every time the library is scanned
// or rescanned, and it is read-only afterwards.
//
// Path classification rules (relative to the configured root):
//
//   - 3+ segments: parts[0] is the artist, parts[1] is the album.
//     parts[2:] (e.g. a CD1/ subfolder) is ignored: the track is
//     attributed to the parent album.
//   - 2 segments: UnknownArtist / parts[0].
//   - 1 segment (file at the root): UnknownArtist / UnknownAlbum.
//   - Anything that resolves outside the root (defensive): the same
//     "unknown" bucket.
//
// Artists and albums are sorted alphabetically. The two unknown
// buckets sort last.
type Tree struct {
	root    string
	tracks  []Track
	artists []artistNode
	pattern string
}

// artistNode is the internal representation of an artist. The TUI
// only ever sees ArtistView values.
type artistNode struct {
	name   string
	albums []albumNode
}

// albumNode is the internal representation of an album. trackIdxs
// are indices into the underlying tracks slice; the order is the
// canonical order of the album (track number, then path).
type albumNode struct {
	name      string
	trackIdxs []int
}

// ArtistView is a read-only view of an artist, post-filter.
type ArtistView struct {
	Name   string
	Albums []AlbumView
	Hidden bool
}

// AlbumView is a read-only view of an album, post-filter.
type AlbumView struct {
	Name       string
	TrackCount int
	Hidden     bool
}

// NewTree builds a Tree from the given tracks. The root argument is
// the same value passed to Scan: the music directory. It is used
// to compute each track's relative path.
//
// A nil tracks slice produces a valid empty Tree. An empty root
// string makes the classifier bucket every track under the unknown
// buckets (defensive default).
func NewTree(root string, tracks []Track) *Tree {
	t := &Tree{
		root:   root,
		tracks: tracks,
	}
	t.rebuild()
	return t
}

// Tracks returns the underlying track slice. The Tree does not own
// it; callers must not mutate it after passing it to NewTree.
func (t *Tree) Tracks() []Track {
	if t == nil {
		return nil
	}
	return t.tracks
}

// Len returns the number of artists in the tree, regardless of the
// active filter.
func (t *Tree) Len() int {
	if t == nil {
		return 0
	}
	return len(t.artists)
}

// TotalTracks returns the total number of tracks across all artists.
func (t *Tree) TotalTracks() int {
	if t == nil {
		return 0
	}
	return len(t.tracks)
}

// Filter returns the active filter pattern. An empty string means
// no filter.
func (t *Tree) Filter() string {
	if t == nil {
		return ""
	}
	return t.pattern
}

// HasFilter reports whether a filter pattern is currently active.
func (t *Tree) HasFilter() bool {
	return t.Filter() != ""
}

// SetFilter sets the filter pattern. The pattern is matched
// case-insensitively against Title, Artist and Album. An empty
// pattern clears the filter.
//
// The filter affects branch visibility only: artists and albums
// with no matching tracks are marked Hidden. The leaf contents
// (the list of tracks within a visible album) are not cropped: the
// TUI decides whether to also crop the right-hand panel in Phase
// 14.
func (t *Tree) SetFilter(pattern string) {
	if t == nil {
		return
	}
	if pattern == t.pattern {
		return
	}
	t.pattern = pattern
}

// Artists returns a defensive slice of ArtistView values. An artist
// is Hidden when the active filter is non-empty and none of its
// tracks match the pattern.
//
// The returned slice is a copy: callers can mutate it freely.
func (t *Tree) Artists() []ArtistView {
	if t == nil {
		return nil
	}
	out := make([]ArtistView, len(t.artists))
	for i, a := range t.artists {
		albums := make([]AlbumView, len(a.albums))
		anyAlbumMatch := false
		for j, al := range a.albums {
			albums[j] = AlbumView{
				Name:       al.name,
				TrackCount: len(al.trackIdxs),
				Hidden:     t.albumHidden(al),
			}
			if !albums[j].Hidden {
				anyAlbumMatch = true
			}
		}
		out[i] = ArtistView{
			Name:   a.name,
			Albums: albums,
			Hidden: t.artistHidden(a) || !anyAlbumMatch && t.HasFilter(),
		}
	}
	return out
}

// ArtistTracks returns the tracks of the given artist, across all of
// its albums, in canonical order. The returned slice is a defensive
// copy. An unknown artist name returns nil.
func (t *Tree) ArtistTracks(artist string) []Track {
	if t == nil {
		return nil
	}
	idx := t.findArtist(artist)
	if idx < 0 {
		return nil
	}
	a := t.artists[idx]
	out := make([]Track, 0, len(a.albums[0].trackIdxs)*len(a.albums))
	for _, al := range a.albums {
		for _, i := range al.trackIdxs {
			out = append(out, t.tracks[i])
		}
	}
	return out
}

// AlbumTracks returns the tracks of the given album of the given
// artist, in canonical order. The returned slice is a defensive
// copy. An unknown artist or album returns nil.
func (t *Tree) AlbumTracks(artist, album string) []Track {
	if t == nil {
		return nil
	}
	ai := t.findArtist(artist)
	if ai < 0 {
		return nil
	}
	ali := t.findAlbum(t.artists[ai], album)
	if ali < 0 {
		return nil
	}
	trackIdxs := t.artists[ai].albums[ali].trackIdxs
	out := make([]Track, len(trackIdxs))
	for k, i := range trackIdxs {
		out[k] = t.tracks[i]
	}
	return out
}

// HasArtist reports whether an artist with the given name exists.
func (t *Tree) HasArtist(name string) bool {
	if t == nil {
		return false
	}
	return t.findArtist(name) >= 0
}

// HasAlbum reports whether the given artist has the given album.
func (t *Tree) HasAlbum(artist, album string) bool {
	if t == nil {
		return false
	}
	ai := t.findArtist(artist)
	if ai < 0 {
		return false
	}
	return t.findAlbum(t.artists[ai], album) >= 0
}

// rebuild recomputes the artist/album index from the underlying
// tracks. It is called once at construction. The Tree is otherwise
// immutable.
func (t *Tree) rebuild() {
	byArtist := make(map[string]*artistNode)
	// Stable order: collect artists in the order they appear in the
	// track slice (which is sorted by path inside Scan), so the
	// initial ordering is predictable for tests.
	for i, tr := range t.tracks {
		artist, album := t.classify(tr)
		a, ok := byArtist[artist]
		if !ok {
			a = &artistNode{name: artist}
			byArtist[artist] = a
		}
		var al *albumNode
		for k := range a.albums {
			if a.albums[k].name == album {
				al = &a.albums[k]
				break
			}
		}
		if al == nil {
			a.albums = append(a.albums, albumNode{name: album})
			al = &a.albums[len(a.albums)-1]
		}
		al.trackIdxs = append(al.trackIdxs, i)
	}

	t.artists = t.artists[:0]
	// Stable, path-based ordering of artists: iterate the track
	// slice and emit each new artist in first-seen order. This
	// keeps the layout deterministic and matches the order of
	// Scan()'s output, which is itself sorted by path.
	seen := make(map[string]bool)
	for _, tr := range t.tracks {
		artist, _ := t.classify(tr)
		if seen[artist] {
			continue
		}
		seen[artist] = true
		if a, ok := byArtist[artist]; ok {
			t.sortAlbums(a)
			t.artists = append(t.artists, *a)
		}
	}
	t.sortArtists()
}

// sortArtists sorts the artists slice alphabetically, with the
// unknown bucket pinned to the end.
func (t *Tree) sortArtists() {
	sort.SliceStable(t.artists, func(i, j int) bool {
		ai, aj := t.artists[i].name, t.artists[j].name
		if ai == UnknownArtist {
			return false
		}
		if aj == UnknownArtist {
			return true
		}
		return ai < aj
	})
}

// sortAlbums sorts the albums of an artist alphabetically, with
// the unknown album bucket pinned to the end. Within an album,
// the tracks are sorted by metadata track number then by path.
func (t *Tree) sortAlbums(a *artistNode) {
	sort.SliceStable(a.albums, func(i, j int) bool {
		ai, aj := a.albums[i].name, a.albums[j].name
		if ai == UnknownAlbum {
			return false
		}
		if aj == UnknownAlbum {
			return true
		}
		return ai < aj
	})
	for k := range a.albums {
		al := &a.albums[k]
		sort.SliceStable(al.trackIdxs, func(i, j int) bool {
			ti, tj := t.tracks[al.trackIdxs[i]], t.tracks[al.trackIdxs[j]]
			if ti.Track != tj.Track {
				if ti.Track == 0 {
					return false
				}
				if tj.Track == 0 {
					return true
				}
				return ti.Track < tj.Track
			}
			return ti.Path < tj.Path
		})
	}
}

// classify returns the (artist, album) derived from the track's
// path, relative to the tree root.
func (t *Tree) classify(tr Track) (artist, album string) {
	if t.root == "" {
		return UnknownArtist, UnknownAlbum
	}
	rel, err := filepath.Rel(t.root, tr.Path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return UnknownArtist, UnknownAlbum
	}
	dir := filepath.Dir(rel)
	if dir == "." || dir == "" {
		return UnknownArtist, UnknownAlbum
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")
	switch len(parts) {
	case 1:
		return UnknownArtist, parts[0]
	default:
		return parts[0], parts[1]
	}
}

// findArtist returns the index of the named artist in t.artists, or
// -1 if not found.
func (t *Tree) findArtist(name string) int {
	for i, a := range t.artists {
		if a.name == name {
			return i
		}
	}
	return -1
}

// findAlbum returns the index of the named album in a.albums, or
// -1 if not found.
func (t *Tree) findAlbum(a artistNode, name string) int {
	for i, al := range a.albums {
		if al.name == name {
			return i
		}
	}
	return -1
}

// artistHidden reports whether the given artist has zero tracks
// matching the active filter.
func (t *Tree) artistHidden(a artistNode) bool {
	if !t.HasFilter() {
		return false
	}
	for _, al := range a.albums {
		if !t.albumHidden(al) {
			return false
		}
	}
	return true
}

// albumHidden reports whether the given album has zero tracks
// matching the active filter.
func (t *Tree) albumHidden(al albumNode) bool {
	if !t.HasFilter() {
		return false
	}
	p := strings.ToLower(t.pattern)
	for _, i := range al.trackIdxs {
		tr := t.tracks[i]
		if strings.Contains(strings.ToLower(tr.Title), p) ||
			strings.Contains(strings.ToLower(tr.Artist), p) ||
			strings.Contains(strings.ToLower(tr.Album), p) {
			return false
		}
	}
	return true
}
