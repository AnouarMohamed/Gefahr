package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

// Managed describes one HTTP server and its optional TLS policy.
type Managed struct {
	HTTP           *http.Server
	TLS            *tls.Config
	MaxConnections int
}

// NewPublic creates a bounded public HTTP server.
func NewPublic(listener config.Listener, cfg config.Config, handler http.Handler, tlsConfig *tls.Config) Managed {
	return Managed{HTTP: &http.Server{Addr: listener.Address, Handler: handler, ReadHeaderTimeout: cfg.Timeouts.ReadHeader.Value(), ReadTimeout: cfg.Timeouts.ReadBody.Value(), WriteTimeout: cfg.Timeouts.Write.Value(), IdleTimeout: cfg.Timeouts.Idle.Value(), MaxHeaderBytes: cfg.Limits.MaxHeaderBytes}, TLS: tlsConfig, MaxConnections: cfg.Limits.MaxConnections}
}

// NewAdmin creates a loopback-oriented operational HTTP server.
func NewAdmin(cfg config.Config, handler http.Handler) Managed {
	return Managed{HTTP: &http.Server{Addr: cfg.Admin.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8 << 10}, MaxConnections: 128}
}

// Run opens every listener before serving and drains all servers when ctx is
// canceled. A bind or serve failure shuts down the complete group.
func Run(ctx context.Context, servers []Managed, shutdownTimeout time.Duration) error {
	listeners := make([]net.Listener, 0, len(servers))
	for _, managed := range servers {
		listener, err := net.Listen("tcp", managed.HTTP.Addr)
		if err != nil {
			for _, open := range listeners {
				open.Close()
			}
			return fmt.Errorf("listen on %s: %w", managed.HTTP.Addr, err)
		}
		listener = newLimitListener(listener, managed.MaxConnections)
		if managed.TLS != nil {
			listener = tls.NewListener(listener, managed.TLS)
		}
		listeners = append(listeners, listener)
	}

	errCh := make(chan error, len(servers))
	var wg sync.WaitGroup
	for i, managed := range servers {
		wg.Add(1)
		go func(managed Managed, listener net.Listener) {
			defer wg.Done()
			if err := managed.HTTP.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}(managed, listeners[i])
	}

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	var shutdownErrs []error
	for _, managed := range servers {
		if err := managed.HTTP.Shutdown(shutdownCtx); err != nil {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("shutdown %s: %w", managed.HTTP.Addr, err))
		}
	}
	// Shutdown does not force-close connections after its context expires.
	// Close every server before waiting so shutdown remains strictly bounded.
	for _, managed := range servers {
		if err := managed.HTTP.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			shutdownErrs = append(shutdownErrs, fmt.Errorf("close %s: %w", managed.HTTP.Addr, err))
		}
	}
	wg.Wait()
	return errors.Join(runErr, errors.Join(shutdownErrs...))
}

type limitListener struct {
	net.Listener
	sem       chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func newLimitListener(listener net.Listener, limit int) net.Listener {
	if limit <= 0 {
		return listener
	}
	return &limitListener{Listener: listener, sem: make(chan struct{}, limit), closed: make(chan struct{})}
}

func (l *limitListener) Accept() (net.Conn, error) {
	select {
	case l.sem <- struct{}{}:
	case <-l.closed:
		return nil, net.ErrClosed
	}
	connection, err := l.Listener.Accept()
	if err != nil {
		<-l.sem
		return nil, err
	}
	return &limitConn{Conn: connection, release: func() { <-l.sem }}, nil
}

func (l *limitListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return l.Listener.Close()
}

type limitConn struct {
	net.Conn
	releaseOnce sync.Once
	release     func()
}

func (c *limitConn) Close() error {
	err := c.Conn.Close()
	c.releaseOnce.Do(c.release)
	return err
}
