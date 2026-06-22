package cmd

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/seed"
)

const projectSeedPrefix = "docker/seed/project-slug"

// seedProjectFn is seed.SeedProject, indirected so tests can stub out the file-copying step.
var seedProjectFn = seed.SeedProject

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
		c, err := docker.New()
		if err != nil {
			return err
		}
		code, err := c.Run(cmd.Context(), docker.RunOptions{
			Tag:          flagTag,
			ConfigDir:    m.config,
			CcboxDir:     m.ccbox,
			WorkspaceDir: m.workspace,
			OAuthToken:   os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"),
			GHToken:      os.Getenv("GH_TOKEN"),
		})
		if err != nil {
			return err
		}
		exitCode = code
		return nil
	},
}

// hostMounts are the host dirs bind-mounted into the devbox, seeded/created before it starts.
type hostMounts struct {
	config, ccbox, workspace string
}

// resolveHostMounts seeds the config dir and resolves the per-project state dir from cwd.
func resolveHostMounts(cacheDir string) (hostMounts, error) {
	config, err := safeSeedClaudeConfig(cacheDir, false)
	if err != nil {
		return hostMounts{}, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return hostMounts{}, err
	}
	ccbox, err := safeSeedProjectDir(cacheDir, cwd)
	if err != nil {
		return hostMounts{}, err
	}
	return hostMounts{config: config, ccbox: ccbox, workspace: cwd}, nil
}

// safeSeedProjectDir is the host dir backing ccboxMount for a project:
// cacheDir/projects/<slug>, where slug is the container mount path with '/'→'-'
// (e.g. -home-ccbox-ccbox) — the same per-project key Claude Code uses.
// A missing dir is seeded from the embedded project tree
func safeSeedProjectDir(cacheDir, workspaceDir string) (string, error) {
	dir := projectDir(cacheDir, workspaceDir)

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
	slog.Info("seeded fresh project dir", "dir", dir)
	return dir, nil
}

// projectDir is the host state dir for a project: cacheDir/projects/<slug>, where
// slug is the container mount path with '/'→'-' (e.g. -home-ccbox-ccbox).
func projectDir(cacheDir, workspaceDir string) string {
	slug := strings.ReplaceAll(docker.ContainerMount(workspaceDir), "/", "-")
	return filepath.Join(cacheDir, "projects", slug)
}
