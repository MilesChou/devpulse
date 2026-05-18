// Command devpulse is the single binary entry point for the DevPulse CLI.
//
// All subcommands (fetch, enrich-pr, repo-add, worker, serve, migrate) are
// wired through cmd/devpulse/cli. Keep this file thin: bootstrap + delegation.
package main

import (
	"context"
	"fmt"
	"os"

	_ "go.uber.org/automaxprocs"

	"github.com/mileschou/devpulse/cmd/devpulse/cli"
)

func main() {
	ctx := context.Background()
	if err := cli.Execute(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

