package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/log"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Print the effective .ccbox.yaml with " + projectcfg.DefaultsToken + " tokens expanded",
	RunE: func(_ *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		log.Info("# Run `ccbox config defaults` for " + projectcfg.DefaultsToken + " expansions")
		printLoadedPaths(cwd)
		out, err := yaml.Marshal(projectCfg) // already resolved by projectcfg.Load
		if err != nil {
			return err
		}
		log.Info(strings.TrimRight(string(out), "\n"))
		return printAbsentMasks(cwd)
	},
}

// printLoadedPaths lists the config files loaded, in load order
func printLoadedPaths(projectDir string) {
	loaded := projectcfg.LoadedPaths(projectDir)
	if len(loaded) == 0 {
		return
	}
	log.Info("\n# Config files, in load order:")
	for _, path := range loaded {
		log.Infof("#   %s", userdir.Tilde(path))
	}
	log.Info("")
}

// printAbsentMasks lists the mask paths this project lacks
func printAbsentMasks(cwd string) error {
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

var configDefaultsCmd = &cobra.Command{
	Use:   "defaults",
	Short: "Print what the " + projectcfg.DefaultsToken + " tokens expand to",
	RunE: func(_ *cobra.Command, _ []string) error {
		groups := []struct {
			field    string
			defaults []string
		}{
			{"tmpfsMasks", projectcfg.TmpfsDefaults()},
			{"volumeMasks", projectcfg.VolumeDefaults()},
			{"allowlist", projectcfg.AllowDefaults()},
		}
		for i, g := range groups {
			if i > 0 {
				log.Info("")
			}
			log.Infof("# %s:", g.field)
			for _, d := range g.defaults {
				log.Infof("#   - %s", d)
			}
		}
		return nil
	},
}

func init() { configCmd.AddCommand(configInitCmd, configDefaultsCmd) }
