// Package tui contains the Bubble Tea models, views and keybindings.
//
// The TUI depends on the player interface from internal/player (through
// internal/app) and never on a concrete audio backend.
package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/i18n"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

// keyMap defines every keybinding exposed by the TUI.
type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Next       key.Binding
	Prev       key.Binding
	Play       key.Binding
	Pause      key.Binding
	Stop       key.Binding
	VolUp      key.Binding
	VolDown    key.Binding
	Rescan     key.Binding
	Shuffle    key.Binding
	Repeat     key.Binding
	Mute       key.Binding
	SeekFwd    key.Binding
	SeekBwd    key.Binding
	SeekHome   key.Binding
	SeekEnd    key.Binding
	Search     key.Binding
	Clear      key.Binding
	Help       key.Binding
	Quit       key.Binding
	QueueUp    key.Binding
	QueueDown  key.Binding
	QueueDel   key.Binding
	QueueClear key.Binding
	QueueAdd   key.Binding
	PlayNext   key.Binding
	Playlists  key.Binding
}

// SeekStep is the time delta applied by left/right seek.
const SeekStep = 5 * time.Second

// defaultKeys returns the v2 keybindings. The interpretation of
// the navigation keys (up/down, left/right, enter) depends on the
// focused panel:
//
//	Panel    | up/k          | down/j       | left/h    | right/l
//	---------+---------------+--------------+-----------+----------
//	Library  | prev row      | next row     | collapse  | expand
//	Tracks   | prev track    | next track   | -5s       | +5s
//	Queue    | prev item     | next item    | seek -5s  | seek +5s
//
// Global keys (work in any panel):
//
//	Tab / S-Tab  cycle focus forward / backward
//	enter        activate (Library: drill in or select artist;
//	             Tracks: play selected track)
//	space        play / pause
//	n / p        next / previous track
//	home / end   seek to start / end
//	+ / -        volume up / down
//	m            mute toggle
//	r            rescan
//	s            shuffle
//	R            repeat (off -> all -> one)
//	/            search
//	esc          clear filter (or exit search)
//	C-l          clear filter
//	?            help
//	x            stop
//	1 / 2 / 3    focus Library / Tracks / Queue
//	q            quit
func defaultKeys() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Next: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next"),
		),
		Prev: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev"),
		),
		Play: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("⏎", "play"),
		),
		Pause: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "play/pause"),
		),
		Stop: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "stop"),
		),
		VolUp: key.NewBinding(
			key.WithKeys("+", "="),
			key.WithHelp("+", "vol +"),
		),
		VolDown: key.NewBinding(
			key.WithKeys("-", "_"),
			key.WithHelp("-", "vol -"),
		),
		Rescan: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "rescan"),
		),
		Shuffle: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "shuffle"),
		),
		Repeat: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "repeat"),
		),
		Mute: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "mute"),
		),
		SeekFwd: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "+5s"),
		),
		SeekBwd: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "-5s"),
		),
		SeekHome: key.NewBinding(
			key.WithKeys("home"),
			key.WithHelp("home", "start"),
		),
		SeekEnd: key.NewBinding(
			key.WithKeys("end"),
			key.WithHelp("end", "end"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Clear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("C-l", "clear filter"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		QueueUp: key.NewBinding(
			key.WithKeys("K", "shift+up"),
			key.WithHelp("K/S-↑", "move up"),
		),
		QueueDown: key.NewBinding(
			key.WithKeys("J", "shift+down"),
			key.WithHelp("J/S-↓", "move down"),
		),
		QueueDel: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "queue del"),
		),
		QueueClear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "queue clear"),
		),
		QueueAdd: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add to queue"),
		),
		PlayNext: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "play next"),
		),
		Playlists: key.NewBinding(
			key.WithKeys("P"),
			key.WithHelp("P", "playlists"),
		),
	}
}

type playlistMode int

const (
	playlistClosed playlistMode = iota
	playlistBrowse
	playlistName
	playlistOverwrite
)

// Model is the root Bubble Tea model for galdr.
type Model struct {
	app    *app.App
	keys   keyMap
	styles theme.Palette
	uiCfg  UIConfig
	tr     i18n.Translator
	focus  *FocusManager

	durations durationProbeState

	width  int
	height int
	help   bool
	// mediumMain remembers which panel occupies the main area while the
	// Library panel has focus in medium mode.
	mediumMain PanelID

	searchMode      bool
	search          textinput.Model
	playlistMode    playlistMode
	playlistNames   []string
	playlistCursor  int
	playlistInput   textinput.Model
	pendingPlaylist string

	// libCursor is the position of the selection in the flat
	// list of Library rows. It is local to the TUI and does not
	// affect playback directly: pressing Enter on an album row
	// updates the App's scope, which the Tracks panel observes.
	libCursor int
	// libExpanded remembers which artists are expanded in the
	// Library panel. Persists across rescan.
	libExpanded map[string]bool

	// queueCursor is the position of the selection in the Queue
	// panel. It is a full-list index (queue position), not a
	// visible index. The Queue panel uses it to highlight the
	// row under the cursor and to drive reorder / remove.
	queueCursor int

	coverTrackPath string
	coverArt       string

	playbackPublisher playbackPublisher
}

type playbackPublisher interface {
	Publish(app.PlaybackSnapshot)
	Seeked(time.Duration)
}

// New constructs a TUI model backed by a, using palette for styling
// and uiCfg for the panel layout. Callers typically pass
// theme.PaletteFor(string(cfg.Theme)) and convert their config.UIConfig
// into a tui.UIConfig.
// Option customizes a Model while preserving existing constructor calls.
type Option func(*Model)

// WithTranslator uses tr for all user-facing TUI text.
func WithTranslator(tr i18n.Translator) Option {
	return func(m *Model) { m.tr = tr }
}

