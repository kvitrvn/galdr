// Package tui contains the Bubble Tea models, views and keybindings.
//
// The TUI depends on the player interface from internal/player (through
// internal/app) and never on a concrete audio backend.
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kvitrvn/galdr/internal/app"
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
//	Queue    | (Phase 15)    | (Phase 15)   | (Phase 15)| (Phase 15)
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
			key.WithKeys("right"),
			key.WithHelp("→", "+5s"),
		),
		SeekBwd: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "-5s"),
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
			key.WithKeys("J", "shift+up"),
			key.WithHelp("J", "queue up"),
		),
		QueueDown: key.NewBinding(
			key.WithKeys("K", "shift+down"),
			key.WithHelp("K", "queue down"),
		),
		QueueDel: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "queue del"),
		),
		QueueClear: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "queue clear"),
		),
	}
}

// Model is the root Bubble Tea model for galdr.
type Model struct {
	app    *app.App
	keys   keyMap
	styles theme.Palette
	uiCfg  UIConfig
	focus  *FocusManager

	width  int
	height int
	help   bool

	searchMode bool
	search     textinput.Model

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
}

// New constructs a TUI model backed by a, using palette for styling
// and uiCfg for the panel layout. Callers typically pass
// theme.PaletteFor(string(cfg.Theme)) and convert their config.UIConfig
// into a tui.UIConfig.
func New(a *app.App, palette theme.Palette, uiCfg UIConfig) *Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "search…"
	ti.CharLimit = 100
	return &Model{
		app:         a,
		keys:        defaultKeys(),
		styles:      palette,
		uiCfg:       uiCfg,
		focus:       NewFocusManager(),
		search:      ti,
		libExpanded: make(map[string]bool),
	}
}

// Init implements tea.Model. It returns a tick command so the status
// display can refresh the playback position periodically.
func (m *Model) Init() tea.Cmd {
	return tickCmd()
}

