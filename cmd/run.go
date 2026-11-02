package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// runVNC serves the desktop session (`--vnc`); Load resolves it with the vnc
// section's enabled, and the runs follow the resolved mode
var runVNC bool

// runCmd boots the devbox container and streams its logs to the foreground
var runCmd = &cobra.Command{
	Use:                   "run",
	Short:                 "Boot the devbox and stream its logs",
	Args:                  cobra.NoArgs,
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runBoot(cmd.Context(), nil)
	},
}

// runBoot builds the image then boots the devbox, streaming its logs to the foreground —
// run and proxy's shared tail
func runBoot(ctx context.Context, proxyOptions *docker.ProxyOptions) error {
	if err := build(ctx, config.ServeVNC()); err != nil {
		return err
	}
	options, clean, err := prepareRun(proxyOptions)
	if err != nil {
		return err
	}
	defer log.Defer("clean run files", clean)
	options.Foreground = &docker.Foreground{Writer: osutil.BlockedCloser(os.Stdout)}

	return docker.Run(dock.MustNewCtxD(ctx), options)
}

// prepareRun renders the boot's options: every command's shared front
func prepareRun(proxyOptions *docker.ProxyOptions) (docker.RunOptions, func() error, error) {
	defer printPresentGuardMounts()

	presentMasksSnapshot := append(config.TmpfsMasksPresent(), config.VolumeMasksPresent()...)
	presentPathsSnapshot := config.ReadOnlyPathsPresent()
	defer warnCreatedGuardMounts(presentMasksSnapshot, presentPathsSnapshot) // compare snapshots to defer time

	if err := seedRunMounts(userdir.Dir()); err != nil {
		return docker.RunOptions{}, nil, err
	}

	options, clean, err := dmap.NewRunMap(config, miseRuntimes.CacheVolumes(), tagByProject).RunOptions(flagTag, proxyOptions)
	if err != nil {
		log.WarnErr("clean run files", clean()) // a render failure cleans its begun shares
		return docker.RunOptions{}, nil, err
	}
	return options, clean, nil
}

func init() {
	for _, c := range []*cobra.Command{rootCmd, runCmd, execCmd} {
		rf := c.Flags()
		rf.BoolVar(&runVNC, "vnc", false, "serve VNC at localhost:5900 (experimental, always no proxy)")
	}
}

// seedRunMounts seeds the run mounts in userDir
func seedRunMounts(userDir string) error {
	c := config.CLI()
	if err := seed.IfNotExist(clitmpl.UserCLIConfigSeedFS(c.Name, c.IsUserDefined()), cli.UserConfigDir(c.Name)); err != nil {
		return err
	}
	return safeSeedCLIDataBinds(userDir, c)
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
