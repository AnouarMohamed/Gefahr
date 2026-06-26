package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

// NewTransport returns a bounded, HTTP/2-capable upstream connection pool.
func NewTransport(cfg config.Config) *http.Transport {
	transport, err := newTransport(cfg, config.PoolTLS{})
	if err != nil {
		panic(err)
	}
	return transport
}

// NewPoolTransport returns a bounded upstream connection pool with pool TLS policy.
func NewPoolTransport(cfg config.Config, pool config.Pool) (*http.Transport, error) {
	return newTransport(cfg, pool.TLS)
}

func newTransport(cfg config.Config, upstreamTLS config.PoolTLS) (*http.Transport, error) {
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = nil
	base.DialContext = (&net.Dialer{Timeout: cfg.Timeouts.Dial.Value(), KeepAlive: 30 * time.Second}).DialContext
	base.ForceAttemptHTTP2 = true
	base.ResponseHeaderTimeout = cfg.Timeouts.ResponseHeader.Value()
	base.MaxResponseHeaderBytes = int64(cfg.Limits.MaxHeaderBytes)
	base.IdleConnTimeout = cfg.Timeouts.Idle.Value()
	base.MaxIdleConns = 256
	base.MaxIdleConnsPerHost = 32
	base.MaxConnsPerHost = 128
	tlsConfig, err := upstreamTLSConfig(upstreamTLS)
	if err != nil {
		return nil, err
	}
	base.TLSClientConfig = tlsConfig
	return base, nil
}

func upstreamTLSConfig(cfg config.PoolTLS) (*tls.Config, error) {
	if !upstreamTLSConfigured(cfg) {
		return nil, nil
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: cfg.ServerName, InsecureSkipVerify: cfg.InsecureSkipVerify}
	if cfg.CAFile != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, fmt.Errorf("load system trust roots: %w", err)
		}
		if roots == nil {
			roots = x509.NewCertPool()
		}
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("load upstream CA file: %w", err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("load upstream CA file: no certificates found")
		}
		tlsConfig.RootCAs = roots
	}
	if cfg.ClientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load upstream client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return tlsConfig, nil
}

func upstreamTLSConfigured(cfg config.PoolTLS) bool {
	return cfg.CAFile != "" || cfg.ServerName != "" || cfg.ClientCertFile != "" || cfg.ClientKeyFile != "" || cfg.InsecureSkipVerify
}
