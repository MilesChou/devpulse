// Package http is a placeholder for the v2 DevPulse HTTP API.
//
// v1 ships a no-op skeleton — New/Start/Shutdown — so main.go's wiring,
// config keys, and the `devpulse serve` command can land in v1 without
// committing to handler shapes. When the API arrives, only routes.go and
// handlers/ need to grow; main.go is already calling these methods.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Config describes the listening surface. v2 will grow auth/TLS/etc here.
type Config struct {
	Addr            string        // ":8080"
	ReadTimeout     time.Duration // default 5s
	WriteTimeout    time.Duration // default 10s
	ShutdownTimeout time.Duration // default 5s
	Logger          *slog.Logger
}

// Server wraps an http.Server. In v1 the routes are empty; Start is a
// no-op that returns immediately so the binary can survive `devpulse serve`
// without a panic.
type Server struct {
	cfg    Config
	logger *slog.Logger
	srv    *http.Server
}

// New constructs a v1 Server. The actual http.Server is built but never
// started until Start() is called.
func New(cfg Config) *Server {
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	mux := http.NewServeMux()
	registerRoutes(mux) // v1: nothing; v2: real handlers

	return &Server{
		cfg:    cfg,
		logger: cfg.Logger,
		srv: &http.Server{
			Addr:         cfg.Addr,
			Handler:      mux,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
		},
	}
}

// Start blocks until ctx is canceled, then performs graceful shutdown.
//
// v1: returns immediately because Addr is empty by convention — the
// `devpulse serve` command prints "not implemented" rather than binding
// a real port.
func (s *Server) Start(ctx context.Context) error {
	if s.cfg.Addr == "" {
		<-ctx.Done()
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("http server starting", slog.String("addr", s.cfg.Addr))
		err := s.srv.ListenAndServe()
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
