// Package app centralises application state and orchestrates the
// library, the audio player and the user-facing status.
//
// The TUI in internal/tui depends on App; App in turn depends on the
// Player interface from internal/player. App never imports TUI types.
package app

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
)

// VolumeStep is the amount by which VolumeUp and VolumeDown adjust the
// player volume. It is exported so the TUI can display the same value
// when rendering volume controls.
const VolumeStep = 5

// RepeatMode describes how the queue behaves when the current track
// reaches its end.
type RepeatMode int

const (
	// RepeatOff stops the player at the end of the queue.
	RepeatOff RepeatMode = iota
	// RepeatAll wraps to the first track when the queue ends.
	RepeatAll
	// RepeatOne reloads the current track when it ends.
	RepeatOne
)

// String returns the canonical name of the mode ("off", "all", "one").
func (r RepeatMode) String() string {
	switch r {
	case RepeatAll:
		return "all"
	case RepeatOne:
		return "one"
	default:
		return "off"
	}
}

// App is the central application state. It coordinates the library queue,
// the audio player and the user-facing status.
//
// App is not safe for concurrent use. The Bubble Tea loop in internal/tui
// is the only caller and runs on a single goroutine.
type App struct {
	queue   *library.Queue
	tree    *library.Tree
	catalog []library.Track
	player  player.Player
	config  *config.Config

	currentTrack *library.Track
	selectedPath string
	filter       string
	queueOrder   []library.Track
	lastShuffle  []library.Track

	shuffle     bool
	repeat      RepeatMode
	savedVolume int
	mute        bool
	random      *rand.Rand

	// scope is the Library/Tracks navigation scope. An empty
	// scope means "the entire library". A non-empty Artist
	// narrows to that artist; with both Artist and Album set,
	// the scope is exactly that album.
	scope scope

	statusMessage string
	lastError     error
}

// scope is the navigation scope of the App. Both fields are
// empty strings when the scope is "all".
type scope struct {
	Artist string
	Album  string
}

type naturalEndReporter interface {
	ConsumeNaturalEnd() bool
}

// New constructs an App with the given config and audio player.
// The playback queue starts empty; LoadLibrary populates only the catalogue.
func New(cfg *config.Config, pl player.Player) *App {
	if cfg == nil {
		cfg = config.Default()
	}
	return &App{
		queue:       library.NewQueue(nil),
		player:      pl,
		config:      cfg,
		savedVolume: pl.Volume(),
		random:      rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0xC0FFEE)),
	}
}

// LoadLibrary scans root and replaces the catalogue. Starting or replacing a
// catalogue never creates a playback queue.
func (a *App) LoadLibrary(root string) error {
	tracks, err := library.Scan(root)
	if err != nil {
		a.lastError = fmt.Errorf("scan %s: %w", root, err)
		a.statusMessage = "Library scan failed"
		return err
	}
	a.catalog = append([]library.Track(nil), tracks...)
	a.queue.Replace(nil)
	a.queueOrder = nil
	a.lastShuffle = nil
	a.tree = library.NewTree(root, tracks)
	a.selectFirstVisible()
	a.statusMessage = fmt.Sprintf("Loaded %d tracks", len(tracks))
	return nil
}

// Rescan re-scans the configured music directory. The currently
// selected and currently playing tracks are preserved by their path:
// if the file is still present, selection/currentTrack are restored;
// if a track has been removed, the selection is moved to a safe
// neighbour and the player is stopped.
//
// The shuffle and repeat settings are preserved.
func (a *App) Rescan() error {
	selectedPath := a.selectedPath
	currentPath := ""
	if a.currentTrack != nil {
		currentPath = a.currentTrack.Path
	}

	tracks, err := library.Scan(a.config.MusicDir)
	if err != nil {
		a.lastError = fmt.Errorf("rescan %s: %w", a.config.MusicDir, err)
		a.statusMessage = "Rescan failed"
		return err
	}
	byPath := make(map[string]library.Track, len(tracks))
	for _, track := range tracks {
		byPath[track.Path] = track
	}
	a.catalog = append([]library.Track(nil), tracks...)
	a.tree = library.NewTree(a.config.MusicDir, tracks)
	a.queueOrder = survivingTracks(a.queueOrder, byPath)
	active := survivingTracks(a.queue.Tracks(), byPath)
	a.queue.Replace(active)
	if currentPath != "" {
		a.relocateQueue(currentPath)
	}

	if _, ok := byPath[selectedPath]; ok {
		a.selectedPath = selectedPath
	} else {
		a.selectFirstVisible()
	}
	// If the current track was removed, stop playback.
	if currentPath != "" {
		track, stillThere := byPath[currentPath]
		if !stillThere {
			_ = a.Stop()
		} else {
			a.currentTrack = trackCopy(track)
		}
	}
	a.statusMessage = fmt.Sprintf("Rescanned: %d tracks", len(tracks))
	return nil
}

