package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var runMode dmap.RunMode

// runVNC serves the desktop session (`--vnc`); Load resolves it with the vnc
// section's enabled, so the runs ride projectConfig's resolved mode
var runVNC bool

// runCmd execs a command in a fresh devbox container; bare, it drops into the
// image's default shell.
var runCmd = &cobra.Command{
	Use:                   "run [command...]",
	Short:                 "Exec a command in the devbox container",
	Args:                  cobra.ArbitraryArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd.Context(), dmap.RunMode{Shell: true, NoProxy: runMode.NoProxy, Args: args})
	},
}

// run builds the image then runs the devbox container
func run(ctx context.Context, mode dmap.RunMode) error {
	if err := build(ctx, projectConfig.ServeVNC()); err != nil {
		return err
	}
	defer printPresentGuardMounts()

	presentMasksSnapshot := append(projectConfig.TmpfsMasksPresent(), projectConfig.VolumeMasksPresent()...)
	presentPathsSnapshot := projectConfig.ReadOnlyPathsPresent()
	defer warnCreatedGuardMounts(presentMasksSnapshot, presentPathsSnapshot) // compare snapshots to defer time

	userDir := userdir.Dir()
	if err := seedRunMounts(userDir); err != nil {
		return err
	}

	options, clean, err := dmap.NewRunMap(userDir, projectConfig, tagByProject).RunOptions(flagTag, mode)
	defer log.Defer("clean run files", clean)
	if err != nil {
		return err
	}

	//nolint:contextcheck // the chain's context.Background cleanups are deliberate: a cancelled ctx can't block cleanup
	return docker.Run(dock.MustNewCtxD(ctx), options)
}

func init() {
	for _, c := range []*cobra.Command{rootCmd, runCmd} {
		rf := c.Flags()
		rf.BoolVar(&runMode.NoProxy, "no-proxy", false, "run without the egress wall: direct network access")
		rf.BoolVar(&runVNC, "vnc", false, "serve VNC at localhost:5900 (experimental, always no proxy)")
	}
	runCmd.Flags().SetInterspersed(false)
}

// seedRunMounts seeds the run mounts in userDir
func seedRunMounts(userDir string) error {
	if err := safeSeedCLIConfig(*projectConfig.CLIName, false); err != nil {
		return err
	}
	return safeSeedCLIDataBinds(userDir, projectConfig.CLI())
}

// safeSeedCLIDataBinds seeds cli's data binds under userDir/data/<cli_name>
func safeSeedCLIDataBinds(userDir string, c cli.CLI) error {
	for key, content := range c.DataBinds {
		host := dmap.CLIDataBindPath(userDir, c.Name, key)
		if content == nil {
			if err := os.MkdirAll(host, osutil.Dir); err != nil {
				return err
			}
		} else if err := seed.SafeFile(host, []byte(*content)); err != nil {
			return err
		}
	}
	return nil
}
