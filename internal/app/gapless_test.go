package app

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/kvitrvn/galdr/internal/player"
)

type gaplessSync struct {
	expected player.PlaybackToken
	next     *player.PreparedEntry
}

type fakeGaplessPlayer struct {
	*player.MockPlayer
	active        player.PlaybackToken
	prepared      *player.PreparedEntry
	events        chan player.PlaybackEvent
	syncs         []gaplessSync
	activateCalls int
}

func newFakeGaplessPlayer(mock *player.MockPlayer) *fakeGaplessPlayer {
	return &fakeGaplessPlayer{
		MockPlayer: mock,
		events:     make(chan player.PlaybackEvent, 32),
	}
}

func (p *fakeGaplessPlayer) LoadEntry(entry player.PreparedEntry) error {
	if err := p.MockPlayer.Load(entry.Path); err != nil {
		return err
	}
	p.active = entry.Token
	p.prepared = nil
	return nil
}

func (p *fakeGaplessPlayer) SyncNext(
	expected player.PlaybackToken,
	next *player.PreparedEntry,
) (player.PlaybackToken, error) {
	p.syncs = append(p.syncs, gaplessSync{expected: expected, next: copyPrepared(next)})
	if expected != p.active {
		return p.active, nil
	}
	p.prepared = copyPrepared(next)
	return p.active, nil
}

func (p *fakeGaplessPlayer) ActivateNext(expected player.PlaybackToken) (player.PlaybackToken, error) {
	p.activateCalls++
	if expected != p.active || p.prepared == nil {
		return p.active, nil
	}
	p.active = p.prepared.Token
	p.events <- player.PlaybackEvent{Kind: player.PlaybackStarted, Token: p.active}
	p.prepared = nil
	return expected, nil
}

func (p *fakeGaplessPlayer) PlaybackEvents() <-chan player.PlaybackEvent { return p.events }

func (p *fakeGaplessPlayer) Stop() error {
	p.active = 0
	p.prepared = nil
	return p.MockPlayer.Stop()
}

func copyPrepared(entry *player.PreparedEntry) *player.PreparedEntry {
	if entry == nil {
		return nil
	}
	copy := *entry
	return &copy
}

func TestGaplessAutomaticTransitionAndStaleEvent(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	gapless := newFakeGaplessPlayer(mock)
	a.player = gapless
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	initialToken := a.activeToken
	prepared := *gapless.prepared
	gapless.active = prepared.Token
	if err := a.HandlePlaybackEvent(player.PlaybackEvent{Kind: player.PlaybackStarted, Token: prepared.Token}); err != nil {
		t.Fatal(err)
	}
	if a.Queue().Index() != 1 || filepath.Base(a.Current().Path) != "02.mp3" {
		t.Fatalf("automatic transition = index %d current %v", a.Queue().Index(), a.Current())
	}
	if gapless.prepared == nil || filepath.Base(gapless.prepared.Path) != "03.mp3" {
		t.Fatalf("prepared successor after transition = %#v", gapless.prepared)
	}
	if err := a.HandlePlaybackEvent(player.PlaybackEvent{Kind: player.PlaybackStarted, Token: initialToken}); err != nil {
		t.Fatal(err)
	}
	if a.Queue().Index() != 1 {
		t.Fatal("stale transition moved the queue backwards")
	}
}

func TestGaplessManualNextUsesPreparedAndPreviousReplaces(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	gapless := newFakeGaplessPlayer(mock)
	a.player = gapless
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	if err := a.Next(); err != nil {
		t.Fatal(err)
	}
	event := <-gapless.events
	if err := a.HandlePlaybackEvent(event); err != nil {
		t.Fatal(err)
	}
	if gapless.activateCalls != 1 || a.Queue().Index() != 1 {
		t.Fatalf("manual next = activate calls %d index %d", gapless.activateCalls, a.Queue().Index())
	}
	loads := len(mock.LoadCalls)
	if err := a.Previous(); err != nil {
		t.Fatal(err)
	}
	if a.Queue().Index() != 0 || len(mock.LoadCalls) != loads+1 {
		t.Fatalf("previous did not replace: index %d loads %d", a.Queue().Index(), len(mock.LoadCalls))
	}
}

func TestGaplessRepeatedNextWhileTransitionPendingActivatesOnce(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	gapless := newFakeGaplessPlayer(mock)
	a.player = gapless
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	if err := a.Next(); err != nil {
		t.Fatal(err)
	}
	if err := a.Next(); err != nil {
		t.Fatal(err)
	}
	if gapless.activateCalls != 1 {
		t.Fatalf("activate calls = %d, want 1", gapless.activateCalls)
	}
}

func TestGaplessQueueEditsReconcilePreparedSuccessor(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	gapless := newFakeGaplessPlayer(mock)
	a.player = gapless
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	if !a.MoveQueueDown(1) {
		t.Fatal("MoveQueueDown failed")
	}
	if gapless.prepared == nil || filepath.Base(gapless.prepared.Path) != "03.mp3" {
		t.Fatalf("prepared after move = %#v", gapless.prepared)
	}
	if !a.RemoveFromQueue(1) {
		t.Fatal("RemoveFromQueue failed")
	}
	if gapless.prepared == nil || filepath.Base(gapless.prepared.Path) != "02.mp3" {
		t.Fatalf("prepared after remove = %#v", gapless.prepared)
	}
	a.ClearQueue()
	if gapless.prepared != nil || a.Queue().Len() != 1 {
		t.Fatalf("clear left successor %#v and queue length %d", gapless.prepared, a.Queue().Len())
	}
}

func TestSuccessorResolutionRepeatModes(t *testing.T) {
	a, _, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	entries := a.Queue().Entries()
	tests := []struct {
		name   string
		mode   RepeatMode
		after  int
		want   int
		nilOut bool
	}{
		{name: "off middle", mode: RepeatOff, after: 1, want: 2},
		{name: "off tail", mode: RepeatOff, after: 2, nilOut: true},
		{name: "all tail wraps", mode: RepeatAll, after: 2, want: 0},
		{name: "one repeats occurrence", mode: RepeatOne, after: 1, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a.repeat = tt.mode
			got := a.automaticSuccessor(entries[tt.after].ID)
			if tt.nilOut {
				if got != nil {
					t.Fatalf("successor = %#v, want nil", got)
				}
				return
			}
			if got == nil || got.ID != entries[tt.want].ID {
				t.Fatalf("successor = %#v, want entry %d", got, entries[tt.want].ID)
			}
		})
	}
}

func TestGaplessFailuresAreBoundedByQueueOccurrences(t *testing.T) {
	a, mock, _ := testApp(t, "A/X/01.mp3", "A/X/02.mp3", "A/X/03.mp3")
	gapless := newFakeGaplessPlayer(mock)
	a.player = gapless
	a.repeat = RepeatAll
	if err := a.PlaySelected(); err != nil {
		t.Fatal(err)
	}
	decodeErr := errors.New("decode failed")
	for range a.Queue().Len() {
		token := a.activeToken
		_ = a.HandlePlaybackEvent(player.PlaybackEvent{
			Kind:  player.PlaybackFailed,
			Token: token,
			Err:   decodeErr,
		})
	}
	if a.Current() != nil || a.State() != player.StateStopped {
		t.Fatalf("all-bad queue did not stop: current %#v state %v", a.Current(), a.State())
	}
	if got := len(mock.LoadCalls); got != a.Queue().Len() {
		t.Fatalf("load attempts = %d, want one per occurrence (%d)", got, a.Queue().Len())
	}
}
