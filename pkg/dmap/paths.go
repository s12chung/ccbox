package dmap

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

const (
	containerHome     = "/home/ccbox"                // mounts sit under containerHome at a per-project leaf
	projectStateMount = "/home/ccbox/.ccbox/project" // per-project devbox state (e.g. lessons)

	gitConfigMount = "/home/ccbox/.config/git" // host global git dir, read-only (git's default XDG path)
)

// CLIConfigDir is the host dir of cli's config: userDir/<cli_name>.
func CLIConfigDir(userDir, cliName string) string {
	return filepath.Join(userDir, harness.MustFor(cliName).Name)
}

// ProjectStateDir is the host state dir for a project: userDir/projects/<slug>
func ProjectStateDir(userDir, projectDir string) string {
	return filepath.Join(userDir, "projects", slug.Path(projectDir))
}

// CLIDataBindPath is a data bind's shared host path: userDir/data/<cli_name>/<slug-of-key>
func CLIDataBindPath(userDir, cliName, key string) string {
	return filepath.Join(userDir, "data", cliName, slug.Path(key))
}

// workspaceMount is the in-container workspace path: the WorkingDir and bind target for the project dir.
func workspaceMount(projectDir string) string {
	return filepath.Join(containerHome, filepath.Base(projectDir))
}
