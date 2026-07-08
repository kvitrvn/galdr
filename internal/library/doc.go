// Package library handles local music directory scanning, track discovery
// and queue construction.
//
// The scanner must not trigger playback; it only produces a stable list of
// Track values that the app layer consumes.
package library
