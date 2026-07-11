package main

import (
	"math"
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// limiter is a per-key token bucket. burst tokens refill linearly over window.
// ponytail: naive per-IP map, fine at single-gateway scale; swap for a sharded/LRU
// limiter only if IP cardinality ever explodes.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	burst   float64
	rate    float64 // tokens per second
	window  time.Duration
}

func newLimiter(burst int, window time.Duration) *limiter {
	return &limiter{
		buckets: make(map[string]*bucket),
		burst:   float64(burst),
		rate:    float64(burst) / window.Seconds(),
		window:  window,
	}
}

func (l *limiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}

	b.tokens = math.Min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

func (l *limiter) sweep(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, b := range l.buckets {
		if now.Sub(b.last) > 2*l.window {
			delete(l.buckets, k)
		}
	}
}

func (l *limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
