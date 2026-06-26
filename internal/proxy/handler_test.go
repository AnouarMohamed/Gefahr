package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	responsecache "github.com/anrorg/gefahr/internal/cache"
	"github.com/anrorg/gefahr/internal/config"
)

func proxyConfig() config.Config {
	cfg := validProxyConfig()
	return cfg
}

func validProxyConfig() config.Config {
	cfg := config.Default()
	cfg.Routes = []config.Route{{Name: "api", Host: "api.test", PathPrefix: "/api", Pool: "api", Strategy: "round_robin"}}
	cfg.Pools["api"] = config.Pool{
		Backends: []config.Backend{{Name: "one", URL: "http://backend.test/base"}},
		Health:   config.Health{Path: "/health", Interval: config.Duration(1), Timeout: config.Duration(1), HealthyThreshold: 1, UnhealthyThreshold: 1},
		Retry:    config.Retry{MaxAttempts: 1},
	}
	return cfg
}

func TestHandlerForwardsMatchedRequest(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "backend.test" || r.URL.Path != "/base/api/users" {
			t.Fatalf("upstream URL = %s", r.URL)
		}
		if r.Host != "api.test" {
			t.Fatalf("upstream Host = %q", r.Host)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("proxied"))}, nil
	})
	h, err := NewHandler(proxyConfig(), transport)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://api.test/api/users", nil)
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "proxied" {
		t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
	}
}

func TestHandlerReturnsNotFoundForUnmatchedRequest(t *testing.T) {
	h, err := NewHandler(proxyConfig(), roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("transport called"); return nil, nil }))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://other.test/api", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestHandlerRejectsAmbiguousRequestPaths(t *testing.T) {
	h, err := NewHandler(proxyConfig(), roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("transport called")
		return nil, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"http://api.test/api%2Fadmin", "http://api.test/api/%2e%2e/admin", "http://api.test/api%252Fadmin", "http://api.test/api\\admin"} {
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("target %q status = %d", target, recorder.Code)
		}
	}
}

func TestHandlerEnforcesRoutePolicy(t *testing.T) {
	tests := []struct {
		name      string
		policy    config.RoutePolicy
		method    string
		target    string
		headers   http.Header
		status    int
		code      string
		wantAllow string
	}{
		{
			name:      "method allowlist",
			policy:    config.RoutePolicy{AllowedMethods: []string{http.MethodGet, http.MethodHead}},
			method:    http.MethodPost,
			target:    "http://api.test/api",
			status:    http.StatusMethodNotAllowed,
			code:      policyReasonMethodNotAllowed,
			wantAllow: "GET, HEAD",
		},
		{
			name:   "denied path prefix",
			policy: config.RoutePolicy{DeniedPathPrefixes: []string{"/api/admin"}},
			method: http.MethodGet,
			target: "http://api.test/api/admin/users",
			status: http.StatusForbidden,
			code:   policyReasonPathDenied,
		},
		{
			name:   "missing required header",
			policy: config.RoutePolicy{RequiredHeaders: []string{"X-Verified-Client"}},
			method: http.MethodGet,
			target: "http://api.test/api",
			status: http.StatusBadRequest,
			code:   policyReasonRequiredHeaderMissing,
		},
		{
			name:    "denied header",
			policy:  config.RoutePolicy{DeniedHeaders: []string{"X-Debug-Bypass"}},
			method:  http.MethodGet,
			target:  "http://api.test/api",
			headers: http.Header{"X-Debug-Bypass": {"1"}},
			status:  http.StatusForbidden,
			code:    policyReasonHeaderDenied,
		},
		{
			name:   "query too large",
			policy: config.RoutePolicy{MaxQueryBytes: 4},
			method: http.MethodGet,
			target: "http://api.test/api?abcde",
			status: http.StatusRequestURITooLong,
			code:   policyReasonQueryTooLarge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := proxyConfig()
			cfg.Routes[0].Policy = tt.policy
			h, err := NewHandler(cfg, roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("transport called")
				return nil, nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(tt.method, tt.target, nil)
			for name, values := range tt.headers {
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}
			recorder := httptest.NewRecorder()
			h.ServeHTTP(recorder, req)
			if recorder.Code != tt.status || !strings.Contains(recorder.Body.String(), tt.code) {
				t.Fatalf("response = %d %q", recorder.Code, recorder.Body.String())
			}
			if got := recorder.Header().Get("Allow"); got != tt.wantAllow {
				t.Fatalf("Allow = %q", got)
			}
		})
	}
}

