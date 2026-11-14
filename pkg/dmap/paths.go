package dmap

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// CLIDataDir is a cli's data dir: userDir/data/<cli_name>; its binds' flattened
// refs sit under it.
func CLIDataDir(userDir, cliName string) string { return filepath.Join(userDir, "data", cliName) }

// GUIAppDataDir is a gui app's data dir: userDir/data/<app_name>/<mode> — each mode
// seeds and mounts its own tree, so a proxied seed never leaks into the plain env.
func GUIAppDataDir(userDir, appName string, isProxied bool) string {
	mode := "no-proxy"
	if isProxied {
		mode = "proxy"
	}
	return filepath.Join(userDir, "data", appName, mode)
}

// devboxHoldersDir is the devbox's holder flock dir on the host, keyed by its name
func devboxHoldersDir(name string) string {
	return filepath.Join(userdir.Tmp(), "devbox-holders", name)
}

// devboxLogPath is the devbox's own host log file, outliving any single run — one file per variant
func devboxLogPath(projectDir string, serveVNC, isProxied bool) string {
	return logPath(projectDir, "devbox"+variantSuffix(serveVNC, isProxied)+".log")
}

// proxyLiveDir is the proxy's live dir on the host: ~/.ccbox/tmp/proxy. A dir bind tracks
// the changing files as the running proxy's current configs.
func proxyLiveDir() string { return filepath.Join(userdir.Tmp(), "proxy") }

// proxyAllowPath is the rendered allow file within proxyLiveDir
func proxyAllowPath() string { return filepath.Join(proxyLiveDir(), pkginfo.ProxyAllowFile) }

// proxyLogPath is tinyproxy's host log file, outliving any single run — the proxy
// only serves the proxied variant, so there is no plain variant's file
func proxyLogPath(projectDir string, serveVNC bool) string {
	return logPath(projectDir, "proxy"+variantSuffix(serveVNC, true)+".log")
}

// logPath is a run's host log file: userdir/logs/<project-slug>/<filename>
func logPath(projectDir, filename string) string {
	return filepath.Join(userdir.Dir(), "logs", slug.Path(projectDir), filename)
}

// workspaceMount is the in-container workspace path: the WorkingDir and bind target for the project dir.
func workspaceMount(projectDir string) string {
	return filepath.Join(pkginfo.ContainerHome, filepath.Base(projectDir))
}
