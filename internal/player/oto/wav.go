package oto

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
)

// wavFormat identifies how a WAV file's audio samples are encoded on
// disk. Both are downsampled to signed 16-bit little-endian PCM before
// being handed to Oto.
type wavFormat int

const (
	// wavFormatPCM is the standard integer PCM encoding. Valid bit
	// depths: 16, 24, 32.
	wavFormatPCM wavFormat = iota
	// wavFormatFloat is IEEE 754 32-bit float in the range [-1, 1].
	// 64-bit float WAVs are not supported.
	wavFormatFloat
)

func (f wavFormat) String() string {
	switch f {
	case wavFormatFloat:
		return "float32"
	default:
		return "pcm"
	}
}

// wavSource reads a RIFF/WAVE file and yields its audio as signed
// 16-bit little-endian PCM, regardless of the source encoding.
//
// Supported source formats:
//
//   - WAVE_FORMAT_PCM (0x0001) at 16, 24 or 32 bits per sample.
//   - WAVE_FORMAT_IEEE_FLOAT (0x0003) at 32 bits per sample.
//   - WAVE_FORMAT_EXTENSIBLE (0xFFFE) wrapping either of the above.
//
// The format and bit depth are read from the fmt chunk. WAVE_FORMAT
// _EXTENSIBLE is resolved by inspecting its SubFormat GUID; the rest
// of the GUID is ignored because Microsoft reuses the same fixed
// prefix across all audio SubFormats.
type wavSource struct {
	f             *os.File
	sampleRate    int
	channels      int
	bitsPerSample int
	format        wavFormat
	totalSamples  int64
	bytesRead     int64

	// inputFrameSize is the number of source bytes for one
	// inter-channel frame (channels * bitsPerSample / 8).
	inputFrameSize int

	// pending holds int16 LE output bytes that have been decoded
	// from the file but not yet copied to a ReadPCM caller.
	pending []byte
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
		format        wavFormat
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
			// Read the 16-byte common header.
			var common [16]byte
			if _, err := io.ReadFull(f, common[:]); err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("wav: read fmt: %w", err)
			}
			formatCode := binary.LittleEndian.Uint16(common[0:2])
			channels = int(binary.LittleEndian.Uint16(common[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(common[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(common[14:16]))

			switch formatCode {
			case 0x0001: // WAVE_FORMAT_PCM
				format = wavFormatPCM
			case 0x0003: // WAVE_FORMAT_IEEE_FLOAT
				format = wavFormatFloat
			case 0xFFFE: // WAVE_FORMAT_EXTENSIBLE
				if size < 40 {
					_ = f.Close()
					return nil, fmt.Errorf("wav: EXTENSIBLE fmt too small (%d), want >= 40", size)
				}
				// Skip cbSize (2) and wValidBitsPerSample (2) and
				// dwChannelMask (4): 8 bytes.
				var ext [8]byte
				if _, err := io.ReadFull(f, ext[:]); err != nil {
					_ = f.Close()
					return nil, fmt.Errorf("wav: read EXTENSIBLE ext: %w", err)
				}
				// SubFormat GUID (16 bytes). The first DWORD is the
				// actual format code; the remaining 12 bytes are the
				// fixed KSDATAFORMAT prefix and are ignored.
				var guid [16]byte
				if _, err := io.ReadFull(f, guid[:]); err != nil {
					_ = f.Close()
					return nil, fmt.Errorf("wav: read SubFormat: %w", err)
				}
				subCode := binary.LittleEndian.Uint32(guid[0:4])
				switch subCode {
				case 0x00000001:
					format = wavFormatPCM
				case 0x00000003:
					format = wavFormatFloat
				default:
					_ = f.Close()
					return nil, fmt.Errorf("wav: unsupported EXTENSIBLE SubFormat 0x%08X", subCode)
				}
			default:
				_ = f.Close()
				return nil, fmt.Errorf("wav: unsupported format code 0x%04X", formatCode)
			}

			// Skip any extra bytes in the fmt chunk beyond what we
			// have already read.
			consumed := int64(16)
			if formatCode == 0xFFFE {
				consumed = 40
			}
			if size > consumed {
				if _, err := f.Seek(size-consumed, io.SeekCurrent); err != nil {
					_ = f.Close()
					return nil, err
				}
			}

		case "data":
			// Validate parameters now that the data chunk is in
			// sight; the fmt chunk must have been seen first.
			if channels < 1 || channels > 2 {
				_ = f.Close()
				return nil, fmt.Errorf("wav: unsupported channel count %d", channels)
			}
			if sampleRate <= 0 {
				_ = f.Close()
				return nil, fmt.Errorf("wav: invalid sample rate %d", sampleRate)
			}
			if !validBits(format, bitsPerSample) {
				_ = f.Close()
				return nil, fmt.Errorf("wav: unsupported %s with %d bits", format, bitsPerSample)
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

	inputFrameSize := channels * bitsPerSample / 8
	return &wavSource{
		f:              f,
		sampleRate:     sampleRate,
		channels:       channels,
		bitsPerSample:  bitsPerSample,
		format:         format,
		totalSamples:   dataSize / int64(inputFrameSize),
		inputFrameSize: inputFrameSize,
	}, nil
}

// validBits reports whether the given bit depth is supported for the
// given source format. 16/24/32-bit integer PCM and 32-bit float are
// the only combinations produced by mainstream tools.
func validBits(f wavFormat, bits int) bool {
	switch f {
	case wavFormatPCM:
		return bits == 16 || bits == 24 || bits == 32
	case wavFormatFloat:
		return bits == 32 || bits == 64
	}
	return false
}

// readFrame reads one inter-channel frame from the source file,
// converts it to int16 LE and appends the result to s.pending.
// Returns io.EOF at end of stream.
func (s *wavSource) readFrame() error {
	raw := make([]byte, s.inputFrameSize)
	n, err := io.ReadFull(s.f, raw)
	if err != nil {
		if err == io.EOF && n == 0 {
			return io.EOF
		}
		// Partial frame: discard (rare; the next read will likely
		// also fail and the consumer will treat it as end-of-track).
		return err
	}
	out := make([]byte, s.channels*2)
	for c := 0; c < s.channels; c++ {
		offset := c * (s.bitsPerSample / 8)
		var sample16 int16
		switch {
		case s.format == wavFormatPCM && s.bitsPerSample == 16:
			v := int16(binary.LittleEndian.Uint16(raw[offset : offset+2]))
			sample16 = v
		case s.format == wavFormatPCM && s.bitsPerSample == 24:
			b0 := uint32(raw[offset])
			b1 := uint32(raw[offset+1])
			b2 := uint32(raw[offset+2])
			v := int32(b0) | int32(b1)<<8 | int32(b2)<<16
			// Sign-extend from 24 bits.
			if v&0x800000 != 0 {
				v |= ^0xFFFFFF
			}
			sample16 = int16(v >> 8)
		case s.format == wavFormatPCM && s.bitsPerSample == 32:
			v := int32(binary.LittleEndian.Uint32(raw[offset : offset+4]))
			sample16 = int16(v >> 16)
		case s.format == wavFormatFloat && s.bitsPerSample == 32:
			bits := binary.LittleEndian.Uint32(raw[offset : offset+4])
			f := math.Float32frombits(bits)
			if f > 1 {
				f = 1
			} else if f < -1 {
				f = -1
			}
			sample16 = int16(f * 32767)
		case s.format == wavFormatFloat && s.bitsPerSample == 64:
			bits := binary.LittleEndian.Uint64(raw[offset : offset+8])
			f := math.Float64frombits(bits)
			if f > 1 {
				f = 1
			} else if f < -1 {
				f = -1
			}
			sample16 = int16(f * 32767)
		default:
			return fmt.Errorf("wav: unsupported conversion %s/%d", s.format, s.bitsPerSample)
		}
		binary.LittleEndian.PutUint16(out[c*2:], uint16(sample16))
	}
	s.pending = append(s.pending, out...)
	return nil
}

func (s *wavSource) ReadPCM(p []byte) (int, error) {
	// Detect "seeked to end" up front: the file pointer is past the
	// last data byte, so any Read would return (0, nil) instead of
	// EOF. Returning EOF here makes the consumer react consistently
	// with the "natural end of track" code path.
	maxBytes := s.totalSamples * int64(s.channels*2)
	if s.bytesRead >= maxBytes {
		return 0, io.EOF
	}
	for len(s.pending) < len(p) {
		if err := s.readFrame(); err != nil {
			if err == io.EOF {
				if len(s.pending) == 0 {
					return 0, io.EOF
				}
				break
			}
			return 0, err
		}
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	s.bytesRead += int64(n)
	return n, nil
}

func (s *wavSource) SampleRate() int { return s.sampleRate }
func (s *wavSource) Channels() int   { return s.channels }
func (s *wavSource) TotalSamples() int64 {
	return s.totalSamples
}

// SeekSample positions the file pointer at the byte offset that
// corresponds to sampleNum, then discards any pending output so the
// next ReadPCM call decodes fresh.
func (s *wavSource) SeekSample(sampleNum int64) error {
	if sampleNum < 0 {
		sampleNum = 0
	}
	if sampleNum > s.totalSamples {
		sampleNum = s.totalSamples
	}
	offset := sampleNum * int64(s.inputFrameSize)
	if _, err := s.f.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	s.bytesRead = sampleNum * int64(s.channels*2)
	s.pending = s.pending[:0]
	return nil
}

func (s *wavSource) Close() error { return s.f.Close() }