func New(a *app.App, palette theme.Palette, uiCfg UIConfig, prober DurationProber, options ...Option) *Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.CharLimit = 100
	pi := textinput.New()
	pi.Prompt = "> "
	pi.CharLimit = 128
	m := &Model{
		app:           a,
		keys:          defaultKeys(),
		styles:        palette,
		uiCfg:         uiCfg,
		tr:            i18n.New(i18n.English),
		focus:         NewFocusManager(),
		mediumMain:    PanelTracks,
		search:        ti,
		playlistInput: pi,
		libExpanded:   make(map[string]bool),
		durations:     durationProbeState{prober: prober},
	}
	for _, option := range options {
		option(m)
	}
	m.search.Placeholder = m.tr.T(i18n.SearchPlaceholder)
	m.playlistInput.Placeholder = m.tr.T(i18n.PlaylistNamePlaceholder)
	return m
}

// SetPlaybackPublisher attaches an observer such as the MPRIS service. The
// observer is called only from the Bubble Tea owner goroutine.
func (m *Model) SetPlaybackPublisher(publisher playbackPublisher) {
	m.playbackPublisher = publisher
}

// Init implements tea.Model. It returns a tick command so the status
// display can refresh the playback position periodically.
func (m *Model) Init() tea.Cmd {
	commands := []tea.Cmd{tickCmd(), m.startDurationProbes()}
	if events, ok := m.app.PlaybackEvents(); ok {
		commands = append(commands, waitPlaybackEventCmd(events))
	}
	return tea.Batch(commands...)
}

