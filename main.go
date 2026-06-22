package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/s12chung/ccbox/cmd"
)

// `go:embed` can't reach above its own package dir.

//go:embed Dockerfile docker/image/*
var buildContext embed.FS

//go:embed docker/tinyproxy/*
var proxyConfig embed.FS

//go:embed docker/seed/claude-config
var seedClaudeConfig embed.FS

//go:embed docker/seed/project-slug
var seedProject embed.FS

func main() {
	// Logs go to stderr; stdout is reserved for real output (build stream, container I/O).
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	os.Exit(cmd.Execute(buildContext, proxyConfig, seedClaudeConfig, seedProject))
}
