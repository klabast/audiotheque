package auth

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Now()

	t.Run("blocks a key once the limit is reached", func(t *testing.T) {
		l := newRateLimiter(3, time.Minute)
		for i := 0; i < 3; i++ {
			if !l.Allow(now, "ip:1.2.3.4") {
				t.Fatalf("attempt %d was blocked before the limit", i)
			}
			l.RecordFailure(now, "ip:1.2.3.4")
		}
		if l.Allow(now, "ip:1.2.3.4") {
			t.Error("expected the 4th attempt to be blocked")
		}
	})

	t.Run("keys are independent buckets", func(t *testing.T) {
		l := newRateLimiter(1, time.Minute)
		l.RecordFailure(now, "ip:1.2.3.4", "user:alice")

		if l.Allow(now, "ip:5.6.7.8", "user:bob") {
			// bob from a fresh IP is untouched
		} else {
			t.Error("an unrelated key was blocked")
		}
		if l.Allow(now, "ip:5.6.7.8", "user:alice") {
			t.Error("expected the exhausted username bucket to block a fresh IP")
		}
	})

	t.Run("failures age out of the window", func(t *testing.T) {
		l := newRateLimiter(1, time.Minute)
		l.RecordFailure(now, "ip:1.2.3.4")
		if l.Allow(now, "ip:1.2.3.4") {
			t.Fatal("expected to be blocked inside the window")
		}
		if !l.Allow(now.Add(2*time.Minute), "ip:1.2.3.4") {
			t.Error("expected the bucket to age out after the window")
		}
	})

	t.Run("success clears the bucket", func(t *testing.T) {
		l := newRateLimiter(1, time.Minute)
		l.RecordFailure(now, "ip:1.2.3.4")
		l.Reset("ip:1.2.3.4")
		if !l.Allow(now, "ip:1.2.3.4") {
			t.Error("expected a successful attempt to clear earlier failures")
		}
	})

	t.Run("empty keys are ignored", func(t *testing.T) {
		l := newRateLimiter(1, time.Minute)
		l.RecordFailure(now, "")
		if !l.Allow(now, "") {
			t.Error("an empty key must never block")
		}
	})
}
