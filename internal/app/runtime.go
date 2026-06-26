// Package app coordinates reloadable proxy runtime state.
package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/anrorg/gefahr/internal/config"
	"github.com/anrorg/gefahr/internal/proxy"
	"github.com/anrorg/gefahr/internal/server"
)

// Runtime owns the active proxy, health workers, configuration, and TLS stores.
type Runtime struct {
	path         string
	observer     proxy.Observer
	dynamic      *proxy.Dynamic
	current      atomic.Pointer[config.Config]
	tlsStores    []*server.CertificateStore
	mu           sync.Mutex
	healthCancel context.CancelFunc
}

// New builds a complete runtime and validates all startup TLS material.
func New(cfg config.Config, path string, observer proxy.Observer) (*Runtime, error) {
	handler, err := proxy.NewHandler(cfg, nil, proxy.WithObserver(observer))
	if err != nil {
		return nil, err
	}
	runtime := &Runtime{path: path, observer: observer, dynamic: proxy.NewDynamic(handler), tlsStores: make([]*server.CertificateStore, len(cfg.Listeners))}
	for i, listener := range cfg.Listeners {
		if listener.TLS == nil {
			continue
		}
		store := new(server.CertificateStore)
		if err := store.Load(*listener.TLS); err != nil {
			return nil, fmt.Errorf("listener %d: %w", i, err)
		}
		runtime.tlsStores[i] = store
	}
	runtime.current.Store(&cfg)
	reconcileObserver(observer, cfg)
	return runtime, nil
}

// Handler returns the atomically reloadable public handler.
func (r *Runtime) Handler() *proxy.Dynamic { return r.dynamic }

// Ready reports whether the active runtime can route every configured pool.
func (r *Runtime) Ready() bool { return r.dynamic.Ready() }

// Config returns the active immutable configuration snapshot.
func (r *Runtime) Config() *config.Config { return r.current.Load() }

// TLSConfig returns the dynamic TLS policy for listener index, or nil for HTTP.
func (r *Runtime) TLSConfig(index int) *tls.Config {
	if r.tlsStores[index] == nil {
		return nil
	}
	return r.tlsStores[index].TLSConfig()
}

// StartHealthChecks starts workers tied to parent.
func (r *Runtime) StartHealthChecks(parent context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startHealthLocked(parent)
}

// Reload validates and stages a complete snapshot before publication. Listener
// addresses and TLS mode are restart-only because changing bound sockets cannot
// be atomic with in-flight requests.
func (r *Runtime) Reload(parent context.Context) error {
	next, err := config.LoadFile(r.path)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := immutableCompatible(*r.current.Load(), next); err != nil {
		return err
	}
	handler, err := proxy.NewHandler(next, nil, proxy.WithObserver(r.observer))
	if err != nil {
		return err
	}
	staged := make([]*tls.Certificate, len(next.Listeners))
	for i, listener := range next.Listeners {
		if listener.TLS == nil {
			continue
		}
		certificate, err := server.LoadCertificate(*listener.TLS)
		if err != nil {
			return fmt.Errorf("listener %d: %w", i, err)
		}
		staged[i] = certificate
	}
	for i, certificate := range staged {
		if certificate != nil {
			r.tlsStores[i].Publish(certificate)
		}
	}
	previous := r.dynamic.Current()
	handler.InheritBackendHealth(previous)
	reconcileObserver(r.observer, next)
	r.dynamic.Swap(handler)
	r.current.Store(&next)
	r.startHealthLocked(parent)
	previous.Retire()
	return nil
}

func reconcileObserver(observer proxy.Observer, cfg config.Config) {
	if reconciler, ok := observer.(interface{ ReconcileConfig(config.Config) }); ok {
		reconciler.ReconcileConfig(cfg)
	}
}

func (r *Runtime) startHealthLocked(parent context.Context) {
	if r.healthCancel != nil {
		r.healthCancel()
	}
	ctx, cancel := context.WithCancel(parent)
	r.healthCancel = cancel
	r.dynamic.Current().StartHealthChecks(ctx, *r.current.Load())
}

func immutableCompatible(current, next config.Config) error {
	var errs []error
	if current.Admin.Address != next.Admin.Address {
		errs = append(errs, errors.New("admin.address requires restart"))
	}
	if current.Admin.AuthTokenEnv != next.Admin.AuthTokenEnv {
		errs = append(errs, errors.New("admin.auth_token_env requires restart"))
	}
	if !reflect.DeepEqual(current.Admin.Tokens, next.Admin.Tokens) {
		errs = append(errs, errors.New("admin.tokens requires restart"))
	}
	if len(current.Listeners) != len(next.Listeners) {
		errs = append(errs, errors.New("listener count requires restart"))
		return errors.Join(errs...)
	}
	for i := range current.Listeners {
		if current.Listeners[i].Address != next.Listeners[i].Address {
			errs = append(errs, fmt.Errorf("listeners[%d].address requires restart", i))
		}
		if (current.Listeners[i].TLS == nil) != (next.Listeners[i].TLS == nil) {
			errs = append(errs, fmt.Errorf("listeners[%d] TLS mode requires restart", i))
		}
	}
	if current.Timeouts.ReadHeader != next.Timeouts.ReadHeader {
		errs = append(errs, errors.New("timeouts.read_header requires restart"))
	}
	if current.Timeouts.ReadBody != next.Timeouts.ReadBody {
		errs = append(errs, errors.New("timeouts.read_body requires restart"))
	}
	if current.Timeouts.Write != next.Timeouts.Write {
		errs = append(errs, errors.New("timeouts.write requires restart"))
	}
	if current.Timeouts.Idle != next.Timeouts.Idle {
		errs = append(errs, errors.New("timeouts.idle requires restart"))
	}
	if current.Timeouts.Shutdown != next.Timeouts.Shutdown {
		errs = append(errs, errors.New("timeouts.shutdown requires restart"))
	}
	if current.Limits.MaxHeaderBytes != next.Limits.MaxHeaderBytes {
		errs = append(errs, errors.New("limits.max_header_bytes requires restart"))
	}
	if current.Limits.MaxConnections != next.Limits.MaxConnections {
		errs = append(errs, errors.New("limits.max_connections requires restart"))
	}
	return errors.Join(errs...)
}
