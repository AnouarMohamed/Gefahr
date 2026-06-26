// Package proxy composes routing, balancing, and Go's streaming reverse proxy.
package proxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anrorg/gefahr/internal/backend"
	"github.com/anrorg/gefahr/internal/balance"
	responsecache "github.com/anrorg/gefahr/internal/cache"
	"github.com/anrorg/gefahr/internal/config"
	"github.com/anrorg/gefahr/internal/routing"
)

type runtimePool struct {
	backends                []*backend.Backend
	balancers               map[string]balance.Balancer
	transport               http.RoundTripper
	retry                   config.Retry
	passiveFailureThreshold int
}

// Handler is an immutable routing snapshot with concurrency-safe backend state.
type Handler struct {
	router       *routing.Router
	pools        map[string]*runtimePool
	rateLimiters map[string]*rateLimiter
	clientIP     *clientIPPolicy
	maxBodyBytes int64
	requestSlots chan struct{}
	cache        *responsecache.Cache
	cacheTTL     time.Duration
	maxCacheBody int64
	cacheCapture atomic.Int64
	inFlight     atomic.Int64
	retired      atomic.Bool
	observer     Observer
}

// Observer receives one bounded-cardinality record after every public request.
type Observer interface {
	ObserveRequest(requestID, route, method, path, backend string, status, attempts int, cacheResult string, duration time.Duration)
}

// Option customizes a Handler without expanding its constructor over time.
type Option func(*Handler)

// WithObserver installs request telemetry and logging observation.
func WithObserver(observer Observer) Option { return func(h *Handler) { h.observer = observer } }

// NewHandler compiles validated configuration into a request handler.
func NewHandler(cfg config.Config, transport http.RoundTripper, options ...Option) (*Handler, error) {
	if err := config.Validate(cfg); err != nil {
		return nil, err
	}
	clientIP, err := newClientIPPolicy(cfg.ClientIP)
	if err != nil {
		return nil, fmt.Errorf("configure client IP policy: %w", err)
	}
	h := &Handler{router: routing.New(cfg.Routes), pools: make(map[string]*runtimePool, len(cfg.Pools)), rateLimiters: make(map[string]*rateLimiter), clientIP: clientIP, maxBodyBytes: cfg.Limits.MaxBodyBytes, requestSlots: make(chan struct{}, cfg.Limits.MaxConcurrentRequests), cache: responsecache.New(cfg.Cache.MaxEntries, cfg.Cache.MaxBytes), cacheTTL: cfg.Cache.DefaultTTL.Value(), maxCacheBody: cfg.Cache.MaxBytes}
	for name, poolCfg := range cfg.Pools {
		poolTransport := transport
		if poolTransport == nil {
			var err error
			poolTransport, err = NewPoolTransport(cfg, poolCfg)
			if err != nil {
				return nil, fmt.Errorf("configure pool %s transport: %w", name, err)
			}
		}
		pool := &runtimePool{transport: poolTransport, retry: poolCfg.Retry, passiveFailureThreshold: poolCfg.Health.UnhealthyThreshold, balancers: map[string]balance.Balancer{"round_robin": &balance.RoundRobin{}, "least_connections": &balance.LeastConnections{}}}
		for _, backendCfg := range poolCfg.Backends {
			target, err := url.Parse(backendCfg.URL)
			if err != nil {
				return nil, fmt.Errorf("parse backend %s: %w", backendCfg.Name, err)
			}
			pool.backends = append(pool.backends, backend.New(backendCfg.Name, target))
		}
		h.pools[name] = pool
	}
	for _, route := range cfg.Routes {
		if route.RateLimit.Enabled {
			h.rateLimiters[route.Name] = newRateLimiter(route.RateLimit)
		}
	}
	for _, option := range options {
		option(h)
	}
	return h, nil
}

