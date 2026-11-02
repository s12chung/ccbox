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

// devboxHoldersDir is the boot's holder flock dir on the host, keyed by its name
func devboxHoldersDir(name string) string {
	return filepath.Join(userdir.Tmp(), "devbox-holders", name)
}

// devboxLogPath is the boot's own host log file, outliving any single run — one file per variant
func devboxLogPath(serveVNC, noProxy bool) string {
	return filepath.Join(userdir.Dir(), "devbox"+variantSuffix(serveVNC, noProxy)+".log")
}

// proxyLogPath is tinyproxy's host log file, outliving any single run — the proxy
// only runs on proxied boots, so there is no plain variant's file
func proxyLogPath(serveVNC bool) string {
	return filepath.Join(userdir.Dir(), "proxy"+variantSuffix(serveVNC, false)+".log")
}

// workspaceMount is the in-container workspace path: the WorkingDir and bind target for the project dir.
func workspaceMount(projectDir string) string {
	return filepath.Join(cfg.ContainerHome, filepath.Base(projectDir))
}
