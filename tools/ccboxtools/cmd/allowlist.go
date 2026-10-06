package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/proxy"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var allowlistCmd = &cobra.Command{
	Use:   "allowlist",
	Short: "Print the proxy's allowed domains, one per line",
	RunE: func(_ *cobra.Command, _ []string) error {
		domains, err := proxy.Domains(pkginfo.ProxyAllowPath)
		if err != nil {
			return err
		}
		for _, d := range domains {
			log.Info(d)
		}
		return nil
	},
}
