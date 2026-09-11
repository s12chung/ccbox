package dmap

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/git"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

const (
	containerHome         = "/home/ccbox"                // mounts sit under containerHome at a per-project leaf
	projectStateMountPath = "/home/ccbox/.ccbox/project" // per-project devbox state (e.g. lessons)

	gitConfigMountPath = "/home/ccbox/.config/git" // host global git dir, read-only (git's default XDG path)
)

// CLIConfigHostPath is the host dir of cli's config: userDir/<cli_name>.
func CLIConfigHostPath(userDir, cliName string) string {
	return filepath.Join(userDir, harness.MustFor(cliName).Name)
}

// ProjectStateHostPath is the host state dir for a project: userDir/projects/<slug>
func ProjectStateHostPath(userDir, cwd string) string {
	return filepath.Join(userDir, "projects", slug.Path(cwd))
}

// gitConfigHostPath resolves the host's ~/.config/git to bind read-only, or "" to skip — when
// host_git_config is disabled in .ccbox.yaml or the dir is absent. HostGitConfig is non-nil:
func gitConfigHostPath(cfg *projectcfg.Config) string {
	if *cfg.HostGitConfig {
		if path, err := git.XDGConfigDir(); err == nil {
			return path
		}
	}
	return ""
}

// CLIDataBindHostPath is a data bind's shared host path: userDir/data/<cli_name>/<slug-of-key>
func CLIDataBindHostPath(userDir, cliName, key string) string {
	return filepath.Join(userDir, "data", cliName, slug.Path(key))
}

// workspaceMountPath is the in-container workspace path: the WorkingDir and bind target for the host cwd.
func workspaceMountPath(hostCwd string) string {
	return filepath.Join(containerHome, filepath.Base(hostCwd))
}
