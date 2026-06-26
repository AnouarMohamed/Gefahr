package proxy

import (
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestRateLimiterPrunesExpiredKeysWhenFull(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := newRateLimiter(config.RateLimit{Requests: 1, Window: config.Duration(time.Second), MaxKeys: 1})
	limiter.now = func() time.Time { return now }
	if allowed, _ := limiter.Allow("old"); !allowed {
		t.Fatal("first key was rejected")
	}

	now = now.Add(2 * time.Second)
	if allowed, _ := limiter.Allow("new"); !allowed {
		t.Fatal("new key was rejected after expired key should have been pruned")
	}
	if _, ok := limiter.buckets["old"]; ok {
		t.Fatal("expired key was not pruned")
	}
}

func TestRateLimiterRejectsNewKeysWhenFull(t *testing.T) {
	limiter := newRateLimiter(config.RateLimit{Requests: 10, Window: config.Duration(time.Minute), MaxKeys: 1})
	if allowed, _ := limiter.Allow("one"); !allowed {
		t.Fatal("first key was rejected")
	}
	if allowed, retryAfter := limiter.Allow("two"); allowed || retryAfter <= 0 {
		t.Fatalf("second key allowed=%t retry_after=%s", allowed, retryAfter)
	}
}
