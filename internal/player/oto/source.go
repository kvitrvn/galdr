package oto

import (
	"fmt"
	"os"

	"github.com/kvitrvn/galdr/internal/library"
)

// pcmSource produces signed 16-bit little-endian PCM samples on demand.
// All methods must be safe to call sequentially from a single goroutine.
type pcmSource interface {
	ReadPCM(p []byte) (int, error)
	SampleRate() int
	Channels() int
	// TotalSamples is the total number of inter-channel samples
	// (i.e. frames, not individual channel samples) in the source.
	TotalSamples() int64
	Close() error
}

// openSource constructs a pcmSource for the given format and path.
func openSource(format library.Format, path string) (pcmSource, error) {
	switch format {
	case library.FormatMP3:
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		s, err := newMP3Source(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		return s, nil
	case library.FormatWAV:
		return newWAVSource(path)
	case library.FormatFLAC:
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		s, err := newFLACSource(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("oto: unsupported format %q", format)
	}
}
