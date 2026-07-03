package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	envNamePattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	httpTokenPattern  = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
)

// Validate reports all configuration errors that can be checked without
// opening listeners or contacting upstreams.
func Validate(cfg Config) error {
	var errs []error
	const (
		maxListeners = 16
		maxRoutes    = 1000
		maxPools     = 1000
		maxBackends  = 10000
	)
	if len(cfg.Listeners) == 0 {
		errs = append(errs, errors.New("at least one listener is required"))
	} else if len(cfg.Listeners) > maxListeners {
		errs = append(errs, fmt.Errorf("listener count exceeds %d", maxListeners))
	}
	seenListeners := map[string]bool{}
	for i, listener := range cfg.Listeners {
		if listener.Address == "" {
			errs = append(errs, fmt.Errorf("listeners[%d].address is required", i))
		}
		if seenListeners[listener.Address] {
			errs = append(errs, fmt.Errorf("listener address %q is duplicated", listener.Address))
		}
		seenListeners[listener.Address] = true
		if listener.TLS != nil && (listener.TLS.CertFile == "" || listener.TLS.KeyFile == "") {
			errs = append(errs, fmt.Errorf("listeners[%d].tls requires cert_file and key_file", i))
		}
	}
	if cfg.Admin.Address == "" {
		errs = append(errs, errors.New("admin.address is required"))
	}
	if cfg.Admin.AuthTokenEnv != "" && !envNamePattern.MatchString(cfg.Admin.AuthTokenEnv) {
		errs = append(errs, fmt.Errorf("admin.auth_token_env must match %s", envNamePattern))
	}
	errs = append(errs, validateAdminTokens(cfg.Admin.AuthTokenEnv, cfg.Admin.Tokens)...)
	if !adminAuthConfigured(cfg.Admin) && !loopbackListenAddress(cfg.Admin.Address) {
		errs = append(errs, errors.New("admin.auth_token_env or admin.tokens is required when admin.address is not loopback"))
	}
	if seenListeners[cfg.Admin.Address] {
		errs = append(errs, errors.New("admin address must differ from public listeners"))
	}
	if len(cfg.Routes) == 0 {
		errs = append(errs, errors.New("at least one route is required"))
	} else if len(cfg.Routes) > maxRoutes {
		errs = append(errs, fmt.Errorf("route count exceeds %d", maxRoutes))
	}
	if len(cfg.Pools) > maxPools {
		errs = append(errs, fmt.Errorf("pool count exceeds %d", maxPools))
	}

	seenNames, seenMatches := map[string]bool{}, map[string]bool{}
	for i, route := range cfg.Routes {
		field := fmt.Sprintf("routes[%d]", i)
		if !identifierPattern.MatchString(route.Name) {
			errs = append(errs, fmt.Errorf("%s.name must match %s", field, identifierPattern))
		}
		if seenNames[route.Name] {
			errs = append(errs, fmt.Errorf("route name %q is duplicated", route.Name))
		}
		seenNames[route.Name] = true
		if route.Host != strings.TrimSpace(route.Host) {
			errs = append(errs, fmt.Errorf("%s.host must not contain surrounding whitespace", field))
		}
		if route.PathPrefix == "" || !strings.HasPrefix(route.PathPrefix, "/") {
			errs = append(errs, fmt.Errorf("%s.path_prefix must start with /", field))
		} else if ambiguousPath(route.PathPrefix) {
			errs = append(errs, fmt.Errorf("%s.path_prefix contains ambiguous separators or segments", field))
		}
		match := normalizedHost(route.Host) + "\x00" + route.PathPrefix
		if seenMatches[match] {
			errs = append(errs, fmt.Errorf("route match host=%q path_prefix=%q is duplicated", route.Host, route.PathPrefix))
		}
		seenMatches[match] = true
		if _, ok := cfg.Pools[route.Pool]; !ok {
			errs = append(errs, fmt.Errorf("%s.pool %q does not exist", field, route.Pool))
		}
		if route.Strategy != "round_robin" && route.Strategy != "least_connections" {
			errs = append(errs, fmt.Errorf("%s.strategy must be round_robin or least_connections", field))
		}
		errs = append(errs, validateRoutePolicy(field+".policy", route.Policy)...)
		if route.RateLimit.Enabled {
			if route.RateLimit.Requests <= 0 || route.RateLimit.Window <= 0 {
				errs = append(errs, fmt.Errorf("%s.rate_limit requires positive requests and window", field))
			}
			if route.RateLimit.MaxKeys < 0 || route.RateLimit.MaxKeys > 1000000 {
				errs = append(errs, fmt.Errorf("%s.rate_limit.max_keys must be between 0 and 1000000", field))
			}
		}
	}

	totalBackends := 0
	for name, pool := range cfg.Pools {
		if !identifierPattern.MatchString(name) {
			errs = append(errs, fmt.Errorf("pool name %q must match %s", name, identifierPattern))
		}
		totalBackends += len(pool.Backends)
		if len(pool.Backends) == 0 {
			errs = append(errs, fmt.Errorf("pool %q requires at least one backend", name))
		}
		seenBackends := map[string]bool{}
		hasHTTPSBackend := false
		for i, backend := range pool.Backends {
			u, err := url.Parse(backend.URL)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
				errs = append(errs, fmt.Errorf("pools.%s.backends[%d].url must be an absolute http(s) URL", name, i))
			}
			if err == nil && u.Scheme == "https" {
				hasHTTPSBackend = true
			}
			if seenBackends[backend.Name] || !identifierPattern.MatchString(backend.Name) {
				errs = append(errs, fmt.Errorf("pools.%s backend names must match %s and be unique", name, identifierPattern))
			}
			seenBackends[backend.Name] = true
		}
		if poolTLSConfigured(pool.TLS) && !hasHTTPSBackend {
			errs = append(errs, fmt.Errorf("pools.%s.tls requires at least one https backend", name))
		}
		if (pool.TLS.ClientCertFile == "") != (pool.TLS.ClientKeyFile == "") {
			errs = append(errs, fmt.Errorf("pools.%s.tls requires both client_cert_file and client_key_file", name))
		}
		if pool.TLS.ServerName != strings.TrimSpace(pool.TLS.ServerName) {
			errs = append(errs, fmt.Errorf("pools.%s.tls.server_name must not contain surrounding whitespace", name))
		}
		if pool.Health.Path == "" || !strings.HasPrefix(pool.Health.Path, "/") || ambiguousPath(pool.Health.Path) || strings.ContainsAny(pool.Health.Path, "?#") {
			errs = append(errs, fmt.Errorf("pools.%s.health.path must start with /", name))
		}
		if pool.Health.Interval <= 0 || pool.Health.Timeout <= 0 || pool.Health.UnhealthyThreshold <= 0 || pool.Health.HealthyThreshold <= 0 {
			errs = append(errs, fmt.Errorf("pools.%s health values must be positive", name))
		}
		if pool.Retry.MaxAttempts < 1 || pool.Retry.MaxAttempts > 2 {
			errs = append(errs, fmt.Errorf("pools.%s.retry.max_attempts must be 1 or 2", name))
		}
	}
	if totalBackends > maxBackends {
		errs = append(errs, fmt.Errorf("backend count exceeds %d", maxBackends))
	}
	if cfg.Timeouts.ReadHeader <= 0 || cfg.Timeouts.ReadBody <= 0 || cfg.Timeouts.Write <= 0 || cfg.Timeouts.Idle <= 0 || cfg.Timeouts.Shutdown <= 0 || cfg.Timeouts.Dial <= 0 || cfg.Timeouts.ResponseHeader <= 0 {
		errs = append(errs, errors.New("all timeouts must be positive"))
	}
	if cfg.Limits.MaxHeaderBytes <= 0 || cfg.Limits.MaxBodyBytes <= 0 || cfg.Limits.MaxConcurrentRequests <= 0 || cfg.Limits.MaxConnections <= 0 {
		errs = append(errs, errors.New("all request limits must be positive"))
	}
	if cfg.Limits.MaxConcurrentRequests > 100000 {
		errs = append(errs, errors.New("limits.max_concurrent_requests must not exceed 100000"))
	}
	if cfg.Limits.MaxConnections > 1000000 {
		errs = append(errs, errors.New("limits.max_connections must not exceed 1000000"))
	}
	if len(cfg.ClientIP.TrustedProxies) > 128 {
		errs = append(errs, errors.New("client_ip.trusted_proxies must not exceed 128 entries"))
	}
	for i, cidr := range cfg.ClientIP.TrustedProxies {
		if cidr != strings.TrimSpace(cidr) {
			errs = append(errs, fmt.Errorf("client_ip.trusted_proxies[%d] must not contain surrounding whitespace", i))
			continue
		}
		if _, err := netip.ParsePrefix(cidr); err != nil {
			errs = append(errs, fmt.Errorf("client_ip.trusted_proxies[%d] must be a CIDR prefix", i))
		}
	}
	if len(cfg.ClientIP.Headers) > 0 && len(cfg.ClientIP.TrustedProxies) == 0 {
		errs = append(errs, errors.New("client_ip.headers requires client_ip.trusted_proxies"))
	}
	if len(cfg.ClientIP.TrustedProxies) > 0 && len(cfg.ClientIP.Headers) == 0 {
		errs = append(errs, errors.New("client_ip.trusted_proxies requires explicit client_ip.headers"))
	}
	seenClientIPHeaders := map[string]bool{}
	for i, header := range cfg.ClientIP.Headers {
		normalized := strings.ToLower(strings.TrimSpace(header))
		if header != strings.TrimSpace(header) {
			errs = append(errs, fmt.Errorf("client_ip.headers[%d] must not contain surrounding whitespace", i))
		}
		if normalized != "x-forwarded-for" && normalized != "x-real-ip" {
			errs = append(errs, fmt.Errorf("client_ip.headers[%d] must be X-Forwarded-For or X-Real-IP", i))
		}
		if seenClientIPHeaders[normalized] {
			errs = append(errs, fmt.Errorf("client_ip header %q is duplicated", header))
		}
		seenClientIPHeaders[normalized] = true
	}
	if cfg.Cache.MaxEntries <= 0 || cfg.Cache.MaxBytes <= 0 || cfg.Cache.DefaultTTL <= 0 {
		errs = append(errs, errors.New("all cache bounds must be positive"))
	}
	if cfg.Logging.Level != "debug" && cfg.Logging.Level != "info" && cfg.Logging.Level != "warn" && cfg.Logging.Level != "error" {
		errs = append(errs, errors.New("logging.level must be debug, info, warn, or error"))
	}
	return errors.Join(errs...)
}

