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
	Up       key.Binding
	Down     key.Binding
	Next     key.Binding
	Prev     key.Binding
	Play     key.Binding
	Pause    key.Binding
	VolUp    key.Binding
	VolDown  key.Binding
	Rescan   key.Binding
	Shuffle  key.Binding
	Repeat   key.Binding
	Mute     key.Binding
	SeekFwd  key.Binding
	SeekBwd  key.Binding
	SeekHome key.Binding
	SeekEnd  key.Binding
	Search   key.Binding
	Clear    key.Binding
	Help     key.Binding
	Quit     key.Binding
}

// SeekStep is the time delta applied by left/right seek.
const SeekStep = 5 * time.Second

// defaultKeys returns the v1 keybindings:
//
//	↑/k         move selection up
//	↓/j         move selection down
//	n           next track (shuffle-aware)
//	p           previous track (shuffle-aware)
//	enter       play selected track (toggles if already playing)
//	space       toggle play / pause
//	←/→         seek -5s / +5s
//	home/end    seek to start / end of current track
//	+/=         volume up
//	-/_         volume down
//	m           mute toggle
//	r           rescan the music directory
//	s           shuffle toggle
//	R           repeat cycle (off -> all -> one -> off)
//	/           enter search mode
//	esc         clear filter / exit search
//	ctrl+l      clear filter
//	?           toggle help
//	q           quit
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
	}
}

// Model is the root Bubble Tea model for galdr.
type Model struct {
	app     *app.App
	keys    keyMap
	styles  theme.Palette
	uiCfg   UIConfig
	focused PanelID

	width  int
	height int
	help   bool

	searchMode bool
	search     textinput.Model
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
		app:     a,
		keys:    defaultKeys(),
		styles:  palette,
		uiCfg:   uiCfg,
		focused: PanelTracks,
		search:  ti,
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
			m.app.SelectPrev()
		case key.Matches(msg, m.keys.Down):
			m.app.SelectNext()
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

	// Attach the tracks panel content (the queue + search + error).
	layout.Tracks.Content = m.tracksPanelContent
	// Reflect the focused state on the panels.
	layout.Library.Focused = m.focused == PanelLibrary
	layout.Tracks.Focused = m.focused == PanelTracks
	layout.Queue.Focused = m.focused == PanelQueue

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

// tracksPanelContent renders the contents of the central Tracks
// panel: the visible track list, the search input (when active),
// and the error line (when set). It is called with the panel's
// inner dimensions (W-2, H-2). The list gets whatever vertical
// space remains after the optional search and error lines.
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

// listViewSized renders the (filtered) track list inside a
// rectangle of (w, h) cells. The selection is kept in view via
// scrollStart. When the filter hides every track, an empty-state
// message is rendered instead.
func (m *Model) listViewSized(w, h int) string {
	visible := m.app.VisibleTracks()
	if len(visible) == 0 {
		msg := "No tracks. Set music_dir in your config or place MP3/WAV/FLAC files in ~/Music."
		if m.app.HasFilter() {
			msg = fmt.Sprintf("No tracks match %q", m.app.Filter())
		}
		if w <= 0 || h <= 0 {
			return ""
		}
		return m.styles.EmptyMsg.Width(w).Height(h).Render(msg)
	}
	if h <= 0 {
		return ""
	}

	selected := m.app.DisplayIndex()
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
	row := func(b key.Binding) string {
		return m.styles.HelpEntry.Render(fmt.Sprintf("  %-7s  %s",
			b.Help().Key, b.Help().Desc))
	}
	lines := []string{
		header,
		row(m.keys.Up),
		row(m.keys.Down),
		row(m.keys.Play),
		row(m.keys.Pause),
		row(m.keys.Next),
		row(m.keys.Prev),
		row(m.keys.SeekBwd),
		row(m.keys.SeekFwd),
		row(m.keys.SeekHome),
		row(m.keys.SeekEnd),
		row(m.keys.VolUp),
		row(m.keys.VolDown),
		row(m.keys.Mute),
		row(m.keys.Rescan),
		row(m.keys.Shuffle),
		row(m.keys.Repeat),
		row(m.keys.Search),
		row(m.keys.Clear),
		row(m.keys.Help),
		row(m.keys.Quit),
		"",
		"press ? to close",
	}
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
