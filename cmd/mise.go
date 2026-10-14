package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/util/osutil"
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
	Short: "Write a starter mise.toml picking the project's runtimes",
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

// seedMiseConfig seeds path with the mise tools picker's pick when path is
// missing — an existing config is never touched. Reports whether it seeded.
func seedMiseConfig(path string) (bool, error) {
	if osutil.Present(path) {
		return false, nil
	}
	tools, err := prompt.MiseTools(path, nil)
	if err != nil {
		return false, err
	}
	if err := mise.SeedConfig(path, tools); err != nil {
		return false, err
	}
	return true, nil
}

func init() { miseCmd.AddCommand(miseInitCmd, miseUpdateCmd) }

// miseUpdate re-picks the picked mise config's tools — the config's current
// selection pre-filled — and syncs the pick into the config's tools region
func miseUpdate() error {
	projectDir := config.ProjectDir()
	toolNames, err := mise.ToolNames(projectDir)
	if err != nil {
		return err
	}
	tools, err := prompt.MiseTools(mise.PickedConfigPath(projectDir), toolNames)
	if err != nil {
		return err
	}
	return mise.SyncTools(projectDir, tools)
}
