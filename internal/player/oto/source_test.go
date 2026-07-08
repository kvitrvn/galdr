package oto

import (
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/kvitrvn/galdr/internal/metadatatest"
)

// writeWAV creates a small valid PCM WAV file in dir and returns its path.
//
// Format: mono, 16-bit signed LE PCM, 8 kHz, 1 second of a 440 Hz sine tone.
func writeWAV(t *testing.T, dir string) string {
	t.Helper()

	const (
		sampleRate    = 8000
		channels      = 1
		bitsPerSample = 16
	)
	numSamples := sampleRate // 1 second
	dataSize := numSamples * (bitsPerSample / 8)

	path := filepath.Join(dir, "test.wav")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create wav: %v", err)
	}
	defer f.Close()

	// RIFF header.
	if _, err := f.Write([]byte("RIFF")); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("WAVE")); err != nil {
		t.Fatal(err)
	}

	// fmt chunk (16 bytes).
	if _, err := f.Write([]byte("fmt ")); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(16)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(1)); err != nil {
		t.Fatal(err) // PCM
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(channels)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(sampleRate*channels*bitsPerSample/8)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(channels*bitsPerSample/8)); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		t.Fatal(err)
	}

	// data chunk header.
	if _, err := f.Write([]byte("data")); err != nil {
		t.Fatal(err)
	}
	if err := binary.Write(f, binary.LittleEndian, uint32(dataSize)); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < numSamples; i++ {
		v := int16(32767 * 0.5 * math.Sin(2*math.Pi*440*float64(i)/float64(sampleRate)))
		if err := binary.Write(f, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestWAVSource_ParseAndRead(t *testing.T) {
	path := writeWAV(t, t.TempDir())

	src, err := newWAVSource(path)
	if err != nil {
		t.Fatalf("newWAVSource: %v", err)
	}
	defer src.Close()

	if got := src.SampleRate(); got != 8000 {
		t.Errorf("SampleRate = %d, want 8000", got)
	}
	if got := src.Channels(); got != 1 {
		t.Errorf("Channels = %d, want 1", got)
	}
	if got := src.TotalSamples(); got != 8000 {
		t.Errorf("TotalSamples = %d, want 8000", got)
	}

	buf := make([]byte, 4096)
	total := 0
	for {
		n, err := src.ReadPCM(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadPCM: %v", err)
		}
	}
	wantBytes := 8000 * 2 // 1 channel * 2 bytes per sample
	if total != wantBytes {
		t.Errorf("total bytes read = %d, want %d", total, wantBytes)
	}
}

func TestWAVSource_NotRIFF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bogus.wav")
	if err := os.WriteFile(path, []byte("not a riff file at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newWAVSource(path); err == nil {
		t.Error("expected error for non-RIFF file, got nil")
	}
}

func TestWAVSource_MissingFile(t *testing.T) {
	if _, err := newWAVSource(filepath.Join(t.TempDir(), "absent.wav")); err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestWAVSource_Seek(t *testing.T) {
	src, err := newWAVSource(writeWAV(t, t.TempDir()))
	if err != nil {
		t.Fatalf("newWAVSource: %v", err)
	}
	defer src.Close()

	// Half a second in.
	const halfSec = int64(8000 / 2)
	if err := src.SeekSample(halfSec); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	buf := make([]byte, 4)
	n, err := src.ReadPCM(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("ReadPCM: %v", err)
	}
	if n == 0 {
		t.Fatal("ReadPCM after Seek returned 0 bytes")
	}
	// Seek past the end: should clamp to totalSamples and yield EOF.
	if err := src.SeekSample(src.TotalSamples() + 1000); err != nil {
		t.Fatalf("Seek past end: %v", err)
	}
	_, err = src.ReadPCM(buf)
	if err != io.EOF {
		t.Errorf("ReadPCM after clamp: err = %v, want io.EOF", err)
	}
	// Seek to negative: should clamp to 0.
	if err := src.SeekSample(-100); err != nil {
		t.Fatalf("Seek negative: %v", err)
	}
}

func TestFLACSource_OpenAndRead(t *testing.T) {
	src, err := newFLACSource(fixturePath(t, "sample.flac"))
	if err != nil {
		t.Fatalf("newFLACSource: %v", err)
	}
	defer src.Close()

	if got := src.SampleRate(); got == 0 {
		t.Error("SampleRate = 0, want > 0")
	}
	if got := src.Channels(); got == 0 {
		t.Error("Channels = 0, want > 0")
	}
	if got := src.TotalSamples(); got <= 0 {
		t.Errorf("TotalSamples = %d, want > 0", got)
	}

	buf := make([]byte, 4096)
	total := 0
	for {
		n, err := src.ReadPCM(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("ReadPCM: %v", err)
		}
	}
	if total == 0 {
		t.Error("ReadPCM produced 0 bytes")
	}
	// Total bytes should match NSamples * channels * 2.
	wantBytes := int(src.TotalSamples()) * src.Channels() * 2
	if total != wantBytes {
		t.Errorf("total bytes = %d, want %d", total, wantBytes)
	}
}

func TestFLACSource_Seek(t *testing.T) {
	src, err := newFLACSource(fixturePath(t, "sample.flac"))
	if err != nil {
		t.Fatalf("newFLACSource: %v", err)
	}
	defer src.Close()

	total := src.TotalSamples()
	if total <= 100 {
		t.Skipf("fixture too small to seek (total=%d)", total)
	}
	// Seek near the end and read.
	if err := src.SeekSample(total - 100); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	buf := make([]byte, 256)
	n, err := src.ReadPCM(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("ReadPCM after Seek: %v", err)
	}
	if n == 0 {
		t.Error("ReadPCM after Seek returned 0 bytes")
	}
	// Clamp to total.
	if err := src.SeekSample(total + 1000); err != nil {
		t.Fatalf("Seek past end: %v", err)
	}
	// Negative clamps to 0.
	if err := src.SeekSample(-50); err != nil {
		t.Fatalf("Seek negative: %v", err)
	}
}

func TestMP3Source_Seek_NoLoad(t *testing.T) {
	// Sanity: newMP3Source returns an error for a missing file.
	dir := t.TempDir()
	missing := filepath.Join(dir, "absent.mp3")
	// newMP3Source takes a *os.File, so we have to create a zero-byte
	// file. The MP3 decoder should reject it.
	if err := os.WriteFile(missing, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(missing)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := newMP3Source(f); err == nil {
		t.Error("expected error for empty MP3, got nil")
	}
}

func TestMP3Source_SeekAndRead(t *testing.T) {
	dir := t.TempDir()
	mp3Path := filepath.Join(dir, "test.mp3")
	if err := os.WriteFile(mp3Path, buildSilentMP3(t), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(mp3Path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	src, err := newMP3Source(f)
	if err != nil {
		t.Fatalf("newMP3Source: %v", err)
	}
	defer src.Close()

	total := src.TotalSamples()
	if total == 0 {
		t.Skip("go-mp3 reports zero total (likely VBR), skipping seek test")
	}
	// Seek to 10 samples forward and read.
	if err := src.SeekSample(10); err != nil {
		t.Fatalf("Seek(10): %v", err)
	}
	buf := make([]byte, 64)
	if _, err := src.ReadPCM(buf); err != nil && err != io.EOF {
		t.Fatalf("ReadPCM: %v", err)
	}
	// Seek to 0 and read.
	if err := src.SeekSample(0); err != nil {
		t.Fatalf("Seek(0): %v", err)
	}
	if _, err := src.ReadPCM(buf); err != nil && err != io.EOF {
		t.Fatalf("ReadPCM at 0: %v", err)
	}
	// Clamp to total.
	if err := src.SeekSample(total + 1000); err != nil {
		t.Fatalf("Seek past end: %v", err)
	}
}

// TestWAVSource_24BitPCM ensures the source decodes 24-bit integer
// PCM correctly. The fixture is generated by metadatatest, so the
// real-world shape matches what ffmpeg / Audacity / Reaper produce.
func TestWAVSource_24BitPCM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pcm24.wav")
	// 1.0s of mono 24-bit 44.1kHz = 44100 frames * 3 bytes = 132300 bytes.
	metadatatest.WriteWAV24PCM(t, path, "", "", "", 0, 0, 132300)

	src, err := newWAVSource(path)
	if err != nil {
		t.Fatalf("newWAVSource: %v", err)
	}
	defer src.Close()

	if got := src.SampleRate(); got != 44100 {
		t.Errorf("SampleRate = %d, want 44100", got)
	}
	if got := src.Channels(); got != 1 {
		t.Errorf("Channels = %d, want 1", got)
	}
	if got := src.TotalSamples(); got != 44100 {
		t.Errorf("TotalSamples = %d, want 44100", got)
	}

	// Read 1 frame; expect 2 bytes (one int16 sample) of PCM output.
	buf := make([]byte, 2)
	n, err := src.ReadPCM(buf)
	if err != nil {
		t.Fatalf("ReadPCM: %v", err)
	}
	if n != 2 {
		t.Errorf("ReadPCM n = %d, want 2", n)
	}
}

// TestWAVSource_Extensible24PCM is the user-reported case: a file
// produced by Audacity / Reaper with WAVE_FORMAT_EXTENSIBLE wrapping
// 24-bit PCM. The bug was the source rejecting it with
// "only PCM (format 1) is supported, got 65534".
func TestWAVSource_Extensible24PCM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ext.wav")
	metadatatest.WriteWAVExtensible24PCM(t, path, "", "", "", 0, 0, 132300)

	src, err := newWAVSource(path)
	if err != nil {
		t.Fatalf("newWAVSource: %v", err)
	}
	defer src.Close()

	if got := src.SampleRate(); got != 44100 {
		t.Errorf("SampleRate = %d, want 44100", got)
	}
	if got := src.TotalSamples(); got != 44100 {
		t.Errorf("TotalSamples = %d, want 44100", got)
	}

	buf := make([]byte, 64)
	if _, err := src.ReadPCM(buf); err != nil {
		t.Fatalf("ReadPCM: %v", err)
	}
}

// TestWAVSource_Float32 ensures 32-bit IEEE float WAV files are read.
func TestWAVSource_Float32(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f32.wav")
	// 1.0s of mono 32-bit float 44.1kHz = 44100 * 4 = 176400 bytes.
	metadatatest.WriteWAVFloat32(t, path, "", "", "", 0, 0, 176400)

	src, err := newWAVSource(path)
	if err != nil {
		t.Fatalf("newWAVSource: %v", err)
	}
	defer src.Close()

	if got := src.SampleRate(); got != 44100 {
		t.Errorf("SampleRate = %d, want 44100", got)
	}
	if got := src.Channels(); got != 1 {
		t.Errorf("Channels = %d, want 1", got)
	}
	if got := src.TotalSamples(); got != 44100 {
		t.Errorf("TotalSamples = %d, want 44100", got)
	}

	buf := make([]byte, 64)
	if _, err := src.ReadPCM(buf); err != nil {
		t.Fatalf("ReadPCM: %v", err)
	}
}

// TestWAVSource_Extensible24PCM_SeekAndRead combines the two
// features that previously broke together: WAVE_FORMAT_EXTENSIBLE +
// Seek.
func TestWAVSource_Extensible24PCM_SeekAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ext.wav")
	metadatatest.WriteWAVExtensible24PCM(t, path, "", "", "", 0, 0, 132300)

	src, err := newWAVSource(path)
	if err != nil {
		t.Fatalf("newWAVSource: %v", err)
	}
	defer src.Close()

	if err := src.SeekSample(1000); err != nil {
		t.Fatalf("SeekSample: %v", err)
	}
	buf := make([]byte, 64)
	if _, err := src.ReadPCM(buf); err != nil {
		t.Fatalf("ReadPCM after Seek: %v", err)
	}
}

func TestPositionFromBytes(t *testing.T) {
	cr := &countingReader{bytesRead: 0}
	if got := positionFromBytes(cr, 44100, 2); got != 0 {
		t.Errorf("zero bytes -> %v, want 0", got)
	}
	cr.bytesRead = 44100 * 2 * 2 // 1 second of stereo 16-bit
	if got := positionFromBytes(cr, 44100, 2); got != 1_000_000_000 {
		t.Errorf("1s worth of bytes -> %v, want 1s", got)
	}
	if got := positionFromBytes(nil, 44100, 2); got != 0 {
		t.Errorf("nil reader -> %v, want 0", got)
	}
}

func TestClampToInt16(t *testing.T) {
	cases := []struct {
		name string
		in   int32
		bps  int
		want int16
	}{
		{"16bit-zero", 0, 16, 0},
		{"16bit-positive", 12345, 16, 12345},
		{"16bit-negative", -12345, 16, -12345},
		{"24bit-scaled-positive", 0x00123400, 24, 0x1234},
		{"24bit-scaled-negative", -0x00123400, 24, -0x1234},
		{"24bit-clamp-positive", 0x00800000, 24, 32767},
		{"24bit-clamp-negative", -0x00800000, 24, -32768},
		{"32bit-scaled", 0x00010000, 32, 1},
		{"clamp-positive", 100000, 8, 32767},
		{"clamp-negative", -100000, 8, -32768},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampToInt16(c.in, c.bps); got != c.want {
				t.Errorf("clampToInt16(%d, %d) = %d, want %d", c.in, c.bps, got, c.want)
			}
		})
	}
}

func TestOpenSource_UnsupportedFormat(t *testing.T) {
	if _, err := openSource("ogg", "/tmp/x.ogg"); err == nil {
		t.Error("expected error for unsupported format, got nil")
	}
}

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Skipf("fixture %s not available: %v", name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// fixturePath returns the absolute path of a file in the package's
// testdata directory. Tests that need to re-open a source for
// seeking (e.g. FLAC, MP3) use this so they can pass a path instead
// of an already-open *os.File.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

// buildSilentMP3 returns the bytes of a minimal CBR MP3 file: a
// no-tag stream made of a single silent MPEG1 Layer III frame at
// 128 kbps, 44.1 kHz, mono. go-mp3 reports a deterministic total
// sample count for this fixture, which is what Seek tests need.
func buildSilentMP3(t *testing.T) []byte {
	t.Helper()
	// 1 silent MPEG1 Layer III frame: 417 bytes total (4-byte header
	// + 413 zero bytes of payload). At 44.1 kHz this frame is 1152
	// samples long, so multiple frames are needed to get a non-zero
	// read after seek. Use 10 frames.
	const frames = 10
	frame := make([]byte, 4+413)
	frame[0] = 0xFF
	frame[1] = 0xFB // MPEG1, Layer III, no CRC
	frame[2] = 0x90 // 128 kbps, 44.1 kHz, no padding, no private
	frame[3] = 0xC0 // mono
	out := make([]byte, 0, frames*len(frame))
	for i := 0; i < frames; i++ {
		out = append(out, frame...)
	}
	return out
}
