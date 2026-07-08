package metadata

import (
	"fmt"
	"time"

	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/meta"
)

// readFLAC extracts tags and duration from a FLAC file.
//
// The mewkiz/flac library parses the mandatory STREAMINFO block
// (which carries SampleRate and NSamples, enough to compute the
// duration) and the optional VORBIS_COMMENT block (which carries the
// tag map).
//
// The audio frames are not decoded; ParseFile stops once the last
// metadata block has been read.
func readFLAC(path string) (Tags, error) {
	stream, err := flac.ParseFile(path)
	if err != nil {
		return Tags{Format: "flac"}, fmt.Errorf("flac: %w", err)
	}
	defer stream.Close()

	out := Tags{Format: "flac"}
	if stream.Info != nil {
		if stream.Info.SampleRate > 0 && stream.Info.NSamples > 0 {
			out.Duration = time.Duration(stream.Info.NSamples) * time.Second / time.Duration(stream.Info.SampleRate)
		}
		for _, block := range stream.Blocks {
			vc, ok := block.Body.(*meta.VorbisComment)
			if !ok {
				continue
			}
			for _, kv := range vc.Tags {
				applyVorbisTag(&out, kv[0], kv[1])
			}
		}
	}
	return out, nil
}

// applyVorbisTag mutates out with the value of a single Vorbis
// comment key/value pair. Unknown keys are ignored.
//
// Year is parsed from the DATE field (commonly "YYYY" or
// "YYYY-MM-DD"). Track is parsed from TRACKNUMBER (commonly "N" or
// "N/M"; only N is kept).
func applyVorbisTag(out *Tags, key, value string) {
	switch key {
	case "TITLE":
		out.Title = value
	case "ARTIST":
		out.Artist = value
	case "ALBUM":
		out.Album = value
	case "DATE", "YEAR":
		if out.Year == 0 {
			if n, ok := parseLeadingInt(value); ok {
				out.Year = n
			}
		}
	case "TRACKNUMBER":
		if out.Track == 0 {
			if n, ok := parseLeadingInt(value); ok {
				out.Track = n
			}
		}
	}
}