// Queue exposes the underlying queue for read-only access.
func (a *App) Queue() *library.Queue { return a.queue }

// Tree exposes the library tree (Artist -> Album -> Track) for the
// most recent scan. The tree is rebuilt on every LoadLibrary and
// Rescan. It is nil until the first successful scan.
func (a *App) Tree() *library.Tree { return a.tree }

// Scope returns the current navigation scope (artist, album).
// Both fields are empty when the scope is "the whole library".
func (a *App) Scope() (artist, album string) {
	return a.scope.Artist, a.scope.Album
}

// SetScope changes only the Tracks view. It never mutates an active queue.
func (a *App) SetScope(artist, album string) {
	if artist == "" {
		a.scope = scope{}
		if a.ScopedIndex() < 0 {
			a.selectFirstVisible()
		}
		a.statusMessage = "Scope: all tracks"
		return
	}
	a.scope = scope{Artist: artist, Album: album}
	if len(a.ScopedTracks()) == 0 {
		a.statusMessage = fmt.Sprintf("Scope: %s/%s (empty)", artist, album)
		return
	}
	if a.ScopedIndex() < 0 {
		a.selectFirstVisible()
	}
	if album != "" {
		a.statusMessage = fmt.Sprintf("Scope: %s/%s", artist, album)
	} else {
		a.statusMessage = fmt.Sprintf("Scope: %s", artist)
	}
}

// ScopedTracks returns the tracks in the current scope, filtered
// by the active search pattern. The slice is a defensive copy and
// may be empty.
func (a *App) ScopedTracks() []library.Track {
	scoped := a.scopedTracksNoFilter()
	pattern := a.filter
	if pattern == "" {
		return scoped
	}
	out := make([]library.Track, 0, len(scoped))
	for _, t := range scoped {
		if a.trackMatches(t, pattern) {
			out = append(out, t)
		}
	}
	return out
}

// ScopedIndex returns the independent Tracks selection within ScopedTracks.
func (a *App) ScopedIndex() int {
	scoped := a.ScopedTracks()
	for i, t := range scoped {
		if t.Path == a.selectedPath {
			return i
		}
	}
	return -1
}

// SelectNextScoped moves the selection to the next track in the
// current scope. It returns true when the selection changed.
func (a *App) SelectNextScoped() bool {
	scoped := a.ScopedTracks()
	if len(scoped) == 0 {
		return false
	}
	cur := a.ScopedIndex()
	if cur < 0 {
		// Jump to the first track of the scope.
		a.selectedPath = scoped[0].Path
		return true
	}
	if cur >= len(scoped)-1 {
		return false
	}
	a.selectedPath = scoped[cur+1].Path
	return true
}

// SelectPrevScoped moves the selection to the previous track in
// the current scope. It returns true when the selection changed.
func (a *App) SelectPrevScoped() bool {
	scoped := a.ScopedTracks()
	if len(scoped) == 0 {
		return false
	}
	cur := a.ScopedIndex()
	if cur <= 0 {
		return false
	}
	a.selectedPath = scoped[cur-1].Path
	return true
}

// scopedTracksNoFilter returns the tracks in the current scope
// without applying the search filter. Used internally for scope
// management (e.g. "is the current selection still in scope?") and
// as the basis of ScopedTracks.
func (a *App) scopedTracksNoFilter() []library.Track {
	if a.tree == nil {
		return nil
	}
	if a.scope.Artist == "" {
		return a.tree.Tracks()
	}
	if a.scope.Album == "" {
		return a.tree.ArtistTracks(a.scope.Artist)
	}
	return a.tree.AlbumTracks(a.scope.Artist, a.scope.Album)
}

