package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/kit/dock"
)

// tmpfsOpts makes a masked project dir's tmpfs writable+executable by that user, so masked build
// outputs (e.g. dist/) can be written and run — Docker's default is root-owned noexec.
var tmpfsOpts = fmt.Sprintf("uid=%s,gid=%s,exec", containerUID, containerUID)

// tmpfsMasks maps each project-relative dir to its tmpfs options.
func tmpfsMasks(hostCwd string, hostDirs []string) (map[string]string, error) {
	workspaceMount := workspaceMount(hostCwd)
	tmpfs := map[string]string{}
	for _, d := range hostDirs {
		containerPath, err := safeContainerPath(workspaceMount, d)
		if err != nil {
			return nil, err
		}
		tmpfs[containerPath] = tmpfsOpts
	}
	return tmpfs, nil
}

// maskVolumeName is hostCwd's persistent volume for a project-relative masked dir
func maskVolumeName(hostCwd, rel string) string {
	return cacheVolumeName(hostCwd, strings.ReplaceAll(rel, "/", "-"))
}

// ensureNamedVolumeMasks builds the mask binds and ensures each volume exists owned by the container user.
func ensureNamedVolumeMasks(ctxD *dock.CtxD, hostCwd, imageTag string, hostDirs []string) ([]string, error) {
	binds, names, err := namedVolumeMasks(hostCwd, hostDirs)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		if err := dock.EnsureOwnedVolume(ctxD, imageTag, name, containerUID, projectVolumeLabels(hostCwd)); err != nil {
			return nil, err
		}
	}
	return binds, nil
}

// namedVolumeMasks returns a "volume:containerPath" bind per masked dir, plus each volume's name.
func namedVolumeMasks(hostCwd string, hostDirs []string) ([]string, []string, error) {
	workspaceMount := workspaceMount(hostCwd)
	var binds, names []string
	for _, d := range hostDirs {
		containerPath, err := safeContainerPath(workspaceMount, d)
		if err != nil {
			return nil, nil, err
		}
		name := maskVolumeName(hostCwd, d)
		binds = append(binds, name+":"+containerPath)
		names = append(names, name)
	}
	return binds, names, nil
}

// readOnlyPathBinds returns a "hostPath:containerPath:ro" bind per read-only path
func readOnlyPathBinds(hostCwd string, paths []string) ([]string, error) {
	workspaceMount := workspaceMount(hostCwd)
	var binds []string
	for _, path := range paths {
		containerPath, err := safeContainerPath(workspaceMount, path)
		if err != nil {
			return nil, err
		}
		binds = append(binds, filepath.Join(hostCwd, path)+":"+containerPath+":ro")
	}
	return binds, nil
}
