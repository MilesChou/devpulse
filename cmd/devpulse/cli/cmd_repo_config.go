package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mileschou/devpulse/internal/persistence"
	"github.com/mileschou/devpulse/internal/repo"
)

// repoConfigKey is the set of operator-tunable per-repo settings.
// Each key corresponds to exactly one column on repos and one
// validation rule. Add a key here when introducing a new setting; the
// dispatch in set/get only needs the entry below.
//
// The CLI-facing name is what the operator types (`devpulse repo
// config set <repo> <key> <value>`). It is intentionally not the SQL
// column name — the schema is an implementation detail.
type repoConfigKey struct {
	name        string // CLI surface, e.g. "pr-start"
	description string // shown in `repo config get` and help text
}

// repoConfigKeys is the registry. Slice keeps insertion order stable
// for `get` listing; lookups go through repoConfigKeyByName.
var repoConfigKeys = []repoConfigKey{
	{
		name:        "pr-start",
		description: "minimum PR number the by-number sync will probe (>=1)",
	},
}

func repoConfigKeyByName(name string) (repoConfigKey, bool) {
	for _, k := range repoConfigKeys {
		if k.name == name {
			return k, true
		}
	}
	return repoConfigKey{}, false
}

// newRepoConfigCmd is the `devpulse repo config` command group. Right
// now it carries one knob (`pr-start`); adding more is a matter of
// extending repoConfigKeys + the dispatch below.
func newRepoConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get or set per-repo operator settings",
		Long:  "Adjust per-repo settings such as the PR-sync floor. Settings are operator-owned and not overwritten by `repo sync`.",
	}
	cmd.AddCommand(newRepoConfigSetCmd(), newRepoConfigGetCmd())
	return cmd
}

func newRepoConfigSetCmd() *cobra.Command {
	keys := availableKeysHint()
	return &cobra.Command{
		Use:     "set <owner/name> <key> <value>",
		Short:   "Set a per-repo setting",
		Long:    "Set a per-repo operator setting. Available keys: " + keys,
		Example: "  devpulse repo config set MilesChou/devpulse pr-start 500",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRepoConfigSet(cmd.Context(), args[0], args[1], args[2])
		},
	}
}

func newRepoConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <owner/name> [key]",
		Short:   "Print per-repo settings",
		Long:    "Print one or all per-repo settings. With no key argument every known setting is printed.",
		Example: "  devpulse repo config get MilesChou/devpulse pr-start\n  devpulse repo config get MilesChou/devpulse",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := ""
			if len(args) == 2 {
				key = args[1]
			}
			return runRepoConfigGet(cmd.Context(), args[0], key)
		},
	}
}

func runRepoConfigSet(ctx context.Context, repoArg, keyArg, valueArg string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}
	if _, ok := repoConfigKeyByName(keyArg); !ok {
		return fmt.Errorf("unknown setting %q (available: %s)", keyArg, availableKeysHint())
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	r, err := d.repos.FindByFullName(ctx, "github", name)
	if errors.Is(err, persistence.ErrRepoNotFound) {
		return fmt.Errorf("repo %s is not registered; run `devpulse repo add %s` first", name, name)
	}
	if err != nil {
		return fmt.Errorf("find repo: %w", err)
	}

	switch keyArg {
	case "pr-start":
		n, err := strconv.Atoi(valueArg)
		if err != nil {
			return fmt.Errorf("invalid value %q for pr-start: must be an integer", valueArg)
		}
		if n < 1 {
			return fmt.Errorf("invalid value %d for pr-start: must be >= 1", n)
		}
		if err := d.repos.UpdatePRSyncStart(ctx, r.ID, n); err != nil {
			return fmt.Errorf("update pr-start: %w", err)
		}
		fmt.Fprintf(stdout(), "%s pr-start=%d\n", name, n)
		return nil
	}
	// Unreachable: repoConfigKeyByName already validated the key.
	return fmt.Errorf("internal: unhandled key %q", keyArg)
}

func runRepoConfigGet(ctx context.Context, repoArg, key string) error {
	name, err := repo.ParseFullName(repoArg)
	if err != nil {
		return fmt.Errorf("invalid repo: %w", err)
	}
	if key != "" {
		if _, ok := repoConfigKeyByName(key); !ok {
			return fmt.Errorf("unknown setting %q (available: %s)", key, availableKeysHint())
		}
	}

	d, err := buildDeps(ctx)
	if err != nil {
		return err
	}
	defer d.close(ctx)

	r, err := d.repos.FindByFullName(ctx, "github", name)
	if errors.Is(err, persistence.ErrRepoNotFound) {
		return fmt.Errorf("repo %s is not registered; run `devpulse repo add %s` first", name, name)
	}
	if err != nil {
		return fmt.Errorf("find repo: %w", err)
	}

	w := stdout()
	switch key {
	case "":
		// All keys.
		for _, k := range repoConfigKeys {
			fmt.Fprintf(w, "%s=%s\n", k.name, formatRepoConfigValue(k.name, r))
		}
	case "pr-start":
		fmt.Fprintln(w, formatRepoConfigValue("pr-start", r))
	}
	return nil
}

func formatRepoConfigValue(key string, r repo.Repo) string {
	switch key {
	case "pr-start":
		return strconv.Itoa(r.PRSyncStartNumber)
	default:
		return "(unknown)"
	}
}

func availableKeysHint() string {
	names := make([]string, 0, len(repoConfigKeys))
	for _, k := range repoConfigKeys {
		names = append(names, k.name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
