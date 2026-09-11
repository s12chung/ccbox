package dmap

import (
	"maps"
	"path"
	"path/filepath"
	"slices"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/git"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// tmpfsMasks maps each project-relative dir to its masked container path.
func tmpfsMasks(hostCwd string, hostDirs []string) []string {
	workspace := workspaceMountPath(hostCwd)
	paths := make([]string, 0, len(hostDirs))
	for _, d := range hostDirs {
		paths = append(paths, filepath.Join(workspace, d))
	}
	return paths
}

// binds composes the run's binds and volumes in a deterministic order: the workspace and
// host dirs, then the shared volumes, then the config's masks and per-CLI binds. The
// shared agents doc's scratch bind is appended after, once AgentsMdShare has begun.
func (rm *RunMap) binds() ([]docker.Mount, func() error, error) {
	cwd := rm.cfg.ProjectDir()
	scratch, clean, err := harness.AgentsMdShare{CLI: rm.cli}.Begin()
	if err != nil {
		return nil, nil, err
	}

	return slices.Concat(
		[]docker.Mount{
			docker.NewBind(cwd, workspaceMountPath(cwd)),
			docker.NewBind(CLIConfigHostPath(rm.userDir, rm.cli.Name), path.Join(containerHome, rm.cli.ConfigHomeMount)),
			docker.NewBind(ProjectStateHostPath(rm.userDir, cwd), projectStateMountPath),
		},
		volumes(globalVolumesMap, true),
		volumes(cacheVolumeNames(cwd), false),
		volumeMasks(cwd, rm.cfg.VolumeMasksPresent()),
		readOnlyBinds(cwd, rm.cfg.ReadOnlyPathsPresent()),
		gitBinds(*rm.cfg.HostGitConfig),
		cliDataBinds(rm.userDir, rm.cli),
		agentsMdBind(scratch, rm.cli),
	), clean, nil
}

// volumeMasks masks each load-validated project-relative dir with a persistent
// per-project volume, owned by the container user so the run's own content seeds it.
func volumeMasks(hostCwd string, hostDirs []string) []docker.Mount {
	workspace := workspaceMountPath(hostCwd)
	volumes := make([]docker.Mount, 0, len(hostDirs))
	for _, d := range hostDirs {
		volumes = append(volumes, docker.NewVolume(volumeName(hostCwd, d), filepath.Join(workspace, d)).Owned())
	}
	return volumes
}

// readOnlyBinds re-mounts each walk-matched project-relative path read-only.
func readOnlyBinds(hostCwd string, hostPaths []string) []docker.Mount {
	workspace := workspaceMountPath(hostCwd)
	binds := make([]docker.Mount, 0, len(hostPaths))
	for _, p := range hostPaths {
		binds = append(binds, docker.NewBind(filepath.Join(hostCwd, p), filepath.Join(workspace, p)).ReadOnly())
	}
	return binds
}

// volumes renders a name→dir map as volume binds under the given scope, sorted for a
// deterministic spec.
func volumes(nameDirs map[string]string, global bool) []docker.Mount {
	volumes := make([]docker.Mount, 0, len(nameDirs))
	for _, name := range slices.Sorted(maps.Keys(nameDirs)) {
		volume := docker.NewVolume(name, nameDirs[name])
		if global {
			volume = volume.Global()
		}
		volumes = append(volumes, volume)
	}
	return volumes
}

// gitBinds binds the host global git dir read-only at gitConfigMountPath — git's default XDG path.
func gitBinds(enabled bool) []docker.Mount {
	if !enabled {
		return nil
	}
	if host := git.MustXDGConfigDir(); host != "" {
		return []docker.Mount{docker.NewBind(host, gitConfigMountPath).ReadOnly()}
	}
	return nil
}

func cliDataBinds(userDir string, cli harness.CLI) []docker.Mount {
	binds := make(map[string]string, len(cli.DataBinds))
	for key := range cli.DataBinds {
		host := CLIDataBindHostPath(userDir, cli.Name, key)
		binds[host] = key
	}

	dataBinds := make([]docker.Mount, 0, len(binds))
	for _, host := range slices.Sorted(maps.Keys(binds)) {
		dataBinds = append(dataBinds, docker.NewBind(host, path.Join(containerHome, binds[host])))
	}
	return dataBinds
}

// agentsMdBind binds the shared doc's scratch copy at the CLI's config path.
// host "" = the CLI has its own agents file: no bind.
func agentsMdBind(host string, cli harness.CLI) []docker.Mount {
	if host == "" {
		return nil
	}
	return []docker.Mount{docker.NewBind(host, path.Join(containerHome, cli.ConfigHomeMount, cli.SeedAgentsFilename))}
}

// globalVolumesMap maps volume name → container directory for volumes shared by every project
var globalVolumesMap = map[string]string{
	"ccbox-clis": install.DefaultRoot, // ccboxtools installs the coding CLI here at start
}

// cacheVolumeSuffix is the suffix for cacheVolumesMap in case of collisions
const cacheVolumeSuffix = "-cache-default"

// cacheVolumesMap maps suffix of volume name → container directory for cacheVolumeNames()
var cacheVolumesMap = map[string]string{
	"go":         "/home/ccbox/go",          // go mod tidy module cache + GOBIN
	"cache":      "/home/ccbox/.cache",      // go-build + pip cache
	"gem":        "/home/ccbox/.gem",        // bundler GEM_HOME
	"npm":        "/home/ccbox/.npm",        // npm download cache
	"npm-global": "/home/ccbox/.npm-global", // global npm packages
	"local":      "/home/ccbox/.local",      // XDG data dir
	"tmp":        "/tmp",                    // agent tmp workspace to try things out
}

// cacheVolumeNames maps volume name → container dir under hostCwd's project slug
func cacheVolumeNames(hostCwd string) map[string]string {
	volumes := make(map[string]string, len(cacheVolumesMap))
	for suffix, dir := range cacheVolumesMap {
		volumes[volumeName(hostCwd, suffix)+cacheVolumeSuffix] = dir
	}
	return volumes
}

// volumeName is hostCwd's named volume for a given cacheVolumesMap suffix.
func volumeName(hostCwd, suffix string) string {
	return "ccbox" + slug.Path(hostCwd) + "-" + slug.Path(suffix)
}