func adminAuthConfigured(admin Admin) bool {
	return admin.AuthTokenEnv != "" || len(admin.Tokens) > 0
}

func loopbackListenAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

func validateAdminTokens(legacyEnv string, tokens []AdminToken) []error {
	var errs []error
	const maxAdminTokens = 16
	if len(tokens) > maxAdminTokens {
		errs = append(errs, fmt.Errorf("admin.tokens must not exceed %d entries", maxAdminTokens))
	}
	seenNames, seenEnvs := map[string]bool{}, map[string]bool{}
	if legacyEnv != "" {
		seenEnvs[legacyEnv] = true
	}
	for i, token := range tokens {
		field := fmt.Sprintf("admin.tokens[%d]", i)
		if !identifierPattern.MatchString(token.Name) {
			errs = append(errs, fmt.Errorf("%s.name must match %s", field, identifierPattern))
		}
		if seenNames[token.Name] {
			errs = append(errs, fmt.Errorf("admin token name %q is duplicated", token.Name))
		}
		seenNames[token.Name] = true
		if !envNamePattern.MatchString(token.Env) {
			errs = append(errs, fmt.Errorf("%s.env must match %s", field, envNamePattern))
		}
		if seenEnvs[token.Env] {
			errs = append(errs, fmt.Errorf("admin token env %q is duplicated", token.Env))
		}
		seenEnvs[token.Env] = true
		if len(token.Scopes) == 0 {
			errs = append(errs, fmt.Errorf("%s.scopes requires at least one scope", field))
		}
		seenScopes := map[string]bool{}
		for j, scope := range token.Scopes {
			if scope != strings.TrimSpace(scope) {
				errs = append(errs, fmt.Errorf("%s.scopes[%d] must not contain surrounding whitespace", field, j))
			}
			if scope != "admin" && scope != "health" && scope != "metrics" && scope != "read" {
				errs = append(errs, fmt.Errorf("%s.scopes[%d] must be admin, health, metrics, or read", field, j))
			}
			if seenScopes[scope] {
				errs = append(errs, fmt.Errorf("%s.scopes contains duplicate scope %q", field, scope))
			}
			seenScopes[scope] = true
		}
	}
	return errs
}

