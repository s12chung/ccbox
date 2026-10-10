// Command ccbox runs the hardened Docker devbox: build, the egress wall, and the
// interactive devbox container launching the configured coding CLI.
package main

import (
	"embed"
	"os"

	"github.com/s12chung/ccbox/cmd"
	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// `go:embed` can't reach above its own package dir and can't
// embed a nested module either, so embed a ccboxtools binary
// that's checked at build time by `ccbox doctor tools`.
// all: keeps the desktop home's dot-dirs (.config) embedded.
//
//go:embed all:docker/* dist/ccboxtools
var embedBuildContext embed.FS

func main() {
	err := cmd.Execute(embedBuildContext)
	if code, ok := errs.ExitCode(err); ok {
		os.Exit(code)
	}
	if err != nil {
		log.Errorf("command failed: %v", err)
		os.Exit(1)
	}
}
