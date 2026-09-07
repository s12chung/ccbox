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
	tmpfsMasks, volumeMasks, err := projectcfg.NotFoundMasks(cwd, projectcfg.Config{CLI: flagCLI})
	if err != nil {
		return err
	}
	if len(tmpfsMasks) > 0 || len(volumeMasks) > 0 {
		log.Info("")
	}
	if len(tmpfsMasks) > 0 {
		log.Infof("# tmpfsMasks not in project, not masked: %s", strings.Join(tmpfsMasks, ", "))
	}
	if len(volumeMasks) > 0 {
		log.Infof("# volumeMasks not in project, not masked: %s", strings.Join(volumeMasks, ", "))
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
