package cli

import (
	"context"
	"fmt"

	"github.com/mileschou/devpulse/internal/config"
	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/github"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/migrator"
	"github.com/mileschou/devpulse/internal/travis"
	"github.com/mileschou/devpulse/internal/x/logx"
	"github.com/mileschou/devpulse/internal/x/otelx"
	"github.com/mileschou/devpulse/migrations"
)

// deps is the resolved bundle of services every subcommand may need.
// Constructed lazily by buildDeps so commands that don't need a DB (e.g.
// `--help`, `--version`) don't pay the cost of opening one.
type deps struct {
	cfg   config.Config
	conn  *persistence.Connection
	pers  *persistence.Persister
	orch  *fetching.Orchestrator
	repos *persistence.RepoPersister
	tp    *otelx.Provider
}

// buildDeps assembles the dependency graph. Caller MUST call deps.close().
func buildDeps(ctx context.Context) (*deps, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger := logx.New(logx.Config{
		Level:       cfg.LogLevel,
		Format:      logFormatFromCfg(cfg.LogFormat),
		ServiceName: cfg.ServiceName,
	})

	tp, err := otelx.NewProvider(ctx, otelx.Config{
		ServiceName:    cfg.ServiceName,
		ServiceVersion: cfg.ServiceVersion,
		Environment:    cfg.Environment,
		Endpoint:       cfg.OTLPEndpoint,
		Insecure:       true,
		SampleRatio:    cfg.OTELSample,
	})
	if err != nil {
		return nil, fmt.Errorf("otel: %w", err)
	}

	conn, err := persistence.Open(ctx, cfg.DSN)
	if err != nil {
		_ = tp.Shutdown(ctx)
		return nil, err
	}

	// Memory DSNs are fresh on every process start, so auto-apply
	// migrations so the binary "just works" for tests and one-off CLI
	// invocations. Mirrors ory/hydra's behaviour.
	if conn.IsMemory {
		if err := migrator.New(conn.DB, conn.Dialect, migrations.FS, logger).MigrateUp(ctx); err != nil {
			_ = conn.DB.Close()
			_ = tp.Shutdown(ctx)
			return nil, fmt.Errorf("memory migrate: %w", err)
		}
	}

	pers := persistence.New(conn, logger)
	repos := persistence.NewRepoPersister(pers)
	builds := persistence.NewBuildPersister(pers)
	prs := persistence.NewPullRequestPersister(pers)
	reviews := persistence.NewReviewPersister(pers)

	ghClient := github.NewClient(github.Config{
		BaseURL:  cfg.GitHubBase,
		Token:    cfg.GitHubToken,
		Timeout:  cfg.HTTPTimeout,
		RetryMax: cfg.HTTPRetry,
		Logger:   logger,
	})
	travisClient := travis.NewClient(travis.Config{
		BaseURL:  cfg.TravisBase,
		Token:    cfg.TravisToken,
		Timeout:  cfg.HTTPTimeout,
		RetryMax: cfg.HTTPRetry,
		Logger:   logger,
	})

	orch := fetching.NewOrchestrator(
		travis.NewProvider(travisClient),
		github.NewProvider(ghClient),
		builds, prs, reviews,
		logger,
	)

	return &deps{
		cfg:   cfg,
		conn:  conn,
		pers:  pers,
		orch:  orch,
		repos: repos,
		tp:    tp,
	}, nil
}

func (d *deps) close(ctx context.Context) {
	if d == nil {
		return
	}
	if d.conn != nil && d.conn.DB != nil {
		_ = d.conn.DB.Close()
	}
	if d.tp != nil {
		_ = d.tp.Shutdown(ctx)
	}
}

func logFormatFromCfg(s string) logx.Format {
	if s == "text" {
		return logx.FormatText
	}
	return logx.FormatJSON
}
