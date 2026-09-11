package cmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/projectstate"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

// Run flags.
var (
	flagContinue bool // -c: continue the last session
	flagResume   bool // -r: resume a session — the picker, or an id from the positional arg
	flagShell    bool // drop into the image's default shell instead of launching the CLI
	flagNoProxy  bool // skip the egress wall: direct network access
)

// resumeArgs allows a single positional session name, and only alongside -r/--resume.
func resumeArgs(_ *cobra.Command, args []string) error {
	switch {
	case len(args) > 1:
		return errors.New("accepts at most one session name")
	case len(args) == 1 && !flagResume:
		return errors.New("a session name requires -r/--resume")
	}
	return nil
}

// run builds the image then runs the devbox container interactively behind the
// egress wall. It is the root command's action — `ccbox` with no subcommand.
func run(cmd *cobra.Command, args []string) error {
	if err := build(cmd.Context()); err != nil {
		return err
	}
	defer printPresentGuardMounts()

	presentMasksSnapshot := append(projectCfg.TmpfsMasksPresent(), projectCfg.VolumeMasksPresent()...)
	presentPathsSnapshot := projectCfg.ReadOnlyPathsPresent()
	defer warnCreatedGuardMounts(presentMasksSnapshot, presentPathsSnapshot) // compare snapshots to defer time

	userDir := userdir.Dir()
	if err := seedRunMounts(userDir); err != nil {
		return err
	}
	runMap := dmap.NewRunMap(userDir, projectCfg)
	hostOptions, clean, err := runMap.HostOptions()
	defer log.Defer("settle shared agents doc", clean)
	if err != nil {
		return err
	}

	ctxD, err := dock.NewCtxD(cmd.Context())
	if err != nil {
		return err
	}
	env, err := runMap.Env()
	if err != nil {
		return err
	}
	proxyOps, err := proxyOptions()
	if err != nil {
		return err
	}

	code, err := docker.Run(ctxD, docker.RunOptions{
		RunHostOptions: hostOptions,
		Tag:            flagTag,
		Env:            env,
		Cmd:            runMap.Cmd(flagShell, flagContinue, flagResume, args),
		Proxy:          proxyOps,
		ProxyLogPath:   filepath.Join(userDir, "proxy.log"),
		NoProxy:        flagNoProxy,
	})
	if err != nil {
		return err
	}
	exitCode = code
	return nil
}

func init() {
	f := rootCmd.Flags()
	f.BoolVarP(&flagContinue, "continue", "c", false, "continue the last session")
	f.BoolVarP(&flagResume, "resume", "r", false, "resume a session: `ccbox -r <name>`, or bare for the picker")
	f.BoolVar(&flagShell, "shell", false, "drop into a shell instead of launching the harness CLI")
	f.BoolVar(&flagNoProxy, "no-proxy", false, "run without the egress wall: direct network access")
	rootCmd.MarkFlagsMutuallyExclusive("continue", "resume", "shell")
}

// seedRunMounts seeds the run mounts in userDir
func seedRunMounts(userDir string) error {
	if err := safeSeedCLIConfig(userDir, *projectCfg.CLIName, false); err != nil {
		return err
	}
	if err := safeSeedProjectStateDir(userDir, projectCfg.ProjectDir()); err != nil {
		return err
	}
	return safeSeedCLIDataBinds(userDir, harness.MustFor(*projectCfg.CLIName))
}

// safeSeedProjectStateDir seeds dmap.ProjectStateDir() if missing
func safeSeedProjectStateDir(userDir, projectDir string) error {
	return safeSeed(fsutil.MustNewFS(projectstate.SeedFS()), dmap.ProjectStateDir(userDir, projectDir), false)
}

// safeSeedCLIDataBinds seeds cli's data binds under userDir/data/<cli_name>
func safeSeedCLIDataBinds(userDir string, cli harness.CLI) error {
	for key, content := range cli.DataBinds {
		host := dmap.CLIDataBindPath(userDir, cli.Name, key)
		if content == nil {
			if err := os.MkdirAll(host, ioutil.Dir); err != nil {
				return err
			}
		} else if err := seed.File(host, *content); err != nil && !errors.Is(err, seed.ErrExists) {
			return err
		}
	}
	return nil
}
