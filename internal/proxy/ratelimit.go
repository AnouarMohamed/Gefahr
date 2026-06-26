package proxy

import (
	"sync"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

const defaultRateLimitMaxKeys = 10000

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	maxKeys int
	now     func() time.Time
	buckets map[string]rateBucket
}

type rateBucket struct {
	start time.Time
	count int
}

func newRateLimiter(cfg config.RateLimit) *rateLimiter {
	maxKeys := cfg.MaxKeys
	if maxKeys == 0 {
		maxKeys = defaultRateLimitMaxKeys
	}
	return &rateLimiter{
		limit:   cfg.Requests,
		window:  cfg.Window.Value(),
		maxKeys: maxKeys,
		now:     time.Now,
		buckets: make(map[string]rateBucket),
	}
}

func (l *rateLimiter) Allow(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if bucket, ok := l.buckets[key]; ok {
		if now.Sub(bucket.start) < l.window {
			if bucket.count >= l.limit {
				return false, bucket.start.Add(l.window).Sub(now)
			}
			bucket.count++
			l.buckets[key] = bucket
			return true, 0
		}
		l.buckets[key] = rateBucket{start: now, count: 1}
		return true, 0
	}

	if len(l.buckets) >= l.maxKeys {
		l.pruneExpired(now)
		if len(l.buckets) >= l.maxKeys {
			return false, time.Second
		}
	}
	l.buckets[key] = rateBucket{start: now, count: 1}
	return true, 0
}

func (l *rateLimiter) pruneExpired(now time.Time) {
	for key, bucket := range l.buckets {
		if now.Sub(bucket.start) >= l.window {
			delete(l.buckets, key)
		}
	}
}
