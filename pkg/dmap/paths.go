package dmap

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// persistMount is the container path of the run's persistent per-project state.
const persistMount = projectcfg.ContainerHome + "/.ccbox/persist"

// CLIConfigDir is the host dir of cli's config: userDir/<cli_name>.
func CLIConfigDir(userDir, cliName string) string {
	return filepath.Join(userDir, harness.MustFor(cliName).Name)
}

// CLIDataBindPath is a data bind's shared host path: userDir/data/<cli_name>/<slug-of-key>
func CLIDataBindPath(userDir, cliName, key string) string {
	return filepath.Join(userDir, "data", cliName, slug.Path(key))
}

// ProxyLogPath is the host file an auto-started wall's logs are appended to: userDir/proxy.log
func ProxyLogPath(userDir string) string { return filepath.Join(userDir, "proxy.log") }

// workspaceMount is the in-container workspace path: the WorkingDir and bind target for the project dir.
func workspaceMount(projectDir string) string {
	return filepath.Join(projectcfg.ContainerHome, filepath.Base(projectDir))
}
