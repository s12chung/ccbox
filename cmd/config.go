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
	Short: "Print the effective .ccbox.yaml with ccbox-defaults tokens expanded",
	RunE: func(_ *cobra.Command, _ []string) error {
		out, err := yaml.Marshal(projectCfg) // already resolved by projectcfg.Load
		if err != nil {
			return err
		}
		log.Info(strings.TrimRight(string(out), "\n"))
		return printAbsentMasks()
	},
}

// printAbsentMasks lists the mask dirs this project lacks
func printAbsentMasks() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	tmpfs, volumes, err := projectcfg.NotFoundMasks(cwd, projectcfg.Config{CLI: flagCLI})
	if err != nil {
		return err
	}
	if len(tmpfs) > 0 || len(volumes) > 0 {
		log.Info("")
	}
	if len(tmpfs) > 0 {
		log.Infof("# tmpfs not in project, not masked: %s", strings.Join(tmpfs, ", "))
	}
	if len(volumes) > 0 {
		log.Infof("# volumes not in project, not masked: %s", strings.Join(volumes, ", "))
	}
	return nil
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter .ccbox.yaml template to fill in",
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
