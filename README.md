# galdr

A lightweight terminal music player for local files, written in Go.

Built first as a personal tool, then prepared for free and open-source release.
Target platform: Linux (Arch / Omarchy first), PipeWire or ALSA audio backend.

## Features

- Local music directory scanning (recursive).
- Plays **MP3**, **WAV** (16-bit PCM), and **FLAC**.
- Keyboard-first terminal UI built with Bubble Tea + Lip Gloss.
- Adaptive theme that reads well on both light and dark terminals; also
  usable in basic 16-color terminals.
- Play / pause / stop, next / previous, volume up / down.
- Minimal TOML config file (optional — sensible defaults out of the box).
- No network calls, no telemetry, no background services.

## Status

This is the **MVP** described in `docs/roadmaps/mvp.md`.
All MVP acceptance criteria are tracked in `docs/release-mvp.md`.

## Requirements

- Go 1.26 or newer.
- Linux with a PipeWire or ALSA-compatible audio stack.
- `libasound2-dev` (ALSA headers) for Oto. On Arch / Omarchy:
  ```sh
  pacman -S alsa-lib
  ```

## Build & Run

```sh
# Build the binary into ./bin/galdr
make build

# Run from source
make run

# Run the already-built binary
./bin/galdr
```

A pre-built release artifact is not provided yet — see the [release
checklist](docs/release-mvp.md) for what still needs to happen before
that becomes useful.

## Configuration

The config file is optional. If absent, galdr uses these defaults:

- `music_dir = ~/Music`
- `volume = 100`
- `theme = auto` (`auto` | `light` | `dark`)

Path: `~/.config/galdr/config.toml`.

Example:

```toml
music_dir = "~/Music"
volume = 80
theme = "dark"
```

A leading `~` in `music_dir` is expanded against the current user's
home directory.

If the config file is missing, malformed, or contains an invalid value,
galdr starts anyway with the defaults and surfaces the issue in the TUI
status line.

## Keybindings

| Key            | Action                            |
| -------------- | --------------------------------- |
| `↑` / `k`      | Move selection up                 |
| `↓` / `j`      | Move selection down               |
| `n`            | Next track                        |
| `p`            | Previous track                    |
| `enter`        | Play selected track (or toggle)   |
| `space`        | Toggle play / pause               |
| `+` / `=`      | Volume up (5)                     |
| `-` / `_`      | Volume down (5)                   |
| `?`            | Toggle help overlay               |
| `q` / `ctrl+c` | Quit                              |

## Supported formats

| Format | Extension   | Tags                       | Duration | Notes                                  |
| ------ | ----------- | -------------------------- | -------- | -------------------------------------- |
| MP3    | `.mp3`      | ID3v1, ID3v2 (all versions)| not shown | All bitrates supported by go-mp3.   |
| WAV    | `.wav`      | RIFF INFO                  | shown    | 16-bit PCM, mono or stereo only.       |
| FLAC   | `.flac`     | Vorbis comments            | shown    | 16/24/32-bit, mono or stereo; downsampled to 16-bit for output. |

Files with unsupported extensions or non-PCM WAV (IEEE float, ADPCM,
etc.) are skipped during the scan or fail gracefully at load time with
an error in the status bar.

The scanner reads tags from every supported file at startup. When
tags are missing, the title falls back to the filename so every
track remains identifiable.

## Project layout

```txt
cmd/player/         entrypoint
internal/app/       application state and orchestration
internal/config/    TOML config loading and defaults
internal/library/   file scanning, Track and Queue models
internal/metadata/  tag and duration extraction (MP3, FLAC, WAV)
internal/metadatatest/  test-only fixture writers for audio files
internal/player/    Player interface and MockPlayer
internal/player/oto/  Oto v3 audio backend (MP3 / WAV / FLAC)
internal/theme/     Lip Gloss palettes (auto / light / dark)
internal/tui/       Bubble Tea models, views and keybindings
docs/roadmaps/      development roadmaps
```

The TUI depends only on the `Player` interface; it never imports a
concrete audio backend. The audio backend never imports TUI packages.
The library scanner never triggers playback.

## Development

```sh
make fmt     # go fmt ./...
make vet     # go vet ./...
make test    # go test ./...
make build   # produces bin/galdr
make run     # runs the player
make tidy    # go mod tidy
make clean   # rm -rf bin/
```

Tests do not require audio hardware, a graphical desktop or network
access. The Oto integration test is skipped automatically when no audio
device is available.

## Arch Linux notes

- galdr talks to ALSA through Oto v3, which is fully compatible with
  PipeWire / WirePlumber (PipeWire ships an ALSA plugin that
  transparently forwards audio to the WirePlumber session).
- Make sure your user is in the `audio` group if you hit permission
  errors when opening `/dev/snd`.
- For per-track playback to work with the system default sink, no extra
  configuration is needed.

## Known limitations (MVP scope)

- No streaming, cloud sync, MPRIS, album art, lyrics or visualizer.
- No queue editing, search, shuffle / repeat, persistent playlists.
- No library database — the directory is rescanned on every launch.
- WAV support is restricted to 16-bit PCM.
- MP3 duration is not displayed: VBR files require decoding the
  whole stream to count samples, which is too slow for a library
  scan. The progress bar shows `[··········]` until a future v1
  feature computes it lazily.
- The position display is derived from PCM bytes consumed by Oto and
  may lag slightly behind real-time.
- The audio backend (Oto) maintains a single global audio context per
  process; switching tracks recreates it transparently.

## License

Not yet chosen. The intent is to release under a permissive license
(MIT or BSD-2-Clause) once the MVP is validated end to end.