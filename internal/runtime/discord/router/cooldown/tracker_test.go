package cooldown

import (
	"testing"
	"time"
)

func TestTrackerBlocksUntilCooldownExpires(t *testing.T) {
	t.Parallel()
	tracker := NewTracker()
	now := time.Unix(1_700_000_000, 0)
	if remaining, ok := tracker.Take(1, "ping", 5*time.Second, now); !ok || remaining != 0 {
		t.Fatalf("first take = (%s, %v)", remaining, ok)
	}
	if remaining, ok := tracker.Take(1, "ping", 5*time.Second, now.Add(2*time.Second)); ok || remaining != 3*time.Second {
		t.Fatalf("blocked take = (%s, %v)", remaining, ok)
	}
	if remaining, ok := tracker.Take(1, "ping", 5*time.Second, now.Add(5*time.Second)); !ok || remaining != 0 {
		t.Fatalf("expired take = (%s, %v)", remaining, ok)
	}
	if _, ok := tracker.Take(2, "ping", 5*time.Second, now); !ok {
		t.Fatal("different user should not share cooldown")
	}
	if _, ok := tracker.Take(1, "other", 5*time.Second, now); !ok {
		t.Fatal("different key should not share cooldown")
	}
}

func TestTrackerAllowsDisabledAndIncompleteKeys(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	var nilTracker *Tracker
	for _, test := range []struct {
		name    string
		tracker *Tracker
		userID  uint64
		key     string
		d       time.Duration
	}{
		{name: "nil tracker", tracker: nilTracker, userID: 1, key: "ping", d: time.Second},
		{name: "zero duration", tracker: NewTracker(), userID: 1, key: "ping"},
		{name: "zero user", tracker: NewTracker(), key: "ping", d: time.Second},
		{name: "empty key", tracker: NewTracker(), userID: 1, d: time.Second},
	} {
		if remaining, ok := test.tracker.Take(test.userID, test.key, test.d, now); !ok || remaining != 0 {
			t.Errorf("%s: take = (%s, %v)", test.name, remaining, ok)
		}
	}
}
