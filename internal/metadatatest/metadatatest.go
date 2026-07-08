// Package metadatatest contains test-only helpers for generating small
// valid MP3, FLAC and WAV files with known tags. It is internal to
// the galdr project so other packages' tests can build fixtures
// without duplicating the binary writers.
//
// The writers are deterministic and small. Audio payload is omitted
// where the tag parser does not need it (FLAC, MP3 tags) and is a
// short block of zeros otherwise (WAV, MP3 audio frame).
package metadatatest

import (
	"encoding/binary"
	"os"
)

// Tags carries the tag set to embed in a generated fixture. Empty
// fields are skipped. DurationSample is the number of audio samples
// to encode in FLAC STREAMINFO; DurationBytes is the number of audio
// bytes to encode in the WAV data chunk.
type Tags struct {
	Title          string
	Artist         string
	Album          string
	Year           int
	Track          int
	DurationSample int64 // FLAC NSamples
	DurationBytes  int   // WAV data chunk size
	Extra          map[string]string
}

// WriteMP3 writes a minimal MP3 file with an ID3v2.3 tag and a
// single silent MPEG1 Layer III frame.
func WriteMP3(t TestingT, path, title, artist, album string, year, track int) {
	t.Helper()
	writeMP3WithID3v2(t, path, Tags{
		Title:  title,
		Artist: artist,
		Album:  album,
		Year:   year,
		Track:  track,
	})
}

// WriteFLAC writes a minimal FLAC file with STREAMINFO and a
// VORBIS_COMMENT block. The audio frames are empty.
func WriteFLAC(t TestingT, path, title, artist, album string, year, track int, nSamples int64) {
	t.Helper()
	writeFLACWithVorbis(t, path, Tags{
		Title:          title,
		Artist:         artist,
		Album:          album,
		Year:           year,
		Track:          track,
		DurationSample: nSamples,
	})
}

// WriteWAV writes a minimal 16-bit mono 44.1 kHz PCM WAV file with a
// LIST/INFO chunk carrying the standard RIFF INFO tags.
func WriteWAV(t TestingT, path, title, artist, album string, year, track int, dataBytes int) {
	t.Helper()
	writeWAVWithINFO(t, path, Tags{
		Title:         title,
		Artist:        artist,
		Album:         album,
		Year:          year,
		Track:         track,
		DurationBytes: dataBytes,
	})
}

