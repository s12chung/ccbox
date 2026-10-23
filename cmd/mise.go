package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/kit/miseutil"
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
	RunE:  func(cmd *cobra.Command, _ []string) error { return miseUpdate(cmd.Context()) },
}

var miseInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a starter mise.toml picking the project's runtimes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		path := mise.ProjectConfigPath(config.ProjectDir())
		seeded, err := seedMiseConfig(cmd.Context(), path)
		if err != nil {
			return err
		}
		if seeded {
			log.Infof("wrote %s", path)
		}
		return nil
	},
}

func init() { miseCmd.AddCommand(miseInitCmd, miseUpdateCmd) }

// seedMiseConfig seeds path with the mise tools picker's pick when path is
// missing — an existing config is never touched. Reports whether it seeded.
func seedMiseConfig(ctx context.Context, path string) (bool, error) {
	if osutil.Present(path) {
		return false, nil
	}
	configSet, err := promptConfigSet(ctx, path, mise.SeedToolState())
	if err != nil {
		return false, err
	}
	if err := mise.SeedConfig(path, configSet); err != nil {
		return false, err
	}
	return true, nil
}

// miseUpdate re-picks the picked config's tools — the current selection
// pre-filled — then syncs the pick into the config's regions, existing pins
// preserved
func miseUpdate(ctx context.Context) error {
	projectDir := config.ProjectDir()
	state, err := mise.ReadToolState(projectDir)
	if err != nil {
		return err
	}
	configSet, err := promptConfigSet(ctx, mise.PickedConfigPath(projectDir), state)
	if err != nil {
		return err
	}
	return mise.Sync(projectDir, configSet)
}

// promptConfigSet picks the mise tools and resolves their pins — the pick
// fetched off each source's latests
func promptConfigSet(ctx context.Context, savePath string, state mise.ToolState) (mise.ConfigSet, error) {
	state.VersionFunc = func(name string) (string, error) {
		return miseutil.LatestVersion(ctx, name)
	}
	tools, packages, err := prompt.MiseTools(savePath, state)
	if err != nil {
		return mise.ConfigSet{}, err
	}
	resolved, err := mise.ResolvedTools(tools, state)
	if err != nil {
		return mise.ConfigSet{}, err
	}
	return mise.ConfigSet{Tools: resolved, Packages: packages}, nil
}
