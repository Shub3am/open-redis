// This file is the active half of key expiry: a background sweep that frees
// keys nobody reads again. Lazy expiry in lookup already hides expired keys
// from clients, so the sweep exists only to reclaim memory. It must not
// change what any command observes.
package store

import (
	"context"
	"time"
)

// Redis samples the same number of keys per round in its active expiry cycle.
const sweepSampleSize = 20

// RunActiveExpiry sweeps expired keys every interval until ctx is cancelled.
func (s *Store) RunActiveExpiry(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweepExpired()
		}
	}
}

// sweepExpired repeats sample rounds while more than a quarter of each sample
// had expired, the same threshold Redis uses to guess that many more keys are
// stale. The lock is released between rounds so clients are not starved while
// a large batch of keys expires.
func (s *Store) sweepExpired() {
	for {
		sampled, expired := s.sweepRound()
		if expired*4 <= sampled {
			return
		}
	}
}

func (s *Store) sweepRound() (sampled, expired int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	// Go randomises map iteration order, which gives a cheap random sample.
	for key, deadline := range s.expiresAt {
		if sampled == sweepSampleSize {
			break
		}
		sampled++
		if !now.Before(deadline) {
			s.remove(key)
			expired++
		}
	}
	return sampled, expired
}
