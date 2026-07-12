package mpris

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/kvitrvn/galdr/internal/app"
)

// DispatchFunc inserts a playback request into the Bubble Tea event loop.
type DispatchFunc func(app.PlaybackRequest)

type busConn interface {
	Export(v any, path dbus.ObjectPath, iface string) error
	ExportWithMap(v any, mapping map[string]string, path dbus.ObjectPath, iface string) error
	RequestName(name string, flags dbus.RequestNameFlags) (dbus.RequestNameReply, error)
	ReleaseName(name string) (dbus.ReleaseNameReply, error)
	Emit(path dbus.ObjectPath, name string, values ...any) error
	Close() error
}

// Service owns the MPRIS state cache, D-Bus exports, and signal lifecycle.
type Service struct {
	mu       sync.RWMutex
	dispatch DispatchFunc
	bus      busConn
	snapshot app.PlaybackSnapshot
	metadata map[string]dbus.Variant
	done     chan struct{}
	close    sync.Once
	report   func(error)
}

// New constructs a stopped service. StartSession connects it to D-Bus.
func New(dispatch DispatchFunc, report func(error)) *Service {
	return &Service{
		dispatch: dispatch,
		metadata: metadata(app.PlaybackSnapshot{}),
		done:     make(chan struct{}),
		report:   report,
	}
}

// StartSession connects to the user's session bus and claims Galdr's name.
func (s *Service) StartSession() error {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("connect to session bus: %w", err)
	}
	if err := s.start(conn); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			return errors.Join(err, fmt.Errorf("close session bus: %w", closeErr))
		}
		return err
	}
	return nil
}

func (s *Service) start(conn busConn) error {
	if s.dispatch == nil {
		return errors.New("mpris dispatcher is nil")
	}
	endpoint := &endpoint{service: s}
	properties := &propertyEndpoint{service: s}
	if err := conn.Export(rootEndpoint{}, objectPath, rootInterface); err != nil {
		return fmt.Errorf("export root interface: %w", err)
	}
	if err := conn.ExportWithMap(
		endpoint,
		map[string]string{"SeekRelative": "Seek"},
		objectPath,
		playerInterface,
	); err != nil {
		return fmt.Errorf("export player interface: %w", err)
	}
	if err := conn.Export(properties, objectPath, propertiesInterface); err != nil {
		return fmt.Errorf("export properties interface: %w", err)
	}
	if err := conn.Export(introspector{}, objectPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		return fmt.Errorf("export introspection interface: %w", err)
	}
	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("request bus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return errors.New("mpris name is already owned")
	}
	s.mu.Lock()
	s.bus = conn
	s.mu.Unlock()
	return nil
}

// Close releases the bus name and closes the connection. It is idempotent.
func (s *Service) Close() error {
	var closeErr error
	s.close.Do(func() {
		close(s.done)
		s.mu.Lock()
		conn := s.bus
		s.bus = nil
		s.mu.Unlock()
		if conn == nil {
			return
		}
		if _, err := conn.ReleaseName(busName); err != nil {
			closeErr = fmt.Errorf("release mpris name: %w", err)
		}
		if err := conn.Close(); err != nil {
			closeErr = errors.Join(closeErr, fmt.Errorf("close mpris bus: %w", err))
		}
	})
	return closeErr
}

// Publish refreshes the cached state and emits only changed Player
// properties. Position is cached but deliberately excluded from signals.
func (s *Service) Publish(snapshot app.PlaybackSnapshot) {
	s.mu.Lock()
	before := s.playerPropertiesLocked()
	trackChanged := s.snapshot.TrackID != snapshot.TrackID ||
		s.snapshot.Duration != snapshot.Duration ||
		!reflect.DeepEqual(s.snapshot.Track, snapshot.Track)
	s.snapshot = snapshot
	if trackChanged {
		s.metadata = metadata(snapshot)
	}
	after := s.playerPropertiesLocked()
	conn := s.bus
	s.mu.Unlock()

	delete(before, "Position")
	delete(after, "Position")
	changed := changedProperties(before, after)
	if conn == nil || len(changed) == 0 {
		return
	}
	if err := conn.Emit(
		objectPath,
		propertiesInterface+".PropertiesChanged",
		playerInterface,
		changed,
		[]string{},
	); err != nil {
		s.disableBus(conn, fmt.Errorf("emit mpris property changes: %w", err))
	}
}