// Update implements tea.Model. It dispatches keyboard input and resizes
// the view to the terminal dimensions.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer m.publishPlayback()
	switch msg := msg.(type) {
	case app.PlaybackRequest:
		result := msg.Apply(m.app)
		m.publishPlayback()
		if result.Seeked && m.playbackPublisher != nil {
			m.playbackPublisher.Seeked(result.Snapshot.Position)
		}
		msg.Respond(result)
		return m, m.reconcileDurationProbes()

	case playbackEventMsg:
		if !msg.ok {
			return m, nil
		}
		before := currentTrackPath(m.app)
		_ = m.app.HandlePlaybackEvent(msg.event)
		m.alignTracksIfCurrentChanged(before)
		m.queueCursorToCurrent()
		events, ok := m.app.PlaybackEvents()
		if !ok {
			return m, nil
		}
		return m, tea.Batch(waitPlaybackEventCmd(events), m.reconcileDurationProbes())

	case durationProbeMsg:
		return m, m.handleDurationProbe(msg)

	case durationSummaryExpiredMsg:
		if msg.generation == m.durations.generation {
			m.durations.showSummary = false
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.help {
			switch {
			case key.Matches(msg, m.keys.Help), msg.Type == tea.KeyEsc:
				m.help = false
				return m, nil
			case key.Matches(msg, m.keys.Quit):
				m.cancelDurationProbeGeneration()
				return m, tea.Quit
			default:
				return m, nil
			}
		}
		if m.playlistMode != playlistClosed {
			return m, m.updatePlaylists(msg)
		}
		// In search mode, every key except Enter, Esc and Quit is
		// fed to the text input. The filter is updated on every
		// change so the list re-renders incrementally.
		if m.searchMode {
			switch {
			case key.Matches(msg, m.keys.Play):
				m.exitSearchMode()
				return m, nil
			case msg.Type == tea.KeyEsc:
				m.exitSearchMode()
				return m, nil
			case key.Matches(msg, m.keys.Quit):
				m.exitSearchMode()
				m.cancelDurationProbeGeneration()
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.app.SetFilter(m.search.Value())
			return m, cmd
		}

		// Tab and Shift+Tab cycle focus between panels. They
		// are not bound to the keyMap because they are pure
		// navigation primitives and have no help entry.
		if msg.Type == tea.KeyTab {
			m.focus.Cycle()
			m.rememberMainPanel()
			return m, nil
		}
		if msg.Type == tea.KeyShiftTab {
			m.focus.CycleBack()
			m.rememberMainPanel()
			return m, nil
		}
		switch msg.String() {
		case "1":
			m.setFocus(PanelLibrary)
			return m, nil
		case "2":
			m.setFocus(PanelTracks)
			return m, nil
		case "3":
			m.setFocus(PanelQueue)
			return m, nil
		}

		// Library panel: j/k/h/l/enter are local navigation
		// keys. They are interpreted differently when the
		// Library panel is focused. We reuse the SeekBwd/SeekFwd
		// bindings for left/right because they map to the same
		// physical keys; the Seek handler still runs when the
		// Tracks panel is focused.
		if m.focus.Current() == PanelLibrary {
			switch {
			case key.Matches(msg, m.keys.Up):
				m.libCursorMove(-1)
				return m, nil
			case key.Matches(msg, m.keys.Down):
				m.libCursorMove(+1)
				return m, nil
			case key.Matches(msg, m.keys.SeekBwd):
				m.libCollapse()
				return m, nil
			case key.Matches(msg, m.keys.SeekFwd):
				m.libExpandOrDrill()
				return m, nil
			case key.Matches(msg, m.keys.Play):
				m.libActivate()
				return m, nil
			}
			// Any other Library-focused key (e.g. volume, search)
			// falls through to the global handler below.
		}

		// Queue panel: j/k move the cursor, K/J (shift+up/down)
		// move the highlighted track, d removes, c clears,
		// enter plays. h/l keep their global seek behavior.
		if m.focus.Current() == PanelQueue {
			switch {
			case key.Matches(msg, m.keys.Up):
				m.queueCursorMove(-1)
				return m, nil
			case key.Matches(msg, m.keys.Down):
				m.queueCursorMove(+1)
				return m, nil
			case key.Matches(msg, m.keys.QueueUp):
				if m.app.MoveQueueUp(m.queueCursor) {
					m.queueCursor--
				}
				m.queueCursorClamp()
				return m, nil
			case key.Matches(msg, m.keys.QueueDown):
				if m.app.MoveQueueDown(m.queueCursor) {
					m.queueCursor++
				}
				m.queueCursorClamp()
				return m, nil
			case key.Matches(msg, m.keys.QueueDel):
				if m.app.RemoveFromQueue(m.queueCursor) {
					m.queueCursorClamp()
				}
				return m, nil
			case key.Matches(msg, m.keys.QueueClear):
				m.app.ClearQueue()
				m.queueCursorClamp()
				return m, nil
			case key.Matches(msg, m.keys.Play):
				if err := m.app.PlayAtIndex(m.queueCursor); err == nil {
					m.app.SelectCurrentInScope()
				}
				return m, m.reconcileDurationProbes()
			}
			// Other Queue-focused keys (volume, search) fall
			// through to the global handler below.
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelDurationProbeGeneration()
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help = !m.help
		case key.Matches(msg, m.keys.Playlists):
			m.openPlaylists()
		case key.Matches(msg, m.keys.Search):
			return m, m.enterSearchMode()
		case key.Matches(msg, m.keys.Clear):
			m.clearFilter()
		case key.Matches(msg, m.keys.Up):
			m.app.SelectPrevScoped()
		case key.Matches(msg, m.keys.Down):
			m.app.SelectNextScoped()
		case key.Matches(msg, m.keys.QueueAdd):
			if m.focus.Current() == PanelTracks {
				_ = m.app.AddSelectedToQueue()
			}
		case key.Matches(msg, m.keys.PlayNext):
			if m.focus.Current() == PanelTracks {
				_ = m.app.PlaySelectedNext()
			}
		case key.Matches(msg, m.keys.Play):
			_ = m.app.PlaySelected()
			m.queueCursorToCurrent()
		case key.Matches(msg, m.keys.Pause):
			_ = m.app.TogglePlay()
		case key.Matches(msg, m.keys.Stop):
			_ = m.app.Stop()
		case key.Matches(msg, m.keys.Next):
			before := currentTrackPath(m.app)
			_ = m.app.Next()
			m.alignTracksIfCurrentChanged(before)
			m.queueCursorToCurrent()
		case key.Matches(msg, m.keys.Prev):
			before := currentTrackPath(m.app)
			_ = m.app.Previous()
			m.alignTracksIfCurrentChanged(before)
			m.queueCursorToCurrent()
		case key.Matches(msg, m.keys.VolUp):
			_ = m.app.VolumeUp()
		case key.Matches(msg, m.keys.VolDown):
			_ = m.app.VolumeDown()
		case key.Matches(msg, m.keys.Rescan):
			if err := m.app.Rescan(); err != nil {
				return m, nil
			}
			m.coverTrackPath = ""
			m.coverArt = ""
			m.queueCursorToCurrent()
			return m, m.startDurationProbes()
		case key.Matches(msg, m.keys.Shuffle):
			m.app.ToggleShuffle()
			m.queueCursorToCurrent()
		case key.Matches(msg, m.keys.Repeat):
			m.app.CycleRepeat()
		case key.Matches(msg, m.keys.Mute):
			m.app.ToggleMute()
		case key.Matches(msg, m.keys.SeekFwd):
			m.seekRelative(SeekStep)
		case key.Matches(msg, m.keys.SeekBwd):
			m.seekRelative(-SeekStep)
		case key.Matches(msg, m.keys.SeekHome):
			m.seek(0)
		case key.Matches(msg, m.keys.SeekEnd):
			m.seek(m.app.Duration())
		case msg.Type == tea.KeyEsc:
			// No-op in search mode (handled above). Out of search
			// mode, Esc clears the filter for symmetry with vim.
			m.clearFilter()
		}
		return m, m.reconcileDurationProbes()

	case tickMsg:
		// Auto-advance to the next track when the current one ends
		// naturally. Has no effect while the user is paused or has
		// manually stopped. Keep the Queue cursor independent: a periodic
		// refresh must not interrupt someone browsing upcoming tracks.
		before := currentTrackPath(m.app)
		_ = m.app.MaybeAdvance()
		m.alignTracksIfCurrentChanged(before)
		return m, tea.Batch(tickCmd(), m.reconcileDurationProbes())
	}

	return m, nil
}

// enterSearchMode seeds the search input with the active filter (so
// the user can edit the existing pattern) and focuses it. A blink
// command is returned so the cursor animates.
func (m *Model) enterSearchMode() tea.Cmd {
	m.searchMode = true
	m.search.SetValue(m.app.Filter())
	m.search.CursorEnd()
	m.search.Focus()
	return textinput.Blink
}

// exitSearchMode blurs the search input. The filter stays active.
func (m *Model) exitSearchMode() {
	m.searchMode = false
	m.search.Blur()
}

// clearFilter empties the search input and clears the filter on the
// app. Used by both Esc and Ctrl+L out of search mode.
func (m *Model) clearFilter() {
	m.search.SetValue("")
	m.app.SetFilter("")
}

func (m *Model) setFocus(id PanelID) {
	m.focus.Set(id)
	m.rememberMainPanel()
}

func (m *Model) rememberMainPanel() {
	current := m.focus.Current()
	if current == PanelTracks || current == PanelQueue {
		m.mediumMain = current
	}
}

// libRows returns the current flat list of Library rows. It is a
// thin wrapper around libraryPanel() that also handles the
// nil-tree case.
func (m *Model) libRows() []libRow {
	if m.app.Tree() == nil {
		return nil
	}
	return libraryPanel(m.app.Tree(), m.libExpanded)
}

// libCursorMove moves the Library cursor by delta, clamping to
// the row range. A no-op when the library is empty.
func (m *Model) libCursorMove(delta int) {
	rows := m.libRows()
	if len(rows) == 0 {
		return
	}
	m.libCursor += delta
	if m.libCursor < 0 {
		m.libCursor = 0
	}
	if m.libCursor >= len(rows) {
		m.libCursor = len(rows) - 1
	}
}

// libCollapse handles the Library panel's collapse / go-up key.
//   - On an artist row that is expanded, the artist is collapsed.
//   - On an album row, the scope is cleared and the cursor moves
//     to the parent artist row (so the user can keep going up).
//   - On a collapsed artist row, no-op (you cannot collapse what
//     is not expanded).
func (m *Model) libCollapse() {
	rows := m.libRows()
	if m.libCursor < 0 || m.libCursor >= len(rows) {
		return
	}
	row := rows[m.libCursor]
	switch row.Kind {
	case libRowArtist:
		if m.libExpanded[row.Artist] {
			delete(m.libExpanded, row.Artist)
		}
	case libRowAlbum:
		// Clear the album scope so the Tracks panel shows the
		// whole artist again. Then move the cursor up to the
		// parent artist row.
		m.app.SetScope(row.Artist, "")
		// Find the parent artist row (the first row with the
		// same artist name at depth 0).
		for i := m.libCursor - 1; i >= 0; i-- {
			if rows[i].Kind == libRowArtist && rows[i].Artist == row.Artist {
				m.libCursor = i
				break
			}
		}
	}
}

// libExpandOrDrill handles the Library panel's expand / drill-in
// key.
//   - On a collapsed artist row, expand it.
//   - On an expanded artist row, no-op (use ↓ to go to an album).
//   - On an album row, set the scope to (artist, album) and
//     move focus to the Tracks panel.
func (m *Model) libExpandOrDrill() {
	rows := m.libRows()
	if m.libCursor < 0 || m.libCursor >= len(rows) {
		return
	}
	row := rows[m.libCursor]
	if row.Kind != libRowArtist {
		return
	}
	if !m.libExpanded[row.Artist] {
		m.libExpanded[row.Artist] = true
		return
	}
	// Already expanded: try to move cursor to the first album.
	for i := m.libCursor + 1; i < len(rows); i++ {
		if rows[i].Kind == libRowArtist {
			return
		}
		if rows[i].Kind == libRowAlbum && rows[i].Artist == row.Artist {
			m.libCursor = i
			return
		}
	}
}

// libActivate handles Enter on a Library row.
//   - On an artist row, narrow the scope to that artist. The
//     artist is also expanded (so the user sees its albums).
//     Focus moves to the Tracks panel so the user can start
//     playing.
//   - On an album row, narrow the scope to (artist, album) and
//     move focus to the Tracks panel.
func (m *Model) libActivate() {
	rows := m.libRows()
	if m.libCursor < 0 || m.libCursor >= len(rows) {
		return
	}
	row := rows[m.libCursor]
	switch row.Kind {
	case libRowArtist:
		m.app.SetScope(row.Artist, "")
		m.libExpanded[row.Artist] = true
		m.setFocus(PanelTracks)
	case libRowAlbum:
		m.app.SetScope(row.Artist, row.Album)
		m.setFocus(PanelTracks)
	}
}

// queueCursorMove moves the Queue cursor by delta, clamping to
// the queue range. A no-op when the queue is empty.
func (m *Model) queueCursorMove(delta int) {
	n := m.app.Queue().Len()
	if n == 0 {
		m.queueCursor = 0
		return
	}
	m.queueCursor += delta
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}
	if m.queueCursor >= n {
		m.queueCursor = n - 1
	}
}

