# galdr

Galdr is a fast, keyboard-first terminal music player for local libraries. It
is written in Go and designed for Linux terminals, with first-class support for
Arch Linux and Omarchy.

It keeps music playback local, starts without mandatory configuration, and
uses libmpv to work with the audio output already available on the system.

## Features

- Recursive local-library scanning with metadata and filename fallbacks.
- MP3, WAV and FLAC playback through libmpv.
- Missing durations appear progressively without delaying startup or
  interrupting playback.
- Responsive three-panel interface for browsing the library, tracks and queue.
- Local album covers from `cover.jpg`, `cover.jpeg` or `cover.png`; artwork is
  never downloaded.
- Automatic theme that follows Omarchy when available, then the terminal's
  ANSI palette; also usable in basic 16-color terminals and without color.
- Play / pause / stop, next / previous, volume up / down, mute.
- Seek: ±5s with `←/→`, jump to start / end with `home/end`.
- Queue reordering and removal, shuffle, repeat and manual library rescans.
- Volume and last-track state persisted between sessions.
- **Incremental search** (`/`) over title, artist, album — case-insensitive
  substring match; the footer shows the active filter and matching count.
- Minimal TOML config file (optional — sensible defaults out of the box).
- No accounts, network calls, telemetry or background daemon.

The footer shows duration-loading progress such as `Durées 42/118`, then
briefly reports how many files were unavailable. Durations are kept for the
current session and recalculated the next time Galdr starts; no library database
or cache file is created.

## Requirements

- Go 1.26 or newer.
- Linux with a PipeWire, PulseAudio or ALSA-compatible audio stack.
- The system `mpv` package, which provides `libmpv.so`. On Arch / Omarchy:
  ```sh
  sudo pacman -S mpv
  ```

Galdr is built with `CGO_ENABLED=0`, but it is not a standalone binary:
`go-mpv` loads a compatible system `libmpv.so` dynamically at startup. If the
library is missing or its SONAME is incompatible, Galdr cannot start.

## Install and run

```sh
git clone https://github.com/kvitrvn/galdr.git
cd galdr

# Build ./bin/galdr
make build

# Run from source
make run

# Run the already-built binary
./bin/galdr
```

## Configuration

The config file is optional. If absent, galdr uses these defaults:

- `music_dir = ~/Music`
- `volume = 100`
- `theme = auto` (`auto` | `light` | `dark`)

`theme = auto` reads the active Omarchy palette from
`~/.config/omarchy/current/theme/colors.toml` without modifying it. Outside
Omarchy, or if that file is unavailable or invalid, Galdr inherits the
terminal's default foreground/background and ANSI palette. The explicit
`light` and `dark` modes keep their fixed palettes as configurable fallbacks.
Restart Galdr after changing the Omarchy theme to load the new palette.

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

| Format | Extension | Tags                        | Duration                | Playback        |
| ------ | --------- | --------------------------- | ----------------------- | --------------- |
| MP3    | `.mp3`    | ID3v1, ID3v2 (all versions) | loaded progressively    | libmpv / FFmpeg |
| WAV    | `.wav`    | RIFF INFO                   | available from startup  | libmpv / FFmpeg |
| FLAC   | `.flac`   | Vorbis comments             | available from startup  | libmpv / FFmpeg |

Files with unsupported extensions are skipped during the scan. Although mpv
supports many more inputs, Galdr deliberately accepts only local MP3, WAV and
FLAC files; URLs, video and additional formats remain out of scope.

Tags are read from every supported file at startup. When tags are missing, the
title falls back to the filename so every track remains identifiable. A manual
rescan refreshes the library and restarts duration loading for newly discovered
tracks.

## Album covers

Galdr never downloads artwork or contacts an external cover service. To display
an album cover, place a JPEG or PNG image next to the music files and name it
exactly `cover.jpg`, `cover.jpeg` or `cover.png`.

When several supported files are present, Galdr prefers `cover.jpg`, then
`cover.jpeg`, then `cover.png`. The image appears in the Now Playing area when
the terminal is large enough; playback works normally without one.

## Contributing

Issues, bug reports and focused pull requests are welcome. Before submitting a
change, keep it scoped to Galdr's local, lightweight terminal-player goals and
run the standard checks:

```sh
make fmt     # go fmt ./...
make vet     # go vet ./...
make test    # go test ./...
make build   # produces bin/galdr
make run     # runs the player
make tidy    # go mod tidy
make clean   # rm -rf bin/
```

Tests do not require audio hardware, a graphical desktop or network access.
The test suite uses a fake mpv client for playback behavior. A system
`libmpv.so` is still required when loading the binding.

## Arch Linux notes

- Install the runtime with `sudo pacman -S mpv`; no development headers or
  CGO toolchain are needed to build Galdr.
- libmpv automatically selects an available PipeWire, PulseAudio or ALSA
  output. Galdr does not force a backend.
- Personal `mpv.conf`, input bindings, scripts, OSC and video output are
  disabled so user mpv configuration cannot alter Galdr playback.

## Limitations

- No streaming, cloud sync, MPRIS, lyrics or visualizer.
- No persistent playlists, library DB or filesystem watcher (manual rescan
  only).
- Enriched durations are held in memory for the current session and are
  recalculated on the next launch.
- Files that libmpv cannot decode or whose duration remains unavailable keep
  the `--:--` placeholder. VBR MP3 duration is provided by libmpv.
- Playback capabilities depend on the installed mpv/libmpv build and its
  codec and audio-output dependencies.
- The binary dynamically depends on a compatible `libmpv.so` SONAME and is
  therefore not a self-contained portable artifact. Galdr does not bundle or
  statically distribute libmpv.
- Volume and last track are saved on quit and restored on next
  launch, but the in-queue position is not.

## License

Galdr is released under the [MIT License](LICENSE).

The `go-mpv` binding is also MIT licensed. The dynamically loaded libmpv is
LGPL 2.1+ or GPL, depending on how the system package was compiled.
