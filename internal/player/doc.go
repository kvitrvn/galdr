// Package player defines the playback interface and holds playback state.
//
// It exposes a small, MVP-focused interface (Load, Play, Pause, Stop,
// Volume, Position, Duration) so that the TUI and app layers stay
// independent from the concrete audio backend.
package player