// queueCursorClamp re-aligns the Queue cursor to a valid range.
// Called after a Remove or Clear, which can shrink the queue.
func (m *Model) queueCursorClamp() {
	n := m.app.Queue().Len()
	if n == 0 {
		m.queueCursor = 0
		return
	}
	if m.queueCursor >= n {
		m.queueCursor = n - 1
	}
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}
}

func (m *Model) queueCursorToCurrent() {
	current := m.app.Current()
	if current == nil {
		m.queueCursorClamp()
		return
	}
	for i, track := range m.app.Queue().Tracks() {
		if track.Path == current.Path {
			m.queueCursor = i
			return
		}
	}
	m.queueCursorClamp()
}

func (m *Model) alignTracksIfCurrentChanged(previousPath string) {
	currentPath := currentTrackPath(m.app)
	if currentPath != "" && currentPath != previousPath {
		m.app.SelectCurrentInScope()
	}
}

func currentTrackPath(a *app.App) string {
	if current := a.Current(); current != nil {
		return current.Path
	}
	return ""
}

// View implements tea.Model.
func (m *Model) View() string {
	width := m.width
	height := m.height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}
	if m.help {
		return m.helpViewSized(width, height)
	}
	if m.playlistMode != playlistClosed {
		return m.playlistViewSized(width, height)
	}

	layout := compute(width, height, m.uiCfg, m.styles, m.tr)
	if layout.TooSmall {
		return m.tooSmallView(layout)
	}

	layout.Library.Title = fmt.Sprintf("%s  %d", m.tr.T(i18n.Library), len(m.libRows()))
	layout.Tracks.Title = fmt.Sprintf("%s  %d", m.tr.T(i18n.Tracks), len(m.app.ScopedTracks()))
	layout.Queue.Title = fmt.Sprintf("%s  %d", m.tr.T(i18n.Queue), m.app.Queue().Len())
	layout.Library.Content = m.libraryPanelContent
	layout.Tracks.Content = m.tracksPanelContent
	layout.Queue.Content = m.queuePanelContent
	focused := m.focus.Current()
	layout.Library.Focused = focused == PanelLibrary
	layout.Tracks.Focused = focused == PanelTracks
	layout.Queue.Focused = focused == PanelQueue

	header := m.nowPlayingView(layout.NowPlaying.W, layout.NowPlaying.H)
	body := m.navigationView(layout)
	footer := m.footerView(layout.Footer.W, layout.Footer.H)
	return header + "\n" + body + "\n" + footer
}

func (m *Model) navigationView(layout Layout) string {
	switch layout.Mode {
	case LayoutWide:
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			layout.Library.View(),
			verticalDivider(m.styles, layout.Library.H),
			layout.Tracks.View(),
			verticalDivider(m.styles, layout.Library.H),
			layout.Queue.View(),
		)
	case LayoutMedium:
		main := layout.Tracks
		if m.focus.Current() == PanelQueue ||
			(m.focus.Current() == PanelLibrary && m.mediumMain == PanelQueue) {
			main = layout.Queue
		}
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			layout.Library.View(),
			verticalDivider(m.styles, layout.Library.H),
			main.View(),
		)
	default:
		switch m.focus.Current() {
		case PanelLibrary:
			return layout.Library.View()
		case PanelQueue:
			return layout.Queue.View()
		default:
			return layout.Tracks.View()
		}
	}
}