// ServeHTTP routes and streams one request through a selected healthy backend.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.inFlight.Add(1)
	defer func() {
		if h.inFlight.Add(-1) == 0 && h.retired.Load() {
			h.closeIdleConnections()
		}
	}()
	started := time.Now()
	requestID := newRequestID()
	w.Header().Set("X-Request-ID", requestID)
	status := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	w = status
	routeName, backendName, cacheResult, attempts := "unmatched", "", "bypass", 0
	defer func() {
		if h.observer != nil {
			h.observer.ObserveRequest(requestID, routeName, r.Method, r.URL.Path, backendName, status.status, attempts, cacheResult, time.Since(started))
		}
	}()
	if r.ContentLength > h.maxBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body too large")
		return
	}
	if r.Body != nil && r.Body != http.NoBody {
		r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	}
	if !safeRequestPath(r.URL) {
		writeError(w, http.StatusBadRequest, "ambiguous_request_path", "request path contains ambiguous separators or segments")
		return
	}
	route, ok := h.router.Match(r.Host, r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "route_not_found", "route not found")
		return
	}
	pool := h.pools[route.Pool]
	routeName = route.Name
	if denial, denied := evaluateRoutePolicy(route.Policy, r); denied {
		h.observePolicyDeny(route.Name, denial.code)
		if len(denial.allowMethods) > 0 {
			w.Header().Set("Allow", strings.Join(denial.allowMethods, ", "))
		}
		writeError(w, denial.status, denial.code, denial.message)
		return
	}
	clientIP := h.clientIP.Identity(r)
	if limiter := h.rateLimiters[route.Name]; limiter != nil {
		if allowed, retryAfter := limiter.Allow(clientIP); !allowed {
			h.observeRateLimit(route.Name, false)
			w.Header().Set("Retry-After", strconv.FormatInt(retryAfterSeconds(retryAfter), 10))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "request rate limit exceeded")
			return
		}
		h.observeRateLimit(route.Name, true)
	}
	select {
	case h.requestSlots <- struct{}{}:
		defer func() { <-h.requestSlots }()
	default:
		w.Header().Set("Retry-After", "1")
		writeError(w, http.StatusServiceUnavailable, "proxy_overloaded", "proxy is at request capacity")
		return
	}
	cacheKey := ""
	if route.Cache.Enabled {
		if eligible, _ := responsecache.RequestEligible(r); eligible {
			cacheKey = responsecache.Key(r)
			if cached, hit := h.cache.Get(cacheKey); hit {
				cacheResult = "hit"
				writeCached(w, cached)
				return
			}
			cacheResult = "miss"
		}
	}
	selected, err := pool.balancers[route.Strategy].Next(pool.backends)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "no_healthy_upstream", "no healthy upstream")
		return
	}
	release := h.acquire(route.Pool, selected)
	backendName = selected.Name()
	attempts = 1

	target := selected.URL()
	originalHost := r.Host
	retrying := &retryTransport{base: pool.transport, poolName: route.Pool, pool: pool, balancer: pool.balancers[route.Strategy], first: selected, firstRelease: release, original: r, rewriteHost: route.RewriteHost, attempts: 1, lastBackend: selected.Name(), handler: h}
	defer func() { attempts, backendName = retrying.attempts, retrying.lastBackend }()
	rp := &httputil.ReverseProxy{
		Transport: retrying,
		Rewrite: func(req *httputil.ProxyRequest) {
			req.SetURL(target)
			if !route.RewriteHost {
				req.Out.Host = originalHost
			}
			setForwardingHeaders(req, clientIP)
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			var tooLarge *http.MaxBytesError
			switch {
			case errors.As(err, &tooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body too large")
			case errors.Is(err, context.DeadlineExceeded):
				writeError(w, http.StatusGatewayTimeout, "upstream_timeout", "upstream timed out")
			default:
				writeError(w, http.StatusBadGateway, "bad_gateway", "upstream request failed")
			}
		},
	}
	if cacheKey != "" {
		rp.ModifyResponse = func(response *http.Response) error {
			if len(response.Trailer) > 0 || response.Header.Get("Content-Range") != "" {
				return nil
			}
			decision := responsecache.Evaluate(r, response.StatusCode, response.Header, h.cacheTTL)
			if decision.Cacheable {
				response.Body = &cacheCaptureBody{ReadCloser: response.Body, max: h.maxCacheBody, reserve: h.reserveCacheCapture, release: func(bytes int64) {
					h.cacheCapture.Add(-bytes)
				}, commit: func(body []byte) {
					h.cache.Set(cacheKey, responsecache.Response{Status: response.StatusCode, Header: response.Header, Body: body}, decision.TTL)
				}}
			}
			return nil
		}
	}
	rp.ServeHTTP(w, r)
}

