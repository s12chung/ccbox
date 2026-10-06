package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the tinyproxy egress wall in the foreground",
	RunE: func(cmd *cobra.Command, _ []string) error {
		proxyLog := &docker.ProxyLog{Writer: os.Stdout, Colored: true, Stop: make(chan struct{})}
		return docker.Proxy(dock.MustNewCtxD(cmd.Context()), *dmap.NewProxyMap(projectConfig).Options(proxyLog))
	},
}
