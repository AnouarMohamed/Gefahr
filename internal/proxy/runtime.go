package proxy

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/anrorg/gefahr/internal/backend"
	"github.com/anrorg/gefahr/internal/config"
)

// BackendObserver receives bounded backend gauges and transition updates.
type BackendObserver interface {
	SetBackendHealth(pool, backend string, healthy bool)
	SetBackendActive(pool, backend string, active int64)
}

// Dynamic atomically switches complete proxy runtime snapshots during reload.
type Dynamic struct{ current atomic.Pointer[Handler] }

// NewDynamic creates a dynamic handler with an initial runtime.
func NewDynamic(initial *Handler) *Dynamic { d := &Dynamic{}; d.current.Store(initial); return d }

// Swap publishes next for new requests; existing requests retain their handler.
func (d *Dynamic) Swap(next *Handler) { d.current.Store(next) }

// ServeHTTP delegates to the currently published runtime.
func (d *Dynamic) ServeHTTP(w http.ResponseWriter, r *http.Request) { d.current.Load().ServeHTTP(w, r) }

// Current returns the active concrete runtime snapshot.
func (d *Dynamic) Current() *Handler { return d.current.Load() }

// Ready reports readiness of the active runtime snapshot.
func (d *Dynamic) Ready() bool { return d.current.Load().Ready() }

// Ready reports whether every configured pool has at least one healthy backend.
func (h *Handler) Ready() bool {
	if len(h.pools) == 0 {
		return false
	}
	for _, pool := range h.pools {
		healthy := false
		for _, candidate := range pool.backends {
			if candidate.Alive() {
				healthy = true
				break
			}
		}
		if !healthy {
			return false
		}
	}
	return true
}

// InheritBackendHealth preserves eligibility for unchanged backends across a
// configuration reload. Changed names or URLs intentionally start fresh.
func (h *Handler) InheritBackendHealth(previous *Handler) {
	if previous == nil {
		return
	}
	for poolName, pool := range h.pools {
		oldPool := previous.pools[poolName]
		if oldPool == nil {
			continue
		}
		for _, candidate := range pool.backends {
			for _, old := range oldPool.backends {
				if candidate.Name() == old.Name() && candidate.URL().String() == old.URL().String() {
					candidate.SetAlive(old.Alive())
					break
				}
			}
		}
	}
}

// StartHealthChecks starts one active checker per pool and returns immediately.
func (h *Handler) StartHealthChecks(ctx context.Context, cfg config.Config) {
	observer, _ := h.observer.(BackendObserver)
	for poolName, pool := range h.pools {
		policy := cfg.Pools[poolName].Health
		if observer != nil {
			for _, candidate := range pool.backends {
				observer.SetBackendHealth(poolName, candidate.Name(), candidate.Alive())
			}
		}
		checker := &backend.Checker{
			Backends: pool.backends,
			Client:   newHealthClient(pool.transport),
			Policy:   backend.HealthPolicy{Path: policy.Path, Interval: policy.Interval.Value(), Timeout: policy.Timeout.Value(), HealthyThreshold: policy.HealthyThreshold, UnhealthyThreshold: policy.UnhealthyThreshold},
		}
		if observer != nil {
			name := poolName
			checker.OnChange = func(candidate *backend.Backend, healthy bool) {
				observer.SetBackendHealth(name, candidate.Name(), healthy)
			}
		}
		go checker.Run(ctx)
	}
}

func newHealthClient(transport http.RoundTripper) *http.Client {
	return &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func (h *Handler) acquire(pool string, candidate *backend.Backend) func() {
	release := candidate.Acquire()
	observer, _ := h.observer.(BackendObserver)
	if observer != nil {
		observer.SetBackendActive(pool, candidate.Name(), candidate.Active())
	}
	return func() {
		release()
		if observer != nil {
			observer.SetBackendActive(pool, candidate.Name(), candidate.Active())
		}
	}
}
