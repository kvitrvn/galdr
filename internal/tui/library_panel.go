package tui

import "github.com/kvitrvn/galdr/internal/library"

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
