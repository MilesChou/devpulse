package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/mileschou/devpulse/internal/repo"
)

type initFile struct {
	Repos []string `yaml:"repos"`
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <file.yaml>",
		Short: "Bootstrap the store from a YAML repo list",
		Long: `Read a YAML file containing a list of repositories and register
them all in the DevPulse store. Each repo is created if it does not
already exist. GitHub metadata is fetched on a best-effort basis.

Example YAML:

  repos:
    - laravel/framework
    - symfony/symfony`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd.Context(), args[0])
		},
	}
}

func runInit(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var f initFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	if len(f.Repos) == 0 {
		return fmt.Errorf("no repos found in %s", path)
	}

	names := make([]repo.FullName, 0, len(f.Repos))
	for _, raw := range f.Repos {
		name, err := repo.ParseFullName(raw)
		if err != nil {
			return fmt.Errorf("invalid repo %q: %w", raw, err)
		}
		names = append(names, name)
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	w := stdout()
	for _, name := range names {
		r, err := registerRepo(ctx, w, d, name)
		if err != nil {
			return err
		}
		printRepoSummary(w, r)
	}

	fmt.Fprintf(w, "\ninit: %d repo(s) registered\n", len(names))
	return nil
}
