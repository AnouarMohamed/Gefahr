// Package metrics defines GoProxy's bounded-label Prometheus text contract.
package metrics

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

type requestKey struct {
	route, method string
	status        int
}
type cacheKey struct{ route, result string }
type backendKey struct{ pool, backend string }
type rateLimitKey struct{ route, decision string }
type policyDenyKey struct{ route, reason string }

// Metrics stores counters and gauges whose labels come only from validated
// configuration or bounded enums, preventing attacker-controlled cardinality.
type Metrics struct {
	mu              sync.Mutex
	requests        map[requestKey]uint64
	durationCount   map[string]uint64
	durationSum     map[string]float64
	cache           map[cacheKey]uint64
	rateLimits      map[rateLimitKey]uint64
	policyDenials   map[policyDenyKey]uint64
	retries         map[string]uint64
	health          map[backendKey]float64
	active          map[backendKey]int64
	allowedRoutes   map[string]bool
	allowedBackends map[backendKey]bool
	reconciled      bool
}

// New creates an empty metrics registry.
func New() *Metrics {
	return &Metrics{requests: map[requestKey]uint64{}, durationCount: map[string]uint64{}, durationSum: map[string]float64{}, cache: map[cacheKey]uint64{}, rateLimits: map[rateLimitKey]uint64{}, policyDenials: map[policyDenyKey]uint64{}, retries: map[string]uint64{}, health: map[backendKey]float64{}, active: map[backendKey]int64{}, allowedRoutes: map[string]bool{}, allowedBackends: map[backendKey]bool{}}
}

// Handler exposes metrics in the Prometheus text exposition format.
func (m *Metrics) Handler() http.Handler { return http.HandlerFunc(m.serveHTTP) }

// ObserveRequest records one completed public request.
func (m *Metrics) ObserveRequest(_, route, method, _, _ string, status, attempts int, cacheResult string, duration time.Duration) {
	method = boundedMethod(method)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reconciled && !m.allowedRoutes[route] && route != "unmatched" {
		route = "retired"
	}
	m.requests[requestKey{route, method, status}]++
	m.durationCount[route]++
	m.durationSum[route] += duration.Seconds()
	if cacheResult != "" {
		m.cache[cacheKey{route, cacheResult}]++
	}
	if attempts > 1 {
		m.retries[route] += uint64(attempts - 1)
	}
}

func boundedMethod(method string) string {
	switch method {
	case http.MethodConnect, http.MethodDelete, http.MethodGet, http.MethodHead,
		http.MethodOptions, http.MethodPatch, http.MethodPost, http.MethodPut, http.MethodTrace:
		return method
	default:
		return "OTHER"
	}
}

// ObserveRateLimit records one configured route rate-limit decision.
func (m *Metrics) ObserveRateLimit(route, decision string) {
	decision = boundedRateLimitDecision(decision)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reconciled && !m.allowedRoutes[route] && route != "unmatched" {
		route = "retired"
	}
	m.rateLimits[rateLimitKey{route, decision}]++
}

func boundedRateLimitDecision(decision string) string {
	switch decision {
	case "allowed", "limited":
		return decision
	default:
		return "other"
	}
}

// ObservePolicyDeny records one configured route request-policy denial.
func (m *Metrics) ObservePolicyDeny(route, reason string) {
	reason = boundedPolicyReason(reason)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reconciled && !m.allowedRoutes[route] && route != "unmatched" {
		route = "retired"
	}
	m.policyDenials[policyDenyKey{route, reason}]++
}

func boundedPolicyReason(reason string) string {
	switch reason {
	case "method_not_allowed", "path_denied", "required_header_missing", "header_denied", "query_too_large":
		return reason
	default:
		return "other"
	}
}

// SetBackendHealth updates the backend eligibility gauge.
func (m *Metrics) SetBackendHealth(pool, backend string, healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reconciled && !m.allowedBackends[backendKey{pool, backend}] {
		return
	}
	if healthy {
		m.health[backendKey{pool, backend}] = 1
	} else {
		m.health[backendKey{pool, backend}] = 0
	}
}

// SetBackendActive updates the backend active-request gauge.
func (m *Metrics) SetBackendActive(pool, backend string, active int64) {
	m.mu.Lock()
	if m.reconciled && !m.allowedBackends[backendKey{pool, backend}] {
		m.mu.Unlock()
		return
	}
	m.active[backendKey{pool, backend}] = active
	m.mu.Unlock()
}