// trackMatches reports whether the track matches the search
// pattern (case-insensitive substring on Title, Artist, Album).
func (a *App) trackMatches(t library.Track, pattern string) bool {
	if pattern == "" {
		return true
	}
	p := strings.ToLower(pattern)
	return strings.Contains(strings.ToLower(t.Title), p) ||
		strings.Contains(strings.ToLower(t.Artist), p) ||
		strings.Contains(strings.ToLower(t.Album), p)
}

func (a *App) selectFirstVisible() {
	tracks := a.ScopedTracks()
	if len(tracks) == 0 {
		a.selectedPath = ""
		return
	}
	a.selectedPath = tracks[0].Path
}

// --- Phase 15: queue manipulation ---

// MoveQueueUp moves the track at position i one step toward the
// head of the queue. No-op when i is 0, out of range, or when
// the queue has fewer than 2 tracks.
func (a *App) MoveQueueUp(i int) bool {
	ok := a.queue.MoveUp(i)
	if ok {
		a.queueOrder = a.queue.Tracks()
		a.statusMessage = "Moved up"
	}
	return ok
}

// MoveQueueDown moves the track at position i one step toward the
// tail of the queue. No-op when i is the last index, out of
// range, or when the queue has fewer than 2 tracks.
func (a *App) MoveQueueDown(i int) bool {
	ok := a.queue.MoveDown(i)
	if ok {
		a.queueOrder = a.queue.Tracks()
		a.statusMessage = "Moved down"
	}
	return ok
}

// RemoveFromQueue removes the track at position i. No-op when
// the position is the currently-playing track or out of range.
func (a *App) RemoveFromQueue(i int) bool {
	tracks := a.queue.Tracks()
	if i < 0 || i >= len(tracks) {
		return false
	}
	if a.currentTrack != nil && tracks[i].Path == a.currentTrack.Path {
		return false
	}
	ok := a.queue.Remove(i)
	if ok {
		a.queueOrder = a.queue.Tracks()
		a.statusMessage = "Removed from queue"
	}
	return ok
}

// ClearQueue keeps the currently loaded track, or empties the queue when no
// track is loaded.
func (a *App) ClearQueue() {
	if a.currentTrack == nil {
		a.queue.Clear()
		a.queueOrder = nil
	} else {
		a.queue.Replace([]library.Track{*a.currentTrack})
		a.queueOrder = a.queue.Tracks()
	}
	a.statusMessage = "Queue cleared"
}

// PlayAtIndex loads and starts playing the track at the given
// full-list position. The selection moves to that position.
func (a *App) PlayAtIndex(i int) error {
	return a.playAt(i)
}

// VisibleTracks is the filtered catalogue view before scope is applied.
func (a *App) VisibleTracks() []library.Track {
	out := make([]library.Track, 0, len(a.catalog))
	for _, track := range a.catalog {
		if a.trackMatches(track, a.filter) {
			out = append(out, track)
		}
	}
	return out
}

// VisibleLen returns the number of tracks visible under the current
// filter.
func (a *App) VisibleLen() int { return len(a.ScopedTracks()) }

// DisplayIndex returns the selected track's position in the visible
// (filtered) view, or -1 if the selection is hidden by the filter.
func (a *App) DisplayIndex() int { return a.ScopedIndex() }

// Filter returns the active filter pattern. An empty string means
// no filter.
func (a *App) Filter() string { return a.filter }

// HasFilter reports whether a filter pattern is currently active.
func (a *App) HasFilter() bool { return a.filter != "" }

// SetFilter sets the active filter pattern. Pass an empty string to
// clear the filter. When a filter is set, navigation (Next, Previous,
// SelectNext, SelectPrev) and the playback queue all operate on the
// visible subset.
func (a *App) SetFilter(pattern string) {
	a.filter = pattern
	if a.tree != nil {
		a.tree.SetFilter(pattern)
	}
	if a.ScopedIndex() < 0 {
		a.selectFirstVisible()
	}
	if pattern == "" {
		a.statusMessage = "Filter cleared"
	} else {
		a.statusMessage = fmt.Sprintf("Filter: %s (%d/%d)",
			pattern, a.VisibleLen(), len(a.scopedTracksNoFilter()))
	}
}

