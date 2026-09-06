package cmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/kit/git"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/projectstate"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
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

func runOptions(m hostMounts, args []string) (docker.RunOptions, error) {
	gitConfigDir, err := hostGitConfigDir()
	if err != nil {
		return docker.RunOptions{}, err
	}
	proxyOps, err := proxyOptions()
	if err != nil {
		return docker.RunOptions{}, err
	}

	return docker.RunOptions{
		Tag:          flagTag,
		CLI:          *projectCfg.CLI,
		ConfigDir:    m.config,
		CcboxDir:     m.ccbox,
		Cwd:          m.cwd,
		GHToken:      os.Getenv("GH_TOKEN"),
		GitConfigDir: gitConfigDir,
		Env:          projectCfg.Env,
		Tmpfs:        projectCfg.Tmpfs,
		Volumes:      projectCfg.Volumes,
		Cmd:          harness.MustFor(*projectCfg.CLI).SessionCmd(flagShell, flagContinue, flagResume, args),

		Proxy:        proxyOps,
		ProxyLogPath: filepath.Join(userdir.Dir(), "proxy.log"),
		NoProxy:      flagNoProxy,
	}, nil
}

// runDevbox builds the image then runs the devbox container interactively behind the
// egress wall. It is the root command's action — `ccbox` with no subcommand.
func runDevbox(cmd *cobra.Command, args []string) error {
	if err := build(cmd.Context()); err != nil {
		return err
	}
	m, err := resolveHostMounts(userdir.Dir())
	if err != nil {
		return err
	}
	// A default mask dir absent now isn't masked this run, but gets masked once it exists.
	// Snapshot the absent ones, then warn after the run for any the container created.
	defer printMasks()
	absentDefaults := masksOnHost(m.cwd, projectcfg.MaskDefaults(), false)
	defer warnCreatedMasks(m.cwd, absentDefaults)

	ctxD, err := dock.NewCtxD(cmd.Context())
	if err != nil {
		return err
	}
	runOpts, err := runOptions(m, args)
	if err != nil {
		return err
	}
	code, err := docker.Run(ctxD, runOpts)
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

// hostMounts are the host dirs bind-mounted into the devbox, seeded/created before it starts.
type hostMounts struct {
	cwd, config, ccbox string
}

// resolveHostMounts seeds the userDir and resolves the per-project state dir from cwd.
func resolveHostMounts(userDir string) (hostMounts, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return hostMounts{}, err
	}
	if err := safeSeedCLIConfig(userDir, *projectCfg.CLI, false); err != nil {
		return hostMounts{}, err
	}
	if err := safeSeedProjectDir(userDir, cwd); err != nil {
		return hostMounts{}, err
	}
	return hostMounts{cwd: cwd, config: cliConfigDir(userDir, *projectCfg.CLI), ccbox: projectDir(userDir, cwd)}, nil
}

// hostGitConfigDir resolves the host's ~/.config/git to bind read-only, or "" to skip — when
// host_git_config is disabled in .ccbox.yaml or the dir is absent. HostGitConfig is non-nil:
// projectcfg.Load always applies Defaulted, which resolves the default-on.
func hostGitConfigDir() (string, error) {
	if !*projectCfg.HostGitConfig {
		return "", nil
	}
	return git.XDGConfigDir()
}

// safeSeedProjectDir seeds projectDir() if missing
func safeSeedProjectDir(userDir, cwd string) error {
	return safeSeed(fsutil.MustNewFS(projectstate.SeedFS()), projectDir(userDir, cwd), false)
}

// projectDir is the host state dir for a project: userDir/projects/<slug>
func projectDir(userDir, cwd string) string {
	return filepath.Join(userDir, "projects", docker.ProjectSlug(cwd))
}
