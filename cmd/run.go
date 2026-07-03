package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/seed"
)

const projectSeedPrefix = "docker/seed/project-slug"

// seedProjectFn is seed.SeedProject, indirected so tests can stub out the file-copying step.
var seedProjectFn = seed.SeedProject

// Run flags.
var (
	flagNoAutoProxy bool // disable starting the egress wall; a wall must already be up
	flagContinue    bool // -c: continue the last Claude session (`claude -c`)
	flagResume      bool // -r: resume a Claude session — the picker, or a name from the positional arg
	flagShell       bool // drop into the image's default shell instead of launching Claude
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

// containerCmd is the command the entrypoint execs: Claude by default, `claude -c` to
// continue the last session, `claude --resume [name]` to pick/name one, or the image
// default (shell) with --shell. args holds the optional resume session name.
func containerCmd(args []string) []string {
	switch {
	case flagShell:
		return nil
	case flagContinue:
		return []string{"claude", "-c"}
	case flagResume:
		return append([]string{"claude", "--resume"}, args...)
	default:
		return []string{"claude"}
	}
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
	proxyFS, err := proxyConfigFS()
	if err != nil {
		return err
	}
	c, err := docker.New()
	if err != nil {
		return err
	}
	code, err := c.Run(cmd.Context(), docker.RunOptions{
		Tag:        flagTag,
		ConfigDir:  m.config,
		CcboxDir:   m.ccbox,
		Cwd:        m.cwd,
		OAuthToken: os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"),
		GHToken:    os.Getenv("GH_TOKEN"),
		Env:        projectCfg.Env,
		Tmpfs:      projectCfg.Tmpfs,
		Cmd:        containerCmd(args),
		AutoProxy:  !flagNoAutoProxy,
		Proxy: docker.ProxyOptions{
			Config:    proxyFS,
			Overrides: docker.AllowOverride(projectCfg.Allowlist),
		},
		ProxyLogPath: filepath.Join(flagCacheDir, "proxy.log"),
	})
	if err != nil {
		return err
	}
	exitCode = code
	return nil
}

func init() {
	f := rootCmd.Flags()
	f.BoolVar(&flagNoAutoProxy, "no-auto-proxy", false,
		"don't start the egress wall; require one already running (`ccbox proxy`)")
	f.BoolVarP(&flagContinue, "continue", "c", false, "continue the last Claude session (`claude -c`)")
	f.BoolVarP(&flagResume, "resume", "r", false, "resume a Claude session: `ccbox -r <name>`, or bare for the picker")
	f.BoolVar(&flagShell, "shell", false, "drop into a shell instead of launching Claude")
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
	config, err := safeSeedClaudeConfig(cacheDir, false)
	if err != nil {
		return hostMounts{}, err
	}
	ccbox, err := safeSeedProjectDir(cacheDir, cwd)
	if err != nil {
		return hostMounts{}, err
	}
	return hostMounts{cwd: cwd, config: config, ccbox: ccbox}, nil
}

// safeSeedProjectDir seeds projectDir() if missing
func safeSeedProjectDir(cacheDir, cwd string) (string, error) {
	dir := projectDir(cacheDir, cwd)

	switch _, err := os.Stat(dir); {
	case err == nil: // exists
		return dir, nil
	case !os.IsNotExist(err): // stat failed for some other reason
		return dir, err
	}

	src, err := fs.Sub(seedProject, projectSeedPrefix)
	if err != nil {
		return dir, err
	}
	if _, err := seedProjectFn(src, dir); err != nil {
		return dir, err
	}
	log.Infof("seeded fresh project dir: %s", dir)
	return dir, nil
}

// projectDir is the host state dir for a project: cacheDir/projects/<slug>
func projectDir(cacheDir, cwd string) string {
	return filepath.Join(cacheDir, "projects", docker.ProjectSlug(cwd))
}
