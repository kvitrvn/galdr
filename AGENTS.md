# AGENTS.md

Repository instructions for AI coding agents.

## Project

This is a lightweight terminal music player written in Go.

Primary goal: local music playback in a fast, simple TUI.

Primary target: Arch Linux / Omarchy, Linux terminal, PipeWire/ALSA-compatible audio.

## Stack

* Language: Go
* TUI: Bubble Tea, Bubbles, Lip Gloss
* Audio MVP: lightweight Go backend, preferably Beep/Oto
* Metadata: tag reader for MP3/FLAC/WAV where possible
* Config: TOML
* Future cache: SQLite only after MVP

Avoid heavy native dependencies, CGO, GStreamer, FFmpeg bindings, or libmpv unless explicitly requested.

## Commands

Use direct Go commands unless a Makefile exists.

```sh
go fmt ./...
go test ./...
go vet ./...
go run ./cmd/player
```

If a Makefile exists, prefer its targets.

## Architecture

Keep these boundaries:

```txt
cmd/player/       entrypoint
internal/app/     app state and orchestration
internal/tui/     Bubble Tea models, views, keybindings
internal/player/  playback interface and state
internal/library/ scanning, track discovery, queues
internal/metadata/ tag and duration extraction
internal/config/  config loading and defaults
internal/theme/   terminal styling
```

The TUI must depend on `internal/player` interfaces, not a concrete audio backend.

The audio backend must not import TUI packages.

The library scanner must not trigger playback.

## MVP Scope

Implement only:

* Scan local music directory
* List tracks in the TUI
* Play MP3, WAV, FLAC
* Play / pause / stop
* Next / previous
* Progress display
* Volume control
* Minimal config file
* Adaptive terminal colors

Do not add streaming, lyrics, album art, MPRIS, database indexing, plugins, visualizers, or remote control unless explicitly requested.

## Code Style

Use idiomatic Go.

Prefer simple structs, small packages, explicit errors, and standard library first.

Avoid global mutable state, panic-based control flow, premature concurrency, and broad abstractions.

Add interfaces only at package boundaries where they protect architecture.

## Testing

Add tests for deterministic logic:

* File scanning
* Supported extension filtering
* Metadata fallback
* Config loading
* Queue navigation
* Player state transitions

Tests must not require a real audio device, network access, or a graphical desktop.

## Agent Behavior

Before large changes, inspect `PROJECT.md` and make a short plan.

Keep changes small and reviewable.

Do not rewrite unrelated code.

Do not add dependencies without explaining why.

After editing, run formatting and relevant tests.

Summaries should include:

* What changed
* Why it changed
* How it was tested
* Any known limitations
