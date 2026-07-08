// Package tui contains the Bubble Tea models, views and keybindings.
//
// The TUI depends on the player interface from internal/player (through
// internal/app) and never on a concrete audio backend.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/library"
	"github.com/kvitrvn/galdr/internal/player"
	"github.com/kvitrvn/galdr/internal/theme"
)

// keyMap defines every keybinding exposed by the TUI.
type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Next    key.Binding
	Prev    key.Binding
	Play    key.Binding
	Pause   key.Binding
	VolUp   key.Binding
	VolDown key.Binding
	Help    key.Binding
	Quit    key.Binding
}

// defaultKeys returns the MVP keybindings:
//
//	↑/k         move selection up
//	↓/j         move selection down
//	n           next track
//	p           previous track
//	enter       play selected track (toggles if already playing)
//	space       toggle play / pause
//	+/=         volume up
//	-/_         volume down
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
	app    *app.App
	keys   keyMap
	styles theme.Palette

	width  int
	height int
	help   bool
}

// New constructs a TUI model backed by a, using palette for styling.
// Callers typically pass theme.PaletteFor(string(cfg.Theme)).
func New(a *app.App, palette theme.Palette) *Model {
	return &Model{
		app:    a,
		keys:   defaultKeys(),
		styles: palette,
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
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.help = !m.help
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
		}

	case tickMsg:
		return m, tickCmd()
	}

	return m, nil
}

// View implements tea.Model. It renders either the main view or the
// help overlay depending on m.help.
func (m *Model) View() string {
	if m.help {
		return m.helpView()
	}
	return m.mainView()
}

// mainView is the primary rendering of the TUI.
//
// Layout:
//
//	[title]
//	[track list]
//	[divider]
//	[status bar]
func (m *Model) mainView() string {
	var sb strings.Builder

	sb.WriteString(m.styles.Title.Render("galdr"))
	sb.WriteString("\n")

	sb.WriteString(m.listView())

	if err := m.app.Error(); err != nil {
		sb.WriteString("\n")
		sb.WriteString(m.styles.ErrorMsg.Render(fmt.Sprintf("error: %v", err)))
	}

	sb.WriteString("\n")
	sb.WriteString(m.styles.Divider.Render(strings.Repeat("─", maxWidth(m.width, 40))))
	sb.WriteString("\n")
	sb.WriteString(m.statusView())

	return sb.String()
}

// listView renders the track list. If the queue is empty, an empty-state
// message is rendered instead.
func (m *Model) listView() string {
	q := m.app.Queue()
	if q.Len() == 0 {
		return m.styles.EmptyMsg.Render(
			"No tracks. Set music_dir in your config or place MP3/WAV/FLAC files in ~/Music.")
	}

	listHeight := m.listHeight()
	start := scrollStart(q.Len(), q.Index(), listHeight)

	cur := m.app.Current()
	curPath := ""
	if cur != nil {
		curPath = cur.Path
	}

	tracks := q.Tracks()
	selected := q.Index()
	var rows []string
	for i := start; i < q.Len() && i < start+listHeight; i++ {
		rows = append(rows, m.renderRow(tracks[i], i == selected, tracks[i].Path == curPath))
	}
	return strings.Join(rows, "\n")
}

// renderRow formats a single list row, with different styles for the
// selected and currently playing rows.
func (m *Model) renderRow(t library.Track, selected, playing bool) string {
	marker := "  "
	switch {
	case selected && playing:
		marker = "▶▶"
	case selected:
		marker = "▶ "
	case playing:
		marker = "♪ "
	}

	text := fmt.Sprintf("%s %s", marker, t.Title)

	switch {
	case selected:
		return m.styles.SelectedRow.Render(text)
	case playing:
		return m.styles.PlayingRow.Render(text)
	default:
		return m.styles.Row.Render(text)
	}
}

// statusView renders the now-playing / volume / status line.
func (m *Model) statusView() string {
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

	progress := renderProgressBar(m.app.Position(), m.app.Duration(), 24)
	progressStr := fmt.Sprintf("%s  %s / %s",
		progress,
		formatDuration(m.app.Position()),
		formatDuration(m.app.Duration()),
	)

	keyS := m.styles.StatusKey
	valS := m.styles.StatusVal
	bar := m.styles.StatusBar

	parts := []string{
		keyS.Render("♪ ") + valS.Render(title),
		keyS.Render(" ") + valS.Render(stateStr),
		valS.Render(progressStr),
		keyS.Render("vol ") + valS.Render(fmt.Sprintf("%d%%", vol)),
		keyS.Render("· ") + valS.Render(m.app.Status()),
	}

	return bar.Width(maxWidth(m.width, 40)).Render(strings.Join(parts, "  "))
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
		row(m.keys.VolUp),
		row(m.keys.VolDown),
		row(m.keys.Help),
		row(m.keys.Quit),
		"",
		"press ? to close",
	}
	return strings.Join(lines, "\n")
}

// listHeight returns the number of rows to render in the track list,
// accounting for the title, status bar, divider and error line.
func (m *Model) listHeight() int {
	const chrome = 6
	if m.height <= 0 {
		return 10
	}
	h := m.height - chrome
	if h < 3 {
		return 3
	}
	return h
}

// maxWidth returns w if positive, otherwise fallback.
func maxWidth(w, fallback int) int {
	if w > 0 {
		return w
	}
	return fallback
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
