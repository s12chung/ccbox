// Command ccbox runs the hardened Docker devbox: build, wall proxy, and the
// interactive devbox container launching the configured coding CLI.
package main

import (
	"embed"
	"os"

	"github.com/s12chung/ccbox/cmd"
)

// `go:embed` can't reach above its own package dir and can't
// embed a nested module either, so embed a ccboxtools binary
// that's checked at build time by `ccbox doctor tools`

//go:embed Dockerfile docker/image/* dist/ccboxtools
var buildContext embed.FS

//go:embed docker/tinyproxy/*
var proxyConfig embed.FS

func main() {
	os.Exit(cmd.Execute(buildContext, proxyConfig))
}
