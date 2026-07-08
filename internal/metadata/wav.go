package metadata

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// readWAV extracts tags and duration from a WAV file (RIFF/WAVE).
//
// The parser walks RIFF chunks in a single pass. The `fmt ` chunk
// provides the byte rate used to compute the duration. The `data`
// chunk's size is the numerator. The `LIST` chunk of type `INFO`
// carries the standard RIFF INFO tags (INAM, IART, IPRD, ICRD,
// ITRK).
//
// Audio frames are not decoded.
//
// WAV files with embedded ID3 chunks (rare) are not supported in v1.
func readWAV(path string) (Tags, error) {
	f, err := openForRead(path)
	if err != nil {
		return Tags{}, err
	}
	defer f.Close()

	// RIFF header: "RIFF" + size(4 LE) + "WAVE"
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return Tags{Format: "wav"}, fmt.Errorf("wav: %w", err)
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return Tags{Format: "wav"}, nil
	}

	var (
		byteRate uint32
		dataSize uint32
		info     = make(map[string]string)
		gotFmt   bool
		gotData  bool
	)

	for {
		var chunkHdr [8]byte
		if _, err := io.ReadFull(f, chunkHdr[:]); err != nil {
			break
		}
		id := string(chunkHdr[0:4])
		size := binary.LittleEndian.Uint32(chunkHdr[4:8])
		body := io.LimitReader(f, int64(size))
		// Chunks are word-aligned: pad byte if size is odd.
		padding := int64(0)
		if size%2 == 1 {
			padding = 1
		}

		switch id {
		case "fmt ":
			gotFmt = parseFmtChunk(body, &byteRate) || gotFmt
		case "data":
			gotData = true
			dataSize = size
		case "LIST":
			parseListInfoChunk(body, info)
		}
		if _, err := io.Copy(io.Discard, body); err != nil {
			break
		}
		// Discard the padding byte, if any.
		if padding > 0 {
			if _, err := f.Seek(padding, io.SeekCurrent); err != nil {
				break
			}
		}
	}

	out := Tags{Format: "wav"}
	if in := info["INAM"]; in != "" {
		out.Title = in
	}
	if in := info["IART"]; in != "" {
		out.Artist = in
	}
	if in := info["IPRD"]; in != "" {
		out.Album = in
	}
	if in := info["ICRD"]; in != "" {
		if n, ok := parseLeadingInt(in); ok {
			out.Year = n
		}
	}
	if in := info["ITRK"]; in != "" {
		if n, ok := parseLeadingInt(in); ok {
			out.Track = n
		}
	}
	if gotFmt && gotData && byteRate > 0 {
		out.Duration = time.Duration(dataSize) * time.Second / time.Duration(byteRate)
	}
	return out, nil
}

// parseFmtChunk reads the format fields of a `fmt ` chunk and
// extracts the byte rate. It returns true if the chunk was a
// recognised format (PCM, IEEE float, or EXTENSIBLE wrapping either).
//
// The body layout (little-endian) is:
//
//	AudioFormat(2) NumChannels(2) SampleRate(4) ByteRate(4)
//	BlockAlign(2)  BitsPerSample(2)
//
// followed by an extension when AudioFormat is 0xFFFE
// (WAVE_FORMAT_EXTENSIBLE):
//
//	cbSize(2) wValidBitsPerSample(2) dwChannelMask(4) SubFormat(16)
//
// We only need ByteRate, which lives in the common header and is
// therefore identical across the three formats we accept.
func parseFmtChunk(r io.Reader, byteRate *uint32) bool {
	var hdr [16]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return false
	}
	audioFormat := binary.LittleEndian.Uint16(hdr[0:2])
	*byteRate = binary.LittleEndian.Uint32(hdr[8:12])
	switch audioFormat {
	case 0x0001, 0x0003:
		return true
	case 0xFFFE:
		var ext [24]byte
		if _, err := io.ReadFull(r, ext[:]); err != nil {
			return false
		}
		subCode := binary.LittleEndian.Uint32(ext[8:12])
		return subCode == 0x00000001 || subCode == 0x00000003
	}
	return false
}

// parseListInfoChunk reads a LIST chunk and extracts INFO sub-chunks
// into out. The LIST chunk body starts with a 4-byte list-type
// identifier ("INFO") followed by sub-chunks of the same shape as
// top-level RIFF chunks: id(4) + size(4 LE) + body.
func parseListInfoChunk(r io.Reader, out map[string]string) {
	var listType [4]byte
	if _, err := io.ReadFull(r, listType[:]); err != nil {
		return
	}
	if string(listType[:]) != "INFO" {
		return
	}
	for {
		var subHdr [8]byte
		if _, err := io.ReadFull(r, subHdr[:]); err != nil {
			return
		}
		id := string(subHdr[0:4])
		size := binary.LittleEndian.Uint32(subHdr[4:8])
		body := make([]byte, size)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		// INFO values are null-terminated ASCII.
		s := string(body)
		if idx := indexByte(s, 0); idx >= 0 {
			s = s[:idx]
		}
		out[id] = s
		// Sub-chunks are also word-aligned.
		if size%2 == 1 {
			var pad [1]byte
			if _, err := io.ReadFull(r, pad[:]); err != nil {
				return
			}
		}
	}
}

// indexByte is the standard strings.IndexByte behaviour but inlined
// here to keep the import set minimal in the metadata package.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
