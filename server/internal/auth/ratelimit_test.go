package auth

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Now()

	t.Run("blocks a key once the limit is reached", func(t *testing.T) {
		l := newRateLimiter(3, 3, time.Minute)
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
		l := newRateLimiter(1, 1, time.Minute)
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
		l := newRateLimiter(1, 1, time.Minute)
		l.RecordFailure(now, "ip:1.2.3.4")
		if l.Allow(now, "ip:1.2.3.4") {
			t.Fatal("expected to be blocked inside the window")
		}
		if !l.Allow(now.Add(2*time.Minute), "ip:1.2.3.4") {
			t.Error("expected the bucket to age out after the window")
		}
	})

	t.Run("success clears the bucket", func(t *testing.T) {
		l := newRateLimiter(1, 1, time.Minute)
		l.RecordFailure(now, "ip:1.2.3.4")
		l.Reset("ip:1.2.3.4")
		if !l.Allow(now, "ip:1.2.3.4") {
			t.Error("expected a successful attempt to clear earlier failures")
		}
	})

	t.Run("empty keys are ignored", func(t *testing.T) {
		l := newRateLimiter(1, 1, time.Minute)
		l.RecordFailure(now, "")
		if !l.Allow(now, "") {
			t.Error("an empty key must never block")
		}
	})
}

// TestRateLimiter_IPBucketIsLooserThanAccountBucket pins the asymmetry.
//
// The per-account bucket is what protects an account. The per-IP bucket
// aggregates every account behind one address — a household NAT, a reverse
// proxy — so holding it to the same number locks out a whole home because one
// person mistyped. Reset-code guessing keeps the tight bound on its IP bucket,
// because there it is the only bound there is.
func TestRateLimiter_IPBucketIsLooserThanAccountBucket(t *testing.T) {
	now := time.Now()

	t.Run("password entry gets the widened IP bucket", func(t *testing.T) {
		l := newRateLimiter(2, 5, time.Minute)
		key := scopeCredential + "|ip:10.0.0.1"

		// The account limit is 2, so anything past 2 proves the IP bucket is
		// being measured against the wider limit instead.
		for i := 0; i < 5; i++ {
			if !l.Allow(now, key) {
				t.Fatalf("blocked after %d failures, want the widened limit of 5", i)
			}
			l.RecordFailure(now, key)
		}
		if l.Allow(now, key) {
			t.Error("still allowed after 5 failures, want blocked")
		}
	})

	t.Run("reset-code guessing keeps the tight IP bucket", func(t *testing.T) {
		l := newRateLimiter(2, 5, time.Minute)
		key := scopeResetConfirm + "|ip:10.0.0.1"

		for i := 0; i < 2; i++ {
			l.RecordFailure(now, key)
		}
		if l.Allow(now, key) {
			t.Error("still allowed after 2 failures, want the tight limit to block")
		}
	})

	t.Run("account bucket stays tight regardless of scope", func(t *testing.T) {
		l := newRateLimiter(2, 5, time.Minute)
		key := scopeCredential + "|user:alice"

		for i := 0; i < 2; i++ {
			l.RecordFailure(now, key)
		}
		if l.Allow(now, key) {
			t.Error("still allowed after 2 failures, want blocked")
		}
	})
}
