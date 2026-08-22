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
)

// Run flags.
var (
	flagContinue bool // -c: continue the last session
	flagResume   bool // -r: resume a session — the picker, or an id from the positional arg
	flagShell    bool // drop into the image's default shell instead of launching the CLI
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
	proxyFS, err := proxyConfigFS()
	if err != nil {
		return docker.RunOptions{}, err
	}

	return docker.RunOptions{
		Tag:          flagTag,
		CLI:          projectCfg.CLI,
		ConfigDir:    m.config,
		CcboxDir:     m.ccbox,
		Cwd:          m.cwd,
		GHToken:      os.Getenv("GH_TOKEN"),
		GitConfigDir: gitConfigDir,
		Env:          projectCfg.Env,
		Tmpfs:        projectCfg.Tmpfs,
		Volumes:      projectCfg.Volumes,
		Cmd:          harness.MustFor(projectCfg.CLI).SessionCmd(flagShell, flagContinue, flagResume, args),
		Proxy: docker.ProxyOptions{
			Config:    proxyFS,
			Overrides: docker.AllowOverride(projectCfg.Allowlist),
		},
		ProxyLogPath: filepath.Join(flagCacheDir, "proxy.log"),
	}, nil
}

// runDevbox builds the image then runs the devbox container interactively behind the
// egress wall. It is the root command's action — `ccbox` with no subcommand.
func runDevbox(cmd *cobra.Command, args []string) error {
	if err := build(cmd.Context()); err != nil {
		return err
	}
	m, err := resolveHostMounts(flagCacheDir)
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
	rootCmd.MarkFlagsMutuallyExclusive("continue", "resume", "shell")
}

// hostMounts are the host dirs bind-mounted into the devbox, seeded/created before it starts.
type hostMounts struct {
	cwd, config, ccbox string
}

// resolveHostMounts seeds the config dir and resolves the per-project state dir from cwd.
func resolveHostMounts(cacheDir string) (hostMounts, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return hostMounts{}, err
	}
	config, err := safeSeedConfig(cacheDir, projectCfg.CLI, false)
	if err != nil {
		return hostMounts{}, err
	}
	ccbox, err := safeSeedProjectDir(cacheDir, cwd)
	if err != nil {
		return hostMounts{}, err
	}
	return hostMounts{cwd: cwd, config: config, ccbox: ccbox}, nil
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
func safeSeedProjectDir(cacheDir, cwd string) (string, error) {
	dir := projectDir(cacheDir, cwd)
	return safeSeed(projectstate.SeedFS(), dir, false, seedSrc{sub: "."})
}

// projectDir is the host state dir for a project: cacheDir/projects/<slug>
func projectDir(cacheDir, cwd string) string {
	return filepath.Join(cacheDir, "projects", docker.ProjectSlug(cwd))
}
