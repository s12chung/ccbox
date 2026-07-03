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
		configFS, err := proxyConfigFS()
		if err != nil {
			return err
		}
		c, err := docker.New()
		if err != nil {
			return err
		}
		return c.Proxy(cmd.Context(), docker.ProxyOptions{
			Config:    configFS,
			Overrides: docker.AllowOverride(projectCfg.Allowlist),
		})
	},
}

// proxyConfigFS roots the embedded tinyproxy configs
func proxyConfigFS() (fs.FS, error) { return fs.Sub(proxyConfig, "docker/tinyproxy") }
