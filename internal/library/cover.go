package library

import (
	"os"
	"path/filepath"
)

var albumCoverNames = [...]string{"cover.jpg", "cover.jpeg", "cover.png"}

// FindAlbumCover returns the first supported cover file next to trackPath.
// Missing and non-regular files are ignored.
func FindAlbumCover(trackPath string) string {
	dir := filepath.Dir(trackPath)
	for _, name := range albumCoverNames {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() {
			return path
		}
	}
	return ""
}
