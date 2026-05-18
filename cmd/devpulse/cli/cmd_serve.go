package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newServeCmd is a v1 placeholder for the future HTTP API surface (see
// internal/http). The command exists so wiring is in place; running it
// today prints a "not implemented" notice and exits 0.
func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the DevPulse HTTP API (v2 — placeholder in v1)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(stdout(), "serve: HTTP API not implemented in v1 (see internal/http).")
			return nil
		},
	}
}

