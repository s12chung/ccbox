// Command toolsbuild builds the embedded ccboxtools binary with the canonical
// flags (pkg/toolsbuild). make runs it via `go run`: it must work before the
// ccbox binary exists, so it can't be a ccbox subcommand (the embed needs
// dist/ccboxtools in place first).
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/toolsbuild"
)

func main() {
	goarch := flag.String("goarch", "", "image arch of the ccboxtools binary (required)")
	out := flag.String("o", "", "output path (required)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	if err := run(ctx, stop, *goarch, *out); err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stop context.CancelFunc, goarch, out string) error {
	defer stop()

	if goarch == "" || out == "" {
		return errors.New("both -goarch and -o are required")
	}
	return toolsbuild.Build(ctx, goarch, out)
}
