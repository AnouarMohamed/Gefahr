package proxy

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestNewTransportAppliesTimeoutsAndBounds(t *testing.T) {
	cfg := proxyConfig()
	cfg.Timeouts.Dial = config.Duration(3 * time.Second)
	cfg.Timeouts.ResponseHeader = config.Duration(7 * time.Second)
	transport := NewTransport(cfg)
	if transport.Proxy != nil {
		t.Fatal("backend transport unexpectedly honors ambient proxy variables")
	}
	if transport.ResponseHeaderTimeout != 7*time.Second {
		t.Fatalf("response header timeout = %s", transport.ResponseHeaderTimeout)
	}
	if transport.MaxIdleConns <= 0 || transport.MaxIdleConnsPerHost <= 0 {
		t.Fatal("idle connection pool is unbounded")
	}
	if transport.MaxResponseHeaderBytes != int64(cfg.Limits.MaxHeaderBytes) || transport.MaxConnsPerHost <= 0 {
		t.Fatal("upstream response headers or connections are unbounded")
	}
	if !transport.ForceAttemptHTTP2 {
		t.Fatal("HTTP/2 is disabled")
	}
}

func TestNewPoolTransportLoadsUpstreamTLS(t *testing.T) {
	cfg := proxyConfig()
	pool := cfg.Pools["api"]
	pool.Backends[0].URL = "https://backend.test"
	pool.TLS = config.PoolTLS{CAFile: writeTestCA(t), ServerName: "backend.internal", InsecureSkipVerify: true}
	transport, err := NewPoolTransport(cfg, pool)
	if err != nil {
		t.Fatal(err)
	}
	if transport.TLSClientConfig == nil {
		t.Fatal("TLS client config was not installed")
	}
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("minimum TLS version = %x", transport.TLSClientConfig.MinVersion)
	}
	if transport.TLSClientConfig.ServerName != "backend.internal" || transport.TLSClientConfig.RootCAs == nil {
		t.Fatalf("TLS client config = %+v", transport.TLSClientConfig)
	}
}

func TestNewPoolTransportRejectsInvalidCAFile(t *testing.T) {
	cfg := proxyConfig()
	pool := cfg.Pools["api"]
	pool.Backends[0].URL = "https://backend.test"
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	pool.TLS.CAFile = path
	if _, err := NewPoolTransport(cfg, pool); err == nil {
		t.Fatal("expected invalid CA error")
	}
}

func writeTestCA(t *testing.T) string {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test ca"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