// Selected returns the currently selected track, or nil if the queue
// is empty.
func (a *App) Selected() *library.Track {
	for _, track := range a.catalog {
		if track.Path == a.selectedPath {
			return trackCopy(track)
		}
	}
	return nil
}

// SelectCurrentInScope aligns the Tracks selection with the currently loaded
// track when that track belongs to the visible scope and filter. It leaves the
// user's navigation context untouched when the current track is excluded.
func (a *App) SelectCurrentInScope() bool {
	if a.currentTrack == nil {
		return false
	}
	for _, track := range a.ScopedTracks() {
		if track.Path == a.currentTrack.Path {
			a.selectedPath = track.Path
			return true
		}
	}
	return false
}

// SelectedIndex returns the selection in the full catalogue.
func (a *App) SelectedIndex() int {
	for i := range a.catalog {
		if a.catalog[i].Path == a.selectedPath {
			return i
		}
	}
	return 0
}

// TotalTracks returns the catalogue size.
func (a *App) TotalTracks() int { return len(a.catalog) }

// MissingDurationPaths returns a copy of the catalogue paths whose duration
// was not available during the library scan. Catalogue order is preserved so
// background enrichment is deterministic and visible rows fill progressively.
func (a *App) MissingDurationPaths() []string {
	paths := make([]string, 0)
	for _, track := range a.catalog {
		if track.Duration <= 0 {
			paths = append(paths, track.Path)
		}
	}
	return paths
}

// ApplyTrackDuration updates every in-memory representation of path. It must
// be called from the application's owning goroutine. Invalid durations and
// paths outside the current catalogue are rejected.
func (a *App) ApplyTrackDuration(path string, duration time.Duration) bool {
	if path == "" || duration <= 0 {
		return false
	}
	for i := range a.catalog {
		if a.catalog[i].Path != path {
			continue
		}
		updated := a.catalog[i]
		updated.Duration = duration
		a.updateTrack(updated)
		return true
	}
	return false
}

// ScopedTotal returns the number of tracks in the active scope before search.
func (a *App) ScopedTotal() int { return len(a.scopedTracksNoFilter()) }

// Current returns the currently playing track, or nil if none.
func (a *App) Current() *library.Track { return a.currentTrack }

// State returns the player state.
func (a *App) State() player.State { return a.player.State() }

// Volume returns the current effective volume in [0, 100]. When the
// app is muted this is 0; SavedVolume reports the last non-muted
// value.
func (a *App) Volume() int {
	if a.mute {
		return 0
	}
	return a.player.Volume()
}

// SavedVolume returns the volume that will be restored when mute is
// turned off. It is independent of the mute state.
func (a *App) SavedVolume() int { return a.savedVolume }

// Muted reports whether the app is currently muting output.
func (a *App) Muted() bool { return a.mute }

// Shuffle reports whether shuffle mode is enabled.
func (a *App) Shuffle() bool { return a.shuffle }

// RepeatMode reports the active repeat mode.
func (a *App) Repeat() RepeatMode { return a.repeat }

// Position returns the current playback position.
func (a *App) Position() time.Duration { return a.player.Position() }

// Duration returns the total length of the current track. A zero return value
// means the backend does not know it yet (see HasDuration). MP3 duration is
// learned from the audio backend on load rather than during the library scan.
func (a *App) Duration() time.Duration { return a.player.Duration() }

// HasDuration reports whether the player knows the duration of the
// current track. It can briefly be false while libmpv is still making the
// property available.
func (a *App) HasDuration() bool { return a.player.Duration() > 0 }

// Status returns the last user-facing status message.
func (a *App) Status() string { return a.statusMessage }

// Error returns the last error encountered by an action.
func (a *App) Error() error { return a.lastError }

// Config returns the active configuration.
func (a *App) Config() *config.Config { return a.config }

// Snapshot returns the current playback state for persistence. The
// returned value is what the app would restore on the next launch
// via ApplySnapshot.
func (a *App) Snapshot() (volume int, currentPath string) {
	volume = a.savedVolume
	if a.currentTrack != nil {
		currentPath = a.currentTrack.Path
	}
	return volume, currentPath
}