func TestHandlerReplacesUntrustedForwardingHeaders(t *testing.T) {
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-Forwarded-For"); got != "192.0.2.10" {
			t.Fatalf("X-Forwarded-For = %q", got)
		}
		if got := r.Header.Get("X-Forwarded-Host"); got != "api.test" {
			t.Fatalf("X-Forwarded-Host = %q", got)
		}
		if got := r.Header.Get("X-Forwarded-Proto"); got != "http" {
			t.Fatalf("X-Forwarded-Proto = %q", got)
		}
		if got := r.Header.Get("Forwarded"); got != `for="192.0.2.10";host="api.test";proto=http` {
			t.Fatalf("Forwarded = %q", got)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody}, nil
	})
	h, err := NewHandler(proxyConfig(), transport)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	req.RemoteAddr = "192.0.2.10:4321"
	req.Header.Set("X-Forwarded-For", "attacker.test")
	req.Header.Set("Forwarded", "for=attacker.test")
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestHandlerUsesTrustedProxyForwardingHeader(t *testing.T) {
	cfg := proxyConfig()
	cfg.ClientIP.TrustedProxies = []string{"10.0.0.0/8"}
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-Forwarded-For"); got != "198.51.100.7" {
			t.Fatalf("X-Forwarded-For = %q", got)
		}
		if got := r.Header.Get("Forwarded"); got != `for="198.51.100.7";host="api.test";proto=http` {
			t.Fatalf("Forwarded = %q", got)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody}, nil
	})
	h, err := NewHandler(cfg, transport)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	req.RemoteAddr = "10.0.0.5:4321"
	req.Header.Set("X-Forwarded-For", "203.0.113.99, 198.51.100.7, 10.0.0.5")
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestForwardedHeaderFormatsIPv6Client(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	if got := forwardedValue(request, "2001:db8::1", "http"); got != `for="[2001:db8::1]";host="api.test";proto=http` {
		t.Fatalf("Forwarded = %q", got)
	}
}

func TestHandlerRejectsDeclaredOversizedBody(t *testing.T) {
	cfg := proxyConfig()
	cfg.Limits.MaxBodyBytes = 4
	h, err := NewHandler(cfg, roundTripFunc(func(*http.Request) (*http.Response, error) { t.Fatal("transport called"); return nil, nil }))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://api.test/api", strings.NewReader("oversized"))
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestHandlerMapsUpstreamTimeoutWithoutLeakingError(t *testing.T) {
	h, err := NewHandler(proxyConfig(), roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded }))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
	if recorder.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Body.String() != "{\"code\":\"upstream_timeout\",\"message\":\"upstream timed out\"}\n" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestHandlerRetriesSafeTransportFailure(t *testing.T) {
	cfg := proxyConfig()
	cfg.Pools["api"] = config.Pool{
		Backends: []config.Backend{{Name: "one", URL: "http://one.test"}, {Name: "two", URL: "http://two.test"}},
		Health:   config.Health{Path: "/health", Interval: config.Duration(1), Timeout: config.Duration(1), HealthyThreshold: 1, UnhealthyThreshold: 1},
		Retry:    config.Retry{MaxAttempts: 2},
	}
	calls := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, context.DeadlineExceeded
		}
		if r.URL.Host != "two.test" {
			t.Fatalf("retry host = %q", r.URL.Host)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("retried"))}, nil
	})
	h, err := NewHandler(cfg, transport)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "retried" || calls != 2 {
		t.Fatalf("response = %d %q, calls=%d", recorder.Code, recorder.Body.String(), calls)
	}
}

func TestLeastConnectionsRetryAvoidsFailedBackend(t *testing.T) {
	cfg := proxyConfig()
	cfg.Routes[0].Strategy = "least_connections"
	cfg.Pools["api"] = config.Pool{
		Backends: []config.Backend{{Name: "one", URL: "http://one.test"}, {Name: "two", URL: "http://two.test"}},
		Health:   config.Health{Path: "/health", Interval: config.Duration(1), Timeout: config.Duration(1), HealthyThreshold: 1, UnhealthyThreshold: 2},
		Retry:    config.Retry{MaxAttempts: 2},
	}
	var hosts []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hosts = append(hosts, r.URL.Host)
		if len(hosts) == 1 {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody}, nil
	})
	h, err := NewHandler(cfg, transport)
	if err != nil {
		t.Fatal(err)
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
	if len(hosts) != 2 || hosts[0] == hosts[1] {
		t.Fatalf("retry hosts = %v", hosts)
	}
}

func TestHandlerServesEligibleResponseFromCache(t *testing.T) {
	cfg := proxyConfig()
	cfg.Routes[0].Cache.Enabled = true
	calls := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Cache-Control": {"public, max-age=60"}}, Body: io.NopCloser(strings.NewReader("cached"))}, nil
	})
	h, err := NewHandler(cfg, transport)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		recorder := httptest.NewRecorder()
		h.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://api.test/api/items?q=one", nil))
		if recorder.Code != http.StatusOK || recorder.Body.String() != "cached" {
			t.Fatalf("response %d = %d %q", i, recorder.Code, recorder.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d", calls)
	}
}

