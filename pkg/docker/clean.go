package docker

import (
	"errors"

	"github.com/containerd/errdefs"

	"github.com/s12chung/ccbox/pkg/kit/dock"
)

// VolumeClean removes hostCwd's cache volumes and the mask volumes for maskPaths
func VolumeClean(ctxD *dock.CtxD, hostCwd string, maskPaths []string) error {
	names := make([]string, 0, len(cacheVolumes)+len(maskPaths))
	for suffix := range cacheVolumes {
		names = append(names, cacheVolumeName(hostCwd, suffix))
	}
	for _, p := range maskPaths {
		names = append(names, maskVolumeName(hostCwd, p))
	}

	var errs []error
	for _, name := range names {
		if err := ctxD.D.VolumeRemove(ctxD.Ctx, name, false); err != nil && !errdefs.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