// ApplySnapshot restores the volume and the "last played" reference
// from a previously saved State. The current track is not loaded: the
// TUI uses Snapshot instead when the user wants to play the last
// track.
//
// A volume value outside [0, 100] is ignored (the underlying player
// clamps it anyway).
func (a *App) ApplySnapshot(volume int, currentPath string) {
	if volume < 0 || volume > 100 {
		volume = 100
	}
	a.savedVolume = volume
	_ = a.player.SetVolume(volume)
	a.statusMessage = fmt.Sprintf("Volume: %d%%", volume)
}

// SelectNext moves the selection down by one row. It is a no-op when the
// queue is empty or the selection is already at the last track.
func (a *App) SelectNext() {
	a.SelectNextScoped()
}

// SelectPrev moves the selection up by one row. It is a no-op when the
// queue is empty or the selection is already at the first track.
func (a *App) SelectPrev() {
	a.SelectPrevScoped()
}

// PlaySelected loads the selected track and starts playback.
// If the selected track is already the current track, this toggles
// play/pause instead of reloading.
func (a *App) PlaySelected() error {
	sel := a.Selected()
	if sel == nil {
		return a.fail(errors.New("nothing visible to play"), "No visible tracks to play")
	}
	reference := a.ScopedTracks()
	if len(reference) == 0 {
		return a.fail(errors.New("nothing visible to play"), "No visible tracks to play")
	}
	a.queueOrder = append([]library.Track(nil), reference...)
	active := append([]library.Track(nil), reference...)
	selectedIndex := indexByPath(active, sel.Path)
	if a.shuffle {
		active = a.nextShuffledOrder(reference, sel.Path, selectedIndex)
	}
	a.queue.Replace(active)
	selectedIndex = indexByPath(active, sel.Path)
	return a.playAt(selectedIndex)
}

// TogglePlay toggles between play and pause. When the player is stopped
// it plays the currently selected track.
func (a *App) TogglePlay() error {
	switch a.player.State() {
	case player.StatePlaying:
		return a.pause()
	case player.StatePaused:
		return a.resume()
	default:
		return a.PlaySelected()
	}
}

// Stop stops playback and clears the current track reference.
func (a *App) Stop() error {
	if err := a.player.Stop(); err != nil {
		a.lastError = err
		return err
	}
	a.currentTrack = nil
	a.statusMessage = "Stopped"
	return nil
}

// Seek moves playback to the given absolute position. See
// player.Player.Seek for semantics.
func (a *App) Seek(position time.Duration) error {
	if err := a.player.Seek(position); err != nil {
		a.lastError = err
		return err
	}
	return nil
}

// MaybeAdvance traverses only the active playback queue.
//
// It is a no-op when playback is not in the stopped state or when there is no
// current track. Backends that expose ConsumeNaturalEnd use it to distinguish
// EOF from decode errors and other stopped states.
func (a *App) MaybeAdvance() error {
	if a.player.State() != player.StateStopped {
		return nil
	}
	if a.currentTrack == nil {
		return nil
	}
	if reporter, ok := a.player.(naturalEndReporter); ok && !reporter.ConsumeNaturalEnd() {
		return nil
	}
	if a.queue.Len() == 0 {
		a.currentTrack = nil
		return nil
	}
	if a.repeat == RepeatOne {
		return a.playAt(a.queue.Index())
	}
	if a.queue.Next() {
		return a.playAt(a.queue.Index())
	}
	if a.repeat == RepeatAll {
		if a.queue.FirstVisible() {
			return a.playAt(a.queue.Index())
		}
	}
	a.currentTrack = nil
	a.statusMessage = "End of queue"
	return nil
}

// Next advances in the already materialised active queue.
func (a *App) Next() error {
	if a.queue.Len() == 0 {
		return errors.New("queue is empty")
	}
	if a.queue.Next() {
		return a.playAt(a.queue.Index())
	}
	if a.repeat == RepeatAll {
		if a.queue.FirstVisible() {
			return a.playAt(a.queue.Index())
		}
	}
	return a.Stop()
}