// ReconcileConfig bounds metric labels across repeated runtime reloads.
func (m *Metrics) ReconcileConfig(cfg config.Config) {
	routes := make(map[string]bool, len(cfg.Routes))
	for _, route := range cfg.Routes {
		routes[route.Name] = true
	}
	backends := make(map[backendKey]bool)
	for poolName, pool := range cfg.Pools {
		for _, backend := range pool.Backends {
			backends[backendKey{poolName, backend.Name}] = true
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowedRoutes, m.allowedBackends, m.reconciled = routes, backends, true
	for key := range m.requests {
		if !routes[key.route] && key.route != "unmatched" && key.route != "retired" {
			delete(m.requests, key)
		}
	}
	for route := range m.durationCount {
		if !routes[route] && route != "unmatched" && route != "retired" {
			delete(m.durationCount, route)
			delete(m.durationSum, route)
		}
	}
	for key := range m.cache {
		if !routes[key.route] && key.route != "unmatched" && key.route != "retired" {
			delete(m.cache, key)
		}
	}
	for key := range m.rateLimits {
		if !routes[key.route] && key.route != "unmatched" && key.route != "retired" {
			delete(m.rateLimits, key)
		}
	}
	for key := range m.policyDenials {
		if !routes[key.route] && key.route != "unmatched" && key.route != "retired" {
			delete(m.policyDenials, key)
		}
	}
	for route := range m.retries {
		if !routes[route] && route != "unmatched" && route != "retired" {
			delete(m.retries, route)
		}
	}
	clear(m.health)
	clear(m.active)
}

func (m *Metrics) serveHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	lines := m.lines()
	m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
}

func (m *Metrics) lines() []string {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	lines := []string{
		"# HELP goproxy_requests_total Completed public requests.", "# TYPE goproxy_requests_total counter",
		"# HELP goproxy_request_duration_seconds Public request duration.", "# TYPE goproxy_request_duration_seconds summary",
		"# HELP goproxy_cache_requests_total Cache outcomes for public requests.", "# TYPE goproxy_cache_requests_total counter",
		"# HELP goproxy_rate_limit_decisions_total Per-route rate-limit admission decisions.", "# TYPE goproxy_rate_limit_decisions_total counter",
		"# HELP goproxy_policy_denials_total Per-route request-policy denials.", "# TYPE goproxy_policy_denials_total counter",
		"# HELP goproxy_retries_total Upstream retry attempts.", "# TYPE goproxy_retries_total counter",
		"# HELP goproxy_backend_healthy Whether a backend is eligible for traffic.", "# TYPE goproxy_backend_healthy gauge",
		"# HELP goproxy_backend_active_requests Requests currently assigned to a backend.", "# TYPE goproxy_backend_active_requests gauge",
		"# HELP go_goroutines Number of goroutines that currently exist.", "# TYPE go_goroutines gauge",
		fmt.Sprintf("go_goroutines %d", runtime.NumGoroutine()),
		"# HELP go_memstats_alloc_bytes Bytes of allocated heap objects.", "# TYPE go_memstats_alloc_bytes gauge",
		fmt.Sprintf("go_memstats_alloc_bytes %d", memory.Alloc),
	}
	fixed := len(lines)
	for key, value := range m.requests {
		lines = append(lines, fmt.Sprintf("goproxy_requests_total{method=%s,route=%s,status=%s} %d", quote(key.method), quote(key.route), quote(strconv.Itoa(key.status)), value))
	}
	for route, value := range m.durationCount {
		lines = append(lines, fmt.Sprintf("goproxy_request_duration_seconds_count{route=%s} %d", quote(route), value), fmt.Sprintf("goproxy_request_duration_seconds_sum{route=%s} %g", quote(route), m.durationSum[route]))
	}
	for key, value := range m.cache {
		lines = append(lines, fmt.Sprintf("goproxy_cache_requests_total{result=%s,route=%s} %d", quote(key.result), quote(key.route), value))
	}
	for key, value := range m.rateLimits {
		lines = append(lines, fmt.Sprintf("goproxy_rate_limit_decisions_total{decision=%s,route=%s} %d", quote(key.decision), quote(key.route), value))
	}
	for key, value := range m.policyDenials {
		lines = append(lines, fmt.Sprintf("goproxy_policy_denials_total{reason=%s,route=%s} %d", quote(key.reason), quote(key.route), value))
	}
	for route, value := range m.retries {
		lines = append(lines, fmt.Sprintf("goproxy_retries_total{route=%s} %d", quote(route), value))
	}
	for key, value := range m.health {
		lines = append(lines, fmt.Sprintf("goproxy_backend_healthy{backend=%s,pool=%s} %g", quote(key.backend), quote(key.pool), value))
	}
	for key, value := range m.active {
		lines = append(lines, fmt.Sprintf("goproxy_backend_active_requests{backend=%s,pool=%s} %d", quote(key.backend), quote(key.pool), value))
	}
	sort.Strings(lines[fixed:])
	return lines
}

func quote(value string) string { return strconv.Quote(value) }
