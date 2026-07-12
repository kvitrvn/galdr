package mpv

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	libmpv "github.com/gen2brain/go-mpv"

	"github.com/kvitrvn/galdr/internal/player"
)

type fakeClient struct {
	mu sync.Mutex

	options      map[string]string
	properties   map[string]any
	propertyErrs map[string]error
	commands     [][]string
	events       chan event

	optionErr     error
	initializeErr error
	commandErr    error
	loadEvent     *event
	loadEvents    []event
	initialized   bool
	destroyed     int
	terminated    int
}

func newFakeClient() *fakeClient {
	return &fakeClient{
		options:      make(map[string]string),
		properties:   make(map[string]any),
		propertyErrs: make(map[string]error),
		events:       make(chan event, 16),
	}
}

func (f *fakeClient) SetOptionString(name, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.optionErr != nil {
		return f.optionErr
	}
	f.options[name] = value
	return nil
}

func (f *fakeClient) Initialize() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.initializeErr != nil {
		return f.initializeErr
	}
	f.initialized = true
	return nil
}

func (f *fakeClient) Command(command []string) error {
	f.mu.Lock()
	f.commands = append(f.commands, append([]string(nil), command...))
	err := f.commandErr
	var loadEvents []event
	if len(command) > 0 && command[0] == "loadfile" {
		if len(f.loadEvents) > 0 {
			loadEvents = append([]event(nil), f.loadEvents...)
		} else if f.loadEvent != nil {
			loadEvents = append(loadEvents, *f.loadEvent)
		}
	}
	f.mu.Unlock()
	if err == nil {
		for _, ev := range loadEvents {
			f.events <- ev
		}
	}
	return err
}

func (f *fakeClient) SetProperty(name string, _ libmpv.Format, value any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.propertyErrs[name]; err != nil {
		return err
	}
	f.properties[name] = value
	return nil
}

func (f *fakeClient) GetProperty(name string, _ libmpv.Format) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.propertyErrs[name]; err != nil {
		return nil, err
	}
	value, ok := f.properties[name]
	if !ok {
		return nil, libmpv.ErrPropertyUnavailable
	}
	return value, nil
}

func (f *fakeClient) WaitEvent(timeout float64) event {
	select {
	case ev := <-f.events:
		return ev
	case <-time.After(time.Duration(timeout * float64(time.Second))):
		return event{id: libmpv.EventNone}
	}
}

func (f *fakeClient) Wakeup() {
	select {
	case f.events <- event{id: libmpv.EventNone}:
	default:
	}
}

func (f *fakeClient) Destroy() {
	f.mu.Lock()
	f.destroyed++
	f.mu.Unlock()
}

func (f *fakeClient) TerminateDestroy() {
	f.mu.Lock()
	f.terminated++
	f.mu.Unlock()
}

func testPlayer(t *testing.T, f *fakeClient) *Player {
	t.Helper()
	p, err := newPlayer(f, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("newPlayer: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

func audioFile(t *testing.T, extension string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "track-*"+extension)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}

func loadTrack(t *testing.T, p *Player, f *fakeClient) string {
	t.Helper()
	f.loadEvent = &event{id: libmpv.EventFileLoaded}
	path := audioFile(t, ".mp3")
	if err := p.Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
	return path
}

func TestNewPlayer_ConfiguresAndCloses(t *testing.T) {
	f := newFakeClient()
	p := testPlayer(t, f)

	wantOptions := map[string]string{
		"config": "no", "terminal": "no", "input-default-bindings": "no",
		"input-vo-keyboard": "no", "osc": "no", "vid": "no",
		"audio-display": "no", "idle": "yes", "pause": "yes",
	}
	if !reflect.DeepEqual(f.options, wantOptions) {
		t.Errorf("options = %#v, want %#v", f.options, wantOptions)
	}
	if !f.initialized {
		t.Error("client was not initialized")
	}
	if got := p.Volume(); got != 100 {
		t.Errorf("Volume = %d, want 100", got)
	}
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State = %v, want stopped", got)
	}

	p.Close()
	p.Close()
	if f.terminated != 1 {
		t.Errorf("TerminateDestroy calls = %d, want 1", f.terminated)
	}
}

func TestNewPlayer_InitializationFailuresReleaseClient(t *testing.T) {
	tests := []struct {
		name           string
		configure      func(*fakeClient)
		wantDestroyed  int
		wantTerminated int
	}{
		{
			name:          "option",
			configure:     func(f *fakeClient) { f.optionErr = errors.New("bad option") },
			wantDestroyed: 1,
		},
		{
			name:          "initialize",
			configure:     func(f *fakeClient) { f.initializeErr = errors.New("init failed") },
			wantDestroyed: 1,
		},
		{
			name:           "initial volume",
			configure:      func(f *fakeClient) { f.propertyErrs["volume"] = errors.New("volume failed") },
			wantTerminated: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeClient()
			tt.configure(f)
			if _, err := newPlayer(f, time.Second); err == nil {
				t.Fatal("newPlayer error = nil")
			}
			if f.destroyed != tt.wantDestroyed || f.terminated != tt.wantTerminated {
				t.Errorf(
					"cleanup = destroy %d terminate %d, want %d %d",
					f.destroyed,
					f.terminated,
					tt.wantDestroyed,
					tt.wantTerminated,
				)
			}
		})
	}
}