// Previous moves to the previous item in the active queue.
func (a *App) Previous() error {
	if a.queue.Len() == 0 {
		return errors.New("queue is empty")
	}
	if !a.queue.Previous() {
		return nil
	}
	return a.playAt(a.queue.Index())
}

// ToggleShuffle recalculates the active order without touching the player.
func (a *App) ToggleShuffle() {
	a.shuffle = !a.shuffle
	if a.queue.Len() > 0 {
		path := ""
		index := a.queue.Index()
		if current := a.queue.Current(); current != nil {
			path = current.Path
		}
		if a.shuffle {
			a.queue.Replace(a.nextShuffledOrder(a.queueOrder, path, index))
		} else {
			a.queue.Replace(a.queueOrder)
		}
		a.relocateQueue(path)
	}
	if a.shuffle {
		a.statusMessage = "Shuffle on"
	} else {
		a.statusMessage = "Shuffle off"
	}
}

// CycleRepeat rotates off -> all -> one -> off.
func (a *App) CycleRepeat() {
	switch a.repeat {
	case RepeatOff:
		a.repeat = RepeatAll
		a.statusMessage = "Repeat: all"
	case RepeatAll:
		a.repeat = RepeatOne
		a.statusMessage = "Repeat: one"
	default:
		a.repeat = RepeatOff
		a.statusMessage = "Repeat: off"
	}
}

// ToggleMute flips the mute state. The underlying audio player is held
// at 0 while mute is on, but the saved volume is preserved so that
// unmuting restores the previous value.
func (a *App) ToggleMute() {
	if !a.mute {
		a.savedVolume = a.player.Volume()
		_ = a.player.SetVolume(0)
		a.mute = true
		a.statusMessage = "Muted"
		return
	}
	_ = a.player.SetVolume(a.savedVolume)
	a.mute = false
	a.statusMessage = fmt.Sprintf("Unmuted (%d%%)", a.savedVolume)
}

// VolumeUp raises the volume by VolumeStep (clamped to 100). While
// muted, the saved value moves but the audio player stays at 0.
func (a *App) VolumeUp() error {
	target := a.savedVolume + VolumeStep
	if target > 100 {
		target = 100
	}
	a.savedVolume = target
	if a.mute {
		a.statusMessage = fmt.Sprintf("Muted (volume: %d%%)", target)
		return nil
	}
	return a.applyVolume(target)
}

// VolumeDown lowers the volume by VolumeStep (clamped to 0). While
// muted, the saved value moves but the audio player stays at 0.
func (a *App) VolumeDown() error {
	target := a.savedVolume - VolumeStep
	if target < 0 {
		target = 0
	}
	a.savedVolume = target
	if a.mute {
		return nil
	}
	return a.applyVolume(target)
}

func (a *App) applyVolume(v int) error {
	if err := a.player.SetVolume(v); err != nil {
		a.lastError = err
		return err
	}
	a.statusMessage = fmt.Sprintf("Volume: %d%%", a.player.Volume())
	return nil
}

func (a *App) fail(err error, status string) error {
	a.lastError = err
	a.statusMessage = status
	return err
}

// playAt loads and plays the track at idx. On success the selection is
// moved to idx and currentTrack is updated. On failure lastError and
// statusMessage reflect the failure, currentTrack is cleared so that
// auto-advance does not loop on a broken track, and the selection is
// preserved.
func (a *App) playAt(idx int) error {
	if idx < 0 || idx >= a.queue.Len() {
		return fmt.Errorf("app: track index %d out of range", idx)
	}
	tracks := a.queue.Tracks()
	t := &tracks[idx]

	if err := a.player.Load(t.Path); err != nil {
		a.lastError = err
		a.statusMessage = "Failed to load track"
		a.currentTrack = nil
		return err
	}
	if duration := a.player.Duration(); duration > 0 {
		t.Duration = duration
		a.updateTrack(*t)
	}
	if err := a.player.Play(); err != nil {
		a.lastError = err
		a.statusMessage = "Failed to start playback"
		a.currentTrack = nil
		return err
	}

	a.currentTrack = t
	a.queue.SetIndex(idx)
	a.statusMessage = fmt.Sprintf("Playing: %s", t.Title)
	return nil
}