func retryAfterSeconds(duration time.Duration) int64 {
	if duration <= 0 {
		return 1
	}
	seconds := int64((duration + time.Second - 1) / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}

func (h *Handler) observeRateLimit(route string, allowed bool) {
	if h.observer == nil {
		return
	}
	observer, ok := h.observer.(interface {
		ObserveRateLimit(route, decision string)
	})
	if !ok {
		return
	}
	decision := "allowed"
	if !allowed {
		decision = "limited"
	}
	observer.ObserveRateLimit(route, decision)
}

func (h *Handler) observePolicyDeny(route, reason string) {
	if h.observer == nil {
		return
	}
	observer, ok := h.observer.(interface {
		ObservePolicyDeny(route, reason string)
	})
	if !ok {
		return
	}
	observer.ObservePolicyDeny(route, reason)
}

func writeCached(w http.ResponseWriter, response responsecache.Response) {
	for name, values := range response.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	age := int64(time.Since(response.Stored).Seconds())
	if age < 0 {
		age = 0
	}
	if initial, err := strconv.ParseInt(strings.TrimSpace(response.Header.Get("Age")), 10, 64); err == nil && initial > 0 && initial <= (1<<63-1)-age {
		age += initial
	}
	w.Header().Set("Age", strconv.FormatInt(age, 10))
	w.WriteHeader(response.Status)
	_, _ = w.Write(response.Body)
}

type cacheCaptureBody struct {
	io.ReadCloser
	buffer   []byte
	max      int64
	overflow bool
	finished bool
	reserved int64
	reserve  func(int64) bool
	release  func(int64)
	commit   func([]byte)
}

func (b *cacheCaptureBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if !b.overflow && n > 0 {
		if int64(len(b.buffer)+n) <= b.max && (b.reserve == nil || b.reserve(int64(n))) {
			b.buffer = append(b.buffer, p[:n]...)
			b.reserved += int64(n)
		} else {
			b.overflow = true
			b.discard()
		}
	}
	if err == io.EOF {
		b.finish(!b.overflow)
	}
	return n, err
}

func (b *cacheCaptureBody) Close() error {
	b.finish(false)
	return b.ReadCloser.Close()
}

func (b *cacheCaptureBody) finish(commit bool) {
	if b.finished {
		return
	}
	b.finished = true
	if commit && b.commit != nil {
		b.commit(append([]byte(nil), b.buffer...))
	}
	b.discard()
}

func (b *cacheCaptureBody) discard() {
	if b.reserved > 0 && b.release != nil {
		b.release(b.reserved)
	}
	b.reserved = 0
	b.buffer = nil
}

type retryTransport struct {
	base         http.RoundTripper
	poolName     string
	pool         *runtimePool
	balancer     balance.Balancer
	first        *backend.Backend
	firstRelease func()
	original     *http.Request
	rewriteHost  bool
	attempts     int
	lastBackend  string
	handler      *Handler
}

func (t *retryTransport) RoundTrip(initial *http.Request) (*http.Response, error) {
	request := initial
	selected := t.first
	release := t.firstRelease
	attempts := t.pool.retry.MaxAttempts
	if !canRetry(t.original) {
		attempts = 1
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := t.base.RoundTrip(request)
		if err == nil {
			selected.RecordPassiveSuccess()
			resp.Body = &releaseBody{ReadCloser: resp.Body, release: release}
			return resp, nil
		}
		release()
		if !isBackendFailure(err) {
			return nil, err
		}
		selected.RecordPassiveFailure(t.pool.passiveFailureThreshold)
		if attempt == attempts {
			return nil, err
		}
		var selectErr error
		selected, selectErr = nextRetryBackend(t.balancer, t.pool.backends, selected)
		if selectErr != nil {
			return nil, selectErr
		}
		release = t.handler.acquire(t.poolName, selected)
		t.attempts++
		t.lastBackend = selected.Name()
		request = initial.Clone(initial.Context())
		if t.original.GetBody != nil {
			request.Body, err = t.original.GetBody()
			if err != nil {
				release()
				return nil, err
			}
		}
		rewrite := &httputil.ProxyRequest{In: t.original, Out: request}
		rewrite.SetURL(selected.URL())
		if !t.rewriteHost {
			request.Host = t.original.Host
		}
	}
	return nil, errors.New("retry attempts exhausted")
}

func nextRetryBackend(balancer balance.Balancer, backends []*backend.Backend, attempted *backend.Backend) (*backend.Backend, error) {
	alternatives := make([]*backend.Backend, 0, len(backends)-1)
	for _, candidate := range backends {
		if candidate != attempted && candidate.Alive() {
			alternatives = append(alternatives, candidate)
		}
	}
	if len(alternatives) > 0 {
		return balancer.Next(alternatives)
	}
	return balancer.Next(backends)
}

func isBackendFailure(err error) bool {
	var tooLarge *http.MaxBytesError
	return !errors.As(err, &tooLarge) && !errors.Is(err, context.Canceled)
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	if status >= 100 && status < 200 && status != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.wrote, w.status = true, status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(bytes[:])
}

func canRetry(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return r.Body == nil || r.Body == http.NoBody || r.GetBody != nil
	default:
		return false
	}
}

