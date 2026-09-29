// Package http serves the DevPulse read-only JSON API that `devpulse
// serve` exposes. The desktop dashboard (desktop/) is its first client.
//
// Every route under /api/ requires `Authorization: Bearer <token>`;
// /healthz is open so load balancers and the dashboard's connection
// test can probe liveness without a credential.
package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mileschou/devpulse/internal/metrics"
	"github.com/mileschou/devpulse/internal/repo"
)

// RepoStore is the subset of persistence.RepoPersister the API reads.
type RepoStore interface {
	ListAll(ctx context.Context) ([]repo.Repo, error)
	FindByFullName(ctx context.Context, provider string, fullName repo.FullName) (repo.Repo, error)
}

// Config describes the listening surface and the data the handlers read.
type Config struct {
	Addr            string        // "127.0.0.1:8080"; empty disables listening
	Token           string        // bearer token required on /api/ routes
	ReadTimeout     time.Duration // default 5s
	WriteTimeout    time.Duration // default 30s; monthly trends run many queries
	ShutdownTimeout time.Duration // default 5s
	Logger          *slog.Logger

	Repos   RepoStore
	Metrics metrics.Source
	Now     func() time.Time // default time.Now; resolves the default month window
}

// Server wraps an http.Server.
type Server struct {
	cfg    Config
	logger *slog.Logger
	srv    *http.Server
}

// ErrTokenRequired is returned by CheckBind when the address is
// reachable from other hosts but no API token is configured.
var ErrTokenRequired = errors.New("http: DEVPULSE_API_TOKEN is required when HTTP_ADDR is not a loopback address")

// CheckBind rejects an unauthenticated API on a non-loopback address.
// Without a token the API is open to anyone who can reach the port, and
// it exposes every tracked repo's data, so only a loopback listener may
// run without one.
func CheckBind(addr, token string) error {
	if token != "" {
		return nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("http: invalid HTTP_ADDR %q: %w", addr, err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	// An empty host (":8080") listens on every interface.
	return ErrTokenRequired
}

// New constructs a Server. The http.Server is built but not started
// until Start is called.
func New(cfg Config) *Server {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	return &Server{
		cfg:    cfg,
		logger: cfg.Logger,
		srv: &http.Server{
			Addr:         cfg.Addr,
			Handler:      NewHandler(cfg),
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
	}
}

// Start listens until ctx is canceled, then performs graceful shutdown.
// With an empty Addr it only waits for ctx, which keeps tests and
// callers that disable the listener simple.
func (s *Server) Start(ctx context.Context) error {
	if s.cfg.Addr == "" {
		<-ctx.Done()
		return nil
	}

	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("http: listen %s: %w", s.cfg.Addr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server listening", slog.String("addr", ln.Addr().String()))
		err := s.srv.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