func TestPlayer_LoadValidation(t *testing.T) {
	f := newFakeClient()
	p := testPlayer(t, f)

	tests := []struct {
		name string
		path string
	}{
		{name: "unsupported extension", path: audioFile(t, ".ogg")},
		{name: "missing file", path: t.TempDir() + "/missing.mp3"},
		{name: "directory", path: filepath.Join(t.TempDir(), "folder.mp3")},
	}
	if err := os.Mkdir(tests[2].path, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.Load(tt.path); err == nil {
				t.Fatal("Load error = nil")
			}
		})
	}
}

func TestPlayer_LoadOutcomes(t *testing.T) {
	tests := []struct {
		name      string
		loadEvent *event
		wantError bool
	}{
		{name: "loaded", loadEvent: &event{id: libmpv.EventFileLoaded}},
		{
			name: "decode error",
			loadEvent: &event{
				id:        libmpv.EventEnd,
				endReason: libmpv.EndFileError,
				err:       libmpv.ErrUnknownFormat,
			},
			wantError: true,
		},
		{name: "timeout", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeClient()
			f.loadEvent = tt.loadEvent
			p := testPlayer(t, f)
			err := p.Load(audioFile(t, ".flac"))
			if (err != nil) != tt.wantError {
				t.Fatalf("Load error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && p.State() != player.StateStopped {
				t.Errorf("State after failure = %v, want stopped", p.State())
			}
		})
	}
}

func TestPlayer_LoadReplacementIgnoresPreviousEndEvent(t *testing.T) {
	f := newFakeClient()
	p := testPlayer(t, f)
	loadTrack(t, p, f)
	if err := p.Play(); err != nil {
		t.Fatal(err)
	}

	f.mu.Lock()
	f.loadEvent = nil
	f.loadEvents = []event{
		{id: libmpv.EventEnd, endReason: libmpv.EndFileStop},
		{id: libmpv.EventFileLoaded},
	}
	f.mu.Unlock()
	if err := p.Load(audioFile(t, ".wav")); err != nil {
		t.Fatalf("replacement Load: %v", err)
	}
	if got := p.State(); got != player.StateStopped {
		t.Errorf("State after replacement = %v, want stopped", got)
	}
	if p.ConsumeNaturalEnd() {
		t.Error("replacement was reported as natural EOF")
	}
}

func TestPlayer_PlayPauseStopTransitions(t *testing.T) {
	f := newFakeClient()
	p := testPlayer(t, f)

	if err := p.Play(); !errors.Is(err, errNoTrack) {
		t.Fatalf("Play without Load = %v, want errNoTrack", err)
	}
	loadTrack(t, p, f)
	if err := p.Play(); err != nil || p.State() != player.StatePlaying {
		t.Fatalf("Play = %v, state %v", err, p.State())
	}
	if err := p.Pause(); err != nil || p.State() != player.StatePaused {
		t.Fatalf("Pause = %v, state %v", err, p.State())
	}
	if err := p.Stop(); err != nil || p.State() != player.StateStopped {
		t.Fatalf("Stop = %v, state %v", err, p.State())
	}
	if p.Position() != 0 || p.Duration() != 0 {
		t.Error("Stop did not clear position and duration")
	}
}