// tooSmallView renders the "terminal too small" message centered
// in the available area. It does not render panels.
func (m *Model) tooSmallView(layout Layout) string {
	if layout.Width <= 0 || layout.Height <= 0 {
		return layout.TooSmallMsg
	}
	return lipgloss.Place(
		layout.Width, layout.Height,
		lipgloss.Center, lipgloss.Center,
		m.styles.TooSmallMsg.Render(layout.TooSmallMsg),
	)
}

// libraryPanelContent renders the contents of the left Library
// panel: a flat list of artist and album rows, with the row
// under the cursor highlighted. The scope (artist/album) of the
// App is shown by a `▶` marker on the matching rows.
//
// j/k navigation is handled by the model's Update method. This
// function only renders. Empty library / no tree show an empty
// state.
func (m *Model) libraryPanelContent(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	tree := m.app.Tree()
	if tree == nil || tree.Len() == 0 {
		return m.styles.EmptyMsg.Width(w).Height(h).Render(m.tr.T(i18n.EmptyLibrary))
	}
	rows := libraryPanel(tree, m.libExpanded)
	if len(rows) == 0 {
		if m.app.HasFilter() {
			return m.styles.EmptyMsg.Render(m.tr.T(i18n.NoLibraryMatches))
		}
		return m.styles.EmptyMsg.Render(m.tr.T(i18n.LibraryEmpty))
	}
	// Keep the cursor in range.
	if m.libCursor < 0 {
		m.libCursor = 0
	}
	if m.libCursor >= len(rows) {
		m.libCursor = len(rows) - 1
	}
	// Compute the scroll window.
	win := h
	start := scrollStart(len(rows), m.libCursor, win)

	curArtist, curAlbum := m.app.Scope()
	var lines []string
	for i := start; i < len(rows) && i < start+win; i++ {
		row := rows[i]
		// Marker column reflects state.
		var marker string
		switch row.Kind {
		case libRowArtist:
			if m.libExpanded[row.Artist] {
				marker = "▾"
			} else {
				marker = "▸"
			}
		default:
			// Album row: show "▶" when the current scope is this
			// album, otherwise a neutral " ".
			if curArtist == row.Artist && curAlbum == row.Album {
				marker = "▶"
			} else {
				marker = " "
			}
		}
		indent := strings.Repeat("  ", row.Depth)
		label := row.Artist
		if row.Kind == libRowAlbum {
			label = row.Album
		}
		count := libraryRowTrackCount(tree, row)
		countText := fmt.Sprintf("(%d)", count)
		base := fmt.Sprintf("%s%s %s", indent, marker, label)
		labelW := w - lipgloss.Width(countText) - 1
		text := padRight(base, max(0, labelW))
		if labelW > 0 {
			text += " " + countText
		}
		text = fitLine(text, w)
		if i == m.libCursor {
			lines = append(lines, m.styles.SelectedRow.Width(w).Render(text))
		} else if row.Kind == libRowArtist && curArtist == row.Artist && curAlbum == "" {
			// Current playing artist: highlight softly.
			lines = append(lines, m.styles.PlayingRow.Render(text))
		} else if row.Kind == libRowAlbum && curArtist == row.Artist && curAlbum == row.Album {
			lines = append(lines, m.styles.PlayingRow.Render(text))
		} else {
			lines = append(lines, m.styles.Row.Render(text))
		}
	}
	return strings.Join(lines, "\n")
}

func libraryRowTrackCount(tree *library.Tree, row libRow) int {
	if tree == nil {
		return 0
	}
	if row.Kind == libRowAlbum {
		if album := findAlbum(tree, row.Artist, row.Album); album != nil {
			return album.TrackCount
		}
		return 0
	}
	for _, artist := range tree.Artists() {
		if artist.Name != row.Artist {
			continue
		}
		total := 0
		for _, album := range artist.Albums {
			total += album.TrackCount
		}
		return total
	}
	return 0
}

// findAlbum returns the AlbumView for the (artist, album) pair, or
// nil if not found. It walks the tree's Artists() output.
func findAlbum(tree *library.Tree, artist, album string) *library.AlbumView {
	for _, a := range tree.Artists() {
		if a.Name != artist {
			continue
		}
		for i, al := range a.Albums {
			if al.Name == album {
				return &a.Albums[i]
			}
		}
	}
	return nil
}

// tracksPanelContent renders the contents of the central Tracks
// panel: the scoped track list (Artist -> Album -> Track), the
// search input (when active), and the error line (when set). It
// is called with the panel's inner dimensions (W-2, H-2). The
// list gets whatever vertical space remains after the optional
// search and error lines.
//
// In v2 the visible tracks are the App's ScopedTracks, not the
// full queue: when the user navigates the Library to an album
// (or an artist), this panel narrows to that scope. The
// selection is updated through app.SelectNextScoped /
// SelectPrevScoped.
func (m *Model) tracksPanelContent(w, h int) string {
	if h <= 0 || w <= 0 {
		return ""
	}
	return m.listViewSized(w, h)
}

// listViewSized renders the (scoped) track list inside a
// rectangle of (w, h) cells. The selection is kept in view via
// scrollStart. When the scope and filter hide every track, an
// empty-state message is rendered instead.
//
// In v2 the list is the App's ScopedTracks (intersection of the
// active scope and the search filter). The selected row is
// ScopedIndex, not DisplayIndex.
func (m *Model) listViewSized(w, h int) string {
	visible := m.app.ScopedTracks()
	if len(visible) == 0 {
		msg := m.tr.T(i18n.EmptyScope)
		artist, album := m.app.Scope()
		if artist != "" && album != "" {
			msg = m.tr.T(i18n.EmptyAlbum, artist, album)
		} else if artist != "" {
			msg = m.tr.T(i18n.EmptyArtist, artist)
		}
		if m.app.HasFilter() {
			msg = m.tr.T(i18n.NoTrackMatches, m.app.Filter())
		} else if m.app.Tree() == nil || m.app.Tree().Len() == 0 {
			msg = m.tr.T(i18n.NoMusicFound)
		}
		if w <= 0 || h <= 0 {
			return ""
		}
		return m.styles.EmptyMsg.Render(msg)
	}
	if h <= 0 {
		return ""
	}

	selected := m.app.ScopedIndex()
	start := scrollStart(len(visible), selected, h)

	cur := m.app.Current()
	curPath := ""
	if cur != nil {
		curPath = cur.Path
	}

	var rows []string
	for i := start; i < len(visible) && i < start+h; i++ {
		rows = append(rows, m.renderRow(
			visible[i],
			i+1,
			i == selected,
			visible[i].Path == curPath,
			w,
		))
	}
	return strings.Join(rows, "\n")
}

