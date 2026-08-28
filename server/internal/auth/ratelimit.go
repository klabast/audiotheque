package auth

import (
	"sync"
	"time"
)

// Credential endpoints (login, reset-confirm, verify-password) are the only
// places where guessing pays off, and MinPasswordLength is deliberately 1, so
// unbounded attempts are an open guessing oracle. Each attempt also costs an
// Argon2id hash at 64MB, which makes unlimited login a cheap way to exhaust
// memory. Failures are what's counted; a successful attempt clears the
// buckets, so ordinary use never runs into the limit.
const (
	CredentialFailureLimit  = 10
	CredentialFailureWindow = 15 * time.Minute
)

// rateLimiterMaxKeys caps memory for the in-process bucket map. Reaching it
// triggers a sweep of buckets that have fully aged out.
const rateLimiterMaxKeys = 4096

// rateLimiter is a per-key sliding-window counter of failed attempts. Keys
// are independent buckets — callers pass one per dimension (client IP,
// username), and any exhausted bucket blocks the attempt.
type rateLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	failures map[string][]time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:    limit,
		window:   window,
		failures: make(map[string][]time.Time),
	}
}

// Allow reports whether an attempt against all of keys may proceed.
func (l *rateLimiter) Allow(now time.Time, keys ...string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	for _, key := range keys {
		if key == "" {
			continue
		}
		recent := withinWindow(l.failures[key], cutoff)
		if len(recent) == 0 {
			delete(l.failures, key)
		} else {
			l.failures[key] = recent
		}
		if len(recent) >= l.limit {
			return false
		}
	}
	return true
}

// RecordFailure counts one failed attempt against every key.
func (l *rateLimiter) RecordFailure(now time.Time, keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	if len(l.failures) >= rateLimiterMaxKeys {
		l.sweepLocked(cutoff)
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		l.failures[key] = append(withinWindow(l.failures[key], cutoff), now)
	}
}

// Reset clears the buckets for keys — called after a successful attempt so a
// legitimate user is never locked out by their own earlier typos.
func (l *rateLimiter) Reset(keys ...string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, key := range keys {
		delete(l.failures, key)
	}
}

func (l *rateLimiter) sweepLocked(cutoff time.Time) {
	for key, times := range l.failures {
		if recent := withinWindow(times, cutoff); len(recent) == 0 {
			delete(l.failures, key)
		} else {
			l.failures[key] = recent
		}
	}
}

func withinWindow(times []time.Time, cutoff time.Time) []time.Time {
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	return kept
}
