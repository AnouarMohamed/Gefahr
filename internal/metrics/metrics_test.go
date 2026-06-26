package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestHandlerExposesRequestAndBackendMetrics(t *testing.T) {
	m := New()
	m.ObserveRequest("request-id", "api", http.MethodGet, "/items", "one", 200, 2, "hit", 25*time.Millisecond)
	m.ObserveRateLimit("api", "limited")
	m.ObservePolicyDeny("api", "path_denied")
	m.SetBackendHealth("api", "one", true)
	m.SetBackendActive("api", "one", 3)
	recorder := httptest.NewRecorder()
	m.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{`goproxy_requests_total{method="GET",route="api",status="200"} 1`, `goproxy_rate_limit_decisions_total{decision="limited",route="api"} 1`, `goproxy_policy_denials_total{reason="path_denied",route="api"} 1`, `goproxy_retries_total{route="api"} 1`, `goproxy_backend_healthy{backend="one",pool="api"} 1`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics missing %q", expected)
		}
	}
}

func TestObserveRequestBoundsAttackerControlledMethods(t *testing.T) {
	m := New()
	for _, method := range []string{"BREW", "CUSTOM", "X-UNIQUE"} {
		m.ObserveRequest("", "api", method, "", "", 200, 1, "bypass", time.Second)
	}
	if len(m.requests) != 1 || m.requests[requestKey{"api", "OTHER", 200}] != 3 {
		t.Fatalf("request series were not bounded: %#v", m.requests)
	}
}

func TestObserveRateLimitBoundsUnknownDecisions(t *testing.T) {
	m := New()
	m.ObserveRateLimit("api", "custom")
	if m.rateLimits[rateLimitKey{"api", "other"}] != 1 {
		t.Fatalf("rate limit series = %#v", m.rateLimits)
	}
}

func TestObservePolicyDenyBoundsUnknownReasons(t *testing.T) {
	m := New()
	m.ObservePolicyDeny("api", "custom")
	if m.policyDenials[policyDenyKey{"api", "other"}] != 1 {
		t.Fatalf("policy denial series = %#v", m.policyDenials)
	}
}

func TestReconcileConfigBoundsLabelsAcrossReloads(t *testing.T) {
	m := New()
	cfg := config.Default()
	cfg.Routes = []config.Route{{Name: "current"}}
	cfg.Pools["pool"] = config.Pool{Backends: []config.Backend{{Name: "current"}}}
	m.ObserveRequest("", "old", http.MethodGet, "", "", 200, 1, "bypass", time.Second)
	m.ObserveRateLimit("old", "allowed")
	m.ObservePolicyDeny("old", "path_denied")
	m.SetBackendHealth("old", "old", true)
	m.ReconcileConfig(cfg)
	if len(m.requests) != 0 || len(m.rateLimits) != 0 || len(m.policyDenials) != 0 || len(m.health) != 0 {
		t.Fatalf("stale metrics survived: requests=%v rate_limits=%v policy_denials=%v health=%v", m.requests, m.rateLimits, m.policyDenials, m.health)
	}
	m.ObserveRequest("", "removed", http.MethodGet, "", "", 200, 1, "bypass", time.Second)
	m.ObserveRateLimit("removed", "allowed")
	m.ObservePolicyDeny("removed", "path_denied")
	m.SetBackendActive("removed", "removed", 1)
	m.SetBackendHealth("removed", "removed", true)
	if m.requests[requestKey{"retired", http.MethodGet, 200}] != 1 || m.rateLimits[rateLimitKey{"retired", "allowed"}] != 1 || m.policyDenials[policyDenyKey{"retired", "path_denied"}] != 1 || len(m.health) != 0 || len(m.active) != 0 {
		t.Fatalf("late labels were not bounded: requests=%v rate_limits=%v policy_denials=%v health=%v active=%v", m.requests, m.rateLimits, m.policyDenials, m.health, m.active)
	}
}