// renderRow formats a single list row, with different styles for the
// selected and currently playing rows. Long titles are truncated to
// fit the given column width.
func (m *Model) renderRow(t library.Track, number int, selected, playing bool, w int) string {
	marker := " "
	switch {
	case selected && playing:
		marker = "▶"
	case selected:
		marker = "›"
	case playing:
		marker = "▶"
	}
	if t.Track > 0 {
		number = t.Track
	}
	prefix := fmt.Sprintf("%s %2d ", marker, number)
	duration := formatDurationOrUnknown(t.Duration, t.Duration > 0)
	remaining := w - lipgloss.Width(prefix) - lipgloss.Width(duration) - 1
	if remaining < 0 {
		remaining = 0
	}

	secondary := ""
	artist, album := m.app.Scope()
	if layoutMode(m.width) == LayoutWide && remaining >= 28 {
		switch {
		case artist == "":
			secondary = strings.Trim(strings.Join([]string{t.Artist, t.Album}, " — "), " —")
		case album == "":
			secondary = t.Album
		default:
			secondary = t.Artist
		}
	} else if layoutMode(m.width) == LayoutMedium && remaining >= 24 {
		if artist == "" {
			secondary = t.Artist
		} else if album == "" {
			secondary = t.Album
		}
	}

	titleW := remaining
	secondaryText := ""
	if secondary != "" {
		secondaryW := min(22, max(10, remaining/3))
		titleW = remaining - secondaryW - 2
		if titleW < 8 {
			titleW = remaining
		} else {
			secondaryText = "  " + padRight(secondary, secondaryW)
		}
	}
	text := prefix + padRight(t.Title, titleW) + secondaryText + " " + duration
	text = fitLine(text, w)

	switch {
	case selected:
		return m.styles.SelectedRow.Render(text)
	case playing:
		return m.styles.PlayingRow.Render(text)
	default:
		return m.styles.Row.Render(text)
	}
}

// statusView is retained as a compact textual status for callers and tests.
// The main view uses separate now-playing and footer regions.
func (m *Model) statusView(width int) string {
	parts := []string{m.nowPlayingView(width, 3), m.footerMessage(width)}
	if m.app.HasFilter() {
		parts = append(parts, m.tr.T(
			i18n.FilterBadge,
			m.app.Filter(), m.app.VisibleLen(), m.app.ScopedTotal(),
		))
	}
	if m.app.Muted() {
		parts = append(parts, m.tr.T(i18n.MuteBadge))
	}
	if m.app.Shuffle() {
		parts = append(parts, m.tr.T(i18n.ShuffleBadge))
	}
	if r := m.app.Repeat(); r != app.RepeatOff {
		parts = append(parts, m.tr.T(i18n.RepeatBadge, m.repeatMode(r)))
	}
	return strings.Join(parts, "  ")
}

func (m *Model) nowPlayingView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	state, icon := m.tr.T(i18n.Stopped), "■"
	switch m.app.State() {
	case player.StatePlaying:
		state, icon = m.tr.T(i18n.Playing), "▶"
	case player.StatePaused:
		state, icon = m.tr.T(i18n.Paused), "Ⅱ"
	}
	title := m.tr.T(i18n.NothingPlaying)
	artist := ""
	album := ""
	if current := m.app.Current(); current != nil {
		title = current.Title
		artist = current.Artist
		album = current.Album
	}
	metadata := strings.Trim(strings.Join([]string{artist, album}, " — "), " —")
	if metadata == "" && m.app.Current() != nil {
		metadata = m.tr.T(i18n.UnknownArtist)
	}

	cover := ""
	contentWidth := width
	const coverWidth = 8
	if height > 3 && width >= 64 {
		cover = m.albumCoverForCurrent(coverWidth, 4)
		if cover != "" {
			contentWidth -= coverWidth + 2
		}
	}

	positionValue := m.app.Position()
	durationValue := m.app.Duration()
	position := formatDuration(positionValue)
	duration := formatDurationOrUnknown(durationValue, durationValue > 0)
	flags := m.playbackFlags(contentWidth)
	timing := position + " / " + duration
	barWidth := contentWidth - lipgloss.Width(timing) - lipgloss.Width(flags) - 4
	if barWidth < 6 {
		barWidth = 6
	}
	progress := renderProgressBar(positionValue, durationValue, barWidth-2)
	progressLine := progress + "  " + timing + "  " + flags

	if height <= 3 {
		first := m.styles.State.Render(icon+" "+state) + "  " + m.styles.NowPlaying.Render(title)
		return strings.Join([]string{
			fitLine(first, width),
			fitLine(progressLine, width),
			m.styles.Divider.Render(strings.Repeat("━", width)),
		}, "\n")
	}

	textLines := []string{
		fitLine(m.styles.PanelTitle.Render(m.tr.T(i18n.NowPlaying))+"  "+m.styles.State.Render(icon+" "+state), contentWidth),
		fitLine(m.styles.NowPlaying.Render(title), contentWidth),
		fitLine(m.styles.Metadata.Render(metadata), contentWidth),
		fitLine(progressLine, contentWidth),
	}
	lines := textLines
	if cover != "" {
		coverLines := strings.Split(cover, "\n")
		lines = make([]string, 0, 5)
		for i := range 4 {
			lines = append(lines, fitLine(coverLines[i]+"  "+textLines[i], width))
		}
	}
	lines = append(lines, m.styles.Divider.Render(strings.Repeat("━", width)))
	return strings.Join(fitLines(strings.Join(lines, "\n"), width, height), "\n")
}

