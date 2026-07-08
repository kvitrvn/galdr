package oto

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// wavSource reads a PCM-coded RIFF/WAVE file and yields its PCM samples
// directly. For MVP, only 16-bit samples in mono or stereo are supported.
type wavSource struct {
	f            *os.File
	sampleRate   int
	channels     int
	totalSamples int64
	bytesRead    int64
}

func newWAVSource(path string) (*wavSource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	var riff [12]byte
	if _, err := io.ReadFull(f, riff[:]); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("wav: read RIFF header: %w", err)
	}
	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		_ = f.Close()
		return nil, fmt.Errorf("wav: not a RIFF/WAVE file")
	}

	var (
		sampleRate    int
		channels      int
		bitsPerSample int
		dataSize      int64
		dataFound     bool
	)

	for {
		var hdr [8]byte
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("wav: read chunk header: %w", err)
		}
		id := string(hdr[0:4])
		size := int64(binary.LittleEndian.Uint32(hdr[4:8]))

		switch id {
		case "fmt ":
			if size < 16 {
				_ = f.Close()
				return nil, fmt.Errorf("wav: fmt chunk too small (%d)", size)
			}
			var fmtChunk [16]byte
			if _, err := io.ReadFull(f, fmtChunk[:]); err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("wav: read fmt: %w", err)
			}
			audioFormat := binary.LittleEndian.Uint16(fmtChunk[0:2])
			if audioFormat != 1 {
				_ = f.Close()
				return nil, fmt.Errorf("wav: only PCM (format 1) is supported, got %d", audioFormat)
			}
			channels = int(binary.LittleEndian.Uint16(fmtChunk[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(fmtChunk[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(fmtChunk[14:16]))
			// Skip any extra bytes in the fmt chunk.
			if size > 16 {
				if _, err := f.Seek(size-16, io.SeekCurrent); err != nil {
					_ = f.Close()
					return nil, err
				}
			}
		case "data":
			if bitsPerSample != 16 {
				_ = f.Close()
				return nil, fmt.Errorf("wav: only 16-bit samples supported, got %d", bitsPerSample)
			}
			if channels < 1 || channels > 2 {
				_ = f.Close()
				return nil, fmt.Errorf("wav: unsupported channel count %d", channels)
			}
			if sampleRate <= 0 {
				_ = f.Close()
				return nil, fmt.Errorf("wav: invalid sample rate %d", sampleRate)
			}
			dataSize = size
			dataFound = true
		default:
			if _, err := f.Seek(size, io.SeekCurrent); err != nil {
				_ = f.Close()
				return nil, err
			}
		}

		if dataFound {
			break
		}
	}

	if !dataFound {
		_ = f.Close()
		return nil, fmt.Errorf("wav: data chunk not found")
	}

	bytesPerFrame := channels * 2
	return &wavSource{
		f:            f,
		sampleRate:   sampleRate,
		channels:     channels,
		totalSamples: dataSize / int64(bytesPerFrame),
	}, nil
}

func (s *wavSource) ReadPCM(p []byte) (int, error) {
	n, err := s.f.Read(p)
	s.bytesRead += int64(n)
	if err != nil && err != io.EOF {
		return n, err
	}
	if err == io.EOF && n == 0 {
		return 0, io.EOF
	}
	return n, err
}

func (s *wavSource) SampleRate() int { return s.sampleRate }
func (s *wavSource) Channels() int   { return s.channels }
func (s *wavSource) TotalSamples() int64 {
	return s.totalSamples
}
func (s *wavSource) Close() error { return s.f.Close() }
