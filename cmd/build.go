package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/npm"
)

const claudeCodePackage = "@anthropic-ai/claude-code"

var flagClaudeCodeVersion string

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the devbox image",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return build(cmd.Context())
	},
}

// build builds the devbox image; `run` calls it too, mirroring the old `run: build`.
func build(ctx context.Context) error {
	// Pin "latest" now so the image records the concrete version, not a moving tag.
	if flagClaudeCodeVersion == "latest" {
		v, err := npm.LatestVersion(claudeCodePackage)
		if err != nil {
			return err
		}
		flagClaudeCodeVersion = v
	}

	return docker.Build(ctx, buildContext, docker.BuildOptions{
		Tag:               flagTag,
		ClaudeCodeVersion: flagClaudeCodeVersion,
	})
}

func init() {
	buildCmd.Flags().StringVar(&flagClaudeCodeVersion, "claude-code-version", "latest",
		"CLAUDE_CODE_VERSION build arg")
}
