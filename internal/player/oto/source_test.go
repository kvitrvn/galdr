package oto

import (
	"encoding/binary"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
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

func TestFLACSource_OpenAndRead(t *testing.T) {
	src, err := newFLACSource(openFixture(t, "sample.flac"))
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