// Update implements tea.Model. It dispatches keyboard input and resizes
// the view to the terminal dimensions.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
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
			return m, nil
		}
		if msg.Type == tea.KeyShiftTab {
			m.focus.CycleBack()
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

		// Queue panel: j/k move the cursor, J/K (shift+up/down)
		// move the highlighted track, d removes, c clears,
		// enter plays. The Queue panel ignores h/l (no
		// collapse/expand semantics here).
		if m.focus.Current() == PanelQueue {
			switch {
			case key.Matches(msg, m.keys.Up):
				m.queueCursorMove(-1)
				return m, nil
			case key.Matches(msg, m.keys.Down):
				m.queueCursorMove(+1)
				return m, nil
			case key.Matches(msg, m.keys.QueueUp):
				m.app.MoveQueueUp(m.queueCursor)
				m.queueCursorClamp()
				return m, nil
			case key.Matches(msg, m.keys.QueueDown):
				m.app.MoveQueueDown(m.queueCursor)
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
				_ = m.app.PlayAtIndex(m.queueCursor)
				return m, nil
			}
			// Other Queue-focused keys (volume, search) fall
			// through to the global handler below.
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help = !m.help
		case key.Matches(msg, m.keys.Search):
			return m, m.enterSearchMode()
		case key.Matches(msg, m.keys.Clear):
			m.clearFilter()
		case key.Matches(msg, m.keys.Up):
			m.app.SelectPrevScoped()
		case key.Matches(msg, m.keys.Down):
			m.app.SelectNextScoped()
		case key.Matches(msg, m.keys.Play):
			_ = m.app.PlaySelected()
		case key.Matches(msg, m.keys.Pause):
			_ = m.app.TogglePlay()
		case key.Matches(msg, m.keys.Next):
			_ = m.app.Next()
		case key.Matches(msg, m.keys.Prev):
			_ = m.app.Previous()
		case key.Matches(msg, m.keys.VolUp):
			_ = m.app.VolumeUp()
		case key.Matches(msg, m.keys.VolDown):
			_ = m.app.VolumeDown()
		case key.Matches(msg, m.keys.Rescan):
			_ = m.app.Rescan()
		case key.Matches(msg, m.keys.Shuffle):
			m.app.ToggleShuffle()
		case key.Matches(msg, m.keys.Repeat):
			m.app.CycleRepeat()
		case key.Matches(msg, m.keys.Mute):
			m.app.ToggleMute()
		case key.Matches(msg, m.keys.SeekFwd):
			m.seekRelative(SeekStep)
		case key.Matches(msg, m.keys.SeekBwd):
			m.seekRelative(-SeekStep)
		case key.Matches(msg, m.keys.SeekHome):
			_ = m.app.Seek(0)
		case key.Matches(msg, m.keys.SeekEnd):
			_ = m.app.Seek(m.app.Duration())
		case msg.Type == tea.KeyEsc:
			// No-op in search mode (handled above). Out of search
			// mode, Esc clears the filter for symmetry with vim.
			m.clearFilter()
		}

	case tickMsg:
		// Auto-advance to the next track when the current one ends
		// naturally. Has no effect while the user is paused or has
		// manually stopped.
		_ = m.app.MaybeAdvance()
		return m, tickCmd()
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
		m.focus.Set(PanelTracks)
	case libRowAlbum:
		m.app.SetScope(row.Artist, row.Album)
		m.focus.Set(PanelTracks)
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

// View implements tea.Model. It renders either the help overlay or
// the main 3-panel layout followed by the status bar.
func (m *Model) View() string {
	if m.help {
		return m.helpView()
	}

	width := m.width
	height := m.height
	if width <= 0 {
		width = 120
	}
	if height <= 0 {
		height = 40
	}

	layout := Compute(width, height, m.uiCfg, m.styles)
	if layout.TooSmall {
		return m.tooSmallView(layout)
	}

	// Attach the panels' content.
	layout.Library.Content = m.libraryPanelContent
	layout.Tracks.Content = m.tracksPanelContent
	layout.Queue.Content = m.queuePanelContent
	// Reflect the focused state on the panels.
	focused := m.focus.Current()
	layout.Library.Focused = focused == PanelLibrary
	layout.Tracks.Focused = focused == PanelTracks
	layout.Queue.Focused = focused == PanelQueue

	body := lipgloss.JoinHorizontal(
		lipgloss.Top,
		layout.Library.View(),
		layout.Tracks.View(),
		layout.Queue.View(),
	)

	status := m.statusView(layout.Width)
	return body + "\n" + status
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
		return m.styles.EmptyMsg.Width(w).Height(h).Render("No library.\nPress r to scan.")
	}
	rows := libraryPanel(tree, m.libExpanded)
	if len(rows) == 0 {
		return m.styles.EmptyMsg.Width(w).Height(h).Render("Library empty.")
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
		// Append a track count for the user to see the album size.
		count := ""
		if row.Kind == libRowAlbum {
			if al := findAlbum(tree, row.Artist, row.Album); al != nil {
				count = fmt.Sprintf("  (%d)", al.TrackCount)
			}
		}
		text := fmt.Sprintf("%s%s %s%s", indent, marker, label, count)
		text = truncate(text, w)
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
	var sb strings.Builder

	listH := h
	if m.searchMode {
		listH--
	}
	if m.app.Error() != nil {
		listH--
	}
	if listH < 1 {
		listH = 1
	}

	if listH > 0 {
		sb.WriteString(m.listViewSized(w, listH))
	}

	if m.searchMode {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(m.search.View())
	}
	if err := m.app.Error(); err != nil {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(m.styles.ErrorMsg.Render(fmt.Sprintf("error: %v", err)))
	}
	return sb.String()
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
		msg := "No tracks in this scope."
		artist, album := m.app.Scope()
		if artist != "" && album != "" {
			msg = fmt.Sprintf("No tracks in %s/%s.", artist, album)
		} else if artist != "" {
			msg = fmt.Sprintf("No tracks by %s.", artist)
		}
		if m.app.HasFilter() {
			msg = fmt.Sprintf("No tracks match %q", m.app.Filter())
		} else if m.app.Tree() == nil || m.app.Tree().Len() == 0 {
			msg = "No tracks. Set music_dir in your config or place MP3/WAV/FLAC files in ~/Music."
		}
		if w <= 0 || h <= 0 {
			return ""
		}
		return m.styles.EmptyMsg.Width(w).Height(h).Render(msg)
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
		rows = append(rows, m.renderRow(visible[i], i == selected, visible[i].Path == curPath, w))
	}
	return strings.Join(rows, "\n")
}

// renderRow formats a single list row, with different styles for the
// selected and currently playing rows. Long titles are truncated to
// fit the given column width.
func (m *Model) renderRow(t library.Track, selected, playing bool, w int) string {
	marker := "  "
	switch {
	case selected && playing:
		marker = "▶▶"
	case selected:
		marker = "▶ "
	case playing:
		marker = "♪ "
	}

	text := fmt.Sprintf("%s %s", marker, truncate(t.Title, maxTitleLen(w)))

	switch {
	case selected:
		return m.styles.SelectedRow.Render(text)
	case playing:
		return m.styles.PlayingRow.Render(text)
	default:
		return m.styles.Row.Render(text)
	}
}

