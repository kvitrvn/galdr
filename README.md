# galdr

A lightweight terminal music player for local files, written in Go.

Built first as a personal tool, then prepared for free and open-source release.
Target platform: Linux (Arch / Omarchy first), PipeWire or ALSA audio backend.

## Features

- Local music directory scanning (recursive).
- Plays **MP3**, **WAV** (PCM 16/24/32-bit, IEEE float 32/64-bit, including WAVE_FORMAT_EXTENSIBLE), and **FLAC**.
- Keyboard-first terminal UI built with Bubble Tea + Lip Gloss.
- Adaptive theme that reads well on both light and dark terminals; also
  usable in basic 16-color terminals.
- Play / pause / stop, next / previous, volume up / down, mute.
- Seek: ±5s with `←/→`, jump to start / end with `home/end`.
- Shuffle, repeat (off / all / one), rescan, persistent state.
- **Incremental search** (`/`) over title, artist, album — case-insensitive
  substring match; the footer shows the active filter and matching count.
- Minimal TOML config file (optional — sensible defaults out of the box).
- No network calls, no telemetry, no background services.

## Status

This is the **v1** described in `docs/roadmaps/v1.md`. It builds on
the MVP (v0.1) and adds metadata, seek, shuffle, repeat, mute, rescan,
persistent state and the search box. Acceptance criteria are tracked
in `docs/release-v1.md` and in the [v1 roadmap](docs/roadmaps/v1.md);
the MVP checklist (`docs/release-mvp.md`) remains authoritative for
the v0.1 feature set.

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

[ui]
# Preferred side-panel widths in wide terminals.
left_width = 22
right_width = 22
min_width = 48
min_height = 14
```

A leading `~` in `music_dir` is expanded against the current user's
home directory.

If the config file is missing, malformed, or contains an invalid value,
galdr starts anyway with the defaults and surfaces the issue in the TUI
message area.

## Keybindings

The TUI adapts to the terminal: all three panels are visible from 110
columns, Library plus Tracks or Queue from 72 to 109 columns, and one
focused panel from 48 to 71 columns. `Tab` / `shift+Tab` cycles focus;
`1`, `2`, and `3` jump directly to Library, Tracks, and Queue. Focus is
shown with a symbol, text weight, and color.

| Key            | Library (focused)               | Tracks / Queue (focused)            |
| -------------- | ------------------------------- | ----------------------------------- |
| `↑` / `k`      | previous row                    | previous track                     |
| `↓` / `j`      | next row                        | next track                         |
| `←` / `h`      | collapse artist / go to parent  | seek -5s                           |
| `→` / `l`      | expand artist / drill in        | seek +5s                           |
| `enter`        | select artist or album          | play selected track                 |
| `tab`          | cycle focus forward             | cycle focus forward                 |
| `shift+tab`    | cycle focus backward            | cycle focus backward                |

Global keys (work in any panel):

| Key            | Action                                          |
| -------------- | ----------------------------------------------- |
| `space`        | Toggle play / pause                             |
| `x`            | Stop playback                                   |
| `n`            | Next track (shuffle-aware, scope + filter)      |
| `p`            | Previous track (shuffle-aware, scope + filter)  |
| `home` / `end` | Seek to start / end of current track            |
| `+` / `=`      | Volume up (5)                                   |
| `-` / `_`      | Volume down (5)                                 |
| `m`            | Toggle mute                                     |
| `r`            | Rescan the music directory                      |
| `s`            | Toggle shuffle                                  |
| `R`            | Cycle repeat (off → all → one → off)            |
| `/`            | Enter search (filter by title / artist / album) |
| `esc`          | Clear filter (or exit search input)             |
| `ctrl+l`       | Clear filter                                    |
| `?`            | Toggle help overlay                             |
| `q` / `ctrl+c` | Quit                                            |

Queue panel (when focused):

| Key             | Action                                        |
| --------------- | --------------------------------------------- |
| `↑` / `k`       | Move cursor up                                |
| `↓` / `j`       | Move cursor down                              |
| `K` / `shift+↑` | Move the highlighted track up in the queue    |
| `J` / `shift+↓` | Move the highlighted track down in the queue  |
| `d`             | Remove the highlighted track (except playing) |
| `c`             | Clear the queue (keep the playing track)      |
| `enter`         | Play the highlighted track immediately        |

The search input is incremental: each keystroke updates the
filter live and the list shrinks in every panel. `enter` or `esc`
exits the input and keeps the filter active; a second `esc` (or
`ctrl+l`) clears it. While the filter is active, the Library
panel hides artists and albums with no matching track, and
`n` / `p` / `↑↓` in the Tracks panel operate on the visible
subset.

## Supported formats

| Format | Extension   | Tags                       | Duration | Notes                                  |
| ------ | ----------- | -------------------------- | -------- | -------------------------------------- |
| MP3    | `.mp3`      | ID3v1, ID3v2 (all versions)| not shown | All bitrates supported by go-mp3.   |
| WAV    | `.wav`      | RIFF INFO                  | shown    | PCM 16/24/32-bit, IEEE float 32/64-bit, mono or stereo. WAVE_FORMAT_EXTENSIBLE supported. Always downsampled to 16-bit PCM for output. |
| FLAC   | `.flac`     | Vorbis comments            | shown    | 16/24/32-bit, mono or stereo; downsampled to 16-bit for output. |

Files with unsupported extensions are skipped during the scan. A WAV
with an unrecognised encoding (A-law, µ-law, ADPCM, ...) fails at
load time with a clear error in the message area.

The scanner reads tags from every supported file at startup. When
tags are missing, the title falls back to the filename so every
track remains identifiable.

## Project layout

```txt
cmd/player/         entrypoint
internal/app/       application state and orchestration (shuffle / repeat / mute / seek / rescan)
internal/config/    TOML config loading and defaults
internal/library/   file scanning, Track and Queue models
internal/metadata/  tag and duration extraction (MP3, FLAC, WAV)
internal/metadatatest/  test-only fixture writers for audio files
internal/player/    Player interface and MockPlayer
internal/player/oto/  Oto v3 audio backend (MP3 / WAV / FLAC) with seek
internal/state/     persistent per-user state (volume, last track)
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

## Known limitations (v1 scope)

- No streaming, cloud sync, MPRIS, album art, lyrics or visualizer.
- No queue editing, persistent playlists, library DB or filesystem
  watcher (manual rescan only).
- WAV: only PCM (16/24/32-bit) and IEEE float (32/64-bit) are
  decoded. Other encodings (A-law, µ-law, ADPCM, MP3-in-WAV, ...)
  fail at load time with a clear error.
- All WAV output is downsampled to 16-bit PCM (Oto limitation).
- MP3 duration is not displayed: VBR files require decoding the
  whole stream to count samples, which is too slow for a library
  scan. The progress bar shows `[··········] --:--`.
- MP3 seek is implemented by re-decoding from the start and
  discarding samples. It is correct for VBR but slow; FLAC and WAV
  seek efficiently.
- FLAC seek may need to scan the whole file the first time to build
  a seektable (mewkiz/flac), which is slow for long files.
- The position display is derived from PCM bytes consumed by Oto and
  may lag slightly behind real-time.
- The audio backend (Oto) maintains a single global audio context per
  process; switching tracks recreates it transparently.
- Volume and last track are saved on quit and restored on next
  launch, but the in-queue position is not.

## License

Not yet chosen. The intent is to release under a permissive license
(MIT or BSD-2-Clause) once the MVP is validated end to end.
