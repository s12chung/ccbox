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
	cli := rm.cfg.CLI()
	projectDir := rm.cfg.ProjectDir()
	share := harness.AgentsMdShare{CLI: cli, ScratchMountDir: cliTmpMount(cli)}
	scratchDir, clean, err := share.Begin()
	if err != nil {
		return nil, nil, err
	}

	return slices.Concat(
		[]docker.Mount{
			docker.NewBind(projectDir, workspaceMount(projectDir)),
			docker.NewBind(CLIConfigDir(rm.userDir, cli.Name), path.Join(containerHome, cli.ConfigHomeMount)),
			docker.NewBind(ProjectStateDir(rm.userDir, projectDir), projectStateMount),
		},
		volumes(globalVolumesMap, true),
		volumes(cacheVolumeNames(projectDir), false),
		volumeMasks(projectDir, rm.cfg.VolumeMasksPresent()),
		readOnlyBinds(projectDir, rm.cfg.ReadOnlyPathsPresent()),
		gitBinds(*rm.cfg.HostGitConfig),
		cliDataBinds(rm.userDir, cli),
		cliTmpBind(scratchDir, share.ScratchMountDir),
	), clean, nil
}

// volumeMasks masks each load-validated project-relative dir with a persistent
// per-project volume, owned by the container user so the run's own content seeds it.
func volumeMasks(projectDir string, relDirs []string) []docker.Mount {
	workspace := workspaceMount(projectDir)
	volumes := make([]docker.Mount, 0, len(relDirs))
	for _, d := range relDirs {
		volumes = append(volumes, docker.NewVolume(volumeName(projectDir, d), filepath.Join(workspace, d)).Owned())
	}
	return volumes
}

// readOnlyBinds re-mounts each walk-matched project-relative path read-only.
func readOnlyBinds(projectDir string, relPaths []string) []docker.Mount {
	workspace := workspaceMount(projectDir)
	binds := make([]docker.Mount, 0, len(relPaths))
	for _, p := range relPaths {
		binds = append(binds, docker.NewBind(filepath.Join(projectDir, p), filepath.Join(workspace, p)).ReadOnly())
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

// gitBinds binds the host global git dir read-only at gitConfigMount — git's default XDG path.
func gitBinds(enabled bool) []docker.Mount {
	if !enabled {
		return nil
	}
	if host := git.MustXDGConfigDir(); host != "" {
		return []docker.Mount{docker.NewBind(host, gitConfigMount).ReadOnly()}
	}
	return nil
}

func cliDataBinds(userDir string, cli harness.CLI) []docker.Mount {
	binds := make(map[string]string, len(cli.DataBinds))
	for key := range cli.DataBinds {
		host := CLIDataBindPath(userDir, cli.Name, key)
		binds[host] = key
	}

	dataBinds := make([]docker.Mount, 0, len(binds))
	for _, host := range slices.Sorted(maps.Keys(binds)) {
		dataBinds = append(dataBinds, docker.NewBind(host, path.Join(containerHome, binds[host])))
	}
	return dataBinds
}

// cliTmpMount is the CLI's tmp dir's container path: <containerHome>/.ccbox/tmp/<cli_name>
func cliTmpMount(cli harness.CLI) string { return path.Join(containerHome, ".ccbox", "tmp", cli.Name) }

// cliTmpBind binds the shared doc's scratch dir at cliTmpMount; the cliFile symlink
// points into it. host "" = no bind.
func cliTmpBind(scratchDir, mount string) []docker.Mount {
	if scratchDir == "" {
		return nil
	}
	return []docker.Mount{docker.NewBind(scratchDir, mount)}
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
