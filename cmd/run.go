package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/kit/git"
	"github.com/s12chung/ccbox/pkg/projectstate"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
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

func runOptions(m hostMounts, args []string) (docker.RunOptions, error) {
	gitConfigDir, err := hostGitConfigDir()
	if err != nil {
		return docker.RunOptions{}, err
	}
	proxyOps, err := proxyOptions()
	if err != nil {
		return docker.RunOptions{}, err
	}
	pkgerJSON, err := harness.MustFor(*projectCfg.CLI).PkgerJSON()
	if err != nil {
		return docker.RunOptions{}, err
	}

	return docker.RunOptions{
		Tag:             flagTag,
		CLI:             *projectCfg.CLI,
		CLIConfigDir:    m.cliConfigDir,
		ProjectStateDir: m.projectState,
		Cwd:             m.cwd,
		CLIDataBinds:    m.cliDataBinds,
		GHToken:         os.Getenv("GH_TOKEN"),
		GitConfigDir:    gitConfigDir,
		Env:             mergeempty.Map(projectCfg.Env, map[string]string{"CLI_PKGER": pkgerJSON}),
		TmpfsMasks:      projectCfg.TmpfsMasksPresent(),
		VolumeMasks:     projectCfg.VolumeMasksPresent(),
		ReadOnlyPaths:   projectCfg.ReadOnlyPathsPresent(),
		Cmd:             harness.MustFor(*projectCfg.CLI).SessionCmd(flagShell, flagContinue, flagResume, args),

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
	defer printPresentGuardMounts()

	presentMasksSnapshot := append(projectCfg.TmpfsMasksPresent(), projectCfg.VolumeMasksPresent()...)
	presentPathsSnapshot := projectCfg.ReadOnlyPathsPresent()
	defer warnCreatedGuardMounts(presentMasksSnapshot, presentPathsSnapshot) // compare snapshots to defer time

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
	cwd, cliConfigDir, projectState string
	cliDataBinds                    map[string]string // host path → $HOME-relative in-container path
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
	if err := safeSeedProjectStateDir(userDir, cwd); err != nil {
		return hostMounts{}, err
	}
	cliDataBinds, err := safeSeedCLIDataBinds(userDir, harness.MustFor(*projectCfg.CLI))
	if err != nil {
		return hostMounts{}, err
	}
	return hostMounts{
		cwd:          cwd,
		cliConfigDir: cliConfigDir(userDir, *projectCfg.CLI),
		projectState: projectStateDir(userDir, cwd),
		cliDataBinds: cliDataBinds,
	}, nil
}

// hostGitConfigDir resolves the host's ~/.config/git to bind read-only, or "" to skip — when
// host_git_config is disabled in .ccbox.yaml or the dir is absent. HostGitConfig is non-nil:
func hostGitConfigDir() (string, error) {
	if !*projectCfg.HostGitConfig {
		return "", nil
	}
	return git.XDGConfigDir()
}

// safeSeedProjectStateDir seeds projectStateDir() if missing
func safeSeedProjectStateDir(userDir, cwd string) error {
	return safeSeed(fsutil.MustNewFS(projectstate.SeedFS()), projectStateDir(userDir, cwd), false)
}

// projectStateDir is the host state dir for a project: userDir/projects/<slug>
func projectStateDir(userDir, cwd string) string {
	return filepath.Join(userDir, "projects", docker.ProjectSlug(cwd))
}

// safeSeedCLIDataBinds seeds cli's data binds under userDir/data/<cli_name> and returns them as
// host path → $HOME-relative in-container path.
func safeSeedCLIDataBinds(userDir string, cli harness.CLI) (map[string]string, error) {
	binds := make(map[string]string, len(cli.DataBinds))
	for key, content := range cli.DataBinds {
		host := cliDataBindHostPath(userDir, cli.Name, key)
		if content == nil {
			if err := os.MkdirAll(host, ioutil.Dir); err != nil {
				return nil, err
			}
		} else if err := seed.File(host, *content); err != nil && !errors.Is(err, seed.ErrExists) {
			return nil, err
		}
		binds[host] = key
	}
	return binds, nil
}

// cliDataBindHostPath is a data bind's shared host path: userDir/data/<cli_name>/<slug-of-key>
func cliDataBindHostPath(userDir, cliName, key string) string {
	return filepath.Join(userDir, "data", cliName, strings.ReplaceAll(key, "/", "-"))
}