func TestPlayer_EndEvents(t *testing.T) {
	tests := []struct {
		name   string
		reason libmpv.Reason
		want   player.State
	}{
		{name: "natural eof stops", reason: libmpv.EndFileEOF, want: player.StateStopped},
		{name: "manual stop event ignored", reason: libmpv.EndFileStop, want: player.StatePlaying},
		{name: "replacement event ignored", reason: libmpv.EndFileRedirect, want: player.StatePlaying},
		{name: "decode error stops", reason: libmpv.EndFileError, want: player.StateStopped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeClient()
			p := testPlayer(t, f)
			loadTrack(t, p, f)
			if err := p.Play(); err != nil {
				t.Fatal(err)
			}
			handled := make(chan struct{})
			f.events <- event{id: libmpv.EventEnd, endReason: tt.reason, err: libmpv.ErrLoadingFailed, handled: handled}
			select {
			case <-handled:
			case <-time.After(time.Second):
				t.Fatal("event was not handled")
			}
			if got := p.State(); got != tt.want {
				t.Errorf("State = %v, want %v", got, tt.want)
			}
			wantNatural := tt.reason == libmpv.EndFileEOF
			if got := p.ConsumeNaturalEnd(); got != wantNatural {
				t.Errorf("ConsumeNaturalEnd = %v, want %v", got, wantNatural)
			}
			if p.ConsumeNaturalEnd() {
				t.Error("ConsumeNaturalEnd reported the same EOF twice")
			}
		})
	}
}

func TestPlayer_VolumeAndSeekClamping(t *testing.T) {
	f := newFakeClient()
	p := testPlayer(t, f)
	loadTrack(t, p, f)
	f.properties["duration"] = 10.0

	tests := []struct {
		input int
		want  int
	}{
		{input: -20, want: 0},
		{input: 42, want: 42},
		{input: 120, want: 100},
	}
	for _, test := range tests {
		if err := p.SetVolume(test.input); err != nil {
			t.Fatal(err)
		}
		if got := p.Volume(); got != test.want {
			t.Errorf("SetVolume(%d) = %d, want %d", test.input, got, test.want)
		}
	}
	if err := p.Seek(-time.Second); err != nil {
		t.Fatal(err)
	}
	if err := p.Seek(12 * time.Second); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	commands := append([][]string(nil), f.commands...)
	f.mu.Unlock()
	if got := commands[len(commands)-2]; !reflect.DeepEqual(got, []string{"seek", "0.000000", "absolute+exact"}) {
		t.Errorf("negative seek command = %#v", got)
	}
	if got := commands[len(commands)-1]; !reflect.DeepEqual(got, []string{"seek", "10.000000", "absolute+exact"}) {
		t.Errorf("clamped seek command = %#v", got)
	}
}

func TestPlayer_PositionDurationUnavailable(t *testing.T) {
	f := newFakeClient()
	p := testPlayer(t, f)
	loadTrack(t, p, f)

	f.properties["time-pos"] = 1.25
	f.properties["duration"] = "wrong type"
	if got := p.Position(); got != 1250*time.Millisecond {
		t.Errorf("Position = %v, want 1.25s", got)
	}
	if got := p.Duration(); got != 0 {
		t.Errorf("Duration wrong type = %v, want 0", got)
	}
	f.propertyErrs["time-pos"] = libmpv.ErrPropertyUnavailable
	if got := p.Position(); got != 0 {
		t.Errorf("Position unavailable = %v, want 0", got)
	}
}

func TestPlayer_ConcurrentCommandsAndEvents(t *testing.T) {
	f := newFakeClient()
	p := testPlayer(t, f)
	loadTrack(t, p, f)
	f.properties["duration"] = 30.0
	f.properties["time-pos"] = 1.0

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = p.SetVolume(50)
			_ = p.Position()
			_ = p.Seek(time.Second)
		}()
		go func() {
			defer wg.Done()
			f.events <- event{id: libmpv.EventEnd, endReason: libmpv.EndFileStop}
			_ = p.State()
		}()
	}
	wg.Wait()
}
