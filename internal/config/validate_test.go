package config

import (
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	cfg := Default()
	cfg.Pools["api"] = Pool{
		Backends: []Backend{{Name: "api-1", URL: "http://127.0.0.1:9001"}},
		Health:   Health{Path: "/health", Interval: Duration(5 * time.Second), Timeout: Duration(time.Second), UnhealthyThreshold: 2, HealthyThreshold: 1},
		Retry:    Retry{MaxAttempts: 2},
	}
	cfg.Routes = []Route{{Name: "api", Host: "example.test", PathPrefix: "/", Pool: "api", Strategy: "round_robin"}}
	return cfg
}

func TestValidateAcceptsValidConfig(t *testing.T) {
	if err := Validate(validConfig()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateReportsMultipleErrors(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].PathPrefix = "invalid"
	cfg.Routes[0].Pool = "missing"
	err := Validate(cfg)
	if err == nil || !strings.Contains(err.Error(), "path_prefix") || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRejectsAdminCollision(t *testing.T) {
	cfg := validConfig()
	cfg.Admin.Address = cfg.Listeners[0].Address
	if err := Validate(cfg); err == nil {
		t.Fatal("expected collision error")
	}
}

func TestValidateRejectsInvalidAdminTokenEnvironment(t *testing.T) {
	cfg := validConfig()
	cfg.Admin.AuthTokenEnv = "invalid-name"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "auth_token_env") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnauthenticatedNonLoopbackAdmin(t *testing.T) {
	cfg := validConfig()
	cfg.Admin.Address = ":9090"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "admin.auth_token_env") {
		t.Fatalf("error = %v", err)
	}
	cfg.Admin.AuthTokenEnv = "GOPROXY_ADMIN_TOKEN"
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Admin.AuthTokenEnv = ""
	cfg.Admin.Tokens = []AdminToken{{Name: "monitor", Env: "GOPROXY_MONITOR_TOKEN", Scopes: []string{"read"}}}
	if err := Validate(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsInvalidAdminScopedTokens(t *testing.T) {
	tests := []func(*Config){
		func(cfg *Config) {
			cfg.Admin.Tokens = []AdminToken{{Name: "bad name", Env: "GOPROXY_TOKEN", Scopes: []string{"read"}}}
		},
		func(cfg *Config) {
			cfg.Admin.Tokens = []AdminToken{{Name: "monitor", Env: "bad-env", Scopes: []string{"read"}}}
		},
		func(cfg *Config) { cfg.Admin.Tokens = []AdminToken{{Name: "monitor", Env: "GOPROXY_TOKEN"}} },
		func(cfg *Config) {
			cfg.Admin.Tokens = []AdminToken{{Name: "monitor", Env: "GOPROXY_TOKEN", Scopes: []string{"write"}}}
		},
		func(cfg *Config) {
			cfg.Admin.Tokens = []AdminToken{{Name: "monitor", Env: "GOPROXY_TOKEN", Scopes: []string{"read", "read"}}}
		},
		func(cfg *Config) {
			cfg.Admin.Tokens = []AdminToken{
				{Name: "monitor", Env: "GOPROXY_ONE", Scopes: []string{"read"}},
				{Name: "monitor", Env: "GOPROXY_TWO", Scopes: []string{"read"}},
			}
		},
		func(cfg *Config) {
			cfg.Admin.AuthTokenEnv = "GOPROXY_TOKEN"
			cfg.Admin.Tokens = []AdminToken{{Name: "monitor", Env: "GOPROXY_TOKEN", Scopes: []string{"read"}}}
		},
	}
	for i, mutate := range tests {
		cfg := validConfig()
		mutate(&cfg)
		if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "admin") {
			t.Fatalf("case %d error = %v", i, err)
		}
	}
}

func TestValidateRejectsInvalidRateLimit(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].RateLimit = RateLimit{Enabled: true, Requests: 0, Window: Duration(time.Minute)}
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "rate_limit") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsInvalidRoutePolicy(t *testing.T) {
	tests := []func(*Config){
		func(cfg *Config) { cfg.Routes[0].Policy.AllowedMethods = []string{"get"} },
		func(cfg *Config) { cfg.Routes[0].Policy.AllowedMethods = []string{"GET", "GET"} },
		func(cfg *Config) { cfg.Routes[0].Policy.DeniedPathPrefixes = []string{"admin"} },
		func(cfg *Config) { cfg.Routes[0].Policy.DeniedPathPrefixes = []string{"/safe/../admin"} },
		func(cfg *Config) { cfg.Routes[0].Policy.RequiredHeaders = []string{"X Envoy"} },
		func(cfg *Config) { cfg.Routes[0].Policy.DeniedHeaders = []string{" X-Debug"} },
		func(cfg *Config) {
			cfg.Routes[0].Policy.RequiredHeaders = []string{"X-Gateway"}
			cfg.Routes[0].Policy.DeniedHeaders = []string{"x-gateway"}
		},
		func(cfg *Config) { cfg.Routes[0].Policy.MaxQueryBytes = -1 },
		func(cfg *Config) { cfg.Routes[0].Policy.MaxQueryBytes = (1 << 20) + 1 },
	}
	for i, mutate := range tests {
		cfg := validConfig()
		mutate(&cfg)
		if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "policy") {
			t.Fatalf("case %d error = %v", i, err)
		}
	}
}

