package dmap

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// CLIDataBindPath is a data bind's shared host path: userDir/data/<cli_name>/<slug-of-key>
func CLIDataBindPath(userDir, cliName, key string) string {
	return filepath.Join(userDir, "data", cliName, slug.Path(key))
}

// proxyLogPath is the proxy session's host log file: ~/.ccbox/proxy.log — the proxy's
// creator appends the container's log stream here, outliving any single run
func proxyLogPath() string { return filepath.Join(userdir.Dir(), "proxy.log") }

// workspaceMount is the in-container workspace path: the WorkingDir and bind target for the project dir.
func workspaceMount(projectDir string) string {
	return filepath.Join(cfg.ContainerHome, filepath.Base(projectDir))
}