func (m *Model) albumCoverForCurrent(width, height int) string {
	current := m.app.Current()
	if current == nil {
		m.coverTrackPath = ""
		m.coverArt = ""
		return ""
	}
	if current.Path == m.coverTrackPath {
		return m.coverArt
	}
	m.coverTrackPath = current.Path
	m.coverArt = ""
	coverPath := library.FindAlbumCover(current.Path)
	if coverPath == "" {
		return ""
	}
	art, err := renderAlbumCover(coverPath, width, height)
	if err == nil {
		m.coverArt = art
	}
	return m.coverArt
}

func (m *Model) playbackFlags(width int) string {
	muted, shuffled := m.tr.T(i18n.Off), m.tr.T(i18n.Off)
	if m.app.Muted() {
		muted = m.tr.T(i18n.On)
	}
	if m.app.Shuffle() {
		shuffled = m.tr.T(i18n.On)
	}
	if width < 72 {
		muteFlag, shuffleFlag := "-", "-"
		if m.app.Muted() {
			muteFlag = "+"
		}
		if m.app.Shuffle() {
			shuffleFlag = "+"
		}
		return fmt.Sprintf("V%d M%s S%s R:%s", m.app.Volume(), muteFlag, shuffleFlag, m.repeatMode(m.app.Repeat()))
	}
	return fmt.Sprintf(
		"%s %d%%  %s %s  %s %s  %s %s",
		m.tr.T(i18n.Volume),
		m.app.Volume(),
		m.tr.T(i18n.Mute),
		muted,
		m.tr.T(i18n.Shuffle),
		shuffled,
		m.tr.T(i18n.Repeat),
		m.repeatMode(m.app.Repeat()),
	)
}

func (m *Model) repeatMode(mode app.RepeatMode) string {
	switch mode {
	case app.RepeatAll:
		return m.tr.T(i18n.All)
	case app.RepeatOne:
		return m.tr.T(i18n.One)
	default:
		return m.tr.T(i18n.Off)
	}
}

func (m *Model) footerView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := []string{m.footerMessage(width)}
	if height > 1 {
		lines = append(lines, m.shortcutLine(width))
	}
	return strings.Join(fitLines(strings.Join(lines, "\n"), width, height), "\n")
}

func (m *Model) footerMessage(width int) string {
	if m.searchMode {
		countValue := m.app.VisibleLen()
		count := "  " + m.tr.N(countValue, i18n.ResultsOne, i18n.ResultsOther, countValue, m.app.ScopedTotal())
		m.search.Width = max(1, width-lipgloss.Width(count)-3)
		return m.styles.SearchBar.Render(fitLine(m.search.View()+count, width))
	}

	message := m.app.Status()
	style := m.styles.Footer
	if err := m.app.Error(); err != nil {
		message = m.tr.T(i18n.ErrorPrefix) + " " + err.Error()
		style = m.styles.ErrorMsg
		return style.Render(fitLine(message, width))
	}
	context := m.scopeAndFilter()
	if message != "" && context != "" {
		message += "  ·  " + context
	} else if context != "" {
		message = context
	}
	if durationStatus := m.durationFooterStatus(); durationStatus != "" {
		if message != "" {
			message += "  ·  " + durationStatus
		} else {
			message = durationStatus
		}
	}
	return style.Render(fitLine(message, width))
}

func (m *Model) scopeAndFilter() string {
	artist, album := m.app.Scope()
	scope := m.tr.T(i18n.ScopeAll)
	if artist != "" {
		scope = m.tr.T(i18n.ScopeNamed, artist)
		if album != "" {
			scope += "/" + album
		}
	}
	if !m.app.HasFilter() {
		return scope
	}
	return m.tr.T(
		i18n.FilterContext,
		scope,
		m.app.Filter(),
		m.app.VisibleLen(),
		m.app.ScopedTotal(),
	)
}

func (m *Model) shortcutLine(width int) string {
	var shortcuts string
	switch m.focus.Current() {
	case PanelLibrary:
		shortcuts = m.tr.T(i18n.ShortcutLibrary)
	case PanelQueue:
		shortcuts = m.tr.T(i18n.ShortcutQueue)
	default:
		shortcuts = m.tr.T(i18n.ShortcutTracks)
	}
	shortcuts += "  ·  " + m.tr.T(i18n.ShortcutGlobal)
	return m.styles.Footer.Render(fitLine(shortcuts, width))
}

// seekRelative moves the playhead by delta relative to the current
// position, clamped to [0, Duration].
func (m *Model) seekRelative(delta time.Duration) {
	target := m.app.Position() + delta
	if target < 0 {
		target = 0
	}
	if max := m.app.Duration(); max > 0 && target > max {
		target = max
	}
	m.seek(target)
}

func (m *Model) seek(target time.Duration) {
	if err := m.app.Seek(target); err == nil && m.playbackPublisher != nil {
		m.playbackPublisher.Seeked(m.app.Position())
	}
}

func (m *Model) publishPlayback() {
	if m.playbackPublisher != nil {
		m.playbackPublisher.Publish(m.app.PlaybackSnapshot())
	}
}

func (m *Model) openPlaylists() {
	names, err := m.app.Playlists()
	if err != nil {
		return
	}
	m.playlistNames = names
	m.playlistCursor = clamp(m.playlistCursor, 0, max(0, len(names)-1))
	m.playlistMode = playlistBrowse
}

