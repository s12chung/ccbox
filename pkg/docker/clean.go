package docker

import (
	"errors"

	"github.com/containerd/errdefs"

	"github.com/s12chung/ccbox/pkg/kit/dock"
)

// VolumeClean removes hostCwd's cache volumes and the mask volumes for maskDirs
func VolumeClean(ctxD *dock.CtxD, hostCwd string, maskDirs []string) error {
	names := make([]string, 0, len(cacheVolumes)+len(maskDirs))
	for suffix := range cacheVolumes {
		names = append(names, cacheVolumeName(hostCwd, suffix))
	}
	for _, d := range maskDirs {
		names = append(names, maskVolumeName(hostCwd, d))
	}

	var errs []error
	for _, name := range names {
		if err := ctxD.D.VolumeRemove(ctxD.Ctx, name, false); err != nil && !errdefs.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
