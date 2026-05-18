package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/persistence/migrator"
	"github.com/mileschou/devpulse/migrations"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Schema migrations",
	}
	cmd.AddCommand(newMigrateUpCmd(), newMigrateDownCmd(), newMigrateStatusCmd())
	return cmd
}

func newMigrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runMigrate(cmd.Context(), "up") },
	}
}

func newMigrateDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Roll back the most recent applied migration",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runMigrate(cmd.Context(), "down") },
	}
}

func newMigrateStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print applied migration versions",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runMigrate(cmd.Context(), "status") },
	}
}

func runMigrate(ctx context.Context, op string) error {
	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	m := migrator.New(d.conn.DB, d.conn.Dialect, migrations.FS, nil)
	switch op {
	case "up":
		if err := m.MigrateUp(ctx); err != nil {
			return err
		}
		fmt.Fprintln(stdout(), "migrations up: ok")
	case "down":
		if err := m.MigrateDown(ctx); err != nil {
			return err
		}
		fmt.Fprintln(stdout(), "migrations down: ok")
	case "status":
		versions, err := m.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout(), "applied %d migrations:\n", len(versions))
		for _, v := range versions {
			fmt.Fprintf(stdout(), "  %d\n", v)
		}
	}
	return nil
}