func (m *Model) updatePlaylists(msg tea.KeyMsg) tea.Cmd {
	switch m.playlistMode {
	case playlistBrowse:
		switch {
		case msg.Type == tea.KeyEsc, key.Matches(msg, m.keys.Playlists):
			m.closePlaylists()
		case key.Matches(msg, m.keys.Up):
			m.playlistCursor = clamp(m.playlistCursor-1, 0, max(0, len(m.playlistNames)-1))
		case key.Matches(msg, m.keys.Down):
			m.playlistCursor = clamp(m.playlistCursor+1, 0, max(0, len(m.playlistNames)-1))
		case msg.String() == "s":
			m.playlistInput.SetValue("")
			m.playlistInput.Focus()
			m.playlistMode = playlistName
			return textinput.Blink
		case key.Matches(msg, m.keys.Play):
			if len(m.playlistNames) == 0 {
				return nil
			}
			if err := m.app.LoadPlaylist(m.playlistNames[m.playlistCursor]); err == nil {
				m.queueCursorToCurrent()
				m.closePlaylists()
			}
		}
	case playlistName:
		switch {
		case msg.Type == tea.KeyEsc:
			m.playlistInput.Blur()
			m.playlistMode = playlistBrowse
		case key.Matches(msg, m.keys.Play):
			name := m.playlistInput.Value()
			if err := m.app.SavePlaylist(name, false); errors.Is(err, app.ErrPlaylistExists) {
				m.pendingPlaylist = name
				m.playlistInput.Blur()
				m.playlistMode = playlistOverwrite
			} else if err == nil {
				m.closePlaylists()
			}
		default:
			if msg.Type == tea.KeySpace {
				msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
			}
			var cmd tea.Cmd
			m.playlistInput, cmd = m.playlistInput.Update(msg)
			return cmd
		}
	case playlistOverwrite:
		switch msg.String() {
		case "y", "Y", "o", "O":
			if err := m.app.SavePlaylist(m.pendingPlaylist, true); err == nil {
				m.closePlaylists()
			}
		case "n", "N", "esc":
			m.pendingPlaylist = ""
			m.playlistMode = playlistBrowse
		}
	}
	return nil
}

func (m *Model) closePlaylists() {
	m.playlistMode = playlistClosed
	m.playlistInput.Blur()
	m.pendingPlaylist = ""
}

func (m *Model) playlistViewSized(width, height int) string {
	lines := []string{m.styles.HelpHeader.Render(m.tr.T(i18n.Playlists)), ""}
	switch m.playlistMode {
	case playlistName:
		lines = append(lines,
			m.styles.PanelTitle.Render(m.tr.T(i18n.PlaylistSaveTitle)),
			m.playlistInput.View(),
			"",
			m.styles.StatusVal.Render(m.app.Status()),
		)
	case playlistOverwrite:
		lines = append(lines,
			m.styles.PanelTitle.Render(m.tr.T(i18n.PlaylistOverwrite, m.pendingPlaylist)),
			m.tr.T(i18n.PlaylistOverwriteHint),
		)
	default:
		if len(m.playlistNames) == 0 {
			lines = append(lines, m.styles.EmptyMsg.Render(m.tr.T(i18n.PlaylistEmpty)))
		} else {
			start := scrollStart(len(m.playlistNames), m.playlistCursor, max(1, height-6))
			end := min(len(m.playlistNames), start+max(1, height-6))
			for i := start; i < end; i++ {
				row := "  " + m.playlistNames[i]
				if i == m.playlistCursor {
					row = m.styles.SelectedRow.Render("› " + m.playlistNames[i])
				}
				lines = append(lines, row)
			}
		}
		lines = append(lines, "", m.styles.Footer.Render(m.tr.T(i18n.PlaylistHint)))
	}
	return strings.Join(fitLines(strings.Join(lines, "\n"), width, height), "\n")
}

func (m *Model) helpView() string {
	width, height := m.width, m.height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}
	return m.helpViewSized(width, height)
}

func (m *Model) helpViewSized(width, height int) string {
	left := []string{
		m.styles.HelpHeader.Render(m.tr.T(i18n.HelpTitle)),
		m.styles.PanelTitle.Render(m.tr.T(i18n.HelpNavigation)),
		m.tr.T(i18n.HelpNavMove),
		m.tr.T(i18n.HelpNavOpen),
		m.tr.T(i18n.HelpNavPanel),
		m.tr.T(i18n.HelpNavChoose),
		m.tr.T(i18n.HelpNavSelect),
		m.tr.T(i18n.HelpNavSeek),
		"",
		m.styles.PanelTitle.Render(m.tr.T(i18n.HelpPlayback)),
		m.tr.T(i18n.HelpPlayPause),
		m.tr.T(i18n.HelpStop),
		m.tr.T(i18n.HelpNextPrev),
		m.tr.T(i18n.HelpVolume),
		m.tr.T(i18n.HelpMute),
		m.tr.T(i18n.HelpShuffleRepeat),
	}
	right := []string{
		m.styles.HelpHeader.Render(" "),
		m.styles.PanelTitle.Render(m.tr.T(i18n.HelpLibrarySearch)),
		m.tr.T(i18n.HelpRescan),
		m.tr.T(i18n.HelpSearch),
		m.tr.T(i18n.HelpClearFilter),
		"",
		m.styles.PanelTitle.Render(m.tr.T(i18n.HelpQueue)),
		m.tr.T(i18n.HelpQueueUp),
		m.tr.T(i18n.HelpQueueDown),
		m.tr.T(i18n.HelpQueueRemove),
		m.tr.T(i18n.HelpQueueAdd),
		m.tr.T(i18n.HelpPlayNext),
		"",
		m.styles.PanelTitle.Render(m.tr.T(i18n.HelpApplication)),
		m.tr.T(i18n.HelpPlaylists),
		m.tr.T(i18n.HelpClose),
		m.tr.T(i18n.HelpQuit),
	}

	var content string
	if width >= 110 {
		gap := 3
		columnW := (width - gap) / 2
		leftBlock := strings.Join(fitLines(strings.Join(left, "\n"), columnW, height), "\n")
		rightBlock := strings.Join(fitLines(strings.Join(right, "\n"), width-columnW-gap, height), "\n")
		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftBlock,
			strings.Join(fitLines("", gap, height), "\n"),
			rightBlock,
		)
	} else {
		compact := append(append([]string(nil), left...), right[1:]...)
		content = strings.Join(fitLines(strings.Join(compact, "\n"), width, height), "\n")
	}
	return content
}

// scrollStart returns the index of the first row to display so that the
// selected row stays in view inside a window of size window.
func scrollStart(total, selected, window int) int {
	if window <= 0 || total <= window {
		return 0
	}
	if selected < window-1 {
		return 0
	}
	start := selected - (window - 1)
	if start+window > total {
		start = total - window
	}
	if start < 0 {
		start = 0
	}
	return start
}
