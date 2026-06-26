// Command goproxy starts the reverse proxy and its operational listener.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/anrorg/gefahr/internal/admin"
	"github.com/anrorg/gefahr/internal/app"
	"github.com/anrorg/gefahr/internal/config"
	"github.com/anrorg/gefahr/internal/metrics"
	"github.com/anrorg/gefahr/internal/observability"
	"github.com/anrorg/gefahr/internal/server"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	return runContext(context.Background(), args)
}

func runContext(parent context.Context, args []string) error {
	flags := flag.NewFlagSet("goproxy", flag.ContinueOnError)
	configPath := flags.String("config", "configs/proxy.example.yaml", "path to YAML configuration")
	checkConfig := flags.Bool("check-config", false, "validate configuration and exit")
	healthcheck := flags.String("healthcheck", "", "check an HTTP readiness URL and exit")
	showVersion := flags.Bool("version", false, "print version information and exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println(versionString())
		return nil
	}
	if *healthcheck != "" {
		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Proxy:                 nil,
				DialContext:           (&net.Dialer{Timeout: 2 * time.Second}).DialContext,
				ResponseHeaderTimeout: 2 * time.Second,
			},
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		return runHealthcheck(client, *healthcheck, os.Getenv("GOPROXY_ADMIN_TOKEN"))
	}
	cfg, err := config.LoadFile(*configPath)
	if err != nil {
		return err
	}
	if *checkConfig {
		fmt.Printf("configuration ok: %s\n", *configPath)
		return nil
	}
	adminCredentials, err := adminCredentials(cfg)
	if err != nil {
		return err
	}

	level := new(slog.LevelVar)
	setLogLevel(level, cfg.Logging.Level)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	metricSet := metrics.New()
	observer := observability.Fanout{Requests: []observability.RequestObserver{observability.RequestLogger{Logger: logger}, metricSet}, Backends: []observability.BackendObserver{metricSet}}
	runtime, err := app.New(cfg, *configPath, observer)
	if err != nil {
		return err
	}

	signalCtx, stopSignals := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()
	var live atomic.Bool
	live.Store(true)
	adminHandler := admin.NewHandler(live.Load, runtime.Ready, metricSet.Handler(), "", admin.WithCredentials(adminCredentials), admin.WithAuditLogger(logger))

	managed := make([]server.Managed, 0, len(cfg.Listeners)+1)
	for i, listener := range cfg.Listeners {
		managed = append(managed, server.NewPublic(listener, cfg, runtime.Handler(), runtime.TLSConfig(i)))
	}
	managed = append(managed, server.NewAdmin(cfg, adminHandler))
	runtime.StartHealthChecks(ctx)

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hup:
				if err := runtime.Reload(ctx); err != nil {
					logger.Error("configuration reload rejected", "error", err)
					continue
				}
				setLogLevel(level, runtime.Config().Logging.Level)
				logger.Info("configuration reloaded")
			}
		}
	}()

	logger.Info("goproxy starting", "listeners", len(cfg.Listeners), "admin", cfg.Admin.Address)
	err = server.Run(ctx, managed, cfg.Timeouts.Shutdown.Value())
	live.Store(false)
	return err
}

func versionString() string {
	return fmt.Sprintf("goproxy version=%s commit=%s", version, commit)
}

func runHealthcheck(client *http.Client, target, bearerToken string) error {
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("healthcheck request: %w", err)
	}
	if bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("healthcheck request: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck status %d", response.StatusCode)
	}
	return nil
}

func adminToken(cfg config.Config) (string, error) {
	if cfg.Admin.AuthTokenEnv == "" {
		return "", nil
	}
	token := os.Getenv(cfg.Admin.AuthTokenEnv)
	if token == "" {
		return "", fmt.Errorf("admin auth token environment variable %s is empty or unset", cfg.Admin.AuthTokenEnv)
	}
	return token, nil
}

func adminCredentials(cfg config.Config) ([]admin.Credential, error) {
	var credentials []admin.Credential
	if cfg.Admin.AuthTokenEnv != "" {
		token, err := adminToken(cfg)
		if err != nil {
			return nil, err
		}
		credentials = append(credentials, admin.Credential{Name: "legacy-admin", Token: token, Scopes: []string{"admin"}})
	}
	for _, configured := range cfg.Admin.Tokens {
		token := os.Getenv(configured.Env)
		if token == "" {
			return nil, fmt.Errorf("admin token environment variable %s is empty or unset", configured.Env)
		}
		credentials = append(credentials, admin.Credential{Name: configured.Name, Token: token, Scopes: configured.Scopes})
	}
	return credentials, nil
}

func setLogLevel(level *slog.LevelVar, configured string) {
	switch strings.ToLower(configured) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
}