// statusView renders the now-playing / volume / status line. The
// filter indicator is shown right after the volume when a filter is
// active, so the user always sees whether they are looking at a
// subset of the library.
func (m *Model) statusView(width int) string {
	cur := m.app.Current()
	state := m.app.State()
	vol := m.app.Volume()

	var title string
	if cur != nil {
		title = cur.Title
	} else {
		title = "—"
	}
	var stateStr string
	switch state {
	case player.StatePlaying:
		stateStr = "▶ playing"
	case player.StatePaused:
		stateStr = "⏸ paused"
	default:
		stateStr = "■ stopped"
	}

	durStr := formatDurationOrUnknown(m.app.Duration(), m.app.HasDuration())
	progress := renderProgressBar(m.app.Position(), m.app.Duration(), 24)
	progressStr := fmt.Sprintf("%s  %s / %s",
		progress,
		formatDuration(m.app.Position()),
		durStr,
	)

	keyS := m.styles.StatusKey
	valS := m.styles.StatusVal
	bar := m.styles.StatusBar

	parts := []string{
		keyS.Render("♪ ") + valS.Render(title),
		keyS.Render(" ") + valS.Render(stateStr),
		valS.Render(progressStr),
		keyS.Render("vol ") + valS.Render(fmt.Sprintf("%d%%", vol)),
	}
	if m.app.HasFilter() {
		parts = append(parts, valS.Render(fmt.Sprintf(
			"[filter: %s %d/%d]",
			m.app.Filter(), m.app.VisibleLen(), m.app.Queue().Len(),
		)))
	}
	if m.app.Muted() {
		parts = append(parts, valS.Render("[mute]"))
	}
	if m.app.Shuffle() {
		parts = append(parts, valS.Render("[shuffle]"))
	}
	if r := m.app.Repeat(); r != app.RepeatOff {
		parts = append(parts, valS.Render(fmt.Sprintf("[repeat: %s]", r)))
	}
	parts = append(parts, keyS.Render("· ")+valS.Render(m.app.Status()))

	if width <= 0 {
		width = 120
	}
	return bar.Width(width).Render(strings.Join(parts, "  "))
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
	_ = m.app.Seek(target)
}

// helpView renders the keybindings help screen.
func (m *Model) helpView() string {
	header := m.styles.HelpHeader.Render("Keybindings")
	section := func(title string, items []string) []string {
		out := []string{m.styles.PanelTitle.Render(title)}
		out = append(out, items...)
		out = append(out, "")
		return out
	}
	row := func(b key.Binding) string {
		return m.styles.HelpEntry.Render(fmt.Sprintf("  %-7s  %s",
			b.Help().Key, b.Help().Desc))
	}
	nav := []string{
		row(m.keys.Up),
		row(m.keys.Down),
		"  Tab       cycle panel focus",
		"  S-Tab     cycle panel focus back",
	}
	playback := []string{
		row(m.keys.Play),
		row(m.keys.Pause),
		row(m.keys.Next),
		row(m.keys.Prev),
		row(m.keys.SeekHome),
		row(m.keys.SeekEnd),
		"  ←/→       ±5s (Tracks panel) or expand/collapse (Library panel)",
	}
	volume := []string{
		row(m.keys.VolUp),
		row(m.keys.VolDown),
		row(m.keys.Mute),
	}
	library := []string{
		row(m.keys.Rescan),
		row(m.keys.Shuffle),
		row(m.keys.Repeat),
		row(m.keys.Search),
		row(m.keys.Clear),
		row(m.keys.Help),
		row(m.keys.Quit),
	}
	queue := []string{
		row(m.keys.QueueUp),
		row(m.keys.QueueDown),
		row(m.keys.QueueDel),
		row(m.keys.QueueClear),
		"  (only when the Queue panel is focused)",
	}

	lines := []string{header}
	lines = append(lines, section("Navigation", nav)...)
	lines = append(lines, section("Playback", playback)...)
	lines = append(lines, section("Volume", volume)...)
	lines = append(lines, section("Queue (panel focused)", queue)...)
	lines = append(lines, section("Library & Misc", library)...)
	lines = append(lines, "press ? to close")
	return strings.Join(lines, "\n")
}

// maxTitleLen returns the maximum number of runes a row title can
// take given the panel's inner width. Two runes are reserved for
// the row marker and one for the space between the marker and the
// title.
func maxTitleLen(w int) int {
	if w <= 3 {
		return 0
	}
	return w - 3
}

// truncate returns s shortened to at most maxRunes runes. An
// ellipsis (…) is appended when truncation actually happens.
// Surrogate pairs and combining marks are handled at rune
// granularity, which is good enough for the panel title: a track
// title is a short human-readable string, not a long-form text.
func truncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(runes[:maxRunes-1]) + "…"
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
