# PROJECT.md

# Go TUI Music Player

A lightweight terminal music player written in Go.

Built first as a personal tool, then prepared for free and open-source release.

## Product Goal

Build a fast, keyboard-first TUI music player for local files.

The app should be:

* Lightweight
* Easy to use
* Fast to launch
* Pleasant in a terminal
* Theme-adaptive
* Reliable on Arch Linux / Omarchy

## Target Platform

Primary:

```txt
Arch Linux
Omarchy
Linux terminal
PipeWire / WirePlumber
ALSA compatibility
```

Secondary:

```txt
Other Linux distributions
```

Out of scope for MVP:

```txt
Windows
macOS
Mobile
Web
```

## MVP

The MVP should support:

* Local folder scanning
* MP3 playback
* WAV playback
* FLAC playback
* Track list
* Keyboard navigation
* Play / pause
* Stop
* Next / previous
* Progress display
* Volume control
* Minimal config
* Adaptive terminal theme

## Not MVP

Do not include initially:

* Streaming services
* Cloud sync
* Album art
* Lyrics
* MPRIS
* SQLite index
* Plugin system
* Equalizer
* Visualizer
* Remote control

## Suggested Stack

```txt
Go
Bubble Tea
Bubbles
Lip Gloss
Beep/Oto or equivalent lightweight audio backend
TOML config
SQLite later, only when library indexing is needed
```

## UX Direction

Default interaction should be simple:

```txt
q          quit
space      play / pause
enter      play selected track
j/down     move down
k/up       move up
n          next
p          previous
+          volume up
-          volume down
?          help
```

The UI should remain readable in small terminals and both light and dark themes.

## Design Principles

1. Local-first.
2. No telemetry.
3. No network calls for MVP.
4. No required config before first launch.
5. Gracefully skip unreadable files.
6. Keep the codebase understandable for one maintainer.
7. Prefer boring, maintainable Go over clever abstractions.

## Future Features

After the MVP is stable:

* Search
* Queue management
* Shuffle / repeat
* Persistent playlists
* SQLite library cache
* Filesystem watcher
* MPRIS
* AUR package
* Release automation
