package cooldown

import (
	"sync"
	"time"
)

type Tracker struct {
	mu   sync.Mutex
	last map[uint64]map[string]time.Time
}

func NewTracker() *Tracker {
	return &Tracker{
		last: map[uint64]map[string]time.Time{},
	}
}

// Take checks whether (userID, key) is on cooldown. If allowed, it records the new usage time.
// Returns remaining duration if blocked.
func (c *Tracker) Take(userID uint64, key string, d time.Duration, now time.Time) (time.Duration, bool) {
	if c == nil || d <= 0 || userID == 0 || key == "" {
		return 0, true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	m, ok := c.last[userID]
	if !ok {
		m = map[string]time.Time{}
		c.last[userID] = m
	}

	prev := m[key]
	if !prev.IsZero() {
		next := prev.Add(d)
		if now.Before(next) {
			return next.Sub(now), false
		}
	}

	m[key] = now
	return 0, true
}