func TestValidateRejectsInvalidClientIPPolicy(t *testing.T) {
	tests := []func(*Config){
		func(cfg *Config) { cfg.ClientIP.TrustedProxies = []string{"10.0.0.0"} },
		func(cfg *Config) { cfg.ClientIP.Headers = []string{"X-Forwarded-For"} },
		func(cfg *Config) { cfg.ClientIP.TrustedProxies = []string{"10.0.0.0/8"} },
		func(cfg *Config) {
			cfg.ClientIP.TrustedProxies = []string{"10.0.0.0/8"}
			cfg.ClientIP.Headers = []string{"Forwarded"}
		},
		func(cfg *Config) {
			cfg.ClientIP.TrustedProxies = []string{"10.0.0.0/8"}
			cfg.ClientIP.Headers = []string{"X-Real-IP", "x-real-ip"}
		},
	}
	for i, mutate := range tests {
		cfg := validConfig()
		mutate(&cfg)
		if err := Validate(cfg); err == nil {
			t.Fatalf("case %d was accepted", i)
		}
	}
}

func TestValidateRejectsIncompleteUpstreamTLS(t *testing.T) {
	cfg := validConfig()
	pool := cfg.Pools["api"]
	pool.Backends[0].URL = "https://127.0.0.1:9001"
	pool.TLS.ClientCertFile = "client.crt"
	cfg.Pools["api"] = pool
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "client_key_file") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsTLSOnHTTPOnlyPool(t *testing.T) {
	cfg := validConfig()
	pool := cfg.Pools["api"]
	pool.TLS.ServerName = "api.internal"
	cfg.Pools["api"] = pool
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "https backend") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsAmbiguousRoutingAndBackendURLs(t *testing.T) {
	tests := []func(*Config){
		func(cfg *Config) { cfg.Routes[0].PathPrefix = "/public/../admin" },
		func(cfg *Config) {
			cfg.Pools["api"] = poolWithURL(cfg.Pools["api"], "http://user:secret@127.0.0.1:9001")
		},
		func(cfg *Config) { cfg.Pools["api"] = poolWithURL(cfg.Pools["api"], "http://127.0.0.1:9001/#fragment") },
	}
	for i, mutate := range tests {
		cfg := validConfig()
		mutate(&cfg)
		if err := Validate(cfg); err == nil {
			t.Fatalf("case %d was accepted", i)
		}
	}
}

func TestValidateDetectsSemanticallyDuplicateHosts(t *testing.T) {
	cfg := validConfig()
	duplicate := cfg.Routes[0]
	duplicate.Name = "duplicate"
	duplicate.Host = "EXAMPLE.TEST."
	cfg.Routes = append(cfg.Routes, duplicate)
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateRejectsUnsafeMetricIdentifiers(t *testing.T) {
	cfg := validConfig()
	cfg.Routes[0].Name = "api\nforged"
	if err := Validate(cfg); err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("error = %v", err)
	}
}

func poolWithURL(pool Pool, target string) Pool {
	pool.Backends[0].URL = target
	return pool
}
