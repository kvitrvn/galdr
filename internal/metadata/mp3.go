package metadata

import (
	"fmt"

	"github.com/dhowden/tag"
)

// readMP3 extracts tags from an MP3 file using dhowden/tag.
//
// The dhowden/tag library auto-detects ID3v1, ID3v2.2, ID3v2.3 and
// ID3v2.4 from the first few bytes of the file. If none is present
// it returns ErrNoTagsFound; we swallow that and return a zero Tags
// with a nil error, letting the caller fall back to the filename.
//
// Duration is left at zero; see the package documentation for the
// rationale.
func readMP3(path string) (Tags, error) {
	f, err := openForRead(path)
	if err != nil {
		return Tags{}, err
	}
	defer f.Close()

	meta, err := tag.ReadFrom(f)
	if err != nil {
		if err == tag.ErrNoTagsFound {
			return Tags{Format: "mp3"}, nil
		}
		return Tags{Format: "mp3"}, fmt.Errorf("mp3: %w", err)
	}
	track, _ := meta.Track()
	return Tags{
		Title:  meta.Title(),
		Artist: meta.Artist(),
		Album:  meta.Album(),
		Year:   meta.Year(),
		Track:  track,
		Format: "mp3",
	}, nil
}