// Seeked emits the standard discontinuous-position signal.
func (s *Service) Seeked(position time.Duration) {
	s.mu.RLock()
	conn := s.bus
	s.mu.RUnlock()
	if conn != nil {
		if err := conn.Emit(
			objectPath,
			playerInterface+".Seeked",
			durationToMicroseconds(position),
		); err != nil {
			s.disableBus(conn, fmt.Errorf("emit mpris seek: %w", err))
		}
	}
}

func (s *Service) disableBus(conn busConn, cause error) {
	s.mu.Lock()
	if s.bus != conn {
		s.mu.Unlock()
		return
	}
	s.bus = nil
	s.mu.Unlock()

	if _, err := conn.ReleaseName(busName); err != nil {
		cause = errors.Join(cause, fmt.Errorf("release mpris name: %w", err))
	}
	if err := conn.Close(); err != nil {
		cause = errors.Join(cause, fmt.Errorf("close mpris bus: %w", err))
	}
	if s.report != nil {
		s.report(cause)
	}
}

func (s *Service) invoke(request app.PlaybackRequest) app.PlaybackResult {
	s.dispatch(request)
	select {
	case result := <-request.Reply():
		return result
	case <-s.done:
		return app.PlaybackResult{Err: errors.New("mpris service stopped")}
	}
}

func (s *Service) playerPropertiesLocked() map[string]dbus.Variant {
	snapshot := s.snapshot
	return map[string]dbus.Variant{
		"PlaybackStatus": dbus.MakeVariant(playbackStatus(snapshot.State)),
		"LoopStatus":     dbus.MakeVariant(loopStatus(snapshot.Repeat)),
		"Rate":           dbus.MakeVariant(1.0),
		"Shuffle":        dbus.MakeVariant(snapshot.Shuffle),
		"Metadata":       dbus.MakeVariant(s.metadata),
		"Volume":         dbus.MakeVariant(volumeToMPRIS(snapshot.Volume)),
		"Position":       dbus.MakeVariant(durationToMicroseconds(snapshot.Position)),
		"MinimumRate":    dbus.MakeVariant(1.0),
		"MaximumRate":    dbus.MakeVariant(1.0),
		"CanGoNext":      dbus.MakeVariant(snapshot.CanNext),
		"CanGoPrevious":  dbus.MakeVariant(snapshot.CanPrevious),
		"CanPlay":        dbus.MakeVariant(snapshot.CanPlay),
		"CanPause":       dbus.MakeVariant(snapshot.CanPause),
		"CanSeek":        dbus.MakeVariant(snapshot.CanSeek),
		"CanControl":     dbus.MakeVariant(true),
	}
}

func rootProperties() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"CanQuit":             dbus.MakeVariant(false),
		"CanRaise":            dbus.MakeVariant(false),
		"Fullscreen":          dbus.MakeVariant(false),
		"CanSetFullscreen":    dbus.MakeVariant(false),
		"HasTrackList":        dbus.MakeVariant(false),
		"Identity":            dbus.MakeVariant("Galdr"),
		"DesktopEntry":        dbus.MakeVariant("galdr"),
		"SupportedUriSchemes": dbus.MakeVariant([]string{}),
		"SupportedMimeTypes": dbus.MakeVariant([]string{
			"audio/flac",
			"audio/mpeg",
			"audio/wav",
			"audio/x-wav",
		}),
	}
}
