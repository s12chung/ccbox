package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var execMode dmap.ExecMode

// execCmd execs a command in the devbox container; bare, it drops into a shell
var execCmd = &cobra.Command{
	Use:                   "exec [command...]",
	Short:                 "Exec a command in the devbox container",
	Args:                  cobra.ArbitraryArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return execSession(cmd.Context(), dmap.ExecMode{Shell: true, NoProxy: execMode.NoProxy, Args: args})
	},
}

// execSession boots the devbox, execing the session in it — the egress wall per the
// config and --no-proxy; always the flags-and-settings variant's devbox
func execSession(ctx context.Context, mode dmap.ExecMode) error {
	var proxyOptions *docker.ProxyOptions
	if !config.NoProxyFor(config.ServeVNC(), mode.NoProxy) {
		proxyOptions = dmap.NewProxyMap(config).Options()
	}
	options, clean, err := prepareRun(proxyOptions)
	if err != nil {
		return err
	}
	defer log.Defer("clean run files", clean)
	options.Exec = dmap.NewExecMap(config.CLI()).Options(mode, proxyOptions == nil)

	ctxD := dock.MustNewCtxD(ctx)
	running, err := docker.DevboxRunning(ctxD, options)
	if err != nil {
		return err
	}
	if !running {
		if err := build(ctx, config.ServeVNC()); err != nil {
			return err
		}
	}
	return docker.Exec(ctxD, options)
}

func init() {
	for _, c := range []*cobra.Command{rootCmd, execCmd} {
		rf := c.Flags()
		rf.BoolVar(&execMode.NoProxy, "no-proxy", false, "run without the egress wall: direct network access")
	}
	execCmd.Flags().SetInterspersed(false)
}
