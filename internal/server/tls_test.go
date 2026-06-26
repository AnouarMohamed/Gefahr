package server

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
	"strings"
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestTLSConfigRequiresTLS12AndAdvertisesHTTP2(t *testing.T) {
	cfg := new(CertificateStore).TLSConfig()
	if cfg.MinVersion != tls.VersionTLS12 {
		t.Fatalf("minimum TLS version = %x", cfg.MinVersion)
	}
	if len(cfg.NextProtos) == 0 || cfg.NextProtos[0] != "h2" {
		t.Fatalf("ALPN protocols = %v", cfg.NextProtos)
	}
}

func TestTLSConfigServesPublishedCertificate(t *testing.T) {
	store := new(CertificateStore)
	cfg := writeTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	if err := store.Load(cfg); err != nil {
		t.Fatal(err)
	}
	certificate, err := store.TLSConfig().GetCertificate(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if certificate.Leaf == nil || certificate.Leaf.Subject.CommonName != "localhost" {
		t.Fatalf("certificate = %+v", certificate.Leaf)
	}

	replacement, err := LoadCertificate(writeTestCertificate(t, time.Now().Add(-time.Hour), time.Now().Add(time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	store.Publish(replacement)
	certificate, err = store.TLSConfig().GetCertificate(&tls.ClientHelloInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if certificate != replacement {
		t.Fatal("published certificate was not served")
	}
}

func TestTLSConfigRejectsEmptyStore(t *testing.T) {
	if _, err := new(CertificateStore).TLSConfig().GetCertificate(&tls.ClientHelloInfo{}); err == nil {
		t.Fatal("expected empty certificate store error")
	}
}

func TestCertificateStoreRejectsMissingPair(t *testing.T) {
	store := new(CertificateStore)
	if err := store.Load(config.TLSConfig{CertFile: "missing-cert.pem", KeyFile: "missing-key.pem"}); err == nil {
		t.Fatal("expected load error")
	}
}

func TestLoadCertificateRejectsExpiredLeaf(t *testing.T) {
	cfg := writeTestCertificate(t, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	if _, err := LoadCertificate(cfg); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("error = %v", err)
	}
}

func writeTestCertificate(t *testing.T, notBefore, notAfter time.Time) config.TLSConfig {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, NotBefore: notBefore, NotAfter: notAfter, KeyUsage: x509.KeyUsageDigitalSignature}
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
