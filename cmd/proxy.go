package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Boot the devbox with the egress wall on, streaming its logs to the foreground",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return run(cmd.Context(), dmap.NewProxyMap(config).Options(reportReload))
	},
}

var proxyReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload the egress wall's allow list on a running proxied devbox",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		running, err := docker.IsAnyDevboxRunning(dock.MustNewCtxD(cmd.Context()), dmap.ProxiedDevboxNames(config.ProjectDir()))
		if err != nil {
			return err
		}
		if !running {
			return errors.New("no proxied devbox running; boot one with `ccbox proxy` or `ccbox exec`")
		}
		changed, err := dmap.NewProxyMap(config).Refresh()
		if err != nil {
			return err
		}
		reportReload(changed)
		return nil
	},
}

func init() { proxyCmd.AddCommand(proxyReloadCmd) }

func reportReload(changed string) {
	if changed == "" {
		log.Info("allow list reloaded: no changes")
		return
	}
	log.Info("allow list reloaded:\n" + changed)
}
