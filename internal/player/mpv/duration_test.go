package mpv

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	libmpv "github.com/gen2brain/go-mpv"
)

func testDurationProber(t *testing.T, f *fakeClient, timeout time.Duration) *DurationProber {
	t.Helper()
	p, err := newDurationProber(f, timeout)
	if err != nil {
		t.Fatalf("newDurationProber: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

func TestNewDurationProber_ConfiguresIsolatedSilentClient(t *testing.T) {
	f := newFakeClient()
	p := testDurationProber(t, f, time.Second)
	want := map[string]string{
		"config": "no", "terminal": "no", "input-default-bindings": "no",
		"input-vo-keyboard": "no", "osc": "no", "vid": "no",
		"audio-display": "no", "idle": "yes", "pause": "yes", "ao": "null",
	}
	if !reflect.DeepEqual(f.options, want) {
		t.Fatalf("options = %#v, want %#v", f.options, want)
	}
	if !f.initialized {
		t.Fatal("client was not initialized")
	}
	p.Close()
	p.Close()
	if f.terminated != 1 {
		t.Fatalf("TerminateDestroy calls = %d, want 1", f.terminated)
	}
}

func TestNewDurationProber_InitializationFailuresReleaseClient(t *testing.T) {
	for _, tt := range []struct {
		name      string
		configure func(*fakeClient)
	}{
		{name: "option", configure: func(f *fakeClient) { f.optionErr = errors.New("bad option") }},
		{name: "initialize", configure: func(f *fakeClient) { f.initializeErr = errors.New("bad init") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeClient()
			tt.configure(f)
			if _, err := newDurationProber(f, time.Second); err == nil {
				t.Fatal("newDurationProber error = nil")
			}
			if f.destroyed != 1 {
				t.Fatalf("Destroy calls = %d, want 1", f.destroyed)
			}
		})
	}
}

func TestDurationProber_ValidDurationAndCleanup(t *testing.T) {
	f := newFakeClient()
	f.properties["duration"] = 125.25
	f.loadEvent = &event{id: libmpv.EventFileLoaded}
	p := testDurationProber(t, f, time.Second)
	path := audioFile(t, ".mp3")

	got, err := p.ProbeDuration(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2*time.Minute+5250*time.Millisecond {
		t.Fatalf("duration = %v, want 2m5.25s", got)
	}
	wantCommands := [][]string{{"loadfile", path, "replace"}, {"stop"}}
	if !reflect.DeepEqual(f.commands, wantCommands) {
		t.Fatalf("commands = %#v, want %#v", f.commands, wantCommands)
	}
}

func TestDurationProber_ValidatesLocalSupportedFiles(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "folder.flac")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		path string
	}{
		{name: "unsupported", path: audioFile(t, ".ogg")},
		{name: "missing", path: filepath.Join(t.TempDir(), "missing.wav")},
		{name: "directory", path: dirPath},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeClient()
			p := testDurationProber(t, f, time.Second)
			if _, err := p.ProbeDuration(context.Background(), tt.path); err == nil {
				t.Fatal("ProbeDuration error = nil")
			}
			if len(f.commands) != 0 {
				t.Fatalf("validation issued commands: %#v", f.commands)
			}
		})
	}
}

func TestDurationProber_ProbeFailures(t *testing.T) {
	for _, tt := range []struct {
		name        string
		configure   func(*fakeClient)
		wantError   error
		wantMessage string
	}{
		{
			name: "decode error",
			configure: func(f *fakeClient) {
				f.loadEvent = &event{id: libmpv.EventEnd, endReason: libmpv.EndFileError, err: libmpv.ErrUnknownFormat}
			},
			wantMessage: "decode",
		},
		{
			name: "property unavailable",
			configure: func(f *fakeClient) {
				f.loadEvent = &event{id: libmpv.EventFileLoaded}
				f.propertyErrs["duration"] = libmpv.ErrPropertyUnavailable
			},
			wantError: libmpv.ErrPropertyUnavailable,
		},
		{
			name: "invalid property",
			configure: func(f *fakeClient) {
				f.loadEvent = &event{id: libmpv.EventFileLoaded}
				f.properties["duration"] = 0.0
			},
			wantMessage: "invalid duration",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeClient()
			tt.configure(f)
			p := testDurationProber(t, f, time.Second)
			_, err := p.ProbeDuration(context.Background(), audioFile(t, ".flac"))
			if err == nil {
				t.Fatal("ProbeDuration error = nil")
			}
			if tt.wantError != nil && !errors.Is(err, tt.wantError) {
				t.Fatalf("error = %v, want errors.Is %v", err, tt.wantError)
			}
			if tt.wantMessage != "" && !strings.Contains(err.Error(), tt.wantMessage) {
				t.Fatalf("error = %q, want %q", err, tt.wantMessage)
			}
			if got := f.commands[len(f.commands)-1]; !reflect.DeepEqual(got, []string{"stop"}) {
				t.Fatalf("last command = %#v, want stop", got)
			}
		})
	}
}

func TestDurationProber_IgnoresOldReplacementStop(t *testing.T) {
	f := newFakeClient()
	f.properties["duration"] = 42.5
	f.loadEvents = []event{
		{id: libmpv.EventEnd, endReason: libmpv.EndFileStop},
		{id: libmpv.EventFileLoaded},
	}
	p := testDurationProber(t, f, time.Second)
	got, err := p.ProbeDuration(context.Background(), audioFile(t, ".wav"))
	if err != nil || got != 42500*time.Millisecond {
		t.Fatalf("ProbeDuration = %v, %v, want 42.5s, nil", got, err)
	}
}

func TestDurationProber_TimeoutAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		timeout   time.Duration
		contextFn func() (context.Context, context.CancelFunc)
		want      error
	}{
		{
			name:    "timeout",
			timeout: 20 * time.Millisecond,
			contextFn: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
			want: context.DeadlineExceeded,
		},
		{
			name:    "cancelled",
			timeout: time.Second,
			contextFn: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			want: context.Canceled,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeClient()
			p := testDurationProber(t, f, tt.timeout)
			ctx, cancel := tt.contextFn()
			defer cancel()
			_, err := p.ProbeDuration(ctx, audioFile(t, ".mp3"))
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want errors.Is %v", err, tt.want)
			}
		})
	}
}

func TestDurationProber_CloseCancelsActiveProbeAndIsIdempotent(t *testing.T) {
	f := newFakeClient()
	p := testDurationProber(t, f, time.Second)
	done := make(chan error, 1)
	path := audioFile(t, ".mp3")
	go func() {
		_, err := p.ProbeDuration(context.Background(), path)
		done <- err
	}()

	deadline := time.Now().Add(time.Second)
	for {
		f.mu.Lock()
		started := len(f.commands) > 0
		f.mu.Unlock()
		if started {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("probe did not start")
		}
		time.Sleep(time.Millisecond)
	}
	p.Close()
	p.Close()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("active probe error = %v, want context.Canceled", err)
	}
	if f.terminated != 1 {
		t.Fatalf("TerminateDestroy calls = %d, want 1", f.terminated)
	}
	if _, err := p.ProbeDuration(context.Background(), audioFile(t, ".mp3")); err == nil {
		t.Fatal("probe after Close error = nil")
	}
}
