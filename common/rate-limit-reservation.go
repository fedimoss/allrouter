package common

import (
	"sync"
	"time"
)

// RateLimitReservation counts an in-flight request until its outcome is known.
// Failed requests release their slot without consuming successful-request quota.
type RateLimitReservation struct {
	limiter  *InMemoryRateLimiter
	key      string
	duration int64
	once     sync.Once
}

func (l *InMemoryRateLimiter) Reserve(key string, maxRequests int, duration int64) *RateLimitReservation {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if maxRequests <= 0 || duration <= 0 {
		return nil
	}
	queue := l.pruneLocked(key, duration, time.Now().Unix())
	if len(*queue)+l.reservations[key] >= maxRequests {
		return nil
	}
	if l.reservations == nil {
		l.reservations = make(map[string]int)
	}
	l.reservations[key]++
	return &RateLimitReservation{limiter: l, key: key, duration: duration}
}

// Complete is safe to call repeatedly and on a nil reservation, so a deferred
// cancellation can release a slot if the downstream handler panics.
func (r *RateLimitReservation) Complete(success bool) {
	if r == nil {
		return
	}
	r.once.Do(func() {
		l := r.limiter
		l.mutex.Lock()
		defer l.mutex.Unlock()
		l.reservations[r.key]--
		if l.reservations[r.key] == 0 {
			delete(l.reservations, r.key)
		}
		if success {
			now := time.Now().Unix()
			// Another request may have observed a longer live configuration while
			// this one was running. Completion must retain that newer history.
			duration := max(r.duration, l.windows[r.key])
			queue := l.pruneLocked(r.key, duration, now)
			*queue = append(*queue, now)
		}
	})
}
