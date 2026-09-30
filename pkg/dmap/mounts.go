package dmap

import (
	"cmp"
	"maps"
	"path"
	"path/filepath"
	"slices"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/dmap/share"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// tmpfsMasks maps each project-relative dir to its masked container path.
func tmpfsMasks(projectDir string, relDirs []string) []string {
	workspace := workspaceMount(projectDir)
	paths := make([]string, 0, len(relDirs))
	for _, d := range relDirs {
		paths = append(paths, filepath.Join(workspace, d))
	}
	return paths
}

// binds composes the run's binds and volumes in a deterministic order
func (rm *RunMap) binds() ([]docker.Mount, func() error, error) {
	c := rm.cfg.CLI()
	projectDir := rm.cfg.ProjectDir()
	var scratchBind, persistBind klean.ShareBind
	clean, err := klean.Share{
		share.AgentsMd(c.Name),
		share.PersistDir(projectDir),
	}.Begin(&scratchBind, &persistBind)
	if err != nil {
		return nil, clean, err
	}

	return slices.Concat(
		[]docker.Mount{
			docker.NewBind(projectDir, workspaceMount(projectDir)),
			docker.NewBind(cli.UserConfigDir(c.Name), path.Join(projectcfg.ContainerHome, c.ConfigHomeMount)),
			docker.NewBind(persistBind.HostPath, persistBind.ContainerPath),
		},
		volumes(globalVolumesMap, true),
		volumes(cacheVolumeNames(projectDir), false),
		volumeMasks(projectDir, rm.cfg.VolumeMasksPresent()),
		readOnlyGlobBinds(projectDir, rm.cfg.ReadOnlyPathsPresent()),
		readOnlyBinds(rm.cfg.ReadOnlyBindsPresent()),
		cliDataBinds(rm.userDir, c),
		cliScratchBind(scratchBind.HostPath, scratchBind.ContainerPath),
	), clean, nil
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

// volumeMasks masks each load-validated project-relative dir with a persistent
// per-project volume; ensured fresh, then chowned to the container user, so the run's
// own content seeds it.
func volumeMasks(projectDir string, relDirs []string) []docker.Mount {
	workspace := workspaceMount(projectDir)
	volumes := make([]docker.Mount, 0, len(relDirs))
	for _, d := range relDirs {
		volumes = append(volumes, docker.NewVolume(volumeName(projectDir, d), filepath.Join(workspace, d)))
	}
	return volumes
}

// readOnlyGlobBinds re-mounts each walk-matched project-relative path read-only.
func readOnlyGlobBinds(projectDir string, relPaths []string) []docker.Mount {
	workspace := workspaceMount(projectDir)
	binds := make([]docker.Mount, 0, len(relPaths))
	for _, p := range relPaths {
		binds = append(binds, docker.NewBind(filepath.Join(projectDir, p), filepath.Join(workspace, p)).ReadOnly())
	}
	return binds
}

// readOnlyBinds binds each host dir read-only at its container mount, sorted by the mount for
// a deterministic spec.
func readOnlyBinds(binds map[string]string) []docker.Mount {
	hosts := slices.SortedFunc(maps.Keys(binds), func(a, b string) int {
		return cmp.Compare(binds[a], binds[b]) // sort by the container mount
	})
	mounts := make([]docker.Mount, 0, len(hosts))
	for _, host := range hosts {
		mounts = append(mounts, docker.NewBind(host, binds[host]).ReadOnly())
	}
	return mounts
}

// cliDataBinds renders cli's data binds as binds under the container home, sorted for a
// deterministic spec.
func cliDataBinds(userDir string, c cli.CLI) []docker.Mount {
	binds := make(map[string]string, len(c.DataBinds))
	for key := range c.DataBinds {
		host := CLIDataBindPath(userDir, c.Name, key)
		binds[host] = key
	}

	dataBinds := make([]docker.Mount, 0, len(binds))
	for _, host := range slices.Sorted(maps.Keys(binds)) {
		dataBinds = append(dataBinds, docker.NewBind(host, path.Join(projectcfg.ContainerHome, binds[host])))
	}
	return dataBinds
}

// cliScratchBind binds the shared doc's scratch file at its container mount; the cliFile
// symlink points at it. host "" = no bind.
func cliScratchBind(scratchFile, mount string) []docker.Mount {
	if scratchFile == "" {
		return nil
	}
	return []docker.Mount{docker.NewBind(scratchFile, mount)}
}

// globalVolumesMap maps volume name → container directory for volumes shared by every project
var globalVolumesMap = map[string]string{
	"ccbox-clis": install.DefaultRoot, // ccboxtools installs the coding CLI here at start
	"ccbox-apps": install.AppsRoot,    // ccboxtools installs the GUI app here at start
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

// cacheVolumeNames maps volume name → container dir under the projectDir's slug
func cacheVolumeNames(projectDir string) map[string]string {
	volumes := make(map[string]string, len(cacheVolumesMap))
	for suffix, dir := range cacheVolumesMap {
		volumes[volumeName(projectDir, suffix)+cacheVolumeSuffix] = dir
	}
	return volumes
}

// volumeName is projectDir's named volume for a given cacheVolumesMap suffix.
func volumeName(projectDir, suffix string) string {
	return "ccbox" + slug.Path(projectDir) + "-" + slug.Path(suffix)
}