func (a *App) updateTrack(updated library.Track) {
	updateTrackInSlice(a.catalog, updated)
	updateTrackInSlice(a.queueOrder, updated)
	updateTrackInSlice(a.lastShuffle, updated)
	active := a.queue.Tracks()
	if updateTrackInSlice(active, updated) {
		index := a.queue.Index()
		a.queue.Replace(active)
		a.queue.SetIndex(index)
	}
	if a.tree != nil {
		a.tree.UpdateTrack(updated)
	}
	if a.currentTrack != nil && a.currentTrack.Path == updated.Path {
		a.currentTrack = trackCopy(updated)
	}
}

func updateTrackInSlice(tracks []library.Track, updated library.Track) bool {
	for i := range tracks {
		if tracks[i].Path == updated.Path {
			tracks[i] = updated
			return true
		}
	}
	return false
}

func (a *App) shuffledOrder(reference []library.Track, currentPath string, currentIndex int) []library.Track {
	if len(reference) < 2 {
		return append([]library.Track(nil), reference...)
	}
	currentReferenceIndex := indexByPath(reference, currentPath)
	if currentReferenceIndex < 0 {
		out := append([]library.Track(nil), reference...)
		a.shuffleTracks(out)
		return out
	}
	if currentIndex < 0 || currentIndex >= len(reference) {
		currentIndex = currentReferenceIndex
	}
	others := make([]library.Track, 0, len(reference)-1)
	for i := range reference {
		if i != currentReferenceIndex {
			others = append(others, reference[i])
		}
	}
	a.shuffleTracks(others)
	out := make([]library.Track, len(reference))
	out[currentIndex] = reference[currentReferenceIndex]
	for i, j := 0, 0; i < len(out); i++ {
		if i == currentIndex {
			continue
		}
		out[i] = others[j]
		j++
	}
	return out
}

func (a *App) nextShuffledOrder(reference []library.Track, currentPath string, currentIndex int) []library.Track {
	for range 8 {
		candidate := a.shuffledOrder(reference, currentPath, currentIndex)
		if !sameTrackOrder(candidate, a.lastShuffle) {
			a.lastShuffle = append([]library.Track(nil), candidate...)
			return candidate
		}
	}
	// With at least two movable tracks, swapping them guarantees a different
	// permutation even if repeated RNG draws returned the previous one.
	candidate := a.shuffledOrder(reference, currentPath, currentIndex)
	movable := make([]int, 0, len(candidate))
	for i := range candidate {
		if candidate[i].Path != currentPath {
			movable = append(movable, i)
		}
	}
	if len(movable) >= 2 && sameTrackOrder(candidate, a.lastShuffle) {
		i, j := movable[0], movable[1]
		candidate[i], candidate[j] = candidate[j], candidate[i]
	}
	a.lastShuffle = append([]library.Track(nil), candidate...)
	return candidate
}

func (a *App) shuffleTracks(tracks []library.Track) {
	for i := len(tracks) - 1; i > 0; i-- {
		j := a.random.IntN(i + 1)
		tracks[i], tracks[j] = tracks[j], tracks[i]
	}
}

func (a *App) relocateQueue(path string) {
	if index := indexByPath(a.queue.Tracks(), path); index >= 0 {
		a.queue.SetIndex(index)
	}
}

func indexByPath(tracks []library.Track, path string) int {
	for i := range tracks {
		if tracks[i].Path == path {
			return i
		}
	}
	return -1
}

func sameTrackOrder(left, right []library.Track) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Path != right[i].Path {
			return false
		}
	}
	return true
}

func survivingTracks(order []library.Track, catalogue map[string]library.Track) []library.Track {
	out := make([]library.Track, 0, len(order))
	for _, old := range order {
		if updated, ok := catalogue[old.Path]; ok {
			out = append(out, updated)
		}
	}
	return out
}

func trackCopy(track library.Track) *library.Track {
	copy := track
	return &copy
}

func (a *App) pause() error {
	if err := a.player.Pause(); err != nil {
		a.lastError = err
		return err
	}
	a.statusMessage = "Paused"
	return nil
}

func (a *App) resume() error {
	if err := a.player.Play(); err != nil {
		a.lastError = err
		return err
	}
	a.statusMessage = "Resumed"
	return nil
}
