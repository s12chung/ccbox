package cmd

import (
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the tinyproxy egress wall in the foreground",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctxD, err := dock.NewCtxD(cmd.Context())
		if err != nil {
			return err
		}
		proxyOps, err := proxyOptions()
		if err != nil {
			return err
		}
		return docker.Proxy(ctxD, proxyOps)
	},
}

func proxyOptions() (docker.ProxyOptions, error) {
	configFS, err := fs.Sub(proxyConfig, "docker/tinyproxy")
	if err != nil {
		return docker.ProxyOptions{}, err
	}
	return docker.ProxyOptions{
		Config:    configFS,
		Overrides: docker.AllowOverride(projectCfg.AllowlistExpanded()),
	}, nil
}
