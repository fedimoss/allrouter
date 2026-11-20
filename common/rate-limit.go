package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	reservations       map[string]int
	windows            map[string]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if l.store == nil {
		l.store = make(map[string]*[]int64)
		l.windows = make(map[string]int64)
		l.expirationDuration = expirationDuration
		if expirationDuration > 0 {
			go l.clearExpiredItems()
		}
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.deleteExpiredItems(time.Now())
	}
}

func (l *InMemoryRateLimiter) deleteExpiredItems(now time.Time) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	for key, queue := range l.store {
		if l.reservations[key] > 0 {
			continue
		}
		if len(*queue) == 0 || now.Unix()-(*queue)[len(*queue)-1] >= l.windows[key] {
			delete(l.store, key)
			delete(l.windows, key)
		}
	}
}

// pruneLocked removes the entire expired prefix, including after a limit is
// lowered. Retention belongs to each key, not the shared cleanup interval.
func (l *InMemoryRateLimiter) pruneLocked(key string, duration, now int64) *[]int64 {
	if l.store == nil {
		l.store = make(map[string]*[]int64)
	}
	if l.windows == nil {
		l.windows = make(map[string]int64)
	}
	if duration > l.windows[key] {
		l.windows[key] = duration
	}
	queue := l.store[key]
	if queue == nil {
		queue = &[]int64{}
		l.store[key] = queue
	}
	for len(*queue) > 0 && now-(*queue)[0] >= duration {
		*queue = (*queue)[1:]
	}
	return queue
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if maxRequestNum <= 0 || duration <= 0 {
		return false
	}
	now := time.Now().Unix()
	queue := l.pruneLocked(key, duration, now)
	if len(*queue) >= maxRequestNum {
		return false
	}
	*queue = append(*queue, now)
	return true
}
