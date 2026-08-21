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

//go:embed docker/seed/claude-config
var seedClaudeConfig embed.FS

//go:embed docker/seed/codex-config
var seedCodexConfig embed.FS

//go:embed docker/seed/opencode-config
var seedOpenCodeConfig embed.FS

//go:embed docker/seed/project-slug
var seedProject embed.FS

func main() {
	os.Exit(cmd.Execute(buildContext, proxyConfig, seedClaudeConfig, seedCodexConfig, seedOpenCodeConfig, seedProject))
}
