// Package app centralizes application state and orchestration.
//
// The app layer coordinates the library, the audio player and the TUI.
// It must not import any TUI or audio-backend concrete types; it depends
// only on the player interface defined in internal/player.
package app
