// Command ccboxtools maintains the container: it is the container's entrypoint —
// verifying identity and egress wall before exec'ing the command — and installs and
// maintains the devbox's coding CLI in the clis volume.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/s12chung/ccbox/tools/ccboxtools/cmd"
)

func main() {
	os.Exit(run())
}

// run builds the boot's stop signal — SIGTERM (docker stop) and SIGINT cancel
// the ctx — and executes the CLI under it.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cmd.Execute(ctx)
}
