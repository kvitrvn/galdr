package mpris

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/config"
	"github.com/kvitrvn/galdr/internal/player"
)

type emittedSignal struct {
	name   string
	values []any
}

type fakeBus struct {
	mu           sync.Mutex
	requestReply dbus.RequestNameReply
	exports      []string
	signals      []emittedSignal
	released     bool
	closed       bool
	emitErr      error
}

func (b *fakeBus) Export(_ any, _ dbus.ObjectPath, iface string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.exports = append(b.exports, iface)
	return nil
}

func (b *fakeBus) ExportWithMap(_ any, _ map[string]string, _ dbus.ObjectPath, iface string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.exports = append(b.exports, iface)
	return nil
}

func (b *fakeBus) RequestName(_ string, _ dbus.RequestNameFlags) (dbus.RequestNameReply, error) {
	return b.requestReply, nil
}

func (b *fakeBus) ReleaseName(_ string) (dbus.ReleaseNameReply, error) {
	b.released = true
	return dbus.ReleaseNameReplyReleased, nil
}

func (b *fakeBus) Emit(_ dbus.ObjectPath, name string, values ...any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.signals = append(b.signals, emittedSignal{name: name, values: values})
	return b.emitErr
}

func (b *fakeBus) Close() error {
	b.closed = true
	return nil
}

