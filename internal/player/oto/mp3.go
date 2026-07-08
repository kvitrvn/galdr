package oto

import (
	"io"

	"github.com/hajimehoshi/go-mp3"
)

// mp3Source wraps a go-mp3 decoder as a pcmSource.
//
// go-mp3 always outputs stereo 16-bit signed little-endian PCM, so the
// channel count returned by Channels() is fixed at 2.
type mp3Source struct {
	dec    *mp3.Decoder
	closer io.Closer
}

func newMP3Source(r io.Reader) (*mp3Source, error) {
	dec, err := mp3.NewDecoder(r)
	if err != nil {
		return nil, err
	}
	closer, _ := r.(io.Closer)
	return &mp3Source{dec: dec, closer: closer}, nil
}

func (s *mp3Source) ReadPCM(p []byte) (int, error) {
	return s.dec.Read(p)
}

func (s *mp3Source) SampleRate() int {
	return s.dec.SampleRate()
}

func (s *mp3Source) Channels() int {
	return 2
}

func (s *mp3Source) TotalSamples() int64 {
	// dec.Length() returns the total number of PCM output bytes.
	// Stereo 16-bit = 4 bytes per frame.
	l := s.dec.Length()
	if l < 0 {
		return 0
	}
	return l / 4
}

func (s *mp3Source) Close() error {
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}
