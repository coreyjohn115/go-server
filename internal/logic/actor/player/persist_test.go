package playeractor

import (
	"context"
	"errors"
	"testing"
	"time"

	"due-server/internal/logic/store/playerstore"
)

type fakeTimer struct{}

func (*fakeTimer) Stop() bool { return true }

type fakeStore struct {
	err      error
	writes   int
	expected int64
}

func (s *fakeStore) Save(_ context.Context, _ *playerstore.Player, expected int64) error {
	s.writes++
	s.expected = expected
	return s.err
}

func TestSaveWindowAndRetry(t *testing.T) {
	s := &fakeStore{err: errors.New("database unavailable")}
	a := &Processor{store: s, data: &playerstore.Player{UID: 1, Version: 1}, savedVersion: 1}
	var pending func()
	scheduled := 0
	a.schedule = func(d time.Duration, f func()) (saveTimer, error) {
		if d != saveDelay {
			t.Fatalf("unexpected delay %s", d)
		}
		scheduled++
		pending = f
		return &fakeTimer{}, nil
	}
	for i := 0; i < 10; i++ {
		a.touch()
	}
	if scheduled != 1 {
		t.Fatalf("continuous changes postponed save: %d timers", scheduled)
	}
	pending()
	if !a.dirty || scheduled != 2 || s.expected != 1 {
		t.Fatal("failed save did not retain dirty data and schedule retry")
	}
	s.err = nil
	pending()
	if a.dirty || a.savedVersion != 11 || s.writes != 2 {
		t.Fatal("retry did not commit latest version")
	}
	a.touch()
	pending()
	if s.expected != 11 {
		t.Fatalf("next save expected version %d, want 11", s.expected)
	}
}

func TestConflictStopsRetries(t *testing.T) {
	s := &fakeStore{err: playerstore.ErrVersionConflict}
	a := &Processor{store: s, data: &playerstore.Player{Version: 2}, savedVersion: 1, dirty: true}
	a.schedule = func(time.Duration, func()) (saveTimer, error) {
		t.Fatal("conflict must not retry stale data")
		return nil, nil
	}
	a.persist()
	if !a.conflict || !a.dirty {
		t.Fatal("conflict must retain dirty state and block mutations")
	}
}

func TestDestroyDoesNotScheduleRetry(t *testing.T) {
	s := &fakeStore{err: errors.New("database unavailable")}
	a := &Processor{store: s, data: &playerstore.Player{Version: 2}, savedVersion: 1, dirty: true}
	a.schedule = func(time.Duration, func()) (saveTimer, error) {
		t.Fatal("destroy must not schedule timers")
		return nil, nil
	}
	a.Destroy()
	if s.writes != 1 || !a.dirty {
		t.Fatal("destroy must attempt final save")
	}
}
