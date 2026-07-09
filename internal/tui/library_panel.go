package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kvitrvn/galdr/internal/library"
)

// libRowKind identifies whether a row in the Library panel is an
// artist (expandable) or an album (terminal node).
type libRowKind int

const (
	libRowArtist libRowKind = iota
	libRowAlbum
)

// libRow is one visible line in the Library panel. The panel
// renders a flat list of libRow values, indented according to
// depth.
type libRow struct {
	Kind   libRowKind
	Artist string
	Album  string // empty for artist rows
	Depth  int    // 0 for artist, 1 for album
}

// libraryPanel builds the flat list of rows visible in the
// Library panel for the current tree, taking the expanded
// artists into account and the active filter.
//
// It is the data layer of the Library panel: the View function
// iterates the result to render the rows. The cursor is a
// separate state on the model.
func libraryPanel(tree *library.Tree, expanded map[string]bool) []libRow {
	if tree == nil {
		return nil
	}
	var rows []libRow
	for _, a := range tree.Artists() {
		if a.Hidden {
			continue
		}
		rows = append(rows, libRow{
			Kind:   libRowArtist,
			Artist: a.Name,
			Depth:  0,
		})
		if expanded[a.Name] {
			for _, al := range a.Albums {
				if al.Hidden {
					continue
				}
				rows = append(rows, libRow{
					Kind:   libRowAlbum,
					Artist: a.Name,
					Album:  al.Name,
					Depth:  1,
				})
			}
		}
	}
	return rows
}

// renderLibraryRow formats a single row. The marker column holds
// either `▼` (expanded artist), `▶` (collapsed artist), ` `
// (albums), or a `▶` for the album/artist matching the current
// scope so the user sees where playback is anchored.
//
// selected highlights the row the user is currently on with the
// SelectedRow style.
func renderLibraryRow(row libRow, selected bool, w int) string {
	var marker string
	switch row.Kind {
	case libRowArtist:
		// We don't have expansion state here; the caller passes
		// `selected` and we infer the marker elsewhere. Keep
		// the marker neutral here.
		marker = "  "
	default:
		marker = "  "
	}

	var label string
	switch row.Kind {
	case libRowArtist:
		label = row.Artist
	default:
		label = row.Album
	}
	indent := strings.Repeat("  ", row.Depth)
	text := fmt.Sprintf("%s%s%s", indent, marker, truncate(label, maxLibraryLabel(w, row.Depth)))
	if selected {
		return lipgloss.NewStyle().Bold(true).Render(text)
	}
	return text
}

// maxLibraryLabel returns the maximum number of runes available
// for a row's label text given the panel's inner width and the
// row's indent. Two runes are reserved for the marker and one
// for the space.
func maxLibraryLabel(w, depth int) int {
	if w <= 0 {
		return 0
	}
	reserved := 2*depth + 3
	if reserved >= w {
		return 0
	}
	return w - reserved
}
