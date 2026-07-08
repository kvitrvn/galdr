package oto

import (
	"encoding/binary"
	"io"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/frame"
)

// flacSource decodes a FLAC stream into signed 16-bit little-endian PCM.
//
// FLAC samples are read frame by frame; each frame provides per-channel
// int32 samples that we interleave and convert to int16 LE on the fly.
// Higher-than-16-bit source sample sizes are scaled down to int16 range.
type flacSource struct {
	stream        *flac.Stream
	closer        io.Closer
	sampleRate    int
	channels      int
	bitsPerSample int
	totalSamples  int64

	pending []byte
}

func newFLACSource(r io.Reader) (*flacSource, error) {
	stream, err := flac.New(r)
	if err != nil {
		return nil, err
	}
	info := stream.Info
	closer, _ := r.(io.Closer)
	return &flacSource{
		stream:        stream,
		closer:        closer,
		sampleRate:    int(info.SampleRate),
		channels:      int(info.NChannels),
		bitsPerSample: int(info.BitsPerSample),
		totalSamples:  int64(info.NSamples),
	}, nil
}

func (s *flacSource) ReadPCM(p []byte) (int, error) {
	for len(s.pending) < len(p) {
		f, err := s.stream.ParseNext()
		if err != nil {
			if err == io.EOF {
				if len(s.pending) == 0 {
					return 0, io.EOF
				}
				break
			}
			return 0, err
		}
		s.appendFrame(f)
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	return n, nil
}

// appendFrame interleaves the per-channel int32 samples of one frame
// and appends them to s.pending as int16 LE bytes.
func (s *flacSource) appendFrame(f *frame.Frame) {
	if f == nil || len(f.Subframes) == 0 {
		return
	}
	nSamples := len(f.Subframes[0].Samples)
	for i := 0; i < nSamples; i++ {
		for c := 0; c < s.channels; c++ {
			if c >= len(f.Subframes) {
				continue
			}
			sample := f.Subframes[c].Samples[i]
			v := clampToInt16(sample, s.bitsPerSample)
			var b [2]byte
			binary.LittleEndian.PutUint16(b[:], uint16(v))
			s.pending = append(s.pending, b[:]...)
		}
	}
}

func clampToInt16(sample int32, bitsPerSample int) int16 {
	var v int32
	switch bitsPerSample {
	case 16:
		v = sample
	case 24:
		v = sample >> 8
	case 32:
		v = sample >> 16
	default:
		v = sample
	}
	if v > 32767 {
		v = 32767
	}
	if v < -32768 {
		v = -32768
	}
	return int16(v)
}

func (s *flacSource) SampleRate() int     { return s.sampleRate }
func (s *flacSource) Channels() int       { return s.channels }
func (s *flacSource) TotalSamples() int64 { return s.totalSamples }
func (s *flacSource) Close() error {
	var err error
	if s.closer != nil {
		err = s.closer.Close()
	}
	return err
}
