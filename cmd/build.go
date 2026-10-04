package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the devbox image",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return build(cmd.Context(), buildVNC)
	},
}

// buildVNC builds the image's desktop variant (`build --vnc`)
var buildVNC bool

func init() {
	buildCmd.Flags().BoolVar(&buildVNC, "vnc", false, "build VNC variant (experimental)")
}

// build builds the image variant the mode asks for; `run` calls it too, mirroring the old `run: build`.
func build(ctx context.Context, serveVNC bool) error {
	return docker.Build(ctx, buildContext, dmap.NewBuildMap(flagTag, serveVNC).Options())
}
