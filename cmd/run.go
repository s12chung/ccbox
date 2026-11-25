package cmd

import (
	"context"
	"os"
	"path/filepath"
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
		rf.BoolVar(&flags.VNC, "vnc", false, "serve VNC at localhost:5900 (experimental)")
	}
}

// run builds the image, then boots the devbox streaming its logs to the foreground —
// run and proxy both land here
func run(ctx context.Context, proxyOptions *docker.ProxyOptions) error {
	if err := build(ctx, config.VNC != nil); err != nil {
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

// prepareRun renders the run's options and seeds its mounts — run and exec both start here
func prepareRun(proxyOptions *docker.ProxyOptions) (docker.RunOptions, func() error, error) {
	defer printPresentGuardMounts()

	presentMasksSnapshot := append(config.TmpfsMasksPresent(), config.VolumeMasksPresent()...)
	presentPathsSnapshot := config.ReadOnlyPathsPresent()
	defer warnCreatedGuardMounts(presentMasksSnapshot, presentPathsSnapshot) // warns against these pre-run snapshots

	if err := seedRunMounts(userdir.Dir(), proxyOptions != nil); err != nil {
		return docker.RunOptions{}, nil, err
	}

	options, clean, err := dmap.NewRunMap(config, miseRuntimes.CacheVolumes(), tagByProject).RunOptions(proxyOptions)
	if err != nil {
		log.WarnErr("clean run files", clean()) // a render failure cleans its begun shares
		return docker.RunOptions{}, nil, err
	}
	return options, clean, nil
}

func printPresentGuardMounts() {
	if dirs := config.TmpfsMasksDirs(); len(dirs) > 0 {
		log.Infof("during run, masked with temp filesystem: %s", strings.Join(dirs, ", "))
	}
	if files := config.TmpfsMasksFiles(); len(files) > 0 {
		log.Infof("during run, masked with an empty file, writes discarded: %s", strings.Join(files, ", "))
	}
	if volumeMasks := config.VolumeMasksPresent(); len(volumeMasks) > 0 {
		log.Infof("during run, masked with persistent volume: %s", strings.Join(volumeMasks, ", "))
	}
	if roPaths := config.ReadOnlyPathsPresent(); len(roPaths) > 0 {
		log.Infof("during run, read-only: %s", strings.Join(roPaths, ", "))
	}
}

// warnCreatedGuardMounts warns when the run created a mask path or glob match — from the
// next run on, it's guarded.
func warnCreatedGuardMounts(presentMasksBefore, presentPathsBefore []string) {
	nowPresent := append(config.TmpfsMasksPresent(), config.VolumeMasksPresent()...)
	if created := slicex.Minus(nowPresent, presentMasksBefore); len(created) > 0 {
		log.Warnf("before run, these paths did not exist. future runs will mask them: %s", strings.Join(created, ", "))
	}
	if created := slicex.Minus(config.ReadOnlyPathsPresent(), presentPathsBefore); len(created) > 0 {
		log.Warnf("before run, these paths did not exist. future runs will re-mount them read-only: %s", strings.Join(created, ", "))
	}
}

// seedRunMounts seeds the run mounts in userDir
func seedRunMounts(userDir string, isProxied bool) error {
	c := config.CLI()
	if err := seed.IfNotExist(clitmpl.UserCLIConfigSeedFS(c.Name, c.IsUserDefined()), cli.UserConfigDir(c.Name)); err != nil {
		return err
	}
	if err := seedDataBinds(dmap.CLIDataDir(userDir, c.Name), c.Setup.DataBindSeedMap(true), osutil.File); err != nil {
		return err
	}
	app := config.GUIApp()
	if app == nil {
		return nil
	}
	return seedDataBinds(dmap.GUIAppDataDir(userDir, app.Name, isProxied), app.Setup.DataBindSeedMap(isProxied), osutil.SecretFile)
}

// seedDataBinds seeds a DataBindSeedMap under its dataDir: nil content mkdirs the
// host dir, content seeds a file, chmod'd fileMode
func seedDataBinds(dataDir string, seedMap map[string]*string, fileMode os.FileMode) error {
	for hostPath, content := range seedMap {
		host := filepath.Join(dataDir, hostPath)
		if content == nil {
			if err := os.MkdirAll(host, osutil.Dir); err != nil {
				return err
			}
		} else {
			if err := seed.SafeFile(host, []byte(*content)); err != nil {
				return err
			}
			if err := os.Chmod(host, fileMode); err != nil {
				return err
			}
		}
	}
	return nil
}
