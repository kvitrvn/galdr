// Package i18n provides Galdr's small, embedded runtime translation catalogues.
package i18n

import (
	"fmt"
	"strings"
)

// Language is a supported user-interface language.
type Language string

const (
	Auto    Language = "auto"
	English Language = "en"
	French  Language = "fr"
	Spanish Language = "es"
	German  Language = "de"
)

// Valid reports whether the value is accepted in configuration.
func (l Language) Valid() bool {
	switch l {
	case Auto, English, French, Spanish, German:
		return true
	default:
		return false
	}
}

// Key identifies one complete user-facing phrase.
type Key string

const (
	Library               Key = "library"
	Tracks                Key = "tracks"
	Queue                 Key = "queue"
	NowPlaying            Key = "now_playing"
	Stopped               Key = "stopped"
	Playing               Key = "playing"
	Paused                Key = "paused"
	NothingPlaying        Key = "nothing_playing"
	UnknownArtist         Key = "unknown_artist"
	EmptyLibrary          Key = "empty_library"
	NoLibraryMatches      Key = "no_library_matches"
	LibraryEmpty          Key = "library_empty"
	EmptyScope            Key = "empty_scope"
	EmptyAlbum            Key = "empty_album"
	EmptyArtist           Key = "empty_artist"
	NoTrackMatches        Key = "no_track_matches"
	NoMusicFound          Key = "no_music_found"
	EmptyQueue            Key = "empty_queue"
	TerminalTooSmall      Key = "terminal_too_small"
	SearchPlaceholder     Key = "search_placeholder"
	ResultsOne            Key = "results_one"
	ResultsOther          Key = "results_other"
	UnavailableOne        Key = "unavailable_one"
	UnavailableOther      Key = "unavailable_other"
	DurationsProgress     Key = "durations_progress"
	DurationsSummaryOne   Key = "durations_summary_one"
	DurationsSummaryOther Key = "durations_summary_other"
	Volume                Key = "volume"
	Mute                  Key = "mute"
	Shuffle               Key = "shuffle"
	Repeat                Key = "repeat"
	On                    Key = "on"
	Off                   Key = "off"
	All                   Key = "all"
	One                   Key = "one"
	ErrorPrefix           Key = "error_prefix"
	ScopeAll              Key = "scope_all"
	ScopeNamed            Key = "scope_named"
	FilterContext         Key = "filter_context"
	FilterBadge           Key = "filter_badge"
	MuteBadge             Key = "mute_badge"
	ShuffleBadge          Key = "shuffle_badge"
	RepeatBadge           Key = "repeat_badge"
	QueueSelected         Key = "queue_selected"
	QueueNow              Key = "queue_now"

	StatusLibraryScanFailed Key = "status_library_scan_failed"
	StatusLoadedOne         Key = "status_loaded_one"
	StatusLoadedOther       Key = "status_loaded_other"
	StatusRescanFailed      Key = "status_rescan_failed"
	StatusRescannedOne      Key = "status_rescanned_one"
	StatusRescannedOther    Key = "status_rescanned_other"
	StatusScopeAll          Key = "status_scope_all"
	StatusScopeEmpty        Key = "status_scope_empty"
	StatusScopeAlbum        Key = "status_scope_album"
	StatusScopeArtist       Key = "status_scope_artist"
	StatusMovedUp           Key = "status_moved_up"
	StatusMovedDown         Key = "status_moved_down"
	StatusRemovedQueue      Key = "status_removed_queue"
	StatusQueueCleared      Key = "status_queue_cleared"
	StatusFilterCleared     Key = "status_filter_cleared"
	StatusFilter            Key = "status_filter"
	StatusVolume            Key = "status_volume"
	StatusNoVisibleTracks   Key = "status_no_visible_tracks"
	StatusStopped           Key = "status_stopped"
	StatusEndQueue          Key = "status_end_queue"
	StatusFailedAdvance     Key = "status_failed_advance"
	StatusPlaying           Key = "status_playing"
	StatusShuffleOn         Key = "status_shuffle_on"
	StatusShuffleOff        Key = "status_shuffle_off"
	StatusRepeatAll         Key = "status_repeat_all"
	StatusRepeatOne         Key = "status_repeat_one"
	StatusRepeatOff         Key = "status_repeat_off"
	StatusMuted             Key = "status_muted"
	StatusUnmuted           Key = "status_unmuted"
	StatusMutedVolume       Key = "status_muted_volume"
	StatusFailedLoad        Key = "status_failed_load"
	StatusFailedStart       Key = "status_failed_start"
	StatusFailedPrepare     Key = "status_failed_prepare"
	StatusFailedDecode      Key = "status_failed_decode"
	StatusFailedStop        Key = "status_failed_stop"
	StatusPaused            Key = "status_paused"
	StatusResumed           Key = "status_resumed"

	ShortcutLibrary   Key = "shortcut_library"
	ShortcutTracks    Key = "shortcut_tracks"
	ShortcutQueue     Key = "shortcut_queue"
	ShortcutGlobal    Key = "shortcut_global"
	HelpTitle         Key = "help_title"
	HelpNavigation    Key = "help_navigation"
	HelpPlayback      Key = "help_playback"
	HelpLibrarySearch Key = "help_library_search"
	HelpQueue         Key = "help_queue"
	HelpApplication   Key = "help_application"
	HelpNavMove       Key = "help_nav_move"
	HelpNavOpen       Key = "help_nav_open"
	HelpNavPanel      Key = "help_nav_panel"
	HelpNavChoose     Key = "help_nav_choose"
	HelpNavSelect     Key = "help_nav_select"
	HelpNavSeek       Key = "help_nav_seek"
	HelpPlayPause     Key = "help_play_pause"
	HelpStop          Key = "help_stop"
	HelpNextPrev      Key = "help_next_prev"
	HelpVolume        Key = "help_volume"
	HelpMute          Key = "help_mute"
	HelpShuffleRepeat Key = "help_shuffle_repeat"
	HelpRescan        Key = "help_rescan"
	HelpSearch        Key = "help_search"
	HelpClearFilter   Key = "help_clear_filter"
	HelpQueueUp       Key = "help_queue_up"
	HelpQueueDown     Key = "help_queue_down"
	HelpQueueRemove   Key = "help_queue_remove"
	HelpClose         Key = "help_close"
	HelpQuit          Key = "help_quit"

	DiagLoadConfig          Key = "diag_load_config"
	DiagInitAudio           Key = "diag_init_audio"
	DiagLoadState           Key = "diag_load_state"
	DiagDurationUnavailable Key = "diag_duration_unavailable"
	DiagMPRISLost           Key = "diag_mpris_lost"
	DiagMPRISUnavailable    Key = "diag_mpris_unavailable"
	DiagMPRISStop           Key = "diag_mpris_stop"
	DiagTUI                 Key = "diag_tui"
	DiagSaveState           Key = "diag_save_state"
)

