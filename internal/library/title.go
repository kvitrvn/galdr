package library

import (
	"path/filepath"
	"strings"
)

// TitleFromPath derives a readable track title from a file path by stripping
// the directory and the file extension.
//
// It is used as a fallback when real metadata is unavailable so that no
// track is unplayable only because tags are missing.
func TitleFromPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}
