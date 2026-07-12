package mpris

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/kvitrvn/galdr/internal/app"
	"github.com/kvitrvn/galdr/internal/library"
)

type endpoint struct {
	service *Service
}

type rootEndpoint struct{}

func (e *endpoint) Next() *dbus.Error     { return e.action(app.PlaybackActionNext) }
func (e *endpoint) Previous() *dbus.Error { return e.action(app.PlaybackActionPrevious) }
func (e *endpoint) Pause() *dbus.Error    { return e.action(app.PlaybackActionPause) }
func (e *endpoint) PlayPause() *dbus.Error {
	return e.action(app.PlaybackActionPlayPause)
}
func (e *endpoint) Stop() *dbus.Error { return e.action(app.PlaybackActionStop) }
func (e *endpoint) Play() *dbus.Error { return e.action(app.PlaybackActionPlay) }

func (e *endpoint) SeekRelative(offset int64) *dbus.Error {
	request := app.NewPlaybackRequest(app.PlaybackCommand{
		Action:   app.PlaybackActionSeek,
		Position: microsecondsToDuration(offset),
	})
	return resultError(e.service.invoke(request))
}

func (e *endpoint) SetPosition(path dbus.ObjectPath, position int64) *dbus.Error {
	id, ok := trackID(path)
	if !ok || position < 0 {
		return invalidArgs("invalid track id or position")
	}
	request := app.NewPlaybackRequest(app.PlaybackCommand{
		Action:   app.PlaybackActionSetPosition,
		TrackID:  id,
		Position: microsecondsToDuration(position),
	})
	return resultError(e.service.invoke(request))
}

func (e *endpoint) action(action app.PlaybackAction) *dbus.Error {
	request := app.NewPlaybackRequest(app.PlaybackCommand{Action: action})
	return resultError(e.service.invoke(request))
}

func resultError(result app.PlaybackResult) *dbus.Error {
	if result.Err == nil {
		return nil
	}
	return dbus.NewError("org.mpris.MediaPlayer2.Player.Error.Failed", []any{result.Err.Error()})
}

func trackID(path dbus.ObjectPath) (library.QueueEntryID, bool) {
	const prefix = "/org/mpris/MediaPlayer2/track/"
	raw := string(path)
	if !strings.HasPrefix(raw, prefix) {
		return 0, false
	}
	value, err := strconv.ParseUint(strings.TrimPrefix(raw, prefix), 10, 64)
	return library.QueueEntryID(value), err == nil && value != 0
}

type propertyEndpoint struct {
	service *Service
}

func (p *propertyEndpoint) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	properties, dbusErr := p.properties(iface)
	if dbusErr != nil {
		return dbus.Variant{}, dbusErr
	}
	value, ok := properties[name]
	if !ok {
		return dbus.Variant{}, invalidArgs(fmt.Sprintf("unknown property %s.%s", iface, name))
	}
	return value, nil
}

func (p *propertyEndpoint) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	return p.properties(iface)
}

func (p *propertyEndpoint) Set(iface, name string, value dbus.Variant) *dbus.Error {
	if iface != playerInterface {
		return invalidArgs("interface has no writable properties")
	}
	request := app.PlaybackRequest{}
	switch name {
	case "Volume":
		volume, ok := value.Value().(float64)
		if !ok {
			return invalidArgs("volume must be a double")
		}
		converted, ok := volumeFromMPRIS(volume)
		if !ok {
			return invalidArgs("volume must be non-negative and finite")
		}
		request = app.NewPlaybackRequest(app.PlaybackCommand{
			Action: app.PlaybackActionSetVolume,
			Volume: converted,
		})
	case "Shuffle":
		shuffle, ok := value.Value().(bool)
		if !ok {
			return invalidArgs("shuffle must be a boolean")
		}
		request = app.NewPlaybackRequest(app.PlaybackCommand{
			Action:  app.PlaybackActionSetShuffle,
			Shuffle: shuffle,
		})
	case "LoopStatus":
		status, ok := value.Value().(string)
		if !ok {
			return invalidArgs("loop status must be a string")
		}
		mode, ok := repeatMode(status)
		if !ok {
			return invalidArgs("invalid loop status")
		}
		request = app.NewPlaybackRequest(app.PlaybackCommand{
			Action: app.PlaybackActionSetRepeat,
			Repeat: mode,
		})
	default:
		return dbus.NewError("org.freedesktop.DBus.Error.PropertyReadOnly", []any{name})
	}
	return resultError(p.service.invoke(request))
}

