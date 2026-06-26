package server

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anrorg/gefahr/internal/config"
)

func TestNewPublicAppliesRequestBounds(t *testing.T) {
	cfg := config.Default()
	cfg.Timeouts.ReadHeader = config.Duration(3 * time.Second)
	cfg.Timeouts.ReadBody = config.Duration(20 * time.Second)
	cfg.Timeouts.Write = config.Duration(45 * time.Second)
	cfg.Limits.MaxHeaderBytes = 12345
	managed := NewPublic(cfg.Listeners[0], cfg, http.NotFoundHandler(), nil)
	if managed.HTTP.ReadHeaderTimeout != 3*time.Second || managed.HTTP.MaxHeaderBytes != 12345 {
		t.Fatalf("server = %#v", managed.HTTP)
	}
	if managed.HTTP.ReadTimeout != 20*time.Second || managed.HTTP.WriteTimeout != 45*time.Second {
		t.Fatal("body read and client write deadlines were not applied")
	}
	if managed.MaxConnections != cfg.Limits.MaxConnections {
		t.Fatal("public connection limit was not applied")
	}
}

func TestNewAdminAppliesWriteDeadline(t *testing.T) {
	cfg := config.Default()
	managed := NewAdmin(cfg, http.NotFoundHandler())
	if managed.HTTP.WriteTimeout <= 0 || managed.HTTP.ReadHeaderTimeout <= 0 || managed.HTTP.MaxHeaderBytes <= 0 {
		t.Fatalf("admin server is not fully bounded: %#v", managed.HTTP)
	}
}

func TestLimitListenerCloseUnblocksCapacityWaiters(t *testing.T) {
	base := &blockingListener{entered: make(chan struct{}), closed: make(chan struct{})}
	listener := newLimitListener(base, 1)
	done := make(chan struct{}, 2)
	go func() { _, _ = listener.Accept(); done <- struct{}{} }()
	<-base.entered
	go func() { _, _ = listener.Accept(); done <- struct{}{} }()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Accept remained blocked after listener close")
		}
	}
}

func TestRunServesRequestsAndStopsOnContextCancel(t *testing.T) {
	addr := freeAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, []Managed{{HTTP: &http.Server{Addr: addr, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		})}, MaxConnections: 16}}, time.Second)
	}()

	waitForHTTP(t, "http://"+addr)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestRunReportsBindFailure(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()

	err = Run(context.Background(), []Managed{
		{HTTP: &http.Server{Addr: freeAddress(t), Handler: http.NotFoundHandler()}},
		{HTTP: &http.Server{Addr: occupied.Addr().String(), Handler: http.NotFoundHandler()}},
	}, time.Second)
	if err == nil || !strings.Contains(err.Error(), "listen on") {
		t.Fatalf("bind error = %v", err)
	}
}

func TestLimitConnReleasesCapacityOnce(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	var releases atomic.Int64
	conn := &limitConn{Conn: client, release: func() { releases.Add(1) }}
	_ = conn.Close()
	_ = conn.Close()
	if releases.Load() != 1 {
		t.Fatalf("releases = %d", releases.Load())
	}
}

func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitForHTTP(t *testing.T, target string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(target)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not become ready at %s", target)
}

type blockingListener struct {
	entered   chan struct{}
	closed    chan struct{}
	enterOnce sync.Once
	closeOnce sync.Once
}

func (l *blockingListener) Accept() (net.Conn, error) {
	l.enterOnce.Do(func() { close(l.entered) })
	<-l.closed
	return nil, net.ErrClosed
}

func (l *blockingListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return nil
}

func (*blockingListener) Addr() net.Addr { return &net.TCPAddr{} }