func validateRoutePolicy(field string, policy RoutePolicy) []error {
	var errs []error
	const (
		maxAllowedMethods = 32
		maxPathPrefixes   = 128
		maxPolicyHeaders  = 64
		maxQueryBytes     = 1 << 20
	)
	if len(policy.AllowedMethods) > maxAllowedMethods {
		errs = append(errs, fmt.Errorf("%s.allowed_methods must not exceed %d entries", field, maxAllowedMethods))
	}
	seenMethods := map[string]bool{}
	for i, method := range policy.AllowedMethods {
		if method != strings.TrimSpace(method) || method == "" || !httpTokenPattern.MatchString(method) || method != strings.ToUpper(method) {
			errs = append(errs, fmt.Errorf("%s.allowed_methods[%d] must be an uppercase HTTP token", field, i))
		}
		if seenMethods[method] {
			errs = append(errs, fmt.Errorf("%s.allowed_methods contains duplicate method %q", field, method))
		}
		seenMethods[method] = true
	}
	if len(policy.DeniedPathPrefixes) > maxPathPrefixes {
		errs = append(errs, fmt.Errorf("%s.denied_path_prefixes must not exceed %d entries", field, maxPathPrefixes))
	}
	seenPaths := map[string]bool{}
	for i, prefix := range policy.DeniedPathPrefixes {
		if prefix != strings.TrimSpace(prefix) || prefix == "" || !strings.HasPrefix(prefix, "/") || strings.ContainsAny(prefix, "?#") || ambiguousPath(prefix) {
			errs = append(errs, fmt.Errorf("%s.denied_path_prefixes[%d] must start with / and avoid ambiguous separators or segments", field, i))
		}
		if seenPaths[prefix] {
			errs = append(errs, fmt.Errorf("%s.denied_path_prefixes contains duplicate prefix %q", field, prefix))
		}
		seenPaths[prefix] = true
	}
	errs = append(errs, validateHeaderList(field, "required_headers", policy.RequiredHeaders, maxPolicyHeaders)...)
	errs = append(errs, validateHeaderList(field, "denied_headers", policy.DeniedHeaders, maxPolicyHeaders)...)
	deniedHeaders := map[string]bool{}
	for _, header := range policy.DeniedHeaders {
		deniedHeaders[strings.ToLower(header)] = true
	}
	for _, header := range policy.RequiredHeaders {
		if deniedHeaders[strings.ToLower(header)] {
			errs = append(errs, fmt.Errorf("%s header %q cannot be both required and denied", field, header))
		}
	}
	if policy.MaxQueryBytes < 0 || policy.MaxQueryBytes > maxQueryBytes {
		errs = append(errs, fmt.Errorf("%s.max_query_bytes must be between 0 and %d", field, maxQueryBytes))
	}
	return errs
}