func TestServiceLifecycleAndSignals(t *testing.T) {
	a := app.New(config.Default(), player.NewMock())
	var service *Service
	service = New(func(request app.PlaybackRequest) {
		result := request.Apply(a)
		service.Publish(result.Snapshot)
		request.Respond(result)
	}, nil)
	service.Publish(a.PlaybackSnapshot())
	bus := &fakeBus{requestReply: dbus.RequestNameReplyPrimaryOwner}
	if err := service.start(bus); err != nil {
		t.Fatal(err)
	}

	service.Publish(a.PlaybackSnapshot())
	if len(bus.signals) != 0 {
		t.Fatalf("unchanged publish emitted %d signals", len(bus.signals))
	}
	a.SetShuffle(true)
	service.Publish(a.PlaybackSnapshot())
	if len(bus.signals) != 1 || bus.signals[0].name != propertiesInterface+".PropertiesChanged" {
		t.Fatalf("signals = %#v", bus.signals)
	}
	service.Publish(a.PlaybackSnapshot())
	if len(bus.signals) != 1 {
		t.Fatal("duplicate property signal emitted")
	}
	service.Seeked(2 * time.Second)
	if got := bus.signals[1].values[0].(int64); got != 2_000_000 {
		t.Fatalf("Seeked position = %d", got)
	}

	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if !bus.released || !bus.closed {
		t.Fatalf("released=%v closed=%v", bus.released, bus.closed)
	}
	if err := service.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestServiceRejectsNameCollision(t *testing.T) {
	service := New(func(app.PlaybackRequest) {}, nil)
	bus := &fakeBus{requestReply: dbus.RequestNameReplyExists}
	if err := service.start(bus); err == nil {
		t.Fatal("start succeeded despite a name collision")
	}
}

func TestStartSessionWithoutBus(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/galdr-session-bus")
	service := New(func(app.PlaybackRequest) {}, nil)
	if err := service.StartSession(); err == nil {
		t.Fatal("StartSession succeeded without a session bus")
	}
}

func TestSignalFailureDisablesBusAndReports(t *testing.T) {
	reported := make(chan error, 1)
	service := New(func(app.PlaybackRequest) {}, func(err error) { reported <- err })
	bus := &fakeBus{
		requestReply: dbus.RequestNameReplyPrimaryOwner,
		emitErr:      errors.New("connection closed"),
	}
	if err := service.start(bus); err != nil {
		t.Fatal(err)
	}
	service.Publish(app.PlaybackSnapshot{Shuffle: true})
	select {
	case err := <-reported:
		if !strings.Contains(err.Error(), "connection closed") {
			t.Fatalf("reported error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connection loss was not reported")
	}
	if !bus.released || !bus.closed {
		t.Fatalf("released=%v closed=%v", bus.released, bus.closed)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPlayerCapabilitiesArePublished(t *testing.T) {
	service := New(func(app.PlaybackRequest) {}, nil)
	service.Publish(app.PlaybackSnapshot{
		CanPlay:     true,
		CanPause:    true,
		CanSeek:     true,
		CanNext:     true,
		CanPrevious: false,
	})
	properties := &propertyEndpoint{service: service}
	all, dbusErr := properties.GetAll(playerInterface)
	if dbusErr != nil {
		t.Fatal(dbusErr)
	}
	checks := map[string]bool{
		"CanPlay": true, "CanPause": true, "CanSeek": true,
		"CanGoNext": true, "CanGoPrevious": false, "CanControl": true,
	}
	for name, want := range checks {
		if got := all[name].Value().(bool); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	root, dbusErr := properties.GetAll(rootInterface)
	if dbusErr != nil {
		t.Fatal(dbusErr)
	}
	for _, name := range []string{"CanQuit", "CanRaise", "HasTrackList", "CanSetFullscreen"} {
		if root[name].Value().(bool) {
			t.Errorf("%s = true, want false", name)
		}
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPropertyWritesDispatchCommands(t *testing.T) {
	a := app.New(config.Default(), player.NewMock())
	service := New(func(request app.PlaybackRequest) {
		request.Respond(request.Apply(a))
	}, nil)
	properties := &propertyEndpoint{service: service}

	if err := properties.Set(playerInterface, "Volume", dbus.MakeVariant(0.42)); err != nil {
		t.Fatal(err)
	}
	if a.Volume() != 42 {
		t.Fatalf("volume = %d", a.Volume())
	}
	if err := properties.Set(playerInterface, "Shuffle", dbus.MakeVariant(true)); err != nil {
		t.Fatal(err)
	}
	if !a.Shuffle() {
		t.Fatal("shuffle was not enabled")
	}
	if err := properties.Set(playerInterface, "LoopStatus", dbus.MakeVariant("Track")); err != nil {
		t.Fatal(err)
	}
	if a.Repeat() != app.RepeatOne {
		t.Fatalf("repeat = %v", a.Repeat())
	}
	if err := properties.Set(playerInterface, "LoopStatus", dbus.MakeVariant("bad")); err == nil {
		t.Fatal("invalid loop status was accepted")
	}
	if err := properties.Set(playerInterface, "Rate", dbus.MakeVariant(2.0)); err == nil {
		t.Fatal("read-only rate was accepted")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentCallsAreSerializedByDispatcher(t *testing.T) {
	requests := make(chan app.PlaybackRequest)
	service := New(func(request app.PlaybackRequest) { requests <- request }, nil)
	done := make(chan struct{})
	var mu sync.Mutex
	active := 0
	maxActive := 0
	go func() {
		defer close(done)
		for range 20 {
			request := <-requests
			mu.Lock()
			active++
			if active > maxActive {
				maxActive = active
			}
			active--
			mu.Unlock()
			request.Respond(app.PlaybackResult{})
		}
	}()

	endpoint := &endpoint{service: service}
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := endpoint.Pause(); err != nil {
				t.Errorf("Pause: %v", err)
			}
		}()
	}
	wg.Wait()
	<-done
	if maxActive != 1 {
		t.Fatalf("max concurrent app calls = %d", maxActive)
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestInvokeUnblocksOnClose(t *testing.T) {
	started := make(chan struct{})
	service := New(func(app.PlaybackRequest) { close(started) }, nil)
	result := make(chan *dbus.Error, 1)
	go func() { result <- (&endpoint{service: service}).Pause() }()
	<-started
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-result:
		if got == nil {
			t.Fatal("call unexpectedly succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("call did not unblock on close")
	}
}
