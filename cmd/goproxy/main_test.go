package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestRunHealthcheckRequiresOKResponse(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		transport  error
		wantErr    bool
		wantClosed bool
	}{
		{name: "ready", status: http.StatusOK, wantClosed: true},
		{name: "not ready", status: http.StatusServiceUnavailable, wantErr: true, wantClosed: true},
		{name: "transport failure", transport: errors.New("dial failed"), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := &trackingBody{Reader: strings.NewReader("status")}
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				if test.transport != nil {
					return nil, test.transport
				}
				return &http.Response{StatusCode: test.status, Body: body, Header: make(http.Header)}, nil
			})}
			err := runHealthcheck(client, "http://127.0.0.1:9090/readyz", "")
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v", err)
			}
			if body.closed != test.wantClosed {
				t.Fatalf("body closed = %t", body.closed)
			}
		})
	}
}

func TestRunHealthcheckSendsBearerToken(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})}
	if err := runHealthcheck(client, "http://127.0.0.1:9090/readyz", "secret"); err != nil {
		t.Fatal(err)
	}
}

func TestAdminTokenReadsConfiguredEnvironment(t *testing.T) {
	t.Setenv("GOPROXY_ADMIN_TOKEN", "secret")
	cfg := config.Default()
	cfg.Admin.AuthTokenEnv = "GOPROXY_ADMIN_TOKEN"
	token, err := adminToken(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if token != "secret" {
		t.Fatalf("token = %q", token)
	}
}

func TestAdminTokenRequiresConfiguredEnvironment(t *testing.T) {
	cfg := config.Default()
	cfg.Admin.AuthTokenEnv = "GOPROXY_ADMIN_TOKEN"
	if _, err := adminToken(cfg); err == nil {
		t.Fatal("expected missing token error")
	}
}

func TestAdminCredentialsLoadLegacyAndScopedTokens(t *testing.T) {
	t.Setenv("GOPROXY_ADMIN_TOKEN", "admin-secret")
	t.Setenv("GOPROXY_MONITOR_TOKEN", "monitor-secret")
	cfg := config.Default()
	cfg.Admin.AuthTokenEnv = "GOPROXY_ADMIN_TOKEN"
	cfg.Admin.Tokens = []config.AdminToken{{Name: "monitor", Env: "GOPROXY_MONITOR_TOKEN", Scopes: []string{"read"}}}
	credentials, err := adminCredentials(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(credentials) != 2 {
		t.Fatalf("credentials = %+v", credentials)
	}
	if credentials[0].Name != "legacy-admin" || credentials[0].Token != "admin-secret" || credentials[0].Scopes[0] != "admin" {
		t.Fatalf("legacy credential = %+v", credentials[0])
	}
	if credentials[1].Name != "monitor" || credentials[1].Token != "monitor-secret" || credentials[1].Scopes[0] != "read" {
		t.Fatalf("scoped credential = %+v", credentials[1])
	}
}

func TestAdminCredentialsRequireScopedTokenEnvironment(t *testing.T) {
	cfg := config.Default()
	cfg.Admin.Tokens = []config.AdminToken{{Name: "monitor", Env: "GOPROXY_MONITOR_TOKEN", Scopes: []string{"read"}}}
	if _, err := adminCredentials(cfg); err == nil || !strings.Contains(err.Error(), "GOPROXY_MONITOR_TOKEN") {
		t.Fatalf("error = %v", err)
	}
}

func TestVersionStringIncludesBuildMetadata(t *testing.T) {
	previousVersion, previousCommit := version, commit
	t.Cleanup(func() { version, commit = previousVersion, previousCommit })
	version, commit = "v1.0.0", "abc1234"
	if got := versionString(); got != "goproxy version=v1.0.0 commit=abc1234" {
		t.Fatalf("version = %q", got)
	}
}

func TestRunHandlesVersionAndHealthcheckFlags(t *testing.T) {
	if err := run([]string{"-version"}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GOPROXY_ADMIN_TOKEN", "secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("Authorization = %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	if err := run([]string{"-healthcheck", server.URL}); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsMissingConfig(t *testing.T) {
	if err := run([]string{"-config", "missing.yaml"}); err == nil {
		t.Fatal("expected missing config error")
	}
}

func TestRunHandlesCheckConfigFlag(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	configPath := filepath.Join(t.TempDir(), "proxy.yaml")
	yaml := strings.Replace(startupYAML, "{{backend}}", upstream.URL, 1)
	if err := os.WriteFile(configPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-config", configPath, "-check-config"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunCheckConfigDoesNotRequireAdminTokenEnvironment(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	configPath := filepath.Join(t.TempDir(), "proxy.yaml")
	yaml := strings.Replace(startupYAML, "{{backend}}", upstream.URL, 1)
	yaml = strings.Replace(yaml, "admin:\n  address: 127.0.0.1:0", "admin:\n  address: 127.0.0.1:0\n  auth_token_env: GOPROXY_ADMIN_TOKEN", 1)
	if err := os.WriteFile(configPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-config", configPath, "-check-config"}); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsInvalidFlags(t *testing.T) {
	if err := run([]string{"-unknown"}); err == nil {
		t.Fatal("expected flag error")
	}
}

func TestRunContextStartsAndStopsServers(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	configPath := filepath.Join(t.TempDir(), "proxy.yaml")
	yaml := strings.Replace(startupYAML, "{{backend}}", upstream.URL, 1)
	if err := os.WriteFile(configPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- runContext(ctx, []string{"-config", configPath}) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not stop")
	}
}

func TestSetLogLevel(t *testing.T) {
	var level slog.LevelVar
	setLogLevel(&level, "debug")
	if level.Level() != slog.LevelDebug {
		t.Fatalf("debug level = %s", level.Level())
	}
	setLogLevel(&level, "warn")
	if level.Level() != slog.LevelWarn {
		t.Fatalf("warn level = %s", level.Level())
	}
	setLogLevel(&level, "error")
	if level.Level() != slog.LevelError {
		t.Fatalf("error level = %s", level.Level())
	}
	setLogLevel(&level, "unknown")
	if level.Level() != slog.LevelInfo {
		t.Fatalf("default level = %s", level.Level())
	}
}

func TestAdminTokenDisabledWhenEnvironmentNameEmpty(t *testing.T) {
	token, err := adminToken(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		t.Fatalf("token = %q", token)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}

const startupYAML = `
listeners:
  - address: localhost:0
admin:
  address: 127.0.0.1:0
routes:
  - name: api
    host: api.test
    path_prefix: /
    pool: api
    strategy: round_robin
pools:
  api:
    backends:
      - name: one
        url: {{backend}}
    health:
      path: /health
      interval: 1h
      timeout: 1s
      unhealthy_threshold: 1
      healthy_threshold: 1
    retry:
      max_attempts: 1
`
