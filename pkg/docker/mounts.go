package docker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/volume"

	"github.com/s12chung/ccbox/pkg/dockerutil"
)

const (
	ccboxLabel   = "ccbox"         // marks every ccbox volume, for cross-project discovery
	projectLabel = "ccbox.project" // the owning project, valued by host path
)

// volumeLabels are the labels stamped on every volume ccbox creates for hostCwd.
func volumeLabels(hostCwd string) map[string]string {
	return map[string]string{ccboxLabel: "true", projectLabel: hostCwd}
}

// containerUID is the unprivileged in-container user (Dockerfile: useradd --uid 1000).
const containerUID = "1000"

// tmpfsOpts makes a workspace tmpfs writable+executable by that user, so masked build
// outputs (e.g. dist/) can be written and run — Docker's default is root-owned noexec.
var tmpfsOpts = fmt.Sprintf("uid=%s,gid=%s,exec", containerUID, containerUID)

// safeContainerPath joins a workspace-relative path under workspaceMount
// rejecting unsafe paths ("..", absolute)
func safeContainerPath(workspaceMount, hostPath string) (string, error) {
	dest := filepath.Join(workspaceMount, hostPath)
	if rel, err := filepath.Rel(workspaceMount, dest); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("mask path escapes workspace: %q", hostPath)
	}
	return dest, nil
}

// tmpfsMasks maps each workspace-relative path to its tmpfs options.
func tmpfsMasks(hostCwd string, hostPaths []string) (map[string]string, error) {
	workspaceMount := WorkspaceMount(hostCwd)
	tmpfs := map[string]string{}
	for _, p := range hostPaths {
		containerPath, err := safeContainerPath(workspaceMount, p)
		if err != nil {
			return nil, err
		}
		tmpfs[containerPath] = tmpfsOpts
	}
	return tmpfs, nil
}

// cacheVolumes maps suffix of volume name → container directory for cacheVolumeBinds()
var cacheVolumes = map[string]string{
	"go":         "/home/ccbox/go",          // go mod tidy module cache + GOBIN
	"cache":      "/home/ccbox/.cache",      // go-build + pip cache
	"gem":        "/home/ccbox/.gem",        // bundler GEM_HOME
	"npm":        "/home/ccbox/.npm",        // npm download cache
	"npm-global": "/home/ccbox/.npm-global", // global npm packages
	"local":      "/home/ccbox/.local",      // pip --user installs and opencode state
}

// cacheVolumeName is hostCwd's named volume for a given cacheVolumes suffix.
func cacheVolumeName(hostCwd, suffix string) string {
	return "ccbox" + ProjectSlug(hostCwd) + "-" + suffix
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

// ensureCacheVolumes creates hostCwd's cache volumes labeled with the project and returns their binds.
func (c *Client) ensureCacheVolumes(ctx context.Context, hostCwd string) ([]string, error) {
	for suffix := range cacheVolumes {
		opts := volume.CreateOptions{Name: cacheVolumeName(hostCwd, suffix), Labels: volumeLabels(hostCwd)}
		if _, err := c.cli.VolumeCreate(ctx, opts); err != nil {
			return nil, err
		}
	}
	return cacheVolumeBinds(hostCwd), nil
}

// maskVolumeName is hostCwd's persistent volume for a workspace-relative masked dir
func maskVolumeName(hostCwd, rel string) string {
	return cacheVolumeName(hostCwd, strings.ReplaceAll(rel, "/", "-"))
}

// namedVolumeMasks returns a "volume:containerPath" bind per masked path, plus each volume's name.
func namedVolumeMasks(hostCwd string, hostPaths []string) (binds, names []string, err error) {
	workspaceMount := WorkspaceMount(hostCwd)
	for _, p := range hostPaths {
		containerPath, err := safeContainerPath(workspaceMount, p)
		if err != nil {
			return nil, nil, err
		}
		name := maskVolumeName(hostCwd, p)
		binds = append(binds, name+":"+containerPath)
		names = append(names, name)
	}
	return binds, names, nil
}

// ensureNamedVolumeMasks builds the mask binds and ensures each volume exists owned by the container user.
func (c *Client) ensureNamedVolumeMasks(ctx context.Context, hostCwd, imageTag string, hostPaths []string) ([]string, error) {
	binds, names, err := namedVolumeMasks(hostCwd, hostPaths)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if err := dockerutil.EnsureOwnedVolume(ctx, c.cli, imageTag, name, containerUID, volumeLabels(hostCwd)); err != nil {
			return nil, err
		}
	}
	return binds, nil
}

// VolumeClean removes hostCwd's cache volumes and the mask volumes for maskDirs
func (c *Client) VolumeClean(ctx context.Context, hostCwd string, maskDirs []string) error {
	names := make([]string, 0, len(cacheVolumes)+len(maskDirs))
	for suffix := range cacheVolumes {
		names = append(names, cacheVolumeName(hostCwd, suffix))
	}
	for _, d := range maskDirs {
		names = append(names, maskVolumeName(hostCwd, d))
	}

	var errs []error
	for _, name := range names {
		if err := c.cli.VolumeRemove(ctx, name, false); err != nil && !errdefs.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
