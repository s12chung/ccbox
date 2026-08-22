package main

import (
	"embed"
	"os"

	"github.com/s12chung/ccbox/cmd"
)

// `go:embed` can't reach above its own package dir.

//go:embed Dockerfile docker/image/*
var buildContext embed.FS

//go:embed docker/tinyproxy/*
var proxyConfig embed.FS

func main() {
	os.Exit(cmd.Execute(buildContext, proxyConfig))
}
