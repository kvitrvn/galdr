package oto

import (
	"errors"
	"io"
	"os"

	"github.com/hajimehoshi/go-mp3"
)

// mp3Source wraps a go-mp3 decoder as a pcmSource.
//
// go-mp3 always outputs stereo 16-bit signed little-endian PCM, so the
// channel count returned by Channels() is fixed at 2.
//
// The source keeps a reference to the underlying *os.File so that
// Seek can rewind and re-decode from the start. This is slow (it
// must decode the whole prefix of the file) but correct for VBR.
type mp3Source struct {
	dec     *mp3.Decoder
	decoded int64 // bytes decoded since the last Seek
	file    *os.File
}

func newMP3Source(f *os.File) (*mp3Source, error) {
	dec, err := mp3.NewDecoder(f)
	if err != nil {
		return nil, err
	}
	return &mp3Source{dec: dec, file: f}, nil
}

func (s *mp3Source) ReadPCM(p []byte) (int, error) {
	n, err := s.dec.Read(p)
	s.decoded += int64(n)
	return n, err
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

// Seek positions the source at the given sample number. The
// implementation re-creates the decoder from the start of the file
// and discards samples until the target is reached. This is O(target)
// for both forward and backward seeks, and is VBR-correct.
func (s *mp3Source) SeekSample(sampleNum int64) error {
	if sampleNum < 0 {
		sampleNum = 0
	}
	// Stereo 16-bit = 4 bytes per inter-channel sample.
	bytesToSkip := sampleNum * 4
	if total := s.TotalSamples(); total > 0 {
		max := total * 4
		if bytesToSkip > max {
			bytesToSkip = max
		}
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dec, err := mp3.NewDecoder(s.file)
	if err != nil {
		return err
	}
	s.dec = dec
	s.decoded = 0
	if bytesToSkip == 0 {
		return nil
	}
	// Discard samples. We read in chunks so large seeks are not
	// bottlenecked by a single tiny buffer.
	const chunk = 4096
	var skipped int64
	buf := make([]byte, chunk)
	for skipped < bytesToSkip {
		want := bytesToSkip - skipped
		if want > int64(len(buf)) {
			want = int64(len(buf))
		}
		n, err := s.dec.Read(buf[:want])
		skipped += int64(n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	s.decoded = skipped
	return nil
}

func (s *mp3Source) Close() error {
	return s.file.Close()
}
