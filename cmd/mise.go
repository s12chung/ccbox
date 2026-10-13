package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var miseCmd = &cobra.Command{
	Use:   "mise",
	Short: "Manage the devbox's mise config",
}

var miseUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the mise config's runtime selection",
	RunE:  func(_ *cobra.Command, _ []string) error { return miseUpdate() },
}

var miseInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter .ccbox-mise.toml picking the project's runtimes",
	RunE: func(_ *cobra.Command, _ []string) error {
		path := mise.ProjectConfigPath(config.ProjectDir())
		seeded, err := seedMiseConfig(path)
		if err != nil {
			return err
		}
		if seeded {
			log.Infof("wrote %s", path)
		}
		return nil
	},
}

// seedMiseConfig seeds path with the runtime picker's pick when path is missing —
// an existing config is never touched. Reports whether it seeded.
func seedMiseConfig(path string) (bool, error) {
	if osutil.Present(path) {
		return false, nil
	}
	picked, err := prompt.Runtimes(path, runtime.SelectableNames(), nil)
	if err != nil {
		return false, err
	}
	if err := mise.SeedConfig(path, runtime.FromMiseToolNames(picked).Tools()); err != nil {
		return false, err
	}
	return true, nil
}

func init() { miseCmd.AddCommand(miseInitCmd, miseUpdateCmd) }

// miseUpdate re-picks the picked mise config's runtimes — the config's current
// selection pre-filled — and syncs the pick into the config's tools region
func miseUpdate() error {
	projectDir := config.ProjectDir()
	toolNames, err := mise.ToolNames(projectDir)
	if err != nil {
		return err
	}
	options := runtime.SelectableNames()
	picked, err := prompt.Runtimes(mise.PickedConfigPath(projectDir), options, slicex.Intersect(options, toolNames))
	if err != nil {
		return err
	}
	return mise.SyncTools(projectDir, runtime.FromMiseToolNames(picked).Tools())
}
