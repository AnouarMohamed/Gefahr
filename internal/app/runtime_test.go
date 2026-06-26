package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestReloadRejectsListenerMutationAndRetainsConfig(t *testing.T) {
	cfg, err := config.Load(strings.NewReader(testYAML))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "proxy.yaml")
	changed := strings.Replace(testYAML, "address: :8080", "address: :8081", 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := New(cfg, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Reload(context.Background()); err == nil || !strings.Contains(err.Error(), "requires restart") {
		t.Fatalf("error = %v", err)
	}
	if runtime.Config().Listeners[0].Address != ":8080" {
		t.Fatal("rejected config was published")
	}
}

func TestRuntimeAccessorsAndHealthChecks(t *testing.T) {
	cfg, err := config.Load(strings.NewReader(testYAML))
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.Handler() == nil || runtime.Config().Routes[0].Name != "api" {
		t.Fatal("runtime accessors returned unexpected values")
	}
	if !runtime.Ready() {
		t.Fatal("runtime should start ready")
	}
	if runtime.TLSConfig(0) != nil {
		t.Fatal("plain listener returned TLS config")
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtime.StartHealthChecks(ctx)
	cancel()
	time.Sleep(10 * time.Millisecond)
}

func TestRuntimeLoadsListenerTLSConfig(t *testing.T) {
	cfg, err := config.Load(strings.NewReader(testYAML))
	if err != nil {
		t.Fatal(err)
	}
	tlsConfig := writeRuntimeTestCertificate(t)
	cfg.Listeners[0].TLS = &tlsConfig
	runtime, err := New(cfg, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := runtime.TLSConfig(0).GetCertificate(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if certificate.Leaf == nil || certificate.Leaf.Subject.CommonName != "localhost" {
		t.Fatalf("certificate = %+v", certificate.Leaf)
	}
}

func TestReloadPublishesMutableConfig(t *testing.T) {
	cfg, err := config.Load(strings.NewReader(testYAML))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "proxy.yaml")
	if err := os.WriteFile(path, []byte(testYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	runtime, err := New(cfg, path, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changed := strings.Replace(testYAML, "strategy: round_robin", "strategy: least_connections", 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if got := runtime.Config().Routes[0].Strategy; got != "least_connections" {
		t.Fatalf("strategy = %q", got)
	}
}

func TestImmutableCompatibleRejectsServerLevelBounds(t *testing.T) {
	current := config.Default()
	next := current
	next.Admin.AuthTokenEnv = "GOPROXY_ADMIN_TOKEN"
	next.Admin.Tokens = []config.AdminToken{{Name: "monitor", Env: "GOPROXY_MONITOR_TOKEN", Scopes: []string{"read"}}}
	next.Timeouts.Write++
	next.Limits.MaxHeaderBytes++
	err := immutableCompatible(current, next)
	if err == nil || !strings.Contains(err.Error(), "admin.auth_token_env") || !strings.Contains(err.Error(), "admin.tokens") || !strings.Contains(err.Error(), "timeouts.write") || !strings.Contains(err.Error(), "limits.max_header_bytes") {
		t.Fatalf("error = %v", err)
	}
}

const testYAML = `
listeners:
  - address: :8080
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
        url: http://127.0.0.1:9001
    health:
      path: /health
      interval: 5s
      timeout: 1s
      unhealthy_threshold: 2
      healthy_threshold: 1
    retry:
      max_attempts: 1
`

func writeRuntimeTestCertificate(t *testing.T) config.TLSConfig {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600); err != nil {
		t.Fatal(err)
	}
	return config.TLSConfig{CertFile: certPath, KeyFile: keyPath}
}
