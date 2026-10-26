// Command ccboxtools maintains the container: the entrypoint that boots and
// supervises the egress wall, the exec entry that drops sessions to ccbox
// inside it, and the coding CLI's installer.
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
