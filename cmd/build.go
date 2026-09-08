package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the devbox image",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return build(cmd.Context())
	},
}

// build builds the CLI-agnostic devbox image; `run` calls it too, mirroring the old `run: build`.
func build(ctx context.Context) error {
	return docker.Build(ctx, buildContext, docker.BuildOptions{Tag: flagTag})
}