func TestHandlerDoesNotCacheResponseWithTrailers(t *testing.T) {
	cfg := proxyConfig()
	cfg.Routes[0].Cache.Enabled = true
	calls := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Cache-Control": {"public, max-age=60"}}, Trailer: http.Header{"Set-Cookie": {"session=secret"}}, Body: io.NopCloser(strings.NewReader("uncached"))}, nil
	})
	h, err := NewHandler(cfg, transport)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d", calls)
	}
}

func TestClientBodyErrorDoesNotEjectBackend(t *testing.T) {
	calls := 0
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return nil, &http.MaxBytesError{Limit: 10}
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("healthy"))}, nil
	})
	h, err := NewHandler(proxyConfig(), transport)
	if err != nil {
		t.Fatal(err)
	}
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
	if first.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("first status = %d", first.Code)
	}
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
	if second.Code != http.StatusOK || second.Body.String() != "healthy" {
		t.Fatalf("second response = %d %q", second.Code, second.Body.String())
	}
}

func TestStatusWriterAllowsInformationalBeforeFinalStatus(t *testing.T) {
	underlying := &statusSequenceWriter{header: make(http.Header)}
	w := &statusWriter{ResponseWriter: underlying, status: http.StatusOK}
	w.WriteHeader(http.StatusEarlyHints)
	w.WriteHeader(http.StatusNoContent)
	if w.status != http.StatusNoContent || len(underlying.statuses) != 2 || underlying.statuses[1] != http.StatusNoContent {
		t.Fatalf("status=%d sequence=%v", w.status, underlying.statuses)
	}
}

func TestCacheCaptureReleasesBudgetOnOverflowAndClose(t *testing.T) {
	var reserved int64
	committed := false
	body := &cacheCaptureBody{
		ReadCloser: io.NopCloser(strings.NewReader("oversized")),
		max:        4,
		reserve: func(n int64) bool {
			if reserved+n > 4 {
				return false
			}
			reserved += n
			return true
		},
		release: func(n int64) { reserved -= n },
		commit:  func([]byte) { committed = true },
	}
	_, _ = io.ReadAll(body)
	_ = body.Close()
	if committed || reserved != 0 {
		t.Fatalf("committed=%v reserved=%d", committed, reserved)
	}
}

func TestWriteCachedAddsResidentTimeToExistingAge(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeCached(recorder, responsecache.Response{Status: http.StatusOK, Header: http.Header{"Age": {"5"}}, Stored: time.Now().Add(-2 * time.Second)})
	age, _ := strconv.Atoi(recorder.Header().Get("Age"))
	if age < 7 {
		t.Fatalf("age = %d", age)
	}
}

func TestRetireClosesIdleConnectionsAfterInflightRequest(t *testing.T) {
	transport := &retirementTransport{entered: make(chan struct{}), release: make(chan struct{})}
	h, err := NewHandler(proxyConfig(), transport)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
		close(done)
	}()
	<-transport.entered
	h.Retire()
	if transport.closes.Load() != 0 {
		t.Fatal("transport closed while request was active")
	}
	close(transport.release)
	<-done
	if transport.closes.Load() != 1 {
		t.Fatalf("close calls = %d", transport.closes.Load())
	}
}

func TestHandlerRejectsRequestAboveConcurrencyLimit(t *testing.T) {
	cfg := proxyConfig()
	cfg.Limits.MaxConcurrentRequests = 1
	transport := &retirementTransport{entered: make(chan struct{}), release: make(chan struct{})}
	h, err := NewHandler(cfg, transport)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
		close(done)
	}()
	<-transport.entered
	overloaded := httptest.NewRecorder()
	h.ServeHTTP(overloaded, httptest.NewRequest(http.MethodGet, "http://api.test/api", nil))
	if overloaded.Code != http.StatusServiceUnavailable || !strings.Contains(overloaded.Body.String(), "proxy_overloaded") {
		t.Fatalf("overload response = %d %q", overloaded.Code, overloaded.Body.String())
	}
	close(transport.release)
	<-done
}

