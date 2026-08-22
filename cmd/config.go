package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/log"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the effective .ccbox.yaml with tmpfs, volumes, and allowlist defaults applied",
	RunE: func(_ *cobra.Command, _ []string) error {
		out, err := yaml.Marshal(projectCfg) // already resolved by projectcfg.Load
		if err != nil {
			return err
		}
		log.Info(strings.TrimRight(string(out), "\n"))
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter .ccbox.yaml with the documented defaults",
	RunE: func(_ *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path, err := projectcfg.Init(cwd)
		if err != nil {
			return err
		}
		log.Infof("wrote %s", path)
		return nil
	},
}

func init() { configCmd.AddCommand(configInitCmd) }