// WriteMP3NoTags writes a minimal MP3 file with no ID3 tag and a
// single silent frame.
func WriteMP3NoTags(t TestingT, path string) {
	t.Helper()
	frame := make([]byte, 4+417)
	frame[0] = 0xFF
	frame[1] = 0xFB
	frame[2] = 0x90
	frame[3] = 0xC0
	if err := os.WriteFile(path, frame, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

// TestingT is the subset of *testing.T we need to fail tests. The
// metadata tests use *testing.T directly; this interface keeps the
// helper importable from other test files.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...any)
}

// writeID3v2TextFrame appends an ID3v2.3 text frame to buf and
// returns the extended slice. text is encoded as ISO-8859-1
// (encoding byte 0x00).
func writeID3v2TextFrame(buf []byte, frameID, text string) []byte {
	body := append([]byte{0x00}, []byte(text)...)
	hdr := make([]byte, 10)
	copy(hdr[0:4], frameID)
	binary.BigEndian.PutUint32(hdr[4:8], uint32(len(body)))
	buf = append(buf, hdr...)
	buf = append(buf, body...)
	return buf
}

func writeMP3WithID3v2(t TestingT, path string, tags Tags) {
	header := []byte{'I', 'D', '3', 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	sizeOffset := len(header) - 4

	body := []byte{}
	if tags.Title != "" {
		body = writeID3v2TextFrame(body, "TIT2", tags.Title)
	}
	if tags.Artist != "" {
		body = writeID3v2TextFrame(body, "TPE1", tags.Artist)
	}
	if tags.Album != "" {
		body = writeID3v2TextFrame(body, "TALB", tags.Album)
	}
	if tags.Year != 0 {
		body = writeID3v2TextFrame(body, "TYER", itoa(tags.Year))
	}
	if tags.Track != 0 {
		body = writeID3v2TextFrame(body, "TRCK", itoa(tags.Track))
	}
	writeSynchsafe32(header[sizeOffset:], uint32(len(body)))

	// Silent MPEG1 Layer III frame: 128 kbps, 44.1 kHz, mono.
	frame := make([]byte, 4+417)
	frame[0] = 0xFF
	frame[1] = 0xFB
	frame[2] = 0x90
	frame[3] = 0xC0

	out := append(header, body...)
	out = append(out, frame...)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func writeFLACWithVorbis(t TestingT, path string, tags Tags) {
	var buf []byte
	buf = append(buf, []byte("fLaC")...)

	// STREAMINFO (type 0). 34 bytes body: 144 bits + 16 MD5 bytes.
	si := make([]byte, 34)
	bw := &bitWriter{buf: si}
	bw.WriteBits(4096, 16)
	bw.WriteBits(4096, 16)
	bw.WriteBits(0, 24)
	bw.WriteBits(0, 24)
	bw.WriteBits(44100, 20)
	bw.WriteBits(0, 3)  // channels - 1 (mono)
	bw.WriteBits(15, 5) // bits per sample - 1 (16)
	bw.WriteBits(uint64(tags.DurationSample), 36)
	writeFLACBlock(&buf, 0, false, si)

	// VORBIS_COMMENT (type 4). Body: vendor + num + entries + framing.
	vendor := "galdr-test"
	vc := []byte{}
	vendorLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(vendorLen, uint32(len(vendor)))
	vc = append(vc, vendorLen...)
	vc = append(vc, []byte(vendor)...)

	entries := [][2]string{}
	if tags.Title != "" {
		entries = append(entries, [2]string{"TITLE", tags.Title})
	}
	if tags.Artist != "" {
		entries = append(entries, [2]string{"ARTIST", tags.Artist})
	}
	if tags.Album != "" {
		entries = append(entries, [2]string{"ALBUM", tags.Album})
	}
	if tags.Year != 0 {
		entries = append(entries, [2]string{"DATE", itoa(tags.Year)})
	}
	if tags.Track != 0 {
		entries = append(entries, [2]string{"TRACKNUMBER", itoa(tags.Track)})
	}
	for k, v := range tags.Extra {
		entries = append(entries, [2]string{k, v})
	}
	numComments := make([]byte, 4)
	binary.LittleEndian.PutUint32(numComments, uint32(len(entries)))
	vc = append(vc, numComments...)
	for _, e := range entries {
		entry := e[0] + "=" + e[1]
		entryLen := make([]byte, 4)
		binary.LittleEndian.PutUint32(entryLen, uint32(len(entry)))
		vc = append(vc, entryLen...)
		vc = append(vc, []byte(entry)...)
	}
	vc = append(vc, 0x01) // framing bit
	writeFLACBlock(&buf, 4, true, vc)

	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func writeWAVWithINFO(t TestingT, path string, tags Tags) {
	const (
		sampleRate    = uint32(44100)
		channels      = uint16(1)
		bitsPerSample = uint16(16)
	)
	byteRate := sampleRate * uint32(channels) * uint32(bitsPerSample) / 8
	blockAlign := channels * (bitsPerSample / 8)

	listEntries := [][2]string{}
	if tags.Title != "" {
		listEntries = append(listEntries, [2]string{"INAM", tags.Title})
	}
	if tags.Artist != "" {
		listEntries = append(listEntries, [2]string{"IART", tags.Artist})
	}
	if tags.Album != "" {
		listEntries = append(listEntries, [2]string{"IPRD", tags.Album})
	}
	if tags.Year != 0 {
		listEntries = append(listEntries, [2]string{"ICRD", itoa(tags.Year)})
	}
	if tags.Track != 0 {
		listEntries = append(listEntries, [2]string{"ITRK", itoa(tags.Track)})
	}
	listBody := []byte("INFO")
	for _, e := range listEntries {
		hdr := make([]byte, 8)
		copy(hdr[0:4], e[0])
		body := append([]byte(e[1]), 0x00)
		binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(body)))
		listBody = append(listBody, hdr...)
		listBody = append(listBody, body...)
		if len(body)%2 == 1 {
			listBody = append(listBody, 0x00)
		}
	}

	fmtBody := make([]byte, 16)
	binary.LittleEndian.PutUint16(fmtBody[0:2], 1)
	binary.LittleEndian.PutUint16(fmtBody[2:4], channels)
	binary.LittleEndian.PutUint32(fmtBody[4:8], sampleRate)
	binary.LittleEndian.PutUint32(fmtBody[8:12], byteRate)
	binary.LittleEndian.PutUint16(fmtBody[12:14], blockAlign)
	binary.LittleEndian.PutUint16(fmtBody[14:16], bitsPerSample)

	dataSize := uint32(tags.DurationBytes)
	dataBody := make([]byte, dataSize)

	out := []byte("RIFF")
	riffSizeOffset := len(out)
	out = append(out, 0, 0, 0, 0)
	out = append(out, []byte("WAVE")...)

	out = append(out, []byte("fmt ")...)
	fmtSize := make([]byte, 4)
	binary.LittleEndian.PutUint32(fmtSize, uint32(len(fmtBody)))
	out = append(out, fmtSize...)
	out = append(out, fmtBody...)

	out = append(out, []byte("LIST")...)
	listSize := make([]byte, 4)
	binary.LittleEndian.PutUint32(listSize, uint32(len(listBody)))
	out = append(out, listSize...)
	out = append(out, listBody...)

	out = append(out, []byte("data")...)
	dataSizeBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(dataSizeBuf, uint32(len(dataBody)))
	out = append(out, dataSizeBuf...)
	out = append(out, dataBody...)

	binary.LittleEndian.PutUint32(out[riffSizeOffset:riffSizeOffset+4], uint32(len(out)-8))

	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func writeFLACBlock(buf *[]byte, blockType byte, isLast bool, body []byte) {
	header := byte(blockType & 0x7F)
	if isLast {
		header |= 0x80
	}
	*buf = append(*buf, header)
	*buf = append(*buf,
		byte((len(body)>>16)&0xFF),
		byte((len(body)>>8)&0xFF),
		byte(len(body)&0xFF),
	)
	*buf = append(*buf, body...)
}

type bitWriter struct {
	buf    []byte
	bitPos int
}

func (w *bitWriter) WriteBits(n uint64, bits int) {
	for i := bits - 1; i >= 0; i-- {
		bit := byte((n >> i) & 1)
		byteIdx := w.bitPos / 8
		bitIdx := 7 - (w.bitPos % 8)
		for byteIdx >= len(w.buf) {
			w.buf = append(w.buf, 0)
		}
		w.buf[byteIdx] |= bit << bitIdx
		w.bitPos++
	}
}

func writeSynchsafe32(b []byte, n uint32) {
	b[0] = byte((n >> 21) & 0x7F)
	b[1] = byte((n >> 14) & 0x7F)
	b[2] = byte((n >> 7) & 0x7F)
	b[3] = byte(n & 0x7F)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
