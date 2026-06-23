package cmd

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/seed"
)

const projectSeedPrefix = "docker/seed/project-slug"

// seedProjectFn is seed.SeedProject, indirected so tests can stub out the file-copying step.
var seedProjectFn = seed.SeedProject

// flagNoAutoProxy disables starting the egress wall for the run; a wall must already be up.
var flagNoAutoProxy bool

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the devbox container interactively behind the egress wall",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := build(cmd.Context()); err != nil {
			return err
		}
		m, err := resolveHostMounts(flagCacheDir)
		if err != nil {
			return err
		}
		cfg, err := projectcfg.Load(m.cwd)
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
			Tag:          flagTag,
			ConfigDir:    m.config,
			CcboxDir:     m.ccbox,
			Cwd:          m.cwd,
			OAuthToken:   os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"),
			GHToken:      os.Getenv("GH_TOKEN"),
			Env:          cfg.Env,
			Tmpfs:        cfg.Tmpfs,
			AutoProxy:    !flagNoAutoProxy,
			ProxyConfig:  proxyFS,
			ProxyLogPath: filepath.Join(flagCacheDir, "proxy.log"),
		})
		if err != nil {
			return err
		}
		exitCode = code
		return nil
	},
}

func init() {
	runCmd.Flags().BoolVar(&flagNoAutoProxy, "no-auto-proxy", false,
		"don't start the egress wall; require one already running (`ccbox proxy`)")
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
