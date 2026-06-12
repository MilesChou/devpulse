package cli

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/mileschou/devpulse/internal/config"
	"github.com/mileschou/devpulse/internal/fetching"
	"github.com/mileschou/devpulse/internal/github"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/persistence/dsn"
	"github.com/mileschou/devpulse/internal/persistence/migrator"
	"github.com/mileschou/devpulse/internal/travis"
	"github.com/mileschou/devpulse/internal/x/httpcache"
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
	vcs   fetching.VCSProvider
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
	if dsn.IsMemory(cfg.DSN) {
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

	// Build an optional cache transport shared by all API clients.
	// When CACHE_ENABLED=true, successful responses are stored on
	// disk. One TTL governs every request — no per-endpoint
	// classification. Most synced resources mutate (open PRs merge,
	// reviews land, watermarked build listings answer the same URL
	// with new content), so a "never expire" bucket silently freezes
	// data; entries past the TTL revalidate with ETag /
	// If-Modified-Since instead, and a 304 costs no GitHub rate limit.
	// Set CACHE_TTL=0 explicitly for replay mode (entries never
	// expire) — intended only for rebuilding the DB from previously
	// fetched responses without network access.
	// CacheDir is already resolved by config.Load (defaults to
	// os.UserCacheDir()/devpulse when empty).
	var cacheTransport http.RoundTripper
	if cfg.CacheEnabled {
		store := httpcache.NewDiskStore(cfg.CacheDir)
		cacheTransport = httpcache.NewTransport(store, cfg.CacheTTL, logger)
		if cfg.CacheTTL == 0 {
			logger.Warn("http cache in replay mode (CACHE_TTL=0): " +
				"cached responses never expire and upstream changes are invisible")
		}
		logger.Info("http cache enabled",
			"dir", cfg.CacheDir,
			"ttl", cfg.CacheTTL.String(),
		)
	}

	// HTTPRetry is not plumbed through to the GitHub client. go-gh's
	// pkg/api does not implement retries, so the GitHub client currently
	// does not retry on any failure; adding retry is tracked separately.
	// Travis still honors HTTPRetry via internal/x/httpx.
	ghClient, err := github.NewClient(github.Config{
		BaseURL:       cfg.GitHubBase,
		Token:         cfg.GitHubToken,
		Timeout:       cfg.HTTPTimeout,
		Logger:        logger,
		BaseTransport: cacheTransport,
	})
	if err != nil {
		_ = conn.DB.Close()
		_ = tp.Shutdown(ctx)
		return nil, fmt.Errorf("github client: %w", err)
	}
	vcs := github.NewProvider(ghClient)

	ciProviders := []fetching.CIProvider{
		github.NewActionsProvider(ghClient),
	}
	if strings.TrimSpace(cfg.TravisToken) != "" {
		travisClient := travis.NewClient(travis.Config{
			BaseURL:       cfg.TravisBase,
			Token:         cfg.TravisToken,
			Timeout:       cfg.HTTPTimeout,
			RetryMax:      cfg.HTTPRetry,
			Logger:        logger,
			BaseTransport: cacheTransport,
		})
		ciProviders = append(ciProviders, travis.NewProvider(travisClient))
	}

	orch := fetching.NewOrchestrator(
		ciProviders,
		vcs,
		builds, prs, reviews,
		logger,
	)

	return &deps{
		cfg:   cfg,
		conn:  conn,
		pers:  pers,
		orch:  orch,
		vcs:   vcs,
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
