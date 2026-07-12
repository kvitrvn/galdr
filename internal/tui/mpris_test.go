package tui

import (
	"testing"
	"time"

	"github.com/kvitrvn/galdr/internal/app"
)

type recordingPlaybackPublisher struct {
	snapshots []app.PlaybackSnapshot
	seeked    []time.Duration
}

func (p *recordingPlaybackPublisher) Publish(snapshot app.PlaybackSnapshot) {
	p.snapshots = append(p.snapshots, snapshot)
}

func (p *recordingPlaybackPublisher) Seeked(position time.Duration) {
	p.seeked = append(p.seeked, position)
}

func TestModelAppliesMPRISRequestsOnUpdate(t *testing.T) {
	m := newTestModel(t, 2)
	publisher := &recordingPlaybackPublisher{}
	m.SetPlaybackPublisher(publisher)

	m.Update(app.NewPlaybackRequest(app.PlaybackCommand{
		Action: app.PlaybackActionSetVolume,
		Volume: 37,
	}))
	if m.app.Volume() != 37 {
		t.Fatalf("volume = %d, want 37", m.app.Volume())
	}
	m.Update(app.NewPlaybackRequest(app.PlaybackCommand{
		Action:  app.PlaybackActionSetShuffle,
		Shuffle: true,
	}))
	if !m.app.Shuffle() {
		t.Fatal("shuffle was not enabled")
	}
	m.Update(app.NewPlaybackRequest(app.PlaybackCommand{
		Action: app.PlaybackActionSetRepeat,
		Repeat: app.RepeatOne,
	}))
	if m.app.Repeat() != app.RepeatOne {
		t.Fatalf("repeat = %v, want one", m.app.Repeat())
	}
	if len(publisher.snapshots) < 3 {
		t.Fatalf("published snapshots = %d, want at least 3", len(publisher.snapshots))
	}
}

func TestModelPublishesMPRISSeekAfterApplyingIt(t *testing.T) {
	m := newTestModel(t, 1)
	publisher := &recordingPlaybackPublisher{}
	m.SetPlaybackPublisher(publisher)
	_ = sendKey(t, m, "enter")
	snapshot := m.app.PlaybackSnapshot()

	m.Update(app.NewPlaybackRequest(app.PlaybackCommand{
		Action:   app.PlaybackActionSetPosition,
		TrackID:  snapshot.TrackID,
		Position: 3 * time.Second,
	}))
	if got := m.app.Position(); got != 3*time.Second {
		t.Fatalf("position = %v, want 3s", got)
	}
	if len(publisher.seeked) != 1 || publisher.seeked[0] != 3*time.Second {
		t.Fatalf("Seeked = %v", publisher.seeked)
	}

	m.Update(app.NewPlaybackRequest(app.PlaybackCommand{
		Action:   app.PlaybackActionSetPosition,
		TrackID:  snapshot.TrackID + 1,
		Position: 5 * time.Second,
	}))
	if len(publisher.seeked) != 1 {
		t.Fatal("stale track id emitted Seeked")
	}
}
