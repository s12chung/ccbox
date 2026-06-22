package cmd

import (
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Run the tinyproxy egress wall in the foreground",
	RunE: func(cmd *cobra.Command, _ []string) error {
		configFS, err := fs.Sub(proxyConfig, "docker/tinyproxy")
		if err != nil {
			return err
		}
		c, err := docker.New()
		if err != nil {
			return err
		}
		return c.Proxy(cmd.Context(), configFS)
	},
}

var proxyCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove the egress wall network",
	RunE: func(cmd *cobra.Command, _ []string) error {
		c, err := docker.New()
		if err != nil {
			return err
		}
		return c.ProxyClean(cmd.Context())
	},
}
