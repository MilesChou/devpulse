package cli

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/config"
	apihttp "github.com/mileschou/devpulse/internal/http"
	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
	"github.com/mileschou/devpulse/internal/syncrun"
)

// newServeCmd runs the JSON API (see internal/http) until SIGINT /
// SIGTERM. The desktop dashboard is its client.
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the DevPulse HTTP API (long-running, ^C to stop)",
		Long: "Serves repos and metrics as JSON on HTTP_ADDR (default 127.0.0.1:8080). " +
			"Every /api/ request must carry `Authorization: Bearer $DEVPULSE_API_TOKEN`. " +
			"The token is required unless HTTP_ADDR is a loopback address.",
		Example: "  DEVPULSE_API_TOKEN=secret HTTP_ADDR=0.0.0.0:8080 devpulse serve",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Validate the bind before opening the DB, so a misconfigured
			// deploy fails fast with the config error rather than later.
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := apihttp.CheckBind(cfg.HTTPAddr, cfg.APIToken); err != nil {
				return err
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			d, err := buildDeps(ctx)
			if err != nil {
				return err
			}
			defer d.close(ctx)

			apiCfg := apihttp.Config{
				Addr:    d.cfg.HTTPAddr,
				Token:   d.cfg.APIToken,
				Repos:   d.repos,
				Metrics: persistence.NewMetricsPersister(d.pers),
				Admin:   d.admin,
			}
			// Syncing needs GitHub; without a token the sync endpoints
			// answer 503 and everything else keeps working.
			var runner *syncrun.Runner
			if strings.TrimSpace(d.cfg.GitHubToken) != "" {
				runner = syncrun.New(ctx, func(ctx context.Context, r repo.Repo) error {
					return syncOneRepo(ctx, d, r)
				}, nil)
				apiCfg.Sync = runner
			}
			srv := apihttp.New(apiCfg)
			fmt.Fprintf(stdout(), "serving on http://%s; press Ctrl-C to stop\n", d.cfg.HTTPAddr)
			if err := srv.Start(ctx); err != nil {
				return err
			}
			if runner != nil {
				// ctx is cancelled, so a sync in progress is stopping;
				// wait for it before closing the DB underneath it.
				runner.Wait()
			}
			fmt.Fprintln(stdout(), "server stopped")
			return nil
		},
	}
}
