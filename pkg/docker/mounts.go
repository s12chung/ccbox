package docker

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/volume"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
)

const (
	trueVolValue = "true"

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
	"local":      "/home/ccbox/.local",      // XDG data dir
	"tmp":        "/tmp",                    // agent tmp workspace to try things out
}

// globalVolumes maps volume name → container directory for volumes shared by every project
var globalVolumes = map[string]string{
	"ccbox-clis": install.DefaultRoot, // ccboxtools installs the coding CLI here at start
}

// cacheVolumeName is hostCwd's named volume for a given cacheVolumes suffix.
func cacheVolumeName(hostCwd, suffix string) string {
	return "ccbox" + ProjectSlug(hostCwd) + "-" + suffix
}

// cacheVolumeNames maps volume name → container dir under hostCwd's project slug.
func cacheVolumeNames(hostCwd string) map[string]string {
	volumes := make(map[string]string, len(cacheVolumes))
	for suffix, dir := range cacheVolumes {
		volumes[cacheVolumeName(hostCwd, suffix)] = dir
	}
	return volumes
}

// workspaceMount is the in-container workspace path: the WorkingDir and bind target for the host cwd.
func workspaceMount(hostCwd string) string {
	return filepath.Join(containerHome, filepath.Base(hostCwd))
}

func globalVolumeLabels() map[string]string {
	return mergeempty.Map(volumeLabels(), map[string]string{"ccbox.global": trueVolValue})
}

// projectVolumeLabels are the labels stamped on every project volume ccbox creates for hostCwd.
func projectVolumeLabels(hostCwd string) map[string]string {
	return mergeempty.Map(volumeLabels(), map[string]string{"ccbox.project": hostCwd})
}

func volumeLabels() map[string]string { return map[string]string{"ccbox": trueVolValue} }

// ProjectSlug is ccbox's per-project key: the host cwd slugified (e.g. /Users/me/app → -Users-me-app).
func ProjectSlug(hostCwd string) string {
	return strings.ReplaceAll(hostCwd, "/", "-")
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

// ensureVolumes creates each named volume with labels and returns its sorted binds.
func ensureVolumes(ctxD *dock.CtxD, volumes map[string]string, labels map[string]string) ([]string, error) {
	for name := range volumes {
		opts := volume.CreateOptions{Name: name, Labels: labels}
		if _, err := ctxD.D.VolumeCreate(ctxD.Ctx, opts); err != nil {
			return nil, err
		}
	}
	return volumeBinds(volumes), nil
}

// volumeBinds renders name:dir binds, sorted for a deterministic spec.
func volumeBinds(volumes map[string]string) []string {
	binds := make([]string, 0, len(volumes))
	for name, dir := range volumes {
		binds = append(binds, name+":"+dir)
	}
	sort.Strings(binds)
	return binds
}
