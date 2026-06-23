// Package cmd holds the ccbox cobra commands. It is thin: each command gathers
// config and calls one pkg/docker operation.
package cmd

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/log"
)

// Injected from main (package main can't be imported, so the embed FSes come in here).
var (
	buildContext     embed.FS // Dockerfile + docker/image/* — the build context
	proxyConfig      embed.FS // docker/tinyproxy/* — the egress wall configs
	seedClaudeConfig embed.FS // docker/seed/claude-config — the seedable Claude config
	seedProject      embed.FS // docker/seed/project-slug — the seedable per-project tree
)

// Shared flags.
var (
	flagTag      string
	flagCacheDir string
)

// exitCode lets `run` propagate the container's exit status out through Execute.
var exitCode int

var rootCmd = &cobra.Command{
	Use:           "ccbox",
	Short:         "Hardened Docker devbox for Claude Code",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the CLI and returns the process exit code.
func Execute(build, proxy, claudeConfig, project embed.FS) int {
	buildContext = build
	proxyConfig = proxy
	seedClaudeConfig = claudeConfig
	seedProject = project
	if err := rootCmd.Execute(); err != nil {
		log.Errorf("command failed: %v", err)
		return 1
	}
	return exitCode
}

func init() {
	home, _ := os.UserHomeDir()
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&flagTag, "tag", docker.DefaultTag, "devbox image tag")
	pf.StringVar(&flagCacheDir, "cache-dir", filepath.Join(home, ".ccbox"), "ccbox cache directory")

	rootCmd.AddCommand(buildCmd, runCmd, proxyCmd, reseedCmd, cleanCmd)
}
