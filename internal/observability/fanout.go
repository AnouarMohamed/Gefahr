package observability

import (
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

// RequestObserver receives completed public request records.
type RequestObserver interface {
	ObserveRequest(requestID, route, method, path, backend string, status, attempts int, cacheResult string, duration time.Duration)
}

// RateLimitObserver receives configured route rate-limit decisions.
type RateLimitObserver interface {
	ObserveRateLimit(route, decision string)
}

// PolicyObserver receives configured route request-policy denials.
type PolicyObserver interface {
	ObservePolicyDeny(route, reason string)
}

// BackendObserver receives backend gauge updates.
type BackendObserver interface {
	SetBackendHealth(pool, backend string, healthy bool)
	SetBackendActive(pool, backend string, active int64)
}

// Fanout sends observations to independent logging and metrics consumers.
type Fanout struct {
	Requests []RequestObserver
	Backends []BackendObserver
}

// ObserveRequest forwards a completed request to every request observer.
func (f Fanout) ObserveRequest(requestID, route, method, path, backend string, status, attempts int, cacheResult string, duration time.Duration) {
	for _, observer := range f.Requests {
		observer.ObserveRequest(requestID, route, method, path, backend, status, attempts, cacheResult, duration)
	}
}

// ObserveRateLimit forwards rate-limit decisions to capable request observers.
func (f Fanout) ObserveRateLimit(route, decision string) {
	for _, observer := range f.Requests {
		if observer, ok := observer.(RateLimitObserver); ok {
			observer.ObserveRateLimit(route, decision)
		}
	}
}

// ObservePolicyDeny forwards request-policy denials to capable request observers.
func (f Fanout) ObservePolicyDeny(route, reason string) {
	for _, observer := range f.Requests {
		if observer, ok := observer.(PolicyObserver); ok {
			observer.ObservePolicyDeny(route, reason)
		}
	}
}

// SetBackendHealth forwards backend health state.
func (f Fanout) SetBackendHealth(pool, backend string, healthy bool) {
	for _, observer := range f.Backends {
		observer.SetBackendHealth(pool, backend, healthy)
	}
}

// SetBackendActive forwards backend active request state.
func (f Fanout) SetBackendActive(pool, backend string, active int64) {
	for _, observer := range f.Backends {
		observer.SetBackendActive(pool, backend, active)
	}
}

// ReconcileConfig forwards an accepted runtime configuration to consumers
// that retain configuration-derived state such as metric label sets.
func (f Fanout) ReconcileConfig(cfg config.Config) {
	for _, observer := range f.Requests {
		if reconciler, ok := observer.(interface{ ReconcileConfig(config.Config) }); ok {
			reconciler.ReconcileConfig(cfg)
		}
	}
}