func (p *propertyEndpoint) properties(iface string) (map[string]dbus.Variant, *dbus.Error) {
	switch iface {
	case rootInterface:
		return rootProperties(), nil
	case playerInterface:
		p.service.mu.RLock()
		defer p.service.mu.RUnlock()
		return p.service.playerPropertiesLocked(), nil
	default:
		return nil, invalidArgs("unknown interface")
	}
}

func invalidArgs(message string) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.InvalidArgs", []any{message})
}

type introspector struct{}

func (introspector) Introspect() (string, *dbus.Error) {
	return introspectionXML, nil
}

const introspectionXML = `<node>
  <interface name="org.mpris.MediaPlayer2">
    <property name="CanQuit" type="b" access="read"/>
    <property name="CanRaise" type="b" access="read"/>
    <property name="Fullscreen" type="b" access="readwrite"/>
    <property name="CanSetFullscreen" type="b" access="read"/>
    <property name="HasTrackList" type="b" access="read"/>
    <property name="Identity" type="s" access="read"/>
    <property name="DesktopEntry" type="s" access="read"/>
    <property name="SupportedUriSchemes" type="as" access="read"/>
    <property name="SupportedMimeTypes" type="as" access="read"/>
  </interface>
  <interface name="org.mpris.MediaPlayer2.Player">
    <method name="Next"/><method name="Previous"/><method name="Pause"/>
    <method name="PlayPause"/><method name="Stop"/><method name="Play"/>
    <method name="Seek"><arg direction="in" type="x" name="Offset"/></method>
    <method name="SetPosition"><arg direction="in" type="o" name="TrackId"/><arg direction="in" type="x" name="Position"/></method>
    <signal name="Seeked"><arg type="x" name="Position"/></signal>
    <property name="PlaybackStatus" type="s" access="read"/>
    <property name="LoopStatus" type="s" access="readwrite"/>
    <property name="Rate" type="d" access="read"/>
    <property name="Shuffle" type="b" access="readwrite"/>
    <property name="Metadata" type="a{sv}" access="read"/>
    <property name="Volume" type="d" access="readwrite"/>
    <property name="Position" type="x" access="read"/>
    <property name="MinimumRate" type="d" access="read"/>
    <property name="MaximumRate" type="d" access="read"/>
    <property name="CanGoNext" type="b" access="read"/>
    <property name="CanGoPrevious" type="b" access="read"/>
    <property name="CanPlay" type="b" access="read"/>
    <property name="CanPause" type="b" access="read"/>
    <property name="CanSeek" type="b" access="read"/>
    <property name="CanControl" type="b" access="read"/>
  </interface>
  <interface name="org.freedesktop.DBus.Properties">
    <method name="Get"><arg direction="in" type="s"/><arg direction="in" type="s"/><arg direction="out" type="v"/></method>
    <method name="Set"><arg direction="in" type="s"/><arg direction="in" type="s"/><arg direction="in" type="v"/></method>
    <method name="GetAll"><arg direction="in" type="s"/><arg direction="out" type="a{sv}"/></method>
    <signal name="PropertiesChanged"><arg type="s"/><arg type="a{sv}"/><arg type="as"/></signal>
  </interface>
  <interface name="org.freedesktop.DBus.Introspectable"><method name="Introspect"><arg direction="out" type="s"/></method></interface>
</node>`
