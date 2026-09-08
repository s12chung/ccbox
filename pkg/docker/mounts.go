package docker

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/volume"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/dock"
)

const (
	ccboxLabel   = "ccbox"         // marks every ccbox volume, for cross-project discovery
	projectLabel = "ccbox.project" // the owning project, valued by host path

	// containerUID is the unprivileged in-container user (Dockerfile: useradd --uid 1000).
	containerUID = "1000"
)

// cacheVolumes maps suffix of volume name → container directory for cacheVolumeBinds()
var cacheVolumes = map[string]string{
	"go":         "/home/ccbox/go",          // go mod tidy module cache + GOBIN
	"cache":      "/home/ccbox/.cache",      // go-build + pip cache
	"gem":        "/home/ccbox/.gem",        // bundler GEM_HOME
	"npm":        "/home/ccbox/.npm",        // npm download cache
	"npm-global": "/home/ccbox/.npm-global", // global npm packages
	"local":      "/home/ccbox/.local",      // pip --user installs and opencode state
	"tmp":        "/tmp",                    // agent tmp workspace to try things out
}

// cacheVolumeName is hostCwd's named volume for a given cacheVolumes suffix.
func cacheVolumeName(hostCwd, suffix string) string {
	return "ccbox" + ProjectSlug(hostCwd) + "-" + suffix
}

// ensureCacheVolumes creates hostCwd's cache volumes labeled with the project and returns their binds.
func ensureCacheVolumes(ctxD *dock.CtxD, hostCwd string) ([]string, error) {
	for suffix := range cacheVolumes {
		opts := volume.CreateOptions{Name: cacheVolumeName(hostCwd, suffix), Labels: volumeLabels(hostCwd)}
		if _, err := ctxD.D.VolumeCreate(ctxD.Ctx, opts); err != nil {
			return nil, err
		}
	}
	return cacheVolumeBinds(hostCwd), nil
}

// cacheVolumeBinds returns the volume binds from cacheVolumes, not mounted for efficiency of small files
func cacheVolumeBinds(hostCwd string) []string {
	binds := make([]string, 0, len(cacheVolumes))
	for suffix, dir := range cacheVolumes {
		binds = append(binds, cacheVolumeName(hostCwd, suffix)+":"+dir)
	}
	sort.Strings(binds)
	return binds
}

// cliConfigMount is the in-container path the persisted config dir binds to for cliName
func cliConfigMount(cliName string) string {
	return path.Join(containerHome, harness.MustFor(cliName).ConfigHomeMount)
}

// workspaceMount is the in-container workspace path: the WorkingDir and bind target for the host cwd.
func workspaceMount(hostCwd string) string {
	return filepath.Join(containerHome, filepath.Base(hostCwd))
}

// ProjectSlug is ccbox's per-project key: the host cwd slugified (e.g. /Users/me/app → -Users-me-app).
// Distinct from Claude Code's .claude/projects slug, which CC derives from its container cwd.
func ProjectSlug(hostCwd string) string {
	return strings.ReplaceAll(hostCwd, "/", "-")
}

// volumeLabels are the labels stamped on every volume ccbox creates for hostCwd.
func volumeLabels(hostCwd string) map[string]string {
	return map[string]string{ccboxLabel: "true", projectLabel: hostCwd}
}

// safeContainerPath joins a project-relative path under workspaceMount
// rejecting unsafe paths ("..", absolute)
func safeContainerPath(workspaceMount, hostPath string) (string, error) {
	dest := filepath.Join(workspaceMount, hostPath)
	if rel, err := filepath.Rel(workspaceMount, dest); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes workspace: %q", hostPath)
	}
	return dest, nil
}