func validateHeaderList(field, name string, headers []string, maxEntries int) []error {
	var errs []error
	if len(headers) > maxEntries {
		errs = append(errs, fmt.Errorf("%s.%s must not exceed %d entries", field, name, maxEntries))
	}
	seen := map[string]bool{}
	for i, header := range headers {
		normalized := strings.ToLower(header)
		if header != strings.TrimSpace(header) || header == "" || !httpTokenPattern.MatchString(header) {
			errs = append(errs, fmt.Errorf("%s.%s[%d] must be an HTTP header token", field, name, i))
		}
		if seen[normalized] {
			errs = append(errs, fmt.Errorf("%s.%s contains duplicate header %q", field, name, header))
		}
		seen[normalized] = true
	}
	return errs
}

func normalizedHost(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return parsed
	}
	return host
}

func ambiguousPath(value string) bool {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || strings.Contains(lower, "%25") || strings.Contains(value, `\`) {
		return true
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." || strings.EqualFold(segment, "%2e") || strings.EqualFold(segment, "%2e%2e") {
			return true
		}
	}
	return false
}

func poolTLSConfigured(tls PoolTLS) bool {
	return tls.CAFile != "" || tls.ServerName != "" || tls.ClientCertFile != "" || tls.ClientKeyFile != "" || tls.InsecureSkipVerify
}