func TestHandlerRateLimitsPerClientAndRoute(t *testing.T) {
	cfg := proxyConfig()
	cfg.Routes[0].RateLimit = config.RateLimit{Enabled: true, Requests: 1, Window: config.Duration(time.Minute), MaxKeys: 10}
	calls := 0
	h, err := NewHandler(cfg, roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	firstReq.RemoteAddr = "192.0.2.10:1234"
	h.ServeHTTP(first, firstReq)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}

	limited := httptest.NewRecorder()
	limitedReq := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	limitedReq.RemoteAddr = "192.0.2.10:5678"
	h.ServeHTTP(limited, limitedReq)
	if limited.Code != http.StatusTooManyRequests || !strings.Contains(limited.Body.String(), "rate_limited") || limited.Header().Get("Retry-After") == "" {
		t.Fatalf("limited response = %d headers=%v body=%q", limited.Code, limited.Header(), limited.Body.String())
	}

	otherClient := httptest.NewRecorder()
	otherReq := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	otherReq.RemoteAddr = "192.0.2.11:1234"
	h.ServeHTTP(otherClient, otherReq)
	if otherClient.Code != http.StatusOK {
		t.Fatalf("other client status = %d", otherClient.Code)
	}
	if calls != 2 {
		t.Fatalf("upstream calls = %d", calls)
	}
}

func TestHandlerObservesRateLimitDecisions(t *testing.T) {
	cfg := proxyConfig()
	cfg.Routes[0].RateLimit = config.RateLimit{Enabled: true, Requests: 1, Window: config.Duration(time.Minute), MaxKeys: 10}
	observer := &recordingObserver{}
	h, err := NewHandler(cfg, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}), WithObserver(observer))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		h.ServeHTTP(httptest.NewRecorder(), req)
	}

	if got := strings.Join(observer.rateLimitDecisions, ","); got != "api:allowed,api:limited" {
		t.Fatalf("rate limit decisions = %q", got)
	}
}

func TestHandlerObservesPolicyDenials(t *testing.T) {
	cfg := proxyConfig()
	cfg.Routes[0].Policy = config.RoutePolicy{DeniedPathPrefixes: []string{"/api/admin"}}
	observer := &recordingObserver{}
	h, err := NewHandler(cfg, roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("transport called")
		return nil, nil
	}), WithObserver(observer))
	if err != nil {
		t.Fatal(err)
	}

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "http://api.test/api/admin", nil))

	if got := strings.Join(observer.policyDenials, ","); got != "api:path_denied" {
		t.Fatalf("policy denials = %q", got)
	}
}

func TestHandlerRateLimitUsesTrustedClientIdentity(t *testing.T) {
	cfg := proxyConfig()
	cfg.ClientIP.TrustedProxies = []string{"10.0.0.0/8"}
	cfg.Routes[0].RateLimit = config.RateLimit{Enabled: true, Requests: 1, Window: config.Duration(time.Minute), MaxKeys: 10}
	h, err := NewHandler(cfg, roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("ok"))}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	firstReq.RemoteAddr = "10.0.0.5:1234"
	firstReq.Header.Set("X-Forwarded-For", "198.51.100.7")
	h.ServeHTTP(first, firstReq)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d", first.Code)
	}

	limited := httptest.NewRecorder()
	limitedReq := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	limitedReq.RemoteAddr = "10.0.0.6:1234"
	limitedReq.Header.Set("X-Forwarded-For", "198.51.100.7")
	h.ServeHTTP(limited, limitedReq)
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d", limited.Code)
	}

	otherClient := httptest.NewRecorder()
	otherReq := httptest.NewRequest(http.MethodGet, "http://api.test/api", nil)
	otherReq.RemoteAddr = "10.0.0.6:1234"
	otherReq.Header.Set("X-Forwarded-For", "198.51.100.8")
	h.ServeHTTP(otherClient, otherReq)
	if otherClient.Code != http.StatusOK {
		t.Fatalf("other status = %d", otherClient.Code)
	}
}

type statusSequenceWriter struct {
	header   http.Header
	statuses []int
}

func (w *statusSequenceWriter) Header() http.Header         { return w.header }
func (w *statusSequenceWriter) WriteHeader(status int)      { w.statuses = append(w.statuses, status) }
func (w *statusSequenceWriter) Write(p []byte) (int, error) { return len(p), nil }

type retirementTransport struct {
	entered chan struct{}
	release chan struct{}
	closes  atomic.Int64
}

func (t *retirementTransport) RoundTrip(*http.Request) (*http.Response, error) {
	close(t.entered)
	<-t.release
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody}, nil
}

func (t *retirementTransport) CloseIdleConnections() { t.closes.Add(1) }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type recordingObserver struct {
	rateLimitDecisions []string
	policyDenials      []string
}

func (o *recordingObserver) ObserveRequest(string, string, string, string, string, int, int, string, time.Duration) {
}

func (o *recordingObserver) ObserveRateLimit(route, decision string) {
	o.rateLimitDecisions = append(o.rateLimitDecisions, route+":"+decision)
}

func (o *recordingObserver) ObservePolicyDeny(route, reason string) {
	o.policyDenials = append(o.policyDenials, route+":"+reason)
}