var keys = []Key{
	Library, Tracks, Queue, NowPlaying, Stopped, Playing, Paused, NothingPlaying, UnknownArtist,
	EmptyLibrary, NoLibraryMatches, LibraryEmpty, EmptyScope, EmptyAlbum, EmptyArtist, NoTrackMatches,
	NoMusicFound, EmptyQueue, TerminalTooSmall, SearchPlaceholder, ResultsOne, ResultsOther,
	UnavailableOne, UnavailableOther, DurationsProgress, DurationsSummaryOne, DurationsSummaryOther,
	Volume, Mute, Shuffle, Repeat, On, Off, All, One, ErrorPrefix, ScopeAll, ScopeNamed,
	FilterContext, FilterBadge, MuteBadge, ShuffleBadge, RepeatBadge, QueueSelected, QueueNow,
	StatusLibraryScanFailed, StatusLoadedOne, StatusLoadedOther, StatusRescanFailed, StatusRescannedOne,
	StatusRescannedOther, StatusScopeAll, StatusScopeEmpty, StatusScopeAlbum, StatusScopeArtist,
	StatusMovedUp, StatusMovedDown, StatusRemovedQueue, StatusQueueCleared, StatusFilterCleared,
	StatusFilter, StatusVolume, StatusNoVisibleTracks, StatusStopped, StatusEndQueue, StatusFailedAdvance,
	StatusPlaying, StatusShuffleOn, StatusShuffleOff, StatusRepeatAll, StatusRepeatOne, StatusRepeatOff,
	StatusMuted, StatusUnmuted, StatusMutedVolume, StatusFailedLoad, StatusFailedStart, StatusFailedPrepare,
	StatusFailedDecode, StatusFailedStop, StatusPaused, StatusResumed, ShortcutLibrary, ShortcutTracks,
	ShortcutQueue, ShortcutGlobal, HelpTitle, HelpNavigation, HelpPlayback, HelpLibrarySearch, HelpQueue,
	HelpApplication, HelpNavMove, HelpNavOpen, HelpNavPanel, HelpNavChoose, HelpNavSelect, HelpNavSeek,
	HelpPlayPause, HelpStop, HelpNextPrev, HelpVolume, HelpMute, HelpShuffleRepeat, HelpRescan, HelpSearch,
	HelpClearFilter, HelpQueueUp, HelpQueueDown, HelpQueueRemove, HelpClose, HelpQuit, DiagLoadConfig,
	DiagInitAudio, DiagLoadState, DiagDurationUnavailable, DiagMPRISLost, DiagMPRISUnavailable,
	DiagMPRISStop, DiagTUI, DiagSaveState,
}

// Keys returns all catalogue keys in stable order.
func Keys() []Key { return append([]Key(nil), keys...) }

// Translator translates messages for one resolved language.
type Translator struct {
	language Language
	catalog  map[Key]string
}

// New returns a translator. Auto and unsupported values resolve to English.
func New(language Language) Translator {
	if language == Auto || !language.Valid() {
		language = English
	}
	return Translator{language: language, catalog: catalogues[language]}
}

// Language returns the translator's resolved language.
func (t Translator) Language() Language { return t.language }

// T translates and formats a phrase, falling back to English per key.
func (t Translator) T(key Key, args ...any) string {
	message := t.catalog[key]
	if message == "" {
		message = catalogues[English][key]
	}
	if message == "" {
		return string(key)
	}
	return fmt.Sprintf(message, args...)
}

// N chooses a complete singular or plural phrase based on count.
func (t Translator) N(count int, singular, plural Key, args ...any) string {
	if count == 1 {
		return t.T(singular, args...)
	}
	return t.T(plural, args...)
}

// Resolve selects a language once using config, then LC_ALL, LC_MESSAGES and LANG.
func Resolve(configured Language, getenv func(string) string) Language {
	if configured != Auto {
		if configured.Valid() {
			return configured
		}
		return English
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		locale := getenv(name)
		if strings.TrimSpace(locale) == "" {
			continue
		}
		if language, ok := fromLocale(locale); ok {
			return language
		}
		return English
	}
	return English
}

func fromLocale(locale string) (Language, bool) {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "", false
	}
	normalized := strings.ToLower(locale)
	if normalized == "c" || normalized == "posix" {
		return English, true
	}
	if index := strings.IndexAny(normalized, "_.@-"); index >= 0 {
		normalized = normalized[:index]
	}
	language := Language(normalized)
	switch language {
	case English, French, Spanish, German:
		return language, true
	default:
		return "", false
	}
}
