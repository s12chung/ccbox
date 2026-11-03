package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/pkg/util/slicex"
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
		return run(cmd.Context(), nil)
	},
}

func init() {
	for _, c := range []*cobra.Command{rootCmd, runCmd, execCmd} {
		rf := c.Flags()
		rf.BoolVar(&runVNC, "vnc", false, "serve VNC at localhost:5900 (experimental, always no proxy)")
	}
}

// run builds the image then boots the devbox, streaming its logs to the foreground —
// the run and proxy commands' shared tail
func run(ctx context.Context, proxyOptions *docker.ProxyOptions) error {
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

// prepareRun renders the run's options: every command's shared front
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

func printPresentGuardMounts() {
	if tmpfsMasks := config.TmpfsMasksPresent(); len(tmpfsMasks) > 0 {
		log.Infof("during run, masked with temp filesystem: %s", strings.Join(tmpfsMasks, ", "))
	}
	if volumeMasks := config.VolumeMasksPresent(); len(volumeMasks) > 0 {
		log.Infof("during run, masked with persistent volume: %s", strings.Join(volumeMasks, ", "))
	}
	if roPaths := config.ReadOnlyPathsPresent(); len(roPaths) > 0 {
		log.Infof("during run, read-only: %s", strings.Join(roPaths, ", "))
	}
}

// warnCreatedGuardMounts warns for each mask dir or glob match the run created: it exists now,
// so future runs guard it — a heads-up that it behaves differently from here.
func warnCreatedGuardMounts(presentMasksBefore, presentPathsBefore []string) {
	nowPresent := append(config.TmpfsMasksPresent(), config.VolumeMasksPresent()...)
	if created := slicex.Minus(nowPresent, presentMasksBefore); len(created) > 0 {
		log.Warnf("before run, these directories did not exist. future runs will mask them: %s", strings.Join(created, ", "))
	}
	if created := slicex.Minus(config.ReadOnlyPathsPresent(), presentPathsBefore); len(created) > 0 {
		log.Warnf("before run, these paths did not exist. future runs will re-mount them read-only: %s", strings.Join(created, ", "))
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