type releaseBody struct {
	io.ReadCloser
	release func()
}

func (b *releaseBody) Close() error {
	err := b.ReadCloser.Close()
	b.release()
	return err
}

func (h *Handler) reserveCacheCapture(bytes int64) bool {
	for {
		current := h.cacheCapture.Load()
		if bytes > h.maxCacheBody-current {
			return false
		}
		if h.cacheCapture.CompareAndSwap(current, current+bytes) {
			return true
		}
	}
}

// Retire releases idle upstream connections once requests using this snapshot
// finish. It is safe to race with a request that loaded the old snapshot just
// before an atomic runtime swap.
func (h *Handler) Retire() {
	h.retired.Store(true)
	if h.inFlight.Load() == 0 {
		h.closeIdleConnections()
	}
}

func (h *Handler) closeIdleConnections() {
	for _, pool := range h.pools {
		if closer, ok := pool.transport.(interface{ CloseIdleConnections() }); ok {
			closer.CloseIdleConnections()
		}
	}
}

func setForwardingHeaders(req *httputil.ProxyRequest, client string) {
	proto := requestProto(req.In)
	req.Out.Header.Set("X-Forwarded-For", client)
	req.Out.Header.Set("X-Forwarded-Host", req.In.Host)
	req.Out.Header.Set("X-Forwarded-Proto", proto)
	req.Out.Header.Set("Forwarded", forwardedValue(req.In, client, proto))
}

func forwardedValue(r *http.Request, client, proto string) string {
	client = forwardedNode(client)
	return "for=" + quoteForwarded(client) + ";host=" + quoteForwarded(r.Host) + ";proto=" + proto
}

func requestProto(r *http.Request) string {
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	return proto
}

func forwardedNode(client string) string {
	if strings.Contains(client, ":") && !strings.HasPrefix(client, "[") {
		return "[" + client + "]"
	}
	return client
}

func quoteForwarded(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	return strconv.Quote(value)
}

func safeRequestPath(u *url.URL) bool {
	escaped := strings.ToLower(u.EscapedPath())
	if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") || strings.Contains(escaped, "%25") || strings.Contains(u.Path, `\`) {
		return false
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
